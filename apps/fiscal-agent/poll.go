package main

import (
	"time"
)

type pollConfig struct {
	IdleIntervalSec        int `json:"idle_interval_sec,omitempty"`
	BusyIntervalSec        int `json:"busy_interval_sec,omitempty"`        // claim-job retry
	AfterPrintIntervalSec  int `json:"after_print_interval_sec,omitempty"` // pause after each job before next pull
	WarmIntervalSec        int `json:"warm_interval_sec,omitempty"`
	WarmAfterActivitySec   int `json:"warm_after_activity_sec,omitempty"`
	ClosedCheckSec         int `json:"closed_check_sec,omitempty"`
	ErrorIntervalSec       int `json:"error_interval_sec,omitempty"`
	LegacyFixedIntervalSec int `json:"fixed_interval_sec,omitempty"` // if set, disables dynamic tiers
}

func defaultPollConfig() pollConfig {
	return pollConfig{
		IdleIntervalSec:       20,
		BusyIntervalSec:       5,
		AfterPrintIntervalSec: 5,
		WarmIntervalSec:       5,
		WarmAfterActivitySec:  1800,
		ClosedCheckSec:        60,
		ErrorIntervalSec:      5,
	}
}

func (p *pollConfig) normalized() pollConfig {
	d := defaultPollConfig()
	if p == nil {
		return d
	}
	out := d
	if p.IdleIntervalSec > 0 {
		out.IdleIntervalSec = p.IdleIntervalSec
	}
	if p.BusyIntervalSec > 0 {
		out.BusyIntervalSec = p.BusyIntervalSec
	}
	if p.AfterPrintIntervalSec > 0 {
		out.AfterPrintIntervalSec = p.AfterPrintIntervalSec
	}
	if p.WarmIntervalSec > 0 {
		out.WarmIntervalSec = p.WarmIntervalSec
	}
	if p.WarmAfterActivitySec > 0 {
		out.WarmAfterActivitySec = p.WarmAfterActivitySec
	}
	if p.ClosedCheckSec > 0 {
		out.ClosedCheckSec = p.ClosedCheckSec
	}
	if p.ErrorIntervalSec > 0 {
		out.ErrorIntervalSec = p.ErrorIntervalSec
	}
	if p.LegacyFixedIntervalSec > 0 {
		out.LegacyFixedIntervalSec = p.LegacyFixedIntervalSec
	}
	return out
}

type pollController struct {
	cfg      pollConfig
	schedule *scheduleConfig
	loc      *time.Location
	lastAct  time.Time
}

func newPollController(schedule *scheduleConfig, poll *pollConfig) (*pollController, error) {
	pc := &pollController{}
	if err := pc.applyRuntime(schedule, poll); err != nil {
		return nil, err
	}
	return pc, nil
}

// applyRuntime refreshes schedule/poll from the current in-memory config.
func (pc *pollController) applyRuntime(schedule *scheduleConfig, poll *pollConfig) error {
	if pc == nil {
		return nil
	}
	p := poll.normalized()
	pc.cfg = p
	pc.schedule = schedule
	if schedule != nil && schedule.enabled() {
		loc, err := schedule.location()
		if err != nil {
			return err
		}
		pc.loc = loc
	} else {
		pc.loc = nil
	}
	return nil
}

func (pc *pollController) now() time.Time {
	if pc.loc != nil {
		return time.Now().In(pc.loc)
	}
	return time.Now()
}

func (pc *pollController) markActivity() {
	pc.lastAct = pc.now()
}

func (pc *pollController) scheduleOpen() (bool, error) {
	if pc.schedule == nil || !pc.schedule.enabled() {
		return true, nil
	}
	return pc.schedule.activeAt(pc.now().In(pc.loc))
}

func (pc *pollController) closedSleep() (time.Duration, error) {
	if d := pc.cfg.ClosedCheckSec; d > 0 {
		wait := time.Duration(d) * time.Second
		if pc.schedule == nil || !pc.schedule.enabled() {
			return wait, nil
		}
		if until, open, err := pc.schedule.nextOpen(pc.now()); err != nil {
			return wait, err
		} else if open {
			return 0, nil
		} else if until > 0 && until < wait {
			return until + time.Second, nil
		}
		return wait, nil
	}
	return 60 * time.Second, nil
}

type pollPhase string

const (
	pollPhaseClosed     pollPhase = "closed"
	pollPhaseBusy       pollPhase = "busy"
	pollPhaseAfterPrint pollPhase = "after_print"
	pollPhaseWarm       pollPhase = "warm"
	pollPhaseIdle       pollPhase = "idle"
	pollPhaseError      pollPhase = "error"
)

func (pc *pollController) phase(hadPending bool, fetchErr bool) pollPhase {
	if fetchErr {
		return pollPhaseError
	}
	open, err := pc.scheduleOpen()
	if err != nil || !open {
		return pollPhaseClosed
	}
	if pc.cfg.LegacyFixedIntervalSec > 0 {
		return pollPhaseIdle
	}
	if hadPending {
		return pollPhaseBusy
	}
	if !pc.lastAct.IsZero() {
		warm := time.Duration(pc.cfg.WarmAfterActivitySec) * time.Second
		if warm > 0 && pc.now().Sub(pc.lastAct) < warm {
			return pollPhaseWarm
		}
	}
	return pollPhaseIdle
}

func (pc *pollController) sleepFor(phase pollPhase) time.Duration {
	if pc.cfg.LegacyFixedIntervalSec > 0 && phase != pollPhaseClosed && phase != pollPhaseError {
		return time.Duration(pc.cfg.LegacyFixedIntervalSec) * time.Second
	}
	switch phase {
	case pollPhaseClosed:
		d, _ := pc.closedSleep()
		return d
	case pollPhaseBusy:
		return time.Duration(pc.cfg.BusyIntervalSec) * time.Second
	case pollPhaseAfterPrint:
		return time.Duration(pc.cfg.AfterPrintIntervalSec) * time.Second
	case pollPhaseWarm:
		return time.Duration(pc.cfg.WarmIntervalSec) * time.Second
	case pollPhaseError:
		return time.Duration(pc.cfg.ErrorIntervalSec) * time.Second
	default:
		return time.Duration(pc.cfg.IdleIntervalSec) * time.Second
	}
}
