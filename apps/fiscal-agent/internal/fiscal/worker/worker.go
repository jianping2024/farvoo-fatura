package worker

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	fiscalprint "farvoo-fiscal-agent/internal/fiscal/print"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

// Sink writes ESC/POS bytes (TCP printer, file, or memory for UAT).
type Sink interface {
	WriteESCPOS(jobID string, data []byte) error
}

// Worker claims local_print_jobs and renders via print.RenderESCPOS — ONLY print drain path.
type Worker struct {
	DB   *store.DB
	Sink Sink
}

// RunOnce processes at most one PENDING job.
func (w *Worker) RunOnce(ctx context.Context) (bool, error) {
	_ = ctx
	jobID, payloadJSON, err := w.DB.ClaimNextPrintJob()
	if errors.Is(err, store.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var payload fiscalprint.Payload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		_ = w.DB.CompletePrintJob(jobID, false, err.Error())
		return true, err
	}
	bytes := fiscalprint.RenderESCPOS(&payload)
	if err := w.Sink.WriteESCPOS(jobID, bytes); err != nil {
		_ = w.DB.CompletePrintJob(jobID, false, err.Error())
		return true, err
	}
	if err := w.DB.CompletePrintJob(jobID, true, ""); err != nil {
		return true, err
	}
	return true, nil
}

// Loop polls until ctx cancelled.
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
				log.Printf("fiscal print worker: %v", err)
			} else if ok {
				log.Printf("fiscal print worker: job printed")
			}
		}
	}
}

// MemorySink keeps the last ticket for UAT asserts.
type MemorySink struct {
	LastJobID string
	LastBytes []byte
}

func (m *MemorySink) WriteESCPOS(jobID string, data []byte) error {
	m.LastJobID = jobID
	m.LastBytes = append([]byte(nil), data...)
	return nil
}
