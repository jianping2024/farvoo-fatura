package print

import (
	"bytes"
	"fmt"
	"strings"
)

// RenderESCPOS is the ONLY fiscal receipt ESC/POS renderer (from frozen Payload).
func RenderESCPOS(p *Payload) []byte {
	var b bytes.Buffer
	b.Write([]byte{0x1B, 0x40}) // init
	w := func(s string) { b.WriteString(s); b.WriteByte('\n') }

	w(p.Merchant.LegalName)
	if p.Merchant.BusinessName != "" && p.Merchant.BusinessName != p.Merchant.LegalName {
		w(p.Merchant.BusinessName)
	}
	w(p.Merchant.Address)
	w(fmt.Sprintf("NIF: PT%s", p.Merchant.TaxRegistrationNumber))
	w("Original")
	w("")
	w(fmt.Sprintf("%s: %s", p.DocumentType, p.InvoiceNo))
	w(fmt.Sprintf("Cliente: %s", p.Customer.CompanyName))
	if p.Customer.TaxID != "" {
		w(fmt.Sprintf("NIF Cliente: %s", p.Customer.TaxID))
	}
	w(strings.Repeat("-", 42))
	for _, ln := range p.Lines {
		name := ln.DisplayName
		if name == "" {
			name = ln.Description
		}
		w(fmt.Sprintf("%s  x%s", name, ln.Quantity))
		w(fmt.Sprintf("  %s  IVA %s  %s", ln.UnitPriceGross, ln.VATRate, ln.LineGross))
	}
	w(strings.Repeat("-", 42))
	w(fmt.Sprintf("Liquido: %s", p.Totals.NetTotal))
	w(fmt.Sprintf("IVA:     %s", p.Totals.TaxPayable))
	w(fmt.Sprintf("TOTAL:   %s", p.Totals.GrossTotal))
	w("")
	w("ATCUD: " + p.Compliance.ATCUD)
	// Native QR (ESC/POS GS ( k) — store content; printers that ignore still get ATCUD text.
	qr := p.Compliance.QR.Content
	if qr != "" {
		writeQR(&b, qr)
	}
	w("")
	w(p.Compliance.CertificationLine)
	w(fmt.Sprintf("Hash: %s", p.Compliance.HashControlChars))
	b.Write([]byte{0x1D, 0x56, 0x00}) // full cut
	return b.Bytes()
}

func writeQR(b *bytes.Buffer, content string) {
	// Model 2, size 6, error correction L — standard ESC/POS QR sequence
	data := []byte(content)
	storeLen := len(data) + 3
	pL := byte(storeLen % 256)
	pH := byte(storeLen / 256)
	b.Write([]byte{0x1D, 0x28, 0x6B, 0x04, 0x00, 0x31, 0x41, 0x32, 0x00})       // model
	b.Write([]byte{0x1D, 0x28, 0x6B, 0x03, 0x00, 0x31, 0x43, 0x06})             // size
	b.Write([]byte{0x1D, 0x28, 0x6B, 0x03, 0x00, 0x31, 0x45, 0x30})             // EC
	b.Write([]byte{0x1D, 0x28, 0x6B, pL, pH, 0x31, 0x50, 0x30})                 // store
	b.Write(data)
	b.Write([]byte{0x1D, 0x28, 0x6B, 0x03, 0x00, 0x31, 0x51, 0x30}) // print
	b.WriteByte('\n')
}
