//go:build !windows

package main

import "log"

func main() {
	log.Fatal("FarvooFiscalClient is Windows-only")
}
