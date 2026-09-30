# PF：形式发票（Fatura pró-forma）

> **状态：定稿**（已落地）  
> **权威：是**（PF 行为与 API；库列以本文 + [`fiscal-sqlite-schema.zh.md`](fiscal-sqlite-schema.zh.md) + `migrations/013_pf_proforma.sql` 为准）  
> **对应实现：** `store.IssuePF`、`store.AnnulPF`、`store.linkProformaInTx`（仅 `IssueFT` 内）、`service.IssueProforma` / `AnnulProforma`、`POST .../manual`（PF）、`POST .../annul`、Admin PF Tab / 开真票 / 作废  
> **测试场景：** [`fiscal-doc-type-test-scenarios.zh.md`](fiscal-doc-type-test-scenarios.zh.md) §8；回归 `scripts/fiscal-pf-regression.mjs`  
> **计划顺序：** RG → **PF** → GR → GT  
> **法律依据（摘要）：** DL 28/2019（税务相关单据）；Portaria 195/2020（ATCUD）；Portaria 363/2010 art.7（签名）；Portaria 302/2016 表 4.3 WorkingDocuments（`WorkType=PF`，`WorkStatus` = N/A/F）

## 1. 目标

签发 **PF**（报价/形式发票）：独立 PF 系列与 Hash 链；票面不可误认为 Fatura；进 SAF-T **WorkingDocuments**（不进 SalesInvoices）；客户确认后可 **可选** 关联并开出 FT/FS/FR；支持作废；幂等。

## 2. 非目标

| 项 | 说明 |
|----|------|
| 用 PF 替代 FT/FS/FR | 禁止 |
| 堂食 / 收银账单 / Farvoo 同步强制或插入 PF | P0 不做 |
| PF 上登记收款 / 定金 | 禁止（定金开 FT/FR） |
| 对 PF 开 NC / ND / RG | 禁止 |
| 行级「已开票金额」进度 | P0 不做（仅单据级 `N`/`A`/`F`） |
| OR / NE / CM 等其它 WorkingDocuments | 不做 |
| 过期自动作废 / 自动禁止转真票 | 不做（`valid_until` 仅展示） |
| 业态门闸（餐馆/商超） | 不做；有 ACTIVE PF 系列即可开 |

## 3. P0 定法

| 项 | 定法 |
|----|------|
| 性质 | 税务相关 **conferência** 单据；**无**纳税义务；**非**销售税票 |
| 入口 | **仅手工**（Admin 手工开票流）；不接收银账单 |
| 系列 | 独立 `series.document_type = 'PF'`；须 AT validation_code（与其它系列同注册路径） |
| 门闸 | **有** ACTIVE PF 系列 → 可开 PF；**无** PF 系列 **不**阻挡 FT/FS 的 `ready_to_issue` |
| 合规要素 | ATCUD、Hash（RSA-SHA1 链）、QR、票面 **Pró-forma** +「Este documento não serve de fatura」 |
| SAF-T | 表 4.3，`WorkType=PF`；`WorkStatus` 见 §4；**不**进 4.1 SalesInvoices |
| 转真票 | **可选**；无 PF 也可直接开 FT/FS/FR |
| 关联方向 | 1 张 PF → **多张** 真票；1 张真票 → **最多 1** 张 PF |
| 标 `F` | FT / FS / **FR** 任一签发时挂上该 PF → PF `WorkStatus=F`（含部分开票） |
| 金额 | 真票与 PF **不必**相等 |
| 带入 | 主路径：PF 详情「开真票」→ 带入客户/行/金额（可改）并自动挂 PF；辅路径：手工开真票时可选挂一张 PF |
| 补挂 / 解挂 | 真票 **签后禁止** 补挂或解挂 PF |
| 客户 / NIF | **与现网手工 FT 同规则** |
| 有效期 | 可选 `valid_until`（日历日）；**不**因过期自动作废或禁转真票 |
| 币种 | P0 仅 EUR（与现网一致） |
| 权限 | 已登录可开票操作员（与 IssueDocument 同；不要求 `can_issue_nc`） |
| 列表 / 营收 | PF **独立**列表（或 Tab）；**不计入**销售营收汇总 |
| 重打 | 允许（与现网重打门闸同思路；`N`/`F` 可重打，`A` 否） |

## 4. WorkStatus（唯一状态机）

与 Portaria 302/2016 `WorkStatus` 对齐，存于 PF 行的 `invoices.document_status`（**仅** `document_type=PF` 时解释为本表；销售票仍用 `SIGNED` 等）：

| 值 | 含义 | 何时写入 |
|----|------|----------|
| `N` | 正常（未作废、尚未关联真票） | `IssuePF` 签发 |
| `F` | 已开票（**含部分**） | 第一张挂本 PF 的 FT/FS/FR **签发成功**时 |
| `A` | 作废 | `AnnulPF` |

| 规则 | 定法 |
|------|------|
| `N` → `F` | 仅经由真票签发事务挂引用 |
| `N` → `A` | 作废；须 `reason`（→ SAF-T `Reason`，≤50） |
| `F` → `A` | **禁止**（已有真票；纠销售票走 NC 等） |
| `A` → 任意 | **禁止** |
| `A` 被真票引用 | **禁止** |
| 签发后改行/改金额 | **禁止**；须作废后新开 PF |
| `F` 后再挂更多真票 | **允许**（保持 `F`） |

状态变更须记录时间与操作员（→ SAF-T `WorkStatusDate` / `SourceID`）；见 §7。

