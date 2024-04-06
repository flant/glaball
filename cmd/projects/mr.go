package projects

import (
	"encoding/csv"
	"fmt"
	"os"
	"regexp"
	"text/tabwriter"

	"github.com/flant/glaball/pkg/limiter"
	"github.com/flant/glaball/pkg/sort/v2"
	"github.com/flant/glaball/pkg/util"

	"github.com/flant/glaball/cmd/common"

	"github.com/flant/glaball/pkg/client"

	go_sort "sort"

	"github.com/hashicorp/go-hclog"
	"github.com/spf13/cobra"
	"github.com/xanzy/go-gitlab"
)

var (
	listProjectMergeRequestsOptions = gitlab.ListProjectMergeRequestsOptions{ListOptions: gitlab.ListOptions{PerPage: 100}}

	byNamespaces []string
)

func NewMergeRequestsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mr",
		Short: "Merge requests API",
	}

	cmd.AddCommand(
		NewMergeRequestListCmd(),
	)

	return cmd
}

func NewMergeRequestListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List merge requests",
		Long:  "Get all merge requests the authenticated user has access to.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return MergeRequestsListCmd()
		},
	}

	cmd.Flags().StringSliceVar(&byNamespaces, "namespaces", []string{},
		"Limit projects by multiple groups. Default: all projects.")

	cmd.Flags().StringSliceVar(&outputFormat, "output", []string{"table"},
		"Output format: [table csv]. Default: table.")

	cmd.Flags().Var(util.NewEnumValue(&sortBy, "asc", "desc"), "sort",
		"Return merge requests sorted in asc or desc order. Default is desc")

	cmd.Flags().StringSliceVar(&orderBy, "order_by", []string{"count", projectDefaultField},
		`Return requests ordered by web_url, created_at, title, updated_at or any nested field. Default is web_url. https://pkg.go.dev/github.com/xanzy/go-gitlab#MergeRequest`)

	listProjectsOptionsFlags(cmd, &listProjectsOptions)
	listProjectMergeRequestsOptionsFlags(cmd, &listProjectMergeRequestsOptions)

	return cmd
}

func MergeRequestsListCmd() error {
	if !sort.ValidOrderBy(orderBy, gitlab.Project{}) {
		orderBy = append(orderBy, projectDefaultField)
	}

	// sort namespaces in ascending order for fast search
	go_sort.Slice(byNamespaces, func(i, j int) bool {
		return byNamespaces[i] < byNamespaces[j]
	})

	// only active projects
	listProjectsPipelinesOptions.Archived = gitlab.Bool(false)

	// set default value to `opened` otherwise we will wait indefinitely
	if listProjectMergeRequestsOptions.State == nil {
		listProjectMergeRequestsOptions.State = gitlab.String("opened")
	}

	// set default to list all merge requests
	if listProjectMergeRequestsOptions.Scope == nil {
		listProjectMergeRequestsOptions.Scope = gitlab.String("all")
	}

	wg := common.Limiter
	data := make(chan interface{})

	for _, h := range common.Client.Hosts {
		fmt.Printf("Getting merge requests from %s ...\n", h.URL)
		wg.Add(1)
		go listProjectsByNamespace(h, byNamespaces, listProjectsOptions, wg, data, common.Client.WithCache())
	}

	go func() {
		wg.Wait()
		close(data)
	}()

	toList := make(sort.Elements, 0)
	for e := range data {
		toList = append(toList, e)
	}

	if len(toList) == 0 {
		return fmt.Errorf("no projects found")
	}

	mergeRequests := make(chan interface{})
	for _, v := range toList.Typed() {
		wg.Add(1)
		go listMergeRequests(v.Host, v.Struct.(*gitlab.Project), listProjectMergeRequestsOptions, wg, mergeRequests, common.Client.WithCache())
	}

	go func() {
		wg.Wait()
		close(mergeRequests)
	}()

	results, err := sort.FromChannel(mergeRequests, &sort.Options{
		OrderBy:    orderBy,
		SortBy:     sortBy,
		StructType: gitlab.MergeRequest{},
	})
	if err != nil {
		return err
	}

	if len(results) == 0 {
		return fmt.Errorf("no merge requests found")
	}

	if util.ContainsString(outputFormat, "csv") {
		w := csv.NewWriter(os.Stdout)
		w.Write([]string{"HOST", "URL", "Title", "Username", "Last"})
		for _, v := range results {
			for _, elem := range v.Elements.Typed() {
				mr := elem.Struct.(*gitlab.MergeRequest)
				w.Write([]string{elem.Host.Project, mr.WebURL, mr.Title, mr.Author.Username, mr.UpdatedAt.Format("2006-01-02 15:04:05")})
			}
		}
		w.Flush()
	}

	if util.ContainsString(outputFormat, "table") {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.TabIndent)
		fmt.Fprintf(w, "HOST\tTitle\tURL\tAuthor\tLast Updated\n")
		total := 0

		for _, v := range results {
			for _, elem := range v.Elements.Typed() {
				total++
				mr := elem.Struct.(*gitlab.MergeRequest)
				fmt.Fprintf(w, "[%s]\t%s\t%s\t[%s]\t%s\n", elem.Host.Project, mr.Title, mr.WebURL, mr.Author.Username, mr.UpdatedAt.Format("2006-01-02 15:04:05"))
			}
		}

		fmt.Fprintf(w, "Total: %d\nErrors: %d\n", total, len(wg.Errors()))

		w.Flush()
	}

	for _, err := range wg.Errors() {
		hclog.L().Error(err.Err.Error())
	}

	return nil
}

