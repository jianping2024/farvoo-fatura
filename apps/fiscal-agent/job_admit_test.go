package main

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
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

func TestFreshAdmitConfigSeesMappingSavedAfterStart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	stale := &config{} // startup snapshot: no mappings (e.g. just re-paired to a new restaurant)
	if err := saveConfig(path, &config{StationPrinters: map[string]string{"st1": "winspool:P1"}}); err != nil {
		t.Fatal(err)
	}
	job := printJob{ID: "j1", Type: "station_ticket", Status: "pending",
		CreatedAt: time.Now().UTC().Format(time.RFC3339), Payload: []byte(`{"print_station_id":"st1"}`)}

	if admitJob(stale, job, "Test") {
		t.Fatal("stale snapshot must not admit an unmapped station")
	}
	q := NewJobQueue()
	_, admitted := admitPendingJobs(freshAdmitConfig(path, stale), q, []printJob{job}, "Test")
	if admitted != 1 || q.Len() != 1 {
		t.Fatalf("fresh config must admit: admitted=%d len=%d", admitted, q.Len())
	}
	if freshAdmitConfig(filepath.Join(dir, "missing.json"), stale) != stale {
		t.Fatal("unreadable file must fall back to snapshot")
	}
}

func TestAdmitSkipIsLogged(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	job := printJob{ID: "skip-log-1", Type: "station_ticket", Payload: []byte(`{"print_station_id":"nope"}`)}
	if admitJob(&config{}, job, "Realtime") {
		t.Fatal("unmapped job must be skipped")
	}
	if !strings.Contains(buf.String(), "skip job skip-log-1") {
		t.Fatalf("skip not logged: %q", buf.String())
	}
}

// Sole-path guard: admit decisions only via admitJob + freshAdmitConfig.
func TestSoleAdmitPath(t *testing.T) {
	for _, f := range []string{"realtime.go", "polling.go", "job_admit.go"} {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		s := string(raw)
		if strings.Contains(s, ".jobEligibleForQueue(") {
			t.Fatalf("%s must gate via admitJob, not jobEligibleForQueue", f)
		}
		if f != "job_admit.go" && strings.Contains(s, "admitPendingJobs(r.config") || strings.Contains(s, "admitPendingJobs(p.config") {
			t.Fatalf("%s must pass freshAdmitConfig to admitPendingJobs", f)
		}
	}
	raw, _ := os.ReadFile("job_admit.go")
	for _, def := range []string{"func admitJob(", "func freshAdmitConfig("} {
		if strings.Count(string(raw), def) != 1 {
			t.Fatalf("want exactly one %s", def)
		}
	}
}
