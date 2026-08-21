package billsync

import (
	"fmt"
	"strconv"
	"strings"

	"farvoo-fiscal-agent/internal/fiscal/domain"
)

// DraftToSaleSnapshot is the ONLY mapper from bill-sync Snapshot → issue SaleSnapshot.
// whole_table only; defaults Consumidor Final + full CASH; scope_id = source_sale_id.
func DraftToSaleSnapshot(snap Snapshot) (domain.SaleSnapshot, error) {
	if strings.TrimSpace(snap.ScopeType) != "whole_table" {
		return domain.SaleSnapshot{}, ingestErr(CodeValidationFailed, fmt.Sprintf("issue-from-draft supports whole_table only (got %q)", snap.ScopeType))
	}
	saleID := strings.TrimSpace(snap.SourceSaleID)
	if saleID == "" {
		return domain.SaleSnapshot{}, ingestErr(CodeValidationFailed, "source_sale_id required")
	}
	lines, err := CollectLines(snap)
	if err != nil {
		return domain.SaleSnapshot{}, err
	}
	if len(lines) == 0 {
		return domain.SaleSnapshot{}, ingestErr(CodeValidationFailed, "no lines")
	}

	outLines := make([]domain.SaleLine, 0, len(lines))
	var gross float64
	for i, ln := range lines {
		code := strings.TrimSpace(ln.ItemCode)
		if code == "" {
			return domain.SaleSnapshot{}, ingestErr(CodeEmptyItemCode, fmt.Sprintf("lines[%d]: item_code required", i))
		}
		if err := ValidateVATPercent(ln.VATRate); err != nil {
			return domain.SaleSnapshot{}, err
		}
		dec, err := PercentVATToDecimal(ln.VATRate)
		if err != nil {
			return domain.SaleSnapshot{}, err
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
			return domain.SaleSnapshot{}, ingestErr(CodeValidationFailed, fmt.Sprintf("lines[%d]: unit_price_gross required", i))
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

	total := strings.TrimSpace(snap.GrossTotal)
	if total == "" {
		total = strconv.FormatFloat(gross, 'f', 2, 64)
	}

	meta := map[string]string{}
	if t := strings.TrimSpace(snap.TableDisplayName); t != "" {
		meta["table_display_name"] = t
	}

	return domain.SaleSnapshot{
		SourceSystem:  "farvoo",
		SourceSaleID:  saleID,
		ScopeType:     "whole_table",
		ScopeID:       saleID,
		FiscalPurpose: "sale",
		Lines:         outLines,
		Customer: domain.CustomerInput{
			TaxID:       "999999990",
			CompanyName: "Consumidor Final",
			Country:     "PT",
		},
		Payments: []domain.PaymentInput{
			{Method: "CASH", Amount: total},
		},
		DisplayMeta: meta,
	}, nil
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
