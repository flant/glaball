package users

import (
	"fmt"
	"os"
	"regexp"
	"text/tabwriter"

	"github.com/perhamm/glaball/pkg/client"
	"github.com/perhamm/glaball/pkg/limiter"
	"github.com/perhamm/glaball/pkg/sort/v2"
	"github.com/perhamm/glaball/pkg/util"

	"github.com/perhamm/glaball/cmd/common"

	"github.com/hashicorp/go-hclog"
	"github.com/spf13/cobra"
	"github.com/xanzy/go-gitlab"
)

var (
	blockBy          string
	blockFieldRegexp *regexp.Regexp
	blockHosts       bool
)

func NewBlockCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "block --by=[email|username|name] [regexp]",
		Short: "Blocks an existing user",
		Long:  "Blocks an existing user. Only administrators can change attributes of a user.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			re, err := regexp.Compile(args[0])
			if err != nil {
				return err
			}
			blockFieldRegexp = re
			return Block()
		},
	}

	cmd.Flags().Var(util.NewEnumValue(&blockBy, "email", "username", "name"), "by", "Search user you want to block")
	cmd.MarkFlagRequired("by")

	cmd.Flags().BoolVar(&blockHosts, "hosts", false, "List hosts where user exists")

	return cmd
}

func Block() error {
	wg := common.Limiter
	data := make(chan interface{})

	fmt.Printf("Searching for user %q...\n", blockFieldRegexp)
	for _, h := range common.Client.Hosts {
		wg.Add(1)
		go listUsersSearch(h, blockBy, blockFieldRegexp, gitlab.ListUsersOptions{
			ListOptions: gitlab.ListOptions{
				PerPage: 100,
			},
		}, wg, data, common.Client.WithNoCache())
	}

	go func() {
		wg.Wait()
		close(data)
	}()

	toBlock := make(sort.Elements, 0)
	for e := range data {
		toBlock = append(toBlock, e)
	}

	if len(toBlock) == 0 {
		return fmt.Errorf("user not found: %s", blockFieldRegexp)
	}

	if blockHosts {
		for _, h := range toBlock.Hosts() {
			fmt.Println(h.Project)
		}
		return nil
	}

	util.AskUser(fmt.Sprintf("Do you really want to block %d user(s) %q in %d gitlab(s) %v ?",
		len(toBlock), blockFieldRegexp, len(toBlock.Hosts()), toBlock.Hosts().Projects(common.Config.ShowAll)))

	blocked := make(chan interface{})
	for _, v := range toBlock.Typed() {
		wg.Add(1)
		go blockUser(v.Host, v.Struct.(*gitlab.User), wg, blocked)
	}

	go func() {
		wg.Wait()
		close(blocked)
	}()

	results, err := sort.FromChannel(blocked, &sort.Options{
		OrderBy:    []string{blockBy},
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

	fmt.Fprintf(w, "Blocked: %d\nErrors: %d\n", total, len(wg.Errors()))

	w.Flush()

	for _, err := range wg.Errors() {
		hclog.L().Error(err.Err.Error())
	}

	return nil

}

func blockUser(h *client.Host, user *gitlab.User, wg *limiter.Limiter, data chan<- interface{},
	options ...gitlab.RequestOptionFunc) {

	defer wg.Done()

	wg.Lock()
	err := h.Client.Users.BlockUser(user.ID, options...)
	if err != nil {
		wg.Error(h, err)
		wg.Unlock()
		return
	}
	wg.Unlock()

	data <- sort.Element{Host: h, Struct: user, Cached: false}
}
