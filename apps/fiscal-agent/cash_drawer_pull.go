package main

import (
	"context"
	"log"
	"strings"

	"farvoo-fiscal-agent/internal/fiscal/cashdrawer"
)

// pullCashDrawersOnce is the ONLY Agent entry that pulls Farvoo cash_drawer_jobs and kicks.
// Used by Realtime doorbell/compensation and Polling fallback — never a second poll loop.
func pullCashDrawersOnce(ctx context.Context, cfg *config) {
	p := fiscalCashDrawerPuller(cfg)
	if p == nil {
		return
	}
	n, err := p.PullAndKick(ctx)
	if err != nil {
		log.Printf("cash-drawer: pull failed: %v", err)
		return
	}
	if n > 0 {
		log.Printf("cash-drawer: kicked %d job(s)", n)
	}
}

func fiscalCashDrawerPuller(cfg *config) *cashdrawer.Puller {
	embeddedFiscalMu.Lock()
	defer embeddedFiscalMu.Unlock()
	if embeddedFiscal == nil || embeddedFiscal.OpenCashDrawer == nil || cfg == nil {
		return nil
	}
	if strings.TrimSpace(cfg.APIBase) == "" || strings.TrimSpace(cfg.AgentJWT) == "" {
		return nil
	}
	kick := embeddedFiscal.OpenCashDrawer
	return &cashdrawer.Puller{
		APIBase: cfg.APIBase,
		JWT:     cfg.AgentJWT,
		Kick:    kick,
	}
}
