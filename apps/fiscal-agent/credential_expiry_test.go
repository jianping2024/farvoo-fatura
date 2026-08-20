package main

import (
	"testing"
	"time"
)

func TestCredentialInReminderWindow(t *testing.T) {
	until := time.Now().Add(20 * 24 * time.Hour).Format(time.RFC3339)
	cfg := &config{ValidUntil: until}
	if !cfg.credentialInReminderWindow(time.Now()) {
		t.Fatal("expected reminder window")
	}
}

func TestCredentialDaysRemaining(t *testing.T) {
	until := time.Now().Add(48 * time.Hour)
	cfg := &config{ValidUntil: until.Format(time.RFC3339)}
	days, ok := cfg.credentialDaysRemaining(time.Now())
	if !ok || days < 1 || days > 3 {
		t.Fatalf("days=%d ok=%v", days, ok)
	}
}
