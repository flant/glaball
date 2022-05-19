package users

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/flant/gitlaball/pkg/client"
	"github.com/flant/gitlaball/pkg/limiter"
	"github.com/flant/gitlaball/pkg/sort"
	"github.com/flant/gitlaball/pkg/util"

	"github.com/flant/gitlaball/cmd/common"

	"github.com/hashicorp/go-hclog"
	"github.com/spf13/cobra"
	"github.com/xanzy/go-gitlab"
)

var (
	listCount       int
	groupBy, sortBy string
	orderBy         []string

	listUsersOptions   = gitlab.ListUsersOptions{ListOptions: gitlab.ListOptions{PerPage: 100}}
	userFieldIndexTree = sort.JsonFieldIndexTree(gitlab.User{})
)

func NewListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List users.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return List()
		},
	}

	cmd.Flags().Var(util.NewEnumValue(&groupBy, "name", "username", "email"), "group_by",
		"Return users grouped by id, name, username, fields.")

	cmd.Flags().Var(util.NewEnumValue(&sortBy, "asc", "desc"), "sort",
		"Return users sorted in asc or desc order. Default is desc")

	//"id", "name", "username", "email", "count"
	cmd.Flags().StringSliceVar(&orderBy, "order_by", []string{"count", "username"},
		"Return users ordered by id, name, username, created_at, or updated_at fields.")

	cmd.Flags().IntVar(&listCount, "count", 1, "Order by count")

	listUsersOptionsFlags(cmd, &listUsersOptions)

	return cmd
}

func listUsersOptionsFlags(cmd *cobra.Command, opt *gitlab.ListUsersOptions) {
	cmd.Flags().Var(util.NewBoolPtrValue(&opt.Active), "active",
		`In addition, you can filter users based on the states blocked and active.
It does not support active=false or blocked=false.
The list of billable users is the total number of users minus the blocked users.`)

	cmd.Flags().Var(util.NewBoolPtrValue(&opt.Blocked), "blocked",
		`In addition, you can filter users based on the states blocked and active.
It does not support active=false or blocked=false.
The list of billable users is the total number of users minus the blocked users.`)

	cmd.Flags().Var(util.NewBoolPtrValue(&opt.ExcludeInternal), "exclude_internal",
		"In addition, you can search for external users only with external=true. It does not support external=false.")

	// The options below are only available for admins.
	cmd.Flags().Var(util.NewStringPtrValue(&opt.Search), "search",
		"You can also search for users by name, username, primary email, or secondary email")

	cmd.Flags().Var(util.NewStringPtrValue(&opt.Username), "username",
		"In addition, you can lookup users by username. Username search is case insensitive")

	cmd.Flags().Var(util.NewStringPtrValue(&opt.ExternalUID), "extern_uid",
		"You can lookup users by external UID and provider")

	cmd.Flags().Var(util.NewStringPtrValue(&opt.Provider), "provider",
		"You can lookup users by external UID and provider")

	cmd.Flags().Var(util.NewTimePtrValue(&opt.CreatedBefore), "created_before",
		"You can search users by creation date time range")

	cmd.Flags().Var(util.NewTimePtrValue(&opt.CreatedAfter), "created_after",
		"You can search users by creation date time range")

	cmd.Flags().Var(util.NewEnumPtrValue(&opt.TwoFactor, "enabled", "disabled"), "two_factor",
		"Filter users by Two-factor authentication. Filter values are enabled or disabled. By default it returns all users")

	cmd.Flags().Var(util.NewBoolPtrValue(&opt.Admins), "admins",
		"Return only admin users. Default is false")

	cmd.Flags().Var(util.NewBoolPtrValue(&opt.External), "external",
		"Flags the user as external - true or false. Default is false")

	cmd.Flags().Var(util.NewBoolPtrValue(&opt.WithoutProjects), "without_projects",
		"Filter users without projects. Default is false, which means that all users are returned, with and without projects")

	cmd.Flags().Var(util.NewBoolPtrValue(&opt.WithCustomAttributes), "with_custom_attributes",
		"You can include the users’ custom attributes in the response. Default is false")
}

func List() error {
	wg := common.Limiter
	data := make(chan interface{})

	for _, h := range common.Client.Hosts {
		fmt.Printf("Fetching users from %s ...\n", h.URL)
		wg.Add(1)
		go listUsers(h, listUsersOptions, wg, data, common.Client.WithCache())
	}

	go func() {
		wg.Wait()
		close(data)
	}()

	results := sort.FromChannel(data, &sort.Options{
		OrderBy:    orderBy,
		SortBy:     sortBy,
		GroupBy:    groupBy,
		StructType: gitlab.User{},
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.TabIndent)
	unique := 0
	total := 0

	for _, v := range results {
		if v.Count < listCount {
			continue
		}

		unique++         // todo
		total += v.Count //todo

		fmt.Fprintf(w, "[%d]\t%s\t%s\t[%s]\n", v.Count, v.Key, v.Elements.Hosts().Projects(), v.Cached)
	}

	fmt.Fprintf(w, "Unique: %d\nTotal: %d\nErrors: %d\n", unique, total, len(wg.Errors()))

	w.Flush()

	for _, err := range wg.Errors() {
		hclog.L().Error(err.Err.Error())
	}

	return nil

}

func listUsers(h *client.Host, opt gitlab.ListUsersOptions,
	wg *limiter.Limiter, data chan<- interface{}, options ...gitlab.RequestOptionFunc) {

	defer wg.Done()

	wg.Lock()
	list, resp, err := h.Client.Users.ListUsers(&opt, options...)
	if err != nil {
		wg.Error(h, err)
		wg.Unlock()
		return
	}
	wg.Unlock()

	for _, v := range list {
		data <- sort.Element{Host: h, Struct: v, Cached: resp.Header.Get("X-From-Cache") == "1"}
	}

	if resp.NextPage > 0 {
		wg.Add(1)
		opt.Page = resp.NextPage
		go listUsers(h, opt, wg, data, options...)
	}
}

func listUsersSearch(h *client.Host, key, value string, opt gitlab.ListUsersOptions,
	wg *limiter.Limiter, data chan<- interface{}, options ...gitlab.RequestOptionFunc) {

	defer wg.Done()

	wg.Lock()
	list, resp, err := h.Client.Users.ListUsers(&opt, options...)
	if err != nil {
		wg.Error(h, err)
		wg.Unlock()
		return
	}
	wg.Unlock()

	for _, v := range list {
		s := sort.ValidFieldValue(userFieldIndexTree, []string{key}, v)
		if s.(string) == value {
			data <- sort.Element{Host: h, Struct: v, Cached: resp.Header.Get("X-From-Cache") == "1"}
			return
		}
	}

	if resp.NextPage > 0 {
		wg.Add(1)
		opt.Page = resp.NextPage
		go listUsersSearch(h, key, value, opt, wg, data, options...)
	}
}