func listProjectsByNamespace(h *client.Host, namespaces []string, opt gitlab.ListProjectsOptions,
	wg *limiter.Limiter, data chan<- interface{}, options ...gitlab.RequestOptionFunc) error {
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
		if len(namespaces) == 0 || util.ContainsString(namespaces, v.Namespace.Name) {
			data <- sort.Element{Host: h, Struct: v, Cached: resp.Header.Get("X-From-Cache") == "1"}
		}
	}

	if resp.NextPage > 0 {
		wg.Add(1)
		opt.Page = resp.NextPage
		go listProjectsByNamespace(h, namespaces, opt, wg, data, options...)
	}

	return nil
}

func ListProjectsByNamespace(h *client.Host, namespaces []string, opt gitlab.ListProjectsOptions,
	wg *limiter.Limiter, data chan<- interface{}, options ...gitlab.RequestOptionFunc) error {
	return listProjectsByNamespace(h, namespaces, opt, wg, data, options...)
}

func listMergeRequests(h *client.Host, project *gitlab.Project, opt gitlab.ListProjectMergeRequestsOptions,
	wg *limiter.Limiter, data chan<- interface{}, options ...gitlab.RequestOptionFunc) error {
	defer wg.Done()

	wg.Lock()
	list, resp, err := h.Client.MergeRequests.ListProjectMergeRequests(project.ID, &opt, options...)
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
		go listMergeRequests(h, project, opt, wg, data, options...)
	}

	return nil
}

func ListMergeRequests(h *client.Host, project *gitlab.Project, opt gitlab.ListProjectMergeRequestsOptions,
	wg *limiter.Limiter, data chan<- interface{}, options ...gitlab.RequestOptionFunc) error {
	return listMergeRequests(h, project, opt, wg, data, options...)
}

func listMergeRequestsByAuthorID(h *client.Host, project *gitlab.Project, authorIDs []int, opt gitlab.ListProjectMergeRequestsOptions,
	wg *limiter.Limiter, data chan<- interface{}, options ...gitlab.RequestOptionFunc) error {
	defer wg.Done()

	wg.Lock()
	list, resp, err := h.Client.MergeRequests.ListProjectMergeRequests(project.ID, &opt, options...)
	if err != nil {
		wg.Error(h, err)
		wg.Unlock()
		return err
	}
	wg.Unlock()

	for _, v := range list {
		if len(authorIDs) == 0 || (v.Author != nil && util.ContainsInt(authorIDs, v.Author.ID)) {
			data <- sort.Element{Host: h, Struct: v, Cached: resp.Header.Get("X-From-Cache") == "1"}
		}
	}

	if resp.NextPage > 0 {
		wg.Add(1)
		opt.Page = resp.NextPage
		go listMergeRequestsByAuthorID(h, project, authorIDs, opt, wg, data, options...)
	}

	return nil
}

