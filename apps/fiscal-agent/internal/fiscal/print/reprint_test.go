package print_test

import (
	"encoding/json"
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/domain"
	fiscalprint "farvoo-fiscal-agent/internal/fiscal/print"
)

func TestClonePayloadForReprint(t *testing.T) {
	orig := &fiscalprint.Payload{
		Version:      fiscalprint.PayloadVersion,
		PayloadHash:  "abc",
		DocumentID:   "inv-1",
		DocumentType: "FT",
		PrintPurpose: string(domain.PrintOriginal),
		InvoiceNo:    "FT X/1",
	}
	raw, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}
	clone, hash, err := fiscalprint.ClonePayloadForReprint(raw)
	if err != nil {
		t.Fatal(err)
	}
	var p fiscalprint.Payload
	if err := json.Unmarshal(clone, &p); err != nil {
		t.Fatal(err)
	}
	if p.PrintPurpose != string(domain.PrintReprint) {
		t.Fatalf("purpose %q", p.PrintPurpose)
	}
	if p.PayloadHash != hash {
		t.Fatalf("hash mismatch")
	}
	if p.DocumentID != "inv-1" || p.InvoiceNo != "FT X/1" {
		t.Fatalf("clone mutated invoice fields")
	}
}
