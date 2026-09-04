package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"farvoo-fiscal-agent/internal/fiscal/fiscalsigning"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

const (
	ErrCodeFiscalProfileMissing = "fiscal_profile_missing"
	ErrCodeTerminalsFull        = "terminals_full"
	ErrCodePairInvalid          = "pairing_invalid"
	ErrCodePairExpired          = "pairing_expired"
)

// LoginResult is returned after successful PIN login.
type LoginResult struct {
	OperatorID   string
	DisplayName  string
	Role         string
	SessionEpoch int
}

// BootstrapOwner creates first admin when operators empty (API path bootstrap-owner retained).
func (s *FiscalService) BootstrapOwner(storeID, displayName, pin string) (string, error) {
	if s == nil || s.db == nil {
		return "", fmt.Errorf("fiscal: service nil")
	}
	return s.db.BootstrapOwner(storeID, displayName, pin)
}

// LoginOperator verifies PIN and returns operator metadata.
func (s *FiscalService) LoginOperator(ctx context.Context, storeID, operatorID, pin, clientIP string) (*LoginResult, error) {
	if err := s.db.VerifyOperatorPIN(storeID, operatorID, pin, clientIP); err != nil {
		return nil, err
	}
	var name, role string
	var epoch int
	err := s.db.SQL.QueryRow(`SELECT display_name, role, session_epoch FROM operators WHERE store_id=? AND id=? AND active=1`,
		storeID, operatorID).Scan(&name, &role, &epoch)
	if err != nil {
		return nil, err
	}
	_ = s.db.InsertAuditLog(operatorID, "LOGIN", "operator", operatorID, "{}")
	return &LoginResult{OperatorID: operatorID, DisplayName: name, Role: role, SessionEpoch: epoch}, nil
}

// ChangeOperatorPIN is self-service PIN change.
func (s *FiscalService) ChangeOperatorPIN(storeID, operatorID, oldPIN, newPIN string) error {
	return s.db.ChangeOperatorPIN(storeID, operatorID, oldPIN, newPIN)
}

// PullAndSaveStorePolicy fetches Ops fiscal_profile only — does NOT overwrite local max_fiscal_terminals.
func (s *FiscalService) PullAndSaveStorePolicy(ctx context.Context, storeID string) error {
	cli := &fiscalsigning.Client{APIBase: s.cloud.APIBase, JWT: s.cloud.JWT}
	policy, err := cli.PullStorePolicy(ctx)
	if err != nil {
		return err
	}
	if policy.FiscalProfile != "restaurant" && policy.FiscalProfile != "retail" {
		return coded(ErrCodeFiscalProfileMissing, "ops fiscal_profile not configured")
	}
	return s.db.SaveOpsFiscalProfile(storeID, policy.FiscalProfile)
}

// TerminalPairResult is local terminal registration result.
type TerminalPairResult struct {
	TerminalID string
	Label      string
}

// AllowNextTerminalCodeResult is returned when manager mints an allow-next code.
type AllowNextTerminalCodeResult struct {
	Code      string
	ExpiresAt string
	Label     string
}

// SetMaxFiscalTerminals is admin-only local max — ONLY service max write.
func (s *FiscalService) SetMaxFiscalTerminals(storeID string, max int, actorID string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("fiscal: service nil")
	}
	if err := s.db.SetMaxFiscalTerminals(storeID, max); err != nil {
		return err
	}
	_ = s.db.InsertAuditLog(actorID, "TERMINAL_MAX_SET", "taxpayer_settings", storeID,
		fmt.Sprintf(`{"max_fiscal_terminals":%d}`, max))
	return nil
}

// AllowNextTerminal mints a one-time local pair code — ONLY allow-next orchestration.
func (s *FiscalService) AllowNextTerminal(storeID, actorID, label string) (*AllowNextTerminalCodeResult, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("fiscal: service nil")
	}
	label = strings.TrimSpace(label)
	if label == "" {
		return nil, coded(ErrCodeValidationFailed, "label required")
	}
	used, max, err := s.TerminalSummary(storeID)
	if err != nil {
		return nil, err
	}
	if used >= max {
		return nil, coded(ErrCodeTerminalsFull, "terminal slots full")
	}
	code, exp, err := s.db.CreateTerminalPairCode(storeID, actorID, label)
	if err != nil {
		return nil, err
	}
	_ = s.db.InsertAuditLog(actorID, "TERMINAL_ALLOW_NEXT", "terminal_pair_code", code, "{}")
	return &AllowNextTerminalCodeResult{Code: code, ExpiresAt: exp, Label: label}, nil
}

// PairFiscalTerminal redeems a local allow-next code — ONLY LAN pair orchestration (no Ops).
func (s *FiscalService) PairFiscalTerminal(storeID, pairingCode, label string) (*TerminalPairResult, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("fiscal: service nil")
	}
	used, max, err := s.TerminalSummary(storeID)
	if err != nil {
		return nil, err
	}
	if used >= max {
		return nil, coded(ErrCodeTerminalsFull, "terminal slots full")
	}
	id, lab, err := s.db.RedeemTerminalPairCode(storeID, pairingCode, strings.TrimSpace(label))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, coded(ErrCodePairInvalid, "invalid pairing code")
		}
		msg := err.Error()
		if strings.Contains(msg, "expired") {
			return nil, coded(ErrCodePairExpired, "pairing code expired")
		}
		if strings.Contains(msg, "already used") {
			return nil, coded(ErrCodePairInvalid, "pairing code already used")
		}
		return nil, err
	}
	// Re-check after redeem (race): if somehow over max, revoke immediately.
	used2, _, _ := s.TerminalSummary(storeID)
	if used2 > max {
		_ = s.db.RevokeFiscalTerminal(storeID, id)
		return nil, coded(ErrCodeTerminalsFull, "terminal slots full")
	}
	_ = s.db.InsertAuditLog("", "TERMINAL_PAIR", "fiscal_terminal", id, "{}")
	return &TerminalPairResult{TerminalID: id, Label: lab}, nil
}

