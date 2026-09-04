package fiscalipc

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAgentMutexNameStable(t *testing.T) {
	const want = `Global\FarvooFiscalAgent-SingleInstance-v1`
	if AgentMutexName != want {
		t.Fatalf("AgentMutexName = %q want %q", AgentMutexName, want)
	}
}

func TestCommandOpenFiscalStable(t *testing.T) {
	if CommandOpenFiscal != "open-fiscal" {
		t.Fatalf("CommandOpenFiscal = %q", CommandOpenFiscal)
	}
}

// TestSoleSingleInstanceWritings locks the unique-path contract from the single-instance UX plan.
func TestSoleSingleInstanceWritings(t *testing.T) {
	agent := filepath.Join(moduleRoot(t), "apps", "fiscal-agent")

	si, err := os.ReadFile(filepath.Join(agent, "single_instance_windows.go"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(si)
	if strings.Count(s, "func exitAlreadyRunning") != 1 {
		t.Fatal("exitAlreadyRunning must be defined exactly once")
	}
	if strings.Contains(s, "messageBoxOK") || strings.Contains(s, "MessageBoxW") || strings.Contains(s, "instance_running") {
		t.Fatal("exitAlreadyRunning path must not show a dialog")
	}
	if strings.Count(s, "func acquireAgentSingleInstance") != 1 {
		t.Fatal("acquireAgentSingleInstance must be defined exactly once")
	}

	common, err := os.ReadFile(filepath.Join(agent, "single_instance_common.go"))
	if err != nil {
		t.Fatal(err)
	}
	csCommon := string(common)
	if strings.Count(csCommon, "func waitAcquireAgentSingleInstance(") != 1 {
		t.Fatal("waitAcquireAgentSingleInstance must be defined exactly once")
	}
	if strings.Count(csCommon, "func waitAcquireAgentSingleInstancePoll(") != 1 {
		t.Fatal("waitAcquireAgentSingleInstancePoll must be defined exactly once")
	}

	mainSrc, err := os.ReadFile(filepath.Join(agent, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	ms := string(mainSrc)
	restartIdx := strings.Index(ms, `os.Args[1] == "--restart-wait"`)
	if restartIdx < 0 {
		t.Fatal("main must handle --restart-wait")
	}
	rest := ms[restartIdx:]
	endMark := "runAgent(nil)\n\t\treturn"
	endIdx := strings.Index(rest, endMark)
	if endIdx < 0 {
		t.Fatal("--restart-wait must end with runAgent(nil); return")
	}
	restartBlock := rest[:endIdx+len(endMark)]
	if !strings.Contains(restartBlock, "waitAcquireAgentSingleInstance(") {
		t.Fatal("main --restart-wait must call waitAcquireAgentSingleInstance")
	}
	if strings.Contains(restartBlock, "guardMainAgentSingleInstance()") {
		t.Fatal("--restart-wait must not call guardMainAgentSingleInstance (use waitAcquire)")
	}
	if strings.Contains(restartBlock, "time.Sleep(1 * time.Second)") {
		t.Fatal("--restart-wait must not fixed-Sleep(1s); waitAcquire polls instead")
	}

	shell, err := os.ReadFile(filepath.Join(agent, "fiscal_shell_windows.go"))
	if err != nil {
		t.Fatal(err)
	}
	ss := string(shell)
	if strings.Count(ss, "func runFiscalCommand") != 1 {
		t.Fatal("runFiscalCommand must be defined exactly once")
	}
	if strings.Contains(ss, "log.Fatal") {
		t.Fatal("runFiscalCommand must not log.Fatal")
	}
	if strings.Count(ss, "RequestOpenFiscal()") != 2 {
		// Agent running + lost-race paths both call the sole IPC helper
		t.Fatalf("runFiscalCommand must call RequestOpenFiscal exactly twice (running + race), got %d", strings.Count(ss, "RequestOpenFiscal()"))
	}

	pipe, err := os.ReadFile(filepath.Join(agent, "internal", "fiscalipc", "pipe_windows.go"))
	if err != nil {
		t.Fatal(err)
	}
	ps := string(pipe)
	if strings.Count(ps, "func RequestOpenFiscal(") != 1 {
		t.Fatal("RequestOpenFiscal must be defined exactly once")
	}
	if !strings.Contains(ps, "openFiscalAttempts") {
		t.Fatal("RequestOpenFiscal must retry via openFiscalAttempts")
	}

	console, err := os.ReadFile(filepath.Join(agent, "console_windows.go"))
	if err != nil {
		t.Fatal(err)
	}
	cs := string(console)
	if strings.Contains(cs, "func toggle"+"ConsoleWindow") {
		t.Fatal("console toggle helper must be removed")
	}
	if strings.Contains(cs, "func showOrFocusConsoleWindow") {
		t.Fatal("showOrFocusConsoleWindow must be removed (no tray debug console)")
	}
	if !strings.Contains(cs, "func attachConsoleWindow") || !strings.Contains(cs, "func showConsoleWindow") {
		t.Fatal("CLI/-console must keep attachConsoleWindow + showConsoleWindow")
	}

	i18n, err := os.ReadFile(filepath.Join(agent, "ui_i18n.go"))
	if err != nil {
		t.Fatal(err)
	}
	i18nS := string(i18n)
	if strings.Contains(i18nS, "instance_running_") {
		t.Fatal("instance_running_* i18n keys must be removed")
	}
	if strings.Contains(i18nS, "menu_console") {
		t.Fatal("menu_console i18n keys must be removed")
	}

	clientMain, err := os.ReadFile(filepath.Join(agent, "cmd", "fiscal-client", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	cm := string(clientMain)
	if !strings.Contains(cm, "AcquireClientSingleInstance") || !strings.Contains(cm, "FocusExistingByTitle") {
		t.Fatal("Client main must gate on AcquireClientSingleInstance + FocusExistingByTitle")
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// .../apps/fiscal-agent/internal/fiscalipc → repo root
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}
