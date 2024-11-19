package info

import (
	"fmt"

	"github.com/perhamm/glaball/pkg/config"
	"github.com/perhamm/glaball/pkg/util"

	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Information about the current build",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(util.PrintVersion(config.ApplicationName))
		},
	}

	return cmd
}
