package projects

import (
	"fmt"
	"os"
	"regexp"
	go_sort "sort"
	"strings"
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
	listProjectsPipelinesOptions = gitlab.ListProjectsOptions{ListOptions: gitlab.ListOptions{PerPage: 100}}
	active                       *bool
	status                       *string
	schedulesDescriptions        []string
	cleanupFilepaths             []string
	cleanupPatterns              []string
	cleanupDescriptions          []string
)

func NewPipelinesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pipelines",
		Short: "Pipelines API",
	}

	cmd.AddCommand(
		NewPipelineSchedulesCmd(),
		NewPipelineCleanupSchedulesCmd(),
	)

	return cmd
}

func NewPipelineSchedulesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schedules",
		Short: "Pipeline schedules API",
		Long:  "Get a list of the pipeline schedules of a project.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return ListPipelineSchedulesCmd()
		},
	}

	cmd.Flags().Var(util.NewBoolPtrValue(&active), "active",
		"Filter pipeline schedules by state --active=[true|false]. Default nil.")
	cmd.Flags().Var(util.NewEnumPtrValue(&status, "created", "waiting_for_resource", "preparing", "pending", "running", "success", "failed", "canceled", "skipped", "manual", "scheduled"), "status",
		"Filter werf cleanup schedules by status --status=[created, waiting_for_resource, preparing, pending, running, success, failed, canceled, skipped, manual, scheduled]. Default nil.")
	cmd.Flags().StringSliceVar(&schedulesDescriptions, "description", []string{".*"},
		"List of regex patterns to search in pipelines schedules descriptions")

	// ListProjectsOptions
	listProjectsOptionsFlags(cmd, &listProjectsPipelinesOptions)

	return cmd
}

func NewPipelineCleanupSchedulesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cleanups",
		Short: "Cleanup schedules API",
		Long:  "Get a list of werf cleanup schedules of a project. https://werf.io",
		RunE: func(cmd *cobra.Command, args []string) error {
			return ListPipelineCleanupSchedulesCmd()
		},
	}

	cmd.Flags().Var(util.NewBoolPtrValue(&active), "active",
		"Filter pipeline schedules by state --active=[true|false]. Default nil.")
	cmd.Flags().Var(util.NewEnumPtrValue(&status, "created", "waiting_for_resource", "preparing", "pending", "running", "success", "failed", "canceled", "skipped", "manual", "scheduled"), "status",
		"Filter werf cleanup schedules by status --status=[created, waiting_for_resource, preparing, pending, running, success, failed, canceled, skipped, manual, scheduled]. Default nil.")
	cmd.Flags().StringSliceVar(&cleanupFilepaths, "filepath", []string{"werf.yaml", "werf.yml"},
		"List of project files to search for pattern")
	cmd.Flags().StringVar(&gitRef, "ref", "", "Git branch to search file in. Default branch if no value provided")
	cmd.Flags().StringSliceVar(&cleanupPatterns, "pattern", []string{"image"},
		"List of regex patterns to search in files")
	cmd.Flags().StringSliceVar(&cleanupDescriptions, "description", []string{"(?i)cleanup"},
		"List of regex patterns to search in pipelines schedules descriptions")

	// ListProjectsOptions
	listProjectsOptionsFlags(cmd, &listProjectsPipelinesOptions)

	return cmd
}

func ListPipelineSchedulesCmd() error {
	desc := make([]*regexp.Regexp, 0, len(schedulesDescriptions))
	for _, p := range schedulesDescriptions {
		r, err := regexp.Compile(p)
		if err != nil {
			return err
		}
		desc = append(desc, r)
	}

	wg := common.Limiter
	data := make(chan interface{})

	for _, h := range common.Client.Hosts {
		fmt.Printf("Fetching projects pipeline schedules from %s ...\n", h.URL)
		// TODO: context with cancel
		wg.Add(1)
		go listProjectsPipelines(h, listProjectsPipelinesOptions, desc, wg, data, common.Client.WithCache())
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
	fmt.Fprintf(w, "COUNT\tREPOSITORY\tSCHEDULES\tHOSTS\tCACHED\n")
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
		hclog.L().Error(err.Err.Error())
	}

	return nil
}

