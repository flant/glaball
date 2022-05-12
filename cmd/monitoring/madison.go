package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/go-hclog"
)

const (
	madisonAlertDateLayout = "2006-01-02 15:04:05 -0700"
)

type MadisonAlert struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartTime   string            `json:"starts_at"`
}

func sendAlert(project, host, triggerName, message, description, severity string) error {
	hclog.L().Info("sending alert to madison", "project", project, "severity", severity)

	authKey, err := madisonAuthInit(project)
	if err != nil {
		return err
	}

	data, err := json.Marshal(MadisonAlert{
		Labels: map[string]string{
			"trigger":        triggerName,
			"gitlab_host":    host,
			"severity_level": severity,
		},
		Annotations: map[string]string{
			"summary":                      message,
			"description":                  description,
			"plk_protocol_version":         "1",
			"plk_markup_format":            "markdown",
			"plk_incident_link_url/gitlab": host,
		},
		StartTime: time.Now().Format(madisonAlertDateLayout),
	})
	if err != nil {
		return fmt.Errorf("marshal data for alert failed with msg: %v", err)
	}

	url := fmt.Sprintf("https://madison.flant.com/api/events/custom/%s", authKey)

	if _, err := httpClient.Post(url, "application/json", bytes.NewBuffer(data)); err != nil {
		return fmt.Errorf("alert failed for project %q with error: %v", project, err)
	}

	return nil
}

func deadMansSwitch() error {
	hclog.L().Info("sending DeadMansSwitch to madison")

	dmsKey, ok := os.LookupEnv("MADISON_DMS_KEY")
	if !ok {
		return fmt.Errorf("MADISON_DMS_KEY is empty")
	}

	data, err := json.Marshal(MadisonAlert{
		Labels: map[string]string{
			"trigger":        "DeadMansSwitch",
			"dms":            "gitlaball",
			"severity_level": "4",
		},
		Annotations: map[string]string{
			"summary":              "Alerting DeadMansSwitch",
			"description":          "This is a DeadMansSwitch meant to ensure that the gitlaball monitoring is functional",
			"plk_protocol_version": "1",
			"plk_markup_format":    "markdown",
		},
	})

	if err != nil {
		return fmt.Errorf("marshal data for dms alert failed with msg: %v", err)
	}

	url := fmt.Sprintf("https://madison.flant.com/api/events/custom/%s", dmsKey)

	if _, err := httpClient.Post(url, "application/json", bytes.NewBuffer(data)); err != nil {
		return fmt.Errorf("dms alert request failed with error: %v", err)
	}

	return nil
}

func madisonAuthInit(project string) (string, error) {
	hclog.L().Info("sending auth request to madison", "project", project)

	setupKey, ok := os.LookupEnv("MADISON_API_KEY")
	if !ok {
		return "", fmt.Errorf("MADISON_API_KEY is empty")
	}

	data, err := json.Marshal(map[string]string{
		"type":             "custom",
		"identifier":       "gitlaball",
		"use_default_team": "true",
	})
	if err != nil {
		return "", fmt.Errorf("marshal data for auth init failed with msg: %v", err)
	}

	url := fmt.Sprintf("https://madison.flant.com/api/%s/self_setup/%s", project, setupKey)

	resp, err := httpClient.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return "", fmt.Errorf("auth request to madison failed for project %q with error: %v", project, err)
	}

	var authResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return "", err
	}

	if err, ok := authResp["error"].(string); ok {
		return "", fmt.Errorf("failed to get auth_key from madison for project %q: %v", project, err)
	}

	key, ok := authResp["auth_key"].(string)
	if !ok {
		return "", fmt.Errorf("failed to parse auth_key for project %q", project)
	}

	return key, nil
}
