package service_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/service"
	"farvoo-fiscal-agent/internal/fiscal/signer"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

func TestActivateFromCloud_SaveActivationOnly(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	dataDir := filepath.Join(dir, "secure")
	storeID := "store-cloud-1"
	if err := db.UpsertTaxpayer(store.TaxpayerInput{
		StoreID: storeID, TaxRegistrationNumber: "517535009", LegalName: "Demo",
		AddressDetail: "Rua 1", City: "Lisboa", PostalCode: "1000-001", Country: "PT", Timezone: "Europe/Lisbon",
		SoftwareCertificateNumber: "0",
	}); err != nil {
		t.Fatal(err)
	}

	productPath := filepath.Join("..", "testdata", "dev_signing_key.pem")
	productPEM, err := os.ReadFile(productPath)
	if err != nil {
		t.Fatal(err)
	}
	inner, err := signer.LoadPEM(productPEM, 1)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := inner.PublicKeyPEM()
	if err != nil {
		t.Fatal(err)
	}

	// Pre-create device key so we can wrap with its public key in the mock.
	dev, err := signer.GenerateDeviceKey()
	if err != nil {
		t.Fatal(err)
	}
	if err := signer.SaveDeviceKey(dataDir, dev); err != nil {
		t.Fatal(err)
	}
	wrapped, err := signer.WrapProductPEM(&dev.Private.PublicKey, productPEM)
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/print-agent/fiscal-signing/register":
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "installation_id": "inst-cloud-1", "status": "registered"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/print-agent/fiscal-signing/provision":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"installation_id":        "inst-cloud-1",
				"signing_key_version":    1,
				"product_public_key_pem": pub,
				"wrapped_private_key":    wrapped,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	svc := service.New(db, nil, nil, dataDir, storeID)
	svc.SetCloudProvision(service.CloudProvision{
		APIBase: srv.URL, JWT: "test-jwt", DeviceID: "11111111-1111-4111-8111-111111111111",
	})
	st, err := svc.ActivateFromCloud(context.Background(), storeID)
	if err != nil {
		t.Fatal(err)
	}
	if !st.ActivatedOK {
		t.Fatalf("expected activated: %+v", st)
	}
	var n int
	_ = db.SQL.QueryRow(`SELECT COUNT(1) FROM signing_keys WHERE status='ACTIVE'`).Scan(&n)
	if n != 1 {
		t.Fatalf("signing_keys rows: %d", n)
	}
	var instDevice string
	_ = db.SQL.QueryRow(`SELECT device_id FROM agent_installations WHERE installation_id=?`, "inst-cloud-1").Scan(&instDevice)
	if instDevice != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("device_id: %q", instDevice)
	}
}

func TestWrapRoundtripRSA(t *testing.T) {
	// sanity: GenerateDeviceKey + Wrap + Unwrap
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKIXPublicKey(&k.PublicKey)
	_ = pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	pemBytes := []byte("hello-product-pem")
	wrapped, err := signer.WrapProductPEM(&k.PublicKey, pemBytes)
	if err != nil {
		t.Fatal(err)
	}
	out, err := signer.UnwrapProductPEM(k, wrapped)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(pemBytes) {
		t.Fatalf("unwrap mismatch")
	}
}
