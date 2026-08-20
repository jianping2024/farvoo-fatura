//go:build !windows

package main

import "fmt"

func winspoolPrint(printerName string, data []byte) error {
	return fmt.Errorf("USB/Windows printer queues are only supported on Windows (got %q)", printerName)
}

func listWinspoolPrinterNames() ([]string, error) {
	return nil, nil
}

func winspoolCheckReady(printerName string) error {
	return fmt.Errorf("USB/Windows printer queues are only supported on Windows (got %q)", printerName)
}
