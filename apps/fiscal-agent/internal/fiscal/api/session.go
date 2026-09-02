package api

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ErrSessionSecretRequired is returned when production lacks FISCAL_SESSION_SECRET
// and autoFile is false (fiscal-local / ops without env).
var ErrSessionSecretRequired = errors.New("api: FISCAL_SESSION_SECRET required when FISCAL_ALLOW_DEV_KEY is not 1")

const (
	sessionCookieName  = "fiscal_session"
	terminalCookieName = "fiscal_terminal_id"
	sessionMaxAge      = 8 * time.Hour
	sessionIdle        = 30 * time.Minute
)

type ctxKey int

const ctxSessionKey ctxKey = 1

// Session holds authenticated operator state.
type Session struct {
	OperatorID  string
	Role        string
	DisplayName string
	Epoch       int
	IssuedAt    time.Time
	LastSeen    time.Time
}

// SessionManager signs HttpOnly session cookies — ONLY session write path.
type SessionManager struct {
	secret []byte
}

const sessionSecretFileName = "session_hmac.key"

func sessionSecretPath(dataDir string) string {
	dir := strings.TrimSpace(dataDir)
	if dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, sessionSecretFileName)
}

func loadPersistedSessionSecret(dataDir string) ([]byte, error) {
	raw, err := os.ReadFile(sessionSecretPath(dataDir))
	if err != nil {
		return nil, err
	}
	s := strings.TrimSpace(string(raw))
	if secret, err := base64.RawStdEncoding.DecodeString(s); err == nil && len(secret) >= 32 {
		return secret, nil
	}
	if len(s) >= 32 {
		return []byte(s), nil
	}
	return nil, fmt.Errorf("session secret file invalid")
}

func createPersistedSessionSecret(dataDir string) ([]byte, error) {
	dir := strings.TrimSpace(dataDir)
	if dir == "" {
		dir = os.TempDir()
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	line := base64.RawStdEncoding.EncodeToString(secret) + "\n"
	if err := os.WriteFile(sessionSecretPath(dir), []byte(line), 0o600); err != nil {
		return nil, err
	}
	return secret, nil
}

// NewSessionManager derives or loads HMAC secret — ONLY session secret read path.
// autoFile: Agent embed may persist {DataDir}/session_hmac.key when env unset (retail installer).
func NewSessionManager(dataDir string, autoFile bool) (*SessionManager, error) {
	if v := strings.TrimSpace(os.Getenv("FISCAL_SESSION_SECRET")); v != "" {
		if len(v) < 32 {
			return nil, fmt.Errorf("FISCAL_SESSION_SECRET must be at least 32 bytes")
		}
		return &SessionManager{secret: []byte(v)}, nil
	}
	if autoFile {
		if secret, err := loadPersistedSessionSecret(dataDir); err == nil {
			return &SessionManager{secret: secret}, nil
		}
		if secret, err := createPersistedSessionSecret(dataDir); err == nil {
			return &SessionManager{secret: secret}, nil
		} else {
			return nil, err
		}
	}
	if !IsFiscalDevMode() {
		return nil, ErrSessionSecretRequired
	}
	path := strings.TrimSpace(dataDir)
	if path == "" {
		path = os.TempDir()
	}
	sum := sha256.Sum256([]byte("fiscal-session:" + path))
	return &SessionManager{secret: sum[:]}, nil
}

type sessionPayload struct {
	OperatorID  string `json:"op"`
	Role        string `json:"role"`
	DisplayName string `json:"name"`
	Epoch       int    `json:"epoch"`
	IssuedAt    int64  `json:"iat"`
	LastSeen    int64  `json:"ls"`
}

func (m *SessionManager) sign(payload []byte) string {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// SetSessionCookie writes fiscal_session cookie.
func (m *SessionManager) SetSessionCookie(w http.ResponseWriter, sess Session) error {
	now := time.Now().UTC()
	if sess.IssuedAt.IsZero() {
		sess.IssuedAt = now
	}
	sess.LastSeen = now
	p := sessionPayload{
		OperatorID:  sess.OperatorID,
		Role:        sess.Role,
		DisplayName: sess.DisplayName,
		Epoch:       sess.Epoch,
		IssuedAt:    sess.IssuedAt.Unix(),
		LastSeen:    sess.LastSeen.Unix(),
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	b64 := base64.RawURLEncoding.EncodeToString(raw)
	sig := m.sign(raw)
	val := b64 + "." + sig
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    val,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionMaxAge.Seconds()),
	})
	return nil
}

// ClearSessionCookie removes session.
func (m *SessionManager) ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true,
	})
}

// ParseRequest loads session from cookie.
func (m *SessionManager) ParseRequest(r *http.Request) (*Session, error) {
	c, err := r.Cookie(sessionCookieName)
	if err != nil || c.Value == "" {
		return nil, errors.New("no session")
	}
	parts := strings.Split(c.Value, ".")
	if len(parts) != 2 {
		return nil, errors.New("bad session")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	if m.sign(raw) != parts[1] {
		return nil, errors.New("bad signature")
	}
	var p sessionPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	issued := time.Unix(p.IssuedAt, 0).UTC()
	last := time.Unix(p.LastSeen, 0).UTC()
	if now.Sub(issued) > sessionMaxAge {
		return nil, errors.New("expired")
	}
	if now.Sub(last) > sessionIdle {
		return nil, errors.New("idle expired")
	}
	return &Session{
		OperatorID:  p.OperatorID,
		Role:        p.Role,
		DisplayName: p.DisplayName,
		Epoch:       p.Epoch,
		IssuedAt:    issued,
		LastSeen:    last,
	}, nil
}

// SessionFromContext returns operator session injected by middleware.
func SessionFromContext(ctx context.Context) *Session {
	s, _ := ctx.Value(ctxSessionKey).(*Session)
	return s
}

// SetTerminalCookie sets fiscal_terminal_id after Ops pairing.
func SetTerminalCookie(w http.ResponseWriter, terminalID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     terminalCookieName,
		Value:    terminalID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((365 * 24 * time.Hour).Seconds()),
	})
}

// TerminalIDFromRequest reads terminal cookie (may be empty on loopback).
func TerminalIDFromRequest(r *http.Request) string {
	c, err := r.Cookie(terminalCookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(c.Value)
}

// IsLoopbackClient reports 127.0.0.1 / ::1 remote addr.
func IsLoopbackClient(r *http.Request) bool {
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i >= 0 {
		host = host[:i]
	}
	host = strings.Trim(host, "[]")
	return host == "127.0.0.1" || host == "::1"
}

// NewTerminalID generates local terminal UUID for loopback implicit slot.
func NewTerminalID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
