// Printer preflight before each job: TCP dial (LAN) or WinSpool OpenPrinter (USB).
// Do not use Windows PRINTER_STATUS_* bits — false positives on USB POS (see docs/print-agent-flow.zh.md §1).
package main

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

// errPrinterNotReady — preparePrint failed; keep job pending and retry later.
var errPrinterNotReady = errors.New("printer not ready; will retry")

func targetKey(t printerTarget) string {
	switch t.Scheme {
	case schemeTCP:
		return schemeTCP + ":" + t.TCPHostPort
	case schemeWinspool:
		return schemeWinspool + ":" + t.WinspoolName
	default:
		return t.Display
	}
}

// preparePrint checks the target is reachable before claiming/processing the job.
func preparePrint(target printerTarget) error {
	if err := targetCheckReady(target); err != nil {
		return fmt.Errorf("%w: %v", errPrinterNotReady, err)
	}
	return nil
}

func targetCheckReady(t printerTarget) error {
	switch t.Scheme {
	case schemeTCP:
		return tcpCheckReady(t.TCPHostPort)
	case schemeWinspool:
		return winspoolCheckReady(t.WinspoolName)
	default:
		return fmt.Errorf("unknown printer scheme %q", t.Scheme)
	}
}

func tcpCheckReady(hostPort string) error {
	hostPort = strings.TrimSpace(hostPort)
	if hostPort == "" {
		return fmt.Errorf("empty tcp host:port")
	}
	c, err := net.DialTimeout("tcp", hostPort, 2*time.Second)
	if err != nil {
		return fmt.Errorf("tcp printer %q not reachable: %w", hostPort, err)
	}
	_ = c.Close()
	return nil
}
