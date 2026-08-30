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
		TableDisplayName: "018",
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
			CertificationLine: "Processado por programa certificado n. 369/AT",
		},
	}
	raw := RenderESCPOS(p)
	plain := decodeTicketText(raw)

	mustContain := []string{
		"Farvoo Demo Lda",
		"NIF: PT517535009",
		"Fatura No.: FT FT2026DEMO01/3",
		"21/08/2026 18:26",
		"1a Via - Original",
		"MESA: 018",
		"Cliente: Consumidor Final",
		"Qtd",
		"Preco",
		"IVA%-Desc",
		"2.00x",
		"23%-",
		"Liquido",
		"TOTAL",
		"Numerario",
		"Resumo IVA",
		"0L2I-Processado por programa",
		"certificado n. 369/AT",
		"ATCUD: CSDF7T5H-3",
	}
	for _, s := range mustContain {
		if !strings.Contains(plain, s) {
			t.Fatalf("missing %q in:\n%s", s, plain)
		}
	}
	if strings.Contains(plain, "Qtd Preco") {
		t.Fatal("old single-space Qtd Preco header must not remain")
	}
	// Symmetric sandwich: rule \n header+Soma \n rule \n — no blank lines either side.
	ruleLine := strings.Repeat("-", receiptWidth)
	headerLine := moneyRow(formatItemLinesHeader(), "Soma", receiptWidth)
	sandwich := ruleLine + "\n" + headerLine + "\n" + ruleLine + "\n"
	if !strings.Contains(plain, sandwich) {
		t.Fatalf("item header must be hugged by equal rules (no blank lines):\nwant substring:\n%s\ngot:\n%s", sandwich, plain)
	}
	if strings.Contains(plain, "Hash:") {
		t.Fatalf("must not print Hash: line:\n%s", plain)
	}
	if strings.Contains(plain, "FT: ") {
		t.Fatalf("bad FT: prefix:\n%s", plain)
	}
	if strings.Contains(plain, "n.º") || strings.Contains(plain, "n.°") {
		t.Fatalf("ticket face must not use ordinal º:\n%s", plain)
	}

	// order: Via → MESA → Cliente; TOTAL → Resumo → cert → ATCUD
	via := strings.Index(plain, "1a Via - Original")
	mesa := strings.Index(plain, "MESA: 018")
	cliente := strings.Index(plain, "Cliente:")
	if !(via >= 0 && mesa > via && cliente > mesa) {
		t.Fatalf("MESA order via=%d mesa=%d cliente=%d\n%s", via, mesa, cliente, plain)
	}
	total := strings.Index(plain, "TOTAL")
	resumo := strings.Index(plain, "Resumo IVA")
	cert := strings.Index(plain, "0L2I-Processado")
	atcud := strings.Index(plain, "ATCUD:")
	if !(total >= 0 && resumo > total && cert > resumo && atcud > cert) {
		t.Fatalf("block order total=%d resumo=%d cert=%d atcud=%d\n%s", total, resumo, cert, atcud, plain)
	}
	// Long cert must print as two lines (prefer split after "programa ").
	if !strings.Contains(plain, "0L2I-Processado por programa\ncertificado n. 369/AT") {
		t.Fatalf("long cert must wrap to 2 lines:\n%s", plain)
	}

	// Store name: 1×2 bold, reset to 1×1, one blank LF, then address at 1×1
	nameEnc := escposenc.Windows1252("Farvoo Demo Lda")
	addrEnc := escposenc.Windows1252("Rua Demo 1, 1000-001 Lisboa")
	nameAt := bytes.Index(raw, nameEnc)
	addrAt := bytes.Index(raw, addrEnc)
	if nameAt < 0 || addrAt <= nameAt {
		t.Fatal("missing store name / address order")
	}
	nameSeg := raw[:nameAt]
	if !bytes.Contains(nameSeg[max(0, len(nameSeg)-16):], []byte{0x1D, 0x21, 0x01}) {
		t.Fatal("LegalName must use GS ! 1×2")
	}
	mid := raw[nameAt+len(nameEnc) : addrAt]
	wantMid := []byte{'\n', 0x1D, 0x21, 0x00, 0x1B, 0x45, 0x00, '\n'}
	if !bytes.Equal(mid, wantMid) {
		t.Fatalf("after LegalName want LF + size/bold reset + blank LF, got %x", mid)
	}

	if bytes.Count(raw, []byte{0x1D, 0x21, 0x01}) < 2 {
		t.Fatal("LegalName and TOTAL must both use GS ! double-height")
	}
	// QR module 6
	if !bytes.Contains(raw, []byte{0x1D, 0x28, 0x6B, 0x03, 0x00, 0x31, 0x43, 0x06}) {
		t.Fatal("QR module size must be 6")
	}
	// feed-then-cut
	if !bytes.Contains(raw, []byte{0x1D, 0x56, 0x42, cutFeedDots}) {
		t.Fatal("must use GS V 66 feed-then-cut")
	}
	if bytes.Contains(raw, []byte{0x1D, 0x56, 0x00}) {
		t.Fatal("must not use immediate GS V 0 cut")
	}

	qi := bytes.Index(raw, []byte{0x1D, 0x28, 0x6B, 0x03, 0x00, 0x31, 0x43, 0x06})
	if qi < 0 {
		t.Fatal("qr marker")
	}
	afterQR := raw[qi:]
	if bytes.Contains(afterQR, []byte("Processado")) || bytes.Contains(afterQR, []byte("Hash:")) || bytes.Contains(afterQR, []byte("ATCUD:")) {
		t.Fatal("no ATCUD/cert/Hash bytes after QR command region")
	}

	if !bytes.Contains(raw, escposenc.SelectCodeTable(escposenc.CodeTableWPC1252)) {
		t.Fatal("missing ESC t 16")
	}
	if !bytes.Contains(raw, []byte{0x1B, 0x61, 1}) {
		t.Fatal("missing center align")
	}
}

