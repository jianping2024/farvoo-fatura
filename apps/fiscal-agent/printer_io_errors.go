package main

import (
	"errors"
	"net"
	"strings"
)

// printerIOFailure is true when the target likely had a transport/spooler fault (not bad payload).
// Do not use Windows PRINTER_STATUS_* / JOB_STATUS_OFFLINE bits — they false-positive on USB POS.
func printerIOFailure(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errPrinterNotReady) {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) && (ne.Timeout() || ne.Temporary()) {
		return true
	}
	s := strings.ToLower(err.Error())
	for _, frag := range []string{
		"not reachable",
		"connection refused",
		"no route",
		"unreachable",
		"broken pipe",
		"reset by peer",
		"forcibly closed",
		"open printer",
		"writeprinter",
		"startdocprinter",
		"startpageprinter",
		"i/o timeout",
	} {
		if strings.Contains(s, frag) {
			return true
		}
	}
	return false
}
