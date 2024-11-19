package cache

import (
	"fmt"

	"github.com/perhamm/glaball/cmd/common"

	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Cache management",
	}

	cmd.AddCommand(
		NewCleanCmd(),
	)

	return cmd
}

func NewCleanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Clean cache",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Clean()
		},
	}

	return cmd
}

func Clean() error {
	diskv, err := common.Config.Cache.Diskv()
	if err != nil {
		return err
	}
	if err := diskv.EraseAll(); err != nil {
		return err
	}

	fmt.Printf("Successfully cleaned up: %s\n", diskv.BasePath)

	return nil
}
