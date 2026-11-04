package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestAppIconAssetsSoleSplit(t *testing.T) {
	agentICO := "assets/agent_app_icon.ico"
	fiscalICO := "assets/fiscal_app_icon.ico"
	for _, p := range []string{
		agentICO,
		fiscalICO,
		"assets/icon-previews/agent_app_icon_48.png",
		"assets/icon-previews/fiscal_app_icon_48.png",
	} {
		st, err := os.Stat(p)
		if err != nil || st.Size() < 64 {
			t.Fatalf("missing or tiny icon asset %s: %v", p, err)
		}
	}
	ab, err := os.ReadFile(agentICO)
	if err != nil {
		t.Fatal(err)
	}
	fb, err := os.ReadFile(fiscalICO)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(ab, fb) {
		t.Fatal("agent_app_icon.ico and fiscal_app_icon.ico must differ (A1 split)")
	}
	embed, err := os.ReadFile("scripts/embed-app-icon.sh")
	if err != nil {
		t.Fatal(err)
	}
	s := string(embed)
	wantAgent := "assets/agent_app_icon.ico,assets/fiscal_app_icon.ico"
	if !strings.Contains(s, wantAgent) {
		t.Fatalf("embed-app-icon.sh must embed Agent as %q (order = shell then WebView)", wantAgent)
	}
	if !strings.Contains(s, `-ico assets/fiscal_app_icon.ico -o cmd/fiscal-client/rsrc_windows_amd64.syso`) {
		t.Fatal("Client rsrc must embed fiscal_app_icon.ico only")
	}
	if strings.Contains(s, "-ico assets/app_icon.ico") {
		t.Fatal("embed-app-icon.sh must not still use legacy sole app_icon.ico for rsrc")
	}
}
