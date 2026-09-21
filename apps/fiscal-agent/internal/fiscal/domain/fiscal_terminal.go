package domain

import "strings"

// LoopbackFiscalTerminalID is the ONLY sentinel id for Agent-host (loopback) issues.
// LAN PCs use fiscal_terminals.id from the pairing cookie.
const LoopbackFiscalTerminalID = "loopback"

// FiscalTerminalDisplayName is the ONLY display-name resolver for an issuing PC:
// non-empty remark/label wins; otherwise IP; last resort 127.0.0.1.
func FiscalTerminalDisplayName(label, ip string) string {
	label = strings.TrimSpace(label)
	if label != "" {
		return label
	}
	ip = strings.TrimSpace(ip)
	if ip != "" {
		return ip
	}
	return "127.0.0.1"
}
