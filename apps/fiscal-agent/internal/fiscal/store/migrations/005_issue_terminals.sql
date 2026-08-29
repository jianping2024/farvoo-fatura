-- M4 §13 P0: paired issue terminals (LAN / Farvoo cashier → Agent).
-- Secret stored as SHA-256 hex; plaintext never persisted.

CREATE TABLE IF NOT EXISTS issue_terminals (
  id TEXT PRIMARY KEY NOT NULL,
  store_id TEXT NOT NULL,
  display_name TEXT NOT NULL DEFAULT '',
  secret_hash TEXT NOT NULL,
  station_id TEXT,
  active INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_issue_terminals_store
  ON issue_terminals(store_id, active);
