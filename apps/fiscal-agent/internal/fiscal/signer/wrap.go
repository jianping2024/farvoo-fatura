package signer

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"farvoo-fiscal-agent/internal/fiscal/protect"
)

// DeviceBundle holds device RSA used to wrap the product fiscal key.
type DeviceBundle struct {
	Private *rsa.PrivateKey
	PublicPEM string
}

// GenerateDeviceKey creates a 2048-bit device keypair (NOT the AT product 1024 key).
func GenerateDeviceKey() (*DeviceBundle, error) {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalPKIXPublicKey(&k.PublicKey)
	if err != nil {
		return nil, err
	}
	pub := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
	return &DeviceBundle{Private: k, PublicPEM: pub}, nil
}

// SaveDeviceKey seals the device private key under dataDir (ONLY device-key persistence path).
func SaveDeviceKey(dataDir string, b *DeviceBundle) error {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	der := x509.MarshalPKCS1PrivateKey(b.Private)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der})
	ct, meta, err := protect.Seal(dataDir, pemBytes)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dataDir, "device_private.seal"), []byte(ct), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dataDir, "device_private.meta"), []byte(meta), 0o600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dataDir, "device_public.pem"), []byte(b.PublicPEM), 0o644)
}

// LoadDeviceKey opens the sealed device private key.
func LoadDeviceKey(dataDir string) (*DeviceBundle, error) {
	ct, err := os.ReadFile(filepath.Join(dataDir, "device_private.seal"))
	if err != nil {
		return nil, err
	}
	meta, err := os.ReadFile(filepath.Join(dataDir, "device_private.meta"))
	if err != nil {
		return nil, err
	}
	pemBytes, err := protect.Open(dataDir, string(ct), string(meta))
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("signer: device key PEM missing")
	}
	k, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	pub, err := os.ReadFile(filepath.Join(dataDir, "device_public.pem"))
	if err != nil {
		der, err2 := x509.MarshalPKIXPublicKey(&k.PublicKey)
		if err2 != nil {
			return nil, err
		}
		pub = pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	}
	return &DeviceBundle{Private: k, PublicPEM: string(pub)}, nil
}

type wrapBlob struct {
	EK string `json:"ek"` // RSA-OAEP wrapped AES key (base64)
	IV string `json:"iv"`
	CT string `json:"ct"`
}

// WrapProductPEM is the ONLY product-key wrap helper (hybrid RSA-OAEP + AES-GCM).
func WrapProductPEM(devicePub *rsa.PublicKey, productPEM []byte) (string, error) {
	aesKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, aesKey); err != nil {
		return "", err
	}
	ek, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, devicePub, aesKey, nil)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nil, nonce, productPEM, nil)
	blob, err := json.Marshal(wrapBlob{
		EK: base64.StdEncoding.EncodeToString(ek),
		IV: base64.StdEncoding.EncodeToString(nonce),
		CT: base64.StdEncoding.EncodeToString(ct),
	})
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(blob), nil
}

// UnwrapProductPEM reverses WrapProductPEM.
func UnwrapProductPEM(devicePriv *rsa.PrivateKey, wrapped string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(wrapped)
	if err != nil {
		return nil, err
	}
	var blob wrapBlob
	if err := json.Unmarshal(raw, &blob); err != nil {
		return nil, err
	}
	ek, err := base64.StdEncoding.DecodeString(blob.EK)
	if err != nil {
		return nil, err
	}
	aesKey, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, devicePriv, ek, nil)
	if err != nil {
		return nil, err
	}
	iv, err := base64.StdEncoding.DecodeString(blob.IV)
	if err != nil {
		return nil, err
	}
	ct, err := base64.StdEncoding.DecodeString(blob.CT)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, iv, ct, nil)
}

// UnwrappingSigner loads wrapped product key via device key — ONLY production Sign path for M1+.
type UnwrappingSigner struct {
	dataDir   string
	keyVersion int
	inner     *PEMSigner
}

// NewUnwrappingSigner opens device key + DB wrapped blob (passed in) and builds PEMSigner.
func NewUnwrappingSigner(dataDir string, wrappedPrivateKey string, keyVersion int) (*UnwrappingSigner, error) {
	if len(wrappedPrivateKey) > 10 && wrappedPrivateKey[:10] == "DEV_PLAIN:" {
		if os.Getenv("FISCAL_ALLOW_DEV_KEY") != "1" {
			return nil, fmt.Errorf("signer: DEV_PLAIN forbidden (set FISCAL_ALLOW_DEV_KEY=1 only for legacy M0)")
		}
		inner, err := LoadPEM([]byte(wrappedPrivateKey[10:]), keyVersion)
		if err != nil {
			return nil, err
		}
		return &UnwrappingSigner{dataDir: dataDir, keyVersion: keyVersion, inner: inner}, nil
	}
	dev, err := LoadDeviceKey(dataDir)
	if err != nil {
		return nil, fmt.Errorf("signer: load device key: %w", err)
	}
	pemBytes, err := UnwrapProductPEM(dev.Private, wrappedPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("signer: unwrap product key: %w", err)
	}
	inner, err := LoadPEM(pemBytes, keyVersion)
	if err != nil {
		return nil, err
	}
	return &UnwrappingSigner{dataDir: dataDir, keyVersion: keyVersion, inner: inner}, nil
}

func (u *UnwrappingSigner) Sign(payload string) (hashBase64 string, hashControl int, keyVersion int, err error) {
	return u.inner.Sign(payload)
}
