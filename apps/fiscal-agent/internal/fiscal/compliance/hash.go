package compliance

import "fmt"

// BuildSignPayload is the ONLY Hash input builder (Despacho 8632/2014).
// Fields: InvoiceDate;SystemEntryDate;InvoiceNo;GrossTotal;PreviousHash
// First document in series: PreviousHash empty → trailing semicolon.
func BuildSignPayload(invoiceDate, systemEntryDate, invoiceNo, grossTotal, previousHash string) string {
	return fmt.Sprintf("%s;%s;%s;%s;%s", invoiceDate, systemEntryDate, invoiceNo, grossTotal, previousHash)
}

// QRHashChars returns Hash runes at 1-based positions 1, 11, 21, 31 (QR field Q).
func QRHashChars(hashBase64 string) (string, error) {
	runes := []rune(hashBase64)
	need := []int{0, 10, 20, 30} // 0-based
	for _, i := range need {
		if i >= len(runes) {
			return "", fmt.Errorf("compliance: hash too short for QR Q field (len=%d)", len(runes))
		}
	}
	return string([]rune{runes[0], runes[10], runes[20], runes[30]}), nil
}
