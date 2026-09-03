-- Local terminal pair codes + last_seen_ip (admin/owner manage; not Ops)

ALTER TABLE fiscal_terminals ADD COLUMN last_seen_ip TEXT;

CREATE TABLE IF NOT EXISTS terminal_pair_codes (
  code TEXT PRIMARY KEY NOT NULL,
  store_id TEXT NOT NULL,
  label TEXT,
  created_by_operator_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  consumed_at TEXT
);

CREATE INDEX IF NOT EXISTS terminal_pair_codes_store_idx
  ON terminal_pair_codes (store_id);
