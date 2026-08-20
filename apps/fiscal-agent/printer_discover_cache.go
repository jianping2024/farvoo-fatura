package main

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

const printerDiscoverCacheTTL = 30 * time.Second

type printerDiscoverSnapshot struct {
	tcp      []printerListEntry
	winspool []printerListEntry
	at       time.Time
}

var (
	printerDiscoverMu sync.Mutex
	printerDiscover   printerDiscoverSnapshot
)

func printersAPIForceRefresh(r *http.Request) bool {
	q := strings.TrimSpace(r.URL.Query().Get("refresh"))
	return q == "1" || strings.EqualFold(q, "true") || strings.EqualFold(q, "yes")
}

func discoverPrintersForAPI(forceRefresh bool) (tcp, winspool []printerListEntry, err error) {
	if !forceRefresh {
		printerDiscoverMu.Lock()
		if !printerDiscover.at.IsZero() && time.Since(printerDiscover.at) < printerDiscoverCacheTTL {
			tcp = append([]printerListEntry(nil), printerDiscover.tcp...)
			winspool = append([]printerListEntry(nil), printerDiscover.winspool...)
			printerDiscoverMu.Unlock()
			return tcp, winspool, nil
		}
		printerDiscoverMu.Unlock()
	}

	tcp, winspool, err = discoverAllPrinters(1200*time.Millisecond, 64)
	printerDiscoverMu.Lock()
	printerDiscover.tcp = append([]printerListEntry(nil), tcp...)
	printerDiscover.winspool = append([]printerListEntry(nil), winspool...)
	printerDiscover.at = time.Now()
	printerDiscoverMu.Unlock()
	return tcp, winspool, err
}