// authorIDs slice must be sorted in ascending order
func ListMergeRequestsByAuthorID(h *client.Host, project *gitlab.Project, authorIDs []int, opt gitlab.ListProjectMergeRequestsOptions,
	wg *limiter.Limiter, data chan<- interface{}, options ...gitlab.RequestOptionFunc) error {
	return listMergeRequestsByAuthorID(h, project, authorIDs, opt, wg, data, options...)
}

func listMergeRequestsByAssigneeID(h *client.Host, project *gitlab.Project, assigneeIDs []int, opt gitlab.ListProjectMergeRequestsOptions,
	wg *limiter.Limiter, data chan<- interface{}, options ...gitlab.RequestOptionFunc) error {
	defer wg.Done()

	wg.Lock()
	list, resp, err := h.Client.MergeRequests.ListProjectMergeRequests(project.ID, &opt, options...)
	if err != nil {
		wg.Error(h, err)
		wg.Unlock()
		return err
	}
	wg.Unlock()

	for _, v := range list {
		if len(assigneeIDs) == 0 || (v.Assignee != nil && util.ContainsInt(assigneeIDs, v.Assignee.ID)) {
			data <- sort.Element{Host: h, Struct: v, Cached: resp.Header.Get("X-From-Cache") == "1"}
		}
	}

	if resp.NextPage > 0 {
		wg.Add(1)
		opt.Page = resp.NextPage
		go listMergeRequestsByAssigneeID(h, project, assigneeIDs, opt, wg, data, options...)
	}

	return nil
}

// assigneeIDs slice must be sorted in ascending order
func ListMergeRequestsByAssigneeID(h *client.Host, project *gitlab.Project, assigneeIDs []int, opt gitlab.ListProjectMergeRequestsOptions,
	wg *limiter.Limiter, data chan<- interface{}, options ...gitlab.RequestOptionFunc) error {
	return listMergeRequestsByAssigneeID(h, project, assigneeIDs, opt, wg, data, options...)
}

func listMergeRequestsByAssigneeOrAuthorID(h *client.Host, project *gitlab.Project, IDs []int, opt gitlab.ListProjectMergeRequestsOptions,
	wg *limiter.Limiter, data chan<- interface{}, options ...gitlab.RequestOptionFunc) error {
	defer wg.Done()

	wg.Lock()
	list, resp, err := h.Client.MergeRequests.ListProjectMergeRequests(project.ID, &opt, options...)
	if err != nil {
		wg.Error(h, err)
		wg.Unlock()
		return err
	}
	wg.Unlock()

	for _, v := range list {
		if len(IDs) == 0 {
			data <- sort.Element{Host: h, Struct: v, Cached: resp.Header.Get("X-From-Cache") == "1"}
			continue
		}

		// if mr has assignee, then check and continue
		if v.Assignee != nil {
			if util.ContainsInt(IDs, v.Assignee.ID) {
				data <- sort.Element{Host: h, Struct: v, Cached: resp.Header.Get("X-From-Cache") == "1"}
			}
			continue
		}

		// otherwise check the author
		if v.Author != nil && util.ContainsInt(IDs, v.Author.ID) {
			data <- sort.Element{Host: h, Struct: v, Cached: resp.Header.Get("X-From-Cache") == "1"}
		}
	}

	if resp.NextPage > 0 {
		wg.Add(1)
		opt.Page = resp.NextPage
		go listMergeRequestsByAssigneeOrAuthorID(h, project, IDs, opt, wg, data, options...)
	}

	return nil
}

