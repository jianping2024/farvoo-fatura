//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/getlantern/systray"
)

type trayRuntime struct {
	mu sync.Mutex
	// ctx/cancel: tray shell lifetime (exit cancels everything).
	ctx    context.Context
	cancel context.CancelFunc
	// workCancel: print loops only — rebind cancels this without killing tray HTTP.
	workCancel context.CancelFunc

	sess     *agentSession
	initErr  error
	initDone bool
	status   *agentStatus

	configureURL string
}

func (rt *trayRuntime) snapshot() (*agentSession, error, bool) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.sess, rt.initErr, rt.initDone
}

func (rt *trayRuntime) uiLocale() string {
	// Configure/setup write config.json; do not use stale sess.cfg from agent startup.
	return loadAgentUILocale()
}

func (rt *trayRuntime) syncConfigFromDisk() {
	sess, _, done := rt.snapshot()
	if !done || sess == nil {
		return
	}
	reloadAgentSessionConfig(sess)
}

// startTrayAgentWork cancels any prior work loop and starts runNotificationLoop under the shell ctx.
func (rt *trayRuntime) startTrayAgentWork(sess *agentSession) {
	if rt == nil || sess == nil {
		return
	}
	ensureFiscalStarted(rt.ctx, sess)
	rt.mu.Lock()
	if rt.workCancel != nil {
		rt.workCancel()
		rt.workCancel = nil
	}
	parent := rt.ctx
	rt.mu.Unlock()
	if parent == nil {
		parent = context.Background()
	}
	workCtx, workCancel := context.WithCancel(parent)
	rt.mu.Lock()
	rt.workCancel = workCancel
	rt.mu.Unlock()
	go runNotificationLoop(workCtx, sess, rt.status)
}

// rebindTrayAgentWork is the sole Connected re-pair path: reload disk config and restart
// work loops without killing the tray or :17892.
func (rt *trayRuntime) rebindTrayAgentWork() {
	sess, err, done := rt.snapshot()
	if !done || err != nil || sess == nil {
		return
	}
	log.Println("tray: pair updated — rebinding work loops (Realtime/polling)")
	reloadAgentSessionConfig(sess)
	ensureLocalPollController(sess)
	rt.status.set("Ready", "Connected to Mesa")
	rt.startTrayAgentWork(sess)
}

// onPairConfigSaved runs after /api/pair writes config. First-time pair (still bootstrapping)
// continues into Connected with the new file. Re-pair while Connected rebinds work in-process.
func (rt *trayRuntime) onPairConfigSaved() {
	_, err, done := rt.snapshot()
	if !done || err != nil {
		return
	}
	rt.rebindTrayAgentWork()
}

func runAgent(args []string) {
	if wantsFiscalStandalone(args) {
		runFiscalStandalone(args)
		return
	}
	if agentArgsWantConsole(args) {
		sess, _, err := initAgentSession(context.Background(), args)
		if err != nil {
			showConsoleWindow()
			log.Fatal(err)
		}
		ensureFiscalStarted(context.Background(), sess)
		runNotificationLoop(context.Background(), sess, nil)
		return
	}

	runAgentTrayFirst(args)
}

func runAgentTrayFirst(args []string) {
	initWindowsAgentLog()
	hideConsoleWindow()

	ctx, cancel := context.WithCancel(context.Background())
	rt := &trayRuntime{
		ctx:    ctx,
		status: &agentStatus{},
		cancel: cancel,
	}
	// Sole auto Realtime restore: same path as tray menu Restart (no confirm dialog).
	rt.status.setPromoteRestartHandler(func() {
		requestTrayRestart(rt)
	})
	onConfigureWizardReady = rt.rememberConfigureWizardURL
	defer func() { onConfigureWizardReady = nil }()
	rt.status.set("Starting", printAgentName)

	// Local /pair+/configure must be up before unpaired bootstrap and Dashboard probe (17892).
	startTrayLocalHTTP(rt)
	startFiscalIPC(ctx, rt.openFiscalShellFromTray)

	go func() {
		// Desktop "Farvoo 开票" cold start: open fiscal when HTTP is healthy — do not wait for Mesa Connected.
		if openFiscalOnTrayStart {
			openFiscalOnTrayStart = false
			rt.openFiscalShellFromTray()
		}
		rt.status.set("Setting up", "Complete pairing or printer mapping in the browser if it opened")
		sess, _, err := initAgentSession(ctx, args)
		rt.mu.Lock()
		rt.sess = sess
		rt.initErr = err
		rt.initDone = true
		rt.mu.Unlock()
		if err != nil {
			if errors.Is(err, context.Canceled) {
				log.Println("tray: init cancelled")
				return
			}
			rt.status.set("Error", err.Error())
			loc := loadAgentUILocale()
			messageBoxOK(uiT(loc, "about_title"), err.Error())
			requestTrayExit(rt)
			return
		}
		// Ready here means Connected (can accept jobs); yellow "Setting up" covered bootstrap.
		rt.status.set("Ready", "Connected to Mesa")
		log.Println("tray: Connected — accepting print jobs")
		rt.startTrayAgentWork(sess)
	}()

	runtime.LockOSThread()
	systray.Run(func() {
		onTrayReady(rt)
	}, func() {
		stopTrayAgentWork(rt)
		exitTrayAgent()
	})
	exitTrayAgent()
}