func ListPipelineCleanupSchedulesCmd() error {
	re := make([]*regexp.Regexp, 0, len(cleanupPatterns))
	for _, p := range cleanupPatterns {
		r, err := regexp.Compile(p)
		if err != nil {
			return err
		}
		re = append(re, r)
	}

	desc := make([]*regexp.Regexp, 0, len(cleanupDescriptions))
	for _, p := range cleanupDescriptions {
		r, err := regexp.Compile(p)
		if err != nil {
			return err
		}
		desc = append(desc, r)
	}

	// only active projects
	listProjectsPipelinesOptions.Archived = gitlab.Bool(false)

	wg := common.Limiter
	data := make(chan interface{})

	for _, h := range common.Client.Hosts {
		fmt.Printf("Searching for cleanups in %s ...\n", h.URL)
		// TODO: context with cancel
		for _, fp := range cleanupFilepaths {
			wg.Add(1)
			// files.go
			go listProjectsFiles(h, fp, gitRef, re, listProjectsPipelinesOptions, wg, data, common.Client.WithCache())
		}
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
		return fmt.Errorf("%s files or patterns %s were not found in any project", cleanupFilepaths, cleanupPatterns)
	}

	schedules := make(chan interface{})
	for _, v := range toList.Typed() {
		wg.Add(1)
		go listPipelineSchedules(v.Host, v.Struct.(*gitlab.Project), gitlab.ListPipelineSchedulesOptions{PerPage: 100},
			desc, wg, schedules, common.Client.WithCache())
	}

	go func() {
		wg.Wait()
		close(schedules)
	}()

	var results []sort.Result
	query := sort.FromChannelQuery(schedules, &sort.Options{
		OrderBy:    []string{"project.web_url"},
		StructType: ProjectPipelineSchedule{},
	})

	query.ToSlice(&results)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.TabIndent)
	fmt.Fprintf(w, "COUNT\tREPOSITORY\tSCHEDULES\tHOSTS\tCACHED\n")
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
				continue
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
		hclog.L().Error(err.Err.Error())
	}

	return nil
}

func listProjectsPipelines(h *client.Host, opt gitlab.ListProjectsOptions, desc []*regexp.Regexp,
	wg *limiter.Limiter, data chan<- interface{}, options ...gitlab.RequestOptionFunc) {

	defer wg.Done()

	wg.Lock()
	list, resp, err := h.Client.Projects.ListProjects(&opt, options...)
	if err != nil {
		wg.Error(h, err)
		wg.Unlock()
		return
	}
	wg.Unlock()

	for _, v := range list {
		wg.Add(1)
		go listPipelineSchedules(h, v, gitlab.ListPipelineSchedulesOptions{PerPage: 100}, desc, wg, data, options...)
	}

	if resp.NextPage > 0 {
		wg.Add(1)
		opt.Page = resp.NextPage
		go listProjectsPipelines(h, opt, desc, wg, data, options...)
	}
}

func listPipelineSchedules(h *client.Host, project *gitlab.Project, opt gitlab.ListPipelineSchedulesOptions,
	desc []*regexp.Regexp, wg *limiter.Limiter, data chan<- interface{}, options ...gitlab.RequestOptionFunc) {

	defer wg.Done()

	wg.Lock()
	list, resp, err := h.Client.PipelineSchedules.ListPipelineSchedules(project.ID, &opt, options...)
	if err != nil {
		wg.Error(h, err)
		wg.Unlock()
		return
	}
	wg.Unlock()

	// filter schedules by matching descriptions if any
	filteredList := make([]*gitlab.PipelineSchedule, 0, len(list))
filter:
	for _, v := range list {
		for _, p := range desc {
			if p.MatchString(v.Description) {
				filteredList = append(filteredList, v)
				continue filter
			}
		}
	}

	if len(filteredList) == 0 {
		// if no schedules were found and no --active flag value was provided
		// return project with nil schedule
		if active == nil && status == nil {
			data <- sort.Element{
				Host:   h,
				Struct: ProjectPipelineSchedule{project, nil},
				Cached: resp.Header.Get("X-From-Cache") == "1"}
		}
	} else {
		for _, v := range filteredList {
			// get entire pipeline schedule to make lastpipeline struct accessible
			// note: init new variables with the same names
			v, resp, err := h.Client.PipelineSchedules.GetPipelineSchedule(project.ID, v.ID, options...)
			if err != nil {
				wg.Error(h, err)
				continue
			}
			// check pipeline schedule state
			if active != nil && v.Active != *active {
				continue
			}
			// check last pipeline status
			// ignore schedules with empty status if defined
			if status != nil && (v.LastPipeline.Status == "" || v.LastPipeline.Status != *status) {
				continue
			}
			// push result to channel
			data <- sort.Element{
				Host:   h,
				Struct: ProjectPipelineSchedule{project, v},
				Cached: resp.Header.Get("X-From-Cache") == "1"}
		}
	}

	if resp.NextPage > 0 {
		wg.Add(1)
		opt.Page = resp.NextPage
		go listPipelineSchedules(h, project, opt, desc, wg, data, options...)
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
		s = append(s, fmt.Sprintf("%s: %q (%s)", v.LastPipeline.Status, v.Description, active))
	}

	go_sort.Strings(s)

	return strings.Join(s, ", ")
}

func ListPipelineSchedules(h *client.Host, project *gitlab.Project, opt gitlab.ListPipelineSchedulesOptions,
	desc []*regexp.Regexp, wg *limiter.Limiter, data chan<- interface{}, options ...gitlab.RequestOptionFunc) {
	listPipelineSchedules(h, project, opt, desc, wg, data, options...)
}
