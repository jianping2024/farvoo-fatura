package billsync

import (
	"encoding/json"
	"strings"

	"farvoo-fiscal-agent/internal/fiscal/compliance"
)

// ListGrossTotalFromPayload returns a display gross_total for bill-draft list rows.
// ONLY list-amount helper for sync payload JSON.
func ListGrossTotalFromPayload(payloadJSON string) string {
	var snap Snapshot
	if err := json.Unmarshal([]byte(payloadJSON), &snap); err != nil {
		return ""
	}
	return snap.ListGrossTotal()
}

// ListGrossTotal is whole_table gross_total or sum of split parts.
func (s Snapshot) ListGrossTotal() string {
	if strings.TrimSpace(s.ScopeType) == "split" {
		sum, ok := sumSplitGross(s.Splits)
		if ok {
			return sum
		}
		return ""
	}
	return strings.TrimSpace(s.GrossTotal)
}

func sumSplitGross(parts []SplitPart) (string, bool) {
	total, err := compliance.ParseDecimal("0")
	if err != nil {
		return "", false
	}
	got := false
	for _, sp := range parts {
		g := strings.TrimSpace(sp.GrossTotal)
		if g == "" {
			continue
		}
		d, err := compliance.ParseDecimal(g)
		if err != nil {
			continue
		}
		total = total.Add(d)
		got = true
	}
	if !got {
		return "", false
	}
	return compliance.Money2(total), true
}
