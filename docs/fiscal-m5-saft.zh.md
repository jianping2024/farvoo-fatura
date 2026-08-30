# M5：SAF-T 月报导出

> **状态：定稿**  
> **权威：是**（M5 行为与 API；库列仍以 [`fiscal-sqlite-schema.zh.md`](fiscal-sqlite-schema.zh.md) + `migrations/001_init.sql` 为准）  
> **对应实现：** M5 已落地（`store.LoadSAFTInvoicesForPeriod`、`saft.Build`、`service.ExportSAFT`、Admin §7、`scripts/fiscal-m5-regression.mjs`）  
> **计划：** [`fiscal-dev-plan.zh.md`](fiscal-dev-plan.zh.md) M5  
> **票库：** 全部已签发票仅本地 SQLite 权威；**不对云同步**；对外合规出口 **只有** 本里程碑 SAF-T 月导（schema §1.1）

## 1. 目标

按月从本地 SQLite 生成 **SAF-T(PT) 1.04_01** XML；归档 `saft_exports`；Admin 可选期间导出、列表、下载；校验文本可编码 Windows-1252；同月 **FT + NC** 均进入同一文件。

## 2. 非目标

| 项 | 说明 |
|----|------|
| 向云推送已签发票副本 | 禁止；`sync_outbox` P0 不写入 |
| Dashboard / Ops 票证查询 | 不要求云上有票 |
| e-Fatura 逐票实时上报 | 不做 |
| AT 在线提交 SAF-T | P0 仅本地文件 + 归档行 |
| XSD 在线校验工具链 | P0 用结构化断言 + Windows-1252 自检 |

## 3. P0 定法

| 项 | 定法 |
|----|------|
| 期间边界 | 自然月；按门店 `taxpayer_settings.timezone` 计算 `[start_date, end_date]`（含首尾日，格式 `YYYY-MM-DD`） |
| 票筛选列 | **`invoices.invoice_date`** 落在上述闭区间 |
| 含票类型 | 该月内所有 **已签** 本地票（P0：**FT + NC**；后续 FS/FR 等同理） |
| 空月 | **拒绝**导出，API `409` + `error=no_invoices`；**不**写空 XML |
| 重复导出 | **追加**新 `saft_exports` 行；**不** UPDATE 历史行 |
| 文件路径 | `{parent(FISCAL_DATA_DIR)}/saft/{nif}/{year}/saft_{nif}_{year}-{MM}.xml` |
| 编码 | Windows-1252，无 BOM |
| FT 行金额 | `DebitAmount` = 行 `gross` |
| NC 行金额 | `CreditAmount` = 行 `gross`；`References` 来自 `invoice_line_references` |
| validation_status | `VALID` / `INVALID`（Windows-1252 不可编码字符 → `INVALID` + `validation_errors` JSON） |
| 审计 | 每次成功导出写 `audit_log.action = EXPORT_SAFT` |

## 4. 唯一路径

```text
Admin §7 exportSAFT() / Local API
  → service.ExportSAFT（唯一编排）
    → store.LoadSAFTInvoicesForPeriod（唯一读票）
    → saft.Build（唯一 XML 构建）
    → 写文件 + store.InsertSAFTExport（唯一归档写）
    → store.InsertAuditLog（EXPORT_SAFT）
```

| 层 | 唯一入口 | 禁止 |
|----|----------|------|
| 读票 | `store.LoadSAFTInvoicesForPeriod` | handler / Admin 直查 invoices 拼 XML |
| XML | `saft.Build` | 第二套 builder |
| 编排 | `service.ExportSAFT` | API handler 内联写文件 |
| 归档 | `store.InsertSAFTExport` | UPDATE 旧 export 行 |
| Admin UI | `exportSAFT()` | 其它按钮直 POST 不同 body |

## 5. 期间边界（`store.SAFTPeriodBounds`）

输入：`year`、`month`（1–12）、`timezone`（IANA，如 `Europe/Lisbon`）。

输出：`start_date = YYYY-MM-01`，`end_date = 该月最后一天`（均在门店时区语义下对应自然月）。

**示例：** `2026-08` + `Europe/Lisbon` → `2026-08-01` … `2026-08-31`；筛选 `invoice_date` 在此闭区间内的已签票。

## 6. `saft_exports` 列（与 schema 一致）

