package at

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testdataCertDir(t *testing.T) string {
	t.Helper()
	return filepath.Join("testdata", "transport", "certificados", "current")
}

func TestEncryptATAuth_RoundTripAES(t *testing.T) {
	pubPath := filepath.Join(testdataCertDir(t), "ChavePublicaAT.cer")
	pub, err := LoadPasswordPublicKey(pubPath)
	if err != nil {
		t.Fatal(err)
	}
	aesKey := []byte("0123456789abcdef") // 16 bytes fixed
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	mat, err := EncryptATAuth("testes1234", pub, EncryptOptions{AESKey: aesKey, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if mat.CreatedUTC != "2026-11-02T10:00:00.000Z" {
		t.Fatalf("created plaintext: got %q", mat.CreatedUTC)
	}
	passCT, err := base64.StdEncoding.DecodeString(mat.PasswordB64)
	if err != nil {
		t.Fatal(err)
	}
	gotPass, err := decryptAES128ECB(aesKey, passCT)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotPass) != "testes1234" {
		t.Fatalf("password round-trip: %q", gotPass)
	}
	createdCT, err := base64.StdEncoding.DecodeString(mat.CreatedB64)
	if err != nil {
		t.Fatal(err)
	}
	gotCreated, err := decryptAES128ECB(aesKey, createdCT)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotCreated) != mat.CreatedUTC {
		t.Fatalf("created round-trip: %q", gotCreated)
	}
	nonceRaw, err := base64.StdEncoding.DecodeString(mat.NonceB64)
	if err != nil {
		t.Fatal(err)
	}
	// RSA-4096 PKCS1 ciphertext is 512 bytes.
	if len(nonceRaw) != pub.Size() {
		t.Fatalf("nonce len=%d want %d", len(nonceRaw), pub.Size())
	}
	// Same AES key + same Created → deterministic AES ciphertexts (RSA nonce is randomized).
	mat2, err := EncryptATAuth("testes1234", pub, EncryptOptions{AESKey: aesKey, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if mat2.PasswordB64 != mat.PasswordB64 || mat2.CreatedB64 != mat.CreatedB64 {
		t.Fatal("AES ciphertexts not stable for fixed key+time")
	}
}

func TestEncryptATAuth_RequiresPasswordAndKey(t *testing.T) {
	pub, err := LoadPasswordPublicKey(filepath.Join(testdataCertDir(t), "ChavePublicaAT.cer"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := EncryptATAuth("", pub, EncryptOptions{}); err == nil {
		t.Fatal("expected empty password error")
	}
	if _, err := EncryptATAuth("x", nil, EncryptOptions{}); err == nil {
		t.Fatal("expected nil pub error")
	}
	if _, err := EncryptATAuth("x", pub, EncryptOptions{AESKey: []byte("short")}); err == nil {
		t.Fatal("expected short aes key error")
	}
}

func TestLoadPasswordPublicKey_Missing(t *testing.T) {
	if _, err := LoadPasswordPublicKey("/no/such/ChavePublicaAT.cer"); err == nil {
		t.Fatal("expected missing file error")
	}
}

func TestLoadClientTLS_OK(t *testing.T) {
	pfx := filepath.Join(testdataCertDir(t), "TesteWebservices.pfx")
	cfg, err := LoadClientTLS(pfx, "TESTEwebservice")
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil || len(cfg.Certificates) != 1 {
		t.Fatalf("tls config: %#v", cfg)
	}
	if len(cfg.Certificates[0].Certificate) < 1 {
		t.Fatal("leaf cert missing")
	}
}

func TestLoadClientTLS_BadPass(t *testing.T) {
	pfx := filepath.Join(testdataCertDir(t), "TesteWebservices.pfx")
	if _, err := LoadClientTLS(pfx, "wrong-pass"); err == nil {
		t.Fatal("expected bad passphrase error")
	}
}

func TestLoadClientTLS_Missing(t *testing.T) {
	if _, err := LoadClientTLS("/no/such.pfx", "x"); err == nil {
		t.Fatal("expected missing file error")
	}
}

func TestBuildUsernameTokenAndEnvelope(t *testing.T) {
	mat := &AuthMaterial{
		PasswordB64: "pWd=",
		NonceB64:    "nNc=",
		CreatedB64:  "cRd=",
	}
	tok, err := BuildUsernameTokenXML(`599999993/37`, mat)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(tok, "<wss:Username>599999993/37</wss:Username>") {
		t.Fatalf("token: %s", tok)
	}
	sec := BuildSecurityHeaderXML(tok)
	env := WrapSOAPEnvelope(sec, `<ns:ping xmlns:ns="urn:test"/>`)
	if !strings.Contains(env, "<S:Header>") || !strings.Contains(env, "<S:Body>") {
		t.Fatalf("envelope: %s", env)
	}
	if !strings.Contains(env, "oasis-200401-wss-wssecurity-secext") {
		t.Fatal("missing wsse ns")
	}
}

func TestLiveMaterialsFromEnv_Aliases(t *testing.T) {
	t.Setenv("FISCAL_AT_CLIENT_CERT", "")
	t.Setenv("FISCAL_AT_CLIENT_CERT_PASS", "")
	t.Setenv("FISCAL_AT_PASSWORD_PUBKEY", "")
	t.Setenv("FISCAL_AT_TRANSPORT_CERT", "/legacy/cert.pfx")
	t.Setenv("FISCAL_AT_TRANSPORT_CERT_PASS", "legacy-pass")
	t.Setenv("FISCAL_AT_TRANSPORT_PUBKEY", "/legacy/pub.cer")
	t.Setenv("FISCAL_AT_TRANSPORT_USER", "599999993/37")
	t.Setenv("FISCAL_AT_TRANSPORT_PASS", "testes1234")
	m := LiveMaterialsFromEnv()
	if m.ClientCertPath != "/legacy/cert.pfx" || m.ClientCertPass != "legacy-pass" {
		t.Fatalf("legacy alias: %+v", m)
	}
	if m.PasswordPubKeyPath != "/legacy/pub.cer" {
		t.Fatalf("pubkey alias: %+v", m)
	}
	if m.ClientUser != "599999993/37" || m.ClientPass != "testes1234" {
		t.Fatalf("user alias: %+v", m)
	}
	t.Setenv("FISCAL_AT_CLIENT_CERT", "/new/cert.pfx")
	t.Setenv("FISCAL_AT_CLIENT_CERT_PASS", "new-pass")
	t.Setenv("FISCAL_AT_PASSWORD_PUBKEY", "/new/pub.cer")
	m2 := LiveMaterialsFromEnv()
	if m2.ClientCertPath != "/new/cert.pfx" || m2.PasswordPubKeyPath != "/new/pub.cer" {
		t.Fatalf("prefer new names: %+v", m2)
	}
	if miss := m2.MissingRequired(); len(miss) != 0 {
		t.Fatalf("missing: %v", miss)
	}
}

func TestMockUnaffectedByA0(t *testing.T) {
	// Ensure default env still builds mock clients (A0 must not force live).
	_ = os.Unsetenv("FISCAL_AT_ENV")
	c := NewFromEnv()
	if _, ok := c.(*MockClient); !ok {
		t.Fatalf("series client type %T", c)
	}
	tc := NewTransportFromEnv()
	if _, ok := tc.(*MockTransportClient); !ok {
		t.Fatalf("transport client type %T", tc)
	}
	res, err := c.RegisterSeries(nil, RegisterRequest{Username: "u", Password: "p", SeriesCode: "X"})
	if err != nil || len(res.ValidationCode) != 8 {
		t.Fatalf("mock register: %v %#v", err, res)
	}
	tr, err := tc.RegisterDocument(nil, TransportDocument{DocumentNumber: "GR X/1", MovementType: "GR"})
	if err != nil || !strings.HasPrefix(tr.ATDocCodeID, "MOCK") {
		t.Fatalf("mock transport: %v %#v", err, tr)
	}
}
