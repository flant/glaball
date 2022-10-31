package client

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/flant/glaball/pkg/config"
	"github.com/gregjones/httpcache"

	"github.com/ahmetb/go-linq"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/xanzy/go-gitlab"
)

type Client struct {
	Hosts Hosts

	config *config.Config
}

type Hosts []*Host

func (a Hosts) Projects(all bool) []string {
	k := len(a)
	if !all && k > 5 {
		k = 5
	}
	s := make([]string, 0, k)
	for _, h := range a[:k] {
		s = append(s, h.ProjectName())
	}
	sort.Strings(s)

	if !all && k == 5 {
		s = append(s, "...")
	}

	return s
}

func (h Hosts) Len() int      { return len(h) }
func (h Hosts) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h Hosts) Less(i, j int) bool {
	switch {
	case h[i].Team < h[j].Team:
		return true
	case h[i].Team > h[j].Team:
		return false
	}
	return h[i].Project < h[j].Project
}

type Host struct {
	Team, Project, Name, URL string
	Client                   *gitlab.Client
}

func (h Host) FullName() string {
	return fmt.Sprintf("%s.%s.%s", h.Team, h.Project, h.Name)
}

func (h Host) ProjectName() string {
	return fmt.Sprintf("%s.%s", h.Project, h.Name)
}

func (h *Host) CompareTo(c linq.Comparable) int {
	a, b := h.Project, c.(*Host).Project

	if a < b {
		return -1
	} else if a > b {
		return 1
	}

	return 0
}

func NewHttpClient(addresses map[string]string, cache *config.CacheOptions) (*http.Client, error) {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		DualStack: true,
	}

	transport := cleanhttp.DefaultPooledTransport()
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		if v, ok := addresses[host]; ok {
			dest := net.JoinHostPort(v, port)
			hclog.Default().Named("http-client").Debug("domain address has been modified", "original", addr, "modified", dest)
			addr = dest
		}
		return dialer.DialContext(ctx, network, addr)
	}

	if cache == nil || !cache.Enabled {
		return &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second,
		}, nil
	}

	diskCache, err := cache.DiskCache()
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

func NewClient(cfg *config.Config) (*Client, error) {
	filter, err := regexp.Compile(cfg.Filter)
	if err != nil {
		return nil, err
	}

	customAddresses := make(map[string]string)

	httpClient, err := NewHttpClient(customAddresses, &cfg.Cache)
	if err != nil {
		return nil, err
	}

	options := []gitlab.ClientOptionFunc{
		gitlab.WithHTTPClient(httpClient),
	}

	if hclog.L().IsDebug() {
		options = append(options, gitlab.WithCustomLeveledLogger(hclog.Default().Named("go-gitlab")))
	}

	client := Client{config: cfg}
	for team, projects := range cfg.Hosts {
		for project, hosts := range projects {
			for name, host := range hosts {
				fullName := strings.Join([]string{team, project, name}, ".")
				if !filter.MatchString(fullName) {
					continue
				}
				if host.URL == "" {
					return nil, fmt.Errorf("missing url for host %q", fullName)
				}
				if host.Token == "" {
					return nil, fmt.Errorf("missing token for host %q", fullName)
				}
				if cfg.Cache.Enabled && !host.RateLimiter.Enabled {
					options = append(options, gitlab.WithCustomLimiter(&FakeLimiter{}))
				}
				gl, err := gitlab.NewClient(host.Token,
					append(options, gitlab.WithBaseURL(host.URL))...)
				if err != nil {
					return nil, err
				}

				if host.IP != "" {
					customAddresses[gl.BaseURL().Hostname()] = host.IP
				}

				client.Hosts = append(client.Hosts, &Host{
					Team:    team,
					Project: project,
					Name:    name,
					URL:     host.URL,
					Client:  gl,
				})
			}
		}
	}

	return &client, nil

}

func (c *Client) WithCache() gitlab.RequestOptionFunc {
	return func(r *retryablehttp.Request) error {
		if c.config.Cache.Enabled {
			if c.config.Cache.TTL != nil {
				r.Header.Set("Cache-Control", fmt.Sprintf("max-age=%d", int(c.config.Cache.TTL.Seconds())))
				r.Header.Set("etag", "W/\"00000000000000000000000000000000-1\"")
			} else {
				r.Header.Set("Cache-Control", "max-stale")
			}
		}
		return nil
	}
}

func (c *Client) WithNoCache() gitlab.RequestOptionFunc {
	return func(r *retryablehttp.Request) error {
		r.Header.Set("Cache-Control", "max-age=0")
		r.Header.Set("etag", "W/\"00000000000000000000000000000000-1\"")
		return nil
	}
}

// Used to avoid unnecessary noncached requests
type FakeLimiter struct{}

func (*FakeLimiter) Wait(context.Context) error {
	return nil
}
