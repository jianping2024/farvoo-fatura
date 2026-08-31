-- M6: track cumulative debit notes on original invoices.
ALTER TABLE invoices ADD COLUMN debited_gross_total TEXT NOT NULL DEFAULT '0.00';
