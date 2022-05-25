package projects

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
	listProjectsOptions = gitlab.ListProjectsOptions{ListOptions: gitlab.ListOptions{PerPage: 100}}
	groupBy, sortBy     string
	orderBy             []string
)

func NewListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List projects.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return List()
		},
	}

	cmd.Flags().Var(util.NewEnumValue(&groupBy, "name", "path"), "group_by",
		"Return projects grouped by id, name, path, fields.")

	cmd.Flags().Var(util.NewEnumValue(&sortBy, "asc", "desc"), "sort",
		"Return projects sorted in asc or desc order. Default is desc")

	cmd.Flags().StringSliceVar(&orderBy, "order_by", []string{"count", "web_url"},
		`Return projects ordered by id, name, path, created_at, updated_at, last_activity_at, or similarity fields.
repository_size, storage_size, packages_size or wiki_size fields are only allowed for administrators.
similarity (introduced in GitLab 14.1) is only available when searching and is limited to projects that the current user is a member of.`)

	listProjectsOptionsFlags(cmd, &listProjectsOptions)

	return cmd
}

func listProjectsOptionsFlags(cmd *cobra.Command, opt *gitlab.ListProjectsOptions) {
	cmd.Flags().Var(util.NewBoolPtrValue(&opt.Archived), "archived",
		"Limit by archived status. (--archived or --no-archived). Default nil")

	cmd.Flags().Var(util.NewIntPtrValue(&opt.IDAfter), "id_after",
		"Limit results to projects with IDs greater than the specified ID.")

	cmd.Flags().Var(util.NewIntPtrValue(&opt.IDBefore), "id_before",
		"Limit results to projects with IDs less than the specified ID.")

	cmd.Flags().Var(util.NewTimePtrValue(&opt.LastActivityAfter), "last_activity_after",
		"Limit results to projects with last_activity after specified time. Format: ISO 8601 (YYYY-MM_DDTHH:MM:SSZ)")

	cmd.Flags().Var(util.NewTimePtrValue(&opt.LastActivityBefore), "last_activity_before",
		"Limit results to projects with last_activity before specified time. Format: ISO 8601 (YYYY-MM_DDTHH:MM:SSZ)")

	cmd.Flags().Var(util.NewBoolPtrValue(&opt.Membership), "membership",
		"Limit by projects that the current user is a member of.")

	cmd.Flags().Var(util.NewBoolPtrValue(&opt.Owned), "owned",
		"Limit by projects explicitly owned by the current user.")

	cmd.Flags().Var(util.NewBoolPtrValue(&opt.RepositoryChecksumFailed), "repository_checksum_failed",
		`Limit projects where the repository checksum calculation has failed (Introduced in GitLab 11.2).
Available in GitLab Premium self-managed, GitLab Premium SaaS, and higher tiers.`)

	cmd.Flags().Var(util.NewStringPtrValue(&opt.Search), "search",
		"Return list of projects matching the search criteria.")

	cmd.Flags().Var(util.NewBoolPtrValue(&opt.SearchNamespaces), "search_namespaces",
		"Include ancestor namespaces when matching search criteria. Default is false.")

	cmd.Flags().Var(util.NewBoolPtrValue(&opt.Simple), "simple",
		`Return only limited fields for each project.
This is a no-op without authentication as then only simple fields are returned.`)

	cmd.Flags().Var(util.NewBoolPtrValue(&opt.Starred), "starred",
		"Limit by projects starred by the current user.")

	cmd.Flags().Var(util.NewBoolPtrValue(&opt.Statistics), "statistics",
		"Include project statistics. Only available to Reporter or higher level role members.")

	cmd.Flags().Var(util.NewStringPtrValue(&opt.Topic), "topic",
		"Comma-separated topic names. Limit results to projects that match all of given topics. See topics attribute.")

	cmd.Flags().Var(util.NewVisibilityPtrValue(&opt.Visibility), "visibility",
		"Limit by visibility public, internal, or private.")

	cmd.Flags().Var(util.NewBoolPtrValue(&opt.WikiChecksumFailed), "wiki_checksum_failed",
		`Limit projects where the wiki checksum calculation has failed (Introduced in GitLab 11.2).
Available in GitLab Premium self-managed, GitLab Premium SaaS, and higher tiers.`)

	cmd.Flags().Var(util.NewBoolPtrValue(&opt.WithCustomAttributes), "with_custom_attributes",
		"Include custom attributes in response. (administrator only)")

	cmd.Flags().Var(util.NewBoolPtrValue(&opt.WithIssuesEnabled), "with_issues_enabled",
		"Limit by enabled issues feature.")

	cmd.Flags().Var(util.NewBoolPtrValue(&opt.WithMergeRequestsEnabled), "with_merge_requests_enabled",
		"Limit by enabled merge requests feature.")

	cmd.Flags().Var(util.NewStringPtrValue(&opt.WithProgrammingLanguage), "with_programming_language",
		"Limit by projects which use the given programming language.")
}

func List() error {
	wg := common.Limiter
	data := make(chan interface{})

	for _, h := range common.Client.Hosts {
		fmt.Printf("Fetching projects from %s ...\n", h.URL)
		// TODO: context with cancel
		wg.Add(1)
		go listProjects(h, listProjectsOptions, wg, data, common.Client.WithCache())
	}

	go func() {
		wg.Wait()
		close(data)
	}()

	results := sort.FromChannel(data, &sort.Options{
		OrderBy:    orderBy,
		SortBy:     sortBy,
		GroupBy:    groupBy,
		StructType: gitlab.Project{},
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.TabIndent)
	fmt.Fprintf(w, "COUNT\tREPOSITORY\tHOSTS\tCACHED\n")
	unique := 0
	total := 0

	for _, v := range results {
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

func listProjects(h *client.Host, opt gitlab.ListProjectsOptions, wg *limiter.Limiter, data chan<- interface{},
	options ...gitlab.RequestOptionFunc) error {

	defer wg.Done()

	wg.Lock()

	list, resp, err := h.Client.Projects.ListProjects(&opt, options...)
	if err != nil {
		wg.Error(h, err)
		wg.Unlock()
		return err
	}

	wg.Unlock()

	for _, v := range list {
		data <- sort.Element{Host: h, Struct: v, Cached: resp.Header.Get("X-From-Cache") == "1"}
	}

	if resp.NextPage > 0 {
		wg.Add(1)
		opt.Page = resp.NextPage
		go listProjects(h, opt, wg, data, options...)
	}

	return nil
}
