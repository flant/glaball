package users

import (
	"fmt"
	"os"
	"regexp"
	"text/tabwriter"

	"github.com/flant/glaball/pkg/client"
	"github.com/flant/glaball/pkg/limiter"
	"github.com/flant/glaball/pkg/sort/v2"
	"github.com/flant/glaball/pkg/util"

	"github.com/flant/glaball/cmd/common"

	"github.com/hashicorp/go-hclog"
	"github.com/spf13/cobra"
	"github.com/xanzy/go-gitlab"
)

var (
	deleteBy          string
	deleteFieldRegexp *regexp.Regexp
	deleteHosts       bool
)

func NewDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete --by=[email|username|name] [regexp]",
		Short: "Deletes a user",
		Long: `Deletes a user. Available only for administrators.
This returns a 204 No Content status code if the operation was successfully,
404 if the resource was not found or 409 if the user cannot be soft deleted.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			re, err := regexp.Compile(args[0])
			if err != nil {
				return err
			}
			deleteFieldRegexp = re
			return Delete()
		},
	}

	cmd.Flags().Var(util.NewEnumValue(&deleteBy, "email", "username", "name"), "by", "Search user you want to delete")
	cmd.MarkFlagRequired("by")

	cmd.Flags().BoolVar(&deleteHosts, "hosts", false, "List hosts where user exists")

	return cmd
}

func Delete() error {
	wg := common.Limiter
	data := make(chan interface{})

	fmt.Printf("Searching for user %q...\n", deleteFieldRegexp)
	for _, h := range common.Client.Hosts {
		wg.Add(1)
		go listUsersSearch(h, deleteBy, deleteFieldRegexp, gitlab.ListUsersOptions{
			ListOptions: gitlab.ListOptions{
				PerPage: 100,
			},
		}, wg, data)
	}

	go func() {
		wg.Wait()
		close(data)
	}()

	toDelete := make(sort.Elements, 0)
	for e := range data {
		toDelete = append(toDelete, e)
	}

	if len(toDelete) == 0 {
		return fmt.Errorf("user not found: %s", deleteFieldRegexp)
	}

	if deleteHosts {
		for _, h := range toDelete.Hosts() {
			fmt.Println(h.Project)
		}
		return nil
	}

	// do not allow to delete more than 1 user
	if len(toDelete) > 1 {
		return fmt.Errorf("you don't want to use it as bulk function")
	}

	util.AskUser(fmt.Sprintf("Do you really want to delete user %q in %d gitlab(s) %v ?",
		deleteFieldRegexp, len(toDelete.Hosts()), toDelete.Hosts().Projects(common.Config.ShowAll)))

	deleted := make(chan interface{})
	for _, v := range toDelete.Typed() {
		wg.Add(1)
		go deleteUser(v.Host, v.Struct.(*gitlab.User), wg, deleted)
	}

	go func() {
		wg.Wait()
		close(deleted)
	}()

	results, err := sort.FromChannel(deleted, &sort.Options{
		OrderBy:    []string{deleteBy},
		StructType: gitlab.User{},
	})
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.TabIndent)
	fmt.Fprintf(w, "COUNT\tUSER\tHOSTS\tCACHED\n")

	total := 0
	for _, v := range results {
		fmt.Fprintf(w, "[%d]\t%s\t%s\t[%s]\n", v.Count, v.Key, v.Elements.Hosts().Projects(common.Config.ShowAll), v.Cached)
		total++
	}

	fmt.Fprintf(w, "Deleted: %d\nErrors: %d\n", total, len(wg.Errors()))

	w.Flush()

	for _, err := range wg.Errors() {
		hclog.L().Error(err.Err.Error())
	}

	return nil

}

func deleteUser(h *client.Host, user *gitlab.User, wg *limiter.Limiter, data chan<- interface{},
	options ...gitlab.RequestOptionFunc) {

	defer wg.Done()

	wg.Lock()
	resp, err := h.Client.Users.DeleteUser(user.ID, options...)
	if err != nil {
		wg.Error(h, err)
		wg.Unlock()
		return
	}
	wg.Unlock()

	data <- sort.Element{Host: h, Struct: user, Cached: resp.Header.Get("X-From-Cache") == "1"}
}
