package compliance_test

import (
	"testing"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/compliance"
)

func TestFormatSequenceSingleWriter(t *testing.T) {
	if compliance.FormatSequence(42) != "42" {
		t.Fatalf("FormatSequence")
	}
	no := compliance.FormatInvoiceNo("FT", "FT2026DEMO01", 1)
	if no != "FT FT2026DEMO01/1" {
		t.Fatalf("invoice no: %s", no)
	}
	at := compliance.FormatATCUD("CSDF7T5H", 1)
	if at != "CSDF7T5H-1" {
		t.Fatalf("atcud: %s", at)
	}
}

func TestBuildSignPayload(t *testing.T) {
	got := compliance.BuildSignPayload("2010-05-18", "2010-05-18T11:22:19", "FAC 001/14", "3.12", "")
	want := "2010-05-18;2010-05-18T11:22:19;FAC 001/14;3.12;"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestBuildQR(t *testing.T) {
	hash := "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnop="
	qr, err := compliance.BuildQR(compliance.QRInput{
		IssuerNIF: "517535009", CustomerTaxID: "999999990", CustomerCountry: "PT",
		DocumentType: "FT", DocumentStatus: "N", InvoiceDate: time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC),
		InvoiceNo: "FT FT2026DEMO01/1", ATCUD: "CSDF7T5H-1",
		Buckets: []compliance.TaxBucket{{Rate: "0.23", TaxBase: "10.16", TaxAmount: "2.34"}},
		TaxPayable: "2.34", GrossTotal: "12.50", HashBase64: hash, SoftwareCertificateNum: "0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if qr == "" || qr[0:2] != "A:" {
		t.Fatalf("qr: %s", qr)
	}
	if !containsAll(qr, []string{"H:CSDF7T5H-1", "O:12.50", "N:2.34", "Q:AKU4"}) {
		t.Fatalf("missing fields: %s", qr)
	}
}

func containsAll(s string, parts []string) bool {
	for _, p := range parts {
		if !contains(s, p) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
