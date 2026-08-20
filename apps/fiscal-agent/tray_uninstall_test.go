package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUniqueNonEmptyStrings(t *testing.T) {
	got := uniqueNonEmptyStrings([]string{" a ", "", "a", "b", " b "})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("got %#v", got)
	}
}

func TestAgentUserDataDirsIncludesConfigParent(t *testing.T) {
	prev := configPathOverride
	dir := t.TempDir()
	configPathOverride = filepath.Join(dir, "farvoo-fiscal-agent", "config.json")
	defer func() { configPathOverride = prev }()

	dirs := agentUserDataDirs()
	want := filepath.Join(dir, "farvoo-fiscal-agent")
	found := false
	for _, d := range dirs {
		if d == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected %q in %#v", want, dirs)
	}
}

func TestMesaPrintAgentUninstallRegistryKeyNames(t *testing.T) {
	keys := mesaPrintAgentUninstallRegistryKeyNames()
	if len(keys) < 1 {
		t.Fatal("expected at least one uninstall registry key")
	}
	// Verified on Will's 0.3.61 Setup machine: Inno AppId={{GUID}} → "{GUID}}_is1".
	wantPrimary := "{" + mesaPrintAgentInnoGUID + "}}_is1"
	if keys[0] != wantPrimary {
		t.Fatalf("primary key: got %q want %q", keys[0], wantPrimary)
	}
	if keys[0] == "{"+mesaPrintAgentInnoGUID+"}_is1" {
		t.Fatal("primary must be Inno double-brace form, not single-brace")
	}
}

func TestMesaPrintAgentUninstallDisplayNameMatch(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"Mesa Print Agent", true},
		{"Mesa Print Agent version 0.3.61", true},
		{"mesa print agent version 0.3.61", true},
		{"Mesa Print AgentX", false},
		{"Other App", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := mesaPrintAgentUninstallDisplayNameMatch(tc.in); got != tc.want {
			t.Fatalf("%q: got %v want %v", tc.in, got, tc.want)
		}
	}
}

func TestParseWindowsCommandLine(t *testing.T) {
	exe, args, err := parseWindowsCommandLine(`"C:\Program Files (x86)\Mesa Print Agent\unins000.exe" /SILENT`)
	if err != nil {
		t.Fatal(err)
	}
	if exe != `C:\Program Files (x86)\Mesa Print Agent\unins000.exe` {
		t.Fatalf("exe=%q", exe)
	}
	if args != "/SILENT" {
		t.Fatalf("args=%q", args)
	}

	exe, args, err = parseWindowsCommandLine(`"C:\Program Files (x86)\Mesa Print Agent\unins000.exe"`)
	if err != nil {
		t.Fatal(err)
	}
	if exe != `C:\Program Files (x86)\Mesa Print Agent\unins000.exe` || args != "" {
		t.Fatalf("exe=%q args=%q", exe, args)
	}
}

func TestEnsureInnoSilentArgs(t *testing.T) {
	if got := ensureInnoSilentArgs(""); got != "/SILENT" {
		t.Fatalf("empty: %q", got)
	}
	if got := ensureInnoSilentArgs("/SILENT"); got != "/SILENT" {
		t.Fatalf("already: %q", got)
	}
	if got := ensureInnoSilentArgs("/VERYSILENT"); got != "/VERYSILENT" {
		t.Fatalf("very: %q", got)
	}
}

func TestUninstallCommandBesideExecutable(t *testing.T) {
	dir := t.TempDir()
	unins := filepath.Join(dir, "unins000.exe")
	if err := os.WriteFile(unins, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// uninstallCommandBesideExecutable uses os.Executable(); we only assert the
	// helper shape via a local join matching production logic.
	got := `"` + unins + `"`
	if !filepath.IsAbs(unins) {
		t.Fatal("expected abs path")
	}
	exe, args, err := parseWindowsCommandLine(got)
	if err != nil || exe != unins || args != "" {
		t.Fatalf("parse beside: exe=%q args=%q err=%v", exe, args, err)
	}
}
