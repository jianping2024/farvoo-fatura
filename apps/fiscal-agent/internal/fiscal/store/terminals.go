package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// IssueTerminalInput is setup payload for pairing an issue terminal.
type IssueTerminalInput struct {
	ID          string `json:"id"`
	StoreID     string `json:"store_id"`
	DisplayName string `json:"display_name"`
	Secret      string `json:"secret"` // plaintext once; stored as SHA-256 hex
	StationID   string `json:"station_id"`
	Active      *bool  `json:"active"`
}

// UpsertIssueTerminal is the ONLY write path for issue_terminals.
func (d *DB) UpsertIssueTerminal(p IssueTerminalInput) error {
	id := strings.TrimSpace(p.ID)
	storeID := strings.TrimSpace(p.StoreID)
	secret := strings.TrimSpace(p.Secret)
	if id == "" || storeID == "" || secret == "" {
		return fmt.Errorf("store: issue terminal id, store_id, secret required")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	sum := sha256.Sum256([]byte(secret))
	hash := hex.EncodeToString(sum[:])
	active := 1
	if p.Active != nil && !*p.Active {
		active = 0
	}
	var existing string
	err := d.SQL.QueryRow(`SELECT id FROM issue_terminals WHERE id = ?`, id).Scan(&existing)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = d.SQL.Exec(`INSERT INTO issue_terminals (
			id, store_id, display_name, secret_hash, station_id, active, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			id, storeID, p.DisplayName, hash, nullStr(p.StationID), active, now, now)
		return err
	}
	if err != nil {
		return err
	}
	_, err = d.SQL.Exec(`UPDATE issue_terminals SET store_id=?, display_name=?, secret_hash=?, station_id=?,
		active=?, updated_at=? WHERE id=?`,
		storeID, p.DisplayName, hash, nullStr(p.StationID), active, now, id)
	return err
}

// VerifyIssueTerminal checks terminal id+secret for store — ONLY verify path for §13 terminal.
func (d *DB) VerifyIssueTerminal(storeID, terminalID, secret string) (stationID string, err error) {
	storeID = strings.TrimSpace(storeID)
	terminalID = strings.TrimSpace(terminalID)
	secret = strings.TrimSpace(secret)
	if storeID == "" || terminalID == "" || secret == "" {
		return "", fmt.Errorf("store: terminal credentials required")
	}
	var hash string
	var active int
	var station sql.NullString
	err = d.SQL.QueryRow(`SELECT secret_hash, active, station_id FROM issue_terminals
		WHERE id = ? AND store_id = ?`, terminalID, storeID).Scan(&hash, &active, &station)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("store: terminal not found")
	}
	if err != nil {
		return "", err
	}
	if active != 1 {
		return "", fmt.Errorf("store: terminal inactive")
	}
	sum := sha256.Sum256([]byte(secret))
	if hex.EncodeToString(sum[:]) != hash {
		return "", fmt.Errorf("store: terminal secret mismatch")
	}
	if station.Valid {
		stationID = station.String
	}
	return stationID, nil
}

// EnsureOperatorFromMesa upserts an active operator by mesa_user_id — ONLY lazy-create path for §13.
func (d *DB) EnsureOperatorFromMesa(storeID, mesaUserID, role, displayName string) (operatorID string, err error) {
	storeID = strings.TrimSpace(storeID)
	mesaUserID = strings.TrimSpace(mesaUserID)
	if storeID == "" || mesaUserID == "" {
		return "", fmt.Errorf("store: store_id and mesa_user_id required")
	}
	role = strings.TrimSpace(role)
	if role == "" {
		role = "cashier"
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		displayName = mesaUserID
	}
	now := time.Now().UTC().Format(time.RFC3339)
	err = d.SQL.QueryRow(`SELECT id FROM operators WHERE mesa_user_id = ?`, mesaUserID).Scan(&operatorID)
	if err == nil {
		_, err = d.SQL.Exec(`UPDATE operators SET store_id=?, role=?, display_name=?, active=1, synced_at=?, updated_at=?
			WHERE id=?`, storeID, role, displayName, now, now, operatorID)
		return operatorID, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	operatorID = "op-" + uuid.NewString()
	_, err = d.SQL.Exec(`INSERT INTO operators (
		id, mesa_user_id, store_id, role, display_name, active, pin_hash, can_issue_nc, synced_at, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, 1, NULL, 0, ?, ?, ?)`,
		operatorID, mesaUserID, storeID, role, displayName, now, now, now)
	return operatorID, err
}
