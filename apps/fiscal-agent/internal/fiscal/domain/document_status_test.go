package domain_test

import (
	"strings"
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/domain"
)

func TestIssuedOriginalDocumentStatusSQLInMatchesHelper(t *testing.T) {
	in := domain.IssuedOriginalDocumentStatusSQLIn()
	for _, st := range domain.IssuedOriginalDocumentStatuses {
		lit := "'" + string(st) + "'"
		if !strings.Contains(in, lit) {
			t.Fatalf("SQL IN %q missing %s", in, lit)
		}
		if !domain.IsReprintableDocumentStatus(string(st)) {
			t.Fatalf("IsReprintableDocumentStatus(%q) want true", st)
		}
	}
	if domain.IsReprintableDocumentStatus("VOID") {
		t.Fatal("VOID must not be reprintable")
	}
}
