# Fiscal Agent SQLite Schema（P0）速查

> **完整字段与规则**：[`docs/fiscal-sqlite-schema.zh.md`](../../../../../../docs/fiscal-sqlite-schema.zh.md)  
> **DDL**：[`migrations/001_init.sql`](migrations/001_init.sql)、[`migrations/002_bill_sync_drafts.sql`](migrations/002_bill_sync_drafts.sql)、[`migrations/003_print_job_station.sql`](migrations/003_print_job_station.sql)、[`migrations/004_bill_draft_allocation.sql`](migrations/004_bill_draft_allocation.sql)、[`migrations/005_issue_terminals.sql`](migrations/005_issue_terminals.sql)

## 原则

| 原则 | 定法 |
|------|------|
| 金额 | `TEXT` 小数串，禁止 `REAL` |
| 时间 | 库内 UTC；`InvoiceDate`/票面按 `taxpayer_settings.timezone`（默认 `Europe/Lisbon`） |
| 密钥 | **双钥**：产品 RSA（签发票）≠ 设备 TPM/DPAPI（解 wrapped）；见设计文档 §2.2 |
| 签后 | invoices / lines / 客户快照 / Hash / ATCUD / QR 不可业务 UPDATE |
| 打印 | 税务权威队列 = `local_print_jobs`；云端 `print_jobs` 仅业务热敏 |
| 商品 | `fiscal_products` 薄投影，无库存 |

## 表分组

```text
taxpayer_settings, at_credentials, signing_keys, agent_installations, operators, issue_terminals
customers, fiscal_product_categories, fiscal_products
series, invoices, invoice_lines, invoice_customer_snapshots,
  invoice_payments, invoice_line_references
idempotency_keys, local_print_jobs, print_attempts, audit_log
saft_exports, sync_outbox
bill_sync_drafts
```

## 签发同事务

idempotency → series 占号 → invoice(+lines/snapshot/payments) → ORIGINAL local_print_job → **sync_outbox INVOICE_ISSUED** → COMMIT  
打印失败不回滚税务行；outbox 推送失败不回滚税务行。

**唯一写路径：** `store.IssueFT`（经 `service.IssueDocument` / `POST /local/v1/fiscal-documents`）。禁止第二套插票逻辑。

**sync_outbox 入队唯一路径：** `store.EnqueueInvoiceIssuedTx`（仅 `IssueFT` 成功签发事务内）。冲刷唯一：`sync.Worker` → `ClaimNextOutbox` → `PushInvoiceCopy` → `MarkOutboxSent` / `MarkOutboxRetry`。

**§13 开票鉴权唯一路径：** `api.withIssueAuth` → `auth.AuthenticateIssue`（终端 `VerifyIssueTerminal` + `VerifyOperatorToken` + `EnsureOperatorFromMesa`）。覆盖 `fiscal-documents` / `manual` / `bill-drafts/.../issue`。

**开票终端唯一写路径：** `store.UpsertIssueTerminal`（`PUT /local/v1/setup/issue-terminal`）。

**账单同步唯一写路径：** `billsync.PullAndIngest` → `IngestCloudJob` → `UpsertBillDraftOpen` + `UpsertFiscalProductByCode`。Realtime/Polling 只门铃/补偿，禁止第二套 HTTP/WS。

**Admin 收银账单提示唯一推送：** `UpsertBillDraftOpen` / `DeleteBillDraftsBySale` → `DB.OnBillDraftsChanged` → `uievents.Hub.NotifyBillDraftsChanged` → `GET /local/v1/events`（SSE）。禁止浏览器空转轮询主路径。UAT 门铃入口：`POST /local/v1/dev/bill-sync/pull`（`FISCAL_ALLOW_DEV_KEY=1`）→ 同进程 `PullAndIngest`（禁止另起进程写库冒充推送）。界面用语「收银账单」见原型 README 方案 A。

**草稿开票唯一路径：** `billsync.DraftToSaleSnapshot`（整桌）/ `billsync.DraftPersonFromAllocation`（按人）→ `ApplyCustomerOverride` → `service.IssueFromBillDraft` → `IssueDocument`/`IssueFT` →（到期时）`DeleteBillDraftsBySale`。丢弃仅 `DiscardBillDrafts` → `DeleteBillDraftsBySale`。再同步靠 `HasSignedFTForSale` / `ListSignedFTScopesForSale`。本机分单：`service.SaveBillDraftAllocation` → `store.SaveBillDraftAllocation`（OCC）。`DraftPartToSaleSnapshot` 仅为 splits→allocation 适配器，不得作为 issue 主路径。

**REMOTE 商品同步唯一写路径：** `IngestCloudJob` → `UpsertFiscalProductByCode`（不覆盖 LOCAL）。

**LOCAL 商品唯一写路径：** `UpsertLocalFiscalProduct`（API `POST /local/v1/products`）。

**LOCAL 客户唯一写路径：** `UpsertLocalCustomer`（API `POST /local/v1/customers`）；开票绑 `customer_id` 仅 `ensureCustomerIDTx`。

**手动 FT 唯一路径：** `catalog.BuildManualSaleSnapshot` → `service.IssueManualFT` → `IssueDocument` → `IssueFT`（API `POST /local/v1/fiscal-documents/manual`）。

**重打唯一路径：** `print.ClonePayloadForReprint` → `store.CreateReprintPrintJob`（API `POST /local/v1/fiscal-documents/{id}/reprints`）。禁止重签；禁止经 `IssueFT` 插入 REPRINT 行。

## 聚合 ↔ 表（实现约束）

| 聚合 / 模块 | 表 | 唯一入口 |
|-------------|-----|----------|
| Series | `series` | 仅 `IssueFT` 更新 `last_number`/`last_hash` |
| Invoice（签后不可变） | `invoices` + lines/snapshot/payments | 仅 `IssueFT` INSERT |
| Fiscal Print Job | `local_print_jobs` + `print_attempts` | 签发插入（含 `station_id`）；`worker.Worker` 认领；物理出纸仅注入 `PrintBytesFn`→`printToTarget` |
| Bill sync draft | `bill_sync_drafts` | 写入仅 `UpsertBillDraftOpen`；allocation 仅 `SaveBillDraftAllocation`；开票后清仅 `DeleteBillDraftsBySale` |
| 序号字符串 | — | 仅 `compliance.FormatSequence` / `FormatInvoiceNo` / `FormatATCUD` |
| Hash 输入 | — | 仅 `compliance.BuildSignPayload` |
| QR | — | 仅 `compliance.BuildQR` |
| 打印快照 / ESC/POS | — | 仅 `print.BuildPayload` / `print.RenderESCPOS`（版式）；出纸不另造 TCP/WinSpool |
