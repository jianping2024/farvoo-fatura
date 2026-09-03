package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveEmbedStoreID_Order(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	prevOverride := configPathOverride
	configPathOverride = cfgPath
	defer func() { configPathOverride = prevOverride }()

	write := func(restaurantID string) {
		t.Helper()
		raw, _ := json.Marshal(config{RestaurantID: restaurantID})
		if err := os.WriteFile(cfgPath, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	t.Setenv("FISCAL_STORE_ID", "")
	write("disk-restaurant-uuid")
	if got := resolveEmbedStoreID(nil); got != "disk-restaurant-uuid" {
		t.Fatalf("nil cfg + disk: got %q", got)
	}
	if got := resolveEmbedStoreID(&config{}); got != "disk-restaurant-uuid" {
		t.Fatalf("empty cfg + disk: got %q", got)
	}
	if got := resolveEmbedStoreID(&config{RestaurantID: "sess-restaurant-uuid"}); got != "sess-restaurant-uuid" {
		t.Fatalf("cfg wins over disk: got %q", got)
	}

	t.Setenv("FISCAL_STORE_ID", "env-store")
	if got := resolveEmbedStoreID(&config{RestaurantID: "sess-restaurant-uuid"}); got != "env-store" {
		t.Fatalf("env wins: got %q", got)
	}
	t.Setenv("FISCAL_STORE_ID", "")

	_ = os.Remove(cfgPath)
	if got := resolveEmbedStoreID(nil); got != defaultEmbedStoreID {
		t.Fatalf("greenfield default: got %q want %q", got, defaultEmbedStoreID)
	}
}

// TestSoleEmbedStoreIDWritings locks: one resolver; startEmbeddedFiscal does not inline demo/default.
func TestSoleEmbedStoreIDWritings(t *testing.T) {
	agent := filepath.Join(moduleRoot(t), "apps", "fiscal-agent")
	embed, err := os.ReadFile(filepath.Join(agent, "fiscal_embed.go"))
	if err != nil {
		t.Fatal(err)
	}
	es := string(embed)
	if strings.Count(es, "func resolveEmbedStoreID(") != 1 {
		t.Fatal("resolveEmbedStoreID must be defined exactly once")
	}
	start := es[strings.Index(es, "func startEmbeddedFiscal("):]
	if i := strings.Index(start[1:], "\nfunc "); i > 0 {
		start = start[:i+1]
	}
	if strings.Count(start, "resolveEmbedStoreID(") != 1 {
		t.Fatal("startEmbeddedFiscal must call resolveEmbedStoreID exactly once")
	}
	if strings.Contains(start, `storeID := "store-demo-001"`) {
		t.Fatal("startEmbeddedFiscal must not inline store-demo-001; use resolveEmbedStoreID")
	}
	if strings.Contains(start, "cfg.RestaurantID") {
		t.Fatal("startEmbeddedFiscal must not re-read RestaurantID; resolveEmbedStoreID is sole")
	}
	if strings.Contains(start, "FISCAL_STORE_ID") {
		t.Fatal("startEmbeddedFiscal must not re-read FISCAL_STORE_ID; resolveEmbedStoreID is sole")
	}

	admin, err := os.ReadFile(filepath.Join(agent, "internal", "fiscal", "bootstrap", "admin", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	as := string(admin)
	fn := as[strings.Index(as, "async function loadLoginOperators("):]
	if i := strings.Index(fn[1:], "\n  async function "); i > 0 {
		fn = fn[:i+1]
	} else if i := strings.Index(fn[1:], "\n  function "); i > 0 {
		fn = fn[:i+1]
	}
	if !strings.Contains(fn, "bootstrap_required") {
		t.Fatal("loadLoginOperators must gate bootstrap on bootstrap_required")
	}
	if strings.Contains(fn, "setLoginBootstrapMode(true)") && !strings.Contains(fn, "st.bootstrap_required") {
		t.Fatal("bootstrap UI true path must check st.bootstrap_required")
	}
	// Forbid the old empty-ops → bootstrap branch.
	if strings.Contains(fn, "if (!ops.length)") && strings.Contains(fn, "setLoginBootstrapMode(true)") {
		block := fn[strings.Index(fn, "if (!ops.length)"):]
		if i := strings.Index(block, "}"); i > 0 {
			block = block[:i]
		}
		if strings.Contains(block, "setLoginBootstrapMode(true)") {
			t.Fatal("must not set bootstrap mode from empty ops alone")
		}
	}
}
