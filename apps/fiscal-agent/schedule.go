package main

import (
	"fmt"
	"strings"
	"time"
)

// timeWindow is a half-open local-time interval [start, end).
// When end is after start on the clock, the window is same-calendar-day.
// When end is before start (e.g. 19:30–02:00), the window crosses midnight:
// open from start through end-of-day, then from midnight until end.
type timeWindow struct {
	Start string `json:"start"` // "HH:MM" or "HH:MM:SS"
	End   string `json:"end"`
}

type daySchedule struct {
	Windows []timeWindow `json:"windows"`
}

type scheduleConfig struct {
	Timezone  string       `json:"timezone,omitempty"`
	Weekday   *daySchedule `json:"weekday,omitempty"`
	Saturday  *daySchedule `json:"saturday,omitempty"`
	Sunday    *daySchedule `json:"sunday,omitempty"`
	Monday    *daySchedule `json:"monday,omitempty"`
	Tuesday   *daySchedule `json:"tuesday,omitempty"`
	Wednesday *daySchedule `json:"wednesday,omitempty"`
	Thursday  *daySchedule `json:"thursday,omitempty"`
	Friday    *daySchedule `json:"friday,omitempty"`
}

func (s *scheduleConfig) enabled() bool {
	if s == nil {
		return false
	}
	return s.Weekday != nil || s.Saturday != nil || s.Sunday != nil ||
		s.Monday != nil || s.Tuesday != nil || s.Wednesday != nil ||
		s.Thursday != nil || s.Friday != nil
}

func (s *scheduleConfig) location() (*time.Location, error) {
	tz := strings.TrimSpace(s.Timezone)
	if tz == "" {
		return time.Local, nil
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, fmt.Errorf("unknown time zone %s (rebuild agent image after code updates; needs time/tzdata or OS tzdata): %w", tz, err)
	}
	return loc, nil
}

func (s *scheduleConfig) windowsFor(t time.Time) []timeWindow {
	if s == nil {
		return nil
	}
	switch t.Weekday() {
	case time.Monday:
		if s.Monday != nil {
			return s.Monday.Windows
		}
	case time.Tuesday:
		if s.Tuesday != nil {
			return s.Tuesday.Windows
		}
	case time.Wednesday:
		if s.Wednesday != nil {
			return s.Wednesday.Windows
		}
	case time.Thursday:
		if s.Thursday != nil {
			return s.Thursday.Windows
		}
	case time.Friday:
		if s.Friday != nil {
			return s.Friday.Windows
		}
	case time.Saturday:
		if s.Saturday != nil {
			return s.Saturday.Windows
		}
	case time.Sunday:
		if s.Sunday != nil {
			return s.Sunday.Windows
		}
	}
	if s.Weekday != nil {
		return s.Weekday.Windows
	}
	return nil
}

func parseClock(s string) (hour, min, sec int, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, 0, fmt.Errorf("empty time")
	}
	parts := strings.Split(s, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, 0, 0, fmt.Errorf("invalid time %q", s)
	}
	var h, m, secVal int
	if _, err = fmt.Sscanf(parts[0], "%d", &h); err != nil {
		return 0, 0, 0, fmt.Errorf("invalid time %q", s)
	}
	if _, err = fmt.Sscanf(parts[1], "%d", &m); err != nil {
		return 0, 0, 0, fmt.Errorf("invalid time %q", s)
	}
	if len(parts) == 3 {
		if _, err = fmt.Sscanf(parts[2], "%d", &secVal); err != nil {
			return 0, 0, 0, fmt.Errorf("invalid time %q", s)
		}
	}
	if h < 0 || h > 23 || m < 0 || m > 59 || secVal < 0 || secVal > 59 {
		return 0, 0, 0, fmt.Errorf("invalid time %q", s)
	}
	return h, m, secVal, nil
}

func (s *scheduleConfig) activeAt(now time.Time) (bool, error) {
	if !s.enabled() {
		return true, nil
	}
	windows := s.windowsFor(now)
	for _, w := range windows {
		ok, err := windowContains(w, now)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

func windowContains(w timeWindow, now time.Time) (bool, error) {
	sh, sm, ss, err := parseClock(w.Start)
	if err != nil {
		return false, err
	}
	eh, em, es, err := parseClock(w.End)
	if err != nil {
		return false, err
	}
	start := time.Date(now.Year(), now.Month(), now.Day(), sh, sm, ss, 0, now.Location())
	end := time.Date(now.Year(), now.Month(), now.Day(), eh, em, es, 0, now.Location())
	if start.Equal(end) {
		return false, fmt.Errorf("window start and end must differ (%s–%s)", w.Start, w.End)
	}
	if end.After(start) {
		// Same calendar day: [start, end)
		return !now.Before(start) && now.Before(end), nil
	}
	// Crosses midnight: [start, next-midnight) ∪ [midnight, end)
	return !now.Before(start) || now.Before(end), nil
}

// nextOpen returns duration until the next window opens; zero if already open or no future window within 7 days.
func (s *scheduleConfig) nextOpen(now time.Time) (time.Duration, bool, error) {
	if !s.enabled() {
		return 0, true, nil
	}
	loc, err := s.location()
	if err != nil {
		return 0, false, err
	}
	now = now.In(loc)
	open, err := s.activeAt(now)
	if err != nil {
		return 0, false, err
	}
	if open {
		return 0, true, nil
	}
	for step := 0; step < 7*24*60; step++ {
		t := now.Add(time.Duration(step) * time.Minute)
		if ok, err := s.activeAt(t); err != nil {
			return 0, false, err
		} else if ok {
			return t.Sub(now), false, nil
		}
	}
	return 24 * time.Hour, false, nil
}
