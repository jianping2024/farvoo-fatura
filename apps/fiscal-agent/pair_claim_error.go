package main

import "strings"

// pairClaimUserError maps claim API failures to operator-facing text.
func pairClaimUserError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "invalid_or_expired"),
		strings.Contains(msg, "404"),
		strings.Contains(msg, "401"),
		strings.Contains(msg, `"invalid"`):
		return "配对码无效或已过期，请在 " + productName + " 后台重新生成"
	case strings.Contains(msg, "code_already_used"):
		return "配对码已被使用，请在 " + productName + " 后台重新生成"
	case strings.Contains(msg, "rate_limited"):
		return "请求过于频繁，请稍后再试"
	default:
		return msg
	}
}
