package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPersistRealtimeSessionTokensPreservesStationPrinters(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := saveConfig(path, &config{
		APIBase:      "https://example.test",
		AgentJWT:     "jwt",
		DeviceID:     "dev-1",
		RestaurantID: "rest-1",
		AnonKey:      "anon",
		AccessToken:  "old-access",
		RefreshToken: "old-refresh",
		StationPrinters: map[string]string{
			"kitchen": "tcp:10.0.0.1:9100",
			"bar":     "winspool:EPSON",
		},
		UILocale: "zh",
	}); err != nil {
		t.Fatal(err)
	}

	merged, err := persistRealtimeSessionTokens(path, "new-access", "new-refresh")
	if err != nil {
		t.Fatalf("persist: %v", err)
	}
	if merged.AccessToken != "new-access" || merged.RefreshToken != "new-refresh" {
		t.Fatalf("merged tokens: access=%q refresh=%q", merged.AccessToken, merged.RefreshToken)
	}
	if merged.StationPrinters["kitchen"] != "tcp:10.0.0.1:9100" || merged.StationPrinters["bar"] != "winspool:EPSON" {
		t.Fatalf("merged lost mappings: %#v", merged.StationPrinters)
	}
	if merged.UILocale != "zh" || merged.AgentJWT != "jwt" {
		t.Fatalf("merged lost other fields: locale=%q jwt=%q", merged.UILocale, merged.AgentJWT)
	}

	disk, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if disk.AccessToken != "new-access" || disk.RefreshToken != "new-refresh" {
		t.Fatalf("disk tokens: access=%q refresh=%q", disk.AccessToken, disk.RefreshToken)
	}
	if disk.StationPrinters["kitchen"] != "tcp:10.0.0.1:9100" {
		t.Fatalf("disk lost mappings: %#v", disk.StationPrinters)
	}
}

func TestPersistRealtimeSessionTokensLoadFailureDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing", "config.json") // parent dir missing → load fails
	_, err := persistRealtimeSessionTokens(path, "a", "b")
	if err == nil {
		t.Fatal("expected load error")
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("must not create config on load failure, stat=%v", statErr)
	}
}

func TestPersistRealtimeSessionTokensRejectsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	_ = saveConfig(path, &config{AccessToken: "a", RefreshToken: "b", AnonKey: "k"})
	if _, err := persistRealtimeSessionTokens("", "a", "b"); err == nil {
		t.Fatal("empty path")
	}
	if _, err := persistRealtimeSessionTokens(path, "", "b"); err == nil {
		t.Fatal("empty access")
	}
	if _, err := persistRealtimeSessionTokens(path, "a", ""); err == nil {
		t.Fatal("empty refresh")
	}
}

func TestEnsureFreshAccessTokenPreservesDiskMappings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"access_token":  "rotated-access",
			"refresh_token": "rotated-refresh",
		})
	}))
	defer srv.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := saveConfig(path, &config{
		APIBase:         "https://example.test",
		AgentJWT:        "jwt",
		DeviceID:        "dev-1",
		RestaurantID:    "rest-1",
		SupabaseURL:     srv.URL,
		AnonKey:         "anon",
		AccessToken:     "disk-access",
		RefreshToken:    "disk-refresh",
		StationPrinters: map[string]string{"kitchen": "tcp:10.0.0.9:9100"},
	}); err != nil {
		t.Fatal(err)
	}

	// Stale in-memory snapshot: no station_printers (the wipe bug).
	mem := &config{
		APIBase:      "https://example.test",
		AgentJWT:     "jwt",
		DeviceID:     "dev-1",
		RestaurantID: "rest-1",
		SupabaseURL:  srv.URL,
		AnonKey:      "anon",
		AccessToken:  "mem-access",
		RefreshToken: "disk-refresh",
	}
	r := &RealtimeNotifier{config: mem, configPath: path}
	if err := r.ensureFreshAccessToken(context.Background(), true); err != nil {
		t.Fatalf("ensureFresh: %v", err)
	}
	if r.config == nil || r.config.StationPrinters["kitchen"] != "tcp:10.0.0.9:9100" {
		t.Fatalf("r.config must adopt disk mappings after persist, got %#v", r.config)
	}
	if r.config.AccessToken != "rotated-access" || r.config.RefreshToken != "rotated-refresh" {
		t.Fatalf("r.config tokens: %#v %#v", r.config.AccessToken, r.config.RefreshToken)
	}
	disk, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if disk.StationPrinters["kitchen"] != "tcp:10.0.0.9:9100" {
		t.Fatalf("disk wiped mappings: %#v", disk.StationPrinters)
	}
	if disk.AccessToken != "rotated-access" {
		t.Fatalf("disk token not updated: %q", disk.AccessToken)
	}
}

