-- M3.2c: bootstrap 首位开票员 role owner → admin（方案 A，每 store 一行）
-- 权威：docs/fiscal-m3-2-operators.zh.md §3.11

UPDATE operators
SET role = 'admin',
    can_issue_nc = 1,
    updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now'),
    session_epoch = session_epoch + 1
WHERE id IN (
  SELECT o.id
  FROM operators o
  WHERE o.role = 'owner'
    AND NOT EXISTS (
      SELECT 1 FROM operators a
      WHERE a.store_id = o.store_id AND a.role = 'admin'
    )
    AND o.created_at = (
      SELECT MIN(o2.created_at) FROM operators o2 WHERE o2.store_id = o.store_id
    )
);
