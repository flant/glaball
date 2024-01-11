package projects

import (
	"github.com/spf13/cobra"
)

const (
	projectDefaultField              = "web_url"
	projectWithLanguagesDefaultField = "project.web_url"
)

var (
	outputFormat []string
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "projects",
		Short: "Projects API",
	}

	cmd.PersistentFlags().StringSliceVar(&outputFormat, "output", []string{"table"},
		"Output format: [table csv]. Default: table.")

	cmd.AddCommand(
		NewEditCmd(),
		NewFilesCmd(),
		NewListCmd(),
		NewPipelinesCmd(),
		NewMergeRequestsCmd(),
		NewBranchesCmd(),
		NewRegistryCmd(),
		NewLanguagesCmd(),
	)

	return cmd
}
