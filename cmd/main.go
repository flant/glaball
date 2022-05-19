package main

import (
	"os"
	"time"

	gconfig "github.com/flant/gitlaball/pkg/config"
	"github.com/flant/gitlaball/pkg/limiter"

	"github.com/flant/gitlaball/cmd/cache"
	"github.com/flant/gitlaball/cmd/common"
	"github.com/flant/gitlaball/cmd/config"
	"github.com/flant/gitlaball/cmd/info"
	"github.com/flant/gitlaball/cmd/projects"
	"github.com/flant/gitlaball/cmd/users"
	"github.com/flant/gitlaball/cmd/versions"

	"github.com/hashicorp/go-hclog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string

	logLevel string // "debug", "info", "warn", "error", "off"
	update   bool
	verbose  bool

	rootCmd = &cobra.Command{
		Use:           "gitlaball",
		Short:         "Bulk Gitlab instances administrator",
		Long:          ``,
		SilenceErrors: false,
		SilenceUsage:  true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if verbose {
				logLevel = "debug"
			}

			if err := setLogLevel(logLevel); err != nil {
				return err
			}

			if update {
				viper.Set("cache.ttl", time.Duration(0))
			}

			if err := common.Init(); err != nil {
				return err
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		hclog.L().Error(err.Error())
		os.Exit(1)
	}
}

func main() {
	rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "config.yaml",
		"Path to the configuration file. (default \"config.yaml\")")

	rootCmd.PersistentFlags().Int("threads", limiter.DefaultLimit,
		"Number of concurrent processes. (default: one process for each Gitlab instances in config file)")

	rootCmd.PersistentFlags().Duration("ttl", time.Duration(time.Hour*24),
		"Override cache TTL set in config file")

	rootCmd.PersistentFlags().StringP("filter", "f", ".*", "Select Gitlab(s) by regexp filter")

	rootCmd.PersistentFlags().StringVar(&logLevel, "log_level", "info",
		"Only log messages with the given severity or above. Valid levels: [debug, info, warn, error, off]")

	rootCmd.PersistentFlags().BoolVarP(&update, "update", "u", false, "Refresh cache")

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

	rootCmd.AddCommand(
		cache.NewCmd(),
		config.NewCmd(),
		info.NewCmd(),
		projects.NewCmd(),
		users.NewCmd(),
		users.NewWhoamiCmd(),
		versions.NewCmd(),
	)
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Search config in home directory with name ".cobra" (without extension).
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName("config.yaml")
	}

	viper.SetDefault("cache.enabled", true)
	viper.SetDefault("cache.size", gconfig.DefaultCacheSize)
	viper.SetDefault("cache.compression", true)
	viper.BindPFlag("cache.ttl", rootCmd.Flags().Lookup("ttl"))

	viper.BindPFlag("filter", rootCmd.Flags().Lookup("filter"))

	viper.BindPFlag("threads", rootCmd.Flags().Lookup("threads"))

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		hclog.L().Debug("Using config file", "config", viper.ConfigFileUsed())
	}

}

func setLogLevel(logLevel string) error {
	options := hclog.LoggerOptions{
		Level:             hclog.LevelFromString(logLevel),
		JSONFormat:        false,
		IncludeLocation:   false,
		DisableTime:       true,
		Color:             hclog.AutoColor,
		IndependentLevels: false,
	}

	hclog.SetDefault(hclog.New(&options))

	return nil
}
