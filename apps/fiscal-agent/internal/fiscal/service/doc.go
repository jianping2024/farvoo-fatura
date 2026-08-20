// Package service orchestrates fiscal workflows.
//
// Responsibilities:
//   - IssueFT / IssueFS (P0: FT default)
//   - IssueNC (full/partial credit against one FT)
//   - Reprint (new print job, same signed document)
//   - Idempotency and business-scope checks
//   - Windows-1252 gate before signing
//   - Single DB transaction: series lock → number → hash → persist → ORIGINAL print job
//
// Must not render ESC/POS or build SAF-T XML directly; delegates to compliance/,
// print/, and saft/.
package service
