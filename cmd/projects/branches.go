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
	"github.com/hashicorp/go-hclog"
	"github.com/spf13/cobra"
	"github.com/xanzy/go-gitlab"
)

const (
	protectedBranchDefaultField = "project.web_url"
)

var (
	listProtectedBranchesOptions     = gitlab.ListProtectedBranchesOptions{PerPage: 100}
	protectRepositoryBranchesOptions = gitlab.ProtectRepositoryBranchesOptions{}
	protectedBranchOrderBy           []string
	protectedBranchFormat            = util.Dict{
		{
			Key:   "COUNT",
			Value: "[%d]",
		},
		{
			Key:   "REPOSITORY",
			Value: "%s",
		},
		{
			Key:   "BRANCH",
			Value: "%s",
		},
		{
			Key:   "HOST",
			Value: "%s",
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
		NewProtectedBranchesCmd(),
	)

	return cmd
}

func NewProtectedBranchesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "protected",
		Short: "Protected branches API",
	}

	cmd.AddCommand(
		NewProtectedBranchesListCmd(),
		NewProtectRepositoryBranchesCmd(),
	)

	return cmd
}

func NewProtectedBranchesListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List protected branches",
		Long:  "Gets a list of protected branches from a project as they are defined in the UI. If a wildcard is set, it is returned instead of the exact name of the branches that match that wildcard.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return ProtectedBranchesListCmd()
		},
	}

	cmd.Flags().Var(util.NewEnumValue(&sortBy, "asc", "desc"), "sort",
		"Return protected branches sorted in asc or desc order. Default is desc")

	cmd.Flags().StringSliceVar(&protectedBranchOrderBy, "order_by", []string{"count", protectedBranchDefaultField},
		`Return protected branches ordered by web_url, created_at, title, updated_at or any nested field. Default is web_url.`)

	listProjectsOptionsFlags(cmd, &listProjectsOptions)

	return cmd
}

func NewProtectRepositoryBranchesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "protect",
		Short: "Protect repository branches",
		Long:  "Protects a single repository branch or several project repository branches using a wildcard protected branch.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return ProtectRepositoryBranchesCmd()
		},
	}

	cmd.Flags().Var(util.NewStringPtrValue(&protectRepositoryBranchesOptions.Name), "name",
		"The name of the branch or wildcard")

	cmd.Flags().Var(util.NewAccessLevelValue(&protectRepositoryBranchesOptions.PushAccessLevel), "push_access_level",
		"Access levels allowed to push (defaults: 40, Maintainer role)")

	cmd.Flags().Var(util.NewAccessLevelValue(&protectRepositoryBranchesOptions.MergeAccessLevel), "merge_access_level",
		"Access levels allowed to merge (defaults: 40, Maintainer role)")

	cmd.Flags().Var(util.NewAccessLevelValue(&protectRepositoryBranchesOptions.UnprotectAccessLevel), "unprotect_access_level",
		"Access levels allowed to unprotect (defaults: 40, Maintainer role)")

	cmd.Flags().Var(util.NewBoolPtrValue(&protectRepositoryBranchesOptions.AllowForcePush), "allow_force_push",
		"Allow all users with push access to force push. (default: false)")

	// cmd.Flags().Var(util.NewBoolPtrValue(&protectRepositoryBranchesOptions.AllowedToPush), "allowed_to_push",
	// 	"Array of access levels allowed to push, with each described by a hash of the form {user_id: integer}, {group_id: integer}, or {access_level: integer}")

	// cmd.Flags().Var(util.NewBoolPtrValue(&protectRepositoryBranchesOptions.AllowedToMerge), "allowed_to_merge",
	// 	"Array of access levels allowed to merge, with each described by a hash of the form {user_id: integer}, {group_id: integer}, or {access_level: integer}")

	// cmd.Flags().Var(util.NewBoolPtrValue(&protectRepositoryBranchesOptions.AllowedToUnprotect), "allowed_to_unprotect",
	// 	"Array of access levels allowed to unprotect, with each described by a hash of the form {user_id: integer}, {group_id: integer}, or {access_level: integer}")

	cmd.Flags().Var(util.NewBoolPtrValue(&protectRepositoryBranchesOptions.CodeOwnerApprovalRequired), "code_owner_approval_required",
		"Prevent pushes to this branch if it matches an item in the CODEOWNERS file. (defaults: false)")

	cmd.MarkFlagRequired("name")

	listProjectsOptionsFlags(cmd, &listProjectsOptions)

	return cmd
}

