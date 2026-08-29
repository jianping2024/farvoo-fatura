# Fiscal Agent Local API 契约（M4）

> **状态：定稿**（M4 D4.1）  
> **权威：是**（本仓 Local API 请求/响应形状；业务规则见对接说明与 schema）  
> **对应实现：** `apps/fiscal-agent/internal/fiscal/api`  
> **写作规范：** [`design-doc-standards.zh.md`](design-doc-standards.zh.md)

依据：需求 v0.17、`restaurant-ordering` `docs/technical/farvoo-fiscal-agent-integration.zh.md`、本仓 [`fiscal-dev-plan.zh.md`](fiscal-dev-plan.zh.md) M4。

冲突时：DDL / schema 定列；本文定 HTTP 形状；对接说明定 Farvoo 侧职责。

---

## 1. 基址与健康

| 项 | 定法 |
|----|------|
| 前缀 | `/local/v1` |
| 默认绑定 | `127.0.0.1:17880`（`FISCAL_BIND`） |
| 健康 | `GET /local/v1/health` → `{"status":"ok","module":"fiscal"}` |

前端 **不得**上传 `InvoiceNo` / Hash / ATCUD / QR / 系列序号。

---

## 2. §13 鉴权（P0 子集 · D4.2）

| 模式 `FISCAL_AUTH_MODE` | 行为 |
|-------------------------|------|
| `loopback_trust`（默认） | 对端为 loopback（`127.0.0.1` / `::1`）时 **不强制**凭证；非 loopback **必须**终端 + `operator_token` |
| `required` | **一律**要求终端 + `operator_token`（回归 / LAN 门禁） |

### 2.1 请求头（鉴权开启时）

| 头 | 说明 |
|----|------|
| `X-Fiscal-Terminal-Id` | 已配对开票终端 id（表 `issue_terminals`） |
| `X-Fiscal-Terminal-Secret` | 配对时明文；库内仅存 SHA-256 hex |
| `X-Fiscal-Operator-Token` 或 `Authorization: Bearer …` | Farvoo 签发的短时 JWT（P0：HS256 + `FISCAL_OPERATOR_JWT_SECRET`；以后可换非对称验签，**验签入口唯一** `auth.VerifyOperatorToken`） |

`operator_token` claims（至少）：`mesa_user_id`、`terminal_id`（须与终端头一致）、`role` ∈ owner/frontdesk/cashier、`exp`（建议 15 分钟）、`store_id` 或 `restaurant_id`（与 Agent `store_id` 一致）。

Agent：验终端 → 验 token → `EnsureOperatorFromMesa` → `SourceID`/`operator_id` = 本地 `operators.id`。**鉴权通过后 body 内 `operator_id` 被忽略。**

### 2.2 覆盖路由（必须挂鉴权）

- `POST /local/v1/fiscal-documents`
- `POST /local/v1/fiscal-documents/manual`
- `POST /local/v1/bill-drafts/{id}/issue`

唯一 HTTP 包装：`api.withIssueAuth` → `auth.AuthenticateIssue`。

### 2.3 配对终端

```http
PUT /local/v1/setup/issue-terminal
Content-Type: application/json

{
  "id": "term-cashier-2",
  "store_id": "store-…",
  "display_name": "2号收银",
  "secret": "once-plaintext",
  "station_id": "optional-print-station-uuid"
}
```

唯一写路径：`store.UpsertIssueTerminal`。

---

## 3. 签发 FT（Farvoo / API）

```http
POST /local/v1/fiscal-documents
Content-Type: application/json

{
  "store_id": "store-…",
  "request_id": "uuid-or-stable-key",
  "operator_id": "op-…",
  "station_id": "print-station-uuid",
  "document_type": "FT",
  "snapshot": {
    "source_system": "farvoo",
    "source_sale_id": "bill_splits.id",
    "scope_type": "whole_table",
    "scope_id": "bill_splits.id",
    "fiscal_purpose": "sale",
    "lines": [
      {
        "product_code": "006",
        "display_name": "CERVEJA",
        "saft_name": "CERVEJA",
        "quantity": "1",
        "unit_price_gross": "2.25",
        "vat_rate": "0.23",
        "product_type": "P",
        "unit_of_measure": "UN"
      }
    ],
    "customer": {
      "tax_id": "999999990",
      "company_name": "Consumidor Final",
      "country": "PT"
    },
    "payments": [{ "method": "CASH", "amount": "2.25" }]
  }
}
```

成功响应：

```json
{
  "document_id": "…",
  "invoice_no": "FT FT2026…/1",
  "atcud": "…",
  "document_type": "FT",
  "document_status": "SIGNED",
  "print_job_id": "…",
  "print_status": "PENDING",
  "issued_at": "2026-08-29T14:00:00Z",
  "idempotent_hit": false
}
```

唯一写路径：`service.IssueDocument` → `store.IssueFT`（同事务写 `local_print_jobs` ORIGINAL + `sync_outbox` `INVOICE_ISSUED`）。

幂等：`store_id+request_id`；业务键 `store_id+source_system+source_sale_id+scope_type+scope_id+fiscal_purpose`。

---

## 4. 收银账单开票

见 [`fiscal-bill-draft-workbench.zh.md`](fiscal-bill-draft-workbench.zh.md)。

```http
POST /local/v1/bill-drafts/{draftId}/issue
```

唯一路径：`IssueFromBillDraft` → `IssueDocument` → `IssueFT`。

---

## 5. sync_outbox → 云端副本（D4.4）

| 项 | 定法 |
|----|------|
| 入队 | **仅** `store.EnqueueInvoiceIssuedTx`，在 `IssueFT` 成功路径、`COMMIT` 前 |
| 幂等命中 | **不**再入队 |
| 冲刷 | `sync.Worker` 认领 PENDING → `POST {FARVOO_API}/api/print-agent/fiscal-invoice-copies`（`Authorization: Bearer {FARVOO_JWT}`） |
| 未配置 Farvoo | 保持 PENDING，**不**烧 attempts |
| 失败 | 指数退避重试；超限 → `FAILED`；**不**回滚本地票 |

payload（无私钥）字段：`event_type=INVOICE_ISSUED`、`document_id`、`invoice_no`、`atcud`、`document_status`、`store_id`、`source_*`、`scope_*`、`fiscal_purpose`、`gross_total`、`issued_at`、`print_job_id`、`print_status`。

Farvoo 云端接口可先 stub；Agent 侧契约以本文为准。

---

## 6. 其它常用路由（摘要）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/local/v1/fiscal-documents/by-request/{requestId}` | 按 request 查票 |
| POST | `/local/v1/fiscal-documents/{id}/reprints` | 重打（不重签） |
| GET | `/local/v1/print-jobs/{id}` | 打印状态 |
| GET/POST | `/local/v1/products` `/local/v1/customers` | LOCAL 主档 |
| POST | `/local/v1/fiscal-documents/manual` | 手工开票 |

---

## 修订记录

| 日期 | 变更 |
|------|------|
| 2026-08-29 | M4 D4.1 首版：签发/§13 P0/outbox 契约冻结 |