| 列 | 类型 | 必填 | 说明 |
|----|------|------|------|
| id | TEXT PK | 是 | UUID |
| store_id | TEXT | 是 | |
| taxpayer_nif | TEXT | 是 | 导出时纳税人 NIF |
| period_year | INTEGER | 是 | |
| period_month | INTEGER | 是 | 1–12 |
| start_date | TEXT | 是 | 期间起 |
| end_date | TEXT | 是 | 期间止 |
| file_name | TEXT | 是 | 如 `saft_517535009_2026-08.xml` |
| file_path | TEXT | 否 | 绝对路径 |
| file_sha256 | TEXT | 否 | 文件 SHA-256 hex |
| invoice_count | INTEGER | 是 | 默认 0 |
| total_net | TEXT | 否 | 汇总 |
| total_tax | TEXT | 否 | 汇总 |
| total_gross | TEXT | 否 | 汇总 |
| validation_status | TEXT | 是 | `VALID` / `INVALID` |
| validation_errors | TEXT | 否 | JSON 数组字符串 |
| created_by | TEXT | 否 | `operator_id` |
| created_at | TEXT | 是 | UTC ISO8601 |
| submitted_at | TEXT | 否 | P0 不用 |
| at_receipt_number | TEXT | 否 | P0 不用 |
| at_receipt_file_path | TEXT | 否 | P0 不用 |

## 7. Local API

### 7.1 `POST /local/v1/saft/exports`

**Body：**

| 字段 | 必填 | 说明 |
|------|------|------|
| year | 是 | ≥ 2000 |
| month | 是 | 1–12 |
| operator_id | 否 | 写入 `created_by` |
| store_id | 否 | 默认进程 `FISCAL_STORE_ID` |

**成功 200：** `export_id`、`file_name`、`file_path`、`file_sha256`、`invoice_count`、`total_*`、`validation_status`、`validation_errors?`

**错误：** `validation_failed`（年月非法）、`no_invoices`（空月）、`taxpayer_missing`

### 7.2 `GET /local/v1/saft/exports?year=&month=`

返回 `{ "exports": [ …SAFTExportRow ] }`，按 `created_at` 降序。

### 7.3 `GET /local/v1/saft/exports/{exportId}`

单条归档元数据。

### 7.4 `GET /local/v1/saft/exports/{exportId}/download`

`Content-Type: application/xml; charset=windows-1252`；附件文件名同 `file_name`。

## 8. XML 节点映射（P0 _subset）

| SAF-T 节点 | 来源 |
|------------|------|
| Header.CompanyID / TaxRegistrationNumber | `taxpayer_settings` |
| Header.FiscalYear / StartDate / EndDate | 请求年月 + `SAFTPeriodBounds` |
| MasterFiles.Customer | 票上客户快照去重 |
| MasterFiles.Product | 行 `product_code` / `saft_name` 去重 |
| MasterFiles.TaxTable | 行 `vat_rate` 去重 |
| SourceDocuments.SalesInvoices.Invoice | 每张 FT/NC 一条 |
| Invoice.DocumentNumber | `invoice_no` |
| Invoice.ATCUD | `atcud` |
| Invoice.DocumentStatus | `document_status` |
| Line.DebitAmount / CreditAmount | FT 借方 / NC 贷方 |
| Line.References | NC：`invoice_line_references` → 原票号 + 原因 |

## 9. Admin §7

设置页 **「7. SAF-T 月导」**：年/月输入、**导出 SAF-T**、**刷新列表**；列表展示 `validation_status`、`invoice_count`、下载链接。

## 10. 回归与 UAT

| 脚本 | 场景 |
|------|------|
| `scripts/fiscal-m5-regression.mjs` | setup → FT → NC → export → sqlite → list → download → NC ref → 空月拒绝 → 重复导出新行 → audit |
| `go test ./internal/fiscal/saft/...` | FT+NC 结构化 XML 断言 |
| Admin §7（Chrome DevTools） | 导出、列表 VALID、下载 |

## 11. 验收对照（dev-plan M5）

1. InvoiceNo / ATCUD / Hash / GrossTotal 与本地库一致 — 回归 + `saft/build_test.go`  
2. 同月 FT + NC 均出现，NC References 与 `invoice_line_references` 一致 — 回归 `saft-xml-nc-ref`  
3. 无票月份拒绝 — `no_invoices`（year=2000, month=1 夹具）  
4. 重复导出不破坏历史行 — 同月第二次 POST 产生第二行 `saft_exports`
