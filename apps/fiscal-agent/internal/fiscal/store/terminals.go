package store

import (
	"database/sql"
	"errors"
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
}

// UpsertFiscalTerminal syncs one terminal from Ops — ONLY terminal upsert path.
func (d *DB) UpsertFiscalTerminal(storeID, opsRef, label string, active bool) (string, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	var existingID string
	err := d.SQL.QueryRow(`SELECT id FROM fiscal_terminals WHERE store_id=? AND ops_terminal_ref=?`,
		storeID, opsRef).Scan(&existingID)
	if err == nil {
		act := 0
		if active {
			act = 1
		}
		_, err = d.SQL.Exec(`UPDATE fiscal_terminals SET label=?, active=?, last_seen_at=? WHERE id=?`,
			terminalLabelOrNull(label), act, now, existingID)
		return existingID, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	id := uuid.NewString()
	act := 0
	if active {
		act = 1
	}
	_, err = d.SQL.Exec(`INSERT INTO fiscal_terminals (
		id, store_id, label, active, ops_terminal_ref, registered_at, last_seen_at
	) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, storeID, terminalLabelOrNull(label), act, opsRef, now, now)
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

// GetFiscalTerminalByID returns terminal row.
func (d *DB) GetFiscalTerminalByID(storeID, id string) (*FiscalTerminalRow, error) {
	var row FiscalTerminalRow
	var label sql.NullString
	var last sql.NullString
	var active int
	err := d.SQL.QueryRow(`SELECT id, store_id, label, active, ops_terminal_ref, registered_at, last_seen_at
		FROM fiscal_terminals WHERE store_id=? AND id=?`, storeID, id).Scan(
		&row.ID, &row.StoreID, &label, &active, &row.OpsTerminalRef, &row.RegisteredAt, &last)
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
	row.Active = active == 1
	return &row, nil
}

// TouchFiscalTerminal updates last_seen_at.
func (d *DB) TouchFiscalTerminal(storeID, id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := d.SQL.Exec(`UPDATE fiscal_terminals SET last_seen_at=? WHERE store_id=? AND id=? AND active=1`,
		now, storeID, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// SyncFiscalTerminalsFromOps replaces active flags from cloud list — ONLY bulk sync path.
func (d *DB) SyncFiscalTerminalsFromOps(storeID string, terminals []FiscalTerminalRow) error {
	for _, t := range terminals {
		if _, err := d.UpsertFiscalTerminal(storeID, t.OpsTerminalRef, t.Label, t.Active); err != nil {
			return err
		}
	}
	return nil
}
