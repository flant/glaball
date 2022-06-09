package users

import (
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users",
		Short: "Users API",
	}

	cmd.AddCommand(
		NewBlockCmd(),
		NewCreateCmd(),
		NewDeleteCmd(),
		NewListCmd(),
		NewModifyCmd(),
		NewSearchCmd(),
		NewWhoamiCmd(),
	)

	return cmd
}
