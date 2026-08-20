package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type remoteCloudConfig struct {
	Schedule                *scheduleConfig `json:"schedule"`
	Poll                    *pollConfig     `json:"poll"`
	RecommendedAgentVersion string          `json:"recommended_agent_version"`
}

type printStationRow struct {
	ID        string `json:"id"`
	NamePt    string `json:"name_pt"`
	NameEn    string `json:"name_en"`
	NameZh    string `json:"name_zh"`
	SortOrder int    `json:"sort_order"`
}

func applyCloudRuntimeConfig(cfg *config, apiBase string) {
	if strings.TrimSpace(cfg.AgentJWT) == "" {
		return
	}

	url := strings.TrimRight(apiBase, "/") + "/api/print-agent/runtime-config"
	remote, err := fetchCloudRuntimeConfig(url, cfg.AgentJWT)
	if err != nil {
		agentLogTech(cfg, "log_runtime_config_fail", err.Error())
		return
	}

	if remote.Schedule != nil && remote.Schedule.enabled() {
		cfg.Schedule = remote.Schedule
	}
	if remote.Poll != nil {
		cfg.Poll = remote.Poll
	}
	if v := strings.TrimSpace(remote.RecommendedAgentVersion); v != "" {
		maybeNotifyVersionUpgrade(v)
	}
}

func logAgentStartup(cfg *config, apiBase string, stationCount int) {
	if cfg.Schedule != nil && cfg.Schedule.enabled() {
		tz := strings.TrimSpace(cfg.Schedule.Timezone)
		if tz == "" {
			tz = "Local"
		}
		for _, line := range formatScheduleLines(cfg.Schedule) {
			agentLog(cfg, "log_schedule_line", line)
		}
		agentLog(cfg, "log_schedule_tz", tz)
	} else {
		agentLog(cfg, "log_schedule_disabled")
	}

	p := defaultPollConfig()
	if cfg.Poll != nil {
		p = cfg.Poll.normalized()
	}
	agentLog(cfg, "log_poll_intervals",
		p.AfterPrintIntervalSec,
		p.WarmIntervalSec,
		p.WarmAfterActivitySec,
		p.IdleIntervalSec,
		p.BusyIntervalSec,
		p.ClosedCheckSec,
	)
}

func formatScheduleLines(s *scheduleConfig) []string {
	var lines []string
	add := func(label string, d *daySchedule) {
		if d == nil || len(d.Windows) == 0 {
			return
		}
		var parts []string
		for _, w := range d.Windows {
			parts = append(parts, fmt.Sprintf("%s–%s", w.Start, w.End))
		}
		lines = append(lines, fmt.Sprintf("%s: %s", label, strings.Join(parts, ", ")))
	}
	add("weekday", s.Weekday)
	add("monday", s.Monday)
	add("tuesday", s.Tuesday)
	add("wednesday", s.Wednesday)
	add("thursday", s.Thursday)
	add("friday", s.Friday)
	add("saturday", s.Saturday)
	add("sunday", s.Sunday)
	return lines
}

func fetchCloudRuntimeConfig(url, jwt string) (*remoteCloudConfig, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("%s: %s", res.Status, string(raw))
	}
	var out remoteCloudConfig
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func fetchPrintStations(apiBase, jwt string) ([]printStationRow, error) {
	url := strings.TrimRight(apiBase, "/") + "/api/print-agent/print-stations"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("%s: %s", res.Status, string(raw))
	}
	var out struct {
		Stations []printStationRow `json:"stations"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out.Stations, nil
}
