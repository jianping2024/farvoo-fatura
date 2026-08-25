-- Local by-item allocation for bill_sync_drafts (frozen payload + mutable allocation).
-- See docs/fiscal-bill-split-workbench-ux.zh.md §6.

ALTER TABLE bill_sync_drafts ADD COLUMN allocation_json TEXT NOT NULL DEFAULT '{}';
ALTER TABLE bill_sync_drafts ADD COLUMN allocation_revision INTEGER NOT NULL DEFAULT 0;
