package main

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// tcpPrintPool keeps one open LAN socket per host:port so firmware does not re-print
// +EVENT=SOCKA_* on every job connect/disconnect.
type tcpPrintPool struct {
	mu    sync.Mutex
	conns map[string]net.Conn
}

var lanPrintPool tcpPrintPool

func (p *tcpPrintPool) print(hostPort string, data []byte) error {
	hostPort = strings.TrimSpace(hostPort)
	if hostPort == "" {
		return fmt.Errorf("%w: empty tcp host:port", errPrinterNotReady)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conns == nil {
		p.conns = make(map[string]net.Conn)
	}

	if c := p.conns[hostPort]; c != nil {
		if err := tcpWrite(c, data); err == nil {
			return nil
		}
		_ = c.Close()
		delete(p.conns, hostPort)
	}

	c, err := net.DialTimeout("tcp", hostPort, 8*time.Second)
	if err != nil {
		return fmt.Errorf("%w: %w", errPrinterNotReady, err)
	}
	if err := tcpWrite(c, data); err != nil {
		_ = c.Close()
		return err
	}
	p.conns[hostPort] = c
	return nil
}

func tcpWrite(c net.Conn, data []byte) error {
	deadline := time.Now().Add(12 * time.Second)
	_ = c.SetWriteDeadline(deadline)
	if _, err := c.Write(data); err != nil {
		return fmt.Errorf("%w: %w", errPrinterNotReady, err)
	}
	return nil
}

// resetTCPPrintPoolForTest closes cached LAN connections (tests only).
func resetTCPPrintPoolForTest() {
	lanPrintPool.mu.Lock()
	defer lanPrintPool.mu.Unlock()
	for host, c := range lanPrintPool.conns {
		_ = c.Close()
		delete(lanPrintPool.conns, host)
	}
}

func tcpPrint(hostPort string, data []byte) error {
	return lanPrintPool.print(hostPort, data)
}
