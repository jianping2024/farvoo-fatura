// Command fiscal-local runs a standalone Fiscal Core HTTP server for local UAT.
//
//	cd apps/fiscal-agent && FISCAL_ALLOW_LOCAL_PROVISION=1 go run ./cmd/fiscal-local
//
// Env: FISCAL_DB, FISCAL_BIND, FISCAL_STORE_ID, FISCAL_SEED=1 (M0 legacy), FISCAL_AT_ENV=mock
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

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

	rt, err := bootstrap.Start(bootstrap.Options{
		DBPath: dbPath, DataDir: dataDir, BindAddr: bind, StoreID: storeID,
		SigningKeyPEMPath: key, SoftwareCertificateNumber: cert, Seed: seed,
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
