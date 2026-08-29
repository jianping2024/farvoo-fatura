// Package auth implements Fiscal Local API §13 P0 subset (terminal + operator_token).
//
// P0 定法:
//   - FISCAL_AUTH_MODE=loopback_trust (default): 127.0.0.1/::1 本机信任，可不带凭证
//   - 非 loopback 或 FISCAL_AUTH_MODE=required: 必须终端凭证 + operator_token
//   - operator_token: HS256 JWT（FISCAL_OPERATOR_JWT_SECRET）；生产可换 Farvoo 公钥验签，验签入口仍唯一 VerifyOperatorToken
package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/store"
)

type ctxKey int

const identityKey ctxKey = 1

// Mode from FISCAL_AUTH_MODE.
const (
	ModeLoopbackTrust = "loopback_trust"
	ModeRequired      = "required"
)

// Identity is the authenticated issuer for an issue request.
type Identity struct {
	OperatorID string
	TerminalID string
	MesaUserID string
	Trusted    bool // true when loopback trust skipped credentials
}

// OperatorClaims are JWT claims for operator_token (P0 subset of integration §13.4).
type OperatorClaims struct {
	RestaurantID string `json:"restaurant_id"`
	StoreID      string `json:"store_id"`
	MesaUserID   string `json:"mesa_user_id"`
	Role         string `json:"role"`
	TerminalID   string `json:"terminal_id"`
	DisplayName  string `json:"display_name"`
	Exp          int64  `json:"exp"`
	Iat          int64  `json:"iat"`
	JTI          string `json:"jti"`
}

// AuthMode returns configured mode (default loopback_trust).
func AuthMode() string {
	m := strings.TrimSpace(strings.ToLower(os.Getenv("FISCAL_AUTH_MODE")))
	if m == "" {
		return ModeLoopbackTrust
	}
	return m
}

// IsLoopback reports whether the request peer is loopback.
func IsLoopback(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// AuthenticateIssue is the ONLY §13 gate for issue routes.
// When trust applies, returns Trusted identity (handlers may use body operator_id).
// When credentials required, verifies terminal + token and resolves operators.id.
func AuthenticateIssue(r *http.Request, db *store.DB, storeID string) (*Identity, error) {
	mode := AuthMode()
	if mode == ModeLoopbackTrust && IsLoopback(r) {
		return &Identity{Trusted: true}, nil
	}
	if db == nil {
		return nil, fmt.Errorf("auth: store unavailable")
	}
	termID := strings.TrimSpace(r.Header.Get("X-Fiscal-Terminal-Id"))
	termSecret := strings.TrimSpace(r.Header.Get("X-Fiscal-Terminal-Secret"))
	if termID == "" || termSecret == "" {
		return nil, errAuth("terminal_required", "X-Fiscal-Terminal-Id and X-Fiscal-Terminal-Secret required")
	}
	if _, err := db.VerifyIssueTerminal(storeID, termID, termSecret); err != nil {
		return nil, errAuth("terminal_invalid", err.Error())
	}
	token := strings.TrimSpace(r.Header.Get("X-Fiscal-Operator-Token"))
	if token == "" {
		if ah := r.Header.Get("Authorization"); strings.HasPrefix(strings.ToLower(ah), "bearer ") {
			token = strings.TrimSpace(ah[7:])
		}
	}
	if token == "" {
		return nil, errAuth("operator_token_required", "X-Fiscal-Operator-Token or Authorization Bearer required")
	}
	secret := strings.TrimSpace(os.Getenv("FISCAL_OPERATOR_JWT_SECRET"))
	if secret == "" {
		return nil, errAuth("auth_misconfigured", "FISCAL_OPERATOR_JWT_SECRET not set")
	}
	claims, err := VerifyOperatorToken([]byte(secret), token)
	if err != nil {
		return nil, errAuth("operator_token_invalid", err.Error())
	}
	if claims.TerminalID != "" && claims.TerminalID != termID {
		return nil, errAuth("terminal_mismatch", "operator_token terminal_id does not match terminal credential")
	}
	claimStore := strings.TrimSpace(claims.StoreID)
	if claimStore == "" {
		claimStore = strings.TrimSpace(claims.RestaurantID)
	}
	if claimStore != "" && claimStore != storeID {
		return nil, errAuth("store_mismatch", "operator_token store/restaurant does not match agent store")
	}
	role := strings.ToLower(strings.TrimSpace(claims.Role))
	switch role {
	case "owner", "frontdesk", "cashier", "":
	default:
		return nil, errAuth("role_forbidden", "role cannot issue fiscal documents")
	}
	opID, err := db.EnsureOperatorFromMesa(storeID, claims.MesaUserID, role, claims.DisplayName)
	if err != nil {
		return nil, errAuth("operator_resolve_failed", err.Error())
	}
	return &Identity{
		OperatorID: opID,
		TerminalID: termID,
		MesaUserID: claims.MesaUserID,
		Trusted:    false,
	}, nil
}

// WithIdentity attaches identity to context.
func WithIdentity(ctx context.Context, id *Identity) context.Context {
	return context.WithValue(ctx, identityKey, id)
}

// IdentityFrom returns identity if present.
func IdentityFrom(ctx context.Context) *Identity {
	id, _ := ctx.Value(identityKey).(*Identity)
	return id
}

// SignOperatorToken builds HS256 JWT — ONLY mint helper for UAT/tests (Farvoo mints in prod).
func SignOperatorToken(secret []byte, c OperatorClaims) (string, error) {
	if len(secret) == 0 {
		return "", errors.New("auth: empty jwt secret")
	}
	if c.MesaUserID == "" {
		return "", errors.New("auth: mesa_user_id required")
	}
	if c.Iat == 0 {
		c.Iat = time.Now().Unix()
	}
	if c.Exp == 0 {
		c.Exp = c.Iat + 15*60
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	body := header + "." + base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(body))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return body + "." + sig, nil
}

// VerifyOperatorToken is the ONLY operator_token verify path.
func VerifyOperatorToken(secret []byte, token string) (*OperatorClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("auth: malformed jwt")
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	want := mac.Sum(nil)
	got, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(got, want) {
		return nil, errors.New("auth: bad signature")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("auth: bad payload")
	}
	var c OperatorClaims
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, errors.New("auth: bad claims json")
	}
	if c.Exp > 0 && time.Now().Unix() > c.Exp {
		return nil, errors.New("auth: token expired")
	}
	if strings.TrimSpace(c.MesaUserID) == "" {
		return nil, errors.New("auth: mesa_user_id missing")
	}
	return &c, nil
}

type authError struct {
	Code string
	Msg  string
}

func (e *authError) Error() string { return e.Code + ": " + e.Msg }

func errAuth(code, msg string) error { return &authError{Code: code, Msg: msg} }

// WriteAuthError maps auth failures to HTTP 401.
func WriteAuthError(w http.ResponseWriter, err error) {
	code, msg := "unauthorized", err.Error()
	var ae *authError
	if errors.As(err, &ae) {
		code, msg = ae.Code, ae.Msg
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": code, "message": msg})
}
