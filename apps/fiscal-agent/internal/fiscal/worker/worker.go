package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	fiscalprint "farvoo-fiscal-agent/internal/fiscal/print"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

// Sink writes ESC/POS bytes (Memory for UAT; production uses PrintBytesFn).
type Sink interface {
	WriteESCPOS(jobID string, data []byte) error
}

// PrintBytesFn is the ONLY physical fiscal out path when set (Agent injects parsePrinterTarget+printToTarget).
type PrintBytesFn func(printerRaw string, data []byte) error

// StationPrintersFn returns live station_id → printer raw (tcp:… / winspool:…).
type StationPrintersFn func() map[string]string

// Worker claims local_print_jobs and renders via print.RenderESCPOS — ONLY print drain path.
type Worker struct {
	DB                *store.DB
	Sink              Sink // optional Memory capture / fiscal-local tests
	StationPrintersFn StationPrintersFn
	PrintBytesFn      PrintBytesFn
}

// RunOnce processes at most one PENDING job.
func (w *Worker) RunOnce(ctx context.Context) (bool, error) {
	_ = ctx
	jobID, stationID, payloadJSON, err := w.DB.ClaimNextPrintJob()
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
	if err := w.writeBytes(jobID, stationID, bytes); err != nil {
		_ = w.DB.CompletePrintJob(jobID, false, err.Error())
		return true, err
	}
	if err := w.DB.CompletePrintJob(jobID, true, ""); err != nil {
		return true, err
	}
	return true, nil
}

func (w *Worker) writeBytes(jobID, stationID string, data []byte) error {
	if w.PrintBytesFn != nil {
		sid := strings.TrimSpace(stationID)
		if sid == "" {
			return fmt.Errorf("worker: station_id required for fiscal print")
		}
		raw := ""
		if w.StationPrintersFn != nil {
			if m := w.StationPrintersFn(); m != nil {
				raw = strings.TrimSpace(m[sid])
			}
		}
		if raw == "" {
			return fmt.Errorf("worker: station %q not mapped in station_printers", sid)
		}
		if err := w.PrintBytesFn(raw, data); err != nil {
			return err
		}
		if w.Sink != nil {
			_ = w.Sink.WriteESCPOS(jobID, data)
		}
		return nil
	}
	if w.Sink != nil {
		return w.Sink.WriteESCPOS(jobID, data)
	}
	return fmt.Errorf("worker: print not configured")
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
