package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type config struct {
	APIBase         string            `json:"api_base"`
	AgentJWT        string            `json:"agentjwt"`
	DeviceID        string            `json:"device_id"`
	RestaurantID    string            `json:"restaurant_id,omitempty"`
	PrinterHost     string            `json:"printer_host,omitempty"`
	DefaultPrinter  string            `json:"default_printer,omitempty"` // legacy file field; not used for receipt routing
	CashierPrinter  string            `json:"cashier_printer,omitempty"` // legacy; cleared on save, not used for routing
	StationPrinters map[string]string `json:"station_printers,omitempty"`
	Schedule        *scheduleConfig   `json:"schedule,omitempty"`
	Poll            *pollConfig       `json:"poll,omitempty"`
	// SupabaseURL: optional explicit Supabase Realtime URL (e.g. https://xxx.supabase.co); if omitted, inferred from api_base.
	SupabaseURL string `json:"supabase_url,omitempty"`
	// AnonKey + AccessToken + RefreshToken: Supabase Auth session for Realtime (not agentjwt).
	AnonKey      string `json:"anon_key,omitempty"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	// NotificationMode: "realtime" (default) or "polling" (fallback).
	NotificationMode string `json:"notification_mode,omitempty"`
	// UILocale: zh | en | pt — Admin + tray UI; invoice chrome via locale.InvoiceLocaleFromUI (scheme A).
	UILocale string `json:"ui_locale,omitempty"`
	// ValidUntil: RFC3339 from claim; used for tray renewal hints (half-year credential).
	ValidUntil string `json:"valid_until,omitempty"`
	// TextEncoding: auto | utf8 | latin — Chinese auto mode renders raster bitmaps; legacy gbk normalizes to auto.
	TextEncoding string `json:"text_encoding,omitempty"`
	// FiscalAllowLocalProvision: allow Admin activate with pasted product PEM (UAT only).
	// nil / omitted → default false (Ops cloud provision). Explicit true enables.
	// Env FISCAL_ALLOW_LOCAL_PROVISION wins when set (CI / forced override).
	FiscalAllowLocalProvision *bool `json:"fiscal_allow_local_provision,omitempty"`
	// FiscalATEnv: mock | test | prod. Empty → mock. Env FISCAL_AT_ENV wins when set.
	FiscalATEnv string `json:"fiscal_at_env,omitempty"`
	// FiscalAllowLAN: allow Admin listen on LAN (0.0.0.0:17880). nil/false → loopback only.
	// Product path: Admin → config.json only (no FISCAL_ALLOW_LAN / FISCAL_BIND).
	FiscalAllowLAN *bool `json:"fiscal_allow_lan,omitempty"`
}

// hasRealtimeSession is true when claim returned Supabase Auth credentials for Realtime.
func (c *config) hasRealtimeSession() bool {
	if c == nil {
		return false
	}
	return strings.TrimSpace(c.AnonKey) != "" &&
		strings.TrimSpace(c.AccessToken) != "" &&
		strings.TrimSpace(c.RefreshToken) != ""
}

// resolveNotificationMode returns the configured mode or default to realtime.
func (c *config) resolveNotificationMode() NotificationMode {
	mode := NotificationMode(c.NotificationMode)
	if mode == NotificationModePolling {
		return NotificationModePolling
	}
	return NotificationModeRealtime
}

// getSupabaseURL returns the Realtime/Auth edge URL after aligning same-host
// supabase_url scheme with api_base (Mode B Tunnel proto mismatch heal).
func (c *config) getSupabaseURL() string {
	_ = alignSupabaseURLWithAPIBase(c)
	if strings.TrimSpace(c.SupabaseURL) != "" {
		return strings.TrimRight(strings.TrimSpace(c.SupabaseURL), "/")
	}
	return inferSupabaseURL(c.APIBase)
}

// alignSupabaseURLWithAPIBase is the sole heal for claim/proxy writing http:// on the
// same host the agent reached via https:// api_base. Different hosts (cloud Supabase
// project URL vs Vercel app) are left unchanged. Returns true if SupabaseURL changed.
func alignSupabaseURLWithAPIBase(c *config) bool {
	if c == nil {
		return false
	}
	apiOrigin, apiHost, ok := httpOriginAndHost(c.APIBase)
	if !ok {
		return false
	}
	sb := strings.TrimSpace(c.SupabaseURL)
	if sb == "" {
		return false
	}
	_, sbHost, ok := httpOriginAndHost(sb)
	if !ok || !strings.EqualFold(apiHost, sbHost) {
		return false
	}
	want := strings.TrimRight(apiOrigin, "/")
	cur := strings.TrimRight(sb, "/")
	if strings.EqualFold(cur, want) {
		if cur != want {
			c.SupabaseURL = want
			return true
		}
		return false
	}
	c.SupabaseURL = want
	return true
}

func httpOriginAndHost(raw string) (origin string, host string, ok bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", "", false
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return "", "", false
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", "", false
	}
	return scheme + "://" + u.Host, u.Hostname(), true
}

// persistAlignedSupabaseURL writes config when same-host scheme heal changed the file.
func persistAlignedSupabaseURL(path string, c *config) {
	if c == nil || path == "" {
		return
	}
	if !alignSupabaseURLWithAPIBase(c) {
		return
	}
	if err := saveConfig(path, c); err != nil {
		return
	}
}

// configPathOverride is set by tests only.
var configPathOverride string

func defaultConfigPath() string {
	if configPathOverride != "" {
		return configPathOverride
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "farvoo-fiscal-agent.json"
	}
	return filepath.Join(home, ".config", "farvoo-fiscal-agent", "config.json")
}

func loadConfig(path string) (*config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c config
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func saveConfig(path string, c *config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

func normalizeHostPort(hostPort string, defaultPort string) string {
	hostPort = strings.TrimSpace(hostPort)
	if hostPort == "" {
		return ""
	}
	if strings.Contains(hostPort, ":") {
		return hostPort
	}
	return hostPort + ":" + defaultPort
}

func isReceiptPrintJobType(jobType string) bool {
	return jobType == "order_receipt" || jobType == "pre_bill"
}

func (c *config) resolveReceiptPrinterID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("receipt_printer_id required")
	}
	if id == "cashier" {
		return "", fmt.Errorf("receipt_printer_id cashier is no longer supported; use station:{id}")
	}
	const prefix = "station:"
	if !strings.HasPrefix(id, prefix) {
		return "", fmt.Errorf("unknown receipt_printer_id %q", id)
	}
	sid := strings.TrimSpace(id[len(prefix):])
	if sid == "" {
		return "", fmt.Errorf("invalid receipt_printer_id")
	}
	if c.StationPrinters != nil {
		if v := strings.TrimSpace(c.StationPrinters[sid]); v != "" {
			return v, nil
		}
	}
	return "", fmt.Errorf("no station_printers mapping for print_station_id %s", sid)
}

func (c *config) printerAddrForJob(job printJob) (string, error) {
	if isReceiptPrintJobType(job.Type) {
		var p struct {
			ReceiptPrinterID string `json:"receipt_printer_id"`
		}
		_ = json.Unmarshal(job.Payload, &p)
		return c.resolveReceiptRouting(job, p.ReceiptPrinterID)
	}

	if job.Type == "station_ticket" {
		var p struct {
			PrintStationID string `json:"print_station_id"`
		}
		_ = json.Unmarshal(job.Payload, &p)
		sid := strings.TrimSpace(p.PrintStationID)
		if sid == "" {
			return "", fmt.Errorf("station_ticket missing print_station_id")
		}
		if c.StationPrinters != nil {
			if v := strings.TrimSpace(c.StationPrinters[sid]); v != "" {
				return v, nil
			}
		}
		return "", fmt.Errorf("no station_printers mapping for print_station_id %s", sid)
	}

	// Unknown job types: do not route to cashier (keeps roles separate).
	return "", fmt.Errorf("unsupported print job type %q", job.Type)
}

// mergePairConfig keeps stable device identity and, on same-restaurant re-pair, local printer routing.
func mergePairConfig(prev, next *config) {
	if prev == nil || next == nil {
		return
	}
	if strings.TrimSpace(prev.DeviceID) != "" && strings.TrimSpace(next.DeviceID) == "" {
		next.DeviceID = prev.DeviceID
	}
	if shouldKeepStationPrinters(prev, next) {
		next.StationPrinters = prev.StationPrinters
	}
	if strings.TrimSpace(prev.UILocale) != "" {
		next.UILocale = prev.UILocale
	}
	if strings.TrimSpace(prev.TextEncoding) != "" && strings.TrimSpace(next.TextEncoding) == "" {
		next.TextEncoding = prev.TextEncoding
	}
}

func shouldKeepStationPrinters(prev, next *config) bool {
	if prev == nil || len(prev.StationPrinters) == 0 {
		return false
	}
	prevRestaurant := strings.TrimSpace(prev.RestaurantID)
	nextRestaurant := strings.TrimSpace(next.RestaurantID)
	if prevRestaurant == "" {
		// Legacy config before restaurant_id was persisted: keep mappings on first upgrade.
		return true
	}
	return nextRestaurant != "" && prevRestaurant == nextRestaurant
}

// mappedPrinterTargets returns deduplicated routing targets from station_printers.
func (c *config) mappedPrinterTargets() []printerTarget {
	if c == nil || c.StationPrinters == nil {
		return nil
	}
	seen := make(map[string]bool)
	var out []printerTarget
	for _, addr := range c.StationPrinters {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		pt, err := parsePrinterTarget(addr)
		if err != nil {
			continue
		}
		key := targetKey(pt)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, pt)
	}
	return out
}

func savePairConfig(configPath string, next *config) error {
	var prev *config
	if p, err := loadConfig(configPath); err == nil {
		prev = p
	}
	mergePairConfig(prev, next)
	_ = alignSupabaseURLWithAPIBase(next)
	return saveConfig(configPath, next)
}

func deviceIDForPairing(configPath string) string {
	if prev, err := loadConfig(configPath); err == nil {
		if id := strings.TrimSpace(prev.DeviceID); looksLikeUUID(id) {
			return id
		}
	}
	return newUUID()
}

func looksLikeUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, ch := range s {
		switch i {
		case 8, 13, 18, 23:
			if ch != '-' {
				return false
			}
		default:
			if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
				return false
			}
		}
	}
	return true
}
