package main

import (
	"context"
	"log"
	"strings"

	"farvoo-fiscal-agent/internal/fiscal/fiscalclear"
)

// pullFiscalClearsOnce is the ONLY Agent entry that pulls Farvoo fiscal_clear_jobs and wipes test sales.
func pullFiscalClearsOnce(ctx context.Context, cfg *config) {
	p := fiscalClearPuller(cfg)
	if p == nil {
		return
	}
	n, err := p.PullAndClear(ctx)
	if err != nil {
		log.Printf("fiscal-clear: pull failed: %v", err)
		return
	}
	if n > 0 {
		log.Printf("fiscal-clear: wiped test sales for %d job(s)", n)
	}
}

func fiscalClearPuller(cfg *config) *fiscalclear.Puller {
	embeddedFiscalMu.Lock()
	defer embeddedFiscalMu.Unlock()
	if embeddedFiscal == nil || embeddedFiscal.DB == nil || cfg == nil {
		return nil
	}
	if strings.TrimSpace(cfg.APIBase) == "" || strings.TrimSpace(cfg.AgentJWT) == "" {
		return nil
	}
	db := embeddedFiscal.DB
	return &fiscalclear.Puller{
		APIBase: cfg.APIBase,
		JWT:     cfg.AgentJWT,
		Clear: func() error {
			return db.ClearTestFiscalSalesData()
		},
	}
}
