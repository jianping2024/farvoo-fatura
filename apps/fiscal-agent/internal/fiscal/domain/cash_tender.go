package domain

import (
	"fmt"
	"strings"

	"farvoo-fiscal-agent/internal/fiscal/compliance"
)

// ApplyCashTender is the ONLY writer for PaymentInput.Tendered and ChangeDue.
// Empty tendered clears both. Non-empty requires CASH and tendered >= Amount;
// ChangeDue is always recomputed server-side (never trusted from the client).
func ApplyCashTender(pay *PaymentInput, tendered string) error {
	if pay == nil {
		return fmt.Errorf("payment required")
	}
	tendered = strings.TrimSpace(tendered)
	if tendered == "" {
		pay.Tendered = ""
		pay.ChangeDue = ""
		return nil
	}
	if NormalizePaymentMethod(pay.Method) != PaymentCash {
		return fmt.Errorf("tendered only allowed for CASH")
	}
	amt, err := compliance.ParseDecimal(pay.Amount)
	if err != nil {
		return fmt.Errorf("payment amount: %w", err)
	}
	tend, err := compliance.ParseDecimal(tendered)
	if err != nil {
		return fmt.Errorf("tendered: %w", err)
	}
	if tend.LessThan(amt) {
		return fmt.Errorf("tendered less than amount")
	}
	pay.Tendered = compliance.Money2(tend)
	pay.ChangeDue = compliance.Money2(tend.Sub(amt))
	return nil
}

// ApplyCashTenderToSale is the ONLY sale-level entry for cash tendered/change.
func ApplyCashTenderToSale(sale *SaleSnapshot, tendered string) error {
	if sale == nil {
		return fmt.Errorf("sale required")
	}
	if len(sale.Payments) == 0 {
		sale.Payments = []PaymentInput{{Method: PaymentCash, Amount: "0.00"}}
	}
	return ApplyCashTender(&sale.Payments[0], tendered)
}
