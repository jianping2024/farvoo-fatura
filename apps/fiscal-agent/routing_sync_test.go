package main

import (
	"errors"
	"net/http"
	"testing"
)

func TestParseRoutingSyncErrorConflict(t *testing.T) {
	body := []byte(`{
		"error": "station_mapping_conflict",
		"code": "station_mapping_conflict",
		"conflicts": [{
			"station_id": "11111111-1111-1111-1111-111111111111",
			"station_label": "Kitchen",
			"other_device_id": "22222222-2222-2222-2222-222222222222",
			"other_device_label": "Bar PC"
		}]
	}`)
	err := parseRoutingSyncError(http.StatusConflict, body)
	rse, ok := isRoutingSyncConflict(err)
	if !ok {
		t.Fatalf("expected conflict error, got %v", err)
	}
	if len(rse.Conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(rse.Conflicts))
	}
	if rse.Conflicts[0].StationLabel != "Kitchen" {
		t.Fatalf("unexpected label %q", rse.Conflicts[0].StationLabel)
	}
}

func TestParseRoutingSyncErrorUnauthorized(t *testing.T) {
	err := parseRoutingSyncError(http.StatusUnauthorized, []byte(`{"error":"unauthorized"}`))
	var rse *RoutingSyncError
	if !errors.As(err, &rse) || rse.Code != "unauthorized" {
		t.Fatalf("expected unauthorized RoutingSyncError, got %v", err)
	}
}
