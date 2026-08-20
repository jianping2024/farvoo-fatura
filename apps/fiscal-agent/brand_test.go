package main

import "testing"

func TestBrandConstants(t *testing.T) {
	if productName != "FARVOO" {
		t.Fatalf("productName = %q", productName)
	}
	if printAgentName != "FARVOO Print Agent" {
		t.Fatalf("printAgentName = %q", printAgentName)
	}
	if printTrayTitleEN != "FARVOO Print" {
		t.Fatalf("printTrayTitleEN = %q", printTrayTitleEN)
	}
}
