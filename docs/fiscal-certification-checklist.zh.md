# Fiscal 认证检查清单

> **状态：草稿**（M6 D6.2 骨架）  
> **权威范围：** 产品认证单据 **FT + FS + NC + ND**（FR 内核保留，不进产品 UI）

## 1. 系列与 Setup

| # | 检查项 | 通过标准 |
|---|--------|----------|
| C1.1 | FT ACTIVE 系列 + validation_code | `setup.status.series_ok` |
| C1.2 | FS ACTIVE 系列 + validation_code | `setup.status.fs_series_ok` |
| C1.3 | NC ACTIVE 系列 | `setup.status.nc_series_ok` |
| C1.4 | ND ACTIVE 系列 | `setup.status.nd_series_ok` |
| C1.5 | 可开票门闸 | `ready_to_issue` = 门店 + FT + **FS** + 激活 + 操作员 |
| C1.6 | 可冲销 / 可借记 | `ready_to_credit` / `ready_to_debit` |

## 2. 签发（FT / FS）

| # | 检查项 | 通过标准 |
|---|--------|----------|
| C2.1 | 默认单据类型 | 省略 `document_type` → **FS** |
| C2.2 | 产品 UI 类型 | 仅 FT、FS 可选 |
| C2.3 | 付款方式 | CASH / CARD / MBWAY / MULTIBANCO / MIXED / OTHER |
| C2.4 | Hash 链 | 同系列 `previous_hash` 连续 |
| C2.5 | ATCUD | 票面与 DB 一致 |
| C2.6 | QR | 可读且含 ATCUD / 税额 |

## 3. NC（冲销）

| # | 检查项 | 通过标准 |
|---|--------|----------|
| C3.1 | 原票类型 | FT、FS 可全额/部分冲销 |
| C3.2 | 原票状态 | `CREDITED_PARTIAL` / `CREDITED_FULL` |
| C3.3 | 剩余金额 | 详情 `remaining_gross_total` 正确 |
| C3.4 | 权限 | `can_issue_nc` enforce |

## 4. ND（借记）

| # | 检查项 | 通过标准 |
|---|--------|----------|
| C4.1 | 原票类型 | FT、FS 可借记 |
| C4.2 | 超额拒绝 | `debit_amount_exceeded` |
| C4.3 | 原票状态 | `DEBITED_PARTIAL` / `DEBITED_FULL` |

## 5. SAF-T 与打印

| # | 检查项 | 通过标准 |
|---|--------|----------|
| C5.1 | 月导含 FT/FS/NC/ND | XML `InvoiceType` 齐全 |
| C5.2 | Windows-1252 | 葡语重音 VALID |
| C5.3 | 原票打印 | ORIGINAL `PRINTED` |
| C5.4 | 副本 Hash | 与详情一致 |

## 6. 账单同步（餐馆）

| # | 检查项 | 通过标准 |
|---|--------|----------|
| C6.1 | 再同步挡重 | `HasSignedSaleForSale`（FT 或 FS 已签 → `already_invoiced`） |
| C6.2 | 分单默认 FS | `IssueFromBillDraft` 可传 `document_type` |

## 7. 运维（D6.3 / D6.4 待实现）

| # | 检查项 | 通过标准 |
|---|--------|----------|
| C7.1 | 备份恢复校验 | 恢复后 `last_number` / `last_hash` 一致 |
| C7.2 | 换机 | 旧 installation revoked + 新 wrap |

## 回归脚本

- `scripts/fiscal-m6-regression.mjs` — 内核 FS/FR/ND
- `scripts/fiscal-m6-product-regression.mjs` — 产品规则（FS 默认、六种付款、NC/ND on FT+FS）
- `scripts/fiscal-m3-regression.mjs` — NC
- `scripts/fiscal-bill-sync-regression.mjs` — 草稿同步
