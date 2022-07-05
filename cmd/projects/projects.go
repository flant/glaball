package projects

import (
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "projects",
		Short: "Projects API",
	}
	cmd.AddCommand(
		NewFilesCmd(),
		NewListCmd(),
		NewPipelinesCmd(),
		NewMergeRequestsCmd(),
	)

	return cmd
}
