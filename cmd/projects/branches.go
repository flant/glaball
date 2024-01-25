package projects

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"dario.cat/mergo"
	"github.com/flant/glaball/cmd/common"
	"github.com/flant/glaball/pkg/client"
	"github.com/flant/glaball/pkg/limiter"
	"github.com/flant/glaball/pkg/sort/v2"
	"github.com/flant/glaball/pkg/util"
	"github.com/google/go-github/v58/github"
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

func protectRepositoryBranches(h *client.Host, pb *ProjectProtectedBranch, forceProtect bool, opt gitlab.ProtectRepositoryBranchesOptions,
	wg *limiter.Limiter, data chan<- interface{}, options ...gitlab.RequestOptionFunc) error {

	defer wg.Done()

	if forceProtect {
		if old, ok := pb.Search(*opt.Name); ok {
			new := opt

			new.AllowForcePush = &old.AllowForcePush
			new.CodeOwnerApprovalRequired = &old.CodeOwnerApprovalRequired

			switch n := len(old.MergeAccessLevels); n {
			case 0:
			case 1:
				new.MergeAccessLevel = &old.MergeAccessLevels[0].AccessLevel
			default:
				allowedToMerge := make([]*gitlab.BranchPermissionOptions, 0, n)
				for _, l := range old.MergeAccessLevels {
					allowedToMerge = append(allowedToMerge, &gitlab.BranchPermissionOptions{
						UserID:      &l.UserID,
						GroupID:     &l.GroupID,
						AccessLevel: &l.AccessLevel,
					})
				}
				new.AllowedToMerge = &allowedToMerge
			}

			switch n := len(old.PushAccessLevels); n {
			case 0:
			case 1:
				new.PushAccessLevel = &old.PushAccessLevels[0].AccessLevel
			default:
				allowedToPush := make([]*gitlab.BranchPermissionOptions, 0, n)
				for _, l := range old.PushAccessLevels {
					allowedToPush = append(allowedToPush, &gitlab.BranchPermissionOptions{
						UserID:      &l.UserID,
						GroupID:     &l.GroupID,
						AccessLevel: &l.AccessLevel,
					})
				}
				new.AllowedToPush = &allowedToPush
			}

			switch n := len(old.UnprotectAccessLevels); n {
			case 0:
			case 1:
				new.UnprotectAccessLevel = &old.UnprotectAccessLevels[0].AccessLevel
			default:
				allowedToUnprotect := make([]*gitlab.BranchPermissionOptions, 0, n)
				for _, l := range old.UnprotectAccessLevels {
					allowedToUnprotect = append(allowedToUnprotect, &gitlab.BranchPermissionOptions{
						UserID:      &l.UserID,
						GroupID:     &l.GroupID,
						AccessLevel: &l.AccessLevel,
					})
				}
				new.AllowedToUnprotect = &allowedToUnprotect
			}

			if err := mergo.Merge(&new, opt, mergo.WithOverwriteWithEmptyValue); err != nil {
				wg.Error(h, err)
				return err
			}

			wg.Lock()
			_, err := h.Client.ProtectedBranches.UnprotectRepositoryBranches(pb.Project.ID, *new.Name, options...)
			wg.Unlock()
			if err != nil {
				wg.Error(h, err)
				return err
			}

			opt = new
		}
	}

	wg.Lock()
	v, resp, err := h.Client.ProtectedBranches.ProtectRepositoryBranches(pb.Project.ID, &opt, options...)
	wg.Unlock()
	if err != nil {
		wg.Error(h, err)
		return err
	}

	data <- sort.Element{
		Host: h,
		Struct: &ProjectProtectedBranch{
			Project:           pb.Project,
			ProtectedBranches: []*gitlab.ProtectedBranch{v}},
		Cached: resp.Header.Get("X-From-Cache") == "1"}

	return nil
}

type ProjectBranch struct {
	Project  *gitlab.Project  `json:"project,omitempty"`
	Branches []*gitlab.Branch `json:"branch,omitempty"`
}

type RepositoryBranch struct {
	Repository *github.Repository `json:"repository,omitempty"`
	Branch     *github.Branch     `json:"branch,omitempty"`
}
