package main

import (
	"testing"
	"time"
)

func TestSchedulePirataWeekdayLunch(t *testing.T) {
	s := &scheduleConfig{
		Timezone: "Europe/Lisbon",
		Weekday: &daySchedule{Windows: []timeWindow{
			{Start: "12:00", End: "15:00"},
			{Start: "19:30", End: "23:00"},
		}},
	}
	loc, err := time.LoadLocation("Europe/Lisbon")
	if err != nil {
		t.Fatal(err)
	}
	// Wednesday 12:45
	wed := time.Date(2026, 5, 13, 12, 45, 0, 0, loc)
	open, err := s.activeAt(wed)
	if err != nil || !open {
		t.Fatalf("expected open at lunch, got open=%v err=%v", open, err)
	}
	// Wednesday 15:00 — between services
	mid := time.Date(2026, 5, 13, 15, 0, 0, 0, loc)
	open, err = s.activeAt(mid)
	if err != nil || open {
		t.Fatalf("expected closed mid-afternoon, got open=%v err=%v", open, err)
	}
	// Sunday 13:00 (uses weekday template)
	sun := time.Date(2026, 5, 17, 13, 0, 0, 0, loc)
	open, err = s.activeAt(sun)
	if err != nil || !open {
		t.Fatalf("expected open Sunday lunch via weekday, got open=%v err=%v", open, err)
	}
}

func TestScheduleOvernightDinner(t *testing.T) {
	s := &scheduleConfig{
		Timezone: "Europe/Lisbon",
		Weekday: &daySchedule{Windows: []timeWindow{
			{Start: "12:00", End: "15:00"},
			{Start: "19:30", End: "02:00"},
		}},
	}
	loc, err := time.LoadLocation("Europe/Lisbon")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		at   time.Time
		want bool
	}{
		{"before dinner", time.Date(2026, 5, 13, 18, 0, 0, 0, loc), false},
		{"dinner evening", time.Date(2026, 5, 13, 23, 30, 0, 0, loc), true},
		{"after midnight", time.Date(2026, 5, 14, 1, 30, 0, 0, loc), true},
		{"exactly end closed", time.Date(2026, 5, 14, 2, 0, 0, 0, loc), false},
		{"after close", time.Date(2026, 5, 14, 3, 0, 0, 0, loc), false},
		{"lunch still same-day", time.Date(2026, 5, 14, 12, 30, 0, 0, loc), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			open, err := s.activeAt(tc.at)
			if err != nil {
				t.Fatalf("activeAt: %v", err)
			}
			if open != tc.want {
				t.Fatalf("open=%v want %v at %s", open, tc.want, tc.at.Format(time.RFC3339))
			}
		})
	}
}

func TestWindowContainsRejectsEqualClocks(t *testing.T) {
	_, err := windowContains(timeWindow{Start: "19:30", End: "19:30"}, time.Now())
	if err == nil {
		t.Fatal("expected error for equal start/end")
	}
}

func TestPollPhaseWarm(t *testing.T) {
	pc := &pollController{
		cfg:     defaultPollConfig(),
		lastAct: time.Now().Add(-5 * time.Minute),
	}
	if pc.phase(false, false) != pollPhaseWarm {
		t.Fatalf("expected warm, got %s", pc.phase(false, false))
	}
	pc.lastAct = time.Now().Add(-2 * time.Hour)
	if pc.phase(false, false) != pollPhaseIdle {
		t.Fatalf("expected idle, got %s", pc.phase(false, false))
	}
}
