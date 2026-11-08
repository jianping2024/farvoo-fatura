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
| XSD 在线校验工具链 | 不做在线校验；本地/CI 用官方 XSD 1.1 离线校验（`scripts/fiscal-saft-xsd-check.sh`） |

## 3. P0 定法

| 项 | 定法 |
|----|------|
| 期间边界 | 任意闭区间 `[start_date, end_date]`（`YYYY-MM-DD`），必须在同一财年内；自然月是区间的特例（API 仍接受 `year`+`month` 简写） |
| 票筛选列 | **`invoices.invoice_date`** 落在上述闭区间 |
| 含票类型 | 该月内所有 **已签** 本地票（P0：**FT + NC**；后续 FS/FR 等同理） |
| 空月 | **拒绝**导出，API `409` + `error=no_invoices`；**不**写空 XML |
| 重复导出 | **追加**新 `saft_exports` 行；**不** UPDATE 历史行 |
| 文件路径 | `{parent(FISCAL_DATA_DIR)}/saft/{nif}/{year}/saft_{nif}_{start}_{end}_{exportID前8位}.xml`；**每次导出一份不可变文件**，重导出不覆盖历史文件，`file_sha256` 永远对应本行文件 |
| 编码 | Windows-1252，无 BOM |
| 行金额口径 | 行 `line_net`（**未含税**，2 位）；Portaria 302/2016 4.1.4.20.6/7 |
| 行借贷方向 | **NC → `DebitAmount`**；FT/FS/FR/ND、PF、GR/GT → `CreditAmount`（唯一规则 `saft.lineIsDebit`） |
| NC References | 来自 `invoice_line_references`，XSD 位置在 `TaxPointDate` 与 `Description` 之间 |
| SettlementAmount | 不输出（系统无行折扣） |
| UnitPrice | 行 `unit_price_net`，6 位（`compliance.UnitPriceDecimals`，开票时 `LineFromGross` 唯一写入）；2026-11 之前的历史行为 2 位 |
| 控制合计 | SalesInvoices / WorkingDocuments `TotalDebit`/`TotalCredit` = Σ 对应侧行金额（PF 排除 `A`） |
| 0% 税率行 | 必须带 `TaxExemptionReason` + `TaxExemptionCode`；缺失 → `INVALID` |
| validation_status | `VALID` / `INVALID` = **builder 层检查**（成品 XML 每个元素可编码 Windows-1252——`saft.encodingErrors`，错误写明单据号 + 元素 + 原文；0% 行缺免税原因；ProductDescription < 2 字符）。**不是** XSD 校验：Agent 端无 XSD 1.1 引擎，XSD 结构由 CI 门禁 `scripts/fiscal-saft-xsd-check.sh` 覆盖全票种样本保证 |
| Header 软件身份 | `TaxAccountingBasis=F`；`ProductCompanyTaxID` / `ProductID`（`Farvoo/FAINA VAIDOSA, LDA`）/ `SoftwareCertificateNumber` 来自 `internal/fiscal/software`（唯一来源，签发与 AT 系列注册同读）；`ProductVersion` = `VERSION`（release 经 ldflags；dev 构建为 `dev`）；`DateCreated` = 导出当天（门店时区）；`TaxEntity=Global`（一个 NIF 一家店） |
| 文本上限 | `domain` 的 SAF-T 上限表为唯一来源；录入时由 `domain.ValidateSAFTText` 统一校验（见 §3.1），导出 `saft.writeText` 按同表截断（只为兜底已签历史单据） |
| SourceID | 单据层 = 创建人（`invoices.source_id`）；`DocumentStatus/SourceID` = 当前状态操作人（`invoices.status_source_id`，迁移 019）；均输出 `operators.saft_user_code` |
| MovementOfGoods 合计 | `NumberOfMovementLines` 含作废（A）单据行（Portaria 4.2.1）；`TotalQuantityIssued` 不含 A（4.2.2） |
| 审计 | 每次成功导出写 `audit_log.action = EXPORT_SAFT` |

