# M6 D6.3 / D6.4 — 备份校验与换机（最小集）

> **状态：定稿**（P0 定法；实现以本文 + `fiscal-dev-plan.zh.md` M6 为准）  
> **权威：是**（本仓 D6.3/D6.4 交付口径）  
> **关联：** [`fiscal-sqlite-schema.zh.md`](fiscal-sqlite-schema.zh.md) §换机；[`fiscal-ops-signing-key.zh.md`](fiscal-ops-signing-key.zh.md)；认证 [`fiscal-certification-checklist.zh.md`](fiscal-certification-checklist.zh.md) C7.1 / C7.2

## 1. 目标与非目标

| 做 | 不做 |
|----|------|
| 本机税务库 **备份**（一致快照文件） | 云端自动备份 / 多机热备 |
| 恢复后 **系列完整性校验**；失败 **阻断开票系列** | API 热替换正在打开的 `fiscal.db`（须停进程后换文件） |
| **换机最小集**：本机吊销授权 + 备份；新机恢复库后重新 wrap/激活 | Ops 云端吊销 UI；完整运维手册长文 |

## 2. P0 定法

### 2.1 备份（D6.3）

| 项 | 定法 |
|----|------|
| 唯一路径 | `store.BackupFiscalDB` → SQLite `VACUUM INTO` 目标文件 |
| 目录 | `{FISCAL_DATA_DIR 或 DB 同级}/backups/fiscal-YYYYMMDD-HHMMSS.db` |
| API | `POST /local/v1/setup/backup` → `{ backup_path, bytes }` |
| UI | 设置「8. 备份与换机」→「备份税务库」 |

### 2.2 完整性校验（D6.3 / C7.1）

对 **每一个** `series` 行（含非 ACTIVE，便于恢复后诊断）：

| 系列字段 | 期望（来自 `invoices`） |
|----------|------------------------|
| `last_number` | 该 `series_id` 的 `MAX(sequence_number)`；无票则为 `0` |
| `last_hash` | 该序号对应发票的 `hash`；无票则为空串 |

| 项 | 定法 |
|----|------|
| 唯一读路径 | `store.VerifySeriesIntegrity` |
| API | `POST /local/v1/setup/integrity/verify` body：`block_on_fail`（默认 `true`）、`heal_on_pass`（默认 `false`） |
| `block_on_fail=true` 且不匹配 | 将该系列 `status` 从 `ACTIVE` 改为 **`FAILED`**；写 `audit_log`（`series_integrity_failed`） |
| 阻断开票 | 签发只认 `ACTIVE`；`ready_to_issue` / `series_ok` 自然变 false |
| `heal_on_pass=true` 且该系列当前匹配 | 若 `status='FAILED'` 且仍有 `validation_code` → 改回 `ACTIVE`（`series_integrity_healed`） |
| 热恢复 | **禁止**：文档要求停 Agent → 替换 `fiscal.db`（及 `-wal`/`-shm` 一并处理或只换干净备份）→ 启动 → 调 verify |

### 2.3 换机（D6.4 / C7.2）

对齐 schema §换机，本机最小编排：

```text
旧机：POST /setup/backup → POST /setup/prepare-swap（ClearLocalActivation）
     → 运维拷走 backup 文件
新机：停进程放入 fiscal.db → 启动 → POST integrity/verify
     → POST activate-from-cloud（新 installation + 新 wrap C）
```

| 项 | 定法 |
|----|------|
| `prepare-swap` | **唯一**本地「主动换机停用」入口：先可选自动备份，再调用已有 **`ClearLocalActivation`**（signing_keys→RETIRED；`agent_installations.revoked_at` 置上） |
| 新 wrap | **不**另开路径；沿用 `ActivateFromCloud` / UAT `ActivateFiscal` |
| Ops 云端 revoke | 仍可由运营触发；Agent 启动 `TryPullCloudProvisionIfNeeded` 收敛；`prepare-swap` 不替代 Ops，只服务「本机先停」 |

## 3. API 一览

| Method | Path | 说明 |
|--------|------|------|
| POST | `/local/v1/setup/backup` | 备份 |
| POST | `/local/v1/setup/integrity/verify` | 校验；可选 block/heal |
| POST | `/local/v1/setup/prepare-swap` | 备份（默认开）+ ClearLocalActivation |

## 4. 验收

- `go test`：integrity 匹配/失配/block/heal；backup 文件可 Open  
- `node scripts/fiscal-d63-d64-regression.mjs`：开票 → backup → 篡改 last_hash → verify block → ready_to_issue false → heal 路径或 prepare-swap 后 activated_ok false  
- 清单 C7.1 / C7.2 改为自动（runner）+ 换机真机步骤仍可手测

## 5. 变更记录

| 日期 | 说明 |
|------|------|
| 2026-08-31 | 首版定稿 |
