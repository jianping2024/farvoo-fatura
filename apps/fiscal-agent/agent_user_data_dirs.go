package main

import (
	"path/filepath"
	"strings"
)

func agentUserDataDirs() []string {
	var dirs []string
	cfg := defaultConfigPath()
	if cfg != "" {
		dirs = append(dirs, filepath.Dir(cfg))
	}
	if d := agentDataDir(); d != "" {
		dirs = append(dirs, d)
	}
	return uniqueNonEmptyStrings(dirs)
}

func uniqueNonEmptyStrings(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
