-- M3.2b: operator session revocation (bump invalidates all issued fiscal_session cookies)

ALTER TABLE operators ADD COLUMN session_epoch INTEGER NOT NULL DEFAULT 0;
