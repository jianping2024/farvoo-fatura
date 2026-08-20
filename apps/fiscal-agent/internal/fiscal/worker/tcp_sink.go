package worker

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

// ResolveFiscalPrinterTCP is the ONLY resolver for fiscal receipt printer host:port.
// Order: FISCAL_PRINTER_TCP env → station_printers["fiscal_receipt_printer"] → strip tcp: prefix.
func ResolveFiscalPrinterTCP(stationPrinters map[string]string) string {
	if v := strings.TrimSpace(os.Getenv("FISCAL_PRINTER_TCP")); v != "" {
		return stripTCPPrefix(v)
	}
	if stationPrinters == nil {
		return ""
	}
	if v := strings.TrimSpace(stationPrinters["fiscal_receipt_printer"]); v != "" {
		return stripTCPPrefix(v)
	}
	return ""
}

func stripTCPPrefix(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(strings.ToLower(raw), "tcp:") {
		return strings.TrimSpace(raw[4:])
	}
	return raw
}

// TCPSink sends ESC/POS to a RAW TCP printer (e.g. :9100). ONLY network fiscal sink.
type TCPSink struct {
	Addr string // host:port
}

func (t *TCPSink) WriteESCPOS(jobID string, data []byte) error {
	_ = jobID
	if strings.TrimSpace(t.Addr) == "" {
		return fmt.Errorf("worker: fiscal printer TCP addr empty")
	}
	c, err := net.DialTimeout("tcp", t.Addr, 3*time.Second)
	if err != nil {
		return fmt.Errorf("worker: fiscal printer dial %s: %w", t.Addr, err)
	}
	defer c.Close()
	_ = c.SetWriteDeadline(time.Now().Add(8 * time.Second))
	if _, err = c.Write(data); err != nil {
		return fmt.Errorf("worker: fiscal printer write: %w", err)
	}
	return nil
}

// TeeSink writes to all sinks; first error wins (still attempts all for Memory capture).
type TeeSink struct {
	Sinks []Sink
}

func (t *TeeSink) WriteESCPOS(jobID string, data []byte) error {
	var first error
	for _, s := range t.Sinks {
		if s == nil {
			continue
		}
		if err := s.WriteESCPOS(jobID, data); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// NewFiscalPrintSink builds the print sink for Agent embed (TCP if resolved, else Memory-only).
func NewFiscalPrintSink(stationPrinters map[string]string, mem *MemorySink) Sink {
	addr := ResolveFiscalPrinterTCP(stationPrinters)
	if mem == nil {
		mem = &MemorySink{}
	}
	if addr == "" {
		return mem
	}
	return &TeeSink{Sinks: []Sink{mem, &TCPSink{Addr: addr}}}
}
