package api

import (
	"context"
	"errors"
	"net/http"

	"farvoo-fiscal-agent/internal/fiscal/store"
)

// validateSessionCookie checks DB active/epoch and refreshes role — ONLY session gate after cookie parse.
func (deps HandlerDeps) validateSessionCookie(w http.ResponseWriter, sess *Session) (*Session, bool) {
	if sess == nil || deps.Fiscal == nil || deps.Fiscal.DB() == nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "session required")
		return nil, false
	}
	st, err := deps.Fiscal.DB().GetOperatorSessionState(deps.StoreID, sess.OperatorID)
	if errors.Is(err, store.ErrNotFound) || st == nil {
		writeErr(w, http.StatusUnauthorized, "operator_inactive", "operator not active")
		return nil, false
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "session_check_failed", err.Error())
		return nil, false
	}
	if !st.Active {
		writeErr(w, http.StatusUnauthorized, "operator_inactive", "operator not active")
		return nil, false
	}
	if sess.Epoch < st.SessionEpoch {
		writeErr(w, http.StatusUnauthorized, "session_revoked", "session revoked")
		return nil, false
	}
	sess.Role = st.Role
	sess.DisplayName = st.DisplayName
	sess.Epoch = st.SessionEpoch
	return sess, true
}

func (deps HandlerDeps) sessionContext(w http.ResponseWriter, r *http.Request, sess *Session) (context.Context, bool) {
	valid, ok := deps.validateSessionCookie(w, sess)
	if !ok {
		return r.Context(), false
	}
	return context.WithValue(r.Context(), ctxSessionKey, valid), true
}
