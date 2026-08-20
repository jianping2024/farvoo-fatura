package main

import "context"

// NotificationMode defines how the agent learns about new print jobs.
type NotificationMode string

const (
	// NotificationModeRealtime: WebSocket push from Supabase (default).
	NotificationModeRealtime NotificationMode = "realtime"
	// NotificationModePolling: HTTP polling fallback (firewall environments).
	NotificationModePolling NotificationMode = "polling"
)

// Notifier delivers new print job notifications to a job queue.
type Notifier interface {
	// Start begins notification delivery (blocks until context canceled).
	Start(ctx context.Context) error
}
