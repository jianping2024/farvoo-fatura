// Package protect seals secrets for at_credentials and device keys.
// ONLY Seal/Open — no second wrap path.
package protect

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

// MetaJSON is stored in at_credentials.wrap_meta.
type Meta struct {
	Scheme string `json:"scheme"` // dpapi | file_aes
	V      int    `json:"v"`
}

// Seal encrypts plaintext. Returns ciphertext + wrap_meta JSON. salt always unused (NULL in DB).
func Seal(dataDir string, plaintext []byte) (ciphertext string, wrapMeta string, err error) {
	scheme := "file_aes"
	if runtime.GOOS == "windows" {
		scheme = "dpapi"
	}
	switch scheme {
	case "dpapi":
		ct, err := sealDPAPI(plaintext)
		if err != nil {
			return "", "", err
		}
		meta, _ := json.Marshal(Meta{Scheme: "dpapi", V: 1})
		return ct, string(meta), nil
	default:
		ct, err := sealFileAES(dataDir, plaintext)
		if err != nil {
			return "", "", err
		}
		meta, _ := json.Marshal(Meta{Scheme: "file_aes", V: 1})
		return ct, string(meta), nil
	}
}

// Open decrypts using wrap_meta scheme.
func Open(dataDir string, ciphertext string, wrapMeta string) ([]byte, error) {
	var m Meta
	if err := json.Unmarshal([]byte(wrapMeta), &m); err != nil {
		return nil, fmt.Errorf("protect: wrap_meta: %w", err)
	}
	switch m.Scheme {
	case "dpapi":
		return openDPAPI(ciphertext)
	case "file_aes":
		return openFileAES(dataDir, ciphertext)
	default:
		return nil, fmt.Errorf("protect: unknown scheme %q", m.Scheme)
	}
}

func wrapKeyPath(dataDir string) string {
	return filepath.Join(dataDir, "wrap.key")
}

func loadOrCreateFileKey(dataDir string) ([]byte, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	p := wrapKeyPath(dataDir)
	if b, err := os.ReadFile(p); err == nil && len(b) == 32 {
		return b, nil
	}
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	if err := os.WriteFile(p, key, 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

func sealFileAES(dataDir string, plaintext []byte) (string, error) {
	key, err := loadOrCreateFileKey(dataDir)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
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
	out := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

func openFileAES(dataDir string, ciphertext string) ([]byte, error) {
	key, err := loadOrCreateFileKey(dataDir)
	if err != nil {
		return nil, err
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize() {
		return nil, fmt.Errorf("protect: ciphertext too short")
	}
	nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ct, nil)
}
