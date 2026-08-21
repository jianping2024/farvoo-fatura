package print

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"farvoo-fiscal-agent/internal/fiscal/domain"
)

const receiptWidth = 42

// RenderESCPOS is the ONLY fiscal receipt ESC/POS renderer (from frozen Payload).
// Layout authority: docs/fiscal-ft-receipt-layout.zh.md
func RenderESCPOS(p *Payload) []byte {
	if p == nil {
		return []byte{0x1B, 0x40, 0x1D, 0x56, 0x00}
	}
	var b bytes.Buffer
	b.Write([]byte{0x1B, 0x40}) // init
	w := func(s string) { b.WriteString(s); b.WriteByte('\n') }
	rule := func() { w(strings.Repeat("-", receiptWidth)) }

	// ① merchant
	w(p.Merchant.LegalName)
	if p.Merchant.BusinessName != "" && p.Merchant.BusinessName != p.Merchant.LegalName {
		w(p.Merchant.BusinessName)
	}
	if p.Merchant.Address != "" {
		w(p.Merchant.Address)
	}
	if p.Merchant.TaxRegistrationNumber != "" {
		w("NIF: PT" + p.Merchant.TaxRegistrationNumber)
	}

	// ② document identity
	w(formatInvoiceLine(p.DocumentType, p.InvoiceNo))
	if dt := formatIssuedAt(p.IssuedAt); dt != "" {
		w(dt)
	}
	w(formatViaLine(p.PrintPurpose))

	// ③ customer
	if p.Customer.CompanyName != "" {
		w("Cliente: " + p.Customer.CompanyName)
	}
	if p.Customer.TaxID != "" {
		w("NIF Cliente: " + p.Customer.TaxID)
	}

	// ⑤ lines
	rule()
	for _, ln := range p.Lines {
		name := strings.TrimSpace(ln.DisplayName)
		if name == "" {
			name = strings.TrimSpace(ln.Description)
		}
		w(truncate(name, receiptWidth))
		left := fmt.Sprintf("x%s  %s  IVA %s", ln.Quantity, ln.UnitPriceGross, formatVATPercent(ln.VATRate))
		w(moneyRow(left, ln.LineGross, receiptWidth))
	}

	// ⑦ totals + payments
	rule()
	w(moneyRow("Liquido", p.Totals.NetTotal, receiptWidth))
	w(moneyRow("IVA", p.Totals.TaxPayable, receiptWidth))
	w(moneyRow("TOTAL", p.Totals.GrossTotal, receiptWidth))
	for _, pay := range p.Payments {
		w(moneyRow(formatPaymentMethod(pay.Method), pay.Amount, receiptWidth))
	}

	// ⑧ IVA summary
	rule()
	w("Resumo IVA")
	w(padColumns([]string{"Taxa", "Base", "IVA", "Total"}, []int{8, 11, 11, 12}))
	for _, row := range p.TaxSummary {
		w(padColumns([]string{
			formatVATPercent(row.VATRate),
			row.TaxBase,
			row.TaxAmount,
			row.Gross,
		}, []int{8, 11, 11, 12}))
	}

	// ⑨–⑪ compliance foot
	w("")
	w("ATCUD: " + p.Compliance.ATCUD)
	if qr := strings.TrimSpace(p.Compliance.QR.Content); qr != "" {
		writeQR(&b, qr)
	}
	if p.Compliance.CertificationLine != "" {
		w(p.Compliance.CertificationLine)
	}
	if p.Compliance.HashControlChars != "" {
		w("Hash: " + p.Compliance.HashControlChars)
	}
	b.Write([]byte{0x1D, 0x56, 0x00}) // full cut
	return b.Bytes()
}

// formatInvoiceLine prints the formal invoice number once (no "FT: FT FT…" pile-up).
func formatInvoiceLine(docType, invoiceNo string) string {
	no := strings.TrimSpace(invoiceNo)
	dt := strings.TrimSpace(docType)
	if no == "" {
		return dt
	}
	if dt != "" {
		prefix := dt + " "
		if strings.HasPrefix(no, prefix) || strings.EqualFold(no, dt) {
			return no
		}
	}
	if dt == "" {
		return no
	}
	return dt + " " + no
}

func formatViaLine(purpose string) string {
	switch strings.TrimSpace(purpose) {
	case string(domain.PrintReprint):
		return "2ª Via — Reprint"
	default:
		return "1ª Via — Original"
	}
}

// formatIssuedAt turns payload issued_at into DD/MM/YYYY HH:MM.
func formatIssuedAt(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("02/01/2006 15:04")
		}
	}
	return s
}

// formatVATPercent maps payload decimal rate (0.23) or percent (23 / 23.00) → "23%".
func formatVATPercent(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimSuffix(s, "%")
	if s == "" {
		return "0%"
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return s + "%"
	}
	if f > 0 && f < 1 {
		f = f * 100
	}
	if f == float64(int(f)) {
		return fmt.Sprintf("%.0f%%", f)
	}
	return fmt.Sprintf("%.2f%%", f)
}

func formatPaymentMethod(method string) string {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case "CASH":
		return "Numerario"
	case "CARD":
		return "Cartao"
	case "MBWAY":
		return "MB Way"
	case "MULTIBANCO":
		return "Multibanco"
	case "MIXED":
		return "Misto"
	case "OTHER":
		return "Outro"
	default:
		if method == "" {
			return "Pagamento"
		}
		return method
	}
}

func moneyRow(label, amount string, width int) string {
	label = truncate(strings.TrimSpace(label), width-1)
	amount = strings.TrimSpace(amount)
	gap := width - len(label) - len(amount)
	if gap < 1 {
		gap = 1
		label = truncate(label, width-len(amount)-1)
		gap = width - len(label) - len(amount)
		if gap < 1 {
			return truncate(label+" "+amount, width)
		}
	}
	return label + strings.Repeat(" ", gap) + amount
}

func padColumns(cols []string, widths []int) string {
	var b strings.Builder
	for i, c := range cols {
		w := 8
		if i < len(widths) {
			w = widths[i]
		}
		cell := truncate(c, w)
		b.WriteString(cell)
		if pad := w - len(cell); pad > 0 && i < len(cols)-1 {
			b.WriteString(strings.Repeat(" ", pad))
		}
	}
	return truncate(b.String(), receiptWidth)
}

func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-3] + "..."
}

func writeQR(b *bytes.Buffer, content string) {
	data := []byte(content)
	storeLen := len(data) + 3
	pL := byte(storeLen % 256)
	pH := byte(storeLen / 256)
	b.Write([]byte{0x1D, 0x28, 0x6B, 0x04, 0x00, 0x31, 0x41, 0x32, 0x00})
	b.Write([]byte{0x1D, 0x28, 0x6B, 0x03, 0x00, 0x31, 0x43, 0x06})
	b.Write([]byte{0x1D, 0x28, 0x6B, 0x03, 0x00, 0x31, 0x45, 0x30})
	b.Write([]byte{0x1D, 0x28, 0x6B, pL, pH, 0x31, 0x50, 0x30})
	b.Write(data)
	b.Write([]byte{0x1D, 0x28, 0x6B, 0x03, 0x00, 0x31, 0x51, 0x30})
	b.WriteByte('\n')
}
