package main

import "testing"

func TestFillBrandTokens(t *testing.T) {
	got := fillBrandTokens("Pair {printAgent} to {brand} ({tray})")
	want := "Pair OULIN Fiscal Agent to OULIN (OULIN Fiscal)"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	zh := fillBrandTokens("卸载 {printAgentZh}")
	if zh != "卸载 OULIN 打印助手" {
		t.Fatalf("zh = %q", zh)
	}
}
