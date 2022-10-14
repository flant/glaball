package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	ApplicationName = "glaball"
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
	IP          string             `yaml:"ip" mapstructure:"ip"`
	Token       string             `yaml:"token" mapstructure:"token"`
	RateLimiter RateLimiterOptions `yaml:"rate_limiter" mapstructure:"rate_limiter"`
}

type RateLimiterOptions struct {
	Enabled bool `yaml:"enabled" mapstructure:"enabled"`
}

func DefaultConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, ".config", ApplicationName), nil
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
