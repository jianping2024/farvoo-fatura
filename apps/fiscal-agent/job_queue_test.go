package main

import (
	"testing"
	"time"
)

func TestJobQueueDedup(t *testing.T) {
	q := NewJobQueue()

	job := printJob{ID: "test-123", Type: "station_ticket", Status: "pending"}

	if !q.Push(job) {
		t.Fatal("First push should succeed")
	}

	if q.Push(job) {
		t.Fatal("Second push should be deduplicated")
	}

	if q.Len() != 1 {
		t.Fatalf("Expected queue length 1, got %d", q.Len())
	}

	popped, ok := q.Pop()
	if !ok {
		t.Fatal("Pop should succeed")
	}
	if popped.ID != job.ID {
		t.Fatalf("Expected job ID %s, got %s", job.ID, popped.ID)
	}

	if q.Len() != 0 {
		t.Fatalf("Expected queue length 0, got %d", q.Len())
	}

	_, ok = q.Pop()
	if ok {
		t.Fatal("Pop from empty queue should fail")
	}
}

func TestJobQueueFIFO(t *testing.T) {
	q := NewJobQueue()

	jobs := []printJob{
		{ID: "job-1", Type: "station_ticket", Status: "pending"},
		{ID: "job-2", Type: "order_receipt", Status: "pending"},
		{ID: "job-3", Type: "pre_bill", Status: "pending"},
	}

	for _, job := range jobs {
		if !q.Push(job) {
			t.Fatalf("Failed to push job %s", job.ID)
		}
	}

	for i, expectedJob := range jobs {
		popped, ok := q.Pop()
		if !ok {
			t.Fatalf("Pop %d should succeed", i)
		}
		if popped.ID != expectedJob.ID {
			t.Fatalf("Expected job ID %s, got %s", expectedJob.ID, popped.ID)
		}
	}
}

func TestJobQueueClearPendingAllowsReenqueue(t *testing.T) {
	q := NewJobQueue()
	job := printJob{ID: "closed-job", Type: "station_ticket", Status: "pending"}
	if !q.Push(job) {
		t.Fatal("push")
	}
	if q.ClearPending() != 1 {
		t.Fatal("expected 1 cleared")
	}
	if q.Len() != 0 {
		t.Fatal("queue should be empty")
	}
	if !q.Push(job) {
		t.Fatal("same job must enqueue again after ClearPending")
	}
}

func TestJobQueueForgetAllowsReenqueue(t *testing.T) {
	q := NewJobQueue()
	job := printJob{ID: "popped-job", Type: "order_receipt", Status: "pending"}
	if !q.Push(job) {
		t.Fatal("push")
	}
	got, ok := q.Pop()
	if !ok || got.ID != job.ID {
		t.Fatal("pop")
	}
	q.Forget(job.ID)
	if !q.Push(job) {
		t.Fatal("forgot id should allow re-push")
	}
}

func TestJobQueueRequeueAfterPop(t *testing.T) {
	q := NewJobQueue()
	job := printJob{ID: "retry-me", Type: "station_ticket", Status: "pending"}
	if !q.Push(job) {
		t.Fatal("push")
	}
	got, ok := q.Pop()
	if !ok || got.ID != job.ID {
		t.Fatal("pop")
	}
	// Push must fail while seen; Requeue is the retry path.
	if q.Push(job) {
		t.Fatal("push after pop must stay blocked by seen")
	}
	q.Requeue(job)
	if q.Len() != 1 {
		t.Fatalf("requeue len=%d", q.Len())
	}
	again, ok := q.Pop()
	if !ok || again.ID != job.ID {
		t.Fatal("pop after requeue")
	}
}

func TestJobQueueRequeueIdempotentWhenAlreadyQueued(t *testing.T) {
	q := NewJobQueue()
	job := printJob{ID: "once", Type: "station_ticket", Status: "pending"}
	q.Requeue(job)
	q.Requeue(job)
	if q.Len() != 1 {
		t.Fatalf("expected single entry, len=%d", q.Len())
	}
}

func TestJobQueueCleanOld(t *testing.T) {
	q := NewJobQueue()

	job := printJob{ID: "old-job", Type: "station_ticket", Status: "pending"}
	q.Push(job)

	q.mu.Lock()
	q.seen[job.ID] = time.Now().Add(-2 * time.Hour)
	q.mu.Unlock()

	for i := 0; i < 1001; i++ {
		q.Push(printJob{ID: "job-" + time.Now().String() + string(rune(i)), Status: "pending"})
	}

	q.mu.Lock()
	_, exists := q.seen[job.ID]
	q.mu.Unlock()

	if exists {
		t.Fatal("Old entry should have been cleaned")
	}
}
