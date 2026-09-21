package domain

import "testing"

func TestFiscalTerminalDisplayName(t *testing.T) {
	if got := FiscalTerminalDisplayName("吧台", "10.0.0.2"); got != "吧台" {
		t.Fatalf("label wins: %q", got)
	}
	if got := FiscalTerminalDisplayName("  ", "10.0.0.2"); got != "10.0.0.2" {
		t.Fatalf("empty label → IP: %q", got)
	}
	if got := FiscalTerminalDisplayName("", ""); got != "127.0.0.1" {
		t.Fatalf("fallback: %q", got)
	}
}
