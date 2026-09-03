//go:build windows

package main

import (
	"flag"
	"fmt"
	"log"

	"farvoo-fiscal-agent/internal/fiscalclient"
	"farvoo-fiscal-agent/internal/fiscalwebview"
)

func main() {
	settings := flag.Bool("settings", false, "change Agent connection settings")
	flag.Parse()

	if *settings {
		runClientSettings()
		return
	}
	runClientMain()
}

func runClientSettings() {
	host, port := "", "17880"
	if cfg, err := fiscalclient.LoadConfig(); err == nil && cfg.AgentBase != "" {
		host, port, _ = fiscalclient.SplitAgentBase(cfg.AgentBase)
	}
	base, err := fiscalclient.RunSettings(host, port)
	if err != nil {
		log.Fatal(err)
	}
	if base != "" {
		fmt.Println("Saved:", base)
	}
}

func runClientMain() {
	// Sole Client process gate: second launch focuses existing shell and exits.
	if !fiscalclient.AcquireClientSingleInstance() {
		_ = fiscalwebview.FocusExistingByTitle(fiscalwebview.WindowTitle)
		return
	}
	cfg, err := fiscalclient.LoadConfig()
	if err != nil || cfg.AgentBase == "" {
		host, port := "", "17880"
		if cfg.AgentBase != "" {
			host, port, _ = fiscalclient.SplitAgentBase(cfg.AgentBase)
		}
		base, err := fiscalclient.RunSettings(host, port)
		if err != nil {
			log.Fatal(err)
		}
		if base == "" {
			return
		}
		cfg.AgentBase = base
	}
	dataPath, err := fiscalclient.WebViewDataDir()
	if err != nil {
		log.Fatal(err)
	}
	url := cfg.AgentBase + "/"
	if err := fiscalwebview.RunWindow(fiscalwebview.Options{
		URL:      url,
		DataPath: dataPath,
	}); err != nil {
		log.Fatal(err)
	}
}
