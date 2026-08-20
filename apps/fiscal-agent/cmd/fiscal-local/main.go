// Command fiscal-local runs a standalone Fiscal Core HTTP server for local UAT.
//
//	cd apps/fiscal-agent && go run ./cmd/fiscal-local
//
// Env:
//
//	FISCAL_DB          default ./data/fiscal-local.db
//	FISCAL_BIND        default 127.0.0.1:17880
//	FISCAL_STORE_ID    default store-demo-001
//	FISCAL_KEY_PEM     path to RSA-1024 PEM
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
	bind := env("FISCAL_BIND", "127.0.0.1:17880")
	storeID := env("FISCAL_STORE_ID", "store-demo-001")
	key := env("FISCAL_KEY_PEM", "")
	if key == "" {
		key = filepath.Join(cwd, "internal", "fiscal", "testdata", "dev_signing_key.pem")
	}
	cert := env("FISCAL_CERT_NO", "0")

	rt, err := bootstrap.Start(bootstrap.Options{
		DBPath:                    dbPath,
		BindAddr:                  bind,
		StoreID:                   storeID,
		SigningKeyPEMPath:         key,
		SoftwareCertificateNumber: cert,
		Seed:                      true,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer rt.Close()
	fmt.Printf("fiscal-local listening on http://%s\n", bind)
	fmt.Printf("db=%s store=%s\n", dbPath, storeID)
	fmt.Println("admin UI: GET /   health: GET /local/v1/health")

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
