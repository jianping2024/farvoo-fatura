package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

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
