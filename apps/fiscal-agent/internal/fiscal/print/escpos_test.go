package print

import (
	"strings"
	"testing"
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
			DisplayName: "Buffet livre", Quantity: "1.00", UnitPriceGross: "17.95",
			VATRate: "0.23", LineGross: "17.95",
		}},
		TaxSummary: []TaxSummaryRow{{
			VATRate: "0.23", TaxBase: "14.59", TaxAmount: "3.36", Gross: "17.95",
		}},
		Totals: TotalsBlock{NetTotal: "14.59", TaxPayable: "3.36", GrossTotal: "17.95"},
		Payments: []PaymentBlock{{Method: "CASH", Amount: "17.95"}},
		Compliance: ComplianceBlock{
			ATCUD:             "CSDF7T5H-3",
			QR:                QRBlock{Content: "A:517535009*B:999999990*C:PT"},
			HashControlChars:  "0L2I",
			CertificationLine: "Processado por programa certificado n.º 0/AT",
		},
	}
	out := string(RenderESCPOS(p))
	plain := stripEscPos(out)

	mustContain := []string{
		"Farvoo Demo Lda",
		"NIF: PT517535009",
		"FT FT2026DEMO01/3",
		"21/08/2026 18:26",
		"1ª Via — Original",
		"Cliente: Consumidor Final",
		"IVA 23%",
		"Liquido",
		"TOTAL",
		"Numerario",
		"Resumo IVA",
		"Taxa",
		"23%",
		"ATCUD: CSDF7T5H-3",
		"Processado por programa certificado n.º 0/AT",
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
	// invoice line once — not "FT: …"
	if strings.Contains(plain, "FT: ") {
		t.Fatalf("must not use FT: prefix:\n%s", plain)
	}
	// compliance after ATCUD; cert after QR marker region — cert must appear after ATCUD
	atcud := strings.Index(plain, "ATCUD:")
	cert := strings.Index(plain, "Processado por programa certificado")
	hash := strings.Index(plain, "Hash:")
	resumo := strings.Index(plain, "Resumo IVA")
	total := strings.Index(plain, "TOTAL")
	if !(total >= 0 && resumo > total && atcud > resumo && cert > atcud && hash > cert) {
		t.Fatalf("block order wrong total=%d resumo=%d atcud=%d cert=%d hash=%d\n%s",
			total, resumo, atcud, cert, hash, plain)
	}
	// decimal rate must not appear raw on face
	if strings.Contains(plain, "IVA 0.23") || strings.Contains(plain, "Taxa    0.23") {
		t.Fatalf("raw decimal VAT on ticket:\n%s", plain)
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

func stripEscPos(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == 0x1B || c == 0x1D {
			// skip until next printable-ish — rough: drop control sequences by skipping non-text
			for i < len(s) && s[i] < 0x20 && s[i] != '\n' {
				i++
			}
			if i < len(s) && s[i] >= 0x20 {
				i--
			}
			continue
		}
		if c == '\n' || c >= 0x20 {
			b.WriteByte(c)
		}
	}
	return b.String()
}
