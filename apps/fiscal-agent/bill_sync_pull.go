package main

import (
	"context"
	"log"
)

// pullBillSyncsOnce is the ONLY Agent entry that pulls Farvoo bill_sync_jobs into SQLite.
// Used by Realtime doorbell/compensation and Polling fallback — never a second poll loop.
func pullBillSyncsOnce(ctx context.Context, cfg *config) {
	p := fiscalBillSyncPuller(cfg)
	if p == nil {
		return
	}
	n, err := p.PullAndIngest(ctx)
	if err != nil {
		log.Printf("bill-sync: pull failed: %v", err)
		return
	}
	if n > 0 {
		log.Printf("bill-sync: ingested %d job(s)", n)
	}
}