func TestFormatCertificationFace(t *testing.T) {
	got := formatCertificationFace("0L2I", "Processado por programa certificado n.º 0/AT")
	if got != "0L2I-Processado por programa certificado n. 0/AT" {
		t.Fatalf("%q", got)
	}
	// Exactly receiptWidth → one line; longer → prefer split after "programa ".
	eqWidth := formatCertificationFaceLines("0L2I", "Processado por programa certificado n. 0/AT", receiptWidth)
	if len(eqWidth) != 1 || eqWidth[0] != got {
		t.Fatalf("eq-width want 1 line, got %#v", eqWidth)
	}
	long := formatCertificationFaceLines("0L2I", "Processado por programa certificado n. 369/AT", receiptWidth)
	if len(long) != 2 || long[0] != "0L2I-Processado por programa" || long[1] != "certificado n. 369/AT" {
		t.Fatalf("long wrap %#v", long)
	}
	short := formatCertificationFaceLines("AB", "ok", receiptWidth)
	if len(short) != 1 || short[0] != "AB-ok" {
		t.Fatalf("short %#v", short)
	}
}

func TestFormatMesaLine(t *testing.T) {
	if formatMesaLine(" 018 ") != "MESA: 018" {
		t.Fatal(formatMesaLine(" 018 "))
	}
	if formatMesaLine("") != "" {
		t.Fatal("empty mesa")
	}
}

func TestRenderESCPOS_NCOriginalReference(t *testing.T) {
	p := &Payload{
		DocumentType: "NC",
		InvoiceNo:    "NC NC2026DEMO01/1",
		PrintPurpose: "ORIGINAL",
		IssuedAt:     "2026-08-21T18:26:25",
		Merchant: MerchantBlock{
			LegalName: "Farvoo Demo Lda", TaxRegistrationNumber: "517535009",
		},
		Customer: CustomerBlock{CompanyName: "Consumidor Final"},
		Lines: []LineBlock{{
			DisplayName: "Item", Quantity: "1", UnitPriceGross: "12.50",
			VATRate: "0.23", LineGross: "12.50",
		}},
		Totals: TotalsBlock{NetTotal: "10.16", TaxPayable: "2.34", GrossTotal: "12.50"},
		Payments: []PaymentBlock{{Method: "CASH", Amount: "12.50"}},
		Compliance: ComplianceBlock{
			OriginalInvoiceNo: "FT FT2026DEMO01/1",
			CreditReason:      "Devolucao",
			ATCUD:             "NCVAL-1",
			CertificationLine: "Processado por programa certificado n. 0/AT",
		},
	}
	plain := decodeTicketText(RenderESCPOS(p))
	for _, want := range []string{
		"Documento original: FT FT2026DEMO01/1",
		"Motivo: Devolucao",
		"Fatura No.: NC NC2026DEMO01/1",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("missing %q in:\n%s", want, plain)
		}
	}
}

