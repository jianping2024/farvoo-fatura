// Package audit is the ONLY action label / summary / RBAC filter source for audit_log UI.
package audit

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// OwnerVisibleActions is the ONLY owner whitelist (design §3.3).
var OwnerVisibleActions = map[string]bool{
	"LOGIN":               true,
	"LOGIN_FAILED":        true,
	"LOGOUT":              true,
	"PIN_CHANGE":          true,
	"PIN_RESET":           true,
	"OPERATOR_ACTIVATE":   true,
	"OPERATOR_DEACTIVATE": true,
	"EXPORT_SAFT":         true,
}

// ownerActionOrder is the ONLY stable ordering for SQL IN and dropdowns.
var ownerActionOrder = []string{
	"LOGIN", "LOGIN_FAILED", "LOGOUT", "PIN_CHANGE", "PIN_RESET",
	"OPERATOR_ACTIVATE", "OPERATOR_DEACTIVATE", "EXPORT_SAFT",
}

// OwnerActionList returns owner-visible actions in stable order.
func OwnerActionList() []string {
	out := make([]string, len(ownerActionOrder))
	copy(out, ownerActionOrder)
	return out
}

var actionLabels = map[string]string{
	"LOGIN":                   "登录",
	"LOGIN_FAILED":            "登录失败",
	"LOGOUT":                  "退出",
	"PIN_CHANGE":              "修改 PIN",
	"PIN_RESET":               "重置 PIN",
	"OPERATOR_ACTIVATE":       "启用开票员",
	"OPERATOR_DEACTIVATE":     "停用开票员",
	"EXPORT_SAFT":             "导出 SAF-T",
	"fiscal_db_backup":        "备份税务库",
	"prepare_machine_swap":    "换机准备",
	"series_integrity_failed": "系列校验失败",
	"series_integrity_healed": "系列校验修复",
}

// ActionLabel maps DB action to product UI text (design §5.1).
func ActionLabel(action string) string {
	if l, ok := actionLabels[action]; ok {
		return l
	}
	return action
}

// OwnerMayView reports whether role owner may see action (admin always may).
func OwnerMayView(action string) bool {
	return OwnerVisibleActions[action]
}

// FilterActionsForRole returns dropdown options for audit filter UI.
func FilterActionsForRole(role string) []FilterAction {
	if role == "admin" {
		out := make([]FilterAction, 0, len(actionLabels))
		for action, label := range actionLabels {
			out = append(out, FilterAction{Value: action, Label: label})
		}
		return sortFilterActions(out)
	}
	if role == "owner" {
		out := make([]FilterAction, 0, len(OwnerVisibleActions))
		for action := range OwnerVisibleActions {
			out = append(out, FilterAction{Value: action, Label: ActionLabel(action)})
		}
		return sortFilterActions(out)
	}
	return nil
}

// FilterAction is one audit filter dropdown entry.
type FilterAction struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

func sortFilterActions(in []FilterAction) []FilterAction {
	// stable sort by label for UI
	for i := 0; i < len(in); i++ {
		for j := i + 1; j < len(in); j++ {
			if in[j].Label < in[i].Label {
				in[i], in[j] = in[j], in[i]
			}
		}
	}
	return in
}

var sensitiveDetailKeys = map[string]bool{
	"password": true, "pin": true, "key": true, "private_key": true,
}

// FormatSummary is the ONLY audit row summary formatter (design §5.2).
func FormatSummary(entityType, entityID, detailJSON, operatorDisplayName string) string {
	entityType = strings.TrimSpace(entityType)
	switch entityType {
	case "operator":
		name := strings.TrimSpace(operatorDisplayName)
		if name == "" {
			name = "未知开票员"
		}
		return "开票员：" + name
	case "saft_exports":
		if s := saftPeriodSummary(detailJSON); s != "" {
			return s
		}
		return "SAF-T 导出"
	case "series":
		if code := detailString(detailJSON, "series_code"); code != "" {
			return "系列：" + code
		}
		return "系列校验"
	case "sqlite":
		if p := detailString(detailJSON, "path"); p != "" {
			return "路径：" + filepath.Base(p)
		}
		return "税务库备份"
	case "installation":
		return "本机换机准备"
	default:
		return safeDetailPreview(detailJSON)
	}
}

func saftPeriodSummary(detailJSON string) string {
	var m map[string]any
	if json.Unmarshal([]byte(strings.TrimSpace(detailJSON)), &m) != nil {
		return ""
	}
	y, yok := m["year"]
	mo, mok := m["month"]
	if !yok || !mok {
		return ""
	}
	return formatPeriod(y, mo)
}

func formatPeriod(y, mo any) string {
	switch yv := y.(type) {
	case float64:
		switch mv := mo.(type) {
		case float64:
			return formatPeriodInt(int(yv), int(mv))
		}
	case int:
		if mv, ok := mo.(int); ok {
			return formatPeriodInt(yv, mv)
		}
	}
	return ""
}

func formatPeriodInt(y, m int) string {
	if y < 2000 || m < 1 || m > 12 {
		return ""
	}
	return fmt.Sprintf("期间：%d-%02d", y, m)
}

func intString(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func detailString(detailJSON, key string) string {
	var m map[string]any
	if json.Unmarshal([]byte(strings.TrimSpace(detailJSON)), &m) != nil {
		return ""
	}
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func safeDetailPreview(detailJSON string) string {
	detailJSON = strings.TrimSpace(detailJSON)
	if detailJSON == "" || detailJSON == "{}" {
		return ""
	}
	var m map[string]any
	if json.Unmarshal([]byte(detailJSON), &m) != nil {
		return ""
	}
	parts := make([]string, 0, len(m))
	for k, v := range m {
		if sensitiveDetailKeys[strings.ToLower(k)] {
			continue
		}
		parts = append(parts, k+"="+ stringifyPreview(v))
	}
	if len(parts) == 0 {
		return ""
	}
	if len(parts) > 3 {
		parts = parts[:3]
	}
	return strings.Join(parts, " ")
}

func stringifyPreview(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return intString(int(t))
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}
