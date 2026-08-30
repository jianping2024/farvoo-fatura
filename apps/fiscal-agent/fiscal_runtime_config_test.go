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
