package domain

import "testing"

func TestParseSaleDocumentType_AllowsFR(t *testing.T) {
	dt, err := ParseSaleDocumentType("FR")
	if err != nil || dt != DocumentFR {
		t.Fatalf("got %q %v", dt, err)
	}
	dt, err = ParseSaleDocumentType("")
	if err != nil || dt != DocumentFS {
		t.Fatalf("default %q %v", dt, err)
	}
	if _, err := ParseSaleDocumentType("NC"); err == nil {
		t.Fatal("expected NC reject")
	}
}

func TestParseBillSyncDocumentType_RejectsFR(t *testing.T) {
	if _, err := ParseBillSyncDocumentType("FR"); err == nil {
		t.Fatal("expected FR reject on bill sync")
	}
	dt, err := ParseBillSyncDocumentType("FT")
	if err != nil || dt != DocumentFT {
		t.Fatalf("got %q %v", dt, err)
	}
}

func TestIsAdjustableOriginal_IncludesFR(t *testing.T) {
	if !IsAdjustableOriginalDocumentType(DocumentFR) {
		t.Fatal("FR must be adjustable")
	}
	if IsSaleScopeDocumentType(DocumentFR) {
		t.Fatal("FR must not be bill-sync scope")
	}
}

func TestParseInvoiceListDocumentType_AllowsFR(t *testing.T) {
	dt, err := ParseInvoiceListDocumentType("FR")
	if err != nil || dt != DocumentFR {
		t.Fatalf("got %q %v", dt, err)
	}
}
