package users

import (
	"fmt"
	"os"
	"regexp"
	"text/tabwriter"

	"github.com/flant/glaball/pkg/client"
	"github.com/flant/glaball/pkg/limiter"
	"github.com/flant/glaball/pkg/sort"
	"github.com/flant/glaball/pkg/util"

	"github.com/flant/glaball/cmd/common"

	"github.com/hashicorp/go-hclog"
	"github.com/spf13/cobra"
	"github.com/xanzy/go-gitlab"
)

var (
	modifyOpt         = gitlab.ModifyUserOptions{}
	modifyBy          string
	modifyFieldRegexp *regexp.Regexp

	listHosts bool
)

func NewModifyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "modify --by=[email|username|name] [regexp]",
		Short: "Modifies an existing user",
		Long:  "Modifies an existing user. Only administrators can change attributes of a user.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			re, err := regexp.Compile(args[0])
			if err != nil {
				return err
			}
			modifyFieldRegexp = re
			return Modify()
		},
	}

	cmd.Flags().Var(util.NewEnumValue(&modifyBy, "email", "username", "name"), "by", "Search user you want to modify")
	cmd.MarkFlagRequired("by")

	cmd.Flags().BoolVar(&listHosts, "hosts", false, "List hosts where user exists")

	// ModifyUserOptions
	cmd.Flags().Var(util.NewStringPtrValue(&modifyOpt.Email), "email", "Email.")
	cmd.Flags().Var(util.NewStringPtrValue(&modifyOpt.Password), "password", "Password.")
	cmd.Flags().Var(util.NewStringPtrValue(&modifyOpt.Username), "username", "Username")
	cmd.Flags().Var(util.NewStringPtrValue(&modifyOpt.Name), "name", "Name")
	cmd.Flags().Var(util.NewStringPtrValue(&modifyOpt.Skype), "skype", "Skype ID")
	cmd.Flags().Var(util.NewStringPtrValue(&modifyOpt.Linkedin), "linkedin", "LinkedIn")
	cmd.Flags().Var(util.NewStringPtrValue(&modifyOpt.Twitter), "twitter", "Twitter account.")
	cmd.Flags().Var(util.NewStringPtrValue(&modifyOpt.WebsiteURL), "website_url", "Website URL")
	cmd.Flags().Var(util.NewStringPtrValue(&modifyOpt.Organization), "organization", "Organization name")
	cmd.Flags().Var(util.NewBoolPtrValue(&modifyOpt.Admin), "admin", "User is admin - true or false (default).")
	cmd.Flags().Var(util.NewBoolPtrValue(&modifyOpt.CanCreateGroup), "can_create_group",
		"User can create groups - true or false.")
	cmd.Flags().Var(util.NewBoolPtrValue(&modifyOpt.SkipReconfirmation), "skip_reconfirmation",
		"Skip reconfirmation - true or false (default).")
	cmd.Flags().Var(util.NewBoolPtrValue(&modifyOpt.External), "external",
		"Flags the user as external - true or false. Default is false")
	cmd.Flags().Var(util.NewBoolPtrValue(&modifyOpt.PrivateProfile), "private_profile",
		"User’s profile is private - true, false (default), or null (is converted to false).")
	cmd.Flags().Var(util.NewStringPtrValue(&modifyOpt.Note), "note", "Admin notes for this user.")

	return cmd
}

func Modify() error {
	wg := common.Limiter
	data := make(chan interface{})

	fmt.Printf("Searching for user %q...\n", modifyFieldRegexp)
	for _, h := range common.Client.Hosts {
		wg.Add(1)
		go listUsersSearch(h, modifyBy, modifyFieldRegexp, gitlab.ListUsersOptions{
			ListOptions: gitlab.ListOptions{
				PerPage: 100,
			},
		}, wg, data, common.Client.WithNoCache())
	}

	go func() {
		wg.Wait()
		close(data)
	}()

	toModify := make(sort.Elements, 0)
	for e := range data {
		toModify = append(toModify, e)
	}

	if len(toModify) == 0 {
		return fmt.Errorf("user not found: %s", modifyFieldRegexp)
	}

	if listHosts {
		for _, h := range toModify.Hosts() {
			fmt.Println(h.Project)
		}
		return nil
	}

	util.AskUser(fmt.Sprintf("Do you really want to modify %d users %q in %d gitlab(s) %v ?",
		len(toModify), modifyFieldRegexp, len(toModify.Hosts()), toModify.Hosts().Projects()))

	modified := make(chan interface{})
	for _, v := range toModify.Typed() {
		wg.Add(1)
		go modifyUser(v.Host, v.Struct.(*gitlab.User).ID, modifyOpt, wg, modified)
	}

	go func() {
		wg.Wait()
		close(modified)
	}()

	results := sort.FromChannel(modified, &sort.Options{
		OrderBy:    []string{modifyBy},
		StructType: gitlab.User{},
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.TabIndent)
	fmt.Fprintf(w, "COUNT\tUSER\tHOSTS\tCACHED\n")

	total := 0
	for _, v := range results {
		fmt.Fprintf(w, "[%d]\t%s\t%s\t[%s]\n", v.Count, v.Key, v.Elements.Hosts().Projects(), v.Cached)
		total++
	}

	fmt.Fprintf(w, "Modified: %d\nErrors: %d\n", total, len(wg.Errors()))

	w.Flush()

	for _, err := range wg.Errors() {
		hclog.L().Error(err.Err.Error())
	}

	return nil

}

func modifyUser(h *client.Host, id int, opt gitlab.ModifyUserOptions, wg *limiter.Limiter, data chan<- interface{},
	options ...gitlab.RequestOptionFunc) {

	defer wg.Done()

	wg.Lock()
	user, resp, err := h.Client.Users.ModifyUser(id, &opt, options...)
	if err != nil {
		wg.Error(h, err)
		wg.Unlock()
		return
	}
	wg.Unlock()

	data <- sort.Element{Host: h, Struct: user, Cached: resp.Header.Get("X-From-Cache") == "1"}
}
