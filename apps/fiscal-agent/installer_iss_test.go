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
	if strings.Contains(iss, "fiscal-devtool") {
		t.Fatal("fiscal-devtool must not ship in Agent Setup")
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
		`Name: "desktopfiscal"; Description: "Create a Farvoo Fiscal (WebView2) shortcut on the desktop"; GroupDescription: "Desktop shortcut:"; Flags: unchecked`,
		"UsePreviousTasks=no",
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

// TestInnoLicenseFileSolePath: one LICENSE.txt; both Setups point at it; no second EULA copy.
func TestInnoLicenseFileSolePath(t *testing.T) {
	const want = "LicenseFile=LICENSE.txt"
	agentRaw, err := os.ReadFile(filepath.Join("installer", "farvoo-fiscal-agent.iss"))
	if err != nil {
		t.Fatal(err)
	}
	clientRaw, err := os.ReadFile(filepath.Join("installer", "farvoo-fiscal-client.iss"))
	if err != nil {
		t.Fatal(err)
	}
	agent, client := string(agentRaw), string(clientRaw)
	if strings.Count(agent, "LicenseFile=") != 1 || !strings.Contains(agent, want) {
		t.Fatalf("agent iss must have exactly one %q", want)
	}
	if strings.Count(client, "LicenseFile=") != 1 || !strings.Contains(client, want) {
		t.Fatalf("client iss must have exactly one %q", want)
	}
	licPath := filepath.Join("installer", "LICENSE.txt")
	lic, err := os.ReadFile(licPath)
	if err != nil {
		t.Fatalf("sole EULA missing at %s: %v", licPath, err)
	}
	body := string(lic)
	if !strings.Contains(body, "End User License Agreement") {
		t.Fatal("LICENSE.txt must be the Farvoo EULA")
	}
	if !strings.Contains(body, "Farvoo Fiscal Agent and Farvoo Fiscal Client") {
		t.Fatal("LICENSE.txt must cover Agent and Client in one text")
	}
	// No parallel EULA filenames beside the sole LICENSE.txt.
	entries, err := os.ReadDir("installer")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		lower := strings.ToLower(name)
		if name == "LICENSE.txt" {
			continue
		}
		if strings.Contains(lower, "license") || strings.Contains(lower, "eula") || strings.HasPrefix(lower, "licence") {
			t.Fatalf("extra license/EULA file %q — sole path is installer/LICENSE.txt", name)
		}
	}
	// wizard-before is ops notes only; must not duplicate a second accept-EULA track.
	before, err := os.ReadFile(filepath.Join("installer", "wizard-before.txt"))
	if err != nil {
		t.Fatal(err)
	}
	bs := strings.ToLower(string(before))
	for _, bad := range []string{"i accept", "end user license", "eula", "limitation of liability"} {
		if strings.Contains(bs, bad) {
			t.Fatalf("wizard-before.txt must not carry EULA text %q — LicenseFile only", bad)
		}
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
