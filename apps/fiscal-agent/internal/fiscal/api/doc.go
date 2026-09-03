// Package api exposes the Agent Local HTTP API.
//
// Routes (v0.17 + bill-sync):
//
//	POST /local/v1/fiscal-documents          (station_id required when Agent PrintBytesFn is live)
//	POST /local/v1/fiscal-documents/manual   (catalog/temp lines → IssueManualFT)
//	GET  /local/v1/products
//	POST /local/v1/products                  (LOCAL only)
//	GET  /local/v1/customers
//	POST /local/v1/customers                 (LOCAL only; not consumidor final)
//	GET  /local/v1/fiscal-documents            (?page & page_size & from & to & q & document_type)
//	GET  /local/v1/fiscal-documents/by-request/{requestId}
//	POST /local/v1/fiscal-documents/{documentId}/reprints
//	POST /local/v1/fiscal-documents/{documentId}/credit-notes
//	POST /local/v1/fiscal-documents/{documentId}/debit-notes
//	GET  /local/v1/print-jobs/{printJobId}
//	GET  /local/v1/bill-drafts
//	GET  /local/v1/bill-drafts/{id}
//	GET  /local/v1/printers                 (mapped station_printers)
//	POST /local/v1/bill-drafts/{id}/issue   (mode + station_id + optional customer_nif)
//	POST /local/v1/bill-drafts/{id}/discard
//	GET  /local/v1/events                   (SSE; bill_drafts_changed)
//	POST /local/v1/dev/bill-sync/pull       (UAT only; FISCAL_ALLOW_DEV_KEY=1 → PullAndIngest)
//
// Default bind: 127.0.0.1; optional STORE_LAN mode with device pairing.
package api
