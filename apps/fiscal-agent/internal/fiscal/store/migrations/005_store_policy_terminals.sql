-- M3.2: Ops store policy + fiscal terminals (Agent cache)

ALTER TABLE taxpayer_settings ADD COLUMN fiscal_profile TEXT;
ALTER TABLE taxpayer_settings ADD COLUMN max_fiscal_terminals INTEGER;
ALTER TABLE taxpayer_settings ADD COLUMN ops_policy_synced_at TEXT;

CREATE TABLE IF NOT EXISTS fiscal_terminals (
  id TEXT PRIMARY KEY NOT NULL,
  store_id TEXT NOT NULL,
  label TEXT,
  active INTEGER NOT NULL DEFAULT 1,
  ops_terminal_ref TEXT NOT NULL,
  registered_at TEXT NOT NULL,
  last_seen_at TEXT
);

CREATE UNIQUE INDEX IF NOT EXISTS fiscal_terminals_ops_ref_idx
  ON fiscal_terminals (store_id, ops_terminal_ref);
