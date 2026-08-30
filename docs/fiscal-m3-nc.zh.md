# M3：NC（冲销 / Nota de Crédito）

> **状态：定稿**  
> **权威：是**（M3 行为与 API；库列仍以 `fiscal-sqlite-schema.zh.md` + `migrations/001_init.sql` 为准）  
> **对应实现：** M3 已落地（`store.IssueNC`、`service.IssueCreditNote`、Admin 冲销、`scripts/fiscal-m3-regression.mjs`）  
> **计划：** [`fiscal-dev-plan.zh.md`](fiscal-dev-plan.zh.md) M3  
> **边界：** 餐馆 Farvoo 不开 NC；[`farvoo-fiscal-agent-integration.zh.md`](../../restaurant-ordering/docs/technical/farvoo-fiscal-agent-integration.zh.md) §4、§7

## 1. 目标

对已签发的正式单据（**FT / FS / FR**）开具 **NC**，支持全额或部分冲销；独立 NC 系列与 Hash 链；打印展示原票引用；幂等；原票累计冲销额与 `document_status` 正确更新。

## 2. 非目标

| 项 | 说明 |
|----|------|
| ND / 新开 FS / FR | 归 M6 |
| Farvoo 桌台 / 云 API 开 NC | 须 Agent Admin / Local API |
| NC 冲 NC | 不做 |
| 会计对账 UI | 不做 |
| 独立 `can_issue_nc` 权限表 | P0 与现 Admin 登录同权；见 §8 备选 |
| 开票时查云 | 不做；吊销收敛仍仅启动同步（Ops 签名钥，非 M3） |

## 3. P0 定法

| 项 | 定法 |
|----|------|
| 可冲原票类型 | `document_type ∈ {FT, FS, FR}` |
| 可冲原票状态 | `SIGNED` 或 `CREDITED_PARTIAL`；`CREDITED_FULL` 拒绝 |
| 引用关系 | **一张 NC 只引一张原票**；行级写 `invoice_line_references` |
| 金额 | NC 行金额为**正数**（与 FT 相同符号约定）；`Σ 本 NC.gross` 加上原票已有 `credited_gross_total` 不得超过原票 `gross_total` |
| 原票可改列 | **仅** `document_status`、`credited_gross_total`（对已签原票的唯一业务 UPDATE） |
| 原票状态推导 | 新累计 `credited_gross_total == gross_total` → `CREDITED_FULL`；否则且 >0 → `CREDITED_PARTIAL` |
| NC 自身 `document_status` | 签发后固定 `SIGNED`（NC 不再被 NC 引用，P0） |
| 系列 | **独立** `series.document_type = 'NC'`；占号、Hash 链与 FT **不共用** `last_hash` |
| 客户快照 | P0 **复制原票** `invoice_customer_snapshots`（不可在冲销 UI 改客户） |
| 付款行 | P0 单行，`amount = NC.gross_total`；`method` 取原票首条 payment，无则 `CASH` |
| 入口 | Agent Admin 发票详情 + `POST /local/v1/fiscal-documents/{id}/credit-notes` |
| 幂等 | 同 FT：`idempotency_keys` 双唯一 `(store_id, request_id)` 与 `(store_id, business_key)` |
| 重打 | 与 FT 相同：仅重打冻结 payload，不重签 |

## 4. 前置：NC 系列

与 M1 FT 系列相同流程，**仅 `document_type` 不同**：

1. Setup：`POST /local/v1/setup/series/register`，body 含 `document_type: "NC"`、`series_code`、`fiscal_year`  
2. 本机 `series.status = ACTIVE` 且 `validation_code` 非空方可 `IssueNC`  
3. 系列命名建议：`NC{YYYY}{suffix}`（与 FT 并列，勿复用 FT 的 `series_code`）

**UAT / 回归：** 脚本在 FT 系列之外再注册一条 NC 系列。

## 5. 唯一写路径

```text
Admin UI / Local API
  → service.IssueCreditNote（唯一编排）
    → store.IssueNC（唯一 SQLite 事务）
```

| 层 | 唯一入口 | 禁止 |
|----|----------|------|
| 编排 | `service.IssueCreditNote` | 在 `IssueFT` / handler 内联插 NC |
| 持久化 | `store.IssueNC` | 第二套 INSERT invoices for NC |
| 合规号 | `compliance.FormatInvoiceNo` / `FormatATCUD` / `BuildSignPayload` / `BuildQR` | 手写 ATCUD/Hash |
| 签名 | `ensureSigner` → `Signer.Sign` | 新签名器 |
| 打印快照 | `print.BuildPayload` | handler 拼 ESC/POS |
| 打印渲染 | `print.RenderESCPOS` | 第二套 renderer |
| 原票更新 | `IssueNC` 事务内 UPDATE `invoices` | 其它路径改 `credited_gross_total` |

**不修改** `IssueFT` 开 NC；FT 仍只经 `IssueFT`。

## 6. 请求 / 响应（Local API）

### 6.1 `POST /local/v1/fiscal-documents/{documentId}/credit-notes`

