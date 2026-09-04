package billsync

import (
	"fmt"
	"strconv"
	"strings"

	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/ptnif"
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

// DraftPartToSaleSnapshot seeds allocation from splits[] then delegates to DraftPersonFromAllocation
// (ONLY person SaleSnapshot builder). Adapter for legacy split payloads / unit tests.
func DraftPartToSaleSnapshot(snap Snapshot, scopeID string) (domain.SaleSnapshot, error) {
	if strings.TrimSpace(snap.ScopeType) != "split" && len(snap.Splits) == 0 {
		return domain.SaleSnapshot{}, ingestErr(CodeValidationFailed, fmt.Sprintf("DraftPartToSaleSnapshot requires split (got %q)", snap.ScopeType))
	}
	if err := FreezeSourceLines(&snap); err != nil {
		return domain.SaleSnapshot{}, err
	}
	alloc, err := AllocationFromSplits(snap)
	if err != nil {
		return domain.SaleSnapshot{}, err
	}
	return DraftPersonFromAllocation(snap, alloc, scopeID)
}

// ApplyCustomerOverride sets buyer on sale. Empty nif → Consumidor Final.
// ONLY customer override for draft→issue / manual FT path.
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
	norm, err := ptnif.NormalizeBuyer(nif)
	if err != nil {
		return ingestErr(CodeValidationFailed, err.Error())
	}
	if name == "" {
		name = norm
	}
	sale.Customer = domain.CustomerInput{
		TaxID:       norm,
		CompanyName: name,
		Country:     "PT",
	}
	return nil
}

// ApplyPaymentOverride sets payment method on sale (keeps Amount; empty → CASH).
// ONLY payment override for draft→issue path.
func ApplyPaymentOverride(sale *domain.SaleSnapshot, method string) error {
	if sale == nil {
		return ingestErr(CodeValidationFailed, "sale required")
	}
	pay := domain.NormalizePaymentMethod(method)
	if !domain.IsKnownPaymentMethod(pay) {
		return ingestErr(CodeValidationFailed, fmt.Sprintf("unknown payment_method %q", method))
	}
	amount := "0.00"
	if len(sale.Payments) > 0 && strings.TrimSpace(sale.Payments[0].Amount) != "" {
		amount = strings.TrimSpace(sale.Payments[0].Amount)
	}
	sale.Payments = []domain.PaymentInput{{Method: pay, Amount: amount}}
	return nil
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
		norm, err := NormalizeVATPercent(ln.VATRate)
		if err != nil {
			return nil, 0, err
		}
		dec, err := PercentVATToDecimal(norm)
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
