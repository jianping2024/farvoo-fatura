// Command fiscal-local runs a standalone Fiscal Core HTTP server for local UAT.
//
//	cd apps/fiscal-agent && FISCAL_ALLOW_LOCAL_PROVISION=1 go run ./cmd/fiscal-local
//
// Env: FISCAL_DB, FISCAL_BIND, FISCAL_STORE_ID, FISCAL_SEED=1 (M0 legacy; demo cashier PIN = store.SeedDemoOperatorPIN), FISCAL_AT_ENV=mock
// Optional UAT: FISCAL_STATION_PRINTERS_JSON, FISCAL_STATION_META_JSON
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"farvoo-fiscal-agent/internal/fiscal/api"
	"farvoo-fiscal-agent/internal/fiscal/bootstrap"
)

func main() {
	cwd, _ := os.Getwd()
	dbPath := env("FISCAL_DB", filepath.Join(cwd, "data", "fiscal-local.db"))
	dataDir := env("FISCAL_DATA_DIR", filepath.Join(filepath.Dir(dbPath), "secure"))
	bind := env("FISCAL_BIND", "127.0.0.1:17880")
	storeID := env("FISCAL_STORE_ID", "store-demo-001")
	key := env("FISCAL_KEY_PEM", filepath.Join(cwd, "internal", "fiscal", "testdata", "dev_signing_key.pem"))
	cert := env("FISCAL_CERT_NO", "0")
	seed := os.Getenv("FISCAL_SEED") == "1"
	// CLI/UAT only: FISCAL_ALLOW_LAN=1 for non-loopback bind (Agent product path uses config.json).
	allowLAN := os.Getenv("FISCAL_ALLOW_LAN") == "1"

	stationPrinters := parseStationPrintersJSON(os.Getenv("FISCAL_STATION_PRINTERS_JSON"))
	stationMeta := parseStationMetaJSON(os.Getenv("FISCAL_STATION_META_JSON"))

	var stationPrintersFn func() map[string]string
	if stationPrinters != nil {
		stationPrintersFn = func() map[string]string { return stationPrinters }
	}
	var stationMetaFn func() []api.StationMeta
	if stationMeta != nil {
		stationMetaFn = func() []api.StationMeta { return stationMeta }
	}

	rt, err := bootstrap.Start(bootstrap.Options{
		DBPath: dbPath, DataDir: dataDir, BindAddr: bind, AllowLAN: allowLAN, StoreID: storeID,
		SigningKeyPEMPath: key, SoftwareCertificateNumber: cert, Seed: seed,
		StationPrintersFn: stationPrintersFn,
		StationMetaFn:     stationMetaFn,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer rt.Close()
	fmt.Printf("fiscal-local listening on http://%s\n", bind)
	fmt.Printf("db=%s dataDir=%s store=%s seed=%v at_env=%s\n", dbPath, dataDir, storeID, seed, env("FISCAL_AT_ENV", "mock"))
	fmt.Println("admin: GET /   setup: /local/v1/setup/*")

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func parseStationPrintersJSON(raw string) map[string]string {
	raw = os.ExpandEnv(raw)
	if raw == "" {
		return nil
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		log.Fatalf("FISCAL_STATION_PRINTERS_JSON: %v", err)
	}
	return out
}

func parseStationMetaJSON(raw string) []api.StationMeta {
	raw = os.ExpandEnv(raw)
	if raw == "" {
		return nil
	}
	var rows []struct {
		ID        string `json:"id"`
		NameZh    string `json:"name_zh"`
		NameEn    string `json:"name_en"`
		NamePt    string `json:"name_pt"`
		SortOrder int    `json:"sort_order"`
	}
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		log.Fatalf("FISCAL_STATION_META_JSON: %v", err)
	}
	out := make([]api.StationMeta, 0, len(rows))
	for _, r := range rows {
		out = append(out, api.StationMeta{
			ID: r.ID, NameZh: r.NameZh, NameEn: r.NameEn, NamePt: r.NamePt, SortOrder: r.SortOrder,
		})
	}
	return out
}
