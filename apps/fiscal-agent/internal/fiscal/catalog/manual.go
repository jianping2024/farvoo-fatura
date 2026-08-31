package catalog

import (
	"fmt"
	"strconv"
	"strings"

	"farvoo-fiscal-agent/internal/fiscal/billsync"
	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/store"
	"farvoo-fiscal-agent/internal/fiscal/vatpercent"
)

// ManualLineInput is one manual FT line: catalog code and/or temp fields.
type ManualLineInput struct {
	ProductCode    string `json:"product_code"`
	DisplayName    string `json:"display_name"`
	SaftName       string `json:"saft_name"`
	UnitPriceGross string `json:"unit_price_gross"`
	VATRatePercent string `json:"vat_rate_percent"`
	Quantity       string `json:"quantity"`
}

// ManualIssueInput is the ONLY API input for manual FT/FS/FR (before snapshot build).
type ManualIssueInput struct {
	RequestID          string
	DocumentType       string
	CustomerNIF        string
	CustomerName       string
	PaymentMethod      string
	TableDisplayName   string
	Lines              []ManualLineInput
}

// BuildManualSaleSnapshot is the ONLY builder for manual FT → IssueDocument.
func BuildManualSaleSnapshot(db *store.DB, in ManualIssueInput) (domain.SaleSnapshot, error) {
	if db == nil {
		return domain.SaleSnapshot{}, fmt.Errorf("catalog: db required")
	}
	req := strings.TrimSpace(in.RequestID)
	if req == "" {
		return domain.SaleSnapshot{}, fmt.Errorf("catalog: request_id required")
	}
	if len(in.Lines) == 0 {
		return domain.SaleSnapshot{}, fmt.Errorf("catalog: lines required")
	}
	outLines := make([]domain.SaleLine, 0, len(in.Lines))
	var gross float64
	for i, ln := range in.Lines {
		code := strings.TrimSpace(ln.ProductCode)
		qty := strings.TrimSpace(ln.Quantity)
		if qty == "" {
			qty = "1"
		}
		var display, saft, price, vatPercent string
		if code != "" {
			row, err := db.GetFiscalProductByCode(code)
			if err != nil {
				return domain.SaleSnapshot{}, fmt.Errorf("catalog: lines[%d]: %w", i, err)
			}
			display = row.DisplayName
			saft = row.SaftName
			price = row.UnitPriceGross
			vatPercent = row.VATRate
			if display == "" {
				display = code
			}
		} else {
			display = strings.TrimSpace(ln.DisplayName)
			saft = strings.TrimSpace(ln.SaftName)
			price = strings.TrimSpace(ln.UnitPriceGross)
			vatPercent = strings.TrimSpace(ln.VATRatePercent)
			if display == "" || saft == "" || price == "" || vatPercent == "" {
				return domain.SaleSnapshot{}, fmt.Errorf("catalog: lines[%d]: temp line needs display_name, saft_name, unit_price_gross, vat_rate_percent", i)
			}
			code = fmt.Sprintf("TEMP-%d", i+1)
		}
		norm, err := vatpercent.Normalize(vatPercent)
		if err != nil {
			return domain.SaleSnapshot{}, fmt.Errorf("catalog: lines[%d]: %w", i, err)
		}
		dec, err := billsync.PercentVATToDecimal(norm)
		if err != nil {
			return domain.SaleSnapshot{}, fmt.Errorf("catalog: lines[%d]: %w", i, err)
		}
		outLines = append(outLines, domain.SaleLine{
			ProductCode:    code,
			DisplayName:    display,
			SaftName:       saft,
			Quantity:       qty,
			UnitPriceGross: price,
			VATRate:        dec,
			ProductType:    "P",
			UnitOfMeasure:  "UN",
		})
		q, _ := strconv.ParseFloat(qty, 64)
		p, _ := strconv.ParseFloat(price, 64)
		gross += q * p
	}
	total := strconv.FormatFloat(gross, 'f', 2, 64)
	pay := domain.NormalizePaymentMethod(in.PaymentMethod)
	if !domain.IsKnownPaymentMethod(pay) {
		return domain.SaleSnapshot{}, fmt.Errorf("catalog: unknown payment_method %q", pay)
	}
	sale := domain.SaleSnapshot{
		SourceSystem:  "manual",
		SourceSaleID:  req,
		ScopeType:     "manual",
		ScopeID:       req,
		FiscalPurpose: "sale",
		Lines:         outLines,
		Payments: []domain.PaymentInput{
			{Method: pay, Amount: total},
		},
	}
	if t := strings.TrimSpace(in.TableDisplayName); t != "" {
		sale.DisplayMeta = map[string]string{"table_display_name": t}
	}
	if err := billsync.ApplyCustomerOverride(&sale, in.CustomerNIF, in.CustomerName); err != nil {
		return domain.SaleSnapshot{}, err
	}
	return sale, nil
}
