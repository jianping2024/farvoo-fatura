package print

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"farvoo-fiscal-agent/internal/fiscal/compliance"
	"farvoo-fiscal-agent/internal/fiscal/domain"
)

// BuildInput feeds BuildPayload — the ONLY print snapshot builder at issue time.
type BuildInput struct {
	DocumentID                string
	DocumentType              string
	PrintPurpose              string
	InvoiceNo                 string
	IssuedAt                  string
	TableDisplayName          string
	LegalName                 string
	BusinessName              string
	TaxRegistrationNumber     string
	Address                   string
	SoftwareCertificateNumber string
	CustomerTaxID             string
	CustomerName              string
	CustomerCountry           string
	Lines                     []LineAmounts
	Buckets                   []compliance.TaxBucket
	NetTotal                  string
	TaxPayable                string
	GrossTotal                string
	Payments                  []domain.PaymentInput
	ATCUD                     string
	QRContent                 string
	Hash                      string
	HashControl               int
}

// LineAmounts is frozen line money for the print payload.
type LineAmounts struct {
	LineNumber         int
	ProductCode        string
	ProductDescription string
	DisplayName        string
	Quantity           string
	UnitOfMeasure      string
	UnitPriceGross     string
	UnitPriceNet       string
	LineGross          string
	LineNet            string
	LineTax            string
	VATRate            string
	TaxCode            string
	ProductType        string
}

// BuildPayload is the ONLY constructor for frozen fiscal print payloads.
func BuildPayload(in BuildInput) (*Payload, string, error) {
	qChars, err := compliance.QRHashChars(in.Hash)
	if err != nil {
		return nil, "", err
	}
	var lines []LineBlock
	for _, ln := range in.Lines {
		lines = append(lines, LineBlock{
			Description:    ln.ProductDescription,
			DisplayName:    ln.DisplayName,
			Quantity:       ln.Quantity,
			UnitPriceGross: ln.UnitPriceGross,
			VATRate:        ln.VATRate,
			LineGross:      ln.LineGross,
			LineNet:        ln.LineNet,
			LineTax:        ln.LineTax,
		})
	}
	var taxRows []TaxSummaryRow
	for _, b := range in.Buckets {
		base, _ := compliance.ParseDecimal(b.TaxBase)
		tax, _ := compliance.ParseDecimal(b.TaxAmount)
		gross := base.Add(tax)
		taxRows = append(taxRows, TaxSummaryRow{
			VATRate:   b.Rate,
			TaxBase:   compliance.Money2(base),
			TaxAmount: compliance.Money2(tax),
			Gross:     compliance.Money2(gross),
		})
	}
	var pays []PaymentBlock
	for _, p := range in.Payments {
		pays = append(pays, PaymentBlock{Method: p.Method, Amount: p.Amount})
	}
	if len(pays) == 0 {
		pays = []PaymentBlock{{Method: "CASH", Amount: in.GrossTotal}}
	}
	p := &Payload{
		Version:      PayloadVersion,
		DocumentID:   in.DocumentID,
		DocumentType: in.DocumentType,
		PrintPurpose: in.PrintPurpose,
		InvoiceNo:    in.InvoiceNo,
		IssuedAt:     in.IssuedAt,
		TableDisplayName: strings.TrimSpace(in.TableDisplayName),
		Merchant: MerchantBlock{
			LegalName:                 in.LegalName,
			BusinessName:              in.BusinessName,
			TaxRegistrationNumber:     in.TaxRegistrationNumber,
			Address:                   in.Address,
			SoftwareCertificateNumber: in.SoftwareCertificateNumber,
		},
		Customer: CustomerBlock{
			TaxID:       in.CustomerTaxID,
			CompanyName: in.CustomerName,
			Country:     in.CustomerCountry,
		},
		Lines:      lines,
		TaxSummary: taxRows,
		Totals: TotalsBlock{
			NetTotal:   in.NetTotal,
			TaxPayable: in.TaxPayable,
			GrossTotal: in.GrossTotal,
		},
		Payments: pays,
		Compliance: ComplianceBlock{
			ATCUD:             in.ATCUD,
			QR:                QRBlock{Content: in.QRContent},
			HashControlChars:  qChars,
			CertificationLine: fmt.Sprintf("Processado por programa certificado n. %s/AT", in.SoftwareCertificateNumber),
		},
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(raw)
	p.PayloadHash = hex.EncodeToString(sum[:])
	return p, p.PayloadHash, nil
}
