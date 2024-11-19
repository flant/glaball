package projects

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/flant/glaball/cmd/common"
	"github.com/flant/glaball/pkg/client"
	"github.com/flant/glaball/pkg/limiter"
	"github.com/flant/glaball/pkg/sort/v2"
	"github.com/flant/glaball/pkg/util"
	"github.com/google/go-github/v56/github"
	"github.com/hashicorp/go-hclog"
	"github.com/spf13/cobra"
	"github.com/xanzy/go-gitlab"
)

const (
	branchDefaultField = "project.web_url"
)

var (
	listBranchesOptions = gitlab.ListBranchesOptions{ListOptions: gitlab.ListOptions{PerPage: 100}}
	branchOrderBy       []string
	branchFormat        = util.Dict{
		{
			Key:   "HOST",
			Value: "[%s]",
		},
		{
			Key:   "URL",
			Value: "%s",
		},
		{
			Key:   "LAST UPDATED",
			Value: "[%s]",
		},
		{
			Key:   "CACHED",
			Value: "[%s]",
		},
	}
)

func NewBranchesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "branches",
		Short: "Branches API",
	}

	cmd.AddCommand(
		NewBranchesListCmd(),
		NewProtectedBranchesCmd(),
	)

	return cmd
}

func NewBranchesListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List repository branches",
		Long:  "Get a list of repository branches from a project, sorted by name alphabetically.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return BranchesListCmd()
		},
	}

	cmd.Flags().Var(util.NewEnumValue(&sortBy, "asc", "desc"), "sort",
		"Return branches sorted in asc or desc order. Default is desc")

	cmd.Flags().StringSliceVar(&branchOrderBy, "order_by", []string{"count", branchDefaultField},
		`Return branches ordered by web_url, created_at, title, updated_at or any nested field. Default is web_url.`)

	listProjectsOptionsFlags(cmd, &listProjectsOptions)

	return cmd
}

type ProjectBranch struct {
	Project  *gitlab.Project  `json:"project,omitempty"`
	Branches []*gitlab.Branch `json:"branches,omitempty"`
}

type RepositoryBranch struct {
	Repository *github.Repository `json:"repository,omitempty"`
	Branch     *github.Branch     `json:"branch,omitempty"`
}

func BranchesListCmd() error {
	if !sort.ValidOrderBy(branchOrderBy, ProjectBranch{}) {
		branchOrderBy = append(branchOrderBy, branchDefaultField)
	}

	wg := common.Limiter
	data := make(chan interface{})
	defer func() {
		for _, err := range wg.Errors() {
			hclog.L().Error(err.Err.Error())
		}
	}()

	for _, h := range common.Client.Hosts {
		fmt.Printf("Getting branches from %s ...\n", h.URL)
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

	branches := make(chan interface{})
	for _, v := range toList.Typed() {
		wg.Add(1)
		go listBranches(v.Host, v.Struct.(*gitlab.Project), listBranchesOptions, wg, branches, common.Client.WithCache())
	}

	go func() {
		wg.Wait()
		close(branches)
	}()

	results, err := sort.FromChannel(branches, &sort.Options{
		OrderBy:    branchOrderBy,
		SortBy:     sortBy,
		GroupBy:    branchDefaultField,
		StructType: ProjectBranch{},
	})
	if err != nil {
		return err
	}

	if len(results) == 0 {
		return fmt.Errorf("no branches found")
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.TabIndent)
	if _, err := fmt.Fprintln(w, strings.Join(branchFormat.Keys(), "\t")); err != nil {
		return err
	}

	unique := 0
	total := 0

	for _, r := range results {
		unique++
		total += r.Count
		for _, v := range r.Elements.Typed() {
			pb := v.Struct.(*ProjectBranch)
			for _, b := range pb.Branches {
				if err := branchFormat.Print(w, "\t",
					v.Host.ProjectName(),
					b.WebURL,
					b.Commit.CommittedDate.Format("2006-01-02 15:04:05"),
					v.Cached,
				); err != nil {
					return err
				}
			}

		}

	}

	if err := totalFormat.Print(w, "\n", unique, total, len(wg.Errors())); err != nil {
		return err
	}

	if err := w.Flush(); err != nil {
		return err
	}

	return nil
}

func listBranches(h *client.Host, project *gitlab.Project, opt gitlab.ListBranchesOptions,
	wg *limiter.Limiter, data chan<- interface{}, options ...gitlab.RequestOptionFunc) error {

	defer wg.Done()

	wg.Lock()
	list, resp, err := h.Client.Branches.ListBranches(project.ID, &opt, options...)
	wg.Unlock()
	if err != nil {
		wg.Error(h, err)
		return err
	}

	data <- sort.Element{
		Host:   h,
		Struct: &ProjectBranch{Project: project, Branches: list},
		Cached: resp.Header.Get("X-From-Cache") == "1"}

	if resp.NextPage > 0 {
		wg.Add(1)
		opt.Page = resp.NextPage
		go listBranches(h, project, opt, wg, data, options...)
	}

	return nil
}
