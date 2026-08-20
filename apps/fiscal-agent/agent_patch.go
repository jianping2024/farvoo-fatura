package main

import (
	"context"
	"time"
)

func sleepOrCancel(ctx context.Context, d time.Duration) {
	if d <= 0 {
		select {
		case <-ctx.Done():
		default:
		}
		return
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

// patchJobStatus updates a job; logs and retries once on failure (avoids silent stuck processing).
func patchJobStatus(ctx context.Context, cfg *config, jobID string, patch map[string]any, action string) bool {
	if cfg == nil || jobID == "" {
		return false
	}
	try := func() error {
		return patchJob(ctx, cfg.APIBase, cfg.AgentJWT, jobID, patch)
	}
	err := try()
	if err == nil {
		return true
	}
	agentLogTech(cfg, "log_patch_job_failed", err.Error(), action, jobID)
	sleepOrCancel(ctx, time.Second)
	err2 := try()
	if err2 == nil {
		agentLog(cfg, "log_patch_job_retry_ok", action, jobID)
		return true
	}
	agentLogTech(cfg, "log_patch_job_retry_failed", err2.Error(), action, jobID)
	return false
}
