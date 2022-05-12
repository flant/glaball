package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Hosts   Hosts        `yaml:"hosts" mapstructure:"hosts"`
	Cache   CacheOptions `yaml:"cache" mapstructure:"cache"`
	Filter  string       `yaml:"filter" mapstructure:"filter"`
	Threads int          `yaml:"threads" mapstructure:"threads"`
}

type Hosts map[string]map[string]map[string]Host

type Host struct {
	URL         string             `yaml:"url" mapstructure:"url"`
	Token       string             `yaml:"token" mapstructure:"token"`
	RateLimiter RateLimiterOptions `yaml:"rate_limiter" mapstructure:"rate_limiter"`
}

type RateLimiterOptions struct {
	Enabled bool `yaml:"enabled" mapstructure:"enabled"`
}

func FromFile(path string) (*Config, error) {
	var config Config
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	if err := dec.Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %q error: %v", path, err)
	}

	return &config, nil
}
