package fiscalclient

import "testing"

func TestNormalizeAgentBase(t *testing.T) {
	got, err := NormalizeAgentBase("192.168.1.10", "17880")
	if err != nil || got != "http://192.168.1.10:17880" {
		t.Fatalf("got %q err=%v", got, err)
	}
	got, err = NormalizeAgentBase("pos-agent", "")
	if err != nil || got != "http://pos-agent:17880" {
		t.Fatalf("default port: got %q err=%v", got, err)
	}
	if _, err := NormalizeAgentBase("", "17880"); err == nil {
		t.Fatal("expected error for empty host")
	}
	if _, err := NormalizeAgentBase("http://bad", "17880"); err == nil {
		t.Fatal("expected error for scheme in host")
	}
}

func TestNormalizeAgentBaseURL(t *testing.T) {
	got, err := NormalizeAgentBaseURL("http://192.168.1.10:17880/")
	if err != nil || got != "http://192.168.1.10:17880" {
		t.Fatalf("got %q err=%v", got, err)
	}
	if _, err := NormalizeAgentBaseURL("https://192.168.1.10:17880"); err == nil {
		t.Fatal("expected https rejected")
	}
	if _, err := NormalizeAgentBaseURL("http://192.168.1.10:17880/extra"); err == nil {
		t.Fatal("expected path rejected")
	}
}

func TestSplitAgentBase(t *testing.T) {
	host, port, err := SplitAgentBase("http://10.0.0.5:17880")
	if err != nil || host != "10.0.0.5" || port != "17880" {
		t.Fatalf("host=%q port=%q err=%v", host, port, err)
	}
}
