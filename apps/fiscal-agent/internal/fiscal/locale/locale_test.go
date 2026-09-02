package locale

import "testing"

func TestNormalizeUILocale(t *testing.T) {
	cases := map[string]string{
		"": "zh", "ZH": "zh", "zh-CN": "zh",
		"en": "en", "English": "en",
		"pt": "pt", "pt-PT": "pt", "Português": "pt",
	}
	for in, want := range cases {
		if got := NormalizeUILocale(in); got != want {
			t.Fatalf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestInvoiceLocaleFromUI_SchemeA(t *testing.T) {
	cases := map[string]string{
		"zh": "pt",
		"en": "en",
		"pt": "pt",
		"":   "pt", // NormalizeUILocale("")=zh → invoice pt
	}
	for in, want := range cases {
		if got := InvoiceLocaleFromUI(in); got != want {
			t.Fatalf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestNormalizeInvoiceLocale(t *testing.T) {
	if NormalizeInvoiceLocale("") != "pt" {
		t.Fatal("empty")
	}
	if NormalizeInvoiceLocale("zh") != "pt" {
		t.Fatal("zh must not stay on ticket")
	}
	if NormalizeInvoiceLocale("en") != "en" {
		t.Fatal("en")
	}
}