func TestSoleRealtimeTokenPersistWriting(t *testing.T) {
	authRaw, err := os.ReadFile("auth_refresh.go")
	if err != nil {
		t.Fatal(err)
	}
	auth := string(authRaw)
	rtRaw, err := os.ReadFile("realtime.go")
	if err != nil {
		t.Fatal(err)
	}
	rt := string(rtRaw)

	if n := strings.Count(auth, "func persistRealtimeSessionTokens("); n != 1 {
		t.Fatalf("persistRealtimeSessionTokens want 1 def, got %d", n)
	}
	if n := strings.Count(rt, "persistRealtimeSessionTokens("); n != 1 {
		t.Fatalf("realtime.go must call persistRealtimeSessionTokens exactly once, got %d", n)
	}
	if strings.Contains(rt, "saveConfig(r.configPath") {
		t.Fatal("realtime.go must not saveConfig(r.configPath, …) — use persistRealtimeSessionTokens only")
	}
	if strings.Contains(rt, "saveConfig(r.configPath, r.config)") {
		t.Fatal("banned stale full-config persist still present")
	}
	// No alternate token persist helpers.
	for _, banned := range []string{
		"func persistAccessToken(",
		"func saveRealtimeTokens(",
		"func writeRealtimeSession(",
	} {
		if strings.Contains(auth, banned) || strings.Contains(rt, banned) {
			t.Fatalf("duplicate token persist helper: %s", banned)
		}
	}
}

func TestRefreshSupabaseSessionJSONBody(t *testing.T) {
	var gotCT string
	var gotBody map[string]string
	var gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.String()
		gotCT = r.Header.Get("Content-Type")
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &gotBody); err != nil {
			http.Error(w, `{"msg":"not json"}`, http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"access_token":  "new-access",
			"refresh_token": "new-refresh",
		})
	}))
	defer srv.Close()

	cfg := &config{
		SupabaseURL:  srv.URL,
		AnonKey:      "anon",
		AccessToken:  "old-access",
		RefreshToken: "old-refresh",
	}
	if err := refreshSupabaseSession(context.Background(), cfg); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if !strings.Contains(gotPath, "grant_type=refresh_token") {
		t.Fatalf("path = %q, want grant_type query", gotPath)
	}
	if !strings.HasPrefix(gotCT, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", gotCT)
	}
	if gotBody["refresh_token"] != "old-refresh" {
		t.Fatalf("body = %#v", gotBody)
	}
	if cfg.AccessToken != "new-access" || cfg.RefreshToken != "new-refresh" {
		t.Fatalf("tokens not updated: access=%q refresh=%q", cfg.AccessToken, cfg.RefreshToken)
	}
}

func TestAccessTokenUnexpired(t *testing.T) {
	if !accessTokenUnexpired(testAccessJWT(t, time.Now().Add(10*time.Minute)), time.Minute) {
		t.Fatal("expected unexpired")
	}
	if accessTokenUnexpired(testAccessJWT(t, time.Now().Add(30*time.Second)), time.Minute) {
		t.Fatal("expected expired under skew")
	}
	if accessTokenUnexpired("not-a-jwt", 0) {
		t.Fatal("garbage should be expired")
	}
}

func TestShouldSkipAccessTokenRefresh(t *testing.T) {
	fresh := testAccessJWT(t, time.Now().Add(10*time.Minute))
	skew := time.Minute
	if !shouldSkipAccessTokenRefresh(fresh, false, skew) {
		t.Fatal("non-force + fresh token should skip")
	}
	if shouldSkipAccessTokenRefresh(fresh, true, skew) {
		t.Fatal("force refresh must never skip")
	}
	stale := testAccessJWT(t, time.Now().Add(30*time.Second))
	if shouldSkipAccessTokenRefresh(stale, false, skew) {
		t.Fatal("near-expiry must not skip")
	}
}

func TestTimeUntilAccessTokenRefresh(t *testing.T) {
	skew := accessTokenRefreshSkew

	if d := timeUntilAccessTokenRefresh("not-a-jwt", skew); d != 0 {
		t.Fatalf("garbage: got %v, want 0", d)
	}
	if d := timeUntilAccessTokenRefresh(testAccessJWT(t, time.Now().Add(skew/2)), skew); d != 0 {
		t.Fatalf("inside skew window: got %v, want 0", d)
	}

	d := timeUntilAccessTokenRefresh(testAccessJWT(t, time.Now().Add(10*time.Minute)), skew)
	// Expect ~8 minutes (10m - 2m skew), allow slack for test runtime.
	if d < 7*time.Minute || d > 9*time.Minute {
		t.Fatalf("far from expiry: got %v, want ~8m", d)
	}
}

func testAccessJWT(t *testing.T, exp time.Time) string {
	t.Helper()
	payload, err := json.Marshal(map[string]int64{"exp": exp.Unix()})
	if err != nil {
		t.Fatal(err)
	}
	return "x." + base64.RawURLEncoding.EncodeToString(payload) + ".y"
}
