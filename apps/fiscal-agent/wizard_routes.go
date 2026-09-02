package main

import (
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

type printerListEntry struct {
	Addr  string `json:"addr"`
	Label string `json:"label"`
}

type setupRequestBody struct {
	StationPrinters map[string]string `json:"station_printers"`
	ForceTakeover   bool              `json:"force_takeover,omitempty"`
	UILocale        string            `json:"ui_locale,omitempty"`
	TextEncoding    string            `json:"text_encoding,omitempty"`
}

func discoverAllPrinters(timeout time.Duration, workers int) (tcp []printerListEntry, winspool []printerListEntry, err error) {
	if workers <= 0 {
		workers = 32
	}
	if timeout <= 0 {
		timeout = 600 * time.Millisecond
	}
	found, err := discoverPrinters9100(timeout, workers)
	if err != nil {
		return nil, nil, err
	}
	for _, p := range found {
		tcp = append(tcp, printerListEntry{
			Addr:  schemeTCP + ":" + p.Addr,
			Label: p.Addr + " (LAN)",
		})
	}
	tcp = dedupePrinterList(tcp)
	names, _ := listWinspoolPrinterNames()
	sort.Strings(names)
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		winspool = append(winspool, printerListEntry{
			Addr:  schemeWinspool + ":" + name,
			Label: name + " (USB / Windows)",
		})
	}
	winspool = dedupePrinterList(winspool)
	return tcp, winspool, nil
}

func dedupePrinterList(list []printerListEntry) []printerListEntry {
	if len(list) < 2 {
		return list
	}
	out := make([]printerListEntry, 0, len(list))
	seen := make(map[string]struct{}, len(list))
	for _, p := range list {
		addr := strings.TrimSpace(p.Addr)
		if addr == "" {
			continue
		}
		key := strings.ToLower(strings.Join(strings.Fields(addr), " "))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		p.Addr = addr
		p.Label = strings.TrimSpace(p.Label)
		out = append(out, p)
	}
	return out
}

func reloadConfig(configPath string, fallback *config) *config {
	if c, err := loadConfig(configPath); err == nil {
		return c
	}
	if fallback != nil {
		return fallback
	}
	return &config{}
}

func writeConfigureState(w http.ResponseWriter, cfg *config) {
	stationCount := 0
	if cfg.StationPrinters != nil {
		stationCount = len(cfg.StationPrinters)
	}
	loc := cfg.uiLocale()
	writePairJSON(w, http.StatusOK, map[string]any{
		"paired":           strings.TrimSpace(cfg.AgentJWT) != "",
		"api_base":         cfg.APIBase,
		"device_id":        cfg.DeviceID,
		"station_printers": cfg.StationPrinters,
		"station_count":    stationCount,
		"ui_locale":        loc,
		"text_encoding":    normalizeTextEncoding(cfg.TextEncoding),
		"ui":               uiBundleMap(loc),
		"pair_path":        "/pair",
	})
}

