package at

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"golang.org/x/crypto/pkcs12"
)

// LoadClientTLS is the ONLY mTLS loader for AT webservice clients (series + transport).
// Supports PKCS#12 (.pfx/.p12). Wrong passphrase or missing file returns a clear error.
func LoadClientTLS(certPath, passphrase string) (*tls.Config, error) {
	certPath = trimPath(certPath)
	if certPath == "" {
		return nil, fmt.Errorf("at: client cert path empty")
	}
	raw, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("at: read client cert: %w", err)
	}
	blocks, err := pkcs12.ToPEM(raw, passphrase)
	if err != nil {
		return nil, fmt.Errorf("at: decode client pfx (check passphrase): %w", err)
	}
	var keyPEM, certPEM []byte
	var leafCN string
	for _, bl := range blocks {
		switch bl.Type {
		case "PRIVATE KEY", "RSA PRIVATE KEY":
			keyPEM = append(keyPEM, pem.EncodeToMemory(bl)...)
		case "CERTIFICATE":
			if leafCN == "" {
				if c, e := x509.ParseCertificate(bl.Bytes); e == nil {
					leafCN = c.Subject.CommonName
				}
			}
			certPEM = append(certPEM, pem.EncodeToMemory(bl)...)
		}
	}
	if len(keyPEM) == 0 || len(certPEM) == 0 {
		return nil, fmt.Errorf("at: pfx missing private key or certificate")
	}
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("at: tls key pair: %w", err)
	}
	_ = leafCN
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}
