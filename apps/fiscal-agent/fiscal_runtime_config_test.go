package main

import (
	"os"
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

func TestApplyFiscalRuntimeFromConfig_LANFromConfig(t *testing.T) {
	t.Setenv("FISCAL_ALLOW_LAN", "")
	t.Setenv("FISCAL_BIND", "")
	_ = os.Unsetenv("FISCAL_ALLOW_LAN")
	_ = os.Unsetenv("FISCAL_BIND")
	on := true
	applyFiscalRuntimeFromConfig(&config{FiscalAllowLAN: &on})
	if os.Getenv("FISCAL_ALLOW_LAN") != "1" {
		t.Fatalf("allow: %q", os.Getenv("FISCAL_ALLOW_LAN"))
	}
	if os.Getenv("FISCAL_BIND") != "0.0.0.0:17880" {
		t.Fatalf("bind: %q", os.Getenv("FISCAL_BIND"))
	}
	if lanEnvLockedAllow || lanEnvLockedBind {
		t.Fatal("config path must not mark env locked")
	}
}

func TestApplyFiscalRuntimeFromConfig_LANOffDefault(t *testing.T) {
	t.Setenv("FISCAL_ALLOW_LAN", "")
	t.Setenv("FISCAL_BIND", "")
	_ = os.Unsetenv("FISCAL_ALLOW_LAN")
	_ = os.Unsetenv("FISCAL_BIND")
	applyFiscalRuntimeFromConfig(nil)
	if os.Getenv("FISCAL_ALLOW_LAN") != "0" {
		t.Fatalf("allow: %q", os.Getenv("FISCAL_ALLOW_LAN"))
	}
	if strings.TrimSpace(os.Getenv("FISCAL_BIND")) != "" {
		t.Fatalf("bind should stay unset when LAN off: %q", os.Getenv("FISCAL_BIND"))
	}
}

func TestApplyFiscalRuntimeFromConfig_LANEnvWins(t *testing.T) {
	t.Setenv("FISCAL_ALLOW_LAN", "1")
	t.Setenv("FISCAL_BIND", "10.0.0.5:17880")
	off := false
	applyFiscalRuntimeFromConfig(&config{FiscalAllowLAN: &off})
	if os.Getenv("FISCAL_ALLOW_LAN") != "1" {
		t.Fatalf("env allow: %q", os.Getenv("FISCAL_ALLOW_LAN"))
	}
	if os.Getenv("FISCAL_BIND") != "10.0.0.5:17880" {
		t.Fatalf("env bind: %q", os.Getenv("FISCAL_BIND"))
	}
	if !lanEnvLockedAllow || !lanEnvLockedBind {
		t.Fatal("pre-set env must lock")
	}
}
