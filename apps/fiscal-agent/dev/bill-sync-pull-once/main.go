// Command bill-sync-pull-once runs the sole billsync.Puller against FARVOO_API (UAT).
package main

import (
	"context"
	"fmt"
	"os"

	"farvoo-fiscal-agent/internal/fiscal/billsync"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

func main() {
	dbPath := os.Getenv("FISCAL_DB")
	api := os.Getenv("FARVOO_API")
	jwt := os.Getenv("FARVOO_JWT")
	if jwt == "" {
		jwt = "test-jwt"
	}
	db, err := store.Open(dbPath)
	if err != nil {
		panic(err)
	}
	defer db.Close()
	p := &billsync.Puller{APIBase: api, JWT: jwt, DB: db}
	n, err := p.PullAndIngest(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(n)
}
