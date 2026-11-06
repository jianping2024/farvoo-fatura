-- SAF-T SourceID is SAFPTtextTypeMandatoryMax30Car; operators.id is a 36-char UUID.
-- saft_user_code is the short per-store user code written as SourceID (U001, U002, …).
-- invoices.source_id stays the operators.id FK; SAF-T loaders join to read the code.
ALTER TABLE operators ADD COLUMN saft_user_code TEXT;

UPDATE operators SET saft_user_code = (
  SELECT printf('U%03d', r.rn) FROM (
    SELECT id, ROW_NUMBER() OVER (PARTITION BY store_id ORDER BY created_at, id) AS rn FROM operators
  ) r WHERE r.id = operators.id
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_operators_store_saft_user_code ON operators(store_id, saft_user_code);

-- ONLY write path for saft_user_code: every new operator gets the next code in its store.
CREATE TRIGGER IF NOT EXISTS trg_operators_saft_user_code AFTER INSERT ON operators
WHEN NEW.saft_user_code IS NULL
BEGIN
  UPDATE operators SET saft_user_code = printf('U%03d', (
    SELECT COALESCE(MAX(CAST(substr(saft_user_code, 2) AS INTEGER)), 0) + 1
    FROM operators WHERE store_id = NEW.store_id AND saft_user_code IS NOT NULL
  )) WHERE id = NEW.id;
END;
