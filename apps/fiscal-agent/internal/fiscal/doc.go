// Package fiscal is the Farvoo Local Fiscal Agent core.
//
// Scope (P0): compliant FT/NC issuance, local SQLite authority, fiscal print
// queue, SAF-T export, AT series registration. Does not own POS checkout,
// inventory, SKU master, or cloud Supabase print_jobs for fiscal documents.
//
// Architecture layers (dependency direction: api → service → store/compliance;
// print/saft/at are called from service):
//
//	domain/     Pure types and invariants (no I/O)
//	compliance/ Hash, ATCUD, QR, Windows-1252, NIF validation
//	store/      SQLite repositories and migrations
//	service/    Issue FT/NC, reprint, idempotency orchestration
//	print/      Frozen print_payload and ESC/POS fiscal receipt rendering
//	saft/       SAF-T(PT) 1.04_01 export
//	at/         Series / ATCUD SOAP client
//	sync/       sync_outbox to cloud (async, non-authoritative)
//	api/        Local HTTP (/local/v1/...)
//
// Existing package main retains cloud business print (kitchen/receipt jobs).
// Fiscal physical print uses local SQLite queue only.
package fiscal
