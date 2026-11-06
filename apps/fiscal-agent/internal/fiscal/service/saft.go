package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/saft"
	"farvoo-fiscal-agent/internal/fiscal/store"

	"github.com/google/uuid"
)

const (
	ErrCodeNoInvoices = "no_invoices"
)

// ExportSAFTInput is input for the ONLY SAF-T export orchestration: an inclusive period
// StartDate..EndDate (YYYY-MM-DD) inside one fiscal year; a calendar month is one such period.
type ExportSAFTInput struct {
	StoreID    string
	StartDate  string
	EndDate    string
	OperatorID string
}

// ExportSAFTResult is the export API response.
type ExportSAFTResult struct {
	ExportID         string `json:"export_id"`
	FileName         string `json:"file_name"`
	FilePath         string `json:"file_path"`
	FileSHA256       string `json:"file_sha256"`
	InvoiceCount     int    `json:"invoice_count"`
	TotalNet         string `json:"total_net"`
	TotalTax         string `json:"total_tax"`
	TotalGross       string `json:"total_gross"`
	ValidationStatus string `json:"validation_status"`
	ValidationErrors string `json:"validation_errors,omitempty"`
}

// ExportSAFT is the ONLY SAF-T export orchestration entry.
func (s *FiscalService) ExportSAFT(_ context.Context, in ExportSAFTInput) (*ExportSAFTResult, error) {
	storeID := in.StoreID
	if storeID == "" {
		storeID = s.storeID
	}
	start, errS := time.Parse("2006-01-02", in.StartDate)
	end, errE := time.Parse("2006-01-02", in.EndDate)
	if errS != nil || errE != nil || end.Before(start) || start.Year() != end.Year() || start.Year() < 2000 {
		return nil, coded(ErrCodeValidationFailed, "start_date and end_date (YYYY-MM-DD) must be ordered within one fiscal year")
	}
	startDate, endDate := in.StartDate, in.EndDate
	taxpayer, err := s.db.GetTaxpayerSettings(storeID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, coded(ErrCodeTaxpayerMissing, "taxpayer not configured")
	}
	if err != nil {
		return nil, err
	}
	loc, err := time.LoadLocation(taxpayer.Timezone)
	if err != nil {
		loc = time.UTC
	}
	invoices, err := s.db.LoadSAFTInvoicesForPeriod(storeID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	payments, err := s.db.LoadSAFTPaymentsForPeriod(storeID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	workDocs, err := s.db.LoadSAFTWorkingDocumentsForPeriod(storeID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	movements, err := s.db.LoadSAFTStockMovementsForPeriod(storeID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	if len(invoices) == 0 && len(payments) == 0 && len(workDocs) == 0 && len(movements) == 0 {
		return nil, coded(ErrCodeNoInvoices, "no invoices in period")
	}

	built, err := saft.Build(saft.BuildInput{
		Taxpayer: taxpayer, StartDate: startDate, EndDate: endDate,
		CreatedDate: time.Now().In(loc).Format("2006-01-02"),
		Invoices:    invoices, Payments: payments, WorkDocs: workDocs, Movements: movements,
	})
	if err != nil {
		return nil, err
	}

	// One immutable file per export row: re-exporting a period never overwrites an archived file.
	exportID := uuid.NewString()
	fileName := fmt.Sprintf("saft_%s_%s_%s_%s.xml", taxpayer.TaxRegistrationNumber, startDate, endDate, exportID[:8])
	dir := filepath.Join(saftRoot(s.dataDir), taxpayer.TaxRegistrationNumber, startDate[:4])
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	filePath := filepath.Join(dir, fileName)
	if err := os.WriteFile(filePath, built.XMLBytes, 0o600); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(built.XMLBytes)
	fileSHA := hex.EncodeToString(sum[:])

	valErrors := ""
	if len(built.ValidationErrors) > 0 {
		b, _ := json.Marshal(built.ValidationErrors)
		valErrors = string(b)
	}
	operatorID := in.OperatorID
	if operatorID == "" {
		operatorID = "op-demo-cashier"
	}
	row, err := s.db.InsertSAFTExport(store.InsertSAFTExportInput{
		ID: exportID, StoreID: storeID, TaxpayerNIF: taxpayer.TaxRegistrationNumber,
		PeriodYear: start.Year(), PeriodMonth: int(start.Month()),
		StartDate: startDate, EndDate: endDate,
		FileName: fileName, FilePath: filePath, FileSHA256: fileSHA,
		InvoiceCount: len(invoices) + len(payments),
		TotalNet: built.TotalNet, TotalTax: built.TotalTax, TotalGross: built.TotalGross,
		ValidationStatus: built.ValidationStatus, ValidationErrors: valErrors,
		CreatedBy: operatorID,
	})
	if err != nil {
		return nil, err
	}
	_ = s.db.InsertAuditLog(operatorID, "EXPORT_SAFT", "saft_exports", row.ID,
		fmt.Sprintf(`{"start_date":%q,"end_date":%q,"invoice_count":%d}`, startDate, endDate, len(invoices)+len(payments)))

	return &ExportSAFTResult{
		ExportID: row.ID, FileName: fileName, FilePath: filePath, FileSHA256: fileSHA,
		InvoiceCount: len(invoices) + len(payments), TotalNet: built.TotalNet, TotalTax: built.TotalTax, TotalGross: built.TotalGross,
		ValidationStatus: built.ValidationStatus, ValidationErrors: valErrors,
	}, nil
}

// ListSAFTExports lists archived exports.
func (s *FiscalService) ListSAFTExports(storeID string, year, month int) ([]store.SAFTExportRow, error) {
	if storeID == "" {
		storeID = s.storeID
	}
	rows, err := s.db.ListSAFTExports(storeID, year, month)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []store.SAFTExportRow{}
	}
	return rows, nil
}

// GetSAFTExport returns export metadata.
func (s *FiscalService) GetSAFTExport(exportID string) (*store.SAFTExportRow, error) {
	row, err := s.db.GetSAFTExport(exportID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, coded(ErrCodeNotFound, "export not found")
	}
	return row, err
}

// GetSAFTExportFile returns export metadata and file bytes for download.
func (s *FiscalService) GetSAFTExportFile(exportID string) (*store.SAFTExportRow, []byte, error) {
	row, err := s.GetSAFTExport(exportID)
	if err != nil {
		return nil, nil, err
	}
	if row.FilePath == "" {
		return row, nil, coded(ErrCodeNotFound, "export file missing")
	}
	data, err := os.ReadFile(row.FilePath)
	if err != nil {
		return row, nil, err
	}
	return row, data, nil
}

func saftRoot(dataDir string) string {
	parent := filepath.Dir(dataDir)
	if parent == "" || parent == "." {
		return filepath.Join(dataDir, "saft")
	}
	return filepath.Join(parent, "saft")
}
