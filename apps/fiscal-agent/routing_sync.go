package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type RoutingSyncConflict struct {
	StationID        string  `json:"station_id"`
	StationLabel     string  `json:"station_label"`
	OtherDeviceID    string  `json:"other_device_id"`
	OtherDeviceLabel *string `json:"other_device_label"`
}

type RoutingSyncError struct {
	Code      string
	Message   string
	Conflicts []RoutingSyncConflict
}

func (e *RoutingSyncError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Code != "" {
		return e.Code
	}
	return "routing sync failed"
}

func parseRoutingSyncError(status int, body []byte) error {
	var payload struct {
		Error     string                `json:"error"`
		Code      string                `json:"code"`
		Message   string                `json:"message"`
		Conflicts []RoutingSyncConflict `json:"conflicts"`
	}
	if len(body) > 0 {
		_ = json.Unmarshal(body, &payload)
	}
	code := strings.TrimSpace(payload.Code)
	if code == "" && status == http.StatusConflict {
		code = "station_mapping_conflict"
	}
	msg := strings.TrimSpace(payload.Error)
	if msg == "" {
		msg = strings.TrimSpace(payload.Message)
	}
	if msg == "" {
		msg = fmt.Sprintf("HTTP %d", status)
	}
	if code == "station_mapping_conflict" || len(payload.Conflicts) > 0 {
		return &RoutingSyncError{Code: "station_mapping_conflict", Message: msg, Conflicts: payload.Conflicts}
	}
	if status == http.StatusUnauthorized {
		return &RoutingSyncError{Code: "unauthorized", Message: msg}
	}
	if code != "" {
		return &RoutingSyncError{Code: code, Message: msg}
	}
	return fmt.Errorf("HTTP %d: %s", status, msg)
}

func syncRoutingToCloud(cfg *config, forceTakeover bool) error {
	if cfg == nil || strings.TrimSpace(cfg.AgentJWT) == "" || strings.TrimSpace(cfg.APIBase) == "" {
		return nil
	}
	body := map[string]any{
		"station_printers": cfg.StationPrinters,
	}
	if forceTakeover {
		body["force_takeover"] = true
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	url := strings.TrimRight(cfg.APIBase, "/") + "/api/print-agent/routing"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.AgentJWT)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		agentLogTech(cfg, "log_routing_sync_fail", err.Error())
		return err
	}
	defer res.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(res.Body, 65536))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		agentLog(cfg, "log_routing_sync_http", res.StatusCode)
		return parseRoutingSyncError(res.StatusCode, respBody)
	}
	agentLog(cfg, "log_routing_sync_ok", len(cfg.StationPrinters))
	return nil
}

func isRoutingSyncConflict(err error) (*RoutingSyncError, bool) {
	var rse *RoutingSyncError
	if errors.As(err, &rse) && rse.Code == "station_mapping_conflict" {
		return rse, true
	}
	return nil, false
}
