package saft

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"farvoo-fiscal-agent/internal/escposenc"
	"farvoo-fiscal-agent/internal/fiscal/compliance"
	"farvoo-fiscal-agent/internal/fiscal/domain"
	"farvoo-fiscal-agent/internal/fiscal/ptnif"
	"farvoo-fiscal-agent/internal/fiscal/store"

	"github.com/shopspring/decimal"
	"golang.org/x/text/encoding/charmap"
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
	Payments  []store.SAFTPayment         // RG receipts → Payments table
	WorkDocs  []store.SAFTWorkingDocument // PF → WorkingDocuments
	Movements []store.SAFTStockMovement   // GR/GT → MovementOfGoods
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

	customers := map[string]customerEntry{}
	products := map[string]store.SAFTLine{}
	taxes := map[taxKey]struct{}{}

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="Windows-1252"?>`)
	b.WriteString(`<AuditFile xmlns="` + auditNS + `">`)
	writeHeader(&b, in, checkText)

	b.WriteString(`<MasterFiles>`)
	collectLine := func(docNo string, ln store.SAFTLine) {
		products[ln.ProductCode] = ln
		taxes[taxKeyFromLine(ln)] = struct{}{}
		if isZeroRate(ln.VATRate) && (strings.TrimSpace(ln.TaxExemptionReason) == "" || strings.TrimSpace(ln.TaxExemptionCode) == "") {
			valErrs = append(valErrs, fmt.Sprintf("%s line %d: 0%% VAT requires TaxExemptionReason and TaxExemptionCode", docNo, ln.LineNumber))
		}
	}
	for _, mov := range in.Movements {
		c := saftCustomer(mov.Customer)
		customers[c.ID] = c
		for _, ln := range mov.Lines {
			collectLine(mov.DocumentNumber, ln)
		}
	}
	for _, inv := range in.Invoices {
		c := saftCustomer(inv.Customer)
		customers[c.ID] = c
		for _, ln := range inv.Lines {
			collectLine(inv.InvoiceNo, ln)
			n, _ := compliance.ParseDecimal(ln.LineNet)
			if lineIsDebit(inv.DocumentType) {
				totalDebit = totalDebit.Add(n)
			} else {
				totalCredit = totalCredit.Add(n)
			}
		}
		g, _ := compliance.ParseDecimal(inv.GrossTotal)
		n, _ := compliance.ParseDecimal(inv.NetTotal)
		t, _ := compliance.ParseDecimal(inv.TaxPayable)
		if lineIsDebit(inv.DocumentType) {
			totalNet = totalNet.Sub(n)
			totalTax = totalTax.Sub(t)
			totalGross = totalGross.Sub(g)
		} else {
			totalNet = totalNet.Add(n)
			totalTax = totalTax.Add(t)
			totalGross = totalGross.Add(g)
		}
	}
	for _, pay := range in.Payments {
		c := saftCustomer(pay.Customer)
		customers[c.ID] = c
	}
	for _, wd := range in.WorkDocs {
		c := saftCustomer(wd.Customer)
		customers[c.ID] = c
		for _, ln := range wd.Lines {
			collectLine(wd.DocumentNumber, ln)
		}
	}
	writeMasterFiles(&b, customers, products, taxes, checkText)
	b.WriteString(`</MasterFiles>`)

	b.WriteString(`<SourceDocuments>`)
	b.WriteString(`<SalesInvoices>`)
	fmt.Fprintf(&b, `<NumberOfEntries>%d</NumberOfEntries>`, len(in.Invoices))
	fmt.Fprintf(&b, `<TotalDebit>%s</TotalDebit>`, compliance.Money2(totalDebit))
	fmt.Fprintf(&b, `<TotalCredit>%s</TotalCredit>`, compliance.Money2(totalCredit))
	for _, inv := range in.Invoices {
		writeInvoice(&b, inv, checkText)
	}
	b.WriteString(`</SalesInvoices>`)
	// XSD order: SalesInvoices, MovementOfGoods, WorkingDocuments, Payments.
	writeMovementOfGoods(&b, in.Movements, checkText)
	writeWorkingDocuments(&b, in.WorkDocs, checkText)
	writePayments(&b, in.Payments, checkText)
	b.WriteString(`</SourceDocuments>`)
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

// lineIsDebit is the ONLY rule for which side a source-document line amount goes on
// (Portaria 302/2016 4.1.4.20.6/7): credit notes reverse revenue (DebitAmount); invoices,
// debit notes, pro-formas and movements credit it (CreditAmount).
func lineIsDebit(docType string) bool { return docType == "NC" }

// writeLineAmount writes the line amount: net of VAT, after discounts (we have none).
func writeLineAmount(b *strings.Builder, docType string, ln store.SAFTLine) {
	if lineIsDebit(docType) {
		writeElem(b, "DebitAmount", ln.LineNet)
	} else {
		writeElem(b, "CreditAmount", ln.LineNet)
	}
}

// writeLineTax writes Tax plus the exemption pair the XSD requires when TaxPercentage is 0.
func writeLineTax(b *strings.Builder, ln store.SAFTLine) {
	b.WriteString(`<Tax>`)
	writeElem(b, "TaxType", ln.TaxType)
	writeElem(b, "TaxCountryRegion", ln.TaxCountryRegion)
	writeElem(b, "TaxCode", ln.TaxCode)
	writeElem(b, "TaxPercentage", vatRatePercent(ln.VATRate))
	b.WriteString(`</Tax>`)
	if isZeroRate(ln.VATRate) {
		writeElem(b, "TaxExemptionReason", ln.TaxExemptionReason)
		writeElem(b, "TaxExemptionCode", ln.TaxExemptionCode)
	}
}

func isZeroRate(vatRate string) bool {
	d, err := compliance.ParseDecimal(vatRate)
	return err == nil && d.IsZero()
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

func writeMasterFiles(b *strings.Builder, customers map[string]customerEntry, products map[string]store.SAFTLine, taxes map[taxKey]struct{}, check func(string, string)) {
	cids := make([]string, 0, len(customers))
	for k := range customers {
		cids = append(cids, k)
	}
	sort.Strings(cids)
	for _, k := range cids {
		c := customers[k]
		check("customer", c.CompanyName)
		b.WriteString(`<Customer>`)
		writeElem(b, "CustomerID", c.ID)
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
	if len(tkeys) == 0 {
		return
	}
	b.WriteString(`<TaxTable>`)
	for _, tk := range tkeys {
		b.WriteString(`<TaxTableEntry>`)
		writeElem(b, "TaxType", tk.TaxType)
		writeElem(b, "TaxCountryRegion", tk.TaxCountryRegion)
		writeElem(b, "TaxCode", tk.TaxCode)
		writeElem(b, "Description", "IVA "+tk.TaxCode)
		writeElem(b, "TaxPercentage", vatRatePercent(tk.VATRate))
		b.WriteString(`</TaxTableEntry>`)
	}
	b.WriteString(`</TaxTable>`)
}

func writeInvoice(b *strings.Builder, inv store.SAFTInvoice, check func(string, string)) {
	check("invoice_no", inv.InvoiceNo)
	b.WriteString(`<Invoice>`)
	writeElem(b, "InvoiceNo", inv.InvoiceNo)
	writeElem(b, "ATCUD", inv.ATCUD)
	// Sales documents have no cancel path; NC/ND crediting an original changes only its
	// business status, the SAF-T InvoiceStatus stays N.
	b.WriteString(`<DocumentStatus>`)
	writeElem(b, "InvoiceStatus", "N")
	writeElem(b, "InvoiceStatusDate", inv.SystemEntryDate)
	writeElem(b, "SourceID", inv.SourceID)
	writeElem(b, "SourceBilling", "P")
	b.WriteString(`</DocumentStatus>`)
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
	writeElem(b, "CustomerID", saftCustomer(inv.Customer).ID)
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
	if ln.LineNumber == 1 {
		if on := strings.TrimSpace(inv.OrderOriginatingON); on != "" {
			b.WriteString(`<OrderReferences>`)
			writeElem(b, "OriginatingON", on)
			if od := strings.TrimSpace(inv.OrderDate); od != "" {
				writeElem(b, "OrderDate", od)
			}
			b.WriteString(`</OrderReferences>`)
		}
	}
	writeElem(b, "ProductCode", ln.ProductCode)
	writeElem(b, "ProductDescription", ln.ProductDescription)
	writeElem(b, "Quantity", ln.Quantity)
	writeElem(b, "UnitOfMeasure", ln.UnitOfMeasure)
	writeElem(b, "UnitPrice", ln.UnitPriceNet)
	writeElem(b, "TaxPointDate", inv.InvoiceDate)
	if ln.ReferenceOriginalInvoice != "" {
		b.WriteString(`<References>`)
		writeElem(b, "Reference", ln.ReferenceOriginalInvoice)
		if ln.ReferenceReason != "" {
			writeElem(b, "Reason", ln.ReferenceReason)
		}
		b.WriteString(`</References>`)
	}
	writeElem(b, "Description", ln.ProductDescription)
	writeLineAmount(b, inv.DocumentType, ln)
	writeLineTax(b, ln)
	b.WriteString(`</Line>`)
}

func writeWorkingDocuments(b *strings.Builder, docs []store.SAFTWorkingDocument, check func(string, string)) {
	if len(docs) == 0 {
		return
	}
	totalCredit := decimal.Zero
	for _, d := range docs {
		if d.WorkStatus == "A" {
			continue
		}
		for _, ln := range d.Lines {
			n, _ := compliance.ParseDecimal(ln.LineNet)
			totalCredit = totalCredit.Add(n)
		}
	}
	b.WriteString(`<WorkingDocuments>`)
	fmt.Fprintf(b, `<NumberOfEntries>%d</NumberOfEntries>`, len(docs))
	writeElem(b, "TotalDebit", "0.00")
	writeElem(b, "TotalCredit", compliance.Money2(totalCredit))
	for _, d := range docs {
		writeWorkDocument(b, d, check)
	}
	b.WriteString(`</WorkingDocuments>`)
}

func writeWorkDocument(b *strings.Builder, d store.SAFTWorkingDocument, check func(string, string)) {
	check("work_doc", d.DocumentNumber)
	b.WriteString(`<WorkDocument>`)
	writeElem(b, "DocumentNumber", d.DocumentNumber)
	writeElem(b, "ATCUD", d.ATCUD)
	b.WriteString(`<DocumentStatus>`)
	writeElem(b, "WorkStatus", d.WorkStatus)
	writeElem(b, "WorkStatusDate", d.WorkStatusDate)
	if r := strings.TrimSpace(d.StatusReason); r != "" {
		writeElem(b, "Reason", r)
	}
	writeElem(b, "SourceID", d.SourceID)
	writeElem(b, "SourceBilling", "P")
	b.WriteString(`</DocumentStatus>`)
	writeElem(b, "Hash", d.Hash)
	fmt.Fprintf(b, `<HashControl>%d</HashControl>`, d.HashControl)
	writeElem(b, "WorkDate", d.WorkDate)
	writeElem(b, "WorkType", "PF")
	writeElem(b, "SourceID", d.SourceID)
	writeElem(b, "SystemEntryDate", d.SystemEntryDate)
	writeElem(b, "CustomerID", saftCustomer(d.Customer).ID)
	for _, ln := range d.Lines {
		writeWorkLine(b, d, ln, check)
	}
	b.WriteString(`<DocumentTotals>`)
	writeElem(b, "TaxPayable", d.TaxPayable)
	writeElem(b, "NetTotal", d.NetTotal)
	writeElem(b, "GrossTotal", d.GrossTotal)
	b.WriteString(`</DocumentTotals>`)
	b.WriteString(`</WorkDocument>`)
}

func writeWorkLine(b *strings.Builder, d store.SAFTWorkingDocument, ln store.SAFTLine, check func(string, string)) {
	check("work_line", ln.ProductDescription)
	b.WriteString(`<Line>`)
	fmt.Fprintf(b, `<LineNumber>%d</LineNumber>`, ln.LineNumber)
	writeElem(b, "ProductCode", ln.ProductCode)
	writeElem(b, "ProductDescription", ln.ProductDescription)
	writeElem(b, "Quantity", ln.Quantity)
	writeElem(b, "UnitOfMeasure", ln.UnitOfMeasure)
	writeElem(b, "UnitPrice", ln.UnitPriceNet)
	writeElem(b, "TaxPointDate", d.WorkDate)
	writeElem(b, "Description", ln.ProductDescription)
	writeLineAmount(b, "PF", ln)
	writeLineTax(b, ln)
	b.WriteString(`</Line>`)
}

func writeMovementOfGoods(b *strings.Builder, moves []store.SAFTStockMovement, check func(string, string)) {
	if len(moves) == 0 {
		return
	}
	lineCount := 0
	qtyIssued := decimal.Zero
	for _, m := range moves {
		for _, ln := range m.Lines {
			lineCount++
			q, _ := compliance.ParseDecimal(ln.Quantity)
			qtyIssued = qtyIssued.Add(q)
		}
	}
	b.WriteString(`<MovementOfGoods>`)
	fmt.Fprintf(b, `<NumberOfMovementLines>%d</NumberOfMovementLines>`, lineCount)
	writeElem(b, "TotalQuantityIssued", compliance.Money2(qtyIssued))
	for _, m := range moves {
		writeStockMovement(b, m, check)
	}
	b.WriteString(`</MovementOfGoods>`)
}

func writeStockMovement(b *strings.Builder, m store.SAFTStockMovement, check func(string, string)) {
	check("stock_mov", m.DocumentNumber)
	b.WriteString(`<StockMovement>`)
	writeElem(b, "DocumentNumber", m.DocumentNumber)
	writeElem(b, "ATCUD", m.ATCUD)
	b.WriteString(`<DocumentStatus>`)
	writeElem(b, "MovementStatus", m.MovementStatus)
	writeElem(b, "MovementStatusDate", m.MovementStatusDate)
	if r := strings.TrimSpace(m.StatusReason); r != "" {
		writeElem(b, "Reason", r)
	}
	writeElem(b, "SourceID", m.SourceID)
	writeElem(b, "SourceBilling", "P")
	b.WriteString(`</DocumentStatus>`)
	writeElem(b, "Hash", m.Hash)
	fmt.Fprintf(b, `<HashControl>%d</HashControl>`, m.HashControl)
	writeElem(b, "MovementDate", m.MovementDate)
	mt := strings.TrimSpace(m.MovementType)
	if mt == "" {
		mt = "GR"
	}
	writeElem(b, "MovementType", mt)
	writeElem(b, "SystemEntryDate", m.SystemEntryDate)
	writeElem(b, "CustomerID", saftCustomer(m.Customer).ID)
	writeElem(b, "SourceID", m.SourceID)
	// XSD order: … SourceID, ShipTo, ShipFrom, MovementStartTime, ATDocCodeID, Line, DocumentTotals.
	writeShippingPoint(b, "ShipTo", m.ShipTo)
	writeShippingPoint(b, "ShipFrom", m.ShipFrom)
	if t := strings.TrimSpace(m.MovementStartAt); t != "" {
		writeElem(b, "MovementStartTime", t)
	}
	if c := strings.TrimSpace(m.ATDocCodeID); c != "" {
		writeElem(b, "ATDocCodeID", c)
	}
	for _, ln := range m.Lines {
		writeMovementLine(b, m, ln, check)
	}
	b.WriteString(`<DocumentTotals>`)
	writeElem(b, "TaxPayable", m.TaxPayable)
	writeElem(b, "NetTotal", m.NetTotal)
	writeElem(b, "GrossTotal", m.GrossTotal)
	b.WriteString(`</DocumentTotals>`)
	b.WriteString(`</StockMovement>`)
}

// writeShippingPoint writes ShippingPointStructure with its AddressStructure (AddressDetail,
// City, PostalCode, Country all mandatory). Pre-017 rows carry only free text → "Desconhecido".
func writeShippingPoint(b *strings.Builder, tag string, a domain.Address) {
	if a.IsZero() {
		return
	}
	n := a.Normalize()
	orUnknown := func(v string) string {
		if v == "" {
			return unknown
		}
		return v
	}
	b.WriteString(`<` + tag + `><Address>`)
	writeElem(b, "AddressDetail", orUnknown(n.Detail))
	writeElem(b, "City", orUnknown(n.City))
	writeElem(b, "PostalCode", orUnknown(n.PostalCode))
	writeElem(b, "Country", n.Country)
	b.WriteString(`</Address></` + tag + `>`)
}

func writeMovementLine(b *strings.Builder, m store.SAFTStockMovement, ln store.SAFTLine, check func(string, string)) {
	check("mov_line", ln.ProductDescription)
	b.WriteString(`<Line>`)
	fmt.Fprintf(b, `<LineNumber>%d</LineNumber>`, ln.LineNumber)
	writeElem(b, "ProductCode", ln.ProductCode)
	writeElem(b, "ProductDescription", ln.ProductDescription)
	writeElem(b, "Quantity", ln.Quantity)
	writeElem(b, "UnitOfMeasure", ln.UnitOfMeasure)
	writeElem(b, "UnitPrice", ln.UnitPriceNet)
	writeElem(b, "Description", ln.ProductDescription)
	writeLineAmount(b, m.MovementType, ln)
	writeLineTax(b, ln)
	b.WriteString(`</Line>`)
}

func writePayments(b *strings.Builder, payments []store.SAFTPayment, check func(string, string)) {
	if len(payments) == 0 {
		return
	}
	totalCredit := decimal.Zero
	for _, p := range payments {
		g, _ := compliance.ParseDecimal(p.GrossTotal)
		totalCredit = totalCredit.Add(g)
	}
	b.WriteString(`<Payments>`)
	fmt.Fprintf(b, `<NumberOfEntries>%d</NumberOfEntries>`, len(payments))
	writeElem(b, "TotalDebit", "0.00")
	writeElem(b, "TotalCredit", compliance.Money2(totalCredit))
	for _, p := range payments {
		writePayment(b, p, check)
	}
	b.WriteString(`</Payments>`)
}

func writePayment(b *strings.Builder, p store.SAFTPayment, check func(string, string)) {
	check("payment_ref", p.PaymentRefNo)
	b.WriteString(`<Payment>`)
	writeElem(b, "PaymentRefNo", p.PaymentRefNo)
	writeElem(b, "ATCUD", p.ATCUD)
	writeElem(b, "TransactionDate", p.TransactionDate)
	writeElem(b, "PaymentType", "RG")
	b.WriteString(`<DocumentStatus>`)
	writeElem(b, "PaymentStatus", "N")
	writeElem(b, "PaymentStatusDate", p.SystemEntryDate)
	writeElem(b, "SourceID", p.SourceID)
	writeElem(b, "SourcePayment", "P")
	b.WriteString(`</DocumentStatus>`)
	b.WriteString(`<PaymentMethod>`)
	writeElem(b, "PaymentMechanism", PaymentMechanism(p.PaymentMethod))
	writeElem(b, "PaymentAmount", p.GrossTotal)
	writeElem(b, "PaymentDate", p.TransactionDate)
	b.WriteString(`</PaymentMethod>`)
	writeElem(b, "SourceID", p.SourceID)
	writeElem(b, "SystemEntryDate", p.SystemEntryDate)
	writeElem(b, "CustomerID", saftCustomer(p.Customer).ID)
	b.WriteString(`<Line>`)
	fmt.Fprintf(b, `<LineNumber>%d</LineNumber>`, 1)
	b.WriteString(`<SourceDocumentID>`)
	writeElem(b, "OriginatingON", p.OriginalInvoiceNo)
	writeElem(b, "InvoiceDate", p.OriginalInvoiceDate)
	b.WriteString(`</SourceDocumentID>`)
	writeElem(b, "CreditAmount", p.GrossTotal)
	b.WriteString(`</Line>`)
	b.WriteString(`<DocumentTotals>`)
	writeElem(b, "TaxPayable", "0.00")
	writeElem(b, "NetTotal", p.GrossTotal)
	writeElem(b, "GrossTotal", p.GrossTotal)
	b.WriteString(`</DocumentTotals>`)
	b.WriteString(`</Payment>`)
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

// unknown is the SAF-T value for a required field the source does not know.
const unknown = "Desconhecido"

// customerEntry is a document customer snapshot with its SAF-T identity resolved.
type customerEntry struct {
	ID string
	store.SAFTCustomer
}

// saftCustomer is the ONLY mapping from a document's customer snapshot to SAF-T identity;
// MasterFiles and every document's CustomerID use it. CustomerTaxID is the NIF, or 999999990
// when absent. CustomerID is the NIF when known, "CF" for the anonymous final consumer, and
// otherwise a stable code from name + address, so recipients without NIF stay distinct.
func saftCustomer(c store.SAFTCustomer) customerEntry {
	nif := strings.TrimSpace(c.CustomerTaxID)
	if nif != "" && nif != ptnif.FinalConsumer {
		return customerEntry{ID: nif, SAFTCustomer: c}
	}
	c.CustomerTaxID = ptnif.FinalConsumer
	name := strings.TrimSpace(c.CompanyName)
	if name == "" || strings.EqualFold(name, ptnif.FinalConsumerName) {
		// One canonical MasterFiles entry; per-document delivery addresses live in ShipTo.
		return customerEntry{ID: "CF", SAFTCustomer: store.SAFTCustomer{
			CustomerTaxID: ptnif.FinalConsumer, CompanyName: ptnif.FinalConsumerName,
			AddressDetail: unknown, City: unknown, PostalCode: unknown, Country: "PT", AccountID: unknown,
		}}
	}
	key := strings.ToLower(strings.Join([]string{name, c.AddressDetail, c.PostalCode, c.City, c.Country}, "|"))
	sum := sha256.Sum256([]byte(key))
	return customerEntry{ID: "C" + hex.EncodeToString(sum[:])[:12], SAFTCustomer: c}
}

func vatRatePercent(vatRate string) string {
	d, err := compliance.ParseDecimal(vatRate)
	if err != nil {
		return "0.00"
	}
	return compliance.Money2(d.Mul(decimal.NewFromInt(100)))
}

func validateWindows1252(s string) error {
	raw := escposenc.Windows1252(s)
	back, err := charmap.Windows1252.NewDecoder().String(string(raw))
	if err != nil {
		return fmt.Errorf("contains non-Windows-1252 characters")
	}
	if back != s {
		return fmt.Errorf("contains non-Windows-1252 characters")
	}
	return nil
}
