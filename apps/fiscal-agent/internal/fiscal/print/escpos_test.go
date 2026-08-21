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
		"Qtd Preco IVA%-Desc",
		"Soma",
		"2.00x",
		"19.95",
		"23%-",
		"39.90",
		"Liquido",
		"TOTAL",
		"Numerario",
		"Resumo IVA",
		"Taxa",
		"ATCUD: CSDF7T5H-3",
		"Processado por programa certificado n. 0/AT",
		"Hash: 0L2I",
	}
	for _, s := range mustContain {
		if !strings.Contains(plain, s) {
			t.Fatalf("missing %q in:\n%s", s, plain)
		}
	}
	if strings.Contains(plain, "FT: FT FT") || strings.Contains(plain, "FT: ") {
		t.Fatalf("bad FT prefix:\n%s", plain)
	}
	// single-line item: no orphan name-only line before qty row
	lines := strings.Split(plain, "\n")
	var itemIdx = -1
	for i, line := range lines {
		if strings.Contains(line, "2.00x") && strings.Contains(line, "23%-") {
			itemIdx = i
			break
		}
	}
	if itemIdx < 0 {
		t.Fatalf("missing single-line item:\n%s", plain)
	}
	if !strings.HasSuffix(strings.TrimRight(lines[itemIdx], " "), "39.90") {
		t.Fatalf("line gross not right-aligned on item row: %q", lines[itemIdx])
	}
	if utf8.RuneCountInString(lines[itemIdx]) > receiptWidth+1 {
		t.Fatalf("item row overwide: %q", lines[itemIdx])
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
		t.Fatalf("raw decimal VAT:\n%s", plain)
	}

	if !bytes.Contains(raw, escposenc.SelectCodeTable(escposenc.CodeTableWPC1252)) {
		t.Fatal("missing ESC t 16")
	}
	// header centered before invoice left
	center := []byte{0x1B, 0x61, 1}
	left := []byte{0x1B, 0x61, 0}
	ci := bytes.Index(raw, center)
	if ci < 0 {
		t.Fatal("missing ESC a 1 for centered header")
	}
	li := bytes.Index(raw[ci:], left)
	if li < 0 {
		t.Fatal("missing ESC a 0 after header")
	}
	inv := bytes.Index(raw, []byte("FT FT2026DEMO01/3"))
	if inv < 0 || inv < ci+li {
		t.Fatal("invoice line must follow left-align after header")
	}
	// QR also centered
	qrMarker := []byte{0x1D, 0x28, 0x6B, 0x03, 0x00, 0x31, 0x43, 0x04}
	qi := bytes.Index(raw, qrMarker)
	if qi < 0 {
		t.Fatal("QR module size 4 missing")
	}
	if bytes.LastIndex(raw[:qi], center) < 0 {
		t.Fatal("QR should be preceded by center align")
	}
	if !bytes.Contains(raw, []byte{0x1B, 0x45, 1}) {
		t.Fatal("merchant name should use bold on")
	}
}

func TestFormatItemLine_SingleRow(t *testing.T) {
	got := formatItemLine("2.00", "19.95", "0.23", "Guarana Antarctica", "39.90", 32)
	if utf8.RuneCountInString(got) != 32 {
		t.Fatalf("width=%d %q", utf8.RuneCountInString(got), got)
	}
	if !strings.HasPrefix(got, "2.00x 19.95 23%-") {
		t.Fatalf("prefix: %q", got)
	}
	if !strings.HasSuffix(got, "39.90") {
		t.Fatalf("suffix: %q", got)
	}
	if strings.Contains(got, "\n") {
		t.Fatal("must be single line")
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
		t.Fatalf("expected Windows-1252 Guaraná %v", encName)
	}
	if bytes.Contains(raw, []byte("Guaraná")) {
		t.Fatal("UTF-8 Guaraná must not appear raw")
	}
}

func TestMoneyRow_FitsWidth(t *testing.T) {
	got := moneyRow("Numerario", "44.30", 32)
	if utf8.RuneCountInString(got) != 32 {
		t.Fatalf("len=%d %q", utf8.RuneCountInString(got), got)
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
}

func decodeTicketText(raw []byte) string {
	var b strings.Builder
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if c == 0x1B || c == 0x1D {
			i++
			for i < len(raw) && raw[i] != '\n' && raw[i] < 0x20 {
				i++
			}
			for i < len(raw) && raw[i] != '\n' && (raw[i] < 0x20 || raw[i] >= 0x7F) {
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
			b.WriteRune(rune(c))
		}
	}
	return b.String()
}
