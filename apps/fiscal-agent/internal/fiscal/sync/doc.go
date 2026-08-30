// Package sync is reserved for a possible future cloud copy path (sync_outbox).
//
// P0 product law: all signed invoices are authoritative in local SQLite only;
// IssueFT/IssueNC do NOT write sync_outbox. External compliance export is M5 SAF-T.
// See docs/fiscal-sqlite-schema.zh.md §1.1.
package sync
