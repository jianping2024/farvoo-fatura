// Package api exposes the Agent Local HTTP API.
//
// Routes (v0.17 + bill-sync):
//   POST /local/v1/fiscal-documents
//   GET  /local/v1/fiscal-documents/by-request/{requestId}
//   POST /local/v1/fiscal-documents/{documentId}/reprints
//   POST /local/v1/fiscal-documents/{documentId}/credit-notes
//   POST /local/v1/fiscal-documents/{documentId}/debit-notes  (deferred UI)
//   GET  /local/v1/print-jobs/{printJobId}
//   GET  /local/v1/bill-drafts
//   GET  /local/v1/bill-drafts/{id}
//   POST /local/v1/bill-drafts/{id}/issue   (mode whole_table|person + optional customer_nif)
//   POST /local/v1/bill-drafts/{id}/discard
//
// Default bind: 127.0.0.1; optional STORE_LAN mode with device pairing.
package api
