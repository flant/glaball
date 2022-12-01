package versions

import (
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/flant/glaball/pkg/client"
	"github.com/flant/glaball/pkg/limiter"
	"github.com/flant/glaball/pkg/sort"

	"github.com/flant/glaball/cmd/common"

	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-hclog"
	"github.com/spf13/cobra"
	"github.com/xanzy/go-gitlab"
)

var (
	httpClient = cleanhttp.DefaultPooledClient()
)

type VersionCheck struct {
	Version     string
	CheckResult string
}

type VersionCheckResponse struct {
	XmlName xml.Name `xml:"svg"`
	Text    string   `xml:"text"`
}

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "versions",
		Short: "Retrieve version information for GitLab instances",
		Long:  "",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Versions()
		},
	}

	return cmd
}

func Versions() error {
	wg := common.Limiter
	data := make(chan interface{})
	for _, h := range common.Client.Hosts {
		fmt.Printf("Getting current version info from %s ...\n", h.URL)
		wg.Add(1)
		go currentVersion(h, wg, data, common.Client.WithNoCache())
	}

	go func() {
		wg.Wait()
		close(data)
	}()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.TabIndent)
	fmt.Fprintf(w, "HOST\tURL\tVERSION\tSTATUS\n")
	total := 0

	results, err := sort.FromChannel(data, &sort.Options{
		OrderBy:    []string{"host"},
		SortBy:     "asc",
		GroupBy:    "",
		StructType: VersionCheck{},
	})
	if err != nil {
		return err
	}

	for _, v := range results {
		total++
		elem := v.Elements.Typed()[0]
		fmt.Fprintf(w, "[%s]\t%s\t%s\t[%s]\n", elem.Host.Project, elem.Host.URL, elem.Struct.(VersionCheck).Version, elem.Struct.(VersionCheck).CheckResult)
	}

	fmt.Fprintf(w, "Total: %d\nErrors: %d\n", total, len(wg.Errors()))

	w.Flush()

	for _, err := range wg.Errors() {
		hclog.L().Error(err.Err.Error())
	}

	return nil
}

func currentVersion(h *client.Host, wg *limiter.Limiter, data chan<- interface{}, options ...gitlab.RequestOptionFunc) {
	defer wg.Done()

	wg.Lock()
	version, resp, err := h.Client.Version.GetVersion()
	if err != nil {
		wg.Error(h, err)
		wg.Unlock()
		return
	}
	check, err := checkVersion(h, version)
	if err != nil {
		wg.Error(h, err)
		wg.Unlock()
		return
	}
	wg.Unlock()

	data <- sort.Element{Host: h, Struct: VersionCheck{version.Version, check}, Cached: resp.Header.Get("X-From-Cache") == "1"}
}

func checkVersion(h *client.Host, version *gitlab.Version) (string, error) {
	b64version := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("{\"version\": \"%s\"}", version.Version)))
	url := fmt.Sprintf("https://version.gitlab.com/check.svg?gitlab_info=%s", b64version)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Referer", fmt.Sprintf("%s/help", h.URL))

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}

	dec := xml.NewDecoder(resp.Body)

	var versionCheckResponse VersionCheckResponse
	if err := dec.Decode(&versionCheckResponse); err != nil {
		return "", err
	}

	return strings.ToLower(versionCheckResponse.Text), nil
}