### 3.1 录入时的 SAF-T 文本校验（签发前拦截）

已签单据不可改，所以 SAF-T 文本必须在**录入和签发前**合格；导出端只负责如实报告历史数据。

**唯一规则** `domain.ValidateSAFTText(field, s, min, max)`：NFC 规范化（macOS 输入的「a + 组合波浪号」→「ã」）+ 去首尾空格 → 必须可编码 Windows-1252（`domain.EncodesWindows1252`，导出端同用）→ 长度 `min..max` 字符（`min=0` 表示可空）。返回清洗后的值，存的就是校验过的值。失败返回 `*domain.SAFTTextError{Field, Reason, Min, Max}`，`Reason` ∈ `encoding` / `too_short` / `too_long`。

| 入口 | 字段（field 路径） | 上限 |
|------|--------------------|------|
| 商品主档 `store.CleanProductText`（本地保存 + 云同步 + bill sync ingest 预检） | `product.product_code` / `product.saft_name`（空则用 display_name） | 1–60 / 2–200 |
| 客户主档 `store.UpsertLocalCustomer` | `customer.company_name` / `address_detail` / `city` / `postal_code` | 0–100 / 210 / 50 / 20 |
| 门店资料 `service.UpsertTaxpayer` | `taxpayer.legal_name` / `business_name` / `address_detail` / `city` / `postal_code` | 100 / 60 / 210 / 50 / 20 |
| 新单签发 `service.validateSaleText`（FT/FS/FR、PF、GR、GT 四个 service 入口，在签名、占号、AT 之前） | `customer.*`；`lines.<n>.saft_name`（`SaleLine.EffectiveSaftName`）/ `lines.<n>.product_code`（空 = 系统编 LINEn） | 同上 |
| GR/GT 装卸货地 `service.resolveShipPoints` | `ship_from.*` / `ship_to.*` | 210 / 50 / 20 |
| 作废 / NC / ND 原因 `domain.ValidateStatusReason` | `reason` | 1–50 |

NC/ND/RG 沿用原单的客户与商品行，不重复校验。

**错误出口（唯一）**：HTTP `400 {error: "saft_text_invalid", field, part, min, max}`（`api.writeSAFTTextErr`）；bill sync ack code `saft_text_invalid`（`billsync.CodeSAFTTextInvalid`，ingest 在存草稿前拒绝整单）。Admin：`FiscalUI.formatError` 生成三语提示（订单行带「第 n 行」与「请在商品资料里填写「葡语名称」」）；`focusSAFTTextField` 按 field 标红并聚焦对应输入框或订单行。

**发版前扫描**：`go run ./cmd/saft-text-scan <fiscal.db>`（同一规则，只读）列出需先修的主档（fix）与已签单据中的不合格文本（signed）。

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

## 5. 期间边界

`service.ExportSAFT` 只收 `start_date`..`end_date`（含首尾，同一财年，`start ≤ end`），筛选 `invoice_date` 在此闭区间内的已签单据；Header `FiscalYear` 取 `start_date` 年份。

`year`+`month` 简写只在 API 层经 `store.SAFTPeriodBounds` 展开为该自然月首尾日，例：`2026-08` → `2026-08-01` … `2026-08-31`。

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
| start_date / end_date | 二选一 | `YYYY-MM-DD`，同一财年 |
| year / month | 二选一 | 自然月简写（≥2000 / 1–12） |
| operator_id | 否 | 写入 `created_by` |
| store_id | 否 | 默认进程 `FISCAL_STORE_ID` |

**成功 200：** `export_id`、`file_name`、`file_path`、`file_sha256`、`invoice_count`、`total_*`、`validation_status`、`validation_errors?`

