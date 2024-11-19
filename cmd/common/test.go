package common

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flant/glaball/pkg/client"
	"github.com/flant/glaball/pkg/config"
	"github.com/flant/glaball/pkg/limiter"
)

// Setup sets up a test HTTP server along with a gitlab.Client that is
// configured to talk to that test server.  Tests should register handlers on
// mux which provide mock responses for the API method being tested.
func Setup(t *testing.T) (*http.ServeMux, *httptest.Server, *client.Client) {
	// mux is the HTTP request multiplexer used with the test server.
	mux := http.NewServeMux()

	// server is a test HTTP server used to provide mock API responses.
	server := httptest.NewServer(mux)

	// client is the Gitlab client being tested.
	client, err := client.NewClient(&config.Config{
		Hosts: map[string]map[string]map[string]config.Host{
			"alfa": {"test": {"local": config.Host{URL: server.URL, Token: "testtoken"}}},
			"beta": {"test": {"local": config.Host{URL: server.URL, Token: "testtoken"}}},
		},
		Cache:   config.CacheOptions{},
		Filter:  "",
		Threads: limiter.DefaultLimit,
	})

	if err != nil {
		server.Close()
		t.Fatalf("Failed to create client: %v", err)
	}

	return mux, server, client
}

// Teardown closes the test HTTP server.
func Teardown(server *httptest.Server) {
	server.Close()
}
