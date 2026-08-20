package main

import (
	"fmt"
	"strings"
)

const (
	schemeTCP      = "tcp"
	schemeWinspool = "winspool"
)

type printerTarget struct {
	Scheme       string
	TCPHostPort  string
	WinspoolName string
	Display      string
}

func parsePrinterTarget(raw string) (printerTarget, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return printerTarget{}, fmt.Errorf("empty printer target")
	}
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, schemeTCP+":") {
		rest := strings.TrimSpace(s[len(schemeTCP)+1:])
		hp := normalizeHostPort(rest, "9100")
		if hp == "" {
			return printerTarget{}, fmt.Errorf("invalid tcp target %q", raw)
		}
		return printerTarget{Scheme: schemeTCP, TCPHostPort: hp, Display: schemeTCP + ":" + hp}, nil
	}
	if strings.HasPrefix(lower, schemeWinspool+":") {
		name := strings.TrimSpace(s[len(schemeWinspool)+1:])
		if name == "" {
			return printerTarget{}, fmt.Errorf("winspool target needs a printer name")
		}
		return printerTarget{Scheme: schemeWinspool, WinspoolName: name, Display: schemeWinspool + ":" + name}, nil
	}
	// Legacy LAN: host or host:port
	if strings.Contains(s, ":") {
		hp := normalizeHostPort(s, "9100")
		return printerTarget{Scheme: schemeTCP, TCPHostPort: hp, Display: hp}, nil
	}
	// Bare name → Windows queue (USB path); validated at print time on non-Windows.
	return printerTarget{Scheme: schemeWinspool, WinspoolName: s, Display: schemeWinspool + ":" + s}, nil
}

func (c *config) printerTargetForJob(job printJob) (printerTarget, error) {
	addr, err := c.printerAddrForJob(job)
	if err != nil {
		return printerTarget{}, err
	}
	return parsePrinterTarget(addr)
}

func (c *config) hasPrinterRouting() bool {
	return c.hasMappedStations()
}

func (c *config) hasMappedStations() bool {
	for _, v := range c.StationPrinters {
		if strings.TrimSpace(v) != "" {
			return true
		}
	}
	return false
}
