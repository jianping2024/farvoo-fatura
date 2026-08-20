package main

import (
	"context"
	"log"
	"time"
)

// PollingNotifier implements Notifier using HTTP polling (fallback mode).
type PollingNotifier struct {
	config *config
	queue  *JobQueue
	pc     *pollController
}

// NewPollingNotifier creates a new polling notifier.
func NewPollingNotifier(cfg *config, queue *JobQueue, pc *pollController) *PollingNotifier {
	return &PollingNotifier{
		config: cfg,
		queue:  queue,
		pc:     pc,
	}
}

// Start begins the polling loop (blocks until context canceled).
func (p *PollingNotifier) Start(ctx context.Context) error {
	log.Println("Polling mode: starting")

	hadJobs := false

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		open, err := p.pc.scheduleOpen()
		if err != nil {
			log.Printf("Polling: schedule error: %v", err)
			select {
			case <-time.After(p.pc.sleepFor(pollPhaseError)):
			case <-ctx.Done():
				return ctx.Err()
			}
			continue
		}

		if !open {
			select {
			case <-time.After(p.pc.sleepFor(pollPhaseClosed)):
			case <-ctx.Done():
				return ctx.Err()
			}
			continue
		}

		if err := p.fetch(ctx); err != nil {
			log.Printf("Polling: fetch failed: %v", err)
			select {
			case <-time.After(p.pc.sleepFor(pollPhaseError)):
			case <-ctx.Done():
				return ctx.Err()
			}
			continue
		}

		queueLen := p.queue.Len()
		phase := p.pc.phase(queueLen > 0, false)

		if queueLen > 0 && !hadJobs {
			p.pc.markActivity()
			hadJobs = true
		} else if queueLen == 0 {
			hadJobs = false
		}

		select {
		case <-time.After(p.pc.sleepFor(phase)):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (p *PollingNotifier) fetch(ctx context.Context) error {
	jobs, err := fetchPending(ctx, p.config.APIBase, p.config.AgentJWT)
	if err != nil {
		return err
	}

	fetched, admitted := admitPendingJobs(p.config, p.queue, jobs, "Polling")
	if admitted > 0 {
		logCompensationSummary("Polling", fetched, admitted)
	}
	return nil
}
