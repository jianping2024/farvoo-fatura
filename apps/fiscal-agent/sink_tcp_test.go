package main

import (
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

func TestTcpPrintSingleConnectionPerJob(t *testing.T) {
	resetTCPPrintPoolForTest()
	t.Cleanup(resetTCPPrintPoolForTest)

	var dials atomic.Int32
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	addr := ln.Addr().String()

	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			dials.Add(1)
			_, _ = io.Copy(io.Discard, c)
			// Keep accepted sockets open — pool reuses them (no per-job close).
		}
	}()

	if err := preparePrint(printerTarget{Scheme: schemeTCP, TCPHostPort: addr, Display: addr}); err != nil {
		t.Fatalf("preparePrint tcp: %v", err)
	}
	payload := []byte{0x1B, 0x40, 'O', 'K', '\n'}
	target := printerTarget{Scheme: schemeTCP, TCPHostPort: addr, Display: addr}
	if err := printToTarget(target, payload); err != nil {
		t.Fatalf("printToTarget: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for dials.Load() < 1 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := dials.Load(); got != 1 {
		t.Fatalf("first tcp print dial count %d want 1", got)
	}
}

func TestTcpPrintReusesConnection(t *testing.T) {
	resetTCPPrintPoolForTest()
	t.Cleanup(resetTCPPrintPoolForTest)

	var dials atomic.Int32
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	addr := ln.Addr().String()

	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			dials.Add(1)
			_, _ = io.Copy(io.Discard, c)
		}
	}()

	target := printerTarget{Scheme: schemeTCP, TCPHostPort: addr, Display: addr}
	payload := []byte{0x1B, 0x40, 'A', '\n'}
	for i := 0; i < 3; i++ {
		if err := printToTarget(target, payload); err != nil {
			t.Fatalf("print %d: %v", i, err)
		}
	}

	deadline := time.Now().Add(2 * time.Second)
	for dials.Load() < 1 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := dials.Load(); got != 1 {
		t.Fatalf("three prints to same target: dial count %d want 1 (reuse)", got)
	}
}
