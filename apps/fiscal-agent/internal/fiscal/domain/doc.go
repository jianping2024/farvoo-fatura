// Package domain defines fiscal document types and sale snapshots.
//
// Rules:
//   - Invoice (signed) vs InvoiceDraft (unsigned) naming per v0.17
//   - Amounts use decimal strings at API boundaries; domain uses shopspring/decimal internally
//   - Sale snapshots are immutable input to issuance; no table/order/inventory fields here
package domain
