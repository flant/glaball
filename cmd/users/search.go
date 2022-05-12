package users

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/flant/gitlaball/cmd/common"
	"github.com/flant/gitlaball/pkg/sort"
	"github.com/flant/gitlaball/pkg/util"

	"github.com/hashicorp/go-hclog"
	"github.com/spf13/cobra"
	"github.com/xanzy/go-gitlab"
)

var (
	searchBy, searchFieldValue string
)

func NewSearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search [by]",
		Short: "Search for user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			searchFieldValue = args[0]
			return Search()
		},
	}

	cmd.Flags().Var(util.NewEnumValue(&searchBy, "email", "username", "name"), "by", "Search field")
	cmd.MarkFlagRequired("by")

	listUsersOptionsFlags(cmd, &listUsersOptions)

	return cmd
}

func Search() error {
	cli, err := common.Client()
	if err != nil {
		return err
	}

	wg := cli.Limiter()
	data := make(chan interface{})

	fmt.Printf("Searching for user %q...\n", searchFieldValue)
	for _, h := range cli.Hosts {
		wg.Add(1)
		go listUsersSearch(h, searchBy, searchFieldValue, gitlab.ListUsersOptions{
			ListOptions: gitlab.ListOptions{
				PerPage: 100,
			},
		}, wg, data, cli.WithCache())
	}

	go func() {
		wg.Wait()
		close(data)
	}()

	results := sort.FromChannel(data, &sort.Options{
		OrderBy:    []string{searchBy},
		StructType: gitlab.User{},
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.TabIndent)

	total := 0
	for _, v := range results {
		fmt.Fprintf(w, "[%d]\t%s\t%s\t[%s]\n", v.Count, v.Key, v.Elements.Hosts().Projects(), v.Cached)
		total++
	}

	fmt.Fprintf(w, "Found: %d\nErrors: %d\n", total, len(wg.Errors()))

	w.Flush()

	for _, err := range wg.Errors() {
		hclog.L().Error(err.Error())
	}

	return nil

}
