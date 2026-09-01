package print

import (
	"bytes"
	"testing"
)

func TestReceiptVerticalPaddingConstants(t *testing.T) {
	if receiptTopGapDotsV040 != 15 {
		t.Fatalf("v0.4.40 top baseline %d want 15", receiptTopGapDotsV040)
	}
	if receiptBottomFeedTotalV040 != 85 {
		t.Fatalf("v0.4.40 bottom baseline %d want 85", receiptBottomFeedTotalV040)
	}
	if receiptTopGapDots != receiptTopGapDotsV040/2 {
		t.Fatalf("top gap %d want %d (half of %d)", receiptTopGapDots, receiptTopGapDotsV040/2, receiptTopGapDotsV040)
	}
	if receiptBottomFeedTotal != receiptBottomFeedTotalV040*2/3 {
		t.Fatalf("bottom total %d want %d (⅔ of %d)", receiptBottomFeedTotal, receiptBottomFeedTotalV040*2/3, receiptBottomFeedTotalV040)
	}
	if int(escposLineDots)+int(cutFeedDots) != receiptBottomFeedTotal {
		t.Fatalf("bottom parts %d+%d != total %d", escposLineDots, cutFeedDots, receiptBottomFeedTotal)
	}
	if cutFeedDots != 26 || receiptTopGapDots != 7 {
		t.Fatalf("got top=%d cut=%d want top=7 cut=26", receiptTopGapDots, cutFeedDots)
	}
}

func TestRenderESCPOS_SoftStreamBeginNoEscAt(t *testing.T) {
	raw := RenderESCPOS(&Payload{
		Merchant: MerchantBlock{LegalName: "Demo"},
		InvoiceNo: "FT FT2026DEMO01/1",
		PrintPurpose: "ORIGINAL",
	})
	if len(raw) >= 2 && raw[0] == 0x1B && raw[1] == 0x40 {
		t.Fatal("normal receipt must not start with ESC @; use receiptStreamBegin only")
	}
	if !bytes.HasPrefix(raw, receiptStreamBegin()) {
		t.Fatalf("stream must begin with receiptStreamBegin prefix, got %x", raw[:min(12, len(raw))])
	}
	if bytes.Contains(raw, []byte{0x1B, 0x40}) {
		t.Fatal("normal receipt must not contain ESC @ anywhere")
	}
}
