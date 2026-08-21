// Package print builds frozen fiscal print payloads and renders ESC/POS bytes.
//
// Responsibilities:
//   - print_payload v1 schema (JSON stored in local_print_jobs.payload)
//   - Fiscal FT receipt layout — ONLY RenderESCPOS; authority docs/fiscal-ft-receipt-layout.zh.md
//   - QR via ESC/POS native commands
//
// Input is the frozen Payload from BuildPayload; never recalculates tax at print time.
package print
