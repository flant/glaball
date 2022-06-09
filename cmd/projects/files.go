package projects

import (
	"fmt"
	"os"
	"regexp"
	"text/tabwriter"

	"github.com/flant/gitlaball/pkg/client"
	"github.com/flant/gitlaball/pkg/limiter"
	"github.com/flant/gitlaball/pkg/sort"

	"github.com/flant/gitlaball/cmd/common"

	"github.com/hashicorp/go-hclog"
	"github.com/spf13/cobra"
	"github.com/xanzy/go-gitlab"
)

var (
	listProjectsFilesOptions = gitlab.ListProjectsOptions{ListOptions: gitlab.ListOptions{PerPage: 100}}

	filepaths []string
	patterns  []string

	gitRef string
)

func NewFilesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "files",
		Short: "Repository files",
	}
	cmd.AddCommand(
		NewSearchCmd(),
	)

	return cmd
}

func NewSearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search repository files content",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Search()
		},
	}

	cmd.Flags().StringSliceVar(&filepaths, "filepath", []string{}, "List of project files to search for pattern")
	cmd.MarkFlagRequired("filepath")

	cmd.Flags().StringSliceVar(&patterns, "pattern", []string{".*"}, "List of regex patterns to search in files")
	cmd.Flags().StringVar(&gitRef, "ref", "", "Git branch to search file in. Default branch if no value provided")

	// ListProjectsOptions
	listProjectsOptionsFlags(cmd, &listProjectsFilesOptions)

	return cmd
}

func Search() error {
	re := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		r, err := regexp.Compile(p)
		if err != nil {
			return err
		}
		re = append(re, r)
	}

	wg := common.Limiter
	data := make(chan interface{})

	for _, h := range common.Client.Hosts {
		fmt.Printf("Searching for files in %s ...\n", h.URL)
		// TODO: context with cancel
		for _, fp := range filepaths {
			wg.Add(1)
			go listProjectsFiles(h, fp, gitRef, re, listProjectsFilesOptions, wg, data, common.Client.WithCache())
		}
	}

	go func() {
		wg.Wait()
		close(data)
	}()

	results := sort.FromChannel(data, &sort.Options{
		OrderBy:    []string{"web_url"},
		SortBy:     "desc",
		GroupBy:    "",
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

	for _, err := range wg.Errors() {
		hclog.L().Error(err.Err.Error())
	}

	w.Flush()

	return nil
}

func SearchRegexp() error {
	// do not allow to list project's tree for more than 1 host
	if len(common.Client.Hosts) > 1 {
		return fmt.Errorf("you don't want to use it as bulk function")
	}

	re := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		r, err := regexp.Compile(p)
		if err != nil {
			return err
		}
		re = append(re, r)
	}

	wg := common.Limiter
	data := make(chan interface{})

	for _, h := range common.Client.Hosts {
		fmt.Printf("Searching for files in %s ...\n", h.URL)
		wg.Add(1)
		go listProjectsFilesRegexp(h, gitRef, re, listProjectsFilesOptions, wg, data, common.Client.WithCache())
	}

	go func() {
		wg.Wait()
		close(data)
	}()

	results := sort.FromChannel(data, &sort.Options{
		OrderBy:    []string{"web_url"},
		SortBy:     "desc",
		GroupBy:    "",
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

func listProjectsFiles(h *client.Host, filepath, ref string, re []*regexp.Regexp, opt gitlab.ListProjectsOptions,
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
		targetRef := ref
		if ref == "" {
			targetRef = v.DefaultBranch
		}
		// TODO: handle deadlock when no files found
		go getRawFile(h, v, filepath, ref, re, gitlab.GetRawFileOptions{Ref: &targetRef}, wg, data, options...)
	}

	if resp.NextPage > 0 {
		wg.Add(1)
		opt.Page = resp.NextPage
		go listProjectsFiles(h, filepath, ref, re, opt, wg, data, options...)
	}
}

func getRawFile(h *client.Host, project *gitlab.Project, filepath, ref string, re []*regexp.Regexp, opt gitlab.GetRawFileOptions,
	wg *limiter.Limiter, data chan<- interface{}, options ...gitlab.RequestOptionFunc) {

	defer wg.Done()

	wg.Lock()
	raw, resp, err := h.Client.RepositoryFiles.GetRawFile(project.ID, filepath, &opt, options...)
	if err != nil {
		hclog.L().Named("files").Trace("get raw file error", "project", project.WebURL, "error", err)
		wg.Unlock()
		return
	}
	wg.Unlock()

	for _, r := range re {
		if r.Match(raw) {
			data <- sort.Element{Host: h, Struct: project, Cached: resp.Header.Get("X-From-Cache") == "1"}
			hclog.L().Named("files").Trace("search pattern was found in file", "team", h.Team, "project", h.Project, "host", h.URL,
				"repo", project.WebURL, "file", filepath, "pattern", r.String(), "content", hclog.Fmt("%s", raw))
			return
		}
	}
}

func listProjectsFilesRegexp(h *client.Host, ref string, re []*regexp.Regexp, opt gitlab.ListProjectsOptions,
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
		// context
		wg.Add(1)
		targetRef := ref
		if ref == "" {
			targetRef = v.DefaultBranch
		}
		go listTree(h, v, re, gitlab.ListTreeOptions{ListOptions: opt.ListOptions, Ref: &targetRef, Recursive: gitlab.Bool(true)},
			wg, data, options...)
	}

	if resp.NextPage > 0 {
		wg.Add(1)
		opt.Page = resp.NextPage
		go listProjectsFilesRegexp(h, ref, re, opt, wg, data, options...)
	}
}

func listTree(h *client.Host, project *gitlab.Project, re []*regexp.Regexp, opt gitlab.ListTreeOptions,
	wg *limiter.Limiter, data chan<- interface{}, options ...gitlab.RequestOptionFunc) {

	defer wg.Done()

	wg.Lock()
	list, resp, err := h.Client.Repositories.ListTree(project.ID, &opt, options...)
	if err != nil {
		wg.Unlock()
		return
	}
	wg.Unlock()

	for _, v := range list {
		if v.Type == "blob" {
			for _, r := range re {
				if r.MatchString(v.Path) {
					wg.Add(1)
					go rawBlobContent(h, project, re, v.ID, wg, data, options...)
					return
				}
			}
		}
	}

	if resp.NextPage > 0 {
		wg.Add(1)
		opt.Page = resp.NextPage
		go listTree(h, project, re, opt, wg, data, options...)
	}

}

func rawBlobContent(h *client.Host, project *gitlab.Project, re []*regexp.Regexp, sha string,
	wg *limiter.Limiter, data chan<- interface{}, options ...gitlab.RequestOptionFunc) {

	defer wg.Done()

	wg.Lock()
	raw, resp, err := h.Client.Repositories.RawBlobContent(project.ID, sha, options...)
	if err != nil {
		wg.Error(h, err)
		wg.Unlock()
		return
	}
	wg.Unlock()

	for _, r := range re {
		if r.Match(raw) {
			data <- sort.Element{Host: h, Struct: project, Cached: resp.Header.Get("X-From-Cache") == "1"}
			return
		}
	}

}

func ListProjectsFiles(h *client.Host, filepath, ref string, re []*regexp.Regexp, opt gitlab.ListProjectsOptions,
	wg *limiter.Limiter, data chan<- interface{}, options ...gitlab.RequestOptionFunc) {
	listProjectsFiles(h, filepath, ref, re, opt, wg, data, options...)
}
