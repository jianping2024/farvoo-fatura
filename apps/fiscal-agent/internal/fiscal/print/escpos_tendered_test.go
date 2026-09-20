package print

import (
	"strings"
	"testing"
)

func TestWritePaymentBlock_TenderedChange(t *testing.T) {
	L := receiptLabels("pt")
	var lines []string
	w := func(s string) { lines = append(lines, s) }
	writePaymentBlock(w, L, PaymentBlock{
		Method: "CASH", Amount: "8.00", Tendered: "10.00", ChangeDue: "2.00",
	}, 48)
	plain := strings.Join(lines, "\n")
	for _, want := range []string{"Numerario", "8.00", "Valor entregue", "10.00", "Troco", "2.00"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("missing %q in:\n%s", want, plain)
		}
	}
	lines = nil
	writePaymentBlock(w, L, PaymentBlock{Method: "CARD", Amount: "8.00"}, 48)
	plain = strings.Join(lines, "\n")
	if strings.Contains(plain, "Valor entregue") || strings.Contains(plain, "Troco") {
		t.Fatalf("card must not print tendered:\n%s", plain)
	}
}

func TestRenderESCPOS_IncludesTenderedLines(t *testing.T) {
	p := &Payload{
		InvoiceNo: "FS FS2026DEMO01/1",
		IssuedAt:  "2026-09-20T12:00:00Z",
		Merchant:  MerchantBlock{LegalName: "Demo Lda", TaxRegistrationNumber: "123456789"},
		Customer:  CustomerBlock{CompanyName: "Consumidor Final"},
		Lines: []LineBlock{{
			DisplayName: "Item", Quantity: "1", UnitPriceGross: "8.00",
			VATRate: "0.23", LineGross: "8.00",
		}},
		Totals: TotalsBlock{NetTotal: "6.50", TaxPayable: "1.50", GrossTotal: "8.00"},
		Payments: []PaymentBlock{{
			Method: "CASH", Amount: "8.00", Tendered: "10.00", ChangeDue: "2.00",
		}},
		Compliance: ComplianceBlock{
			ATCUD:             "ABC-1",
			HashControlChars:  "ABCD",
			CertificationLine: "Processado por programa certificado n. 0/AT",
		},
	}
	plain := decodeTicketText(RenderESCPOS(p))
	for _, want := range []string{"Valor entregue", "10.00", "Troco", "2.00"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("missing %q in:\n%s", want, plain)
		}
	}
}
