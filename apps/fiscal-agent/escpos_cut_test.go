package main

import "testing"

func cutDotsInRaw(raw []byte) byte {
	for i := 0; i+3 < len(raw); i++ {
		if raw[i] == 0x1D && raw[i+1] == 0x56 && raw[i+2] == 0x42 {
			return raw[i+3]
		}
	}
	return 0
}

func marginLinesBeforeContent(raw []byte, marker []byte) int {
	idx := bytesIndex(raw, marker)
	if idx < 0 {
		return -1
	}
	n := 0
	for i := idx - 1; i >= 0 && raw[i] == '\n'; i-- {
		n++
	}
	return n
}

func bytesIndex(b, sub []byte) int {
	for i := 0; i+len(sub) <= len(b); i++ {
		if string(b[i:i+len(sub)]) == string(sub) {
			return i
		}
	}
	return -1
}

func TestCutSequenceUsesFeedThenCut(t *testing.T) {
	w := newEscpos()
	w.text("footer")
	w.lf()
	raw := w.finish(true)
	if !containsSubslice(raw, []byte{0x1D, 0x56, 0x42}) {
		t.Fatal("expected GS V 66 feed-then-cut")
	}
	dots := cutDotsInRaw(raw)
	if dots != escposCutFeedDotsDefault {
		t.Fatalf("expected default cut dots 0x%x, got 0x%x", escposCutFeedDotsDefault, dots)
	}
	if containsSubslice(raw, []byte{0x1D, 0x56, 0x00}) {
		t.Fatal("immediate GS V 0 cut should not be used")
	}
}

func TestTicketTopMarginMinimal(t *testing.T) {
	w := newEscpos()
	w.text("restaurant")
	raw := w.finish(true)
	before := marginLinesBeforeContent(raw, []byte("restaurant"))
	if before != escposTopMarginLines {
		t.Fatalf("want %d line feeds before content, got %d", escposTopMarginLines, before)
	}
}

func TestTicketBottomMarginBeforeCut(t *testing.T) {
	w := newEscpos()
	w.text("footer")
	w.lf()
	raw := w.finish(true)
	idx := bytesIndex(raw, []byte{0x1D, 0x56, 0x42})
	if idx < 0 {
		t.Fatal("missing cut command")
	}
	n := 0
	for i := idx - 1; i >= 0 && raw[i] == '\n'; i-- {
		n++
	}
	if n != escposBottomMarginLines {
		t.Fatalf("want %d line feeds before cut, got %d", escposBottomMarginLines, n)
	}
}

func TestCutDotsIncreaseWhenDoubleHeight(t *testing.T) {
	plain := newEscpos()
	plain.text("x")
	plainRaw := plain.finish(true)

	tall := newEscpos()
	tall.size(true, true)
	tall.text("TITLE")
	tall.lf()
	tallRaw := tall.finish(true)

	if cutDotsInRaw(plainRaw) >= cutDotsInRaw(tallRaw) {
		t.Fatalf("double-height ticket should use more cut feed: plain=0x%x tall=0x%x",
			cutDotsInRaw(plainRaw), cutDotsInRaw(tallRaw))
	}
}

func containsSubslice(b, sub []byte) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i+len(sub) <= len(b); i++ {
		ok := true
		for j := range sub {
			if b[i+j] != sub[j] {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}
