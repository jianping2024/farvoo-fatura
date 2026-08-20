package main

import (
	"errors"
	"fmt"
	"testing"
)

func TestPrinterIOFailure(t *testing.T) {
	if printerIOFailure(nil) {
		t.Fatal("nil is not IO failure")
	}
	if !printerIOFailure(errPrinterNotReady) {
		t.Fatal("errPrinterNotReady")
	}
	if !printerIOFailure(fmt.Errorf("wrap: %w", errPrinterNotReady)) {
		t.Fatal("wrapped errPrinterNotReady")
	}
	if !printerIOFailure(fmt.Errorf("tcp printer %q not reachable: dial tcp: connection refused", "1.2.3.4:9100")) {
		t.Fatal("connection refused")
	}
	if printerIOFailure(errors.New("invalid escpos payload length")) {
		t.Fatal("payload error is not IO")
	}
}
