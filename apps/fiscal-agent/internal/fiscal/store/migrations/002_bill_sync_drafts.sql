-- Bill sync drafts (Farvoo bill_sync_jobs → Agent local JSON). See farvoo-fiscal-bill-sync-api.zh.md.

CREATE TABLE IF NOT EXISTS bill_sync_drafts (
  id TEXT PRIMARY KEY NOT NULL,
  request_id TEXT NOT NULL,
  source_sale_id TEXT NOT NULL,
  payload_json TEXT NOT NULL,
  status TEXT NOT NULL,
  cloud_job_id TEXT,
  last_error TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_bill_sync_drafts_request
  ON bill_sync_drafts(request_id);

CREATE INDEX IF NOT EXISTS idx_bill_sync_drafts_sale_status
  ON bill_sync_drafts(source_sale_id, status);
