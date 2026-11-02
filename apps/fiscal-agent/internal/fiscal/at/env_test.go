package at

import (
	"context"
	"strings"
	"testing"
)

func TestParseEnv(t *testing.T) {
	ok := map[string]Env{"": EnvMock, " mock ": EnvMock, "TEST": EnvTest, "Prod": EnvProd}
	for in, want := range ok {
		got, err := ParseEnv(in)
		if err != nil || got != want {
			t.Fatalf("ParseEnv(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"production", "prd", "tset", "live"} {
		if _, err := ParseEnv(in); err == nil {
			t.Fatalf("ParseEnv(%q) must fail closed", in)
		}
	}
}

func TestNewFromEnv_InvalidFailsClosed(t *testing.T) {
	t.Setenv("FISCAL_AT_ENV", "production")
	if _, err := NewFromEnv().RegisterSeries(context.Background(), RegisterRequest{}); err == nil || !strings.Contains(err.Error(), "invalid fiscal_at_env") {
		t.Fatalf("series: want invalid env error, got %v", err)
	}
	if _, err := NewTransportFromEnv().RegisterDocument(context.Background(), TransportDocument{DocumentNumber: "GR A/1"}); err == nil || !strings.Contains(err.Error(), "invalid fiscal_at_env") {
		t.Fatalf("transport: want invalid env error, got %v", err)
	}
}

func TestNewFromEnv_TrimmedEnvStored(t *testing.T) {
	t.Setenv("FISCAL_AT_ENV", " Test ")
	c, ok := NewFromEnv().(*SOAPClient)
	if !ok || c.Env != EnvTest {
		t.Fatalf("got %#v", c)
	}
	if _, ok := NewFromEnv().(*MockClient); ok {
		t.Fatal("test env must not be mock")
	}
}
