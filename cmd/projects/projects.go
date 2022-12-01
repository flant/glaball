package projects

import (
	"github.com/spf13/cobra"
)

const (
	projectDefaultField = "web_url"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "projects",
		Short: "Projects API",
	}
	cmd.AddCommand(
		NewEditCmd(),
		NewFilesCmd(),
		NewListCmd(),
		NewPipelinesCmd(),
		NewMergeRequestsCmd(),
	)

	return cmd
}
