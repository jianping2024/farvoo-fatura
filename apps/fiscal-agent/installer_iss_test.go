package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"farvoo-fiscal-agent/internal/fiscalipc"
)

// Sole upgrade story in farvoo-fiscal-agent.iss — fail if AppMutex / CloseApplications
// yes-no / lowest privilege reappears beside admin + PrepareToInstall taskkill.
func TestInnoSetupUpgradeStory(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("installer", "farvoo-fiscal-agent.iss"))
	if err != nil {
		t.Fatal(err)
	}
	iss := string(raw)

	mustContain := []string{
		"PrivilegesRequired=admin",
		"UsePreviousAppDir=yes",
		"CloseApplications=no",
		"Flags: ignoreversion restartreplace",
		"AppId={{" + fiscalAgentInnoGUID + "}}",
		"AppVerName={#MyAppName} {#MyAppVersion}",
		"UninstallDisplayName={#MyAppName}",
		`#define MyAppName "Farvoo Fiscal Agent"`,
		`#define MyAppExe "FarvooFiscalAgent.exe"`,
		"OutputBaseFilename=FarvooFiscalAgent-Setup-amd64",
		"function PrepareToInstall(",
		"taskkill.exe",
		"/F /IM {#MyAppExe} /T",
		"/F /IM {#MyLegacyExe} /T",
		`#define MyLegacyExe "MesaPrintAgent.exe"`,
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
	if strings.Count(iss, "taskkill.exe") != 2 {
		t.Fatal("expected exactly two taskkill calls (current + legacy Mesa exe)")
	}
}

func TestClientInnoSetup(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("installer", "farvoo-fiscal-client.iss"))
	if err != nil {
		t.Fatal(err)
	}
	iss := string(raw)
	mustContain := []string{
		`#define MyAppExe "FarvooFiscalClient.exe"`,
		"OutputBaseFilename=FarvooFiscalClient-Setup-amd64",
		`Parameters: "--settings"`,
		"Tasks: webview2",
		"Name: \"webview2\"",
		"SetupIconFile=..\\assets\\app_icon.ico",
		"function NeedsWebView2",
		"Farvoo 开票",
	}
	for _, s := range mustContain {
		if !strings.Contains(iss, s) {
			t.Fatalf("client installer missing %q", s)
		}
	}
	// Inno [Tasks] has no "checked" flag (default is checked); "Flags: checked" fails ISCC.
	if strings.Contains(iss, "Flags: checked") {
		t.Fatal(`client installer must not use invalid Tasks flag "checked"`)
	}
}

func TestAgentInstallerFiscalShortcut(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("installer", "farvoo-fiscal-agent.iss"))
	if err != nil {
		t.Fatal(err)
	}
	iss := string(raw)
	for _, s := range []string{
		`Parameters: "fiscal"`,
		"desktopfiscal",
		"Tasks: webview2",
		"Name: \"webview2\"",
		"SetupIconFile=..\\assets\\app_icon.ico",
		"function NeedsWebView2",
	} {
		if !strings.Contains(iss, s) {
			t.Fatalf("agent installer missing %q", s)
		}
	}
	if strings.Contains(iss, "Flags: checked") {
		t.Fatal(`agent installer must not use invalid Tasks flag "checked"`)
	}
}

func TestAgentMutexNameStable(t *testing.T) {
	if agentMutexName != fiscalipc.AgentMutexName {
		t.Fatalf("agentMutexName must match fiscalipc.AgentMutexName")
	}
	if agentMutexName != `Global\FarvooFiscalAgent-SingleInstance-v1` {
		t.Fatalf("agentMutexName is tray single-instance only; changed to %q", agentMutexName)
	}
}

func TestFiscalAgentDisplayNamePrefixSole(t *testing.T) {
	if fiscalAgentDisplayNamePrefix != "Farvoo Fiscal Agent" {
		t.Fatalf("display prefix = %q", fiscalAgentDisplayNamePrefix)
	}
}
