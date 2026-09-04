package main

import (
	"os"
	"strings"
	"testing"
)

func TestSoleLanBindWritings(t *testing.T) {
	raw, err := os.ReadFile("fiscal_embed.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	if n := strings.Count(src, "func captureLanOpsEnvLocksOnce("); n != 1 {
		t.Fatalf("captureLanOpsEnvLocksOnce want 1, got %d", n)
	}
	if n := strings.Count(src, "func loopbackAdminURL("); n != 1 {
		t.Fatalf("loopbackAdminURL want 1, got %d", n)
	}
	if n := strings.Count(src, "func applyFiscalRuntimeFromConfig("); n != 1 {
		t.Fatalf("applyFiscalRuntimeFromConfig want 1, got %d", n)
	}
	if n := strings.Count(src, "func startEmbeddedFiscal("); n != 1 {
		t.Fatalf("startEmbeddedFiscal want 1, got %d", n)
	}
	if strings.Contains(src, "lanEnvLockedAllow") || strings.Contains(src, "lanEnvLockedBind") {
		t.Fatal("retired sticky lanEnvLocked* names must be gone")
	}
	// fiscalAdminBaseURL must route through loopbackAdminURL (not return embedded URL raw).
	idx := strings.Index(src, "func fiscalAdminBaseURL(")
	if idx < 0 {
		t.Fatal("missing fiscalAdminBaseURL")
	}
	body := src[idx:]
	end := strings.Index(body, "\nfunc ")
	if end > 0 {
		body = body[:end]
	}
	if !strings.Contains(body, "loopbackAdminURL(embeddedFiscalURL)") {
		t.Fatal("fiscalAdminBaseURL must call loopbackAdminURL(embeddedFiscalURL)")
	}
	if strings.Contains(body, "return embeddedFiscalURL") {
		t.Fatal("fiscalAdminBaseURL must not return raw embeddedFiscalURL (0.0.0.0 breaks WebView)")
	}
}
