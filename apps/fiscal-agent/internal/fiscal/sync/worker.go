// Package sync pushes non-authoritative invoice copies to Farvoo (sync_outbox).
//
// Unique paths:
//   - Enqueue: store.EnqueueInvoiceIssuedTx (inside IssueFT txn only)
//   - Drain:   Worker.RunOnce → store.ClaimNextOutbox → PushInvoiceCopy → MarkOutboxSent/Retry
package sync

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/store"
)

const maxAttempts = 8

// Pusher delivers one outbox payload to Farvoo. Tests inject a stub.
type Pusher interface {
	PushInvoiceCopy(ctx context.Context, payloadJSON []byte) error
}

// HTTPPusher POSTs to {FARVOO_API}/api/print-agent/fiscal-invoice-copies with FARVOO_JWT.
// ONLY production push implementation.
type HTTPPusher struct {
	Client *http.Client
	API    string
	JWT    string
}

// PushInvoiceCopy posts the cloud copy — ONLY HTTP path for INVOICE_ISSUED delivery.
func (p *HTTPPusher) PushInvoiceCopy(ctx context.Context, payloadJSON []byte) error {
	api := strings.TrimRight(strings.TrimSpace(p.API), "/")
	jwt := strings.TrimSpace(p.JWT)
	if api == "" || jwt == "" {
		return fmt.Errorf("sync: FARVOO_API and FARVOO_JWT required to push outbox")
	}
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, api+"/api/print-agent/fiscal-invoice-copies", bytes.NewReader(payloadJSON))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwt)
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("sync: push status %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// NewHTTPPusherFromEnv builds the default pusher from FARVOO_API / FARVOO_JWT.
func NewHTTPPusherFromEnv() *HTTPPusher {
	return &HTTPPusher{
		API: os.Getenv("FARVOO_API"),
		JWT: os.Getenv("FARVOO_JWT"),
	}
}

// Worker drains sync_outbox — ONLY outbox drain loop.
type Worker struct {
	DB     *store.DB
	Pusher Pusher
}

// RunOnce claims at most one row and pushes it.
func (w *Worker) RunOnce(ctx context.Context) (bool, error) {
	if w.DB == nil || w.Pusher == nil {
		return false, nil
	}
	if hp, ok := w.Pusher.(*HTTPPusher); ok {
		if strings.TrimSpace(hp.API) == "" || strings.TrimSpace(hp.JWT) == "" {
			return false, nil // no Farvoo endpoint configured — keep PENDING, do not burn attempts
		}
	}
	row, err := w.DB.ClaimNextOutbox(time.Now().UTC())
	if err == store.ErrNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := w.Pusher.PushInvoiceCopy(ctx, []byte(row.PayloadJSON)); err != nil {
		backoff := time.Duration(1<<min(row.Attempts, 6)) * time.Second
		next := time.Now().UTC().Add(backoff)
		_ = w.DB.MarkOutboxRetry(row.ID, row.Attempts, err.Error(), next, maxAttempts)
		return true, err
	}
	if err := w.DB.MarkOutboxSent(row.ID, time.Now().UTC()); err != nil {
		return true, err
	}
	return true, nil
}

// Loop polls until ctx cancelled. Not an interval poll of Farvoo — only local outbox flush.
func (w *Worker) Loop(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			ok, err := w.RunOnce(ctx)
			if err != nil {
				log.Printf("fiscal-sync: %v", err)
			}
			_ = ok
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
