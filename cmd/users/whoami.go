package users

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/flant/gitlaball/cmd/common"
	"github.com/flant/gitlaball/pkg/client"
	"github.com/flant/gitlaball/pkg/limiter"
	"github.com/flant/gitlaball/pkg/sort"

	"github.com/hashicorp/go-hclog"
	"github.com/spf13/cobra"
	"github.com/xanzy/go-gitlab"
)

func NewWhoamiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Current API user",
		Long:  "Get info about the user whose token is used for API calls.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Whoami()
		},
	}

	return cmd
}

func Whoami() error {
	cli, err := common.Client()
	if err != nil {
		return err
	}

	wg := cli.Limiter()
	data := make(chan interface{})
	for _, h := range cli.Hosts {
		fmt.Printf("Getting current user info from %s ...\n", h.URL)
		wg.Add(1)
		go currentUser(h, wg, data, cli.WithNoCache())
	}

	go func() {
		wg.Wait()
		close(data)
	}()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.TabIndent)
	total := 0

	results := sort.FromChannel(data, &sort.Options{
		OrderBy:    []string{"username"},
		SortBy:     "desc",
		GroupBy:    "",
		StructType: gitlab.User{},
	})

	for _, v := range results {
		total += v.Count //todo

		fmt.Fprintf(w, "[%d]\t%s\t%s\t[%s]\n", v.Count, v.Key, v.Elements.Hosts().Projects(), v.Cached)
	}

	fmt.Fprintf(w, "Total: %d\nErrors: %d\n", total, len(wg.Errors()))

	w.Flush()

	for _, err := range wg.Errors() {
		hclog.L().Error(err.Error())
	}

	return nil
}

func currentUser(h *client.Host, wg *limiter.Limiter, data chan<- interface{}, options ...gitlab.RequestOptionFunc) {
	defer wg.Done()

	wg.Lock()
	user, resp, err := h.Client.Users.CurrentUser(options...)
	if err != nil {
		wg.Error(err)
		wg.Unlock()
		return
	}
	wg.Unlock()

	data <- sort.Element{Host: h, Struct: user, Cached: resp.Header.Get("X-From-Cache") == "1"}
}
