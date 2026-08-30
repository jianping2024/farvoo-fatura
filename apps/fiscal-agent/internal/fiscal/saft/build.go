package saft

import (
	"fmt"
	"sort"
	"strings"

	"farvoo-fiscal-agent/internal/escposenc"
	"farvoo-fiscal-agent/internal/fiscal/compliance"
	"farvoo-fiscal-agent/internal/fiscal/store"

	"github.com/shopspring/decimal"
)

const auditNS = "urn:OECD:StandardAuditFile-Tax:PT_1.04_01"

// BuildInput is input for the ONLY SAF-T XML builder.
type BuildInput struct {
	Taxpayer  *store.TaxpayerSettings
	Year      int
	Month     int
	StartDate string
	EndDate   string
	Invoices  []store.SAFTInvoice
}

// BuildResult is SAF-T XML output and validation outcome.
type BuildResult struct {
	XMLBytes         []byte
	ValidationStatus string
	ValidationErrors []string
	TotalNet         string
	TotalTax         string
	TotalGross       string
	TotalCredit      string
	TotalDebit       string
}

// Build generates SAF-T(PT) 1.04_01 XML in Windows-1252 (no BOM).
func Build(in BuildInput) (*BuildResult, error) {
	if in.Taxpayer == nil {
		return nil, fmt.Errorf("saft: taxpayer required")
	}
	var valErrs []string
	checkText := func(label, s string) {
		if err := validateWindows1252(s); err != nil {
			valErrs = append(valErrs, fmt.Sprintf("%s: %v", label, err))
		}
	}

	totalNet := decimal.Zero
	totalTax := decimal.Zero
	totalGross := decimal.Zero
	totalCredit := decimal.Zero
	totalDebit := decimal.Zero

	customers := map[string]store.SAFTCustomer{}
	products := map[string]store.SAFTLine{}
	taxes := map[taxKey]struct{}{}

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="Windows-1252"?>`)
	b.WriteString(`<AuditFile xmlns="` + auditNS + `">`)
	writeHeader(&b, in, checkText)

	b.WriteString(`<MasterFiles>`)
	for _, inv := range in.Invoices {
		cid := customerKey(inv.Customer)
		customers[cid] = inv.Customer
		for _, ln := range inv.Lines {
			products[ln.ProductCode] = ln
			taxes[taxKeyFromLine(ln)] = struct{}{}
		}
		g, _ := compliance.ParseDecimal(inv.GrossTotal)
		n, _ := compliance.ParseDecimal(inv.NetTotal)
		t, _ := compliance.ParseDecimal(inv.TaxPayable)
		if inv.DocumentType == "NC" {
			totalDebit = totalDebit.Add(g)
			totalNet = totalNet.Sub(n)
			totalTax = totalTax.Sub(t)
			totalGross = totalGross.Sub(g)
		} else {
			totalCredit = totalCredit.Add(g)
			totalNet = totalNet.Add(n)
			totalTax = totalTax.Add(t)
			totalGross = totalGross.Add(g)
		}
	}
	writeMasterFiles(&b, customers, products, taxes, checkText)
	b.WriteString(`</MasterFiles>`)

	b.WriteString(`<SourceDocuments><SalesInvoices>`)
	fmt.Fprintf(&b, `<NumberOfEntries>%d</NumberOfEntries>`, len(in.Invoices))
	fmt.Fprintf(&b, `<TotalDebit>%s</TotalDebit>`, compliance.Money2(totalDebit))
	fmt.Fprintf(&b, `<TotalCredit>%s</TotalCredit>`, compliance.Money2(totalCredit))
	for _, inv := range in.Invoices {
		writeInvoice(&b, inv, checkText)
	}
	b.WriteString(`</SalesInvoices></SourceDocuments>`)
	b.WriteString(`</AuditFile>`)

	xmlUTF8 := b.String()
	xmlBytes := escposenc.Windows1252(xmlUTF8)
	status := "VALID"
	if len(valErrs) > 0 {
		status = "INVALID"
	}
	return &BuildResult{
		XMLBytes:         xmlBytes,
		ValidationStatus: status,
		ValidationErrors: valErrs,
		TotalNet:         compliance.Money2(totalNet),
		TotalTax:         compliance.Money2(totalTax),
		TotalGross:       compliance.Money2(totalGross),
		TotalCredit:      compliance.Money2(totalCredit),
		TotalDebit:       compliance.Money2(totalDebit),
	}, nil
}

type taxKey struct {
	TaxType          string
	TaxCountryRegion string
	TaxCode          string
	VATRate          string
}

func taxKeyFromLine(ln store.SAFTLine) taxKey {
	return taxKey{ln.TaxType, ln.TaxCountryRegion, ln.TaxCode, ln.VATRate}
}

func writeHeader(b *strings.Builder, in BuildInput, check func(string, string)) {
	t := in.Taxpayer
	check("legal_name", t.LegalName)
	check("address", t.AddressDetail)
	b.WriteString(`<Header>`)
	writeElem(b, "AuditFileVersion", "1.04_01")
	writeElem(b, "CompanyID", t.TaxRegistrationNumber)
	writeElem(b, "TaxRegistrationNumber", t.TaxRegistrationNumber)
	writeElem(b, "TaxAccountingBasis", "P")
	writeElem(b, "CompanyName", t.LegalName)
	if t.BusinessName != "" {
		writeElem(b, "BusinessName", t.BusinessName)
	}
	b.WriteString(`<CompanyAddress>`)
	writeElem(b, "AddressDetail", t.AddressDetail)
	writeElem(b, "City", t.City)
	writeElem(b, "PostalCode", t.PostalCode)
	writeElem(b, "Country", t.Country)
	b.WriteString(`</CompanyAddress>`)
	fmt.Fprintf(b, `<FiscalYear>%d</FiscalYear>`, in.Year)
	writeElem(b, "StartDate", in.StartDate)
	writeElem(b, "EndDate", in.EndDate)
	writeElem(b, "CurrencyCode", "EUR")
	writeElem(b, "DateCreated", in.EndDate)
	writeElem(b, "TaxEntity", "Global")
	writeElem(b, "ProductCompanyTaxID", t.TaxRegistrationNumber)
	writeElem(b, "SoftwareCertificateNumber", t.SoftwareCertificateNumber)
	writeElem(b, "ProductID", t.ProductID)
	writeElem(b, "ProductVersion", t.ProductVersion)
	b.WriteString(`</Header>`)
}

func writeMasterFiles(b *strings.Builder, customers map[string]store.SAFTCustomer, products map[string]store.SAFTLine, taxes map[taxKey]struct{}, check func(string, string)) {
	cids := make([]string, 0, len(customers))
	for k := range customers {
		cids = append(cids, k)
	}
	sort.Strings(cids)
	for _, k := range cids {
		c := customers[k]
		check("customer", c.CompanyName)
		b.WriteString(`<Customer>`)
		writeElem(b, "CustomerID", c.CustomerTaxID)
		writeElem(b, "AccountID", c.AccountID)
		writeElem(b, "CustomerTaxID", c.CustomerTaxID)
		writeElem(b, "CompanyName", c.CompanyName)
		b.WriteString(`<BillingAddress>`)
		writeElem(b, "AddressDetail", c.AddressDetail)
		writeElem(b, "City", c.City)
		writeElem(b, "PostalCode", c.PostalCode)
		writeElem(b, "Country", c.Country)
		b.WriteString(`</BillingAddress>`)
		fmt.Fprintf(b, `<SelfBillingIndicator>%d</SelfBillingIndicator>`, c.SelfBillingIndicator)
		b.WriteString(`</Customer>`)
	}
	pcodes := make([]string, 0, len(products))
	for k := range products {
		pcodes = append(pcodes, k)
	}
	sort.Strings(pcodes)
	for _, code := range pcodes {
		p := products[code]
		check("product", p.ProductDescription)
		b.WriteString(`<Product>`)
		writeElem(b, "ProductType", p.ProductType)
		writeElem(b, "ProductCode", p.ProductCode)
		writeElem(b, "ProductDescription", p.ProductDescription)
		writeElem(b, "ProductNumberCode", p.ProductCode)
		b.WriteString(`</Product>`)
	}
	tkeys := make([]taxKey, 0, len(taxes))
	for k := range taxes {
		tkeys = append(tkeys, k)
	}
	sort.Slice(tkeys, func(i, j int) bool {
		return tkeys[i].TaxCode < tkeys[j].TaxCode
	})
	for _, tk := range tkeys {
		b.WriteString(`<TaxTable>`)
		writeElem(b, "TaxType", tk.TaxType)
		writeElem(b, "TaxCountryRegion", tk.TaxCountryRegion)
		writeElem(b, "TaxCode", tk.TaxCode)
		writeElem(b, "Description", "IVA "+tk.TaxCode)
		writeElem(b, "TaxPercentage", vatRatePercent(tk.VATRate))
		b.WriteString(`</TaxTable>`)
	}
}

func writeInvoice(b *strings.Builder, inv store.SAFTInvoice, check func(string, string)) {
	check("invoice_no", inv.InvoiceNo)
	b.WriteString(`<Invoice>`)
	writeElem(b, "InvoiceNo", inv.InvoiceNo)
	writeElem(b, "ATCUD", inv.ATCUD)
	writeElem(b, "DocumentStatus", "N")
	writeElem(b, "Hash", inv.Hash)
	fmt.Fprintf(b, `<HashControl>%d</HashControl>`, inv.HashControl)
	period := inv.InvoiceDate
	if len(period) >= 7 {
		period = period[5:7]
	}
	writeElem(b, "Period", period)
	writeElem(b, "InvoiceDate", inv.InvoiceDate)
	writeElem(b, "InvoiceType", inv.DocumentType)
	b.WriteString(`<SpecialRegimes>`)
	fmt.Fprintf(b, `<SelfBillingIndicator>%d</SelfBillingIndicator>`, inv.Customer.SelfBillingIndicator)
	b.WriteString(`<CashVATSchemeIndicator>0</CashVATSchemeIndicator><ThirdPartiesBillingIndicator>0</ThirdPartiesBillingIndicator></SpecialRegimes>`)
	writeElem(b, "SourceID", inv.SourceID)
	writeElem(b, "SystemEntryDate", inv.SystemEntryDate)
	writeElem(b, "CustomerID", inv.Customer.CustomerTaxID)
	for _, ln := range inv.Lines {
		writeLine(b, inv, ln, check)
	}
	b.WriteString(`<DocumentTotals>`)
	writeElem(b, "TaxPayable", inv.TaxPayable)
	writeElem(b, "NetTotal", inv.NetTotal)
	writeElem(b, "GrossTotal", inv.GrossTotal)
	b.WriteString(`</DocumentTotals>`)
	b.WriteString(`</Invoice>`)
}

func writeLine(b *strings.Builder, inv store.SAFTInvoice, ln store.SAFTLine, check func(string, string)) {
	check("line_desc", ln.ProductDescription)
	b.WriteString(`<Line>`)
	fmt.Fprintf(b, `<LineNumber>%d</LineNumber>`, ln.LineNumber)
	writeElem(b, "ProductCode", ln.ProductCode)
	writeElem(b, "ProductDescription", ln.ProductDescription)
	writeElem(b, "Quantity", ln.Quantity)
	writeElem(b, "UnitOfMeasure", ln.UnitOfMeasure)
	writeElem(b, "UnitPrice", ln.UnitPriceNet)
	writeElem(b, "TaxPointDate", inv.InvoiceDate)
	writeElem(b, "Description", ln.ProductDescription)
	if inv.DocumentType == "NC" {
		writeElem(b, "CreditAmount", ln.LineGross)
	} else {
		writeElem(b, "DebitAmount", ln.LineGross)
	}
	b.WriteString(`<Tax>`)
	writeElem(b, "TaxType", ln.TaxType)
	writeElem(b, "TaxCountryRegion", ln.TaxCountryRegion)
	writeElem(b, "TaxCode", ln.TaxCode)
	writeElem(b, "TaxPercentage", vatRatePercent(ln.VATRate))
	b.WriteString(`</Tax>`)
	writeElem(b, "SettlementAmount", ln.LineGross)
	if ln.ReferenceOriginalInvoice != "" {
		b.WriteString(`<References>`)
		writeElem(b, "Reference", ln.ReferenceOriginalInvoice)
		if ln.ReferenceReason != "" {
			writeElem(b, "Reason", ln.ReferenceReason)
		}
		b.WriteString(`</References>`)
	}
	b.WriteString(`</Line>`)
}

func writeElem(b *strings.Builder, name, value string) {
	fmt.Fprintf(b, "<%s>%s</%s>", name, xmlEscape(value), name)
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

func customerKey(c store.SAFTCustomer) string { return c.CustomerTaxID }

func vatRatePercent(vatRate string) string {
	d, err := compliance.ParseDecimal(vatRate)
	if err != nil {
		return "0.00"
	}
	return compliance.Money2(d.Mul(decimal.NewFromInt(100)))
}

func validateWindows1252(s string) error {
	raw := escposenc.Windows1252(s)
	back := string(raw)
	if back != s {
		return fmt.Errorf("contains non-Windows-1252 characters")
	}
	return nil
}
