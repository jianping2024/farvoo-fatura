package main

import (
	"strconv"
	"strings"
)

// agentVersionOlderThan reports whether current is strictly less than recommended (x.y.z).
func agentVersionOlderThan(current, recommended string) bool {
	cur, okCur := parseSemverTriple(current)
	rec, okRec := parseSemverTriple(recommended)
	if !okCur || !okRec {
		return strings.TrimSpace(current) != "" &&
			strings.TrimSpace(recommended) != "" &&
			strings.TrimSpace(current) != strings.TrimSpace(recommended)
	}
	if cur[0] != rec[0] {
		return cur[0] < rec[0]
	}
	if cur[1] != rec[1] {
		return cur[1] < rec[1]
	}
	return cur[2] < rec[2]
}

func parseSemverTriple(s string) ([3]int, bool) {
	var out [3]int
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	if s == "" {
		return out, false
	}
	parts := strings.Split(s, ".")
	if len(parts) < 1 {
		return out, false
	}
	for i := 0; i < 3; i++ {
		if i >= len(parts) {
			break
		}
		seg := parts[i]
		if i == 0 && strings.HasPrefix(seg, "print-agent-v") {
			seg = strings.TrimPrefix(seg, "print-agent-v")
		}
		n, err := strconv.Atoi(seg)
		if err != nil {
			return out, false
		}
		out[i] = n
	}
	return out, true
}