**Path：** `documentId` = 原票 `invoices.id`（被冲销的 FT/FS/FR）。

**Body（JSON）：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| request_id | string | 是 | 客户端幂等键 |
| operator_id | string | 是 | → `invoices.source_id` |
| station_id | string | 否 | 打印档口；空则用 setup 默认 |
| reason | string | 是 | → `invoice_line_references.reason`；同时进 print `credit_reason` |
| credit_full | boolean | 否 | 默认 `false`；`true` 表示按原票**全部行全额**冲销 |
| lines | array | 条件 | `credit_full=false` 时必填；见 §6.2 |

**成功 200：** 与 `POST /local/v1/fiscal-documents` 同形（`document_id`、`invoice_no`、`atcud`、`document_status`、`print_job_id` …）。

**错误码（稳定）：**

| code | HTTP | 含义 |
|------|------|------|
| `not_found` | 404 | 原票不存在 |
| `validation_failed` | 400 | body 非法、reason 空、lines 空等 |
| `credit_not_allowed` | 409 | 原票类型不在白名单或状态不可冲 |
| `credit_amount_exceeded` | 409 | 累计冲销超原票 gross |
| `series_missing` | 409 | 无 ACTIVE NC 系列或缺 validation_code |
| `signer_not_ready` | 409 | 未激活 / 无法解封产品钥 |
| `idempotency_conflict` | 409 | 同 request_id 不同 payload |
| `issue_failed` | 400 | 其它 |

### 6.2 部分冲销 `lines[]`

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| original_line_number | integer | 是 | 原票 `invoice_lines.line_number` |
| quantity | string | 否 | 默认原行剩余可冲数量；有理数串，规则同 FT `ParseQtyString` |
| line_gross | string | 否 | 与 quantity 二选一指定金额；指定时须 ≤ 该行剩余可冲 gross |

**行剩余可冲（P0）：**  
`remaining_line_gross = original_line.line_gross − Σ(已开 NC 对该 original_line_id 的引用行 gross)`  
（实现时在 `IssueNC` 内聚合 `invoice_line_references` + 对应 NC 行金额。）

**全额冲销（`credit_full: true`）：** 忽略 `lines`；对所有原票行按剩余可冲全额生成 NC 行。

## 7. 幂等

### 7.1 `business_key`

```
{store_id}|credit|{original_invoice_id}|{mode}|{fingerprint}
```

| 段 | 说明 |
|----|------|
| mode | `full` 或 `partial` |
| fingerprint | `full` 时固定字面量 `full`；`partial` 时为规范化 `lines[]` JSON 的 SHA-256 hex |

规范化 partial payload（稳定排序 `original_line_number` 升序）示例：

```json
[{"original_line_number":1,"quantity":"1","line_gross":"12.50"}]
```

### 7.2 行为

1. 先查 `(store_id, request_id)`：命中且 `request_payload_hash` 一致 → 返回原 NC  
2. 同 `request_id` 不同 hash → `idempotency_conflict`  
3. 再查 `(store_id, business_key)`：命中 → 返回原 NC（防双开）  
4. 成功写入 `idempotency_keys.invoice_id` = 新 NC 的 `document_id`

NC 的 `invoices` 业务键列（`source_system` 等）P0 填：

| 列 | 值 |
|----|-----|
| source_system | 原票 `source_system`，空则 `LOCAL` |
| source_sale_id | 原票 `source_sale_id` |
| scope_type | 原票 `scope_type` |
| scope_id | 原票 `scope_id` |
| fiscal_purpose | 固定 `credit` |

## 8. 事务写序（`IssueNC`）

同事务 `BEGIN IMMEDIATE`（与 schema §7 一致）：

1. `SELECT` 原票 + lines + customer snapshot（FOR UPDATE 语义：先 UPDATE series 锁）  
2. 校验类型、状态、可冲余额  
3. 计算 NC 行与 totals  
4. 查 / 写 `idempotency_keys`  
5. 更新 **NC 系列** `last_number` / `last_hash`  
6. INSERT NC `invoices`（`document_type=NC`，`document_status=SIGNED`）  
7. INSERT NC `invoice_lines`  
8. INSERT `invoice_line_references`（每 NC 行一条）  
9. INSERT NC `invoice_customer_snapshots`（复制原票）  
10. INSERT NC `invoice_payments`  
11. UPDATE 原票 `credited_gross_total`、`document_status`  
12. INSERT `local_print_jobs`（ORIGINAL；payload 含 `original_invoice_no`、`credit_reason`）  
13. COMMIT  

**不入** `sync_outbox`（M4 范围外，与当前 FT 餐馆路径一致）。

## 9. 表与列（与 DDL 一致）

M3 **不新增 migration**；使用现有表。

### 9.1 原票 `invoices`（UPDATE 仅此二列）

| 列 | 说明 |
|----|------|
| credited_gross_total | TEXT 金额两位小数；累加本 NC gross |
| document_status | `CREDITED_PARTIAL` 或 `CREDITED_FULL` |

### 9.2 `invoice_line_references`

