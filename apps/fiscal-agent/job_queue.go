package main

import (
	"sync"
	"time"
)

// JobQueue is a thread-safe FIFO queue with deduplication.
//
// Lifecycle (one representation):
//   - Push: admit a job the notifier just discovered (no-op if already seen).
//   - Pop: take the head for processing (seen entry remains until terminal or Requeue).
//   - Requeue: after Pop, put the same job back for a later attempt (temporary hold).
//   - Forget: drop the seen barrier so a later Push can admit the same id again
//     (Dashboard Retry, schedule reopen, or after a terminal outcome).
type JobQueue struct {
	mu    sync.Mutex
	queue []printJob
	seen  map[string]time.Time // job_id -> first seen / last requeue time
}

// NewJobQueue creates an empty job queue.
func NewJobQueue() *JobQueue {
	return &JobQueue{
		queue: []printJob{},
		seen:  make(map[string]time.Time),
	}
}

// Push adds a job to the queue if not already seen.
// Returns true if added, false if duplicate.
func (q *JobQueue) Push(job printJob) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	if job.ID == "" {
		return false
	}
	if _, exists := q.seen[job.ID]; exists {
		return false
	}

	q.seen[job.ID] = time.Now()
	q.queue = append(q.queue, job)

	if len(q.seen) > 1000 {
		q.cleanOld()
	}

	return true
}

// Requeue returns a previously Pop'd job to the queue for another attempt.
// Unlike Push, an existing seen entry is expected and does not block re-entry.
func (q *JobQueue) Requeue(job printJob) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if job.ID == "" {
		return
	}
	for _, j := range q.queue {
		if j.ID == job.ID {
			q.seen[job.ID] = time.Now()
			return
		}
	}
	q.seen[job.ID] = time.Now()
	q.queue = append(q.queue, job)
}

// Pop removes and returns the first job from the queue.
// Returns zero value if empty.
func (q *JobQueue) Pop() (printJob, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.queue) == 0 {
		return printJob{}, false
	}

	job := q.queue[0]
	q.queue = q.queue[1:]
	return job, true
}

// Len returns the current queue length.
func (q *JobQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.queue)
}

// ClearPending drops every queued job and forgets their dedupe entries so the
// same pending jobs can be enqueued again after business hours resume.
func (q *JobQueue) ClearPending() int {
	q.mu.Lock()
	defer q.mu.Unlock()

	n := len(q.queue)
	for _, job := range q.queue {
		delete(q.seen, job.ID)
	}
	q.queue = q.queue[:0]
	return n
}

// Forget removes a job id from the dedupe map (e.g. a Pop'd job discarded while closed).
func (q *JobQueue) Forget(id string) {
	if id == "" {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.seen, id)
}

// cleanOld removes entries older than 1 hour (called under lock).
func (q *JobQueue) cleanOld() {
	cutoff := time.Now().Add(-1 * time.Hour)
	for id, t := range q.seen {
		if t.Before(cutoff) {
			delete(q.seen, id)
		}
	}
}
