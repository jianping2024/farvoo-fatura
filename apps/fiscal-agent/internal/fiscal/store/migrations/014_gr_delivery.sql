-- GR (guia de remessa): movement fields + FT→GR references.
ALTER TABLE invoices ADD COLUMN ship_from_address TEXT;
ALTER TABLE invoices ADD COLUMN ship_to_address TEXT;
ALTER TABLE invoices ADD COLUMN movement_start_at TEXT;
ALTER TABLE invoices ADD COLUMN at_transport_doc_code TEXT;
ALTER TABLE invoices ADD COLUMN recipient_is_vat_payer INTEGER;

CREATE TABLE IF NOT EXISTS invoice_delivery_references (
  id TEXT PRIMARY KEY NOT NULL,
  sale_invoice_id TEXT NOT NULL,
  delivery_invoice_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY (sale_invoice_id) REFERENCES invoices(id),
  FOREIGN KEY (delivery_invoice_id) REFERENCES invoices(id),
  UNIQUE (sale_invoice_id, delivery_invoice_id)
);

CREATE INDEX IF NOT EXISTS idx_delivery_refs_sale
  ON invoice_delivery_references(sale_invoice_id);

CREATE INDEX IF NOT EXISTS idx_delivery_refs_gr
  ON invoice_delivery_references(delivery_invoice_id);
