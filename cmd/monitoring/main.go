package main

import (
	"fmt"
	"sync"

	"github.com/flant/gitlaball/pkg/client"
	"github.com/flant/gitlaball/pkg/config"
	"github.com/flant/gitlaball/pkg/util"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-retryablehttp"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	httpClient = retryablehttp.NewClient()

	logLevel = kingpin.Flag("log-level",
		"Only log messages with the given severity or above. Valid levels: [debug, info, warn, error, off]",
	).Default("info").Enum("debug", "info", "warn", "error", "off")

	configPath = kingpin.Flag("config", "config path").Default("config.yaml").String()
)

func init() {
	kingpin.New("gitlaball", "Simple tool to monitoring gitlab versions")
	kingpin.Version(util.PrintVersion("gitlaball"))
	kingpin.HelpFlag.Short('h')

	kingpin.CommandLine.Action(setLogLevel)

	checkAllCommand := kingpin.Command("check-all", "Check all hosts in config")
	checkAllCommand.Action(checkGitlabs)

	validateConfigCommand := kingpin.Command("config-validate", "Validate config.")
	validateConfigCommand.Action(validateConfig)
}

func main() {
	kingpin.Parse()
}

func initClient(configPath string) (*client.Client, error) {
	cfg, err := config.FromFile(configPath)
	if err != nil {
		return nil, err
	}

	cli, err := client.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	return cli, nil
}

func checkGitlabs(ctx *kingpin.ParseContext) error {
	cli, err := initClient(*configPath)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	errs := make(chan error)

	for _, gitlab := range cli.Hosts {
		wg.Add(1)
		go func(gitlab *client.Host) {
			defer wg.Done()

			if err := checkGitlab(gitlab); err != nil {
				hclog.L().Error(err.Error())

				if !hclog.L().IsDebug() {
					// send alert if check failed
					if err := sendAlert(gitlab.Project, gitlab.URL, "GitlabCheckFailed", "Failed to check current gitlab version", err.Error(), "5"); err != nil {
						errs <- err
						// send alert to team if project does not exist or any other error occurs
						if err := sendAlert(fmt.Sprintf("team-%s", gitlab.Team), gitlab.URL, "GitlabCheckFailed", "Failed to check current gitlab version", err.Error(), "4"); err != nil {
							errs <- err
						}
					}
				}
			}
		}(gitlab)
	}

	go func() {
		wg.Wait()
		close(errs)
	}()

	// print all errors
	for err := range errs {
		hclog.L().Error(err.Error())
	}

	if !hclog.L().IsDebug() {
		if err := deadMansSwitch(); err != nil {
			return err
		}
	}

	return nil
}

func validateConfig(ctx *kingpin.ParseContext) error {
	cli, err := initClient(*configPath)
	if err != nil {
		return err
	}

	for _, gitlab := range cli.Hosts {
		hclog.L().Info("validate config", "project", gitlab.Project, "host", gitlab.URL, "name", gitlab.Name, "team", gitlab.Team)
	}

	return nil
}

func setLogLevel(ctx *kingpin.ParseContext) error {
	options := hclog.LoggerOptions{
		Level:             hclog.LevelFromString(*logLevel),
		JSONFormat:        true,
		IncludeLocation:   false,
		DisableTime:       false,
		Color:             hclog.AutoColor,
		IndependentLevels: false,
	}

	hclog.SetDefault(hclog.New(&options))

	httpClient.Logger = hclog.L()

	return nil
}
