package config

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/flant/glaball/cmd/common"

	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Information about the current configuration",
	}

	cmd.AddCommand(
		NewListCmd(),
	)

	return cmd
}

func NewListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List gitlabs stored in config",
		RunE: func(cmd *cobra.Command, args []string) error {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.TabIndent)
			fmt.Fprintf(w, "HOST\tURL\n")
			total := 0

			sort.Sort(common.Client.Hosts)
			for _, h := range common.Client.Hosts {
				fmt.Fprintf(w, "[%s]\t%s\n", h.FullName(), h.URL)
				total++
			}

			fmt.Fprintf(w, "Total: %d\n", total)

			w.Flush()

			return nil
		},
	}

	return cmd
}
