package projects

import (
	"fmt"
	"os"
	"regexp"
	"text/tabwriter"

	"github.com/flant/gitlaball/cmd/common"
	"github.com/flant/gitlaball/pkg/client"
	"github.com/flant/gitlaball/pkg/limiter"
	"github.com/flant/gitlaball/pkg/sort"

	"github.com/hashicorp/go-hclog"
	"github.com/spf13/cobra"
	"github.com/xanzy/go-gitlab"
)

var (
	listProjectsFilesOptions = gitlab.ListProjectsOptions{ListOptions: gitlab.ListOptions{PerPage: 100}}

	filepaths []string
	patterns  []string

	ref string
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
	cmd.Flags().StringVar(&ref, "ref", "master", "Git branch to search file in")

	// ListProjectsOptions
	listProjectsOptionsFlags(cmd, &listProjectsFilesOptions)

	return cmd
}

func Search() error {
	cli, err := common.Client()
	if err != nil {
		return err
	}

	re := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		r, err := regexp.Compile(p)
		if err != nil {
			return err
		}
		re = append(re, r)
	}

	wg := cli.Limiter()
	data := make(chan interface{})

	for _, h := range cli.Hosts {
		fmt.Printf("Searching for files in %s ...\n", h.URL)
		// TODO: context with cancel
		for _, fp := range filepaths {
			wg.Add(1)
			go listProjectsFiles(h, fp, ref, re, listProjectsFilesOptions, wg, data, cli.WithCache())
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
	unique := 0
	total := 0

	for _, v := range results {
		unique++         // todo
		total += v.Count //todo
		fmt.Fprintf(w, "[%d]\t%s\t%s\t[%s]\n", v.Count, v.Key, v.Elements.Hosts().Projects(), v.Cached)
	}

	fmt.Fprintf(w, "Unique: %d\nTotal: %d\nErrors: %d\n", unique, total, len(wg.Errors()))

	for _, err := range wg.Errors() {
		hclog.L().Error(err.Error())
	}

	w.Flush()

	return nil
}

func SearchRegexp() error {
	cli, err := common.Client()
	if err != nil {
		return err
	}

	// do not allow to list project's tree for more than 1 host
	if len(cli.Hosts) > 1 {
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

	wg := cli.Limiter()
	data := make(chan interface{})

	for _, h := range cli.Hosts {
		fmt.Printf("Searching for files in %s ...\n", h.URL)
		wg.Add(1)
		go listProjectsFilesRegexp(h, re, listProjectsFilesOptions, wg, data, cli.WithCache())
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
		hclog.L().Error(err.Error())
	}

	return nil
}

func listProjectsFiles(h *client.Host, filepath, ref string, re []*regexp.Regexp, opt gitlab.ListProjectsOptions,
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
		go getRawFile(h, v, filepath, ref, re, gitlab.GetRawFileOptions{Ref: &ref}, wg, data, options...)
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

func listProjectsFilesRegexp(h *client.Host, re []*regexp.Regexp, opt gitlab.ListProjectsOptions,
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
		// context
		wg.Add(1)
		go listTree(h, v, re, gitlab.ListTreeOptions{
			ListOptions: opt.ListOptions,
			Ref:         &ref,
			Recursive:   gitlab.Bool(true),
		}, wg, data, options...)
	}

	if resp.NextPage > 0 {
		wg.Add(1)
		opt.Page = resp.NextPage
		go listProjectsFilesRegexp(h, re, opt, wg, data, options...)
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
		wg.Error(err)
		wg.Unlock()
		return
	}
	wg.Unlock()

	for _, r := range re {
		if r.Match(raw) {
			data <- sort.Element{Host: h, Struct: project, Cached: resp.Header.Get("X-From-Cache") == "1"}
		}
	}

}
