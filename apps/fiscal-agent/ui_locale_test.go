package main

import "testing"

func TestNormalizeUILocale(t *testing.T) {
	cases := map[string]string{
		"": "zh", "zh": "zh", "EN": "en", "pt-BR": "pt", "bogus": "zh",
	}
	for in, want := range cases {
		if got := normalizeUILocale(in); got != want {
			t.Fatalf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestUILocaleDefaultZh(t *testing.T) {
	c := &config{}
	if c.uiLocale() != "zh" {
		t.Fatalf("default %q", c.uiLocale())
	}
	c.UILocale = "en"
	if c.uiLocale() != "en" {
		t.Fatalf("got %q", c.uiLocale())
	}
}
