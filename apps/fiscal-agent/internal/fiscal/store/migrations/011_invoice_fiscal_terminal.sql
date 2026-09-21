-- invoices: freeze issuing fiscal terminal (open PC), not print station_id
ALTER TABLE invoices ADD COLUMN fiscal_terminal_id TEXT;
ALTER TABLE invoices ADD COLUMN fiscal_terminal_label TEXT;

CREATE INDEX IF NOT EXISTS idx_invoices_store_date_terminal
  ON invoices(store_id, invoice_date, fiscal_terminal_id);
