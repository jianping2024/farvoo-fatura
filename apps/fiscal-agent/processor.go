package main

import (
	"context"
	"errors"
	"log"
	"strconv"
	"time"
)

// JobProcessor processes print jobs from a queue.
type JobProcessor struct {
	queue   *JobQueue
	config  *config
	session *agentSession
	status  *agentStatus
}

// NewJobProcessor creates a new job processor.
func NewJobProcessor(queue *JobQueue, session *agentSession, status *agentStatus) *JobProcessor {
	return &JobProcessor{
		queue:   queue,
		session: session,
		status:  status,
	}
}

// Start begins processing jobs from the queue (blocks until context canceled).
func (p *JobProcessor) Start(ctx context.Context) error {
	log.Println("Job processor: starting")

	var throttle pollLogThrottle
	const waitLogEvery = 60 * time.Second

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		reloadAgentSessionConfig(p.session)
		p.config = p.session.cfg
		if p.config == nil {
			p.setStatus("Error", "config missing")
			sleepOrCancel(ctx, 5*time.Second)
			continue
		}

		job, ok := p.queue.Pop()
		if !ok {
			// Empty: Ready only sticks when open (closed tray owned by schedule loop).
			p.setStatus("Ready", "Waiting for jobs")
			sleepOrCancel(ctx, time.Second)
			continue
		}

		if p.session != nil && p.session.pc != nil {
			open, err := p.session.pc.scheduleOpen()
			if err != nil {
				p.setStatus("Schedule error", err.Error())
				p.queue.Forget(job.ID)
				p.queue.ClearPending()
				sleepOrCancel(ctx, 5*time.Second)
				continue
			}
			if !open {
				// Discard in-flight + remaining; schedule loop also clears and owns yellow tray.
				p.queue.Forget(job.ID)
				p.queue.ClearPending()
				sleepOrCancel(ctx, 5*time.Second)
				continue
			}
		}

		p.setStatus("Processing", strconv.Itoa(p.queue.Len()+1)+" job(s) in queue")

		if jobPrintExpired(job) {
			if patchJobStatus(ctx, p.config, job.ID, map[string]any{
				"status":        "failed",
				"error_message": errPrintJobExpired.Error(),
			}, "failed:expired") {
				agentLog(p.config, "log_skipped_expired", job.ID)
			} else {
				agentLog(p.config, "log_job_still_pending", job.ID, "failed:expired")
			}
			p.queue.Forget(job.ID)
			continue
		}

		target, err := p.config.printerTargetForJob(job)
		if err != nil {
			if errors.Is(err, errReceiptPrintDeferred) {
				if throttle.allow("receipt_deferred", waitLogEvery) {
					agentLog(p.config, "log_receipt_deferred")
				}
				p.setStatus("Waiting for receipt printer", "Map a station in Settings")
				p.queue.Requeue(job)
				sleepOrCancel(ctx, 5*time.Second)
				continue
			}

			if patchJobStatus(ctx, p.config, job.ID, map[string]any{
				"status":        "failed",
				"error_message": err.Error(),
			}, "failed:route") {
				agentLog(p.config, "log_route_error", job.ID, jobRouteStationID(job), err.Error())
			} else {
				agentLog(p.config, "log_job_still_pending", job.ID, "failed:route")
			}
			p.queue.Forget(job.ID)
			continue
		}

		if prepErr := preparePrint(target); prepErr != nil {
			if errors.Is(prepErr, errPrinterNotReady) {
				if throttle.allow("printer_not_ready:"+targetKey(target), waitLogEvery) {
					agentLog(p.config, "log_printer_not_ready", target.Display, jobRouteStationID(job), prepErr.Error())
				}
				p.setStatus("Waiting for printer", "Printer not ready; retrying")
				p.queue.Requeue(job)
				sleepOrCancel(ctx, 5*time.Second)
				continue
			}

			if patchJobStatus(ctx, p.config, job.ID, map[string]any{
				"status":        "failed",
				"error_message": prepErr.Error(),
			}, "failed:prepare") {
				agentLog(p.config, "log_prepare_print_error", job.ID, prepErr.Error())
			} else {
				agentLog(p.config, "log_job_still_pending", job.ID, "failed:prepare")
			}
			p.queue.Forget(job.ID)
			continue
		}

		agentLog(p.config, "log_printing_job", job.ID, jobRouteStationID(job), target.Display, job.Type)
		if age, ok := jobAgeSeconds(job); ok {
			agentLog(p.config, "log_job_queue_age", job.ID, age)
		}
		p.setStatus("Printing", summarizeJobPayload(job))

		printStarted := time.Now()
		// pending-jobs claim-on-fetch already sets processing; only PATCH when still pending
		// (older servers / race). Same-device processing re-claim is idempotent on the API.
		if job.Status == "" || job.Status == "pending" {
			if !patchJobStatus(ctx, p.config, job.ID, map[string]any{"status": "processing"}, "processing") {
				// Claim failed (network or state race): keep trying via queue, do not drop.
				p.queue.Requeue(job)
				sleepOrCancel(ctx, 5*time.Second)
				continue
			}
			agentLog(p.config, "log_job_claimed", job.ID)
		} else {
			agentLog(p.config, "log_job_claimed", job.ID)
		}

		data := escposFromJob(job)
		if err := printToTarget(target, data); err != nil {
			if patchJobStatus(ctx, p.config, job.ID, map[string]any{
				"status":        "failed",
				"error_message": err.Error(),
			}, "failed:print") {
				p.session.hb.recordPrint(false)
				agentLog(p.config, "log_print_failed", job.ID, jobRouteStationID(job), target.Display, err.Error())
			} else {
				agentLog(p.config, "log_print_failed_stuck", job.ID, target.Display)
			}
			p.setStatus("Print failed", err.Error())
			// Forget so Dashboard Retry (same id → pending) can Push again.
			p.queue.Forget(job.ID)
		} else {
			printDur := time.Since(printStarted).Round(time.Millisecond)
			if patchJobStatus(ctx, p.config, job.ID, map[string]any{"status": "done"}, "done") {
				agentLog(p.config, "log_printed_ok", target.Display, summarizeJobPayload(job), job.ID, printDur)
				p.session.hb.recordPrint(true)
				p.setStatus("Ready", "Last print OK")
			} else {
				agentLog(p.config, "log_print_ok_patch_stuck", job.ID, target.Display)
				p.session.hb.recordPrint(true)
			}
			p.queue.Forget(job.ID)
			p.status.notePollingFallbackPrintOK()
			tryPromoteRestartAfterPollingBatch(p.status, p.queue.Len())
		}

		if p.session.pc != nil {
			p.session.pc.markActivity()
		}
	}
}

func (p *JobProcessor) setStatus(summary, detail string) {
	if p.status != nil {
		p.status.set(summary, detail)
	}
}
