package main

import "testing"

func TestTrayLevelForSummary(t *testing.T) {
	cases := []struct {
		in   string
		want trayLevel
	}{
		{"Ready", trayLevelGreen},
		{"Printing", trayLevelGreen},
		{"Outside business hours", trayLevelYellow},
		{"Connection problem", trayLevelRed},
		{"Print failed", trayLevelRed},
	}
	for _, tc := range cases {
		if got := trayLevelForSummary(tc.in); got != tc.want {
			t.Fatalf("%q: got %v want %v", tc.in, got, tc.want)
		}
	}
}

func TestAgentStatusScheduleClosedBlocksReady(t *testing.T) {
	s := &agentStatus{}
	s.setScheduleClosed(true, "Not polling until next window")
	s.set("Ready", "Waiting for jobs")
	if s.level() != trayLevelYellow {
		t.Fatalf("closed + Ready overwrite: want yellow, got %v", s.level())
	}
	s.set("Error", "boom")
	s.mu.RLock()
	sum := s.summary
	s.mu.RUnlock()
	if sum != "Error" {
		t.Fatalf("Error should still show while closed, got %q", sum)
	}
	s.setScheduleClosed(false, "")
	s.set("Ready", "Waiting for jobs")
	if s.level() != trayLevelGreen {
		t.Fatalf("after open Ready should be green, got %v", s.level())
	}
}

func TestTrayUserSummaryLocalized(t *testing.T) {
	if got := trayUserSummary("Ready", "zh"); got != "运行正常" {
		t.Fatalf("zh got %q", got)
	}
	if got := trayUserSummary("Ready", "en"); got != "Running OK" {
		t.Fatalf("en got %q", got)
	}
	if got := testPrintPhrase("en"); got != "PRINT TEST" {
		t.Fatalf("en test phrase got %q", got)
	}
}
