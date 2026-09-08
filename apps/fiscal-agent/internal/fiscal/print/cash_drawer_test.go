package print

import (
	"bytes"
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/domain"
)

func TestNormalizeCashDrawerPin(t *testing.T) {
	if NormalizeCashDrawerPin(5) != 5 {
		t.Fatal("pin5")
	}
	for _, in := range []int{0, 1, 2, 3, 4, 6, -1} {
		if NormalizeCashDrawerPin(in) != 2 {
			t.Fatalf("pin %d want 2", in)
		}
	}
}

func TestCashDrawerKickBytesPin2And5(t *testing.T) {
	p2 := CashDrawerKickBytes(2)
	want2 := []byte{0x1B, 0x70, 0x00, 0x19, 0xFA}
	if !bytes.Equal(p2, want2) {
		t.Fatalf("pin2 got %x want %x", p2, want2)
	}
	p5 := CashDrawerKickBytes(5)
	want5 := []byte{0x1B, 0x70, 0x01, 0x19, 0xFA}
	if !bytes.Equal(p5, want5) {
		t.Fatalf("pin5 got %x want %x", p5, want5)
	}
}

func TestShouldAppendCashDrawerKick(t *testing.T) {
	base := &Payload{
		DocumentType: "FT",
		PrintPurpose: string(domain.PrintOriginal),
		Payments:     []PaymentBlock{{Method: domain.PaymentCash, Amount: "10.00"}},
	}
	if !ShouldAppendCashDrawerKick(base) {
		t.Fatal("cash FT ORIGINAL should kick")
	}
	fs := *base
	fs.DocumentType = "FS"
	if !ShouldAppendCashDrawerKick(&fs) {
		t.Fatal("cash FS should kick")
	}
	mixed := *base
	mixed.Payments = []PaymentBlock{{Method: domain.PaymentMixed, Amount: "10.00"}}
	if !ShouldAppendCashDrawerKick(&mixed) {
		t.Fatal("MIXED should kick")
	}
	card := *base
	card.Payments = []PaymentBlock{{Method: domain.PaymentCard, Amount: "10.00"}}
	if ShouldAppendCashDrawerKick(&card) {
		t.Fatal("CARD must not kick")
	}
	reprint := *base
	reprint.PrintPurpose = string(domain.PrintReprint)
	if ShouldAppendCashDrawerKick(&reprint) {
		t.Fatal("REPRINT must not kick")
	}
	nc := *base
	nc.DocumentType = "NC"
	if ShouldAppendCashDrawerKick(&nc) {
		t.Fatal("NC must not kick")
	}
	emptyPay := *base
	emptyPay.Payments = nil
	if !ShouldAppendCashDrawerKick(&emptyPay) {
		t.Fatal("empty payments default cash tender")
	}
}

func TestAppendCashDrawerKick(t *testing.T) {
	raw := []byte{0x41, 0x42}
	out := AppendCashDrawerKick(raw, 2)
	if !bytes.HasPrefix(out, raw) {
		t.Fatal("must keep receipt prefix")
	}
	if !bytes.HasSuffix(out, CashDrawerKickBytes(2)) {
		t.Fatal("must end with kick")
	}
}
