-- Cash tendered / change (operational; amount remains settlement).
ALTER TABLE invoice_payments ADD COLUMN tendered TEXT;
ALTER TABLE invoice_payments ADD COLUMN change_due TEXT;
