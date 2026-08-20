package main

import (
	"os"
	"path/filepath"
	"strings"
)

// mesaPrintAgentInnoGUID is the sole GUID body for installer AppId={{…}} in
// installer/mesa-print-agent.iss. Uninstall registry subkeys are derived only here.
const mesaPrintAgentInnoGUID = "A3B8F2E1-9C4D-4A2B-8E1F-0D5C6B7A8E9F"

// mesaPrintAgentDisplayNamePrefix is the sole product DisplayName stem (Inno may
// append " version X.Y.Z" via AppVerName).
const mesaPrintAgentDisplayNamePrefix = "Mesa Print Agent"

// mesaPrintAgentUninstallRegistryKeyNames returns Uninstall subkey names to try.
// Inno AppId={{GUID}} lands as "{GUID}}_is1" (extra closing brace; verified on
// 0.3.61 Setup). Also try "{GUID}_is1" for tolerance — both derived from one GUID.
func mesaPrintAgentUninstallRegistryKeyNames() []string {
	return []string{
		"{" + mesaPrintAgentInnoGUID + "}}_is1",
		"{" + mesaPrintAgentInnoGUID + "}_is1",
	}
}

func mesaPrintAgentUninstallDisplayNameMatch(display string) bool {
	d := strings.TrimSpace(display)
	if d == "" {
		return false
	}
	prefix := mesaPrintAgentDisplayNamePrefix
	if strings.EqualFold(d, prefix) {
		return true
	}
	return strings.HasPrefix(strings.ToLower(d), strings.ToLower(prefix)+" ")
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
