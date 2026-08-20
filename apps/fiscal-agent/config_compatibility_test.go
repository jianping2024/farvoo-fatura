package main

import (
	"testing"
)

// Test backward compatibility: old configs without notification_mode should default to realtime
func TestResolveNotificationModeDefault(t *testing.T) {
	// Old config without notification_mode field
	c := &config{
		APIBase:      "https://example.com",
		AgentJWT:     "test-jwt",
		RestaurantID: "test-restaurant",
	}
	
	mode := c.resolveNotificationMode()
	if mode != NotificationModeRealtime {
		t.Errorf("Expected default mode to be 'realtime', got '%s'", mode)
	}
}

// Test explicit realtime mode
func TestResolveNotificationModeRealtime(t *testing.T) {
	c := &config{
		APIBase:          "https://example.com",
		AgentJWT:         "test-jwt",
		RestaurantID:     "test-restaurant",
		NotificationMode: "realtime",
	}
	
	mode := c.resolveNotificationMode()
	if mode != NotificationModeRealtime {
		t.Errorf("Expected mode 'realtime', got '%s'", mode)
	}
}

// Test explicit polling mode
func TestResolveNotificationModePolling(t *testing.T) {
	c := &config{
		APIBase:          "https://example.com",
		AgentJWT:         "test-jwt",
		RestaurantID:     "test-restaurant",
		NotificationMode: "polling",
	}
	
	mode := c.resolveNotificationMode()
	if mode != NotificationModePolling {
		t.Errorf("Expected mode 'polling', got '%s'", mode)
	}
}

// Test invalid mode falls back to realtime
func TestResolveNotificationModeInvalid(t *testing.T) {
	c := &config{
		APIBase:          "https://example.com",
		AgentJWT:         "test-jwt",
		RestaurantID:     "test-restaurant",
		NotificationMode: "invalid-mode",
	}
	
	mode := c.resolveNotificationMode()
	if mode != NotificationModeRealtime {
		t.Errorf("Expected invalid mode to fallback to 'realtime', got '%s'", mode)
	}
}
