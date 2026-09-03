package main

import (
	"os"
	"strings"
	"testing"
)

func TestPrintTicketLabelsHonorsPrintLocale(t *testing.T) {
	cases := map[string]string{
		"zh": "预结账单",
		"en": "Table Consultation",
		"pt": "Consulta Mesa",
	}
	for loc, wantPreBill := range cases {
		got := printTicketLabels(loc).preBill
		if got != wantPreBill {
			t.Fatalf("%s preBill=%q want %q", loc, got, wantPreBill)
		}
	}
	if printTicketLabels("pt").tableNo == printTicketLabels("en").tableNo {
		t.Fatal("pt and en tableNo labels must differ")
	}
	if printTicketLabels("").preBill != printTicketLabels("pt").preBill {
		t.Fatal("empty payload.locale defaults to pt chrome")
	}
	if printTicketLabels("pt").notAnInvoice != "ESTE DOCUMENTO NÃO SERVE DE FATURA" {
		t.Fatal("pt pre-bill disclaimer must stay Portuguese legal wording")
	}
	if printTicketLabels("zh").invoiceFillName != "姓名" || printTicketLabels("en").invoiceFillNIF != "NIF" {
		t.Fatal("pre-bill fill labels must follow print_locale")
	}
	if printTicketLabels("").notAnInvoice != printTicketLabels("pt").notAnInvoice {
		t.Fatal("empty locale must use pt not-an-invoice copy")
	}
}

func TestProductionTicketChromeUsesPrintTicketLabelsOnly(t *testing.T) {
	for _, file := range []string{"escpos.go", "escpos_encoding.go"} {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		src := string(b)
		if n := strings.Count(src, "lab.notAnInvoice"); n > 1 {
			t.Fatalf("%s must use lab.notAnInvoice only in writePreBillLegalBlock, got %d", file, n)
		}
	}
}
