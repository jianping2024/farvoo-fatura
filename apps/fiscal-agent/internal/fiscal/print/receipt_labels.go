package print

import "farvoo-fiscal-agent/internal/fiscal/locale"

// ReceiptLabels is the ONLY fiscal ticket chrome copy (scheme A: en | pt).
// Certification line stays Portuguese in BuildPayload — not in this struct.
type ReceiptLabels struct {
	FaturaNoPrefix   string
	ClientePrefix    string
	NIFClientePrefix string
	OriginalDocPrefix string
	ReasonPrefix     string
	MesaPrefix       string
	ViaOriginal      string
	ViaReprint       string
	HeaderQty        string
	HeaderPrice      string
	HeaderDesc       string
	Sum              string
	Net              string
	VAT              string // column / money label "IVA" (same both langs)
	Total            string
	VATSummaryTitle  string
	ColRate          string
	ColBase          string
	ColVAT           string
	ColTot           string
	PayCash          string
	PayCard          string
	PayMBWay         string
	PayMultibanco    string
	PayMixed         string
	PayOther         string
	PayFallback      string
}

// receiptLabels is the ONLY constructor for fiscal ticket chrome labels.
func receiptLabels(invoiceLocale string) ReceiptLabels {
	switch locale.NormalizeInvoiceLocale(invoiceLocale) {
	case "en":
		return ReceiptLabels{
			FaturaNoPrefix:    "Invoice No.: ",
			ClientePrefix:     "Customer: ",
			NIFClientePrefix:  "Customer NIF: ",
			OriginalDocPrefix: "Original doc: ",
			ReasonPrefix:      "Reason: ",
			MesaPrefix:        "TABLE: ",
			ViaOriginal:       "1st Copy - Original",
			ViaReprint:        "2nd Copy - Reprint",
			HeaderQty:         "Qty",
			HeaderPrice:       "Price",
			HeaderDesc:        "VAT%-Desc",
			Sum:               "Sum",
			Net:               "Net",
			VAT:               "IVA",
			Total:             "TOTAL",
			VATSummaryTitle:   "VAT summary",
			ColRate:           "Rate",
			ColBase:           "Base",
			ColVAT:            "IVA",
			ColTot:            "Tot",
			PayCash:           "Cash",
			PayCard:           "Card",
			PayMBWay:          "MB Way",
			PayMultibanco:     "Multibanco",
			PayMixed:          "Mixed",
			PayOther:          "Other",
			PayFallback:       "Payment",
		}
	default:
		return ReceiptLabels{
			FaturaNoPrefix:    "Fatura No.: ",
			ClientePrefix:     "Cliente: ",
			NIFClientePrefix:  "NIF Cliente: ",
			OriginalDocPrefix: "Documento original: ",
			ReasonPrefix:      "Motivo: ",
			MesaPrefix:        "MESA: ",
			ViaOriginal:       "1a Via - Original",
			ViaReprint:        "2a Via - Reprint",
			HeaderQty:         "Qtd",
			HeaderPrice:       "Preco",
			HeaderDesc:        "IVA%-Desc",
			Sum:               "Soma",
			Net:               "Liquido",
			VAT:               "IVA",
			Total:             "TOTAL",
			VATSummaryTitle:   "Resumo IVA",
			ColRate:           "Taxa",
			ColBase:           "Base",
			ColVAT:            "IVA",
			ColTot:            "Tot",
			PayCash:           "Numerario",
			PayCard:           "Cartao",
			PayMBWay:          "MB Way",
			PayMultibanco:     "Multibanco",
			PayMixed:          "Misto",
			PayOther:          "Outro",
			PayFallback:       "Pagamento",
		}
	}
}
