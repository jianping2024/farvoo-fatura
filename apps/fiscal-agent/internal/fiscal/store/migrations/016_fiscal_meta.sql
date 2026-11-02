-- fiscal_meta: per-DB facts that must never change after first bind (e.g. at_env).
CREATE TABLE IF NOT EXISTS fiscal_meta (
  key TEXT PRIMARY KEY NOT NULL,
  value TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
