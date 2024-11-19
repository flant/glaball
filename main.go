package main

import (
	"os"
	"time"

	gconfig "github.com/perhamm/glaball/pkg/config"
	"github.com/perhamm/glaball/pkg/limiter"

	"github.com/perhamm/glaball/cmd/cache"
	"github.com/perhamm/glaball/cmd/common"
	"github.com/perhamm/glaball/cmd/config"
	"github.com/perhamm/glaball/cmd/info"
	"github.com/perhamm/glaball/cmd/projects"
	"github.com/perhamm/glaball/cmd/users"
	"github.com/perhamm/glaball/cmd/versions"

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
		Use:           gconfig.ApplicationName,
		Short:         "Gitlab bulk administration tool",
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

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		"Path to the configuration file. (default \"$HOME/.config/glaball/config.yaml\")")

	rootCmd.PersistentFlags().Int("threads", limiter.DefaultLimit,
		"Number of concurrent processes. (default: one process for each Gitlab instances in config file)")

	rootCmd.PersistentFlags().Duration("ttl", time.Duration(time.Hour*24),
		"Override cache TTL set in config file")

	rootCmd.PersistentFlags().StringP("filter", "f", ".*", "Select Gitlab(s) by regexp filter")

	rootCmd.PersistentFlags().BoolP("all", "a", false, "Show all hosts in grouped output")

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
		// Search config in default directory
		configDir, _ := gconfig.DefaultConfigDir()
		viper.AddConfigPath(configDir)
		viper.SetConfigType("yaml")
		viper.SetConfigName("config.yaml")
	}

	viper.SetDefault("cache.enabled", true)
	viper.SetDefault("cache.size", gconfig.DefaultCacheSize)
	viper.SetDefault("cache.compression", true)
	viper.BindPFlag("cache.ttl", rootCmd.Flags().Lookup("ttl"))

	viper.BindPFlag("filter", rootCmd.Flags().Lookup("filter"))

	viper.BindPFlag("all", rootCmd.Flags().Lookup("all"))

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
