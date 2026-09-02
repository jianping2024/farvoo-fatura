package bootstrap

import (
	"regexp"
	"strings"
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/domain"
)

func TestAdminPaymentMethodKeysMatchDomain(t *testing.T) {
	codes := domain.KnownPaymentMethods()
	i18n := string(fiscalUIAdminI18nJS)
	for _, code := range codes {
		key := "'pay." + code + "'"
		if !strings.Contains(i18n, key) {
			t.Fatalf("admin-i18n.js missing %s", key)
		}
	}
	// PAYMENT_METHODS array in Admin must list the same codes in order.
	re := regexp.MustCompile(`const PAYMENT_METHODS = \[([^\]]+)\]`)
	m := re.FindStringSubmatch(adminHTML)
	if m == nil {
		t.Fatal("admin HTML missing PAYMENT_METHODS")
	}
	var got []string
	for _, part := range strings.Split(m[1], ",") {
		part = strings.TrimSpace(part)
		part = strings.Trim(part, `'"`)
		if part != "" {
			got = append(got, part)
		}
	}
	if len(got) != len(codes) {
		t.Fatalf("PAYMENT_METHODS len=%d want %d (%v)", len(got), len(codes), got)
	}
	for i := range codes {
		if got[i] != codes[i] {
			t.Fatalf("PAYMENT_METHODS[%d]=%q want %q", i, got[i], codes[i])
		}
	}
}
