-- RG (recibo): receivable tracking on originals + receipt→FT reference.
ALTER TABLE invoices ADD COLUMN received_gross_total TEXT NOT NULL DEFAULT '0.00';

CREATE TABLE IF NOT EXISTS invoice_receipt_references (
  id TEXT PRIMARY KEY NOT NULL,
  receipt_invoice_id TEXT NOT NULL UNIQUE,
  original_invoice_id TEXT NOT NULL,
  original_invoice_no TEXT NOT NULL,
  amount TEXT NOT NULL,
  FOREIGN KEY (receipt_invoice_id) REFERENCES invoices(id),
  FOREIGN KEY (original_invoice_id) REFERENCES invoices(id)
);

CREATE INDEX IF NOT EXISTS idx_receipt_refs_original
  ON invoice_receipt_references(original_invoice_id);
