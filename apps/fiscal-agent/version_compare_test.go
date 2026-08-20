package main

import "testing"

func TestAgentVersionOlderThan(t *testing.T) {
	tests := []struct {
		cur, rec string
		want     bool
	}{
		{"0.2.64", "0.2.65", true},
		{"0.2.65", "0.2.65", false},
		{"0.3.0", "0.2.99", false},
		{"0.2.9", "0.2.10", true},
		{"0.0.1", "0.2.66", true},
	}
	for _, tc := range tests {
		if got := agentVersionOlderThan(tc.cur, tc.rec); got != tc.want {
			t.Fatalf("agentVersionOlderThan(%q, %q) = %v, want %v", tc.cur, tc.rec, got, tc.want)
		}
	}
}
