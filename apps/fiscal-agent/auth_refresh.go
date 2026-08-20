package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// refreshSupabaseSession exchanges refresh_token for a new access/refresh pair (GoTrue JSON API).
// Updates cfg in place; caller persists config when desired.
func refreshSupabaseSession(ctx context.Context, cfg *config) error {
	if cfg == nil || !cfg.hasRealtimeSession() {
		return fmt.Errorf("no realtime session")
	}
	base := strings.TrimRight(cfg.getSupabaseURL(), "/")
	if base == "" {
		return fmt.Errorf("missing supabase_url")
	}

	body, err := json.Marshal(map[string]string{
		"refresh_token": cfg.RefreshToken,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		base+"/auth/v1/token?grant_type=refresh_token",
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", cfg.AnonKey)
	req.Header.Set("Authorization", "Bearer "+cfg.AnonKey)

	client := &http.Client{Timeout: 15 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("refresh %s: %s", res.Status, string(raw))
	}

	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return err
	}
	if strings.TrimSpace(out.AccessToken) == "" || strings.TrimSpace(out.RefreshToken) == "" {
		return fmt.Errorf("refresh: missing tokens")
	}
	cfg.AccessToken = out.AccessToken
	cfg.RefreshToken = out.RefreshToken
	return nil
}

// accessTokenRefreshSkew: refresh/reconnect this far before JWT exp so Realtime
// never sits on an expired channel (Supabase default access TTL is ~1h).
const accessTokenRefreshSkew = 2 * time.Minute

// accessTokenExp returns the JWT exp claim. ok=false if unparseable.
func accessTokenExp(accessToken string) (exp time.Time, ok bool) {
	parts := strings.Split(accessToken, ".")
	if len(parts) != 3 {
		return time.Time{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Some issuers pad; try standard raw with padding.
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return time.Time{}, false
		}
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Exp <= 0 {
		return time.Time{}, false
	}
	return time.Unix(claims.Exp, 0), true
}

// accessTokenUnexpired reports whether JWT access_token exp is still after now+skew.
// Unparseable tokens are treated as expired (caller must refresh).
func accessTokenUnexpired(accessToken string, skew time.Duration) bool {
	return timeUntilAccessTokenRefresh(accessToken, skew) > 0
}

// timeUntilAccessTokenRefresh is how long to wait before renewing the Realtime
// session (disconnect → ensureFreshAccessToken → reconnect). Zero means now.
func timeUntilAccessTokenRefresh(accessToken string, skew time.Duration) time.Duration {
	exp, ok := accessTokenExp(accessToken)
	if !ok {
		return 0
	}
	d := time.Until(exp.Add(-skew))
	if d < 0 {
		return 0
	}
	return d
}