// ListFiscalTerminals returns registered LAN terminals.
func (s *FiscalService) ListFiscalTerminals(storeID string) ([]store.FiscalTerminalRow, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("fiscal: service nil")
	}
	return s.db.ListFiscalTerminals(storeID)
}

// RevokeFiscalTerminal deactivates a LAN terminal — ONLY revoke orchestration.
func (s *FiscalService) RevokeFiscalTerminal(storeID, terminalID, actorID string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("fiscal: service nil")
	}
	if err := s.db.RevokeFiscalTerminal(storeID, terminalID); err != nil {
		return err
	}
	_ = s.db.InsertAuditLog(actorID, "TERMINAL_REVOKE", "fiscal_terminal", terminalID, "{}")
	return nil
}

// ActivateFiscalTerminal re-enables inactive terminal — ONLY activate orchestration.
func (s *FiscalService) ActivateFiscalTerminal(storeID, terminalID, actorID string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("fiscal: service nil")
	}
	used, max, err := s.TerminalSummary(storeID)
	if err != nil {
		return err
	}
	if used >= max {
		return coded(ErrCodeTerminalsFull, "terminal slots full")
	}
	if err := s.db.ActivateFiscalTerminal(storeID, terminalID); err != nil {
		return err
	}
	_ = s.db.InsertAuditLog(actorID, "TERMINAL_ACTIVATE", "fiscal_terminal", terminalID, "{}")
	return nil
}

// DeleteInactiveFiscalTerminal removes inactive terminal — ONLY delete orchestration.
func (s *FiscalService) DeleteInactiveFiscalTerminal(storeID, terminalID, actorID string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("fiscal: service nil")
	}
	if err := s.db.DeleteInactiveFiscalTerminal(storeID, terminalID); err != nil {
		return err
	}
	_ = s.db.InsertAuditLog(actorID, "TERMINAL_DELETE", "fiscal_terminal", terminalID, "{}")
	return nil
}

// SetFiscalTerminalDefaultStation sets LAN terminal print station — ONLY terminal station writer.
func (s *FiscalService) SetFiscalTerminalDefaultStation(storeID, terminalID, stationID, actorID string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("fiscal: service nil")
	}
	if err := s.db.SetFiscalTerminalDefaultStation(storeID, terminalID, stationID); err != nil {
		return err
	}
	_ = s.db.InsertAuditLog(actorID, "TERMINAL_STATION_SET", "fiscal_terminal", terminalID,
		fmt.Sprintf(`{"station_id":%q}`, strings.TrimSpace(stationID)))
	return nil
}

// SetFiscalTerminalLabel sets LAN terminal note — ONLY terminal label writer.
func (s *FiscalService) SetFiscalTerminalLabel(storeID, terminalID, label, actorID string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("fiscal: service nil")
	}
	label = strings.TrimSpace(label)
	if label == "" {
		return coded(ErrCodeValidationFailed, "label required")
	}
	if err := s.db.SetFiscalTerminalLabel(storeID, terminalID, label); err != nil {
		return err
	}
	_ = s.db.InsertAuditLog(actorID, "TERMINAL_LABEL_SET", "fiscal_terminal", terminalID,
		fmt.Sprintf(`{"label":%q}`, label))
	return nil
}

// GetLocalDefaultStation returns loopback default print station.
func (s *FiscalService) GetLocalDefaultStation(storeID string) (string, error) {
	if s == nil || s.db == nil {
		return "", fmt.Errorf("fiscal: service nil")
	}
	return s.db.GetLocalDefaultStation(storeID)
}

// SetLocalDefaultStation sets loopback default print station — ONLY local station writer.
func (s *FiscalService) SetLocalDefaultStation(storeID, stationID, actorID string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("fiscal: service nil")
	}
	if err := s.db.SetLocalDefaultStation(storeID, stationID); err != nil {
		return err
	}
	_ = s.db.InsertAuditLog(actorID, "LOCAL_STATION_SET", "taxpayer_settings", storeID,
		fmt.Sprintf(`{"station_id":%q}`, strings.TrimSpace(stationID)))
	return nil
}

// GetFiscalTerminal returns one LAN terminal row.
func (s *FiscalService) GetFiscalTerminal(storeID, terminalID string) (*store.FiscalTerminalRow, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("fiscal: service nil")
	}
	return s.db.GetFiscalTerminalByID(storeID, terminalID)
}

// TerminalSummary returns used/max terminal slots (LAN only).
func (s *FiscalService) TerminalSummary(storeID string) (used, max int, err error) {
	used, err = s.db.CountActiveFiscalTerminals(storeID)
	if err != nil {
		return 0, 1, err
	}
	_, _, max, err = s.db.FiscalProfileOK(storeID)
	if err != nil {
		return used, 1, err
	}
	return used, max, nil
}
