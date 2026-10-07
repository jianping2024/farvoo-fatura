package escposenc

import (
	"bytes"
	"strings"
	"testing"
)

func TestCodePageProbeRowsEncodesEachTable(t *testing.T) {
	raw := CodePageProbeRows()
	// ESC t n + label + "ã õ ç" in that table's bytes.
	cases := []struct {
		n     byte
		label string
		aoc   []byte
	}{
		{2, "CP850", []byte{0xC6, ' ', 0xE4, ' ', 0x87}},
		{3, "CP860", []byte{0x84, ' ', 0x94, ' ', 0x87}},
		{16, "WPC1252", []byte{0xE3, ' ', 0xF5, ' ', 0xE7}},
		{19, "CP858", []byte{0xC6, ' ', 0xE4, ' ', 0x87}},
	}
	for _, c := range cases {
		row := append(SelectCodeTable(c.n), []byte("n="+itoa(c.n))...)
		i := bytes.Index(raw, row)
		if i < 0 {
			t.Fatalf("missing row for ESC t %d", c.n)
		}
		line := raw[i:]
		if j := bytes.IndexByte(line, '\n'); j >= 0 {
			line = line[:j]
		}
		if !bytes.Contains(line, []byte(c.label)) || !bytes.Contains(line, c.aoc) {
			t.Fatalf("ESC t %d row wrong: % x", c.n, line)
		}
		if bytes.Contains(line, []byte("ã")) {
			t.Fatalf("ESC t %d row leaked UTF-8", c.n)
		}
	}
	if !bytes.HasSuffix(raw, SelectCodeTable(CodeTableWPC1252)) {
		t.Fatal("probe must restore WPC1252")
	}
	for _, ln := range strings.Split(string(raw), "\n") {
		if len(ln) > 48+3 { // 48 cols + ESC t n prefix
			t.Fatalf("row exceeds 48 cols: %q", ln)
		}
	}
}

func itoa(n byte) string {
	if n < 10 {
		return string('0' + rune(n))
	}
	return string('0'+rune(n/10)) + string('0'+rune(n%10))
}

func TestHanProbeRowsOrderAndRestore(t *testing.T) {
	raw := HanProbeRows()
	gbkGong := []byte{0xB9, 0xAC} // 宫 in GBK
	a := bytes.Index(raw, []byte("A GBK"))
	b := bytes.Index(raw, []byte("B FS&+GBK"))
	c := bytes.Index(raw, []byte("C UTF-8"))
	if a < 0 || b <= a || c <= b {
		t.Fatalf("rows out of order: A=%d B=%d C=%d", a, b, c)
	}
	if !bytes.Contains(raw[a:b], gbkGong) || bytes.Contains(raw[a:b], []byte{0x1C, 0x26}) {
		t.Fatal("row A must be raw GBK without FS &")
	}
	rowB := raw[b:c]
	if !bytes.Contains(rowB, append([]byte{0x1C, 0x26}, gbkGong...)) || !bytes.Contains(rowB, []byte{0x1C, 0x2E}) {
		t.Fatalf("row B must be FS & + GBK + FS .: % x", rowB)
	}
	rowC := raw[c:]
	if !bytes.Contains(rowC, append([]byte{0x1B, 0x39, 0x01}, []byte("宫")...)) {
		t.Fatal("row C must be ESC 9 1 + UTF-8")
	}
	if !bytes.HasSuffix(raw, append([]byte{0x1B, 0x40}, SelectCodeTable(CodeTableWPC1252)...)) {
		t.Fatal("Han probe must end with ESC @ + WPC1252")
	}
}
