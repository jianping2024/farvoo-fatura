package store

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// FiscalTerminalRow is a registered LAN terminal slot.
type FiscalTerminalRow struct {
	ID             string
	StoreID        string
	Label          string
	Active         bool
	OpsTerminalRef string
	RegisteredAt   string
	LastSeenAt     string
	LastSeenIP     string
}

// SetMaxFiscalTerminals writes local max — ONLY max write path (admin).
func (d *DB) SetMaxFiscalTerminals(storeID string, max int) error {
	if max < 1 {
		return errors.New("store: max_fiscal_terminals must be >= 1")
	}
	if max > 99 {
		return errors.New("store: max_fiscal_terminals must be <= 99")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := d.SQL.Exec(`UPDATE taxpayer_settings SET max_fiscal_terminals=?, updated_at=? WHERE store_id=?`,
		max, now, storeID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// RegisterLocalFiscalTerminal inserts a LAN terminal — ONLY local register path.
func (d *DB) RegisterLocalFiscalTerminal(storeID, label string) (string, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	id := uuid.NewString()
	ref := "local:" + id
	_, err := d.SQL.Exec(`INSERT INTO fiscal_terminals (
		id, store_id, label, active, ops_terminal_ref, registered_at, last_seen_at, last_seen_ip
	) VALUES (?, ?, ?, 1, ?, ?, ?, NULL)`,
		id, storeID, terminalLabelOrNull(label), ref, now, now)
	return id, err
}

func terminalLabelOrNull(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// CountActiveFiscalTerminals counts LAN terminals with active=1.
func (d *DB) CountActiveFiscalTerminals(storeID string) (int, error) {
	var n int
	err := d.SQL.QueryRow(`SELECT COUNT(1) FROM fiscal_terminals WHERE store_id=? AND active=1`, storeID).Scan(&n)
	return n, err
}

// ListFiscalTerminals returns all terminals for store (active and revoked).
func (d *DB) ListFiscalTerminals(storeID string) ([]FiscalTerminalRow, error) {
	rows, err := d.SQL.Query(`SELECT id, store_id, label, active, ops_terminal_ref, registered_at, last_seen_at, last_seen_ip
		FROM fiscal_terminals WHERE store_id=? ORDER BY registered_at DESC`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FiscalTerminalRow
	for rows.Next() {
		var row FiscalTerminalRow
		var label, last, ip sql.NullString
		var active int
		if err := rows.Scan(&row.ID, &row.StoreID, &label, &active, &row.OpsTerminalRef, &row.RegisteredAt, &last, &ip); err != nil {
			return nil, err
		}
		if label.Valid {
			row.Label = label.String
		}
		if last.Valid {
			row.LastSeenAt = last.String
		}
		if ip.Valid {
			row.LastSeenIP = ip.String
		}
		row.Active = active == 1
		out = append(out, row)
	}
	return out, rows.Err()
}

// GetFiscalTerminalByID returns terminal row.
func (d *DB) GetFiscalTerminalByID(storeID, id string) (*FiscalTerminalRow, error) {
	var row FiscalTerminalRow
	var label sql.NullString
	var last sql.NullString
	var ip sql.NullString
	var active int
	err := d.SQL.QueryRow(`SELECT id, store_id, label, active, ops_terminal_ref, registered_at, last_seen_at, last_seen_ip
		FROM fiscal_terminals WHERE store_id=? AND id=?`, storeID, id).Scan(
		&row.ID, &row.StoreID, &label, &active, &row.OpsTerminalRef, &row.RegisteredAt, &last, &ip)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if label.Valid {
		row.Label = label.String
	}
	if last.Valid {
		row.LastSeenAt = last.String
	}
	if ip.Valid {
		row.LastSeenIP = ip.String
	}
	row.Active = active == 1
	return &row, nil
}

// TouchFiscalTerminal updates last_seen_at / last_seen_ip when active=1 — ONLY runtime touch path.
func (d *DB) TouchFiscalTerminal(storeID, id, clientIP string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := d.SQL.Exec(`UPDATE fiscal_terminals SET last_seen_at=?, last_seen_ip=? WHERE store_id=? AND id=? AND active=1`,
		now, terminalIPOrNull(clientIP), storeID, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func terminalIPOrNull(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// RevokeFiscalTerminal sets active=0 — ONLY local revoke path.
func (d *DB) RevokeFiscalTerminal(storeID, id string) error {
	res, err := d.SQL.Exec(`UPDATE fiscal_terminals SET active=0 WHERE store_id=? AND id=? AND active=1`, storeID, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// TerminalPairCodeTTL is how long an allow-next code stays valid.
const TerminalPairCodeTTL = 15 * time.Minute

// CreateTerminalPairCode mints a one-time local pair code — ONLY allow-next write path.
func (d *DB) CreateTerminalPairCode(storeID, createdBy, label string) (code string, expiresAt string, err error) {
	code, err = randomPairCode()
	if err != nil {
		return "", "", err
	}
	now := time.Now().UTC()
	exp := now.Add(TerminalPairCodeTTL)
	_, err = d.SQL.Exec(`INSERT INTO terminal_pair_codes (
		code, store_id, label, created_by_operator_id, created_at, expires_at, consumed_at
	) VALUES (?, ?, ?, ?, ?, ?, NULL)`,
		code, storeID, terminalLabelOrNull(label), createdBy, now.Format(time.RFC3339), exp.Format(time.RFC3339))
	if err != nil {
		return "", "", err
	}
	return code, exp.Format(time.RFC3339), nil
}

func randomPairCode() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b), nil
}

// RedeemTerminalPairCode consumes a valid code and registers a terminal — ONLY local pair redeem path.
func (d *DB) RedeemTerminalPairCode(storeID, code, labelOverride string) (terminalID, label string, err error) {
	code = trimUpper(code)
	if code == "" {
		return "", "", errors.New("store: pairing_code required")
	}
	tx, err := d.SQL.Begin()
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback()

	var rowLabel sql.NullString
	var expires, consumed sql.NullString
	err = tx.QueryRow(`SELECT label, expires_at, consumed_at FROM terminal_pair_codes
		WHERE code=? AND store_id=?`, code, storeID).Scan(&rowLabel, &expires, &consumed)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrNotFound
	}
	if err != nil {
		return "", "", err
	}
	if consumed.Valid && consumed.String != "" {
		return "", "", fmt.Errorf("store: pairing_code already used")
	}
	exp, err := time.Parse(time.RFC3339, expires.String)
	if err != nil || time.Now().UTC().After(exp) {
		return "", "", fmt.Errorf("store: pairing_code expired")
	}
	label = labelOverride
	if label == "" && rowLabel.Valid {
		label = rowLabel.String
	}
	now := time.Now().UTC().Format(time.RFC3339)
	id := uuid.NewString()
	ref := "local:" + id
	_, err = tx.Exec(`INSERT INTO fiscal_terminals (
		id, store_id, label, active, ops_terminal_ref, registered_at, last_seen_at, last_seen_ip
	) VALUES (?, ?, ?, 1, ?, ?, ?, NULL)`,
		id, storeID, terminalLabelOrNull(label), ref, now, now)
	if err != nil {
		return "", "", err
	}
	res, err := tx.Exec(`UPDATE terminal_pair_codes SET consumed_at=? WHERE code=? AND store_id=? AND consumed_at IS NULL`,
		now, code, storeID)
	if err != nil {
		return "", "", err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return "", "", fmt.Errorf("store: pairing_code already used")
	}
	if err := tx.Commit(); err != nil {
		return "", "", err
	}
	return id, label, nil
}

func trimUpper(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		if c == ' ' || c == '-' {
			continue
		}
		out = append(out, c)
	}
	return string(out)
}
