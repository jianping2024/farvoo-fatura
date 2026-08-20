package main

import (
	"os"
	"strings"
	"testing"
)

func TestApplyFiscalRuntimeFromConfig_DefaultsAllowProvision(t *testing.T) {
	t.Setenv("FISCAL_ALLOW_LOCAL_PROVISION", "")
	t.Setenv("FISCAL_AT_ENV", "")
	_ = os.Unsetenv("FISCAL_ALLOW_LOCAL_PROVISION")
	_ = os.Unsetenv("FISCAL_AT_ENV")

	applyFiscalRuntimeFromConfig(nil)
	if os.Getenv("FISCAL_ALLOW_LOCAL_PROVISION") != "1" {
		t.Fatalf("default provision: %q", os.Getenv("FISCAL_ALLOW_LOCAL_PROVISION"))
	}
	if os.Getenv("FISCAL_AT_ENV") != "mock" {
		t.Fatalf("default at env: %q", os.Getenv("FISCAL_AT_ENV"))
	}
}

func TestApplyFiscalRuntimeFromConfig_ConfigCanDisable(t *testing.T) {
	t.Setenv("FISCAL_ALLOW_LOCAL_PROVISION", "")
	t.Setenv("FISCAL_AT_ENV", "")
	_ = os.Unsetenv("FISCAL_ALLOW_LOCAL_PROVISION")
	_ = os.Unsetenv("FISCAL_AT_ENV")

	off := false
	applyFiscalRuntimeFromConfig(&config{
		FiscalAllowLocalProvision: &off,
		FiscalATEnv:               "test",
	})
	if os.Getenv("FISCAL_ALLOW_LOCAL_PROVISION") != "0" {
		t.Fatalf("disabled: %q", os.Getenv("FISCAL_ALLOW_LOCAL_PROVISION"))
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
