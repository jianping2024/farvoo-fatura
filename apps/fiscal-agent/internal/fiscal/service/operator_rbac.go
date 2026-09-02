package service

import (
	"errors"
	"strings"

	"farvoo-fiscal-agent/internal/fiscal/store"
)

const ErrCodeForbidden = "forbidden"

// UpsertOperatorWithActor applies M3.2c role write policy — ONLY operator upsert entry from handlers.
func (s *FiscalService) UpsertOperatorWithActor(actorRole, actorID, id, storeID, role, name string) error {
	if s == nil || s.db == nil {
		return errors.New("fiscal: service nil")
	}
	role = strings.TrimSpace(role)
	if role == "" {
		role = "cashier"
	}
	if actorID != "" && actorID == id {
		currentRole, err := s.db.GetOperatorPolicyRole(storeID, id)
		if err != nil {
			return err
		}
		if role != currentRole {
			return coded(ErrCodeForbidden, "cannot change own role")
		}
		return s.db.UpsertOperator(id, storeID, currentRole, name, "local-"+id)
	}
	if err := validateOperatorRoleWrite(actorRole, role); err != nil {
		return err
	}
	if actorRole == "owner" {
		targetRole, err := s.db.GetOperatorPolicyRole(storeID, id)
		if err == nil && targetRole != "cashier" {
			return coded(ErrCodeForbidden, "owner may only manage cashiers")
		}
		if errors.Is(err, store.ErrNotFound) && role != "cashier" {
			return coded(ErrCodeForbidden, "owner may only create cashiers")
		}
	}
	return s.db.UpsertOperator(id, storeID, role, name, "local-"+id)
}

// SetOperatorActiveWithActor applies M3.2c deactivate policy.
func (s *FiscalService) SetOperatorActiveWithActor(actorRole, storeID, operatorID string, active bool) error {
	if s == nil || s.db == nil {
		return errors.New("fiscal: service nil")
	}
	targetRole, err := s.db.GetOperatorPolicyRole(storeID, operatorID)
	if err != nil {
		return err
	}
	if actorRole == "owner" && targetRole != "cashier" {
		return coded(ErrCodeForbidden, "owner may only manage cashiers")
	}
	return s.db.SetOperatorActive(storeID, operatorID, active)
}

// SetOperatorCanIssueNCWithActor applies M3.2c can_issue_nc policy.
func (s *FiscalService) SetOperatorCanIssueNCWithActor(actorRole, storeID, operatorID string, canIssue bool) error {
	if s == nil || s.db == nil {
		return errors.New("fiscal: service nil")
	}
	if actorRole == "owner" {
		targetRole, err := s.db.GetOperatorPolicyRole(storeID, operatorID)
		if err != nil {
			return err
		}
		if targetRole != "cashier" {
			return coded(ErrCodeForbidden, "owner may only manage cashiers")
		}
	}
	return s.db.SetOperatorCanIssueNC(storeID, operatorID, canIssue)
}

// SetOperatorPINWithActor applies M3.2c PIN reset policy — ONLY handler PIN reset entry.
func (s *FiscalService) SetOperatorPINWithActor(actorRole, actorID, storeID, operatorID, pin string) error {
	if s == nil || s.db == nil {
		return errors.New("fiscal: service nil")
	}
	if actorRole != "admin" && actorRole != "owner" {
		return coded(ErrCodeForbidden, "forbidden")
	}
	if actorRole == "owner" {
		targetRole, err := s.db.GetOperatorPolicyRole(storeID, operatorID)
		if err != nil {
			return err
		}
		if targetRole != "cashier" {
			return coded(ErrCodeForbidden, "owner may only manage cashiers")
		}
	}
	return s.db.SetOperatorPIN(storeID, operatorID, pin)
}

func validateOperatorRoleWrite(actorRole, role string) error {
	switch actorRole {
	case "admin":
		if role == "admin" {
			return coded(ErrCodeForbidden, "cannot create admin via API")
		}
		if role != "owner" && role != "cashier" {
			return coded(ErrCodeForbidden, "invalid role")
		}
	case "owner":
		if role != "cashier" {
			return coded(ErrCodeForbidden, "owner may only assign cashier role")
		}
	default:
		return coded(ErrCodeForbidden, "forbidden")
	}
	return nil
}
