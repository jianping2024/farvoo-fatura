package main

import (
	"os"
	"path/filepath"
	"strings"
)

// fiscalAgentInnoGUID is the sole GUID body for installer AppId={{…}} in
// installer/farvoo-fiscal-agent.iss. Uninstall registry subkeys are derived only here.
const fiscalAgentInnoGUID = "A3B8F2E1-9C4D-4A2B-8E1F-0D5C6B7A8E9F"

// fiscalAgentDisplayNamePrefix is the sole product DisplayName stem (Inno may
// append " version X.Y.Z" via AppVerName).
const fiscalAgentDisplayNamePrefix = "Farvoo Fiscal Agent"

// legacyMesaPrintAgentDisplayNamePrefix is accepted only for uninstall lookup
// during the Mesa → Farvoo rename migration window.
const legacyMesaPrintAgentDisplayNamePrefix = "Mesa Print Agent"

// fiscalAgentUninstallRegistryKeyNames returns Uninstall subkey names to try.
// Inno AppId={{GUID}} lands as "{GUID}}_is1" (extra closing brace; verified on
// 0.3.61 Setup). Also try "{GUID}_is1" for tolerance — both derived from one GUID.
func fiscalAgentUninstallRegistryKeyNames() []string {
	return []string{
		"{" + fiscalAgentInnoGUID + "}}_is1",
		"{" + fiscalAgentInnoGUID + "}_is1",
	}
}

func displayNameMatchesPrefix(display, prefix string) bool {
	d := strings.TrimSpace(display)
	if d == "" || strings.TrimSpace(prefix) == "" {
		return false
	}
	if strings.EqualFold(d, prefix) {
		return true
	}
	return strings.HasPrefix(strings.ToLower(d), strings.ToLower(prefix)+" ")
}

func fiscalAgentUninstallDisplayNameMatch(display string) bool {
	return displayNameMatchesPrefix(display, fiscalAgentDisplayNamePrefix) ||
		displayNameMatchesPrefix(display, legacyMesaPrintAgentDisplayNamePrefix)
}

// parseWindowsCommandLine splits an UninstallString / QuietUninstallString into
// exe path and remaining args (handles quoted paths with spaces).
func parseWindowsCommandLine(cmdline string) (exe string, args string, err error) {
	cmdline = strings.TrimSpace(cmdline)
	if cmdline == "" {
		return "", "", os.ErrInvalid
	}
	if strings.HasPrefix(cmdline, `"`) {
		rest := cmdline[1:]
		end := strings.IndexByte(rest, '"')
		if end < 0 {
			return "", "", os.ErrInvalid
		}
		exe = rest[:end]
		args = strings.TrimSpace(rest[end+1:])
		if strings.TrimSpace(exe) == "" {
			return "", "", os.ErrInvalid
		}
		return exe, args, nil
	}
	sp := strings.IndexByte(cmdline, ' ')
	if sp < 0 {
		return cmdline, "", nil
	}
	return cmdline[:sp], strings.TrimSpace(cmdline[sp+1:]), nil
}

func ensureInnoSilentArgs(args string) string {
	u := strings.ToUpper(args)
	if strings.Contains(u, "/SILENT") || strings.Contains(u, "/VERYSILENT") {
		return args
	}
	if strings.TrimSpace(args) == "" {
		return "/SILENT"
	}
	return args + " /SILENT"
}

// uninstallCommandBesideExecutable returns a command line for unins000.exe next to
// this process image, or "" if absent (portable zip / non-Setup).
func uninstallCommandBesideExecutable() string {
	self, err := os.Executable()
	if err != nil {
		return ""
	}
	unins := filepath.Join(filepath.Dir(self), "unins000.exe")
	st, err := os.Stat(unins)
	if err != nil || st.IsDir() {
		return ""
	}
	return `"` + unins + `"`
}
