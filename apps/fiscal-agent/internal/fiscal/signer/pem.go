package signer

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
)

// PEMSigner signs Hash payloads with RSA-SHA1 (PKCS#1 v1.5). ONLY signing implementation for P0/dev.
type PEMSigner struct {
	key     *rsa.PrivateKey
	version int
}

// LoadPEMFile loads a PKCS#1 or PKCS#8 RSA private key PEM.
func LoadPEMFile(path string, keyVersion int) (*PEMSigner, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return LoadPEM(b, keyVersion)
}

// LoadPEM parses PEM bytes.
func LoadPEM(pemBytes []byte, keyVersion int) (*PEMSigner, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("signer: no PEM block")
	}
	var key *rsa.PrivateKey
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		key = k
	} else if k8, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		rk, ok := k8.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("signer: PKCS8 is not RSA")
		}
		key = rk
	} else {
		return nil, fmt.Errorf("signer: parse private key: %w", err)
	}
	if key.N.BitLen() < 1024 {
		return nil, fmt.Errorf("signer: RSA key too small (%d bits)", key.N.BitLen())
	}
	return &PEMSigner{key: key, version: keyVersion}, nil
}

// Sign returns Base64 Hash, HashControl (=key version), and key version.
func (s *PEMSigner) Sign(payload string) (hashBase64 string, hashControl int, keyVersion int, err error) {
	sum := sha1.Sum([]byte(payload))
	sig, err := rsa.SignPKCS1v15(rand.Reader, s.key, crypto.SHA1, sum[:])
	if err != nil {
		return "", 0, 0, err
	}
	return base64.StdEncoding.EncodeToString(sig), s.version, s.version, nil
}

// PublicKeyPEM exports the public key (for seed / self-check).
func (s *PEMSigner) PublicKeyPEM() (string, error) {
	der, err := x509.MarshalPKIXPublicKey(&s.key.PublicKey)
	if err != nil {
		return "", err
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})), nil
}
