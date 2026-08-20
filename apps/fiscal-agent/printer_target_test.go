package main

import "testing"

func TestParsePrinterTarget(t *testing.T) {
	cases := []struct {
		in      string
		scheme  string
		display string
	}{
		{"tcp:192.168.1.50:9100", schemeTCP, "tcp:192.168.1.50:9100"},
		{"192.168.1.50:9100", schemeTCP, "192.168.1.50:9100"},
		{"winspool:UK56009", schemeWinspool, "winspool:UK56009"},
		{"UK56009 Receipt", schemeWinspool, "winspool:UK56009 Receipt"},
	}
	for _, tc := range cases {
		tgt, err := parsePrinterTarget(tc.in)
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if tgt.Scheme != tc.scheme || tgt.Display != tc.display {
			t.Fatalf("%q => %+v want scheme=%s display=%s", tc.in, tgt, tc.scheme, tc.display)
		}
	}
}
