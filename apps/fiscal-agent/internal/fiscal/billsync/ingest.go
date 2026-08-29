package billsync

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"farvoo-fiscal-agent/internal/fiscal/store"
	"farvoo-fiscal-agent/internal/fiscal/vatpercent"
)

// Error codes for ack / HTTP.
const (
	CodeAlreadyInvoiced   = "already_invoiced"
	CodeValidationFailed  = "validation_failed"
	CodeItemCodeConflict  = "item_code_conflict"
	CodeInvalidVATRate    = "invalid_vat_rate"
	CodeEmptyItemCode     = "empty_item_code"
	CodePersistFailed     = "persist_failed"
)

// IngestError is a typed failure for ack.
type IngestError struct {
	Code    string
	Message string
}

func (e *IngestError) Error() string { return e.Message }

func ingestErr(code, msg string) error { return &IngestError{Code: code, Message: msg} }

// Snapshot is the Farvoo bill_sync_jobs payload body (+ Agent freeze fields).
type Snapshot struct {
	RequestID        string      `json:"request_id"`
	SourceSystem     string      `json:"source_system"`
	SourceSaleID     string      `json:"source_sale_id"`
	TableDisplayName string      `json:"table_display_name"`
	ScopeType        string      `json:"scope_type"`
	Lines            []Line      `json:"lines"`
	GrossTotal       string      `json:"gross_total"`
	Splits           []SplitPart `json:"splits"`
	// SourceLines is frozen at ingest (Agent); allocation shares reference LineKey.
	SourceLines []Line `json:"source_lines,omitempty"`
}

// Line is a sale line (qty/line_gross not stored in product master).
type Line struct {
	LineKey        string `json:"line_key,omitempty"`
	ItemCode       string `json:"item_code"`
	Name           string `json:"name"`
	Qty            string `json:"qty"`
	UnitPriceGross string `json:"unit_price_gross"`
	LineGross      string `json:"line_gross"`
	VATRate        string `json:"vat_rate"`
}

// SplitPart is optional business split draft.
type SplitPart struct {
	ScopeID    string `json:"scope_id"`
	Name       string `json:"name"`
	Lines      []Line `json:"lines"`
	GrossTotal string `json:"gross_total"`
}

// CloudJob is one pending-bill-syncs row.
type CloudJob struct {
	ID      string          `json:"id"`
	Status  string          `json:"status"`
	Payload json.RawMessage `json:"payload"`
}

// ValidateVATPercent is kept for call sites that only need an error; canonical path is vatpercent.Normalize.
func ValidateVATPercent(raw string) error {
	_, err := vatpercent.Normalize(raw)
	if err != nil {
		return ingestErr(CodeInvalidVATRate, err.Error())
	}
	return nil
}

// NormalizeVATPercent is the billsync wrapper around vatpercent.Normalize (typed ingest error).
func NormalizeVATPercent(raw string) (string, error) {
	n, err := vatpercent.Normalize(raw)
	if err != nil {
		return "", ingestErr(CodeInvalidVATRate, err.Error())
	}
	return n, nil
}

// CollectLines returns all lines from whole_table or splits.
func CollectLines(snap Snapshot) ([]Line, error) {
	switch snap.ScopeType {
	case "whole_table":
		if len(snap.Splits) > 0 {
			return nil, ingestErr(CodeValidationFailed, "whole_table must not include splits")
		}
		return snap.Lines, nil
	case "split":
		if len(snap.Lines) > 0 {
			return nil, ingestErr(CodeValidationFailed, "split must not include top-level lines")
		}
		var out []Line
		for _, sp := range snap.Splits {
			out = append(out, sp.Lines...)
		}
		return out, nil
	default:
		return nil, ingestErr(CodeValidationFailed, fmt.Sprintf("scope_type %q invalid", snap.ScopeType))
	}
}

