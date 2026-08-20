// FARVOO print agent: LAN 9100 discover, pairing, Realtime (polling fallback) print_jobs, route per print_station.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "time/tzdata" // IANA zones (e.g. Europe/Lisbon) in Alpine/minimal images
)

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b)
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

func claim(apiBase, code, deviceID string) (*config, error) {
	bodyMap := map[string]string{
		"code":      code,
		"device_id": deviceID,
		"api_base":  strings.TrimRight(strings.TrimSpace(apiBase), "/"),
	}
	if host, err := os.Hostname(); err == nil {
		if label := strings.TrimSpace(host); label != "" {
			bodyMap["label"] = label
		}
	}
	body, _ := json.Marshal(bodyMap)
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(apiBase, "/")+"/api/print-agent/claim", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("claim %s: %s", res.Status, string(raw))
	}
	var out struct {
		AgentJWT     string `json:"agentjwt"`
		ValidUntil   string `json:"valid_until"`
		SupabaseURL  string `json:"supabase_url"`
		RestaurantID string `json:"restaurant_id"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		AnonKey      string `json:"anon_key"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out.AgentJWT == "" {
		return nil, fmt.Errorf("claim: missing agentjwt")
	}
	cfg := &config{
		APIBase:      strings.TrimRight(strings.TrimSpace(apiBase), "/"),
		AgentJWT:     out.AgentJWT,
		DeviceID:     deviceID,
		RestaurantID: out.RestaurantID,
		ValidUntil:   out.ValidUntil,
		SupabaseURL:  out.SupabaseURL,
		AccessToken:  out.AccessToken,
		RefreshToken: out.RefreshToken,
		AnonKey:      out.AnonKey,
	}
	_ = alignSupabaseURLWithAPIBase(cfg)
	return cfg, nil
}

type printJob struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Status    string          `json:"status"`
	CreatedAt string          `json:"created_at"`
	Payload   json.RawMessage `json:"payload"`
}

func fetchPending(ctx context.Context, apiBase, jwt string) ([]printJob, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(apiBase, "/")+"/api/print-agent/pending-jobs", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("pending-jobs %s: %s", res.Status, string(raw))
	}
	var out struct {
		Jobs []printJob `json:"jobs"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out.Jobs, nil
}

func patchJob(ctx context.Context, apiBase, jwt, id string, patch map[string]any) error {
	body, _ := json.Marshal(patch)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, strings.TrimRight(apiBase, "/")+"/api/print-agent/jobs/"+id, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwt)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("patch job %s: %s", res.Status, string(raw))
	}
	return nil
}

func summarizeJobPayload(job printJob) string {
	p := parseJobPayload(job)
	if p.ConnectionTest {
		return "connection test"
	}
	station := p.stationName()
	var parts []string
	if station != "" {
		parts = append(parts, "station="+station)
	}
	if id := strings.TrimSpace(p.TableID); id != "" {
		parts = append(parts, "table_id="+id)
	}
	if t := strings.TrimSpace(p.TableDisplayName); t != "" {
		parts = append(parts, "table="+t)
	}
	for _, ln := range p.Lines {
		q := ln.Qty
		if q <= 0 {
			q = 1
		}
		parts = append(parts, fmt.Sprintf("%dx %s", q, ln.DisplayName))
	}
	if len(parts) == 0 {
		return job.Type
	}
	return strings.Join(parts, " | ")
}

func main() {
	windowsPrepareConsole(os.Args)
	if len(os.Args) > 1 && os.Args[1] == "--restart-wait" {
		time.Sleep(1 * time.Second)
		guardMainAgentSingleInstance()
		runAgent(nil)
		return
	}
	guardMainAgentSingleInstance()
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "discover":
			runDiscover(os.Args[2:])
			return
		case "pair":
			path := defaultConfigPath()
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
			defer cancel()
			if err := runPairingWizard(ctx, path, ""); err != nil {
				log.Fatal(err)
			}
			fmt.Println("Pairing saved to", path)
			return
		case "configure", "config":
			path := defaultConfigPath()
			prefill := ""
			if loaded, err := loadConfig(path); err == nil && loaded.APIBase != "" {
				prefill = loaded.APIBase
			}
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
			defer cancel()
			if err := runConfigureWizard(ctx, path, prefill, ""); err != nil {
				log.Fatal(err)
			}
			fmt.Println("Configuration saved to", path)
			return
		case "setup":
			path := defaultConfigPath()
			cfg, err := loadConfig(path)
			if err != nil || cfg.AgentJWT == "" {
				log.Fatal("pair with " + productName + " first (run MesaPrintAgent configure or pair)")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()
			if err := runSetupWizard(ctx, path, cfg); err != nil {
				log.Fatal(err)
			}
			fmt.Println("Printer settings saved to", path)
			return
		case "help", "-h", "--help":
			fmt.Printf("%s %s\n\n", printAgentName, Version)
			fmt.Println(`Usage:
  MesaPrintAgent              Run agent (Windows: system tray; first run opens pairing on :17892/pair)
  MesaPrintAgent -console     Run with visible console (debug; Windows)
  MesaPrintAgent configure     Printer/station mapping (http://127.0.0.1:17892/configure; /pair on same port)
  MesaPrintAgent pair          Standalone pairing UI only when tray is not running (http://127.0.0.1:17890/pair)
  MesaPrintAgent setup         Legacy printer setup wizard (first-run bootstrap; port 17891)
  MesaPrintAgent discover      Scan LAN :9100 and list Windows printers
  MesaPrintAgent [-api URL] [-code CODE]   Optional CLI pairing (advanced)

Tray / Dashboard: http://127.0.0.1:17892/configure and /pair (up from tray start, including unpaired)
CLI pair only: http://127.0.0.1:17890/pair

Config: schedule + poll intervals. See README.`)
			return
		case "version", "-v", "--version":
			fmt.Println(Version)
			return
		case "run":
			runAgent(os.Args[2:])
			return
		}
	}
	runAgent(os.Args[1:])
}
