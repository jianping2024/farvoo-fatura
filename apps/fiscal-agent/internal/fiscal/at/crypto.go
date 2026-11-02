package at

import (
	"crypto/aes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"time"
)

// AuthMaterial is the encrypted UsernameToken fields required by AT webservice manuals.
// ONLY EncryptATAuth produces this for live SOAP (series + transport share this path).
type AuthMaterial struct {
	PasswordB64 string // Base64(AES-128-ECB(password))
	NonceB64    string // Base64(RSA-PKCS1v15(aesKey))
	CreatedB64  string // Base64(AES-128-ECB(createdUTC))
	CreatedUTC  string // plaintext Created used as AES input (ISO-8601 UTC)
}

// EncryptOptions controls non-secret inputs for tests. Production leaves zero values.
type EncryptOptions struct {
	// AESKey must be 16 bytes when set; nil → crypto/rand.
	AESKey []byte
	// Now overrides wall clock for Created; zero → time.Now().UTC().
	Now time.Time
}

// EncryptATAuth is the ONLY AT SOAP password-encryption path (手册: AES-128-ECB + RSA-PKCS1).
// Password and Created are AES-encrypted with a fresh (or injected) 16-byte key;
// that key is RSA-encrypted with ChavePublicaAT and placed in Nonce.
func EncryptATAuth(password string, pub *rsa.PublicKey, opts EncryptOptions) (*AuthMaterial, error) {
	if pub == nil {
		return nil, fmt.Errorf("at: password public key required")
	}
	if password == "" {
		return nil, fmt.Errorf("at: password required")
	}
	aesKey := opts.AESKey
	if aesKey == nil {
		aesKey = make([]byte, 16)
		if _, err := rand.Read(aesKey); err != nil {
			return nil, fmt.Errorf("at: aes key: %w", err)
		}
	}
	if len(aesKey) != 16 {
		return nil, fmt.Errorf("at: aes key must be 16 bytes")
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	// Manual / community common form: yyyy-MM-ddTHH:mm:ss.000Z
	created := now.Format("2006-01-02T15:04:05.000Z")

	passCT, err := encryptAES128ECB(aesKey, []byte(password))
	if err != nil {
		return nil, err
	}
	createdCT, err := encryptAES128ECB(aesKey, []byte(created))
	if err != nil {
		return nil, err
	}
	nonceCT, err := rsa.EncryptPKCS1v15(rand.Reader, pub, aesKey)
	if err != nil {
		return nil, fmt.Errorf("at: rsa encrypt aes key: %w", err)
	}

	return &AuthMaterial{
		PasswordB64: base64.StdEncoding.EncodeToString(passCT),
		NonceB64:    base64.StdEncoding.EncodeToString(nonceCT),
		CreatedB64:  base64.StdEncoding.EncodeToString(createdCT),
		CreatedUTC:  created,
	}, nil
}

// LoadPasswordPublicKey loads ChavePublicaAT (.cer PEM or DER). ONLY loader for AT password pubkey.
func LoadPasswordPublicKey(path string) (*rsa.PublicKey, error) {
	path = trimPath(path)
	if path == "" {
		return nil, fmt.Errorf("at: password pubkey path empty")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("at: read password pubkey: %w", err)
	}
	der := data
	if block, _ := pem.Decode(data); block != nil {
		der = block.Bytes
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("at: parse password pubkey cert: %w", err)
	}
	pub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("at: password pubkey is not RSA")
	}
	return pub, nil
}

func encryptAES128ECB(key, plain []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("at: aes: %w", err)
	}
	padded := pkcs7Pad(plain, aes.BlockSize)
	out := make([]byte, len(padded))
	for i := 0; i < len(padded); i += aes.BlockSize {
		block.Encrypt(out[i:i+aes.BlockSize], padded[i:i+aes.BlockSize])
	}
	return out, nil
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	pad := blockSize - (len(data) % blockSize)
	out := make([]byte, len(data)+pad)
	copy(out, data)
	for i := len(data); i < len(out); i++ {
		out[i] = byte(pad)
	}
	return out
}

// decryptAES128ECB is test/helper only (round-trip asserts). Not used by live SOAP.
func decryptAES128ECB(key, cipherText []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(cipherText) == 0 || len(cipherText)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("at: bad ciphertext length")
	}
	out := make([]byte, len(cipherText))
	for i := 0; i < len(cipherText); i += aes.BlockSize {
		block.Decrypt(out[i:i+aes.BlockSize], cipherText[i:i+aes.BlockSize])
	}
	return pkcs7Unpad(out, aes.BlockSize)
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, fmt.Errorf("at: bad padding length")
	}
	pad := int(data[len(data)-1])
	if pad == 0 || pad > blockSize || pad > len(data) {
		return nil, fmt.Errorf("at: bad pkcs7 pad")
	}
	for i := 0; i < pad; i++ {
		if data[len(data)-1-i] != byte(pad) {
			return nil, fmt.Errorf("at: bad pkcs7 pad bytes")
		}
	}
	return data[:len(data)-pad], nil
}

func trimPath(p string) string { return strings.TrimSpace(p) }
