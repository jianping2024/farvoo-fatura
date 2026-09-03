package bootstrap

import (
	"regexp"
	"strings"
	"testing"
)

func i18nBundleKeys(src, locale string) []string {
	re := regexp.MustCompile(`(?s)` + locale + `: \{([^}]+n)\s*\},`)
	// Fallback: extract keys between locale: { and next "    },\n    "
	start := strings.Index(src, locale+": {")
	if start < 0 {
		return nil
	}
	rest := src[start:]
	end := strings.Index(rest, "\n    }")
	if end < 0 {
		return nil
	}
	block := rest[:end]
	keyRe := regexp.MustCompile(`'([a-zA-Z0-9_.]+)':`)
	var keys []string
	seen := map[string]bool{}
	for _, m := range keyRe.FindAllStringSubmatch(block, -1) {
		k := m[1]
		if seen[k] {
			continue
		}
		seen[k] = true
		keys = append(keys, k)
	}
	_ = re
	return keys
}

func TestAdminI18nBundlesAlignedAndUnique(t *testing.T) {
	src := string(fiscalUIAdminI18nJS)
	zh := i18nBundleKeys(src, "zh")
	en := i18nBundleKeys(src, "en")
	pt := i18nBundleKeys(src, "pt")
	if len(zh) < 200 {
		t.Fatalf("zh bundle too small: %d keys", len(zh))
	}
	if len(zh) != len(en) || len(zh) != len(pt) {
		t.Fatalf("bundle sizes zh=%d en=%d pt=%d", len(zh), len(en), len(pt))
	}
	enSet := map[string]bool{}
	for _, k := range en {
		enSet[k] = true
	}
	ptSet := map[string]bool{}
	for _, k := range pt {
		ptSet[k] = true
	}
	for _, k := range zh {
		if !enSet[k] {
			t.Fatalf("en missing key %s", k)
		}
		if !ptSet[k] {
			t.Fatalf("pt missing key %s", k)
		}
	}
	if !strings.Contains(src, "function fmt(") {
		t.Fatal("FiscalAdminI18n.fmt must be the only interpolation helper")
	}
	if n := strings.Count(src, "function fmt("); n != 1 {
		t.Fatalf("fmt must be defined once, got %d", n)
	}
}

func TestAdminSettingsI18nUniqueWriters(t *testing.T) {
	html := adminHTML
	i18n := string(fiscalUIAdminI18nJS)
	if n := strings.Count(html, `data-i18n="settings.nav.terminals"`); n != 3 {
		t.Fatalf("settings.nav.terminals must appear 3 times (nav+mobile+summary), got %d", n)
	}
	if strings.Contains(html, `data-settings-nav="terminals"`) && strings.Count(html, `data-settings-nav="terminals"`) != 2 {
		t.Fatal("terminals nav anchors must be exactly 2")
	}
	for _, nav := range []string{"overview", "store", "series", "operators", "audit", "printers", "terminals", "saft", "advanced"} {
		if !strings.Contains(html, `data-i18n="settings.nav.`+nav+`"`) {
			t.Fatalf("missing data-i18n for settings.nav.%s", nav)
		}
		if !strings.Contains(i18n, "'settings.nav."+nav+"'") {
			t.Fatalf("dictionary missing settings.nav.%s", nav)
		}
	}
	if n := strings.Count(html, "function applyAdminLocale"); n != 1 {
		t.Fatalf("applyAdminLocale must be the only locale apply path, got %d", n)
	}
	if n := strings.Count(html, "FiscalAdminI18n.setLocale("); n != 1 {
		t.Fatalf("setLocale must be called only from applyAdminLocale, got %d", n)
	}
	if n := strings.Count(html, "function listPagerLabels"); n != 1 {
		t.Fatalf("listPagerLabels must be the only pager copy, got %d", n)
	}
	if strings.Contains(html, "LIST_PAGE_INFO_ROWS") {
		t.Fatal("LIST_PAGE_INFO_ROWS must not remain")
	}
	if n := strings.Count(html, "getLabels: listPagerLabels"); n != 6 {
		t.Fatalf("all 6 pagination bars must use listPagerLabels, got %d", n)
	}
	if n := strings.Count(html, "function escHtml"); n != 1 {
		t.Fatalf("escHtml must be the only HTML escape helper, got %d", n)
	}
	if strings.Contains(html, "function escapeHtml") {
		t.Fatal("escapeHtml duplicate must be folded into escHtml")
	}
	if n := strings.Count(html, "function cloudSyncFailText"); n != 1 {
		t.Fatalf("cloudSyncFailText once, got %d", n)
	}
	if strings.Contains(html, "label: '门店与税务'") || strings.Contains(html, "s.label") {
		t.Fatal("wizard steps must use settings.nav.* via s.nav, not a second label field")
	}
	if n := strings.Count(string(fiscalUIListPaginationJS), "relabel:"); n != 1 {
		t.Fatalf("pagination relabel must exist once, got %d", n)
	}
}
