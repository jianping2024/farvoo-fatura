package main

import (
	"log"
	"sync"
	"time"
)

var (
	admitSkipMu       sync.Mutex
	admitSkipThrottle pollLogThrottle
)

// freshAdmitConfig is the ONLY config source for Notifier admit decisions: station
// mappings saved in Settings (or a re-pair) land on disk, while the notifier's own
// *config is a startup snapshot. Falls back to the snapshot if the file is unreadable.
func freshAdmitConfig(path string, fallback *config) *config {
	if path == "" {
		return fallback
	}
	if c, err := loadConfig(path); err == nil && c != nil {
		return c
	}
	return fallback
}

// admitJob is the ONLY admit gate + skip log (Realtime event, compensation, polling).
// A skipped job is logged (throttled per job) so "never printed" is visible.
func admitJob(cfg *config, job printJob, source string) bool {
	if cfg == nil {
		return false
	}
	err := cfg.jobAdmitSkipReason(job)
	if err == nil {
		return true
	}
	admitSkipMu.Lock()
	ok := admitSkipThrottle.allow("skip:"+job.ID, 60*time.Second)
	admitSkipMu.Unlock()
	if ok {
		log.Printf("%s: skip job %s (type=%s): %v", source, job.ID, job.Type, err)
	}
	return false
}

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
		if !admitJob(cfg, job, source) {
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
