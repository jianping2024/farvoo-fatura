package main

import (
	"errors"
	"testing"
)

func TestTargetKey(t *testing.T) {
	if got := targetKey(printerTarget{Scheme: schemeTCP, TCPHostPort: "192.168.1.10:9100"}); got != "tcp:192.168.1.10:9100" {
		t.Fatalf("tcp key: %q", got)
	}
	if got := targetKey(printerTarget{Scheme: schemeWinspool, WinspoolName: "Kitchen"}); got != "winspool:Kitchen" {
		t.Fatalf("winspool key: %q", got)
	}
}

func TestPreparePrintUnknownScheme(t *testing.T) {
	err := preparePrint(printerTarget{Scheme: "ftp", Display: "ftp:x"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, errPrinterNotReady) {
		t.Fatalf("got %v", err)
	}
}
