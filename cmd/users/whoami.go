package users

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/flant/glaball/pkg/client"
	"github.com/flant/glaball/pkg/limiter"
	"github.com/flant/glaball/pkg/sort"

	"github.com/flant/glaball/cmd/common"

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
	wg := common.Limiter
	data := make(chan interface{})
	for _, h := range common.Client.Hosts {
		fmt.Printf("Getting current user info from %s ...\n", h.URL)
		wg.Add(1)
		go currentUser(h, wg, data, common.Client.WithNoCache())
	}

	go func() {
		wg.Wait()
		close(data)
	}()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.TabIndent)
	fmt.Fprintf(w, "COUNT\tUSER\tHOSTS\tCACHED\n")
	total := 0

	results := sort.FromChannel(data, &sort.Options{
		OrderBy:    []string{"username"},
		SortBy:     "desc",
		GroupBy:    "",
		StructType: gitlab.User{},
	})

	for _, v := range results {
		total += v.Count //todo

		fmt.Fprintf(w, "[%d]\t%s\t%s\t[%s]\n", v.Count, v.Key, v.Elements.Hosts().Projects(common.Config.ShowAll), v.Cached)
	}

	fmt.Fprintf(w, "Total: %d\nErrors: %d\n", total, len(wg.Errors()))

	w.Flush()

	for _, err := range wg.Errors() {
		hclog.L().Error(err.Err.Error())
	}

	return nil
}

func currentUser(h *client.Host, wg *limiter.Limiter, data chan<- interface{}, options ...gitlab.RequestOptionFunc) {
	defer wg.Done()

	wg.Lock()
	user, resp, err := h.Client.Users.CurrentUser(options...)
	if err != nil {
		wg.Error(h, err)
		wg.Unlock()
		return
	}
	wg.Unlock()

	data <- sort.Element{Host: h, Struct: user, Cached: resp.Header.Get("X-From-Cache") == "1"}
}
