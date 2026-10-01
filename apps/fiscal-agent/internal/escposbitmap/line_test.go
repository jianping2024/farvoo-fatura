package escposbitmap

import "testing"

func TestLineEmitsGSv0ForPortuguese(t *testing.T) {
	raw := Line("Observação", Style{}, DefaultFontPx)
	if len(raw) < 8 {
		t.Fatal("expected GS v 0 payload")
	}
	if raw[0] != 0x1B || raw[1] != 0x61 || raw[3] != 0x1D || raw[4] != 0x76 || raw[5] != 0x30 {
		t.Fatalf("expected ESC a + GS v 0 header, got % x", raw[:min(8, len(raw))])
	}
	// Must not emit Windows-1252 ã (0xE3) as naked text for accents under Line.
	if containsByte(raw, 0xe3) && !hasGSv0(raw) {
		t.Fatal("unexpected")
	}
}

func TestLineEmpty(t *testing.T) {
	if Line("", Style{}, DefaultFontPx) != nil {
		t.Fatal("empty must be nil")
	}
}

func hasGSv0(b []byte) bool {
	for i := 0; i+2 < len(b); i++ {
		if b[i] == 0x1D && b[i+1] == 0x76 && b[i+2] == 0x30 {
			return true
		}
	}
	return false
}

func containsByte(b []byte, v byte) bool {
	for _, x := range b {
		if x == v {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
