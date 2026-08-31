package service

import (
	"encoding/json"
	"fmt"

	"farvoo-fiscal-agent/internal/fiscal/store"
)

// BackupFiscalDB snapshots the open SQLite file (D6.3).
func (s *FiscalService) BackupFiscalDB() (path string, size int64, err error) {
	path, size, err = s.db.BackupFiscalDB("")
	if err != nil {
		return "", 0, err
	}
	detail, _ := json.Marshal(map[string]any{"path": path, "bytes": size})
	_ = s.db.InsertAuditLog("", "fiscal_db_backup", "sqlite", path, string(detail))
	return path, size, nil
}

// VerifySeriesIntegrity audits series tips vs invoices (D6.3).
func (s *FiscalService) VerifySeriesIntegrity(blockOnFail, healOnPass bool, operatorID string) (*store.SeriesIntegrityReport, error) {
	return s.db.VerifySeriesIntegrity(store.VerifySeriesIntegrityOptions{
		BlockOnFail: blockOnFail,
		HealOnPass:  healOnPass,
		OperatorID:  operatorID,
	})
}

// PrepareMachineSwap backs up (default) then ClearLocalActivation (D6.4).
func (s *FiscalService) PrepareMachineSwap(doBackup bool, operatorID string) (backupPath string, backupBytes int64, err error) {
	if doBackup {
		backupPath, backupBytes, err = s.BackupFiscalDB()
		if err != nil {
			return "", 0, fmt.Errorf("backup before swap: %w", err)
		}
	}
	if err := s.db.ClearLocalActivation(); err != nil {
		return backupPath, backupBytes, err
	}
	s.signer = nil
	detail, _ := json.Marshal(map[string]any{"backup_path": backupPath, "backup_bytes": backupBytes})
	_ = s.db.InsertAuditLog(operatorID, "prepare_machine_swap", "installation", "", string(detail))
	return backupPath, backupBytes, nil
}
