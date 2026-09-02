package audit

import "testing"

func TestActionLabel(t *testing.T) {
	if ActionLabel("LOGIN") != "登录" {
		t.Fatal("LOGIN label")
	}
	if ActionLabel("unknown_xyz") != "unknown_xyz" {
		t.Fatal("unknown passthrough")
	}
}

func TestOwnerMayView(t *testing.T) {
	if !OwnerMayView("LOGIN") || OwnerMayView("fiscal_db_backup") {
		t.Fatal("owner whitelist")
	}
}

func TestFormatSummary(t *testing.T) {
	if got := FormatSummary("operator", "id1", "{}", "张三"); got != "开票员：张三" {
		t.Fatalf("operator: %q", got)
	}
	if got := FormatSummary("saft_exports", "", `{"year":2026,"month":3}`, ""); got != "期间：2026-03" {
		t.Fatalf("saft: %q", got)
	}
	if got := FormatSummary("sqlite", "", `{"path":"/tmp/fiscal-backup.db"}`, ""); got != "路径：fiscal-backup.db" {
		t.Fatalf("sqlite: %q", got)
	}
	if got := FormatSummary("", "", `{"pin":"secret"}`, ""); got != "" {
		t.Fatalf("sensitive filtered: %q", got)
	}
}

func TestFilterActionsForRole(t *testing.T) {
	owner := FilterActionsForRole("owner")
	if len(owner) != len(OwnerVisibleActions) {
		t.Fatalf("owner filters: got %d want %d", len(owner), len(OwnerVisibleActions))
	}
	admin := FilterActionsForRole("admin")
	if len(admin) < len(owner) {
		t.Fatal("admin should have more filters than owner")
	}
}
