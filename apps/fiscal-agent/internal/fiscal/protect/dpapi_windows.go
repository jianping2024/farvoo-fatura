//go:build windows

package protect

import (
	"encoding/base64"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

func sealDPAPI(plaintext []byte) (string, error) {
	in := windows.DataBlob{Size: uint32(len(plaintext)), Data: &plaintext[0]}
	var out windows.DataBlob
	if err := windows.CryptProtectData(&in, nil, nil, 0, nil, 0, &out); err != nil {
		return "", fmt.Errorf("protect: CryptProtectData: %w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	b := unsafe.Slice(out.Data, out.Size)
	cp := make([]byte, len(b))
	copy(cp, b)
	return base64.StdEncoding.EncodeToString(cp), nil
}

func openDPAPI(ciphertext string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, err
	}
	in := windows.DataBlob{Size: uint32(len(raw)), Data: &raw[0]}
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(&in, nil, nil, 0, nil, 0, &out); err != nil {
		return nil, fmt.Errorf("protect: CryptUnprotectData: %w", err)
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	b := unsafe.Slice(out.Data, out.Size)
	cp := make([]byte, len(b))
	copy(cp, b)
	return cp, nil
}