type ProjectProtectedBranch struct {
	Project         *gitlab.Project         `json:"project,omitempty"`
	ProtectedBranch *gitlab.ProtectedBranch `json:"protected_branch,omitempty"`
}

func ProtectedBranchesListCmd() error {
	if !sort.ValidOrderBy(protectedBranchOrderBy, ProjectProtectedBranch{}) {
		protectedBranchOrderBy = append(protectedBranchOrderBy, protectedBranchDefaultField)
	}

	wg := common.Limiter
	data := make(chan interface{})

	for _, h := range common.Client.Hosts {
		fmt.Printf("Getting protected branches from %s ...\n", h.URL)
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

	protectedBranches := make(chan interface{})
	for _, v := range toList.Typed() {
		wg.Add(1)
		go listProtectedBranches(v.Host, v.Struct.(*gitlab.Project), listProtectedBranchesOptions, wg, protectedBranches, common.Client.WithCache())
	}

	go func() {
		wg.Wait()
		close(protectedBranches)
	}()

	results, err := sort.FromChannel(protectedBranches, &sort.Options{
		OrderBy:    protectedBranchOrderBy,
		SortBy:     sortBy,
		GroupBy:    protectedBranchDefaultField,
		StructType: ProjectProtectedBranch{},
	})
	if err != nil {
		return err
	}

	if len(results) == 0 {
		return fmt.Errorf("no protected branches found")
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.TabIndent)
	if _, err := fmt.Fprintln(w, strings.Join(protectedBranchFormat.Keys(), "\t")); err != nil {
		return err
	}

	unique := 0
	total := 0

	for _, r := range results {
		unique++
		total += r.Count
		branches := make([]string, 0)
		hosts := make([]string, 0)

		for _, v := range r.Elements.Typed() {
			if b := v.Struct.(*ProjectProtectedBranch).ProtectedBranch; b != nil {
				branches = util.InsertString(branches, b.Name)
			}
			if s := v.Host.ProjectName(); !util.ContainsString(hosts, s) {
				hosts = append(hosts, s)
			}
		}

		if err := protectedBranchFormat.Print(w, "\t",
			r.Count,
			r.Key,
			branches,
			hosts,
			r.Cached,
		); err != nil {
			return err
		}
	}

	if err := totalFormat.Print(w, "\n", unique, total, len(wg.Errors())); err != nil {
		return err
	}

	if err := w.Flush(); err != nil {
		return err
	}

	for _, err := range wg.Errors() {
		hclog.L().Error(err.Err.Error())
	}

	return nil
}

func ProtectRepositoryBranchesCmd() error {
	if !sort.ValidOrderBy(protectedBranchOrderBy, ProjectProtectedBranch{}) {
		protectedBranchOrderBy = append(protectedBranchOrderBy, protectedBranchDefaultField)
	}

	wg := common.Limiter
	data := make(chan interface{})

	for _, h := range common.Client.Hosts {
		fmt.Printf("Getting protected branches from %s ...\n", h.URL)
		wg.Add(1)
		go listProjects(h, listProjectsOptions, wg, data, common.Client.WithCache())
	}

	go func() {
		wg.Wait()
		close(data)
	}()

	toProtect := make(sort.Elements, 0)
	for e := range data {
		toProtect = append(toProtect, e)
	}

	if len(toProtect) == 0 {
		return fmt.Errorf("no projects found")
	}

	util.AskUser(fmt.Sprintf("Do you really want to protect branch %q in %d repositories in %v ?",
		*protectRepositoryBranchesOptions.Name, len(toProtect), common.Client.Hosts.Projects(common.Config.ShowAll)))

	protectedBranches := make(chan interface{})
	for _, v := range toProtect.Typed() {
		wg.Add(1)
		go protectRepositoryBranches(v.Host, v.Struct.(*gitlab.Project), protectRepositoryBranchesOptions, wg, protectedBranches, common.Client.WithNoCache())
	}

	go func() {
		wg.Wait()
		close(protectedBranches)
	}()

	results, err := sort.FromChannel(protectedBranches, &sort.Options{
		OrderBy:    protectedBranchOrderBy,
		SortBy:     sortBy,
		GroupBy:    protectedBranchDefaultField,
		StructType: ProjectProtectedBranch{},
	})
	if err != nil {
		return err
	}

	if len(results) == 0 {
		return fmt.Errorf("no protected branches found")
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.TabIndent)
	if _, err := fmt.Fprintln(w, strings.Join(protectedBranchFormat.Keys(), "\t")); err != nil {
		return err
	}

	unique := 0
	total := 0

	for _, r := range results {
		unique++
		total += r.Count
		branches := make([]string, 0)
		hosts := make([]string, 0)

		for _, v := range r.Elements.Typed() {
			if b := v.Struct.(*ProjectProtectedBranch).ProtectedBranch; b != nil {
				branches = util.InsertString(branches, b.Name)
			}
			if s := v.Host.ProjectName(); !util.ContainsString(hosts, s) {
				hosts = append(hosts, s)
			}
		}

		if err := protectedBranchFormat.Print(w, "\t",
			r.Count,
			r.Key,
			branches,
			hosts,
			r.Cached,
		); err != nil {
			return err
		}
	}

	if err := totalFormat.Print(w, "\n", unique, total, len(wg.Errors())); err != nil {
		return err
	}

	if err := w.Flush(); err != nil {
		return err
	}

	for _, err := range wg.Errors() {
		hclog.L().Error(err.Err.Error())
	}

	return nil
}

func listProtectedBranches(h *client.Host, project *gitlab.Project, opt gitlab.ListProtectedBranchesOptions,
	wg *limiter.Limiter, data chan<- interface{}, options ...gitlab.RequestOptionFunc) error {

	defer wg.Done()

	wg.Lock()
	list, resp, err := h.Client.ProtectedBranches.ListProtectedBranches(project.ID, &opt)
	wg.Unlock()
	if err != nil {
		wg.Error(h, err)
		return err
	}

	if len(list) == 0 {
		data <- sort.Element{
			Host: h,
			Struct: &ProjectProtectedBranch{
				Project:         project,
				ProtectedBranch: nil},
			Cached: resp.Header.Get("X-From-Cache") == "1"}
		return nil
	}

	for _, v := range list {
		data <- sort.Element{
			Host: h,
			Struct: &ProjectProtectedBranch{
				Project:         project,
				ProtectedBranch: v},
			Cached: resp.Header.Get("X-From-Cache") == "1"}
	}

	if resp.NextPage > 0 {
		wg.Add(1)
		opt.Page = resp.NextPage
		go listProtectedBranches(h, project, opt, wg, data, options...)
	}

	return nil
}

func protectRepositoryBranches(h *client.Host, project *gitlab.Project, opt gitlab.ProtectRepositoryBranchesOptions,
	wg *limiter.Limiter, data chan<- interface{}, options ...gitlab.RequestOptionFunc) error {

	defer wg.Done()

	wg.Lock()
	v, resp, err := h.Client.ProtectedBranches.ProtectRepositoryBranches(project.ID, &opt)
	wg.Unlock()
	if err != nil {
		wg.Error(h, err)
		return err
	}

	data <- sort.Element{
		Host: h,
		Struct: &ProjectProtectedBranch{
			Project:         project,
			ProtectedBranch: v},
		Cached: resp.Header.Get("X-From-Cache") == "1"}

	return nil
}