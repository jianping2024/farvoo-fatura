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

// PullAndSaveStorePolicy fetches Ops policy and writes taxpayer_settings — ONLY policy sync entry.
func (s *FiscalService) PullAndSaveStorePolicy(ctx context.Context, storeID string) error {
	cli := &fiscalsigning.Client{APIBase: s.cloud.APIBase, JWT: s.cloud.JWT}
	policy, err := cli.PullStorePolicy(ctx)
	if err != nil {
		return err
	}
	if policy.FiscalProfile != "restaurant" && policy.FiscalProfile != "retail" {
		return coded(ErrCodeFiscalProfileMissing, "ops fiscal_profile not configured")
	}
	max := policy.MaxFiscalTerminals
	if max < 1 {
		max = 1
	}
	return s.db.SaveOpsStorePolicy(storeID, policy.FiscalProfile, max)
}

// SyncFiscalTerminalsFromCloud pulls terminal list from Ops.
func (s *FiscalService) SyncFiscalTerminalsFromCloud(ctx context.Context, storeID string) error {
	cli := &fiscalsigning.Client{APIBase: s.cloud.APIBase, JWT: s.cloud.JWT}
	list, err := cli.ListFiscalTerminals(ctx)
	if err != nil {
		return err
	}
	rows := make([]store.FiscalTerminalRow, 0, len(list))
	for _, t := range list {
		ref := t.OpsTerminalRef
		if ref == "" {
			ref = t.ID
		}
		rows = append(rows, store.FiscalTerminalRow{
			OpsTerminalRef: ref,
			Label:          t.Label,
			Active:         t.Active,
		})
	}
	return s.db.SyncFiscalTerminalsFromOps(storeID, rows)
}

// TerminalPairResult is local terminal registration result.
type TerminalPairResult struct {
	TerminalID string
	Label      string
}

// PairFiscalTerminal redeems Ops pairing code — ONLY terminal pair orchestration.
func (s *FiscalService) PairFiscalTerminal(ctx context.Context, storeID, pairingCode, label string) (*TerminalPairResult, error) {
	if strings.TrimSpace(s.cloud.APIBase) == "" || strings.TrimSpace(s.cloud.JWT) == "" {
		return nil, fmt.Errorf("fiscal: cloud pairing requires agent jwt")
	}
	if err := s.PullAndSaveStorePolicy(ctx, storeID); err != nil {
		var ce *CodedError
		if errors.As(err, &ce) && ce.Code == ErrCodeFiscalProfileMissing {
			return nil, err
		}
	}
	used, max, err := s.TerminalSummary(storeID)
	if err != nil {
		return nil, err
	}
	if used >= max {
		return nil, coded(ErrCodeTerminalsFull, "terminal slots full")
	}
	cli := &fiscalsigning.Client{APIBase: s.cloud.APIBase, JWT: s.cloud.JWT}
	res, err := cli.PairFiscalTerminal(ctx, pairingCode, label)
	if err != nil {
		if strings.Contains(err.Error(), "terminals_full") {
			return nil, coded(ErrCodeTerminalsFull, "terminal slots full")
		}
		return nil, err
	}
	ref := res.OpsTerminalRef
	if ref == "" {
		ref = res.TerminalID
	}
	id, err := s.db.UpsertFiscalTerminal(storeID, ref, res.Label, true)
	if err != nil {
		return nil, err
	}
	_ = s.SyncFiscalTerminalsFromCloud(ctx, storeID)
	return &TerminalPairResult{TerminalID: id, Label: res.Label}, nil
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
