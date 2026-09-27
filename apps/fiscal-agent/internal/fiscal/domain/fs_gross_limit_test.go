package domain

import "testing"

func TestApplyFSGrossLimit(t *testing.T) {
	line := func(gross string) SaleLine {
		return SaleLine{
			Quantity: "1", UnitPriceGross: gross, VATRate: "0.23",
		}
	}
	cases := []struct {
		name  string
		dt    DocumentType
		lines []SaleLine
		want  DocumentType
	}{
		{"fs at limit stays", DocumentFS, []SaleLine{line("100.00")}, DocumentFS},
		{"fs over limit promotes", DocumentFS, []SaleLine{line("60.00"), line("40.01")}, DocumentFT},
		{"ft under stays", DocumentFT, []SaleLine{line("10.00")}, DocumentFT},
		{"ft over stays", DocumentFT, []SaleLine{line("250.00")}, DocumentFT},
		{"fr over stays", DocumentFR, []SaleLine{line("250.00")}, DocumentFR},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ApplyFSGrossLimit(tc.dt, tc.lines)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
}

func TestApplyFSGrossLimit_BadLine(t *testing.T) {
	_, err := ApplyFSGrossLimit(DocumentFS, []SaleLine{{
		Quantity: "x", UnitPriceGross: "1.00", VATRate: "0.23",
	}})
	if err == nil {
		t.Fatal("expected error")
	}
}
