-- SAF-T keeps two users per document (Portaria 302/2016): SourceID = who created it
-- (4.1.4.10 / 4.2.3.x / 4.3.4.x), DocumentStatus/SourceID = who set the current status
-- (4.1.4.3.4). invoices.source_id is the creator only; status changes write status_source_id.
-- Rows whose status already changed before 019 lost the creator (source_id was overwritten).
ALTER TABLE invoices ADD COLUMN status_source_id TEXT REFERENCES operators(id);

UPDATE invoices SET status_source_id = source_id;

-- ONLY initial write: a new document's current status is set by its creator.
CREATE TRIGGER IF NOT EXISTS trg_invoices_status_source_id AFTER INSERT ON invoices
WHEN NEW.status_source_id IS NULL
BEGIN
  UPDATE invoices SET status_source_id = NEW.source_id WHERE id = NEW.id;
END;
