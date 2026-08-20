package main

import "testing"

func TestTextModeForConfiguredChineseDefaultsBitmap(t *testing.T) {
	if got := textModeForConfiguredChinese(true); got != escposTextBitmap {
		t.Fatalf("default chinese text mode = %v", got)
	}
	if got := textModeForConfiguredChinese(false); got != escposTextLatin {
		t.Fatalf("latin text mode = %v", got)
	}
}

func TestNormalizeTextEncodingNoGBKMode(t *testing.T) {
	if got := normalizeTextEncoding("gbk"); got != "auto" {
		t.Fatalf("gbk must fall back to auto bitmap mode, got %q", got)
	}
	if got := normalizeTextEncoding("utf-8"); got != "utf8" {
		t.Fatalf("utf8 alias = %q", got)
	}
}
