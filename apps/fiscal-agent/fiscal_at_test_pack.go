package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

// Packaged AT test materials (public AT test pfx; see docs/fiscal-at-live-integration.zh.md §3).
const (
	packagedATCertDir         = "at-certs"
	packagedATClientCertName  = "TesteWebservices.pfx"
	packagedATPasswordPubName = "ChavePublicaAT.cer"
	packagedATClientCertRel   = packagedATCertDir + "/" + packagedATClientCertName
	packagedATPasswordPubRel  = packagedATCertDir + "/" + packagedATPasswordPubName
	packagedATClientCertPass  = "TESTEwebservice" // public AT test passphrase
)

// packagedATTestMaterialsIn reports whether AT test certs sit under installDir/at-certs/.
func packagedATTestMaterialsIn(installDir string) bool {
	installDir = strings.TrimSpace(installDir)
	if installDir == "" {
		return false
	}
	pfx := filepath.Join(installDir, packagedATCertDir, packagedATClientCertName)
	cer := filepath.Join(installDir, packagedATCertDir, packagedATPasswordPubName)
	if _, err := os.Stat(pfx); err != nil {
		return false
	}
	if _, err := os.Stat(cer); err != nil {
		return false
	}
	return true
}

func agentInstallDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(exe)
}

// ensurePackagedATTestDefaults fills empty AT live fields for the AT-test Setup/zip pack.
// ONLY when packaged at-certs exist beside the exe. Does not override an explicit fiscal_at_env.
// Unique write path for this pack's config seed (product path remains config.json).
func ensurePackagedATTestDefaults(cfg *config) *config {
	return ensurePackagedATTestDefaultsAt(cfg, agentInstallDir(), defaultConfigPath())
}

func ensurePackagedATTestDefaultsAt(cfg *config, installDir, configPath string) *config {
	if !packagedATTestMaterialsIn(installDir) {
		return cfg
	}
	if cfg == nil {
		if c, err := loadConfig(configPath); err == nil {
			cfg = c
		} else {
			cfg = &config{}
		}
	}
	changed := false
	if strings.TrimSpace(cfg.FiscalATEnv) == "" {
		cfg.FiscalATEnv = "test"
		changed = true
	}
	if strings.EqualFold(strings.TrimSpace(cfg.FiscalATEnv), "test") {
		if strings.TrimSpace(cfg.FiscalATClientCert) == "" {
			cfg.FiscalATClientCert = packagedATClientCertRel
			changed = true
		}
		if strings.TrimSpace(cfg.FiscalATClientCertPass) == "" {
			cfg.FiscalATClientCertPass = packagedATClientCertPass
			changed = true
		}
		if strings.TrimSpace(cfg.FiscalATPasswordPubkey) == "" {
			cfg.FiscalATPasswordPubkey = packagedATPasswordPubRel
			changed = true
		}
	}
	if !changed {
		return cfg
	}
	if err := saveConfig(configPath, cfg); err != nil {
		log.Printf("fiscal: AT test pack: could not save config defaults: %v", err)
		return cfg
	}
	log.Printf("fiscal: AT test pack: seeded config.json fiscal_at_env=%q (certs under %s/)", cfg.FiscalATEnv, packagedATCertDir)
	return cfg
}
