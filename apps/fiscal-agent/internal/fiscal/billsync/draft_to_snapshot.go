package billsync

import (
	"fmt"
	"strconv"
	"strings"

	"farvoo-fiscal-agent/internal/fiscal/domain"

	"github.com/google/uuid"
)

const (
	defaultCustomerTaxID = "999999990"
	defaultCustomerName  = "Consumidor Final"
)

// DraftToSaleSnapshot maps a whole_table sync Snapshot → SaleSnapshot.
// ONLY whole_table path in the Draft*ToSaleSnapshot family (see DraftPartToSaleSnapshot).
func DraftToSaleSnapshot(snap Snapshot) (domain.SaleSnapshot, error) {
	if strings.TrimSpace(snap.ScopeType) != "whole_table" {
		return domain.SaleSnapshot{}, ingestErr(CodeValidationFailed, fmt.Sprintf("DraftToSaleSnapshot requires whole_table (got %q)", snap.ScopeType))
	}
	saleID := strings.TrimSpace(snap.SourceSaleID)
	if saleID == "" {
		return domain.SaleSnapshot{}, ingestErr(CodeValidationFailed, "source_sale_id required")
	}
	lines, err := CollectLines(snap)
	if err != nil {
		return domain.SaleSnapshot{}, err
	}
	outLines, gross, err := mapDraftLines(lines)
	if err != nil {
		return domain.SaleSnapshot{}, err
	}
	total := strings.TrimSpace(snap.GrossTotal)
	if total == "" {
		total = strconv.FormatFloat(gross, 'f', 2, 64)
	}
	return buildSaleSnapshot(saleID, "whole_table", saleID, outLines, total, tableMeta(snap)), nil
}

// DraftPartToSaleSnapshot maps one splits[] part of a split Snapshot → person SaleSnapshot.
// ONLY person path in the Draft*ToSaleSnapshot family.
func DraftPartToSaleSnapshot(snap Snapshot, scopeID string) (domain.SaleSnapshot, error) {
	if strings.TrimSpace(snap.ScopeType) != "split" {
		return domain.SaleSnapshot{}, ingestErr(CodeValidationFailed, fmt.Sprintf("DraftPartToSaleSnapshot requires split (got %q)", snap.ScopeType))
	}
	saleID := strings.TrimSpace(snap.SourceSaleID)
	if saleID == "" {
		return domain.SaleSnapshot{}, ingestErr(CodeValidationFailed, "source_sale_id required")
	}
	scopeID = strings.TrimSpace(scopeID)
	if scopeID == "" {
		return domain.SaleSnapshot{}, ingestErr(CodeValidationFailed, "scope_id required for person issue")
	}
	if _, err := uuid.Parse(scopeID); err != nil {
		return domain.SaleSnapshot{}, ingestErr(CodeValidationFailed, fmt.Sprintf("scope_id must be UUID (got %q)", scopeID))
	}
	var part *SplitPart
	for i := range snap.Splits {
		if strings.TrimSpace(snap.Splits[i].ScopeID) == scopeID {
			part = &snap.Splits[i]
			break
		}
	}
	if part == nil {
		return domain.SaleSnapshot{}, ingestErr(CodeValidationFailed, fmt.Sprintf("scope_id %q not in splits", scopeID))
	}
	outLines, gross, err := mapDraftLines(part.Lines)
	if err != nil {
		return domain.SaleSnapshot{}, err
	}
	total := strings.TrimSpace(part.GrossTotal)
	if total == "" {
		total = strconv.FormatFloat(gross, 'f', 2, 64)
	}
	meta := tableMeta(snap)
	if n := strings.TrimSpace(part.Name); n != "" {
		meta["split_name"] = n
	}
	return buildSaleSnapshot(saleID, "person", scopeID, outLines, total, meta), nil
}

