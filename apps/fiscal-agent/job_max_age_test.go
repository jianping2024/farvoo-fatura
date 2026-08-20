package main

import (
	"testing"
	"time"
)

func TestJobPrintExpired(t *testing.T) {
	old := printJob{ID: "a", CreatedAt: time.Now().Add(-11 * time.Minute).UTC().Format(time.RFC3339)}
	if !jobPrintExpired(old) {
		t.Fatal("expected expired")
	}
	fresh := printJob{ID: "b", CreatedAt: time.Now().Add(-5 * time.Minute).UTC().Format(time.RFC3339)}
	if jobPrintExpired(fresh) {
		t.Fatal("expected fresh")
	}
}
