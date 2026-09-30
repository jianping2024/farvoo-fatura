-- PF (pró-forma): work-status columns + sale→PF reference.
ALTER TABLE invoices ADD COLUMN status_reason TEXT;
ALTER TABLE invoices ADD COLUMN status_changed_at TEXT;
ALTER TABLE invoices ADD COLUMN valid_until TEXT;

CREATE TABLE IF NOT EXISTS invoice_proforma_references (
  id TEXT PRIMARY KEY NOT NULL,
  sale_invoice_id TEXT NOT NULL UNIQUE,
  proforma_invoice_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY (sale_invoice_id) REFERENCES invoices(id),
  FOREIGN KEY (proforma_invoice_id) REFERENCES invoices(id)
);

CREATE INDEX IF NOT EXISTS idx_proforma_refs_pf
  ON invoice_proforma_references(proforma_invoice_id);
