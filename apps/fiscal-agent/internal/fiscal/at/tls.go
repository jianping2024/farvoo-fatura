package at

import (
	"crypto/tls"
	"fmt"
	"os"

	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

// LoadClientTLS is the ONLY mTLS loader for AT webservice clients (series + transport).
// Supports PKCS#12 (.pfx/.p12) in legacy (3DES/SHA-1, AT public test pfx) and modern
// (AES/PBKDF2/SHA-256, OpenSSL 3 default) formats. Leaf is pinned first in the chain.
// Wrong passphrase or missing file returns a clear error.
func LoadClientTLS(certPath, passphrase string) (*tls.Config, error) {
	certPath = trimPath(certPath)
	if certPath == "" {
		return nil, fmt.Errorf("at: client cert path empty")
	}
	raw, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("at: read client cert: %w", err)
	}
	key, leaf, chain, err := pkcs12.DecodeChain(raw, passphrase)
	if err != nil {
		return nil, fmt.Errorf("at: decode client pfx (check passphrase): %w", err)
	}
	if key == nil || leaf == nil {
		return nil, fmt.Errorf("at: pfx missing private key or certificate")
	}
	tlsCert := tls.Certificate{PrivateKey: key, Leaf: leaf, Certificate: [][]byte{leaf.Raw}}
	for _, c := range chain {
		tlsCert.Certificate = append(tlsCert.Certificate, c.Raw)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}
