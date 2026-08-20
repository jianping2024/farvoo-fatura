package main

import (
	"testing"
)

// Test Mode B heal: same host http supabase_url follows https api_base
func TestAlignSupabaseURLWithAPIBaseSameHost(t *testing.T) {
	c := &config{
		APIBase:     "https://pirata.farvoo.com",
		SupabaseURL: "http://pirata.farvoo.com",
	}
	if !alignSupabaseURLWithAPIBase(c) {
		t.Fatal("expected align")
	}
	if c.SupabaseURL != "https://pirata.farvoo.com" {
		t.Fatalf("got %q", c.SupabaseURL)
	}
	if c.getSupabaseURL() != "https://pirata.farvoo.com" {
		t.Fatalf("getSupabaseURL %q", c.getSupabaseURL())
	}
}

// Cloud: different hosts must not overwrite supabase project URL
func TestAlignSupabaseURLLeavesCloudProject(t *testing.T) {
	c := &config{
		APIBase:     "https://app.example.com",
		SupabaseURL: "https://xxxx.supabase.co",
	}
	if alignSupabaseURLWithAPIBase(c) {
		t.Fatal("must not align different hosts")
	}
	if c.getSupabaseURL() != "https://xxxx.supabase.co" {
		t.Fatalf("got %q", c.getSupabaseURL())
	}
}

// Test getSupabaseURL with inference fallback
func TestGetSupabaseURLInfer(t *testing.T) {
	c := &config{
		APIBase: "http://localhost:3000",
	}
	
	got := c.getSupabaseURL()
	want := "http://localhost:54321"
	
	if got != want {
		t.Errorf("getSupabaseURL() = %q, want %q", got, want)
	}
}

// Test backward compatibility: old configs without supabase_url still work
func TestBackwardCompatibilityOldConfig(t *testing.T) {
	c := &config{
		APIBase:      "https://example.com",
		AgentJWT:     "test-jwt",
		RestaurantID: "test-restaurant",
		// No SupabaseURL, no NotificationMode - old config
	}
	
	// Should default to Realtime
	mode := c.resolveNotificationMode()
	if mode != NotificationModeRealtime {
		t.Errorf("Expected default mode Realtime, got %s", mode)
	}
	
	// Should infer URL
	url := c.getSupabaseURL()
	if url == "" {
		t.Error("Expected inferred URL, got empty string")
	}
}

// Test explicit polling mode configuration
func TestExplicitPollingMode(t *testing.T) {
	c := &config{
		APIBase:          "https://example.com",
		NotificationMode: "polling",
	}
	
	mode := c.resolveNotificationMode()
	if mode != NotificationModePolling {
		t.Errorf("Expected Polling mode, got %s", mode)
	}
}
