// Package sync pushes non-authoritative copies to cloud (sync_outbox).
//
// Cloud never assigns InvoiceNo, Hash, or ATCUD. Failures do not block local issue.
package sync
