package main

import (
	"fmt"
	"net"
	"time"
)

func tcpPrint(hostPort string, data []byte) error {
	c, err := net.DialTimeout("tcp", hostPort, 8*time.Second)
	if err != nil {
		return fmt.Errorf("%w: %w", errPrinterNotReady, err)
	}
	defer c.Close()
	_ = c.SetWriteDeadline(time.Now().Add(12 * time.Second))
	if _, err = c.Write(data); err != nil {
		return fmt.Errorf("%w: %w", errPrinterNotReady, err)
	}
	return nil
}
