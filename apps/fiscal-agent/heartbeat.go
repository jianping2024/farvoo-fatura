package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type heartbeatSnapshot struct {
	lastPrintAt     time.Time
	lastPrintStatus string
}

func (c *config) mappedStationCount() int {
	if c == nil || c.StationPrinters == nil {
		return 0
	}
	n := 0
	for _, v := range c.StationPrinters {
		if strings.TrimSpace(v) != "" {
			n++
		}
	}
	return n
}

func (hb *heartbeatSnapshot) recordPrint(ok bool) {
	hb.lastPrintAt = time.Now()
	if ok {
		hb.lastPrintStatus = "done"
	} else {
		hb.lastPrintStatus = "failed"
	}
}

func postHeartbeat(ctx context.Context, cfg *config, scheduleOpen bool, hb *heartbeatSnapshot, mode NotificationMode) error {
	if cfg == nil || strings.TrimSpace(cfg.AgentJWT) == "" {
		return nil
	}
	body := map[string]any{
		"agent_version":        Version,
		"mapped_station_count": cfg.mappedStationCount(),
		"schedule_open":        scheduleOpen,
	}
	if mode == NotificationModeRealtime || mode == NotificationModePolling {
		body["notification_mode"] = string(mode)
	}
	if hb != nil && !hb.lastPrintAt.IsZero() && hb.lastPrintStatus != "" {
		body["last_print_at"] = hb.lastPrintAt.UTC().Format(time.RFC3339)
		body["last_print_status"] = hb.lastPrintStatus
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(cfg.APIBase, "/")+"/api/print-agent/heartbeat",
		bytes.NewReader(raw),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.AgentJWT)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	respBody, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("heartbeat %s: %s", res.Status, string(respBody))
	}
	return nil
}
