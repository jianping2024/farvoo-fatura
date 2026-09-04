package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyFiscalRuntimeFromConfig_DefaultsDenyLocalProvision(t *testing.T) {
	t.Setenv("FISCAL_ALLOW_LOCAL_PROVISION", "")
	t.Setenv("FISCAL_AT_ENV", "")
	_ = os.Unsetenv("FISCAL_ALLOW_LOCAL_PROVISION")
	_ = os.Unsetenv("FISCAL_AT_ENV")

	applyFiscalRuntimeFromConfig(nil)
	if os.Getenv("FISCAL_ALLOW_LOCAL_PROVISION") != "0" {
		t.Fatalf("default provision: %q", os.Getenv("FISCAL_ALLOW_LOCAL_PROVISION"))
	}
	if os.Getenv("FISCAL_AT_ENV") != "mock" {
		t.Fatalf("default at env: %q", os.Getenv("FISCAL_AT_ENV"))
	}
}

func TestApplyFiscalRuntimeFromConfig_ConfigCanEnable(t *testing.T) {
	t.Setenv("FISCAL_ALLOW_LOCAL_PROVISION", "")
	t.Setenv("FISCAL_AT_ENV", "")
	_ = os.Unsetenv("FISCAL_ALLOW_LOCAL_PROVISION")
	_ = os.Unsetenv("FISCAL_AT_ENV")

	on := true
	applyFiscalRuntimeFromConfig(&config{
		FiscalAllowLocalProvision: &on,
		FiscalATEnv:               "test",
	})
	if os.Getenv("FISCAL_ALLOW_LOCAL_PROVISION") != "1" {
		t.Fatalf("enabled: %q", os.Getenv("FISCAL_ALLOW_LOCAL_PROVISION"))
	}
	if os.Getenv("FISCAL_AT_ENV") != "test" {
		t.Fatalf("at env: %q", os.Getenv("FISCAL_AT_ENV"))
	}
}

func TestApplyFiscalRuntimeFromConfig_EnvWins(t *testing.T) {
	t.Setenv("FISCAL_ALLOW_LOCAL_PROVISION", "0")
	t.Setenv("FISCAL_AT_ENV", "prod")
	on := true
	applyFiscalRuntimeFromConfig(&config{
		FiscalAllowLocalProvision: &on,
		FiscalATEnv:               "mock",
	})
	if os.Getenv("FISCAL_ALLOW_LOCAL_PROVISION") != "0" {
		t.Fatalf("env should win: %q", os.Getenv("FISCAL_ALLOW_LOCAL_PROVISION"))
	}
	if got := strings.TrimSpace(os.Getenv("FISCAL_AT_ENV")); got != "prod" {
		t.Fatalf("env at: %q", got)
	}
}

func TestApplyFiscalRuntimeFromConfig_DoesNotTouchLANEnv(t *testing.T) {
	t.Setenv("FISCAL_ALLOW_LAN", "1")
	t.Setenv("FISCAL_BIND", "10.0.0.5:17880")
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	configPathOverride = path
	t.Cleanup(func() { configPathOverride = "" })
	off := false
	if err := saveConfig(path, &config{FiscalAllowLAN: &off}); err != nil {
		t.Fatal(err)
	}
	applyFiscalRuntimeFromConfig(nil)
	if os.Getenv("FISCAL_ALLOW_LAN") != "1" {
		t.Fatalf("must not rewrite LAN env: %q", os.Getenv("FISCAL_ALLOW_LAN"))
	}
	if os.Getenv("FISCAL_BIND") != "10.0.0.5:17880" {
		t.Fatalf("must not rewrite BIND env: %q", os.Getenv("FISCAL_BIND"))
	}
}

func TestResolveFiscalListenBind_FromDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	configPathOverride = path
	t.Cleanup(func() { configPathOverride = "" })

	allow, bind := resolveFiscalListenBind()
	if allow || bind != fiscalListenLoopback {
		t.Fatalf("default: allow=%v bind=%q", allow, bind)
	}

	on := true
	if err := saveConfig(path, &config{FiscalAllowLAN: &on}); err != nil {
		t.Fatal(err)
	}
	allow, bind = resolveFiscalListenBind()
	if !allow || bind != fiscalListenLAN {
		t.Fatalf("lan on: allow=%v bind=%q", allow, bind)
	}

	// Stale Machine-style env must not affect Agent bind.
	t.Setenv("FISCAL_ALLOW_LAN", "0")
	t.Setenv("FISCAL_BIND", "127.0.0.1:17880")
	allow, bind = resolveFiscalListenBind()
	if !allow || bind != fiscalListenLAN {
		t.Fatalf("disk wins over env: allow=%v bind=%q", allow, bind)
	}

	off := false
	if err := saveConfig(path, &config{FiscalAllowLAN: &off}); err != nil {
		t.Fatal(err)
	}
	allow, bind = resolveFiscalListenBind()
	if allow || bind != fiscalListenLoopback {
		t.Fatalf("lan off: allow=%v bind=%q", allow, bind)
	}
}

func TestLoopbackAdminURL(t *testing.T) {
	if got := loopbackAdminURL("http://0.0.0.0:17880"); got != "http://127.0.0.1:17880" {
		t.Fatalf("got %q", got)
	}
	if got := loopbackAdminURL("http://127.0.0.1:17880"); got != "http://127.0.0.1:17880" {
		t.Fatalf("loopback passthrough: %q", got)
	}
	if got := loopbackAdminURL("http://10.0.0.5:17880"); got != "http://10.0.0.5:17880" {
		t.Fatalf("specific IP: %q", got)
	}
}
