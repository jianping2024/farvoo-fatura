// Package uievents is the ONLY Agent→Admin push path (SSE).
package uievents

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// EventBillDraftsChanged is the SSE event name for open bill-draft list changes.
const EventBillDraftsChanged = "bill_drafts_changed"

// BillDraftsChangedPayload is the JSON body of bill_drafts_changed.
type BillDraftsChangedPayload struct {
	OpenCount          int    `json:"open_count"`
	TableDisplayName   string `json:"table_display_name,omitempty"`
	Kind               string `json:"kind,omitempty"` // upsert | delete
}

// Hub fans out Admin UI events. ONE hub per fiscal HTTP stack.
type Hub struct {
	mu      sync.Mutex
	clients map[chan []byte]struct{}
}

// NewHub returns an empty SSE hub.
func NewHub() *Hub {
	return &Hub{clients: make(map[chan []byte]struct{})}
}

// NotifyBillDraftsChanged is the ONLY publisher for bill_drafts_changed.
func (h *Hub) NotifyBillDraftsChanged(p BillDraftsChangedPayload) {
	if h == nil {
		return
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return
	}
	msg := []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", EventBillDraftsChanged, raw))
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- msg:
		default:
			// slow client: drop this tick; next change will retry
		}
	}
}

// ServeSSE is the ONLY HTTP handler for GET /local/v1/events.
func (h *Hub) ServeSSE(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		http.Error(w, "events unavailable", http.StatusServiceUnavailable)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch := make(chan []byte, 8)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.clients, ch)
		h.mu.Unlock()
		close(ch)
	}()

	// Initial comment so proxies keep the stream open.
	_, _ = fmt.Fprintf(w, ": ok\n\n")
	flusher.Flush()

	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-ch:
			if _, err := w.Write(msg); err != nil {
				return
			}
			flusher.Flush()
		case <-ticker.C:
			if _, err := fmt.Fprintf(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