| 列 | 类型 | 必填 | 说明 |
|----|------|------|------|
| id | TEXT PK | 是 | UUID |
| credit_line_id | TEXT UQ FK | 是 | → NC 的 `invoice_lines.id` |
| original_invoice_id | TEXT FK | 是 | 被冲原票 id |
| original_invoice_no | TEXT | 是 | 完整 InvoiceNo 快照 |
| original_line_id | TEXT | 是 | 原 `invoice_lines.id` |
| original_line_number | INTEGER | 是 | 原行号 |
| reason | TEXT | 是 | 冲销原因 |

### 9.3 NC 新票 `invoices`

与 FT 相同列集；`document_type = NC`；`document_status = SIGNED`；`previous_hash` 来自 **NC 系列**上一张。

## 10. 合规与 Hash

| 项 | 定法 |
|----|------|
| InvoiceNo | `compliance.FormatInvoiceNo("NC", series_code, seq)` |
| ATCUD | `compliance.FormatATCUD(validation_code, seq)` |
| Hash 链 | 仅在同一条 NC `series_id` 内链接 `previous_hash` |
| QR `DocumentType` | `NC` |
| QR `DocumentStatus` | `N`（与 FT 签发一致） |
| 行税额 | 复用 `buildLines` 同类逻辑；VAT 桶与原行 `vat_rate` / `tax_code` 一致 |

## 11. 打印（P0）

`print.BuildPayload` 已有字段（实现时填入）：

| 字段 | 来源 |
|------|------|
| compliance.original_invoice_no | 原票 `invoice_no` |
| compliance.credit_reason | 请求 `reason` |
| document_type | `NC` |

`print.RenderESCPOS` **新增**（P0）：在票头区（Via 行之后）打印：

- `Documento original: {original_invoice_no}`  
- `Motivo: {credit_reason}`  

版式 authority 仍跟 [`fiscal-ft-receipt-layout.zh.md`](fiscal-ft-receipt-layout.zh.md)；NC 差异仅上述两行 + 标题用 NC 的 `formatFaturaNoLine`。

## 12. Admin UI（P0）

位置：**发票详情**抽屉（已有重打按钮旁）。

| 条件 | UI |
|------|-----|
| 原票 `document_type ∈ {FT,FS,FR}` 且 `document_status ∈ {SIGNED, CREDITED_PARTIAL}` | 显示 **冲销** |
| `CREDITED_FULL` | 隐藏冲销 |
| 表单 | `reason`（必填）；P0 默认 **全额**（`credit_full: true`） |
| 成功 | toast + 刷新详情/列表；原票状态更新可见 |

**不做：** Farvoo 入口；M3 不做按行部分冲销 UI（API 支持 partial，UI 可 M3.1 再加）。

## 13. 交付物与验收（对应 fiscal-dev-plan D3.x）

| # | 交付物 | 完成定义 |
|---|--------|----------|
| D3.1 | 本文 | 定稿且与 DDL 一致 |
| D3.2 | `store.IssueNC` + 单测 | §8 事务；全额/部分/超额/幂等 |
| D3.3 | API + `service.IssueCreditNote` | §6 |
| D3.4 | Admin 详情冲销 | §12 |
| D3.5 | ESC/POS 原票引用 | §11 |
| D3.6 | `scripts/fiscal-m3-regression.mjs` | FT 系列 + NC 系列 → FT → 全额 NC → 幂等 → 原票 CREDITED_FULL → 打印 payload 断言 |

### 验收清单

1. NC 使用独立系列，不占 FT 序号。  
2. FT 全额/部分冲销正确；FS/FR 原票在单测中至少各 1 例类型校验（不必等 FS/FR 签发产品化）。  
3. 原票 `CREDITED_*` 与 `credited_gross_total` 正确。  
4. 重复 `request_id` / 相同 `business_key` 不双开 NC。  
5. 已 CREDITED_FULL 再冲拒绝。  
6. Admin 可冲销；Farvoo 无 NC。

## 14. 备选 / 以后

| 项 | 说明 |
|----|------|
| Admin 按行部分冲销 UI | API 已支持 `lines[]` |
| `can_issue_nc` 操作员权限 | 独立列或 role 表 |
| FS/FR 产品化签发 | M6；NC 内核已支持原票类型白名单 |
| `sync_outbox` NC 事件 | 云副本推送，非 M3 |
| SAF-T 导出含 NC | M5 |

## 15. 参考

| 文档 | 用途 |
|------|------|
| [`fiscal-sqlite-schema.zh.md`](fiscal-sqlite-schema.zh.md) §6.10–6.14、§7 | 表与写序 |
| [`fiscal-m1-identity-series.zh.md`](fiscal-m1-identity-series.zh.md) | 系列注册 |
| [`farvoo-fiscal-agent-integration.zh.md`](../../restaurant-ordering/docs/technical/farvoo-fiscal-agent-integration.zh.md) §7 | 状态机、NC 规则 |
| [`fiscal-dev-plan.zh.md`](fiscal-dev-plan.zh.md) M3 | 里程碑 |
