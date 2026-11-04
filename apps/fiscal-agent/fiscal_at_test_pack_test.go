package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPackagedATTestMaterialsIn(t *testing.T) {
	dir := t.TempDir()
	if packagedATTestMaterialsIn(dir) {
		t.Fatal("empty dir")
	}
	certs := filepath.Join(dir, packagedATCertDir)
	if err := os.MkdirAll(certs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(certs, packagedATClientCertName), []byte("pfx"), 0o644); err != nil {
		t.Fatal(err)
	}
	if packagedATTestMaterialsIn(dir) {
		t.Fatal("pfx only")
	}
	if err := os.WriteFile(filepath.Join(certs, packagedATPasswordPubName), []byte("cer"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !packagedATTestMaterialsIn(dir) {
		t.Fatal("want both present")
	}
}

func TestEnsurePackagedATTestDefaultsSeedsEmpty(t *testing.T) {
	install := t.TempDir()
	certs := filepath.Join(install, packagedATCertDir)
	if err := os.MkdirAll(certs, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(certs, packagedATClientCertName), []byte("pfx"), 0o644)
	_ = os.WriteFile(filepath.Join(certs, packagedATPasswordPubName), []byte("cer"), 0o644)

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	got := ensurePackagedATTestDefaultsAt(nil, install, cfgPath)
	if got.FiscalATEnv != "test" {
		t.Fatalf("env=%q", got.FiscalATEnv)
	}
	if got.FiscalATClientCert != packagedATClientCertRel || got.FiscalATPasswordPubkey != packagedATPasswordPubRel {
		t.Fatalf("paths %+v", got)
	}
	if got.FiscalATClientCertPass != packagedATClientCertPass {
		t.Fatalf("pass %q", got.FiscalATClientCertPass)
	}
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	var disk config
	if err := json.Unmarshal(raw, &disk); err != nil {
		t.Fatal(err)
	}
	if disk.FiscalATEnv != "test" {
		t.Fatalf("disk env %q", disk.FiscalATEnv)
	}
}

func TestEnsurePackagedATTestDefaultsRespectsExplicitMock(t *testing.T) {
	install := t.TempDir()
	certs := filepath.Join(install, packagedATCertDir)
	_ = os.MkdirAll(certs, 0o755)
	_ = os.WriteFile(filepath.Join(certs, packagedATClientCertName), []byte("pfx"), 0o644)
	_ = os.WriteFile(filepath.Join(certs, packagedATPasswordPubName), []byte("cer"), 0o644)

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	got := ensurePackagedATTestDefaultsAt(&config{FiscalATEnv: "mock"}, install, cfgPath)
	if got.FiscalATEnv != "mock" {
		t.Fatalf("got %q", got.FiscalATEnv)
	}
	if _, err := os.Stat(cfgPath); !os.IsNotExist(err) {
		t.Fatal("must not rewrite config when env already set and cert fields empty under non-test")
	}
}

func TestEnsurePackagedATTestDefaultsNoOpWithoutMaterials(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	got := ensurePackagedATTestDefaultsAt(&config{}, t.TempDir(), cfgPath)
	if got.FiscalATEnv != "" {
		t.Fatalf("seeded without materials: %q", got.FiscalATEnv)
	}
}
