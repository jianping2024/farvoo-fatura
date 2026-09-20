package bootstrap

import (
	"strings"
	"testing"
)

func TestCashTenderUniqueWriters(t *testing.T) {
	html := string(adminHTML)
	checks := []struct {
		name string
		n    int
		sub  string
	}{
		{"bindCashTenderUI def", 1, "function bindCashTenderUI("},
		{"syncCashTenderWrap def", 1, "function syncCashTenderWrap("},
		{"readCashTenderedOrToast def", 1, "function readCashTenderedOrToast("},
		{"inv wrap", 1, `id="invCashTenderWrap"`},
		{"split wrap", 1, `id="splitCashTenderWrap"`},
		{"inv tendered", 1, `id="invTendered"`},
		{"split tendered", 1, `id="splitTendered"`},
		{"orderCashTenderDue uses currentOrder", 1, "orderMoneyBreakdown(currentOrder)"},
	}
	for _, c := range checks {
		got := strings.Count(html, c.sub)
		if got != c.n {
			t.Fatalf("%s: count=%d want %d", c.name, got, c.n)
		}
	}
	i18n := string(fiscalUIAdminI18nJS)
	for _, key := range []string{
		"'pay.tendered'", "'pay.change'",
		"'pay.toast.need_tendered'", "'pay.toast.tendered_low'",
	} {
		if strings.Count(i18n, key) != 3 { // zh + en + pt
			t.Fatalf("%s must appear once per locale (3), got %d", key, strings.Count(i18n, key))
		}
	}
}
