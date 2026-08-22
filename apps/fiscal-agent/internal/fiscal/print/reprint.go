package print

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"farvoo-fiscal-agent/internal/fiscal/domain"
)

// ClonePayloadForReprint is the ONLY reprint snapshot builder (clone ORIGINAL payload, patch purpose).
func ClonePayloadForReprint(originalJSON []byte) (payloadJSON []byte, payloadHash string, err error) {
	var p Payload
	if err := json.Unmarshal(originalJSON, &p); err != nil {
		return nil, "", fmt.Errorf("print: reprint unmarshal: %w", err)
	}
	p.PrintPurpose = string(domain.PrintReprint)
	p.PayloadHash = ""
	raw, err := json.Marshal(&p)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(raw)
	payloadHash = hex.EncodeToString(sum[:])
	p.PayloadHash = payloadHash
	raw, err = json.Marshal(&p)
	if err != nil {
		return nil, "", err
	}
	return raw, payloadHash, nil
}
