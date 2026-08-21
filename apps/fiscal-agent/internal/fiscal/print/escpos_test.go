package print

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"

	"farvoo-fiscal-agent/internal/escposenc"
)

func TestRenderESCPOS_LayoutP0(t *testing.T) {
	p := &Payload{
		DocumentType: "FT",
		InvoiceNo:    "FT FT2026DEMO01/3",
		PrintPurpose: "ORIGINAL",
		IssuedAt:     "2026-08-21T18:26:25",
		Merchant: MerchantBlock{
			LegalName: "Farvoo Demo Lda", Address: "Rua Demo 1, 1000-001 Lisboa",
			TaxRegistrationNumber: "517535009",
		},
		Customer: CustomerBlock{TaxID: "999999990", CompanyName: "Consumidor Final"},
		Lines: []LineBlock{{
			DisplayName: "Guarana Antarctica", Quantity: "2.00", UnitPriceGross: "19.95",
			VATRate: "0.23", LineGross: "39.90",
		}},
		TaxSummary: []TaxSummaryRow{{
			VATRate: "0.23", TaxBase: "32.44", TaxAmount: "7.46", Gross: "39.90",
		}},
		Totals: TotalsBlock{NetTotal: "32.44", TaxPayable: "7.46", GrossTotal: "39.90"},
		Payments: []PaymentBlock{{Method: "CASH", Amount: "39.90"}},
		Compliance: ComplianceBlock{
			ATCUD:             "CSDF7T5H-3",
			QR:                QRBlock{Content: "A:517535009*B:999999990*C:PT"},
			HashControlChars:  "0L2I",
			CertificationLine: "Processado por programa certificado n. 0/AT",
		},
	}
	raw := RenderESCPOS(p)
	plain := decodeTicketText(raw)

	mustContain := []string{
		"Farvoo Demo Lda",
		"NIF: PT517535009",
		"FT FT2026DEMO01/3",
		"21/08/2026 18:26",
		"1a Via - Original",
		"Cliente: Consumidor Final",
		"IVA 23%",
		"Liquido",
		"TOTAL",
		"Numerario",
		"Resumo IVA",
		"Taxa",
		"23%",
		"ATCUD: CSDF7T5H-3",
		"Processado por programa certificado n. 0/AT",
		"Hash: 0L2I",
	}
	for _, s := range mustContain {
		if !strings.Contains(plain, s) {
			t.Fatalf("missing %q in:\n%s", s, plain)
		}
	}
	if strings.Contains(plain, "FT: FT FT") || strings.Count(plain, "FT FT FT") > 0 {
		t.Fatalf("duplicate FT prefix:\n%s", plain)
	}
	if strings.Contains(plain, "FT: ") {
		t.Fatalf("must not use FT: prefix:\n%s", plain)
	}
	atcud := strings.Index(plain, "ATCUD:")
	cert := strings.Index(plain, "Processado por programa certificado")
	hash := strings.Index(plain, "Hash:")
	resumo := strings.Index(plain, "Resumo IVA")
	total := strings.Index(plain, "TOTAL")
	if !(total >= 0 && resumo > total && atcud > resumo && cert > atcud && hash > cert) {
		t.Fatalf("block order wrong total=%d resumo=%d atcud=%d cert=%d hash=%d\n%s",
			total, resumo, atcud, cert, hash, plain)
	}
	if strings.Contains(plain, "IVA 0.23") {
		t.Fatalf("raw decimal VAT on ticket:\n%s", plain)
	}

	// money rows must fit receiptWidth (no orphan amount on next line from moneyRow alone)
	for _, line := range strings.Split(plain, "\n") {
		if utf8.RuneCountInString(line) > receiptWidth+2 { // allow tiny slack for decode noise
			if strings.Contains(line, "Numerario") || strings.Contains(line, "TOTAL") || strings.Contains(line, "Liquido") {
				t.Fatalf("overwide money row %d runes: %q", utf8.RuneCountInString(line), line)
			}
		}
	}

	// ESC t 16 present
	if !bytes.Contains(raw, escposenc.SelectCodeTable(escposenc.CodeTableWPC1252)) {
		t.Fatal("missing ESC t 16 code table select")
	}
	// QR center then left
	center := []byte{0x1B, 0x61, 1}
	left := []byte{0x1B, 0x61, 0}
	ci := bytes.Index(raw, center)
	if ci < 0 {
		t.Fatal("missing ESC a 1 before QR")
	}
	if bytes.Index(raw[ci:], left) < 0 {
		t.Fatal("missing ESC a 0 after QR")
	}
	// module size 4
	if !bytes.Contains(raw, []byte{0x1D, 0x28, 0x6B, 0x03, 0x00, 0x31, 0x43, 0x04}) {
		t.Fatal("QR module size must be 4")
	}
}

