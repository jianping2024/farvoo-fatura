package main

import (
	"log"
	"time"
)

// jobAgeSeconds returns how long the job has waited since created_at (cloud).
func jobAgeSeconds(job printJob) (int, bool) {
	t, ok := parseJobCreatedAt(job)
	if !ok {
		return 0, false
	}
	sec := int(time.Since(t).Seconds())
	if sec < 0 {
		sec = 0
	}
	return sec, true
}

// logJobEnqueue is the single admit log shape: source + id + type + created_at + age.
func logJobEnqueue(source string, job printJob) {
	if age, ok := jobAgeSeconds(job); ok {
		log.Printf("%s: enqueued job %s (type=%s, created_at=%s, age=%ds)",
			source, job.ID, job.Type, job.CreatedAt, age)
		return
	}
	log.Printf("%s: enqueued job %s (type=%s)", source, job.ID, job.Type)
}

// admitPendingJobs pushes eligible jobs into the local queue (one representation for
// Realtime compensation and polling fetch). Returns server fetch count and newly admitted.
func admitPendingJobs(cfg *config, queue *JobQueue, jobs []printJob, source string) (fetched, admitted int) {
	if cfg == nil || queue == nil {
		return len(jobs), 0
	}
	fetched = len(jobs)
	for _, job := range jobs {
		if !cfg.jobEligibleForQueue(job) {
			continue
		}
		if queue.Push(job) {
			admitted++
			logJobEnqueue(source, job)
		}
	}
	return fetched, admitted
}

func logCompensationSummary(source string, fetched, admitted int) {
	log.Printf("%s: compensation fetch fetched=%d newly_queued=%d", source, fetched, admitted)
}
