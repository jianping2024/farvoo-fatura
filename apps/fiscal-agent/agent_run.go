package main

import (
	"context"
	"log"
	"time"
)

// runNotificationLoop starts the agent with the configured notification mode.
func runNotificationLoop(ctx context.Context, sess *agentSession, status *agentStatus) {
	reloadAgentSessionConfig(sess)
	cfg := sess.cfg
	if cfg == nil {
		log.Fatal("config missing")
		return
	}

	ensureFiscalStarted(ctx, sess)

	mode := cfg.resolveNotificationMode()
	if mode == NotificationModeRealtime && !cfg.hasRealtimeSession() {
		log.Println("Realtime requested but session credentials missing; using polling")
		mode = NotificationModePolling
	}
	setNotifyMode(status, mode)
	log.Printf("Starting agent in %s mode", mode)

	queue := NewJobQueue()

	processor := NewJobProcessor(queue, sess, status)
	go func() {
		if err := processor.Start(ctx); err != nil && err != context.Canceled {
			log.Printf("Processor error: %v", err)
		}
	}()

	go func() {
		if err := runScheduleLoop(ctx, sess, queue, status); err != nil && err != context.Canceled {
			log.Printf("Schedule loop error: %v", err)
		}
	}()

	var notifier Notifier

	if mode == NotificationModeRealtime {
		rt, err := NewRealtimeNotifier(cfg, queue, sess.pc, sess.cfgPath)
		if err != nil {
			log.Printf("Realtime mode unavailable: %v, falling back to polling", err)
			mode = NotificationModePolling
			setNotifyMode(status, mode)
			status.markRealtimePollingFallback()
			notifier = NewPollingNotifier(cfg, queue, sess.pc)
		} else {
			notifier = rt
		}
	} else {
		notifier = NewPollingNotifier(cfg, queue, sess.pc)
	}

	go func() {
		if err := runHeartbeatLoop(ctx, sess, status); err != nil && err != context.Canceled {
			log.Printf("Heartbeat error: %v", err)
		}
	}()

	if err := notifier.Start(ctx); err != nil && err != context.Canceled {
		log.Printf("Notifier error: %v", err)

		if mode == NotificationModeRealtime {
			log.Println("Realtime failed, falling back to polling mode")
			mode = NotificationModePolling
			setNotifyMode(status, mode)
			status.markRealtimePollingFallback()
			notifier = NewPollingNotifier(cfg, queue, sess.pc)
			if err := notifier.Start(ctx); err != nil && err != context.Canceled {
				log.Printf("Polling also failed: %v", err)
			}
		}
	}
}

func setNotifyMode(status *agentStatus, mode NotificationMode) {
	if status != nil {
		status.setMode(mode)
	}
	log.Printf("Print notify mode: %s", mode)
}

// runScheduleLoop owns outside-hours tray state and clears the print queue when
// closed. Cloud schedule/poll is applied only at process start (restart to refresh).
func runScheduleLoop(ctx context.Context, sess *agentSession, queue *JobQueue, status *agentStatus) error {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	wasOpen := true
	first := true

	apply := func() {
		ensureLocalPollController(sess)
		cfg := sess.cfg
		if cfg == nil || sess.pc == nil {
			return
		}

		open, err := sess.pc.scheduleOpen()
		if err != nil {
			agentLogTech(cfg, "log_schedule_error", err.Error())
			status.setScheduleClosed(false, "")
			status.set("Schedule error", err.Error())
			return
		}

		if !open {
			detail := "Not polling"
			if wait, werr := sess.pc.closedSleep(); werr == nil && wait > 0 {
				detail = "Not polling until next window"
			}
			if n := queue.ClearPending(); n > 0 {
				log.Printf("Schedule: cleared %d queued job(s) (outside hours)", n)
			}
			status.setScheduleClosed(true, detail)
			if first || wasOpen {
				if wait, werr := sess.pc.closedSleep(); werr == nil {
					agentLog(cfg, "log_outside_schedule_sleep", wait.Round(time.Second))
				} else {
					agentLog(cfg, "log_outside_schedule")
				}
			}
			wasOpen = false
			first = false
			return
		}

		if !wasOpen {
			agentLog(cfg, "log_schedule_resume")
		}
		status.setScheduleClosed(false, "")
		wasOpen = true
		first = false
	}

	apply()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			apply()
		}
	}
}

// ensureLocalPollController keeps schedule/poll from the in-memory config (startup
// cloud merge + local file). Does not call the network.
func ensureLocalPollController(sess *agentSession) {
	if sess == nil {
		return
	}
	reloadAgentSessionConfig(sess)
	cfg := sess.cfg
	if cfg == nil {
		return
	}
	if sess.pc == nil {
		pc, err := newPollController(cfg.Schedule, cfg.Poll)
		if err != nil {
			log.Printf("Schedule: cannot create poll controller: %v", err)
			return
		}
		sess.pc = pc
		return
	}
	if err := sess.pc.applyRuntime(cfg.Schedule, cfg.Poll); err != nil {
		log.Printf("Schedule: runtime refresh failed: %v", err)
	}
}

// runHeartbeatLoop sends periodic heartbeats to the server.
func runHeartbeatLoop(ctx context.Context, sess *agentSession, status *agentStatus) error {
	send := func() {
		cfg := sess.cfg
		if cfg == nil {
			return
		}
		open := true
		if sess.pc != nil {
			if o, err := sess.pc.scheduleOpen(); err == nil {
				open = o
			}
		}
		mode := NotificationMode("")
		if status != nil {
			mode = status.notifyMode()
		}
		if err := postHeartbeat(ctx, cfg, open, &sess.hb, mode); err != nil {
			agentLogTech(cfg, "log_heartbeat_error", err.Error())
		}
	}

	send()
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			send()
		}
	}
}
