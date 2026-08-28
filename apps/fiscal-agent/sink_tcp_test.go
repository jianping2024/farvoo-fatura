package main

import (
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

func TestTcpPrintSingleConnection(t *testing.T) {
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
			_ = c.Close()
		}
	}()

	if err := preparePrint(printerTarget{Scheme: schemeTCP, TCPHostPort: addr, Display: addr}); err != nil {
		t.Fatalf("preparePrint tcp: %v", err)
	}
	payload := []byte{0x1B, 0x40, 'O', 'K', '\n'}
	if err := printToTarget(printerTarget{Scheme: schemeTCP, TCPHostPort: addr, Display: addr}, payload); err != nil {
		t.Fatalf("printToTarget: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for dials.Load() < 1 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := dials.Load(); got != 1 {
		t.Fatalf("tcp print dial count %d want 1 (preflight must not dial)", got)
	}
}
