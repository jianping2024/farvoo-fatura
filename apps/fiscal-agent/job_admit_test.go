package main

import (
	"testing"
	"time"
)

func TestJobAgeSeconds(t *testing.T) {
	job := printJob{CreatedAt: time.Now().Add(-90 * time.Second).UTC().Format(time.RFC3339)}
	age, ok := jobAgeSeconds(job)
	if !ok {
		t.Fatal("expected ok")
	}
	if age < 89 || age > 95 {
		t.Fatalf("age=%d want ~90", age)
	}
	if _, ok := jobAgeSeconds(printJob{}); ok {
		t.Fatal("empty created_at should not be ok")
	}
}

func TestAdmitPendingJobs(t *testing.T) {
	cfg := &config{
		StationPrinters: map[string]string{"st1": "winspool:P1"},
	}
	q := NewJobQueue()
	jobs := []printJob{
		{
			ID:        "a",
			Type:      "station_ticket",
			Status:    "pending",
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
			Payload:   []byte(`{"print_station_id":"st1"}`),
		},
		{
			ID:        "a",
			Type:      "station_ticket",
			Status:    "pending",
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
			Payload:   []byte(`{"print_station_id":"st1"}`),
		},
	}
	fetched, admitted := admitPendingJobs(cfg, q, jobs, "Test")
	if fetched != 2 || admitted != 1 {
		t.Fatalf("fetched=%d admitted=%d want 2,1", fetched, admitted)
	}
	if q.Len() != 1 {
		t.Fatalf("queue len=%d", q.Len())
	}
}
