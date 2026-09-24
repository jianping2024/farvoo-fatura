# 点餐侧经 bill_sync 开票（auto_issue）

> **状态：定稿**  
> **权威：是**（Farvoo「打印发票」→ Agent 出税票；本仓实现以本文 + `IssueFromBillDraft` 为准）  
> **对应实现：** `service.IngestBillSyncJob` / `ProcessBillSyncJob`；`billsync.PullAndIngest`；Admin 手动路径仍为 `POST /local/v1/bill-drafts/{id}/issue`  
> **挂单契约（Restaurant）：** restaurant-ordering `docs/technical/farvoo-fiscal-bill-sync-api.zh.md`  
> **本机工作台：** [`fiscal-bill-draft-workbench.zh.md`](fiscal-bill-draft-workbench.zh.md)

---

## 1. 一句话

Farvoo 结账「打印发票」复用云端 `bill_sync_jobs` 管道；Agent 拉取后若载荷 `auto_issue=true`，在写入收银账单草稿后**自动** `IssueFromBillDraft` 并出纸；ack 成功可带回 `invoice_no`。浏览器**不**直连 Local `/issue`。

---

## 2. 流程

```text
Farvoo 组装载荷（含 auto_issue + 购方/付款/document_type）
  → 云端 bill_sync_jobs pending
  → Agent PullAndIngest → service.IngestBillSyncJob
       → UpsertBillDraftOpen（+ products）
       → auto_issue → IssueFromBillDraft → 出纸
  → ack succeeded { invoice_no? }
```

无 `auto_issue`（或 `false`）：行为与旧同步关台相同——只落草稿，店员在 Admin 手动签发。

---

## 3. 载荷字段（相对 bill-sync 快照扩展）

| 字段 | 必填（auto_issue） | 说明 |
|------|-------------------|------|
| `auto_issue` | ✓（为 true） | 否则仅 ingest |
| `document_type` | ✓ | **`FT` \| `FS`**。**由 Farvoo 显式传入**（产品约定：现金→`FS`，其它付款→`FT`）。Agent **按入队字段签发，禁止**再按 `payment_method` 推导类型 |
| `customer_nif` | 否 | 空 → 既有 `ApplyCustomerOverride` 散客 `999999990` |
| `customer_name` | 否 | 配合 NIF；空 NIF 时忽略 |
| `payment_method` | 否 | 空 → `CASH`（既有 `ApplyPaymentOverride`） |
| `issue_mode` | 否 | `whole_table` \| `person`；空则由 `scope_type` 推导 |
| `issue_scope_id` / `scope_id` | person 时 ✓ | 按人开票的稳定 UUID |

### 模式定法

| `scope_type` + auto_issue | 行为 |
|---------------------------|------|
| `whole_table` | `IssueFromBillDraft` `mode=whole_table` |
| `split` + person scope | ingest 已从 `splits` 种 allocation；`mode=person` 对该 `scope_id` 签发。同一 sale 后续人：复用**未删** open 草稿（不覆盖 allocation），不再走 `already_invoiced` 挡死 |

Admin 手动签发路径不变；与 auto_issue 共用 `IssueFromBillDraft`。

---

## 4. Agent 本机默认（无 Admin 会话）

| 项 | 定法 |
|----|------|
| 操作员 | `ListOperatorsForLogin` 第一名（角色序） |
| 打印档口 | `taxpayer_settings.local_default_station_id`（未设 → ack `station_required`） |
| 终端 | `LoopbackFiscalTerminalID` |

---

## 5. Ack

| status | 字段 |
|--------|------|
| `succeeded` | 可选 `invoice_no`（auto_issue 成功或幂等命中时） |
| `failed` | `error_code` / `error_message`（含 `document_type required for auto_issue`、`already_invoiced`、`station_required` 等） |

Restaurant 云端 ack 路由可先忽略未知字段；`invoice_no` 落库属 Farvoo 侧后续刀。

---

## 6. 非目标

- Farvoo 浏览器直 POST Agent Local API  
- Agent 按付款方式推导 `document_type`（已否决；由 Farvoo 传入）  
- 删除 Admin 收银账单手动签发  
