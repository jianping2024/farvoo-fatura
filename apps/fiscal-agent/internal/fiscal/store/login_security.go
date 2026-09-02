package store

import (
	"errors"
	"fmt"
	"strings"
)

const (
	loginFailureWindowSQL   = `datetime('now', '-15 minutes')`
	loginFailureOperatorMax = 5
	loginFailureIPMax       = 30
	loginFailureAction      = "LOGIN_FAILED"
	loginFailureEntityIP    = "client_ip"
	loginFailureEntityOp    = "operator"
)

// ErrIPRateLimited is returned when client IP exceeded failed login threshold.
var ErrIPRateLimited = errors.New("store: ip rate limited")

// ClientIPFromRemoteAddr extracts client IP from RemoteAddr (no port). ONLY client IP parse path.
func ClientIPFromRemoteAddr(remoteAddr string) string {
	host := remoteAddr
	if i := strings.LastIndex(host, ":"); i >= 0 {
		host = host[:i]
	}
	return strings.Trim(host, "[]")
}

// IsLoginIPRateLimited reports whether clientIP hit the IP login failure threshold — ONLY IP rate read path.
func (d *DB) IsLoginIPRateLimited(clientIP string) (bool, error) {
	clientIP = strings.TrimSpace(clientIP)
	if clientIP == "" {
		return false, nil
	}
	if err := d.cleanupLoginFailures(clientIP, loginFailureEntityIP); err != nil {
		return false, err
	}
	var n int
	err := d.SQL.QueryRow(`SELECT COUNT(1) FROM audit_log
		WHERE action=? AND entity_type=? AND entity_id=? AND at > `+loginFailureWindowSQL,
		loginFailureAction, loginFailureEntityIP, clientIP).Scan(&n)
	if err != nil {
		return false, err
	}
	return n >= loginFailureIPMax, nil
}

// RecordLoginFailures records operator and client IP failed login rows — ONLY LOGIN_FAILED write orchestration.
func (d *DB) RecordLoginFailures(storeID, operatorID, clientIP string) error {
	key := LoginFailureEntityKey(storeID, operatorID)
	if err := d.recordLoginFailureRow(operatorID, loginFailureEntityOp, key); err != nil {
		return err
	}
	clientIP = strings.TrimSpace(clientIP)
	if clientIP == "" {
		return nil
	}
	return d.recordLoginFailureRow("", loginFailureEntityIP, clientIP)
}

func (d *DB) recordLoginFailureRow(operatorID, entityType, entityID string) error {
	if err := d.cleanupLoginFailures(entityID, entityType); err != nil {
		return err
	}
	return d.InsertAuditLog(operatorID, loginFailureAction, entityType, entityID, "{}")
}

func (d *DB) cleanupLoginFailures(entityID, entityType string) error {
	_, err := d.SQL.Exec(`DELETE FROM audit_log
		WHERE action=? AND entity_type=? AND entity_id=? AND at <= `+loginFailureWindowSQL,
		loginFailureAction, entityType, entityID)
	return err
}

// ClearOperatorLoginFailures removes operator login failure rows after successful PIN — ONLY operator clear path.
func (d *DB) ClearOperatorLoginFailures(storeID, operatorID string) error {
	key := LoginFailureEntityKey(storeID, operatorID)
	_, err := d.SQL.Exec(`DELETE FROM audit_log WHERE action=? AND entity_type=? AND entity_id=?`,
		loginFailureAction, loginFailureEntityOp, key)
	return err
}

// LoginFailureEntityKey is the ONLY entity_id format for operator LOGIN_FAILED rows.
func LoginFailureEntityKey(storeID, operatorID string) string {
	return fmt.Sprintf("login_fail:%s:%s", storeID, operatorID)
}