// authorIDs slice must be sorted in ascending order
func ListMergeRequestsByAuthorOrAssigneeID(h *client.Host, project *gitlab.Project, IDs []int, opt gitlab.ListProjectMergeRequestsOptions,
	wg *limiter.Limiter, data chan<- interface{}, options ...gitlab.RequestOptionFunc) error {
	return listMergeRequestsByAssigneeOrAuthorID(h, project, IDs, opt, wg, data, options...)
}

func listMergeRequestsSearch(h *client.Host, project *gitlab.Project, key string, value *regexp.Regexp, opt gitlab.ListProjectMergeRequestsOptions,
	wg *limiter.Limiter, data chan<- interface{}, options ...gitlab.RequestOptionFunc) error {
	defer wg.Done()

	wg.Lock()
	list, resp, err := h.Client.MergeRequests.ListProjectMergeRequests(project.ID, &opt, options...)
	if err != nil {
		wg.Error(h, err)
		wg.Unlock()
		return err
	}
	wg.Unlock()

	for _, v := range list {
		s, err := sort.ValidFieldValue([]string{key}, v)
		if err != nil {
			wg.Error(h, err)
			return err
		}
		// This will panic if value is not a string
		if value.MatchString(s.(string)) {
			data <- sort.Element{Host: h, Struct: v, Cached: resp.Header.Get("X-From-Cache") == "1"}
		}
	}

	if resp.NextPage > 0 {
		wg.Add(1)
		opt.Page = resp.NextPage
		go listMergeRequestsSearch(h, project, key, value, opt, wg, data, options...)
	}

	return nil
}

func ListMergeRequestsSearch(h *client.Host, project *gitlab.Project, key string, value *regexp.Regexp, opt gitlab.ListProjectMergeRequestsOptions,
	wg *limiter.Limiter, data chan<- interface{}, options ...gitlab.RequestOptionFunc) error {
	return listMergeRequestsSearch(h, project, key, value, opt, wg, data, options...)
}

