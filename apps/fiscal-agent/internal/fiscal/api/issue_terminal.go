package api

import (
	"errors"
	"net/http"
	"strings"

	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

// resolveIssueFiscalTerminal is the ONLY issuer→PC freeze for document / bill issue handlers.
// Loopback → domain.LoopbackFiscalTerminalID; LAN → cookie fiscal_terminal_id.
func resolveIssueFiscalTerminal(r *http.Request, deps HandlerDeps) (id, label string, err error) {
	ip := store.ClientIPFromRemoteAddr(r.RemoteAddr)
	if IsLoopbackClient(r) {
		id = domain.LoopbackFiscalTerminalID
		label = domain.FiscalTerminalDisplayName("", ip)
		return id, label, nil
	}
	tid := strings.TrimSpace(TerminalIDFromRequest(r))
	if tid == "" {
		return "", "", errTerminalRequired
	}
	if deps.Fiscal == nil || deps.Fiscal.DB() == nil {
		return "", "", errFiscalUnavailable
	}
	row, err := deps.Fiscal.DB().GetFiscalTerminalByID(deps.StoreID, tid)
	if err != nil {
		return "", "", err
	}
	label = domain.FiscalTerminalDisplayName(row.Label, firstNonEmpty(row.LastSeenIP, ip))
	return row.ID, label, nil
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return strings.TrimSpace(a)
	}
	return strings.TrimSpace(b)
}

type codedResolveErr struct{ code, msg string }

func (e codedResolveErr) Error() string { return e.msg }

var (
	errTerminalRequired  = codedResolveErr{code: "terminal_required", msg: "pair this PC before issuing"}
	errFiscalUnavailable = codedResolveErr{code: "fiscal_unavailable", msg: "fiscal service not configured"}
)

// writeIssueTerminalErr is the ONLY HTTP mapper for resolveIssueFiscalTerminal failures.
func writeIssueTerminalErr(w http.ResponseWriter, err error) {
	var ce codedResolveErr
	if errors.As(err, &ce) {
		status := http.StatusForbidden
		if ce.code == "fiscal_unavailable" {
			status = http.StatusServiceUnavailable
		}
		writeErr(w, status, ce.code, ce.msg)
		return
	}
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusForbidden, "terminal_revoked", "terminal unknown or revoked; ask manager for a new pairing code")
		return
	}
	writeErr(w, http.StatusInternalServerError, "terminal_resolve_failed", err.Error())
}