// ApplyCustomerOverride sets buyer on sale. Empty nif → Consumidor Final.
// ONLY customer override for draft→issue path.
func ApplyCustomerOverride(sale *domain.SaleSnapshot, nif, name string) error {
	if sale == nil {
		return ingestErr(CodeValidationFailed, "sale required")
	}
	nif = strings.TrimSpace(nif)
	name = strings.TrimSpace(name)
	if nif == "" {
		sale.Customer = domain.CustomerInput{
			TaxID:       defaultCustomerTaxID,
			CompanyName: defaultCustomerName,
			Country:     "PT",
		}
		return nil
	}
	if !validPTNIF(nif) {
		return ingestErr(CodeValidationFailed, fmt.Sprintf("customer_nif %q invalid (need 9 digits)", nif))
	}
	if name == "" {
		name = nif
	}
	sale.Customer = domain.CustomerInput{
		TaxID:       nif,
		CompanyName: name,
		Country:     "PT",
	}
	return nil
}

func validPTNIF(nif string) bool {
	if len(nif) != 9 {
		return false
	}
	for _, c := range nif {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func tableMeta(snap Snapshot) map[string]string {
	meta := map[string]string{}
	if t := strings.TrimSpace(snap.TableDisplayName); t != "" {
		meta["table_display_name"] = t
	}
	return meta
}

func buildSaleSnapshot(saleID, scopeType, scopeID string, lines []domain.SaleLine, total string, meta map[string]string) domain.SaleSnapshot {
	if meta == nil {
		meta = map[string]string{}
	}
	return domain.SaleSnapshot{
		SourceSystem:  "farvoo",
		SourceSaleID:  saleID,
		ScopeType:     scopeType,
		ScopeID:       scopeID,
		FiscalPurpose: "sale",
		Lines:         lines,
		Customer: domain.CustomerInput{
			TaxID:       defaultCustomerTaxID,
			CompanyName: defaultCustomerName,
			Country:     "PT",
		},
		Payments: []domain.PaymentInput{
			{Method: "CASH", Amount: total},
		},
		DisplayMeta: meta,
	}
}

func mapDraftLines(lines []Line) ([]domain.SaleLine, float64, error) {
	if len(lines) == 0 {
		return nil, 0, ingestErr(CodeValidationFailed, "no lines")
	}
	outLines := make([]domain.SaleLine, 0, len(lines))
	var gross float64
	for i, ln := range lines {
		code := strings.TrimSpace(ln.ItemCode)
		if code == "" {
			return nil, 0, ingestErr(CodeEmptyItemCode, fmt.Sprintf("lines[%d]: item_code required", i))
		}
		if err := ValidateVATPercent(ln.VATRate); err != nil {
			return nil, 0, err
		}
		dec, err := PercentVATToDecimal(ln.VATRate)
		if err != nil {
			return nil, 0, err
		}
		qty := strings.TrimSpace(ln.Qty)
		if qty == "" {
			qty = "1"
		}
		name := strings.TrimSpace(ln.Name)
		if name == "" {
			name = code
		}
		price := strings.TrimSpace(ln.UnitPriceGross)
		if price == "" {
			return nil, 0, ingestErr(CodeValidationFailed, fmt.Sprintf("lines[%d]: unit_price_gross required", i))
		}
		outLines = append(outLines, domain.SaleLine{
			ProductCode:    code,
			DisplayName:    name,
			SaftName:       name,
			Quantity:       qty,
			UnitPriceGross: price,
			VATRate:        dec,
			ProductType:    "P",
			UnitOfMeasure:  "UN",
		})
		lg := strings.TrimSpace(ln.LineGross)
		if lg != "" {
			if v, e := strconv.ParseFloat(lg, 64); e == nil {
				gross += v
				continue
			}
		}
		q, _ := strconv.ParseFloat(qty, 64)
		p, _ := strconv.ParseFloat(price, 64)
		gross += q * p
	}
	return outLines, gross, nil
}

// PercentVATToDecimal converts "23.00" → "0.23" for SaleSnapshot / IssueFT.
func PercentVATToDecimal(percent string) (string, error) {
	if err := ValidateVATPercent(percent); err != nil {
		return "", err
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(percent), 64)
	if err != nil {
		return "", ingestErr(CodeInvalidVATRate, err.Error())
	}
	return strconv.FormatFloat(f/100.0, 'f', 2, 64), nil
}