func TestRenderESCPOS_AccentEncoding(t *testing.T) {
	p := &Payload{
		DocumentType: "FT",
		InvoiceNo:    "FT X/1",
		PrintPurpose: "ORIGINAL",
		IssuedAt:     "2026-08-21T12:00:00",
		Merchant:     MerchantBlock{LegalName: "Demo", TaxRegistrationNumber: "1"},
		Lines: []LineBlock{{
			DisplayName: "Guaraná", Quantity: "1.00", UnitPriceGross: "1.00",
			VATRate: "0.23", LineGross: "1.00",
		}},
		Totals: TotalsBlock{GrossTotal: "1.00"},
		Compliance: ComplianceBlock{
			ATCUD: "A-1",
			QR:    QRBlock{Content: "A:1"},
		},
	}
	raw := RenderESCPOS(p)
	encName := escposenc.Windows1252("Guaraná")
	if !bytes.Contains(raw, encName) {
		t.Fatalf("expected Windows-1252 Guaraná %v in output", encName)
	}
	if bytes.Contains(raw, []byte("Guaraná")) {
		t.Fatal("UTF-8 Guaraná must not appear raw on wire")
	}
}

func TestMoneyRow_FitsWidth(t *testing.T) {
	got := moneyRow("Numerario", "44.30", 32)
	if utf8.RuneCountInString(got) != 32 {
		t.Fatalf("len=%d %q", utf8.RuneCountInString(got), got)
	}
	if !strings.HasSuffix(got, "44.30") {
		t.Fatalf("%q", got)
	}
}

func TestFormatInvoiceLine_NoDoubleFT(t *testing.T) {
	got := formatInvoiceLine("FT", "FT FT2026DEMO01/1")
	if got != "FT FT2026DEMO01/1" {
		t.Fatalf("got %q", got)
	}
	got = formatInvoiceLine("FT", "SERIES/1")
	if got != "FT SERIES/1" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatVATPercent(t *testing.T) {
	if formatVATPercent("0.23") != "23%" {
		t.Fatal(formatVATPercent("0.23"))
	}
	if formatVATPercent("13.00") != "13%" {
		t.Fatal(formatVATPercent("13.00"))
	}
	if formatVATPercent("0") != "0%" {
		t.Fatal(formatVATPercent("0"))
	}
}

// decodeTicketText maps Windows-1252 printable bytes back for assertions; drops ESC/POS cmds roughly.
func decodeTicketText(raw []byte) string {
	var b strings.Builder
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if c == 0x1B || c == 0x1D {
			// skip command payloads heuristically: consume until we hit LF or long gap
			i++
			for i < len(raw) && raw[i] != '\n' && raw[i] < 0x20 {
				i++
			}
			// if next looks like binary length params, skip a few more
			for i < len(raw) && raw[i] != '\n' && (raw[i] < 0x20 || raw[i] >= 0x7F) {
				// keep high latin (1252) — break to emit
				if raw[i] >= 0x80 {
					break
				}
				i++
			}
			i--
			continue
		}
		if c == '\n' {
			b.WriteByte('\n')
			continue
		}
		if c >= 0x20 && c < 0x7F {
			b.WriteByte(c)
			continue
		}
		if c >= 0x80 {
			// best-effort: leave as latin1 rune for Contains checks on ASCII labels only
			b.WriteRune(rune(c))
		}
	}
	return b.String()
}
