package compliance

import (
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// TaxBucket is one VAT rate group for QR I-fields.
type TaxBucket struct {
	Rate   string // "0.06" / "0.13" / "0.23" / "0.00"
	TaxBase string
	TaxAmount string
}

// QRInput holds values for the AT QR string (v0.17 MVP mapping).
type QRInput struct {
	IssuerNIF              string
	CustomerTaxID          string
	CustomerCountry        string
	DocumentType           string // FT
	DocumentStatus         string // N
	InvoiceDate            time.Time
	InvoiceNo              string
	ATCUD                  string
	Buckets                []TaxBucket
	TaxPayable             string
	GrossTotal             string
	HashBase64             string
	SoftwareCertificateNum string
}

// BuildQR is the ONLY QR content builder. Fields joined with '*', key:value.
func BuildQR(in QRInput) (string, error) {
	qChars, err := QRHashChars(in.HashBase64)
	if err != nil {
		return "", err
	}
	dateF := in.InvoiceDate.Format("20060102")
	parts := []string{
		"A:" + in.IssuerNIF,
		"B:" + in.CustomerTaxID,
		"C:" + in.CustomerCountry,
		"D:" + in.DocumentType,
		"E:" + in.DocumentStatus,
		"F:" + dateF,
		"G:" + in.InvoiceNo,
		"H:" + in.ATCUD,
	}
	parts = append(parts, mainlandIFields(in.Buckets)...)
	parts = append(parts,
		"N:"+in.TaxPayable,
		"O:"+in.GrossTotal,
		"Q:"+qChars,
		"R:"+in.SoftwareCertificateNum,
	)
	return strings.Join(parts, "*"), nil
}

func mainlandIFields(buckets []TaxBucket) []string {
	type slot struct {
		baseKey, taxKey string
		base, tax       decimal.Decimal
	}
	agg := map[string]*slot{
		"0.00": {baseKey: "I1", taxKey: "I2"},
		"0.06": {baseKey: "I3", taxKey: "I4"},
		"0.13": {baseKey: "I5", taxKey: "I6"},
		"0.23": {baseKey: "I7", taxKey: "I8"},
	}
	for _, b := range buckets {
		rate := normalizeRate(b.Rate)
		s, ok := agg[rate]
		if !ok {
			continue
		}
		base, _ := ParseDecimal(b.TaxBase)
		tax, _ := ParseDecimal(b.TaxAmount)
		if s.base.IsZero() && s.tax.IsZero() {
			s.base = base
			s.tax = tax
		} else {
			s.base = s.base.Add(base)
			s.tax = s.tax.Add(tax)
		}
	}
	order := []string{"0.00", "0.06", "0.13", "0.23"}
	var out []string
	for _, r := range order {
		s := agg[r]
		if s.base.IsZero() && s.tax.IsZero() {
			continue
		}
		out = append(out, fmt.Sprintf("%s:%s", s.baseKey, Money2(s.base)))
		out = append(out, fmt.Sprintf("%s:%s", s.taxKey, Money2(s.tax)))
	}
	return out
}

func normalizeRate(r string) string {
	d, err := ParseDecimal(r)
	if err != nil {
		return r
	}
	return d.StringFixed(2)
}