func TestFormatFaturaNoLine(t *testing.T) {
	if formatFaturaNoLine("FT FT2026DEMO01/1") != "Fatura No.: FT FT2026DEMO01/1" {
		t.Fatal(formatFaturaNoLine("FT FT2026DEMO01/1"))
	}
}

func TestRenderESCPOS_FaturaNoBoldOnly(t *testing.T) {
	p := &Payload{
		InvoiceNo: "FT FT2026DEMO01/3",
		PrintPurpose: "ORIGINAL",
		IssuedAt:     "2026-08-21T18:26:25",
		Merchant: MerchantBlock{
			LegalName: "Farvoo Demo Lda", Address: "Rua Demo 1",
			TaxRegistrationNumber: "517535009",
		},
		Customer: CustomerBlock{TaxID: "999999990", CompanyName: "Consumidor Final"},
		Totals:   TotalsBlock{NetTotal: "1.00", TaxPayable: "0.00", GrossTotal: "1.00"},
		Compliance: ComplianceBlock{
			ATCUD: "X-1", QR: QRBlock{Content: "A:1"}, HashControlChars: "ABCD",
			CertificationLine: "Processado por programa certificado n. 0/AT",
		},
	}
	raw := RenderESCPOS(p)
	fatura := escposenc.Windows1252("Fatura No.: FT FT2026DEMO01/3")
	date := escposenc.Windows1252("21/08/2026 18:26")
	fi := bytes.Index(raw, fatura)
	di := bytes.Index(raw, date)
	if fi < 0 || di <= fi {
		t.Fatal("fatura/date order")
	}
	before := raw[:fi]
	if !bytes.HasSuffix(before, []byte{0x1B, 0x45, 0x01}) {
		tail := before
		if len(tail) > 8 {
			tail = tail[len(tail)-8:]
		}
		t.Fatalf("Fatura No. must end with ESC E 1 before text, tail %x", tail)
	}
	between := raw[fi+len(fatura) : di]
	// After fatura text: LF, ESC E 0, then date (no GS ! size change on fatura)
	if !bytes.HasPrefix(between, []byte{'\n', 0x1B, 0x45, 0x00}) {
		t.Fatalf("after Fatura want LF + ESC E 0, got %x", between)
	}
	if bytes.Contains(between, []byte{0x1D, 0x21}) {
		t.Fatal("Fatura No. must not use GS ! double-height")
	}
}

func TestFormatItemLine_SingleRow(t *testing.T) {
	got := formatItemLine("2.00", "19.95", "0.23", "Guarana Antarctica", "39.90", receiptWidth)
	if utf8.RuneCountInString(got) != receiptWidth {
		t.Fatalf("width=%d %q", utf8.RuneCountInString(got), got)
	}
	if !strings.HasPrefix(strings.TrimRight(got, " "), "2.00x") {
		t.Fatalf("prefix: %q", got)
	}
	// Qty band + price band must leave a visible gap (not "2.00x 19.95" single space only).
	if strings.Contains(got, "2.00x 19.95") && !strings.Contains(got, "2.00x ") {
		t.Fatalf("qty/price too tight: %q", got)
	}
	qtyEnd := itemQtyBandW
	price := got[qtyEnd : qtyEnd+itemPriceBandW]
	if !strings.Contains(price, "19.95") {
		t.Fatalf("price band %q want 19.95 in %q", price, got)
	}
}

