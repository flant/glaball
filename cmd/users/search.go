package users

import (
	"fmt"
	"os"
	"regexp"
	"text/tabwriter"

	"github.com/perhamm/glaball/pkg/sort/v2"
	"github.com/perhamm/glaball/pkg/util"

	"github.com/perhamm/glaball/cmd/common"

	"github.com/hashicorp/go-hclog"
	"github.com/spf13/cobra"
	"github.com/xanzy/go-gitlab"
)

var (
	searchBy          string
	searchFieldRegexp *regexp.Regexp
)

func NewSearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search --by=[email|username|name] [regexp]",
		Short: "Search for user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			re, err := regexp.Compile(args[0])
			if err != nil {
				return err
			}
			searchFieldRegexp = re
			return Search()
		},
	}

	cmd.Flags().Var(util.NewEnumValue(&searchBy, "email", "username", "name"), "by", "Search field")
	cmd.MarkFlagRequired("by")

	listUsersOptionsFlags(cmd, &listUsersOptions)

	return cmd
}

func Search() error {
	wg := common.Limiter
	data := make(chan interface{})

	fmt.Printf("Searching for user %s %q...\n", searchBy, searchFieldRegexp)
	for _, h := range common.Client.Hosts {
		wg.Add(1)
		go listUsersSearch(h, searchBy, searchFieldRegexp, listUsersOptions, wg, data, common.Client.WithCache())
	}

	go func() {
		wg.Wait()
		close(data)
	}()

	results, err := sort.FromChannel(data, &sort.Options{
		OrderBy:    []string{searchBy},
		StructType: gitlab.User{},
	})
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.TabIndent)
	fmt.Fprintf(w, "COUNT\tUSER\tHOSTS\tCACHED\n")

	total := 0
	for _, v := range results {
		fmt.Fprintf(w, "[%d]\t%s\t%s\t[%s]\n", v.Count, v.Key, v.Elements.Hosts().Projects(common.Config.ShowAll), v.Cached)
		total++
	}

	fmt.Fprintf(w, "Found: %d\nErrors: %d\n", total, len(wg.Errors()))

	w.Flush()

	for _, err := range wg.Errors() {
		hclog.L().Error(err.Err.Error())
	}

	return nil

}
