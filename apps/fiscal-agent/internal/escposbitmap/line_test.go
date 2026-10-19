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

func TestScaleMirrorsFirmwareSizes(t *testing.T) {
	cases := []struct {
		dw, dh       bool
		px, stretchY int
	}{
		{false, false, 24, 1}, // 1×1 = 24×24 Han cell
		{false, true, 24, 2},  // 1×2 = dot rows doubled (GS ! 0x01)
		{true, true, 48, 1},   // 2×2 = rendered at 2× base
	}
	for _, c := range cases {
		px, sy := Scale(DefaultFontPx, c.dw, c.dh)
		if px != c.px || sy != c.stretchY {
			t.Fatalf("Scale(%v,%v) = %d,%d want %d,%d", c.dw, c.dh, px, sy, c.px, c.stretchY)
		}
	}
}

func TestStretchYAndTrim(t *testing.T) {
	img := Image{Width: 2, Height: 5, Pixels: []byte{
		0, 0,
		0, 0,
		1, 0,
		0, 1,
		0, 0,
	}}
	st := StretchY(img, 2)
	if st.Height != 10 || st.Pixels[4*2] != 1 || st.Pixels[5*2] != 1 {
		t.Fatalf("stretch: %+v", st)
	}
	tr := Trim(img)
	if tr.Height != 2+trimPadTop+trimPadBottom {
		t.Fatalf("trim height %d", tr.Height)
	}
	if tr.Pixels[trimPadTop*2] != 1 || tr.Pixels[(trimPadTop+1)*2+1] != 1 {
		t.Fatalf("trim lost ink: %+v", tr)
	}
}

func TestLineDoubleHIsTallerThanNormal(t *testing.T) {
	normal := Line("宫保鸡丁", Style{}, DefaultFontPx)
	tall := Line("宫保鸡丁", Style{DoubleH: true}, DefaultFontPx)
	h := func(b []byte) int { return int(b[9]) | int(b[10])<<8 } // ESC a n + GS v 0 m xL xH yL yH
	if len(normal) == 0 || len(tall) == 0 || h(tall) <= h(normal) {
		t.Fatalf("1×2 height %d must exceed 1×1 height %d", h(tall), h(normal))
	}
}
