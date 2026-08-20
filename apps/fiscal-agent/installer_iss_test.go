package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Sole upgrade story in mesa-print-agent.iss — fail if AppMutex / CloseApplications
// yes-no / lowest privilege reappears beside admin + PrepareToInstall taskkill.
func TestInnoSetupUpgradeStory(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("installer", "mesa-print-agent.iss"))
	if err != nil {
		t.Fatal(err)
	}
	iss := string(raw)

	mustContain := []string{
		"PrivilegesRequired=admin",
		"UsePreviousAppDir=yes",
		"CloseApplications=no",
		"Flags: ignoreversion restartreplace",
		"AppId={{" + mesaPrintAgentInnoGUID + "}}",
		"AppVerName={#MyAppName}",
		"UninstallDisplayName={#MyAppName}",
		"function PrepareToInstall(",
		"taskkill.exe",
		"/F /IM {#MyAppExe} /T",
	}
	for _, s := range mustContain {
		if !strings.Contains(iss, s) {
			t.Fatalf("installer missing required directive %q", s)
		}
	}
	if strings.Contains(iss, "PrivilegesRequired=lowest") {
		t.Fatal("PrivilegesRequired=lowest must not remain — admin is the sole Setup privilege path")
	}
	if strings.Contains(iss, "AppMutex=") {
		t.Fatal("AppMutex must not appear — it blocks Setup with please-close OK/Cancel")
	}
	if strings.Contains(iss, "CloseApplications=yes") || strings.Contains(iss, "CloseApplications=force") {
		t.Fatal("CloseApplications yes/force must not appear — that asks the user to close apps")
	}
	if strings.Count(iss, "PrivilegesRequired=") != 1 {
		t.Fatal("expected exactly one PrivilegesRequired= line")
	}
	if strings.Count(iss, "function PrepareToInstall(") != 1 {
		t.Fatal("expected exactly one PrepareToInstall — sole quiet-close path")
	}
}

func TestAgentMutexNameStable(t *testing.T) {
	const want = `Global\FarvooFiscalAgent-SingleInstance-v1`
	if agentMutexName != want {
		t.Fatalf("agentMutexName is tray single-instance only; changed to %q", agentMutexName)
	}
}
