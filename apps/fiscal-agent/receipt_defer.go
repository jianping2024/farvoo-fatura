package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const receiptPrintDeferWindow = 10 * time.Minute

// errReceiptPrintDeferred — job stays pending; agent retries until printer is mapped or window expires.
var errReceiptPrintDeferred = errors.New("receipt printer not ready; will retry within 10 minutes")

func parseJobCreatedAt(job printJob) (time.Time, bool) {
	raw := strings.TrimSpace(job.CreatedAt)
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999999Z07:00",
	} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func jobCreatedWithin(job printJob, within time.Duration) bool {
	t, ok := parseJobCreatedAt(job)
	if !ok {
		return true
	}
	return time.Since(t) <= within
}

func (c *config) firstMappedStation() (stationID, rawAddr string) {
	if c == nil || c.StationPrinters == nil {
		return "", ""
	}
	ids := c.mappedStationIDsSorted()
	if len(ids) == 0 {
		return "", ""
	}
	sid := ids[0]
	return sid, strings.TrimSpace(c.StationPrinters[sid])
}

func (c *config) firstMappedStationAddr() (string, bool) {
	addr := ""
	if c != nil {
		_, addr = c.firstMappedStation()
	}
	addr = strings.TrimSpace(addr)
	return addr, addr != ""
}

func (c *config) mappedStationIDsSorted() []string {
	var ids []string
	for id, addr := range c.StationPrinters {
		if strings.TrimSpace(id) == "" || strings.TrimSpace(addr) == "" {
			continue
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// jobEligibleForQueue is the single gate for Notifier admit (Realtime event,
// compensation fetch, and polling). Routable jobs and in-window receipt defer
// are admitted; permanent route errors stay out until the cloud row changes.
func (c *config) jobEligibleForQueue(job printJob) bool {
	_, err := c.printerTargetForJob(job)
	if err == nil {
		return true
	}
	return errors.Is(err, errReceiptPrintDeferred)
}

func (c *config) resolveReceiptRouting(job printJob, explicitID string) (string, error) {
	id := strings.TrimSpace(explicitID)
	if id != "" {
		return c.resolveReceiptPrinterID(id)
	}
	if addr, ok := c.firstMappedStationAddr(); ok {
		return addr, nil
	}
	if jobCreatedWithin(job, receiptPrintDeferWindow) {
		return "", errReceiptPrintDeferred
	}
	return "", fmt.Errorf("receipt_printer_id required (no station printers configured within 10 minutes)")
}
