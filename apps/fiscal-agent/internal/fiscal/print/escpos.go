package print

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"farvoo-fiscal-agent/internal/escposenc"
	"farvoo-fiscal-agent/internal/fiscal/domain"
)

// Narrow thermal usable width (chars). Avoids orphan amount wrap on POS-80.
const receiptWidth = 32

// cutFeedDots — GS V 66 n; enough paper past knife so last line is not bisected.
const cutFeedDots byte = 80

// RenderESCPOS is the ONLY fiscal receipt ESC/POS renderer (from frozen Payload).
// Layout authority: docs/fiscal-ft-receipt-layout.zh.md
func RenderESCPOS(p *Payload) []byte {
	if p == nil {
		return []byte{0x1B, 0x40, 0x1D, 0x56, 0x42, cutFeedDots}
	}
	var b bytes.Buffer
	b.Write([]byte{0x1B, 0x40}) // init
	b.Write(escposenc.SelectCodeTable(escposenc.CodeTableWPC1252))

	align := func(n byte) { b.Write([]byte{0x1B, 0x61, n}) }
	bold := func(on bool) {
		if on {
			b.Write([]byte{0x1B, 0x45, 1})
		} else {
			b.Write([]byte{0x1B, 0x45, 0})
		}
	}
	w := func(s string) {
		b.Write(escposenc.Windows1252(s))
		b.WriteByte('\n')
	}
	rule := func() { w(strings.Repeat("-", receiptWidth)) }

	// ① merchant — centered
	align(1)
	bold(true)
	w(p.Merchant.LegalName)
	bold(false)
	if p.Merchant.BusinessName != "" && p.Merchant.BusinessName != p.Merchant.LegalName {
		w(p.Merchant.BusinessName)
	}
	if p.Merchant.Address != "" {
		w(p.Merchant.Address)
	}
	if p.Merchant.TaxRegistrationNumber != "" {
		w("NIF: PT" + p.Merchant.TaxRegistrationNumber)
	}
	align(0)

	// ② document identity
	w(formatFaturaNoLine(p.InvoiceNo))
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

	// ⑤ single-line items
	rule()
	w(moneyRow("Qtd Preco IVA%-Desc", "Soma", receiptWidth))
	for _, ln := range p.Lines {
		name := strings.TrimSpace(ln.DisplayName)
		if name == "" {
			name = strings.TrimSpace(ln.Description)
		}
		w(formatItemLine(ln.Quantity, ln.UnitPriceGross, ln.VATRate, name, ln.LineGross, receiptWidth))
	}

	// ⑦ totals + payments — TOTAL emphasized (sample: larger/bold total line)
	rule()
	w(moneyRow("Liquido", p.Totals.NetTotal, receiptWidth))
	w(moneyRow("IVA", p.Totals.TaxPayable, receiptWidth))
	bold(true)
	b.Write([]byte{0x1D, 0x21, 0x01}) // GS ! — double height only (width stays 32 cols)
	w(moneyRow("TOTAL", p.Totals.GrossTotal, receiptWidth))
	b.Write([]byte{0x1D, 0x21, 0x00})
	bold(false)
	for _, pay := range p.Payments {
		w(moneyRow(formatPaymentMethod(pay.Method), pay.Amount, receiptWidth))
	}

	// ⑧ IVA summary
	rule()
	w("Resumo IVA")
	w(padColumns([]string{"Taxa", "Base", "IVA", "Tot"}, []int{6, 9, 8, 9}))
	for _, row := range p.TaxSummary {
		w(padColumns([]string{
			formatVATPercent(row.VATRate),
			row.TaxBase,
			row.TaxAmount,
			row.Gross,
		}, []int{6, 9, 8, 9}))
	}

	// ⑥⑦⑧ foot: cert → ATCUD → QR (centered); nothing below QR but feed+cut
	align(1)
	if face := formatCertificationFace(p.Compliance.HashControlChars, p.Compliance.CertificationLine); face != "" {
		w(face)
	}
	if p.Compliance.ATCUD != "" {
		w("ATCUD: " + p.Compliance.ATCUD)
	}
	if qr := strings.TrimSpace(p.Compliance.QR.Content); qr != "" {
		writeQR(&b, qr, 6)
	}
	align(0)

	// feed-then-cut (same family as kitchen GS V 66); no business text after QR
	b.Write([]byte{'\n', '\n'})
	b.Write([]byte{0x1D, 0x56, 0x42, cutFeedDots})
	return b.Bytes()
}

