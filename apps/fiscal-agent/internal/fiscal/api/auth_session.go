package api

import (
	"context"
	"errors"
	"net/http"

	"farvoo-fiscal-agent/internal/fiscal/store"
)

var (
	errSessionRequired  = errors.New("session required")
	errOperatorInactive = errors.New("operator not active")
	errSessionRevoked   = errors.New("session revoked")
)

// refreshSessionFromDB is the ONLY DB active/epoch/role refresh for a parsed session cookie.
// Callers decide HTTP vs silent failure.
func (deps HandlerDeps) refreshSessionFromDB(sess *Session) (*Session, error) {
	if sess == nil || deps.Fiscal == nil || deps.Fiscal.DB() == nil {
		return nil, errSessionRequired
	}
	st, err := deps.Fiscal.DB().GetOperatorSessionState(deps.StoreID, sess.OperatorID)
	if errors.Is(err, store.ErrNotFound) || st == nil {
		return nil, errOperatorInactive
	}
	if err != nil {
		return nil, err
	}
	if !st.Active {
		return nil, errOperatorInactive
	}
	if sess.Epoch < st.SessionEpoch {
		return nil, errSessionRevoked
	}
	out := *sess
	out.Role = st.Role
	out.DisplayName = st.DisplayName
	out.Epoch = st.SessionEpoch
	return &out, nil
}

// validateSessionCookie checks DB active/epoch and refreshes role — ONLY HTTP session gate after cookie parse.
func (deps HandlerDeps) validateSessionCookie(w http.ResponseWriter, sess *Session) (*Session, bool) {
	valid, err := deps.refreshSessionFromDB(sess)
	if err != nil {
		switch {
		case errors.Is(err, errSessionRequired):
			writeErr(w, http.StatusUnauthorized, "unauthorized", "session required")
		case errors.Is(err, errOperatorInactive):
			writeErr(w, http.StatusUnauthorized, "operator_inactive", "operator not active")
		case errors.Is(err, errSessionRevoked):
			writeErr(w, http.StatusUnauthorized, "session_revoked", "session revoked")
		default:
			writeErr(w, http.StatusInternalServerError, "session_check_failed", err.Error())
		}
		return nil, false
	}
	return valid, true
}

// sessionIfValidCookie elevates anonymous /setup/status — ONLY full-status session path.
// Invalid/revoked/expired cookies yield nil (caller returns SetupStatusPublic); never writes 401 here.
func (deps HandlerDeps) sessionIfValidCookie(r *http.Request) *Session {
	if deps.Sessions == nil {
		return nil
	}
	parsed, err := deps.Sessions.ParseRequest(r)
	if err != nil || parsed == nil {
		return nil
	}
	valid, err := deps.refreshSessionFromDB(parsed)
	if err != nil || valid == nil {
		return nil
	}
	return valid
}

func (deps HandlerDeps) sessionContext(w http.ResponseWriter, r *http.Request, sess *Session) (context.Context, bool) {
	valid, ok := deps.validateSessionCookie(w, sess)
	if !ok {
		return r.Context(), false
	}
	return context.WithValue(r.Context(), ctxSessionKey, valid), true
}
