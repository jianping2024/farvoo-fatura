package main

import "testing"

func TestBrandConstants(t *testing.T) {
	if productName != "OULIN" {
		t.Fatalf("productName = %q", productName)
	}
	if printAgentName != "OULIN Fiscal Agent" {
		t.Fatalf("printAgentName = %q", printAgentName)
	}
	if printTrayTitleEN != "OULIN Fiscal" {
		t.Fatalf("printTrayTitleEN = %q", printTrayTitleEN)
	}
}
