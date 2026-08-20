package main

import "testing"

func TestJobRouteStationID(t *testing.T) {
	t.Parallel()
	j := printJob{Type: "station_ticket", Payload: []byte(`{"print_station_id":"kitchen"}`)}
	if jobRouteStationID(j) != "kitchen" {
		t.Fatalf("got %q", jobRouteStationID(j))
	}
}