func registerUILocaleRoute(mux *http.ServeMux, configPath string, cfg **config) {
	// Wizard companion to Admin PUT /local/v1/setup/ui-locale: same applyUILocaleToConfig
	// mutation; may also persist text_encoding. Do not merge HTTP surfaces.
	mux.HandleFunc("/api/ui-locale", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Locale       string `json:"ui_locale"`
			TextEncoding string `json:"text_encoding"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 512)).Decode(&body); err != nil {
			writePairJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
			return
		}
		c := reloadConfig(configPath, *cfg)
		if strings.TrimSpace(body.Locale) != "" {
			applyUILocaleToConfig(c, body.Locale)
		}
		if strings.TrimSpace(body.TextEncoding) != "" {
			c.TextEncoding = normalizeTextEncoding(body.TextEncoding)
		}
		if err := saveConfig(configPath, c); err != nil {
			writePairJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		*cfg = c
		loc := c.uiLocale()
		writePairJSON(w, http.StatusOK, map[string]any{
			"status":        "ok",
			"ui_locale":     loc,
			"text_encoding": normalizeTextEncoding(c.TextEncoding),
			"ui":            uiBundleMap(loc),
		})
	})
}

func applyPrinterSetup(cfg *config, body setupRequestBody) (map[string]string, error) {
	cfg.DefaultPrinter = ""
	cfg.PrinterHost = ""
	cfg.CashierPrinter = ""
	cleaned := map[string]string{}
	for sid, raw := range body.StationPrinters {
		sid = strings.TrimSpace(sid)
		raw = strings.TrimSpace(raw)
		if sid == "" || raw == "" {
			continue
		}
		st, err := parsePrinterTarget(raw)
		if err != nil {
			return nil, err
		}
		cleaned[sid] = st.Display
	}
	if len(cleaned) > 0 {
		cfg.StationPrinters = cleaned
	} else {
		cfg.StationPrinters = nil
	}
	return cleaned, nil
}

func registerPrinterWizardRoutes(mux *http.ServeMux, configPath string, cfg **config, logPrefix string) {
	registerWizardUISharedRoute(mux)

	mux.HandleFunc("/api/printers", func(w http.ResponseWriter, r *http.Request) {
		tcp, win, err := discoverPrintersForAPI(printersAPIForceRefresh(r))
		if err != nil && len(tcp) == 0 && len(win) == 0 {
			writePairJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writePairJSON(w, http.StatusOK, map[string]any{"tcp": tcp, "winspool": win})
	})

	mux.HandleFunc("/api/print-stations", func(w http.ResponseWriter, r *http.Request) {
		c := reloadConfig(configPath, *cfg)
		*cfg = c
		if strings.TrimSpace(c.AgentJWT) == "" {
			writePairJSON(w, http.StatusUnauthorized, map[string]string{"error": "请先完成配对"})
			return
		}
		stations, err := fetchPrintStations(c.APIBase, c.AgentJWT)
		if err != nil {
			writePairJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
		writePairJSON(w, http.StatusOK, map[string]any{"stations": stations})
	})

	mux.HandleFunc("/api/setup-state", func(w http.ResponseWriter, r *http.Request) {
		c := reloadConfig(configPath, *cfg)
		*cfg = c
		writeConfigureState(w, c)
	})

	mux.HandleFunc("/api/setup", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		c := reloadConfig(configPath, *cfg)
		if strings.TrimSpace(c.AgentJWT) == "" {
			writePairJSON(w, http.StatusUnauthorized, map[string]string{"error": "请先完成配对"})
			return
		}
		var body setupRequestBody
		if err := json.NewDecoder(io.LimitReader(r.Body, 65536)).Decode(&body); err != nil {
			writePairJSON(w, http.StatusBadRequest, map[string]string{"error": "无效的请求"})
			return
		}
		if loc := strings.TrimSpace(body.UILocale); loc != "" {
			applyUILocaleToConfig(c, loc)
		}
		if enc := strings.TrimSpace(body.TextEncoding); enc != "" {
			c.TextEncoding = normalizeTextEncoding(enc)
		}
		status, resp := persistStationPrinterSetup(configPath, cfg, c, body)
		writePairJSON(w, status, resp)
	})

	mux.HandleFunc("/api/test-print", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		c := reloadConfig(configPath, *cfg)
		if strings.TrimSpace(c.AgentJWT) == "" {
			writePairJSON(w, http.StatusUnauthorized, map[string]string{"error": "请先完成配对"})
			return
		}
		var body testPrintRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil {
			writePairJSON(w, http.StatusBadRequest, map[string]string{"error": "无效的请求"})
			return
		}
		if err := runTestPrintForStation(c, body.StationID, body.Printer, body.Locale); err != nil {
			writePairJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		agentLog(c, "log_test_print_station", strings.TrimSpace(body.StationID))
		writePairJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}

func registerPairWizardRoute(mux *http.ServeMux, configPath string, cfg **config, logPrefix string, onSuccess func()) {
	mux.HandleFunc("/api/pair", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			APIBase string `json:"api_base"`
			Code    string `json:"code"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil {
			writePairJSON(w, http.StatusBadRequest, map[string]string{"error": "无效的请求"})
			return
		}
		apiBase, err := normalizeAPIBase(body.APIBase)
		if err != nil {
			writePairJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		code := strings.TrimSpace(body.Code)
		if len(code) != 6 {
			writePairJSON(w, http.StatusBadRequest, map[string]string{"error": "配对码须为 6 位数字"})
			return
		}
		for _, ch := range code {
			if ch < '0' || ch > '9' {
				writePairJSON(w, http.StatusBadRequest, map[string]string{"error": "配对码须为 6 位数字"})
				return
			}
		}

		deviceID := deviceIDForPairing(configPath)
		next, err := claim(apiBase, code, deviceID)
		if err != nil {
			writePairJSON(w, http.StatusBadRequest, map[string]string{"error": pairClaimUserError(err)})
			return
		}
		if err := savePairConfig(configPath, next); err != nil {
			writePairJSON(w, http.StatusInternalServerError, map[string]string{"error": "保存配置失败: " + err.Error()})
			return
		}
		if cfg != nil {
			*cfg = reloadConfig(configPath, next)
		}
		agentLogLocale(localeFromConfigPath(configPath), "log_pair_ok")
		writePairJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		schedulePairOnSuccess(onSuccess)
	})
}
