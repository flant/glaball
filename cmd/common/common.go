package common

import (
	"github.com/flant/gitlaball/pkg/client"
	"github.com/flant/gitlaball/pkg/config"

	"github.com/spf13/viper"
)

func Client() (*client.Client, error) {
	var cfg config.Config

	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return client.NewClient(&cfg)
}

func Config() (*config.Config, error) {
	var cfg config.Config

	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
