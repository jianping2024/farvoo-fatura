package store

import (
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"
)

var pinSixDigits = regexp.MustCompile(`^\d{6}$`)

var (
	ErrInvalidPIN          = errors.New("store: invalid pin")
	ErrPINMismatch         = errors.New("store: pin mismatch")
	ErrOperatorLocked      = errors.New("store: operator locked")
	ErrBootstrapNotEmpty   = errors.New("store: bootstrap not empty")
	ErrLastOwnerConstraint = errors.New("store: cannot remove last owner with nc")
)

// OperatorLoginRow is safe for login page listing.
type OperatorLoginRow struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	HasPIN      bool   `json:"has_pin"`
}

type pinParams struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
}

var defaultPinParams = pinParams{
	memory:      64 * 1024,
	iterations:  2,
	parallelism: 1,
	saltLength:  16,
	keyLength:   32,
}

func hashPIN(pin string) (string, error) {
	if !pinSixDigits.MatchString(pin) {
		return "", ErrInvalidPIN
	}
	p := defaultPinParams
	salt := make([]byte, p.saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(pin), salt, p.iterations, p.memory, p.parallelism, p.keyLength)
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.memory, p.iterations, p.parallelism, b64Salt, b64Hash), nil
}

func verifyPINHash(pin, encoded string) bool {
	if !pinSixDigits.MatchString(pin) {
		return false
	}
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var version int
	var memory, iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(pin), salt, iterations, memory, parallelism, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// SetOperatorPIN is the ONLY PIN write path.
func (d *DB) SetOperatorPIN(storeID, operatorID, pin string) error {
	h, err := hashPIN(pin)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := d.SQL.Exec(`UPDATE operators SET pin_hash=?, updated_at=? WHERE store_id=? AND id=? AND active=1`,
		h, now, storeID, operatorID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ChangeOperatorPIN verifies old PIN then writes new hash — ONLY self-change path.
func (d *DB) ChangeOperatorPIN(storeID, operatorID, oldPIN, newPIN string) error {
	var hash sql.NullString
	err := d.SQL.QueryRow(`SELECT pin_hash FROM operators WHERE store_id=? AND id=? AND active=1`,
		storeID, operatorID).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if !hash.Valid || !verifyPINHash(oldPIN, hash.String) {
		return ErrPINMismatch
	}
	return d.SetOperatorPIN(storeID, operatorID, newPIN)
}

// VerifyOperatorPIN checks PIN and records lockout — ONLY login verify path.
func (d *DB) VerifyOperatorPIN(storeID, operatorID, pin string) error {
	if locked, _ := d.isOperatorLocked(storeID, operatorID); locked {
		return ErrOperatorLocked
	}
	var hash sql.NullString
	var active int
	err := d.SQL.QueryRow(`SELECT pin_hash, active FROM operators WHERE store_id=? AND id=?`,
		storeID, operatorID).Scan(&hash, &active)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if active != 1 {
		return ErrNotFound
	}
	if !hash.Valid {
		return ErrInvalidPIN
	}
	if verifyPINHash(pin, hash.String) {
		_ = d.clearLoginFailures(storeID, operatorID)
		return nil
	}
	_ = d.recordLoginFailure(storeID, operatorID)
	if locked, _ := d.isOperatorLocked(storeID, operatorID); locked {
		return ErrOperatorLocked
	}
	return ErrPINMismatch
}

func (d *DB) recordLoginFailure(storeID, operatorID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	key := "login_fail:" + storeID + ":" + operatorID
	_, err := d.SQL.Exec(`INSERT INTO audit_log (id, at, operator_id, action, entity_type, entity_id, detail_json)
		VALUES (?, ?, ?, 'LOGIN_FAILED', 'operator', ?, '{}')`,
		uuid.NewString(), now, operatorID, key)
	return err
}

func (d *DB) clearLoginFailures(storeID, operatorID string) error {
	key := "login_fail:" + storeID + ":" + operatorID
	_, err := d.SQL.Exec(`DELETE FROM audit_log WHERE action='LOGIN_FAILED' AND entity_id=?`, key)
	return err
}

func (d *DB) isOperatorLocked(storeID, operatorID string) (bool, error) {
	key := "login_fail:" + storeID + ":" + operatorID
	var n int
	err := d.SQL.QueryRow(`SELECT COUNT(1) FROM audit_log WHERE action='LOGIN_FAILED' AND entity_id=? AND at > datetime('now', '-15 minutes')`,
		key).Scan(&n)
	return n >= 5, err
}

// CountOperators returns operator count for store.
func (d *DB) CountOperators(storeID string) (int, error) {
	var n int
	err := d.SQL.QueryRow(`SELECT COUNT(1) FROM operators WHERE store_id=?`, storeID).Scan(&n)
	return n, err
}

// BootstrapOwner creates first owner when operators empty — ONLY bootstrap write path (H3).
func (d *DB) BootstrapOwner(storeID, displayName, pin string) (string, error) {
	if strings.TrimSpace(displayName) == "" {
		return "", errors.New("store: display_name required")
	}
	tx, err := d.SQL.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var n int
	if err := tx.QueryRow(`SELECT COUNT(1) FROM operators WHERE store_id=?`, storeID).Scan(&n); err != nil {
		return "", err
	}
	if n > 0 {
		return "", ErrBootstrapNotEmpty
	}
	id := uuid.NewString()
	mesaID := "local-" + id
	h, err := hashPIN(pin)
	if err != nil {
		return "", err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = tx.Exec(`INSERT INTO operators (
		id, mesa_user_id, store_id, role, display_name, active, pin_hash, can_issue_nc, synced_at, created_at, updated_at
	) VALUES (?, ?, ?, 'owner', ?, 1, ?, 1, NULL, ?, ?)`,
		id, mesaID, storeID, displayName, h, now, now)
	if err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return id, nil
}

// ListOperatorsForLogin lists operators without pin_hash.
func (d *DB) ListOperatorsForLogin(storeID string) ([]OperatorLoginRow, error) {
	rows, err := d.SQL.Query(`SELECT id, display_name, role, pin_hash FROM operators
		WHERE store_id=? AND active=1 ORDER BY display_name`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OperatorLoginRow
	for rows.Next() {
		var r OperatorLoginRow
		var pin sql.NullString
		if err := rows.Scan(&r.ID, &r.DisplayName, &r.Role, &pin); err != nil {
			return nil, err
		}
		r.HasPIN = pin.Valid && pin.String != ""
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetOperatorRole returns role for operator.
func (d *DB) GetOperatorRole(storeID, operatorID string) (string, error) {
	var role string
	var active int
	err := d.SQL.QueryRow(`SELECT role, active FROM operators WHERE store_id=? AND id=?`, storeID, operatorID).Scan(&role, &active)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if active != 1 {
		return "", ErrNotFound
	}
	return role, nil
}