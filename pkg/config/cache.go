package config

import (
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/alecthomas/units"
	"github.com/gregjones/httpcache"
	"github.com/gregjones/httpcache/diskcache"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/peterbourgon/diskv"
)

const (
	DefaultCacheSize = "100MB"
)

type CacheOptions struct {
	Enabled      bool           `yaml:"enabled" mapstructure:"enabled"`
	BasePath     string         `yaml:"path" mapstructure:"path"`
	CacheSizeMax string         `yaml:"size" mapstructure:"size"`
	Compression  bool           `yaml:"compression" mapstructure:"compression"`
	TTL          *time.Duration `yaml:"ttl" mapstructure:"ttl"`
}

func DefaultCacheDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, ".cache", ApplicationName), nil
}

func (c *CacheOptions) DiskvOptions() (diskv.Options, error) {
	if c.BasePath == "" {
		defaultCacheDir, err := DefaultCacheDir()
		if err != nil {
			return diskv.Options{}, err
		}
		c.BasePath = defaultCacheDir
	}

	if c.CacheSizeMax == "" {
		c.CacheSizeMax = DefaultCacheSize
	}
	size, err := units.ParseStrictBytes(c.CacheSizeMax)
	if err != nil {
		return diskv.Options{}, err
	}

	var compression diskv.Compression = nil
	if c.Compression {
		compression = diskv.NewGzipCompression()
	}

	return diskv.Options{
		BasePath:     c.BasePath,
		CacheSizeMax: uint64(size),
		Compression:  compression,
	}, nil
}

func (c *CacheOptions) Diskv() (*diskv.Diskv, error) {
	diskvOpts, err := c.DiskvOptions()
	if err != nil {
		return nil, err
	}

	return diskv.New(diskvOpts), nil
}

func (c *CacheOptions) DiskCache() (*diskcache.Cache, error) {
	diskv, err := c.Diskv()
	if err != nil {
		return nil, err
	}
	return diskcache.NewWithDiskv(diskv), nil
}

func (c *CacheOptions) HttpCacheClient() (*http.Client, error) {
	transport := cleanhttp.DefaultPooledTransport()

	if !c.Enabled {
		return &http.Client{Transport: transport}, nil
	}

	diskCache, err := c.DiskCache()
	if err != nil {
		return nil, err
	}

	return &http.Client{
		Transport: &httpcache.Transport{
			Transport:           transport,
			Cache:               diskCache,
			MarkCachedResponses: true,
		},
		Timeout: 10 * time.Second,
	}, nil

}
