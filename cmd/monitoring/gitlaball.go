package main

import (
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/flant/gitlaball/pkg/client"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-retryablehttp"
)

type VersionCheckResponse struct {
	XmlName xml.Name `xml:"svg"`
	Text    string   `xml:"text"`
}

func checkGitlab(gitlab *client.Host) error {
	version, _, err := gitlab.Client.Version.GetVersion()
	if err != nil {
		return err
	}

	b64version := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("{\"version\": \"%s\"}", version.Version)))
	url := fmt.Sprintf("https://version.gitlab.com/check.svg?gitlab_info=%s", b64version)

	req, err := retryablehttp.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Referer", fmt.Sprintf("%s/help", gitlab.URL))

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}

	dec := xml.NewDecoder(resp.Body)

	var versionCheckResponse VersionCheckResponse
	if err := dec.Decode(&versionCheckResponse); err != nil {
		return err
	}

	switch strings.ToLower(versionCheckResponse.Text) {
	case "up-to-date":
		hclog.L().Info("host is up to date", "project", gitlab.Project, "host", gitlab.URL, "name", gitlab.Name)
	case "up to date":
		hclog.L().Info("host is up to date", "project", gitlab.Project, "host", gitlab.URL, "name", gitlab.Name)
	case "new version out":
		//alertMessage := fmt.Sprintf("Update available for your gitlab on %s. Current version %s", gitlab.URL, version.Version)
		//sendAlert(gitlab.Project, gitlab.URL,"GitlabUpdateAvailable","Update for gitlab available", alertMessage,"5")
		hclog.L().Info("update is available", "project", gitlab.Project, "host", gitlab.URL, "name", gitlab.Name, "current_version", version.Version)
	case "update available":
		//alertMessage := fmt.Sprintf("Update available for your gitlab on %s. Current version %s", gitlab.URL, version.Version)
		//sendAlert(gitlab.Project, gitlab.URL, "GitlabUpdateAvailable", "Update for gitlab available", alertMessage, "5")
		hclog.L().Info("update is available", "project", gitlab.Project, "host", gitlab.URL, "name", gitlab.Name, "current_version", version.Version)
	case "update asap":
		hclog.L().Warn("update asap", "project", gitlab.Project, "host", gitlab.URL, "name", gitlab.Name, "current_version", version.Version)
		if !hclog.L().IsDebug() {
			return sendAlert(gitlab.Project, gitlab.URL, "GitlabUpdateAsap", "Update gitlab ASAP",
				fmt.Sprintf("Update ASAP your gitlab version on %s. Current version %s", gitlab.URL, version.Version), "4")
		}
	default:
		return fmt.Errorf("something went wrong with host %s in project %s: %s", gitlab.URL, gitlab.Project, versionCheckResponse.Text)
	}

	return nil
}
