package projects

import (
	"fmt"
	"os"
	go_sort "sort"
	"strings"
	"text/tabwriter"

	"github.com/flant/gitlaball/cmd/common"
	"github.com/flant/gitlaball/pkg/client"
	"github.com/flant/gitlaball/pkg/limiter"
	"github.com/flant/gitlaball/pkg/sort"
	"github.com/flant/gitlaball/pkg/util"

	"github.com/hashicorp/go-hclog"
	"github.com/spf13/cobra"
	"github.com/xanzy/go-gitlab"
)

var (
	listProjectsPipelinesOptions = gitlab.ListProjectsOptions{ListOptions: gitlab.ListOptions{PerPage: 100}}
	active                       *bool
)

func NewPipelinesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pipelines",
		Short: "Pipelines API",
	}

	cmd.AddCommand(
		NewPipelineSchedulesCmd(),
	)

	return cmd
}

func NewPipelineSchedulesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schedules",
		Short: "Pipeline schedules API",
		Long:  "Get a list of the pipeline schedules of a project.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return ListPipelineSchedules()
		},
	}

	cmd.Flags().Var(util.NewBoolPtrValue(&active), "active",
		"Filter pipeline schedules by state --active=[true|false]. Default nil.")

	// ListProjectsOptions
	listProjectsOptionsFlags(cmd, &listProjectsPipelinesOptions)

	return cmd
}

func ListPipelineSchedules() error {
	cli, err := common.Client()
	if err != nil {
		return err
	}

	wg := cli.Limiter()
	data := make(chan interface{})

	for _, h := range cli.Hosts {
		fmt.Printf("Fetching projects pipeline schedules from %s ...\n", h.URL)
		// TODO: context with cancel
		wg.Add(1)
		go listProjectsPipelines(h, listProjectsPipelinesOptions, wg, data, cli.WithCache())
	}

	go func() {
		wg.Wait()
		close(data)
	}()

	var results []sort.Result
	query := sort.FromChannelQuery(data, &sort.Options{
		OrderBy:    []string{"project.web_url"},
		StructType: ProjectPipelineSchedule{},
	})

	query.ToSlice(&results)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.TabIndent)
	unique := 0
	total := 0

	for _, r := range results {
		unique++         // todo
		total += r.Count //todo
		schedules := make(Schedules, 0, len(r.Elements))
		for _, v := range r.Elements.Typed() {
			sched := v.Struct.(ProjectPipelineSchedule).Schedule
			if sched != nil {
				schedules = append(schedules, sched)
			}
		}
		fmt.Fprintf(w, "[%d]\t%s\t[%s]\t%s\t[%s]\n",
			len(schedules),
			r.Key,
			schedules.Descriptions(),
			r.Elements.Hosts().Projects(),
			r.Cached)
	}

	fmt.Fprintf(w, "Unique: %d\nTotal: %d\nErrors: %d\n", unique, total, len(wg.Errors()))

	w.Flush()

	for _, err := range wg.Errors() {
		hclog.L().Error(err.Error())
	}

	return nil
}

func listProjectsPipelines(h *client.Host, opt gitlab.ListProjectsOptions,
	wg *limiter.Limiter, data chan<- interface{}, options ...gitlab.RequestOptionFunc) {

	defer wg.Done()

	wg.Lock()
	list, resp, err := h.Client.Projects.ListProjects(&opt, options...)
	if err != nil {
		wg.Error(err)
		wg.Unlock()
		return
	}
	wg.Unlock()

	for _, v := range list {
		wg.Add(1)
		go listPipelineSchedules(h, v, gitlab.ListPipelineSchedulesOptions{PerPage: 100}, wg, data, options...)
	}

	if resp.NextPage > 0 {
		wg.Add(1)
		opt.Page = resp.NextPage
		go listProjectsPipelines(h, opt, wg, data, options...)
	}
}

func listPipelineSchedules(h *client.Host, project *gitlab.Project, opt gitlab.ListPipelineSchedulesOptions,
	wg *limiter.Limiter, data chan<- interface{}, options ...gitlab.RequestOptionFunc) {

	defer wg.Done()

	wg.Lock()
	list, resp, err := h.Client.PipelineSchedules.ListPipelineSchedules(project.ID, &opt, options...)
	if err != nil {
		wg.Error(err)
		wg.Unlock()
		return
	}
	wg.Unlock()

	if len(list) == 0 {
		if active == nil {
			data <- sort.Element{
				Host:   h,
				Struct: ProjectPipelineSchedule{project, nil},
				Cached: resp.Header.Get("X-From-Cache") == "1"}
		}
	} else {
		for _, v := range list {
			if active != nil && v.Active != *active {
				continue
			}

			data <- sort.Element{
				Host:   h,
				Struct: ProjectPipelineSchedule{project, v},
				Cached: resp.Header.Get("X-From-Cache") == "1"}
		}
	}

	if resp.NextPage > 0 {
		wg.Add(1)
		opt.Page = resp.NextPage
		go listPipelineSchedules(h, project, opt, wg, data, options...)
	}
}

type ProjectPipelineSchedule struct {
	Project  *gitlab.Project          `json:"project,omitempty"`
	Schedule *gitlab.PipelineSchedule `json:"schedule,omitempty"`
}

type Schedules []*gitlab.PipelineSchedule

func (a Schedules) Descriptions() string {
	if len(a) == 0 {
		return "-"
	}

	s := make([]string, 0, len(a))
	for _, v := range a {
		active := "inactive"
		if v.Active {
			active = "active"
		}
		s = append(s, fmt.Sprintf("%s: %q", active, v.Description))
	}

	go_sort.Strings(s)

	return strings.Join(s, ", ")
}