func TestFormatItemLinesHeader_SeparatedBands(t *testing.T) {
	h := formatItemLinesHeader()
	if strings.Contains(h, "Qtd Preco") {
		t.Fatalf("header still single-spaced: %q", h)
	}
	if !strings.HasPrefix(h, "Qtd") || !strings.Contains(h, "Preco") || !strings.Contains(h, "IVA%-Desc") {
		t.Fatalf("header: %q", h)
	}
	if utf8.RuneCountInString(h) < itemQtyBandW+itemPriceBandW+len("IVA%-Desc") {
		t.Fatalf("header too short: %q", h)
	}
}

func TestRenderESCPOS_AccentEncoding(t *testing.T) {
	p := &Payload{
		DocumentType: "FT", InvoiceNo: "FT X/1", PrintPurpose: "ORIGINAL",
		IssuedAt: "2026-08-21T12:00:00",
		Merchant: MerchantBlock{LegalName: "Demo", TaxRegistrationNumber: "1"},
		Lines: []LineBlock{{
			DisplayName: "Guaraná", Quantity: "1.00", UnitPriceGross: "1.00",
			VATRate: "0.23", LineGross: "1.00",
		}},
		Totals:     TotalsBlock{GrossTotal: "1.00"},
		Compliance: ComplianceBlock{ATCUD: "A-1", QR: QRBlock{Content: "A:1"}, HashControlChars: "AbC/"},
	}
	raw := RenderESCPOS(p)
	if !bytes.Contains(raw, escposenc.Windows1252("Guaraná")) {
		t.Fatal("1252 Guaraná missing")
	}
	if bytes.Contains(raw, []byte("Guaraná")) {
		t.Fatal("UTF-8 Guaraná on wire")
	}
	if bytes.Contains(raw, []byte("Hash:")) {
		t.Fatal("no Hash: line")
	}
}

func TestMoneyRow_FitsWidth(t *testing.T) {
	got := moneyRow("Numerario", "44.30", receiptWidth)
	if utf8.RuneCountInString(got) != receiptWidth {
		t.Fatalf("len=%d", utf8.RuneCountInString(got))
	}
}

func TestReceiptWidthMatchesKitchen(t *testing.T) {
	if receiptWidth != 48 {
		t.Fatalf("receiptWidth=%d want 48 (kitchen escposWidth)", receiptWidth)
	}
	sum := 0
	for _, w := range ivaSummaryColWidths {
		sum += w
	}
	if sum != receiptWidth {
		t.Fatalf("ivaSummaryColWidths sum=%d want %d", sum, receiptWidth)
	}
}

func TestRenderESCPOS_RuleFillsWidth(t *testing.T) {
	p := &Payload{
		DocumentType: "FT", InvoiceNo: "FT X/1", PrintPurpose: "ORIGINAL",
		IssuedAt: "2026-08-21T12:00:00",
		Merchant: MerchantBlock{LegalName: "Demo", TaxRegistrationNumber: "1"},
		Lines: []LineBlock{{
			DisplayName: "Tea", Quantity: "1.00", UnitPriceGross: "1.00",
			VATRate: "0.23", LineGross: "1.00",
		}},
		Totals:     TotalsBlock{NetTotal: "0.81", TaxPayable: "0.19", GrossTotal: "1.00"},
		Compliance: ComplianceBlock{ATCUD: "A-1", QR: QRBlock{Content: "A:1"}, HashControlChars: "Ab12"},
	}
	plain := decodeTicketText(RenderESCPOS(p))
	wantRule := strings.Repeat("-", receiptWidth)
	if !strings.Contains(plain, wantRule) {
		t.Fatalf("missing full-width rule %q in:\n%s", wantRule, plain)
	}
	if strings.Contains(plain, strings.Repeat("-", 32)+"\n") && !strings.Contains(plain, wantRule) {
		t.Fatal("still using 32-col rule")
	}
}

func TestFormatVATPercent(t *testing.T) {
	if formatVATPercent("0.23") != "23%" {
		t.Fatal(formatVATPercent("0.23"))
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
