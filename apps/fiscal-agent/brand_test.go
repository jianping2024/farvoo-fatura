package main

import "testing"

func TestBrandConstants(t *testing.T) {
	if productName != "FARVOO" {
		t.Fatalf("productName = %q", productName)
	}
	if printAgentName != "FARVOO Fiscal Agent" {
		t.Fatalf("printAgentName = %q", printAgentName)
	}
	if printTrayTitleEN != "FARVOO Fiscal" {
		t.Fatalf("printTrayTitleEN = %q", printTrayTitleEN)
	}
}