// formatFaturaNoLine is the ONLY ticket label for invoice number (sample: "Fatura No.: …").
func formatFaturaNoLine(invoiceNo string) string {
	return "Fatura No.: " + strings.TrimSpace(invoiceNo)
}

// formatCertificationFace is the ONLY ticket face for cert + control chars (no separate Hash: line).
// Sample style: "XLM/-Processado por programa certificado 369/AT"
func formatCertificationFace(controlChars, certLine string) string {
	cert := strings.TrimSpace(certLine)
	cert = strings.ReplaceAll(cert, "n.º", "n.")
	cert = strings.ReplaceAll(cert, "n.°", "n.")
	chars := strings.TrimSpace(controlChars)
	if chars == "" {
		return cert
	}
	if cert == "" {
		return chars
	}
	return chars + "-" + cert
}

// formatItemLine builds one item row: "2.00x 19.95 23%-Name……39.90"
func formatItemLine(qty, unitPrice, vatRate, name, lineGross string, width int) string {
	q := strings.TrimSpace(qty)
	if q != "" && !strings.HasSuffix(strings.ToLower(q), "x") {
		q += "x"
	}
	pct := formatVATPercent(vatRate)
	vatTag := pct + "-"
	prefix := strings.TrimSpace(q + " " + strings.TrimSpace(unitPrice) + " " + vatTag)
	aw := utf8.RuneCountInString(strings.TrimSpace(lineGross))
	maxName := width - utf8.RuneCountInString(prefix) - aw - 1
	if maxName < 1 {
		return moneyRow(truncateRunes(prefix, width-aw-1), lineGross, width)
	}
	return moneyRow(prefix+truncateRunes(strings.TrimSpace(name), maxName), lineGross, width)
}

func formatViaLine(purpose string) string {
	switch strings.TrimSpace(purpose) {
	case string(domain.PrintReprint):
		return "2a Via - Reprint"
	default:
		return "1a Via - Original"
	}
}

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
	label = strings.TrimSpace(label)
	amount = strings.TrimSpace(amount)
	lw := utf8.RuneCountInString(label)
	aw := utf8.RuneCountInString(amount)
	if lw+aw+1 > width {
		keep := width - aw - 1
		if keep < 1 {
			return truncateRunes(amount, width)
		}
		label = truncateRunes(label, keep)
		lw = utf8.RuneCountInString(label)
	}
	gap := width - lw - aw
	if gap < 1 {
		gap = 1
	}
	return label + strings.Repeat(" ", gap) + amount
}

func padColumns(cols []string, widths []int) string {
	var b strings.Builder
	for i, c := range cols {
		w := 6
		if i < len(widths) {
			w = widths[i]
		}
		cell := truncateRunes(c, w)
		b.WriteString(cell)
		pad := w - utf8.RuneCountInString(cell)
		if pad > 0 && i < len(cols)-1 {
			b.WriteString(strings.Repeat(" ", pad))
		}
	}
	return truncateRunes(b.String(), receiptWidth)
}

func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	var b strings.Builder
	i := 0
	for _, r := range s {
		if i >= n-3 {
			break
		}
		b.WriteRune(r)
		i++
	}
	b.WriteString("...")
	return b.String()
}

func writeQR(b *bytes.Buffer, content string, moduleSize byte) {
	if moduleSize < 1 {
		moduleSize = 6
	}
	data := []byte(content)
	storeLen := len(data) + 3
	pL := byte(storeLen % 256)
	pH := byte(storeLen / 256)
	b.Write([]byte{0x1D, 0x28, 0x6B, 0x04, 0x00, 0x31, 0x41, 0x32, 0x00})
	b.Write([]byte{0x1D, 0x28, 0x6B, 0x03, 0x00, 0x31, 0x43, moduleSize})
	b.Write([]byte{0x1D, 0x28, 0x6B, 0x03, 0x00, 0x31, 0x45, 0x30})
	b.Write([]byte{0x1D, 0x28, 0x6B, pL, pH, 0x31, 0x50, 0x30})
	b.Write(data)
	b.Write([]byte{0x1D, 0x28, 0x6B, 0x03, 0x00, 0x31, 0x51, 0x30})
	b.WriteByte('\n')
}
