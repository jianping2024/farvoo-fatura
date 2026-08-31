# M6：FS / FR / ND + 认证扫尾（D6.1）

> **状态：定稿**  
> **权威：是**（M6 D6.1 行为与 API；库列仍以 [`fiscal-sqlite-schema.zh.md`](fiscal-sqlite-schema.zh.md) + `migrations/*.sql` 为准）  
> **对应实现：** M6 D6.1 已落地（`IssueDocument` FT/FS/FR、`store.IssueND`、`service.IssueDebitNote`、Admin 系列与借记）  
> **计划：** [`fiscal-dev-plan.zh.md`](fiscal-dev-plan.zh.md) M6  
> **后续：** D6.2–D6.4（认证清单、备份恢复、换机）另里程碑细化

## 1. 目标（D6.1）

| 项 | 定法 |
|----|------|
| FS / FR 签发 | 与 FT 同一路径：`service.IssueDocument` → `store.IssueFT`；`document_type` 可选 `FT` / `FS` / `FR`；各自 **ACTIVE** 系列 |
| ND 借记 | 唯一写路径：`service.IssueDebitNote` → `store.IssueND`；API `POST /local/v1/fiscal-documents/{id}/debit-notes` |
| 可借记原票 | **FT / FS / FR**；状态 `SIGNED` 或 `DEBITED_PARTIAL` |
| 累计借记 | 原票 `debited_gross_total`；满额 → `DEBITED_FULL` |
| 权限 | P0 复用 `operators.can_issue_nc`（Admin 文案「NC / ND」） |
| SAF-T | 同月 **FT + NC + ND + FS + FR** 进入同一 XML；ND 行引用同 NC（`invoice_line_references`） |

## 2. API

### 2.1 签发 FS / FR

`POST /local/v1/fiscal-documents` 或 `POST /local/v1/fiscal-documents/manual`（`document_type`: `FS` / `FR`）。

### 2.2 借记 ND

```http
POST /local/v1/fiscal-documents/{documentId}/debit-notes
```

| 字段 | 说明 |
|------|------|
| request_id | 幂等键 |
| operator_id | 操作员 |
| station_id | 打印档口 |
| reason | 1–200 字 |
| debit_full | 全额借记 |
| lines | 部分借记行（同 NC `lines` 结构） |

错误码：`debit_not_allowed`、`debit_amount_exceeded`、`series_missing`（无 ND 系列）。

## 3. 迁移

`migrations/003_debited_gross.sql`：`invoices.debited_gross_total TEXT NOT NULL DEFAULT '0.00'`。

## 4. 回归

- `go test ./internal/fiscal/store/...`（`issue_m6_test.go`）
- `node scripts/fiscal-m6-regression.mjs`（FS/FR/ND/NC + SAF-T 五类型）

## 5. 非目标（本里程碑）

- D6.2 认证检查清单文档与自动报告
- D6.3 备份/恢复校验工具
- D6.4 换机最小流程
