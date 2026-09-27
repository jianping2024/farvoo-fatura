package domain

import (
	"github.com/shopspring/decimal"

	"farvoo-fiscal-agent/internal/fiscal/compliance"
)

// FSGrossLimitEUR is the tax-inclusive gross ceiling for fatura simplificada.
// 100.00 stays FS; any greater amount is signed as FT (CIVA art. 40 services).
const FSGrossLimitEUR = "100.00"

// ApplyFSGrossLimit promotes an FS request to FT when this document's
// tax-inclusive gross (same line rounding as store.buildLines) is greater
// than FSGrossLimitEUR. FT and FR are unchanged. Signed documents are not rewritten.
func ApplyFSGrossLimit(docType DocumentType, lines []SaleLine) (DocumentType, error) {
	if docType != DocumentFS {
		return docType, nil
	}
	gross, err := saleGrossTotal(lines)
	if err != nil {
		return "", err
	}
	limit, err := compliance.ParseDecimal(FSGrossLimitEUR)
	if err != nil {
		return "", err
	}
	if gross.GreaterThan(limit) {
		return DocumentFT, nil
	}
	return DocumentFS, nil
}

func saleGrossTotal(lines []SaleLine) (decimal.Decimal, error) {
	var tot decimal.Decimal
	for _, sl := range lines {
		lg, _, _, _, err := compliance.LineFromGross(sl.Quantity, sl.UnitPriceGross, sl.VATRate)
		if err != nil {
			return decimal.Zero, err
		}
		g, err := compliance.ParseDecimal(lg)
		if err != nil {
			return decimal.Zero, err
		}
		tot = tot.Add(g)
	}
	return tot.Round(2), nil
}
