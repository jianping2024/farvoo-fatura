package main

import "testing"

func TestFillBrandTokens(t *testing.T) {
	got := fillBrandTokens("Pair {printAgent} to {brand} ({tray})")
	want := "Pair FARVOO Print Agent to FARVOO (FARVOO Print)"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	zh := fillBrandTokens("卸载 {printAgentZh}")
	if zh != "卸载 FARVOO 打印助手" {
		t.Fatalf("zh = %q", zh)
	}
}
