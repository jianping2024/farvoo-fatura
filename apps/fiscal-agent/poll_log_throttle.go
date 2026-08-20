package main

import "time"

type pollLogThrottle struct {
	last map[string]time.Time
}

func (t *pollLogThrottle) allow(key string, every time.Duration) bool {
	if key == "" {
		return true
	}
	if t.last == nil {
		t.last = make(map[string]time.Time)
	}
	now := time.Now()
	if prev, ok := t.last[key]; ok && now.Sub(prev) < every {
		return false
	}
	t.last[key] = now
	return true
}
