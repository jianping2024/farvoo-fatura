# RG：收款收据（Recibo）

> **状态：定稿**  
> **权威：是**（RG 行为与 API；库列以本文 + [`fiscal-sqlite-schema.zh.md`](fiscal-sqlite-schema.zh.md) + `migrations/012_rg_receipt.sql` 为准）  
> **对应实现：** `store.IssueRG`、`service.IssueReceipt`、`POST .../receipts`、Admin FT「登记收款」  
> **测试场景：** [`fiscal-doc-type-test-scenarios.zh.md`](fiscal-doc-type-test-scenarios.zh.md) §7  
> **计划顺序：** RG → PF → GR → GT

## 1. 目标

对已签 **FT**（挂账）开具 **RG**，证明事后收款；独立 RG 系列与 Hash 链；打印展示原 FT；幂等；原票 `received_gross_total` 与未收余额正确。

## 2. 非目标

| 项 | 说明 |
|----|------|
| RC（IVA de caixa） | 不做 |
| 对 FS / FR 开 RG | P0 禁止 |
| 无原票裸收据 | 禁止 |
| PF / GR / GT | 后续里程碑 |
| 用 RG 替代 FR | 禁止 |

## 3. P0 定法

| 项 | 定法 |
|----|------|
| 可收原票 | **仅** `document_type = FT` |
| 原票状态 | `SIGNED` / `CREDITED_PARTIAL` / `DEBITED_*`；`CREDITED_FULL` 且应收为 0 则拒绝 |
| 挂账识别 | 付款方式 **`ACCOUNT`** 不计入「开票时已结清」；其它方式计入 |
| 未收余额 | `gross − credited − settled_at_issue − received_via_rg`（见 §4） |
| 金额 | 单次 RG ≤ 未收余额；允许多张分期 |
| 系列 | 独立 `series.document_type = 'RG'` |
| SAF-T | 表 4.4 Payments，`PaymentType=RG`；**不**进 SalesInvoices |
| 权限 | 已登录可开票操作员（与 IssueDocument 同；不要求 `can_issue_nc`） |
| 入口 | FT 详情「登记收款」+ `POST /local/v1/fiscal-documents/{id}/receipts` |

## 4. 未收余额（唯一算法）

```text
settled_at_issue = Σ invoice_payments.amount WHERE method ≠ ACCOUNT
received_via_rg  = invoices.received_gross_total   （IssueRG 事务内累加）
credited         = invoices.credited_gross_total
receivable       = max(0, gross − credited − settled_at_issue − received_via_rg)
```

读者唯一：`store.ReceiptRemainingForInvoice`。  
写入原票 `received_gross_total` 唯一：`store.IssueRG` 事务内 UPDATE。

## 5. 唯一写路径

```text
Admin UI / Local API
  → service.IssueReceipt（唯一编排）
    → store.IssueRG（唯一 SQLite 事务）
```

| 层 | 唯一入口 | 禁止 |
|----|----------|------|
| 编排 | `service.IssueReceipt` | handler / IssueFT 内联插 RG |
| 持久化 | `store.IssueRG` | 第二套 INSERT invoices for RG |
| 原票引用 | `invoice_receipt_references` | 复用 `invoice_line_references` 冒充 |
| 未收余额 | `ReceiptRemainingForInvoice` | 第二套公式 |
| 打印 | `print.BuildPayload` + `RenderESCPOS` | handler 拼 ESC/POS |

## 6. API

### `POST /local/v1/fiscal-documents/{documentId}/receipts`

Path：`documentId` = 原 **FT** `invoices.id`。

| 字段 | 必填 | 说明 |
|------|------|------|
| request_id | 是 | 幂等键 |
| operator_id | 是（会话） | |
| station_id | 否 | |
| amount | 条件 | `receive_full=false` 时必填；正数 money |
| receive_full | 否 | `true` = 收剩余全部未收 |
| payment_method | 否 | 默认 CASH；**禁止 ACCOUNT**（收据本身是实收） |
| reason | 否 | 备注；空则默认 `Recibo` |

**错误码：** `not_found`、`validation_failed`、`receipt_not_allowed`、`receipt_amount_exceeded`、`series_missing`、`idempotency_conflict`。

## 7. 库表

见 `migrations/012_rg_receipt.sql`：

- `invoices.received_gross_total`
- `invoice_receipt_references`（receipt_invoice_id → original FT）

## 8. 回归

- `go test ./internal/fiscal/store/ ./internal/fiscal/service/ ./internal/fiscal/saft/ ./internal/fiscal/print/ ...`
- `node scripts/fiscal-rg-regression.mjs`

## 9. 修订

| 日期 | 说明 |
|------|------|
| 2026-09-24 | 初稿定稿并落地 |