**错误：** `validation_failed`（期间非法或跨财年）、`no_invoices`（空月）、`taxpayer_missing`

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
| Header.FiscalYear / StartDate / EndDate | 请求区间（`FiscalYear` = 起始年） |
| MasterFiles.Customer | 票上客户快照经 `saft.saftCustomer` 去重：有 NIF → `CustomerID=NIF`；无 NIF 的 Consumidor Final → `CF`（地址 Desconhecido）；无 NIF 署名收货方 → `C`+名称地址哈希 12 位；`CustomerTaxID` 无 NIF 时 = 999999990。各单据 `CustomerID` 用同一函数 |
| SourceID（全部单据） | `operators.saft_user_code`（迁移 018，门店内 `U001`…，触发器分配；XSD ≤30 字符）；`invoices.source_id` 仍为 operators.id |
| MasterFiles.Product | 行 `product_code` / `saft_name` 去重 |
| MasterFiles.TaxTable | 一个 `TaxTable`，每个行 `vat_rate` 去重为一条 `TaxTableEntry` |
| SourceDocuments.SalesInvoices.Invoice | 每张 FT/NC 一条 |
| Invoice.DocumentNumber | `invoice_no` |
| Invoice.ATCUD | `atcud` |
| Invoice.DocumentStatus | 复合节点：`InvoiceStatus=N`、`InvoiceStatusDate=system_entry_date`、`SourceID`、`SourceBilling=P`（销售单无作废路径；NC/ND 冲销只改原票业务状态） |
| Line.CreditAmount / DebitAmount | `line_net`；FT 等贷方 / NC 借方 |
| Line.Tax 后 | 0% 行：`TaxExemptionReason` / `TaxExemptionCode` |
| SourceDocuments 顺序 | SalesInvoices → MovementOfGoods → WorkingDocuments → Payments（XSD 顺序） |
| Payment | `TransactionDate`、`PaymentType` 在 `DocumentStatus` 前；`DocumentStatus.SourcePayment=P`；行金额 = 收款额（RG 无 Tax） |
| Line.References | NC：`invoice_line_references` → 原票号 + 原因 |

## 9. Admin §7

设置页 **「7. SAF-T」**：日期区间（`FiscalUI.createDateRangeFilter`，默认「本月」，可自由选起止）、**导出 SAF-T**、**刷新列表**；列表展示起止日期、`validation_status`、`invoice_count`、每次导出各自的下载链接。

## 10. 回归与 UAT

| 脚本 | 场景 |
|------|------|
| `scripts/fiscal-m5-regression.mjs` | setup → FT → NC → export → sqlite → list → download → NC ref → 空月拒绝 → 重复导出新行 → audit |
| `go test ./internal/fiscal/saft/...` | FT+NC 结构化 XML 断言；`TestBuildLineAmountsNetAndSided`：Σ行=NetTotal、Net+Tax=Gross、借贷方向、Qty×UP≈行金额 |
| `go test ./internal/fiscal/domain/ ./internal/fiscal/service/ -run SAFTText` | 录入规则（重音 / € / NFD / 中文 / emoji / 2 与 200 边界）、四类入口拒绝、被拒不占号、GR 不触 AT |
| `scripts/fiscal-saft-xsd-check.sh` | 全票种样本（FT/NC/PF/GR/GT/RG/0% 免税）对官方 XSD 1.1 校验；接入 `check-fiscal-agent.sh` 与 CI |
| Admin §7（Chrome DevTools） | 导出、列表 VALID、下载 |

## 11. 验收对照（dev-plan M5）

1. InvoiceNo / ATCUD / Hash / GrossTotal 与本地库一致 — 回归 + `saft/build_test.go`  
2. 同月 FT + NC 均出现，NC References 与 `invoice_line_references` 一致 — 回归 `saft-xml-nc-ref`  
3. 无票月份拒绝 — `no_invoices`（year=2000, month=1 夹具）  
4. 重复导出不破坏历史行 — 同月第二次 POST 产生第二行 `saft_exports`
