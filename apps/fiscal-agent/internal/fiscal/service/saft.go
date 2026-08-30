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

	"farvoo-fiscal-agent/internal/fiscal/saft"
	"farvoo-fiscal-agent/internal/fiscal/store"
)

const (
	ErrCodeNoInvoices = "no_invoices"
)

// ExportSAFTInput is input for the ONLY SAF-T export orchestration.
type ExportSAFTInput struct {
	StoreID    string
	Year       int
	Month      int
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
	if in.Year < 2000 || in.Month < 1 || in.Month > 12 {
		return nil, coded("validation_failed", "year and month required")
	}
	taxpayer, err := s.db.GetTaxpayerSettings(storeID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, coded(ErrCodeTaxpayerMissing, "taxpayer not configured")
	}
	if err != nil {
		return nil, err
	}
	startDate, endDate, err := store.SAFTPeriodBounds(in.Year, in.Month, taxpayer.Timezone)
	if err != nil {
		return nil, err
	}
	invoices, err := s.db.LoadSAFTInvoicesForPeriod(storeID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	if len(invoices) == 0 {
		return nil, coded(ErrCodeNoInvoices, "no invoices in period")
	}

	built, err := saft.Build(saft.BuildInput{
		Taxpayer: taxpayer, Year: in.Year, Month: in.Month,
		StartDate: startDate, EndDate: endDate, Invoices: invoices,
	})
	if err != nil {
		return nil, err
	}

	fileName := fmt.Sprintf("saft_%s_%d-%02d.xml", taxpayer.TaxRegistrationNumber, in.Year, in.Month)
	dir := filepath.Join(saftRoot(s.dataDir), taxpayer.TaxRegistrationNumber, fmt.Sprintf("%d", in.Year))
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
		StoreID: storeID, TaxpayerNIF: taxpayer.TaxRegistrationNumber,
		PeriodYear: in.Year, PeriodMonth: in.Month,
		StartDate: startDate, EndDate: endDate,
		FileName: fileName, FilePath: filePath, FileSHA256: fileSHA,
		InvoiceCount: len(invoices),
		TotalNet: built.TotalNet, TotalTax: built.TotalTax, TotalGross: built.TotalGross,
		ValidationStatus: built.ValidationStatus, ValidationErrors: valErrors,
		CreatedBy: operatorID,
	})
	if err != nil {
		return nil, err
	}
	_ = s.db.InsertAuditLog(operatorID, "EXPORT_SAFT", "saft_exports", row.ID,
		fmt.Sprintf(`{"year":%d,"month":%d,"invoice_count":%d}`, in.Year, in.Month, len(invoices)))

	return &ExportSAFTResult{
		ExportID: row.ID, FileName: fileName, FilePath: filePath, FileSHA256: fileSHA,
		InvoiceCount: len(invoices), TotalNet: built.TotalNet, TotalTax: built.TotalTax, TotalGross: built.TotalGross,
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
