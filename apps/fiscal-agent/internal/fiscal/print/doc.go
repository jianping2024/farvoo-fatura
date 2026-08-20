// Package print builds frozen fiscal print payloads and renders ESC/POS bytes.
//
// Responsibilities:
//   - print_payload v1 schema (JSON stored in local_print_jobs.payload)
//   - Fiscal receipt layout (80mm): header, lines, IVA summary, ATCUD, QR, cert line
//   - QR graphics via ESC/POS native commands (not bitmap)
//   - Chinese display_name via bitmap path (reuse main escpos_bitmap_text)
//
// Input is read-only SignedDocument + lines + customer snapshot; never recalculates tax.
package print
