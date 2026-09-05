package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// StateFileName is the ONLY default basename next to the exe.
const StateFileName = "fiscal-devtool-state.json"

// SettlementEntry is one settled business day for one fiscal.db.
type SettlementEntry struct {
	BusinessDate string `json:"business_date"`
	DBPath       string `json:"db_path"`
	SettledAt    string `json:"settled_at"`
	KeepTarget   string `json:"keep_target"`
	KeepActual   string `json:"keep_actual"`
	DeletedCount int    `json:"deleted_count"`
	CutoffSeq    int64  `json:"cutoff_seq"`
	SeriesID     string `json:"series_id"`
	DetailJSON   string `json:"detail_json"`
}

// ToolState is the ONLY on-disk model for PIN + settlements (exe-adjacent JSON).
type ToolState struct {
	PinHash       string                      `json:"pin_hash"`
	MustChangePIN bool                        `json:"must_change_pin"`
	UpdatedAt     string                      `json:"updated_at"`
	Settlements   map[string]SettlementEntry  `json:"settlements"` // key = settleKey(dbPath, businessDate)
}

var stateMu sync.Mutex

// DefaultStatePath is the ONLY default path: <exeDir>/fiscal-devtool-state.json.
func DefaultStatePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return filepath.Join(filepath.Dir(exe), StateFileName), nil
	}
	return filepath.Join(filepath.Dir(exe), StateFileName), nil
}

// NormalizeDBPath is the ONLY db identity for settlement keys.
func NormalizeDBPath(dbPath string) (string, error) {
	abs, err := filepath.Abs(strings.TrimSpace(dbPath))
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

func settleKey(dbPath, businessDate string) string {
	return dbPath + "|" + businessDate
}

// LoadToolState is the ONLY state file reader (creates empty bootstrap PIN state if missing).
func LoadToolState(path string) (*ToolState, error) {
	stateMu.Lock()
	defer stateMu.Unlock()
	return loadToolStateLocked(path)
}

func loadToolStateLocked(path string) (*ToolState, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		h, err := hashDevtoolPIN(InitialDevtoolPIN)
		if err != nil {
			return nil, err
		}
		st := &ToolState{
			PinHash:       h,
			MustChangePIN: true,
			UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
			Settlements:   map[string]SettlementEntry{},
		}
		if err := saveToolStateLocked(path, st); err != nil {
			return nil, err
		}
		return st, nil
	}
	var st ToolState
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, fmt.Errorf("state file: %w", err)
	}
	if st.Settlements == nil {
		st.Settlements = map[string]SettlementEntry{}
	}
	if strings.TrimSpace(st.PinHash) == "" {
		h, err := hashDevtoolPIN(InitialDevtoolPIN)
		if err != nil {
			return nil, err
		}
		st.PinHash = h
		st.MustChangePIN = true
		st.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		if err := saveToolStateLocked(path, &st); err != nil {
			return nil, err
		}
	}
	return &st, nil
}

// SaveToolState is the ONLY state file writer.
func SaveToolState(path string, st *ToolState) error {
	stateMu.Lock()
	defer stateMu.Unlock()
	return saveToolStateLocked(path, st)
}

func saveToolStateLocked(path string, st *ToolState) error {
	if st.Settlements == nil {
		st.Settlements = map[string]SettlementEntry{}
	}
	st.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	body, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// IsDateSettledInState reports whether businessDate is settled for this dbPath.
func IsDateSettledInState(st *ToolState, dbPath, businessDate string) bool {
	if st == nil || st.Settlements == nil {
		return false
	}
	_, ok := st.Settlements[settleKey(dbPath, businessDate)]
	return ok
}

// MarkSettledInState is the ONLY settlement-mark writer (mutates st; caller SaveToolState).
func MarkSettledInState(st *ToolState, entry SettlementEntry) error {
	if st == nil {
		return fmt.Errorf("nil state")
	}
	dbPath, err := NormalizeDBPath(entry.DBPath)
	if err != nil {
		return err
	}
	entry.DBPath = dbPath
	entry.BusinessDate = strings.TrimSpace(entry.BusinessDate)
	if entry.BusinessDate == "" {
		return fmt.Errorf("business_date required")
	}
	if st.Settlements == nil {
		st.Settlements = map[string]SettlementEntry{}
	}
	key := settleKey(dbPath, entry.BusinessDate)
	if _, exists := st.Settlements[key]; exists {
		return fmt.Errorf("营业日 %s 已结算", entry.BusinessDate)
	}
	st.Settlements[key] = entry
	return nil
}
