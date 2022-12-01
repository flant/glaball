package projects

import (
	"fmt"
	"os"
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
	editProjectsOptions = gitlab.EditProjectOptions{}
)

func NewEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Edit projects.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Edit()
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
	editProjectsOptionsFlags(cmd, &editProjectsOptions)

	return cmd
}

func editProjectsOptionsFlags(cmd *cobra.Command, opt *gitlab.EditProjectOptions) {
	cmd.Flags().Var(util.NewStringPtrValue(&opt.AutoCancelPendingPipelines), "auto_cancel_pending_pipelines",
		"Auto-cancel pending pipelines. This isn’t a boolean, but enabled/disabled.")

	cmd.Flags().Var(util.NewIntPtrValue(&opt.CIDefaultGitDepth), "ci_default_git_depth",
		"Default number of revisions for shallow cloning.")

	cmd.Flags().Var(util.NewBoolPtrValue(&opt.CIForwardDeploymentEnabled), "ci_forward_deployment_enabled",
		"Enable or disable prevent outdated deployment jobs.")

	cmd.Flags().Var(util.NewStringPtrValue(&opt.DefaultBranch), "default_branch",
		"The default branch name.")
}

func Edit() error {
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

	toList := make(sort.Elements, 0)
	for e := range data {
		toList = append(toList, e)
	}

	if len(toList) == 0 {
		return fmt.Errorf("no projects found")
	}

	projects := make(chan interface{})
	for _, v := range toList.Typed() {
		wg.Add(1)
		go editProject(v.Host, v.Struct.(*gitlab.Project), editProjectsOptions, wg, projects, common.Client.WithCache())
	}

	go func() {
		wg.Wait()
		close(projects)
	}()

	results, err := sort.FromChannel(projects, &sort.Options{
		OrderBy:    orderBy,
		SortBy:     sortBy,
		GroupBy:    groupBy,
		StructType: gitlab.Project{},
	})
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.TabIndent)
	fmt.Fprintf(w, "COUNT\tREPOSITORY\tHOSTS\tCACHED\n")
	unique := 0
	total := 0

	for _, v := range results {
		unique++         // todo
		total += v.Count //todo
		fmt.Fprintf(w, "[%d]\t%s\t%s\t[%s]\n", v.Count, v.Key, v.Elements.Hosts().Projects(common.Config.ShowAll), v.Cached)
	}

	fmt.Fprintf(w, "Unique: %d\nTotal: %d\nErrors: %d\n", unique, total, len(wg.Errors()))

	w.Flush()

	for _, err := range wg.Errors() {
		hclog.L().Error(err.Err.Error())
	}

	return nil
}

func editProject(h *client.Host, project *gitlab.Project, opt gitlab.EditProjectOptions, wg *limiter.Limiter,
	data chan<- interface{}, options ...gitlab.RequestOptionFunc) error {

	defer wg.Done()

	wg.Lock()

	v, resp, err := h.Client.Projects.EditProject(project.ID, &opt, options...)
	if err != nil {
		wg.Error(h, err)
		wg.Unlock()
		return err
	}

	wg.Unlock()

	data <- sort.Element{Host: h, Struct: v, Cached: resp.Header.Get("X-From-Cache") == "1"}

	return nil
}
