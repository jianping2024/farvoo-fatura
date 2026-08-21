package escposenc

import "testing"

func TestWindows1252Portuguese(t *testing.T) {
	raw := Windows1252("Guaraná")
	if len(raw) < 7 {
		t.Fatalf("len=%d %v", len(raw), raw)
	}
	// á in Windows-1252 is 0xE1
	if raw[len(raw)-1] != 0xE1 {
		t.Fatalf("expected trailing 0xE1, got %v", raw)
	}
}

func TestWindows1252OmitsHan(t *testing.T) {
	raw := Windows1252("客人ab")
	if string(raw) != "ab" {
		t.Fatalf("got %q", raw)
	}
}