## 5. 唯一写路径

```text
签发 PF：
  Admin 手工开票 document_type=PF
    → service.IssueProforma（唯一编排）
      → store.IssuePF（唯一 SQLite 事务：系列序号 + Hash + ATCUD + 行 + 客户快照）

作废 PF：
  Admin PF 详情
    → service.AnnulProforma
      → store.AnnulPF（仅 N→A）

开真票并挂 PF：
  A) PF 详情「开真票」→ 预填 IssueManual + proforma_id
  B) 手工开 FT/FS/FR + 可选 proforma_id
    → service.IssueDocument / IssueManualFT（现有销售签发）
      → store.IssueFT 事务内：若 proforma_id 非空则校验并 INSERT 引用；若 PF 仍为 N 则置 F
```

| 层 | 唯一入口 | 禁止 |
|----|----------|------|
| 签发 PF | `service.IssueProforma` → `store.IssuePF` | 复用 `IssueFT` 冒充 PF；handler 直写 SQLite |
| 作废 | `store.AnnulPF` | 物理 DELETE；把 `F` 改回 `N` |
| 引用 | `invoice_proforma_references`（真票 → PF） | 签后 UPDATE 补挂/解挂 |
| 打印 | `print.BuildPayload` + `RenderESCPOS`（PF 文案分支） | 可被改成「Fatura」主标题的布局 |

## 6. API（P0 形状）

### 6.1 签发 PF

`POST /local/v1/fiscal-documents/manual`，`document_type=PF`（扩展现有手工开票；或等价专用路由，**须单一编排入口**）。

| 字段 | 必填 | 说明 |
|------|------|------|
| request_id | 是 | 幂等键 |
| operator_id | 是（会话） | |
| document_type | 是 | `PF` |
| lines | 是 | 与手工 FT 同行结构（含税计价规则同现网） |
| customer | 条件 | **与手工 FT 同规则** |
| valid_until | 否 | `YYYY-MM-DD`；仅展示 |
| station_id | 否 | |

**错误码：** `validation_failed`、`series_missing`、`idempotency_conflict`（及现网手工开票同类码）。

### 6.2 作废 PF

`POST /local/v1/fiscal-documents/{documentId}/annul`

| 字段 | 必填 | 说明 |
|------|------|------|
| request_id | 是 | 幂等 |
| reason | 是 | 非空；截断/校验 ≤50 字符（SAF-T） |

仅 `document_type=PF` 且 `document_status=N`。  
**错误码：** `not_found`、`annul_not_allowed`、`validation_failed`、`idempotency_conflict`。

### 6.3 开真票时挂 PF

在现有 `IssueDocument` / manual 销售签发请求上增加：

| 字段 | 必填 | 说明 |
|------|------|------|
| proforma_id | 否 | 目标 PF 的 `invoices.id` |

校验（失败则整单签发失败）：

1. PF 存在且 `document_type=PF`
2. `document_status` ∈ {`N`,`F`}（**排除** `A`）
3. 本张真票未挂其它 PF（P0 最多 1）
4. 金额/行 **不**与 PF 对齐校验

成功：写入 `invoice_proforma_references`；若 PF 为 `N` → `F`。

### 6.4 读取

- `GET /local/v1/fiscal-documents/{id}`：PF 返回 `work_status`（= `document_status`）、`valid_until`、已关联真票列表（可空）。  
- 真票详情：若有引用，返回 `proforma_id` / `proforma_invoice_no`。  
- 列表：支持 `document_type=PF` 筛选；营收汇总 **排除** PF。

## 7. 库表

见 `migrations/013_pf_proforma.sql`：

- `invoices.status_reason` / `status_changed_at` / `valid_until`
- `invoice_proforma_references`（sale → PF）

## 8. SAF-T / 打印

| 项 | 定法 |
|----|------|
| 导出表 | 4.3 WorkingDocuments；`WorkType=PF` |
| WorkStatus | 导出 `document_status`（N/A/F） |
| OrderReferences | 真票行/单头按 Portaria：写 PF 的类型+系列+序号与日期 |
| SalesInvoices | **不得**把 PF 当发票导出 |
| 打印主标题 | 须含 **Pró-forma**；禁止以「Fatura」作主标题误导 |
| 强制句 | 可见「Este documento não serve de fatura」（或产品等价葡/中双语，葡文不可缺） |

## 9. Admin UI（P0）

| 项 | 定法 |
|----|------|
| 开 PF | 手工开票选类型 PF（有系列才可选） |
| PF 详情 | 展示状态 N/A/F；「开真票」「作废」（仅 N）；「重打」 |
| 开真票 | 预填可改；确认后签 FT/FS/FR 并挂本 PF |
| 手工开销售票 | 可选「关联 PF」；可不选 |
| 列表 | PF 独立 Tab/筛选；营收数字不含 PF |

## 10. 回归

落地后：

- `go test`：store / service / saft / print（含 PF 分支）
- `node scripts/fiscal-pf-regression.mjs`（对齐场景文 PF-01…PF-10，并补：FR→F、作废禁引用、一真票一 PF、签后禁补挂）

## 11. 修订

| 日期 | 说明 |
|------|------|
| 2026-09-30 | 定稿：会话拍板 P0（关联、状态、合规、仅手工、FR→F、列表/营收等） |
| 2026-09-30 | 落地：migration 013、IssuePF/AnnulPF、IssueFT 挂引用、SAF-T WorkingDocuments、Admin、regression |
