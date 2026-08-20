package main

import "fmt"

func printToTarget(t printerTarget, data []byte) error {
	switch t.Scheme {
	case schemeTCP:
		return tcpPrint(t.TCPHostPort, data)
	case schemeWinspool:
		return winspoolPrint(t.WinspoolName, data)
	default:
		return fmt.Errorf("unknown printer scheme %q", t.Scheme)
	}
}
