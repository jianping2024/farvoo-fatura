package main

import (
	"fmt"
	"time"
)

var errPrintJobExpired = fmt.Errorf("print job expired (older than 10 minutes); not printed")

func jobPrintExpired(job printJob) bool {
	t, ok := parseJobCreatedAt(job)
	if !ok {
		return false
	}
	return time.Since(t) >= receiptPrintDeferWindow
}