// ValidateAndDedupeProducts checks item_code / vat_rate and same-code field conflicts.
// Returns one ProductUpsertInput per distinct item_code (first occurrence wins when identical).
func ValidateAndDedupeProducts(lines []Line) ([]store.ProductUpsertInput, error) {
	type seen struct {
		name, price, vat string
	}
	m := map[string]seen{}
	var order []string
	for i, ln := range lines {
		code := strings.TrimSpace(ln.ItemCode)
		if code == "" {
			return nil, ingestErr(CodeEmptyItemCode, fmt.Sprintf("lines[%d]: item_code required", i))
		}
		norm, err := NormalizeVATPercent(ln.VATRate)
		if err != nil {
			return nil, err
		}
		name := strings.TrimSpace(ln.Name)
		price := strings.TrimSpace(ln.UnitPriceGross)
		vat := norm
		if prev, ok := m[code]; ok {
			if prev.name != name || prev.price != price || prev.vat != vat {
				return nil, ingestErr(CodeItemCodeConflict, fmt.Sprintf("item_code %q conflict on name/price/vat_rate", code))
			}
			continue
		}
		m[code] = seen{name: name, price: price, vat: vat}
		order = append(order, code)
	}
	var out []store.ProductUpsertInput
	for _, code := range order {
		s := m[code]
		out = append(out, store.ProductUpsertInput{
			ProductCode:    code,
			DisplayName:    s.name,
			SaftName:       s.name,
			UnitPriceGross: s.price,
			VATRate:        s.vat,
			TaxCode:        store.TaxCodeFromVATPercent(s.vat),
		})
	}
	return out, nil
}

// IngestCloudJob is the ONLY path that persists a Farvoo bill sync job into SQLite + products.
// Caller must ack succeeded only after this returns nil.
// already_invoiced is gated ONLY by store.HasSignedFTForSale (tax DB), never draft status.
func IngestCloudJob(db *store.DB, job CloudJob) (*store.BillSyncDraft, error) {
	if db == nil {
		return nil, ingestErr(CodePersistFailed, "db nil")
	}
	var snap Snapshot
	if err := json.Unmarshal(job.Payload, &snap); err != nil {
		return nil, ingestErr(CodeValidationFailed, "payload json: "+err.Error())
	}
	if strings.TrimSpace(snap.SourceSystem) != "" && snap.SourceSystem != "farvoo" {
		return nil, ingestErr(CodeValidationFailed, "source_system must be farvoo")
	}
	snap.SourceSystem = "farvoo"
	if strings.TrimSpace(snap.RequestID) == "" || strings.TrimSpace(snap.SourceSaleID) == "" {
		return nil, ingestErr(CodeValidationFailed, "request_id and source_sale_id required")
	}

	hasFT, err := db.HasSignedFTForSale("", snap.SourceSystem, snap.SourceSaleID)
	if err != nil {
		return nil, ingestErr(CodePersistFailed, err.Error())
	}
	if hasFT {
		return nil, ingestErr(CodeAlreadyInvoiced, "bill already invoiced; use Agent reprint/NC")
	}

	lines, err := CollectLines(snap)
	if err != nil {
		return nil, err
	}
	if len(lines) == 0 {
		return nil, ingestErr(CodeValidationFailed, "no lines")
	}
	if err := FreezeSourceLines(&snap); err != nil {
		return nil, err
	}
	products, err := ValidateAndDedupeProducts(lines)
	if err != nil {
		return nil, err
	}

	var allocJSON string
	var allocRev int64
	if strings.TrimSpace(snap.ScopeType) == "split" {
		alloc, err := AllocationFromSplits(snap)
		if err != nil {
			return nil, err
		}
		NormalizeAllocation(&alloc)
		if err := ValidateAllocation(snap, alloc); err != nil {
			return nil, err
		}
		rawAlloc, err := json.Marshal(alloc)
		if err != nil {
			return nil, ingestErr(CodePersistFailed, err.Error())
		}
		allocJSON = string(rawAlloc)
		allocRev = 1
	} else {
		allocJSON = "{}"
		allocRev = 0
	}

	draft, err := db.UpsertBillDraftOpen(snap.RequestID, snap.SourceSaleID, job.ID, snap, allocJSON, allocRev)
	if err != nil {
		return nil, ingestErr(CodePersistFailed, err.Error())
	}

	for _, p := range products {
		if _, _, err := db.UpsertFiscalProductByCode(p); err != nil {
			return nil, ingestErr(CodePersistFailed, "product upsert: "+err.Error())
		}
	}
	return draft, nil
}

// AsIngestError unwraps IngestError.
func AsIngestError(err error) *IngestError {
	var ie *IngestError
	if errors.As(err, &ie) {
		return ie
	}
	return nil
}