func listProjectMergeRequestsOptionsFlags(cmd *cobra.Command, opt *gitlab.ListProjectMergeRequestsOptions) {
	cmd.Flags().Var(util.NewEnumPtrValue(&opt.State, "opened", "closed", "locked", "merged", "all"), "state",
		"Return all merge requests or just those that are opened, closed, locked, or merged. Default is opened.")

	cmd.Flags().Var(util.NewStringPtrValue(&opt.Milestone), "milestone",
		"Return merge requests for a specific milestone. None returns merge requests with no milestone. Any returns merge requests that have an assigned milestone.")

	cmd.Flags().Var(util.NewStringPtrValue(&opt.View), "view",
		"If `simple`, returns the iid, URL, title, description, and basic state of merge request.")

	cmd.Flags().Var(util.NewLabelsPtrValue(&opt.Labels), "labels",
		"Return merge requests matching a comma-separated list of labels. None lists all merge requests with no labels. Any lists all merge requests with at least one label. Predefined names are case-insensitive.")

	cmd.Flags().Var(util.NewBoolPtrValue(&opt.WithLabelsDetails), "with_labels_details",
		"If true, response returns more details for each label in labels field: :name, :color, :description, :description_html, :text_color. Default is false. Introduced in GitLab 12.7.")

	cmd.Flags().Var(util.NewBoolPtrValue(&opt.WithMergeStatusRecheck), "with_merge_status_recheck",
		"If true, this projection requests (but does not guarantee) that the merge_status field be recalculated asynchronously. Default is false. Introduced in GitLab 13.0.")

	cmd.Flags().Var(util.NewTimePtrValue(&opt.CreatedAfter), "created_after",
		"Return merge requests created on or after the given time. Expected in ISO 8601 format (2019-03-15T08:00:00Z).")

	cmd.Flags().Var(util.NewTimePtrValue(&opt.CreatedBefore), "created_before",
		"Return merge requests created on or before the given time. Expected in ISO 8601 format (2019-03-15T08:00:00Z).")

	cmd.Flags().Var(util.NewTimePtrValue(&opt.UpdatedAfter), "updated_after",
		"Return merge requests updated on or after the given time. Expected in ISO 8601 format (2019-03-15T08:00:00Z).")

	cmd.Flags().Var(util.NewTimePtrValue(&opt.UpdatedBefore), "updated_before",
		"Return merge requests updated on or before the given time. Expected in ISO 8601 format (2019-03-15T08:00:00Z).")

	cmd.Flags().Var(util.NewStringPtrValue(&opt.Scope), "scope",
		"Return merge requests for the given scope: created_by_me, assigned_to_me or all.")

	cmd.Flags().Var(util.NewIntPtrValue(&opt.AuthorID), "author_id",
		"Returns merge requests created by the given user id. Mutually exclusive with author_username.")

	cmd.Flags().Var(util.NewStringPtrValue(&opt.AuthorUsername), "author_username",
		"Returns merge requests created by the given username. Mutually exclusive with author_id. Introduced in GitLab 12.10.")

	cmd.Flags().Var(util.NewAssigneeIDPtrValue(&opt.AssigneeID), "assignee_id",
		"Returns merge requests assigned to the given user id. None returns unassigned merge requests. Any returns merge requests with an assignee.")

	// TODO:
	// 	cmd.Flags().Var(util.NewVisibilityPtrValue(&opt.ApproverIDs), "approver_ids",
	// 		`Returns merge requests which have specified all the users with the given ids as individual approvers.
	// None returns merge requests without approvers. Any returns merge requests with an approver.
	// Available in GitLab Premium self-managed, GitLab Premium SaaS, and higher tiers.`)

	// TODO:
	// 	cmd.Flags().Var(util.NewBoolPtrValue(&opt.ApprovedByIDs), "approved_by_ids",
	// 		`Returns merge requests which have been approved by all the users with the given ids (Max: 5). None returns merge requests with no approvals.
	// Any returns merge requests with an approval.
	// Available in GitLab Premium self-managed, GitLab Premium SaaS, and higher tiers.`)

	cmd.Flags().Var(util.NewReviewerIDPtrValue(&opt.ReviewerID), "reviewer_id",
		`Returns merge requests which have the user as a reviewer with the given user id. None returns merge requests with no reviewers.
Any returns merge requests with any reviewer. Mutually exclusive with reviewer_username.`)

	cmd.Flags().Var(util.NewStringPtrValue(&opt.ReviewerUsername), "reviewer_username",
		`Returns merge requests which have the user as a reviewer with the given username. None returns merge requests with no reviewers.
Any returns merge requests with any reviewer. Mutually exclusive with reviewer_id. Introduced in GitLab 13.8.`)

	cmd.Flags().Var(util.NewStringPtrValue(&opt.MyReactionEmoji), "my_reaction_emoji",
		`Return merge requests reacted by the authenticated user by the given emoji. None returns issues not given a reaction.
Any returns issues given at least one reaction.`)

	cmd.Flags().Var(util.NewStringPtrValue(&opt.SourceBranch), "source_branch",
		"Return merge requests with the given source branch.")

	cmd.Flags().Var(util.NewStringPtrValue(&opt.TargetBranch), "target_branch",
		"Return merge requests with the given target branch.")

	// TODO: projects command has the same flag
	// cmd.Flags().Var(util.NewStringPtrValue(&opt.Search), "search",
	// 	"Search merge requests against their title and description.")

	// TODO: field is not available in github.com/xanzy/go-gitlab at this time
	// cmd.Flags().Var(util.NewBoolPtrValue(&opt.NonArchived), "non_archived",
	// 	"Return merge requests from non archived projects only. Default is true. (Introduced in GitLab 12.8).")

	// TODO: field is not available in github.com/xanzy/go-gitlab at this time
	// cmd.Flags().Var(util.NewStringPtrValue(&opt.Not), "not",
	// 	"Return merge requests that do not match the parameters supplied. Accepts: labels, milestone, author_id, author_username, assignee_id, assignee_username, reviewer_id, reviewer_username, my_reaction_emoji.")
}