func onTrayReady(rt *trayRuntime) {
	log.Println("tray: ready (UI only; waiting for Connected)")
	var lastIcon trayLevel = -1
	applyTrayIcon := func() {
		lvl := rt.status.level()
		if lvl == lastIcon {
			return
		}
		lastIcon = lvl
		if icon := trayIconForLevel(lvl); len(icon) > 0 {
			systray.SetIcon(icon)
		}
	}
	applyTrayIcon()

	loc := rt.uiLocale()
	systray.SetTitle(uiT(loc, "tray_title"))
	systray.SetTooltip(rt.status.tooltip(Version, loc))
	go maybeNotifyTrayReady()
	go func() {
		time.Sleep(3 * time.Second)
		maybeNotifyCredentialRenewal()
	}()

	// Keep status enabled so the first tray line uses the same menu text color as peers
	// (Disable greys it out on Windows). Clicks are ignored — no ClickedCh handler.
	mStatus := systray.AddMenuItem(rt.status.menuStatusLine(loc), "")
	systray.AddSeparator()
	mSettings := systray.AddMenuItem(uiT(loc, "menu_settings"), uiT(loc, "menu_settings_tip"))
	mFiscal := systray.AddMenuItem(uiT(loc, "menu_fiscal"), uiT(loc, "menu_fiscal_tip"))
	mOpenLog := systray.AddMenuItem(uiT(loc, "menu_open_log"), uiT(loc, "menu_open_log_tip"))
	mOpenLogDir := systray.AddMenuItem(uiT(loc, "menu_open_log_dir"), uiT(loc, "menu_open_log_dir_tip"))
	systray.AddSeparator()
	mUILang := systray.AddMenuItem(uiT(loc, "menu_ui_locale"), uiT(loc, "menu_ui_locale_tip"))
	mLangZh := mUILang.AddSubMenuItem(uiLocaleOptionTitle(loc, "zh"), "")
	mLangEn := mUILang.AddSubMenuItem(uiLocaleOptionTitle(loc, "en"), "")
	mLangPt := mUILang.AddSubMenuItem(uiLocaleOptionTitle(loc, "pt"), "")
	systray.AddSeparator()
	mAbout := systray.AddMenuItem(uiT(loc, "menu_about"), uiT(loc, "menu_about_tip"))
	mRestart := systray.AddMenuItem(uiT(loc, "menu_restart"), uiT(loc, "menu_restart_tip"))
	mQuit := systray.AddMenuItem(uiT(loc, "menu_quit"), uiT(loc, "menu_quit_tip"))

	go func() {
		tick := time.NewTicker(2 * time.Second)
		defer tick.Stop()
		var lastLoc string
		for range tick.C {
			rt.syncConfigFromDisk()
			loc = rt.uiLocale()
			if loc != lastLoc {
				lastLoc = loc
				systray.SetTitle(uiT(loc, "tray_title"))
				applyTrayMenuLabels(mStatus, mSettings, mFiscal, mOpenLog, mOpenLogDir, mAbout, mRestart, mQuit, loc)
				applyTrayUILocaleSubmenu(mUILang, mLangZh, mLangEn, mLangPt, loc)
			}
			mStatus.SetTitle(rt.status.menuStatusLine(loc))
			tip := rt.status.tooltip(Version, loc)
			if cfg, err := loadConfig(defaultConfigPath()); err == nil && cfg != nil {
				if suffix := cfg.credentialStatusSuffix(loc, time.Now()); suffix != "" {
					tip += "\n" + suffix
				}
			}
			systray.SetTooltip(tip)
			applyTrayIcon()
		}
	}()

	applyTrayUILocaleChoice := func(code string) {
		cur := normalizeUILocale(rt.uiLocale())
		code = normalizeUILocale(code)
		if cur == code {
			return
		}
		if err := setAgentUILocale(code); err != nil {
			messageBoxOK(uiT(cur, "about_title"), err.Error())
			return
		}
		agentLogLocale(code, "log_tray_ui_locale", uiLocaleOptionLogLabel(code))
		loc = code
		systray.SetTitle(uiT(loc, "tray_title"))
		applyTrayMenuLabels(mStatus, mSettings, mFiscal, mOpenLog, mOpenLogDir, mAbout, mRestart, mQuit, loc)
		applyTrayUILocaleSubmenu(mUILang, mLangZh, mLangEn, mLangPt, loc)
		systray.SetTooltip(rt.status.tooltip(Version, loc))
	}

	go func() {
		for {
			select {
			case <-mLangZh.ClickedCh:
				applyTrayUILocaleChoice("zh")
			case <-mLangEn.ClickedCh:
				applyTrayUILocaleChoice("en")
			case <-mLangPt.ClickedCh:
				applyTrayUILocaleChoice("pt")
			case <-mSettings.ClickedCh:
				rt.startTrayConfigureWizard("")
			case <-mFiscal.ClickedCh:
				rt.openFiscalShellFromTray()
			case <-mOpenLog.ClickedCh:
				if err := openAgentLog(); err != nil {
					log.Println("tray:", err)
					loc := rt.uiLocale()
					messageBoxOK(uiT(loc, "about_title"), fmt.Sprintf(uiT(loc, "open_log_fail"), err.Error()))
				}
			case <-mOpenLogDir.ClickedCh:
				if err := openAgentLogFolder(); err != nil {
					log.Println("tray:", err)
					loc := rt.uiLocale()
					messageBoxOK(uiT(loc, "about_title"), fmt.Sprintf(uiT(loc, "open_log_dir_fail"), err.Error()))
				}
			case <-mAbout.ClickedCh:
				messageBoxOK(uiT(rt.uiLocale(), "about_title"), trayAboutText(rt, rt.uiLocale()))
			case <-mRestart.ClickedCh:
				loc := rt.uiLocale()
				if !confirmTrayRestart(loc) {
					continue
				}
				requestTrayRestart(rt)
				return
			case <-mQuit.ClickedCh:
				loc := rt.uiLocale()
				if !confirmTrayExit(loc) {
					continue
				}
				requestTrayExit(rt)
				return
			}
		}
	}()
}
