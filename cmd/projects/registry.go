package projects

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/alecthomas/units"
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
	registryRepositoryDefaultField = "project.web_url"
)

var (
	listRegistryRepositoriesOptions = gitlab.ListRegistryRepositoriesOptions{
		ListOptions: gitlab.ListOptions{PerPage: 100},
		TagsCount:   gitlab.Bool(true),
	}
	registryRepositoryTotalSize bool
	registryRepositoryhOrderBy  []string
	registryRepositoriesFormat  = util.Dict{
		{
			Key:   "COUNT",
			Value: "[%d]",
		},
		{
			Key:   "REPOSITORY",
			Value: "%s",
		},
		{
			Key:   "TAGS COUNT",
			Value: "[%d]",
		},
		{
			Key:   "TOTAL SIZE",
			Value: "%s",
		},
		{
			Key:   "HOST",
			Value: "[%s]",
		},
		{
			Key:   "CACHED",
			Value: "[%s]",
		},
	}
)

func NewRegistryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "registry",
		Short: "Container Registry API",
	}

	cmd.AddCommand(
		NewRegistryListCmd(),
	)

	return cmd
}

func NewRegistryListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List registry repositories",
		Long:  "Get a list of registry repositories in a project.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RegistryListCmd()
		},
	}

	cmd.Flags().Var(util.NewEnumValue(&sortBy, "asc", "desc"), "sort",
		"Return protected branches sorted in asc or desc order. Default is desc")

	cmd.Flags().StringSliceVar(&registryRepositoryhOrderBy, "order_by", []string{"count", registryRepositoryDefaultField},
		`Return protected branches ordered by web_url, created_at, title, updated_at or any nested field. Default is web_url.`)

	cmd.Flags().BoolVar(&registryRepositoryTotalSize, "size", false,
		`If the parameter is included as true, the response includes "size". This is the deduplicated size of all images within the repository.`)

	listProjectsOptionsFlags(cmd, &listProjectsOptions)

	return cmd
}

func RegistryListCmd() error {
	if !sort.ValidOrderBy(registryRepositoryhOrderBy, ProjectProtectedBranch{}) {
		registryRepositoryhOrderBy = append(registryRepositoryhOrderBy, registryRepositoryDefaultField)
	}

	if registryRepositoryTotalSize {
		listRegistryRepositoriesOptions.Tags = &registryRepositoryTotalSize
	}

	wg := common.Limiter
	data := make(chan interface{})
	defer func() {
		for _, err := range wg.Errors() {
			hclog.L().Error(err.Err.Error())
		}
	}()

	for _, h := range common.Client.Hosts {
		fmt.Printf("Getting registry repositories from %s ...\n", h.URL)
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

	registryRepositories := make(chan interface{})
	for _, v := range toList.Typed() {
		wg.Add(1)
		go listRegistryRepositories(v.Host, v.Struct.(*gitlab.Project), listRegistryRepositoriesOptions, wg, registryRepositories, common.Client.WithCache())
	}

	go func() {
		wg.Wait()
		close(registryRepositories)
	}()

	if registryRepositoryTotalSize {
		registryRepositoriesList := make(sort.Elements, 0)
		for v := range registryRepositories {
			e := v.(sort.Element)
			rep := e.Struct.(*ProjectRegistryRepository)
			for _, reg := range rep.RegistryRepositories {
				for _, tag := range reg.Tags {
					wg.Add(1)
					go getRegistryRepositoryTagDetail(e.Host, rep.Project, reg, tag, wg, common.Client.WithCache())
				}
			}
			registryRepositoriesList = append(registryRepositoriesList, v)
		}
		wg.Wait()
		registryRepositories = make(chan interface{})
		go func() {
			for _, v := range registryRepositoriesList {
				registryRepositories <- v
			}
			close(registryRepositories)
		}()
	}

	results, err := sort.FromChannel(registryRepositories, &sort.Options{
		OrderBy:    registryRepositoryhOrderBy,
		SortBy:     sortBy,
		GroupBy:    registryRepositoryDefaultField,
		StructType: ProjectRegistryRepository{},
	})
	if err != nil {
		return err
	}

	if len(results) == 0 {
		return fmt.Errorf("no registry repositories found")
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.TabIndent)
	if _, err := fmt.Fprintln(w, strings.Join(registryRepositoriesFormat.Keys(), "\t")); err != nil {
		return err
	}

	unique := 0
	total := 0

	for _, r := range results {
		unique++
		total += r.Count
		for _, v := range r.Elements.Typed() {
			pr := v.Struct.(*ProjectRegistryRepository)
			if err := registryRepositoriesFormat.Print(w, "\t",
				len(pr.RegistryRepositories),
				r.Key,
				pr.TagsCount(),
				units.Base2Bytes(pr.TotalSize()).Floor(),
				v.Host.ProjectName(),
				r.Cached,
			); err != nil {
				return err
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

type ProjectRegistryRepository struct {
	Project              *gitlab.Project              `json:"project,omitempty"`
	RegistryRepositories []*gitlab.RegistryRepository `json:"registry_repositories,omitempty"`
}

func (pr *ProjectRegistryRepository) TagsCount() (i int) {
	for _, v := range pr.RegistryRepositories {
		i += v.TagsCount
	}
	return i
}

func (pr *ProjectRegistryRepository) TotalSize() (i int) {
	if registryRepositoryTotalSize {
		for _, v := range pr.RegistryRepositories {
			for _, t := range v.Tags {
				i += t.TotalSize
			}
		}
	}
	return i
}

func listRegistryRepositories(h *client.Host, project *gitlab.Project, opt gitlab.ListRegistryRepositoriesOptions,
	wg *limiter.Limiter, data chan<- interface{}, options ...gitlab.RequestOptionFunc) {

	defer wg.Done()

	wg.Lock()
	list, resp, err := h.Client.ContainerRegistry.ListProjectRegistryRepositories(project.ID, &opt, options...)
	wg.Unlock()
	if err != nil {
		wg.Error(h, err)
		return
	}

	data <- sort.Element{
		Host: h,
		Struct: &ProjectRegistryRepository{
			Project:              project,
			RegistryRepositories: list},
		Cached: resp.Header.Get("X-From-Cache") == "1"}

	if resp.NextPage > 0 {
		wg.Add(1)
		opt.Page = resp.NextPage
		go listRegistryRepositories(h, project, opt, wg, data, options...)
	}
}

func getRegistryRepositoryTagDetail(h *client.Host, project *gitlab.Project, repository *gitlab.RegistryRepository,
	tag *gitlab.RegistryRepositoryTag, wg *limiter.Limiter, options ...gitlab.RequestOptionFunc) {

	defer wg.Done()

	wg.Lock()
	v, _, err := h.Client.ContainerRegistry.GetRegistryRepositoryTagDetail(project.ID, repository.ID, tag.Name, options...)
	wg.Unlock()
	if err != nil {
		wg.Error(h, err)
		return
	}

	*tag = *v
}
