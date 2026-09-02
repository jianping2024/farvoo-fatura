package main

import (
	"os"
	"strings"
	"testing"
)

func TestPrintTicketLabelsHonorsPrintLocale(t *testing.T) {
	cases := map[string]string{
		"zh": "预结单",
		"en": "Pre-Bill",
		"pt": "Pré-conta",
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
}

func TestProductionTicketChromeUsesPrintTicketLabelsOnly(t *testing.T) {
	for _, file := range []string{"escpos.go", "escpos_encoding.go"} {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		src := string(b)
		if strings.Contains(src, "labelsFor(p.Locale)") {
			t.Fatalf("%s must not call labelsFor(p.Locale); use printTicketLabels", file)
		}
	}
}
