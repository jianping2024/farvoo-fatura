package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMappedStationCount(t *testing.T) {
	c := &config{StationPrinters: map[string]string{"a": "tcp:1.2.3.4:9100", "b": "", "c": "winspool:UK"}}
	if c.mappedStationCount() != 2 {
		t.Fatalf("got %d", c.mappedStationCount())
	}
}

func TestHeartbeatSnapshotRecordPrint(t *testing.T) {
	var hb heartbeatSnapshot
	hb.recordPrint(true)
	if hb.lastPrintStatus != "done" || hb.lastPrintAt.IsZero() {
		t.Fatal("expected done timestamp")
	}
	hb.recordPrint(false)
	if hb.lastPrintStatus != "failed" {
		t.Fatal("expected failed")
	}
}

func TestPostHeartbeatIncludesNotificationMode(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/api/print-agent/heartbeat") {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &gotBody); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &config{
		APIBase:  srv.URL,
		AgentJWT: "test-jwt",
	}
	if err := postHeartbeat(context.Background(), cfg, true, nil, NotificationModePolling); err != nil {
		t.Fatal(err)
	}
	if gotBody["notification_mode"] != "polling" {
		t.Fatalf("notification_mode = %v", gotBody["notification_mode"])
	}
}
