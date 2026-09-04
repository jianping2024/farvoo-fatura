package main

import (
	"os"
	"strings"
	"testing"
)

func TestSoleLanBindWritings(t *testing.T) {
	embedRaw, err := os.ReadFile("fiscal_embed.go")
	if err != nil {
		t.Fatal(err)
	}
	embed := string(embedRaw)
	lanRaw, err := os.ReadFile("ui_lan_config.go")
	if err != nil {
		t.Fatal(err)
	}
	lan := string(lanRaw)

	if n := strings.Count(lan, "func resolveFiscalListenBind("); n != 1 {
		t.Fatalf("resolveFiscalListenBind want 1, got %d", n)
	}
	if n := strings.Count(embed, "func loopbackAdminURL("); n != 1 {
		t.Fatalf("loopbackAdminURL want 1, got %d", n)
	}
	if n := strings.Count(embed, "func applyFiscalRuntimeFromConfig("); n != 1 {
		t.Fatalf("applyFiscalRuntimeFromConfig want 1, got %d", n)
	}
	if n := strings.Count(embed, "func startEmbeddedFiscal("); n != 1 {
		t.Fatalf("startEmbeddedFiscal want 1, got %d", n)
	}
	if n := strings.Count(lan, "func loadAgentFiscalAllowLANState("); n != 1 {
		t.Fatalf("loadAgentFiscalAllowLANState want 1, got %d", n)
	}
	if n := strings.Count(lan, "func setAgentFiscalAllowLAN("); n != 1 {
		t.Fatalf("setAgentFiscalAllowLAN want 1, got %d", n)
	}
	if n := strings.Count(lan, "func buildAgentLanAccessSnapshot("); n != 1 {
		t.Fatalf("buildAgentLanAccessSnapshot want 1, got %d", n)
	}
	if n := strings.Count(lan, "func agentLanAccessSet("); n != 1 {
		t.Fatalf("agentLanAccessSet want 1, got %d", n)
	}

	// Product path must not pipe LAN through env.
	for _, banned := range []string{
		"captureLanOpsEnvLocksOnce",
		"lanOpsLockedAllow",
		"lanOpsLockedBind",
		"Setenv(\"FISCAL_ALLOW_LAN\"",
		"Setenv(\"FISCAL_BIND\"",
		"Getenv(\"FISCAL_ALLOW_LAN\")",
		"Getenv(\"FISCAL_BIND\")",
	} {
		if strings.Contains(embed, banned) || strings.Contains(lan, banned) {
			t.Fatalf("banned LAN-env path still present: %s", banned)
		}
	}
	if !strings.Contains(embed, "resolveFiscalListenBind()") {
		t.Fatal("startEmbeddedFiscal must call resolveFiscalListenBind")
	}
	if !strings.Contains(embed, "AllowLAN:") {
		t.Fatal("bootstrap.Options must set AllowLAN from resolveFiscalListenBind")
	}

	idx := strings.Index(embed, "func fiscalAdminBaseURL(")
	if idx < 0 {
		t.Fatal("missing fiscalAdminBaseURL")
	}
	body := embed[idx:]
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
