# Fiscal 认证检查清单

> **状态：草稿**（M6 D6.2–D6.4：检查项已分类；自动化以本页 + runner 出具）  
> **权威范围：** 产品认证单据 **FT + FS + NC + ND**（FR 内核保留，不进产品 UI）  
> **出具命令：** `node scripts/fiscal-d62-cert-regression.mjs`（逐项 pass/fail）；运维细测 `fiscal-d63-d64-regression.mjs`  
> **分类：** `自动` = runner/单测可判；`手测` = 真机/扫枪/UI 肉眼  
> **设计：** [`fiscal-m6-backup-swap.zh.md`](fiscal-m6-backup-swap.zh.md)（D6.3/D6.4）

## 如何读本表

| 列 | 含义 |
|----|------|
| 分类 | 自动 / 手测 |
| 验证方式 | 命令或操作；自动项以 d62 runner 为准 |
| 结果 | 以最近一次 `fiscal-d62-cert-regression.mjs` 输出为准（本文件不手填历史） |

关联脚本（runner 未覆盖的补充证据）：

- `scripts/fiscal-m6-regression.mjs` — FS/FR/ND 内核
- `scripts/fiscal-m6-product-regression.mjs` — 产品规则
- `scripts/fiscal-m3-regression.mjs` — NC 部分冲销
- `scripts/fiscal-m5-regression.mjs` — SAF-T
- `scripts/fiscal-bill-sync-regression.mjs` — 账单同步挡重
- `scripts/fiscal-reprint-regression.mjs` — 重打 Hash 不变
- `scripts/fiscal-d63-d64-regression.mjs` — 备份 / 完整性 / 换机停用

---

## 1. 系列与 Setup

| # | 检查项 | 通过标准 | 分类 | 验证方式 |
|---|--------|----------|------|----------|
| C1.1 | FT ACTIVE 系列 + validation_code | `setup.status.series_ok` | 自动 | d62 → GET `/local/v1/setup/status` |
| C1.2 | FS ACTIVE 系列 + validation_code | `setup.status.fs_series_ok` | 自动 | 同上 |
| C1.3 | NC ACTIVE 系列 | `setup.status.nc_series_ok` | 自动 | 同上 |
| C1.4 | ND ACTIVE 系列 | `setup.status.nd_series_ok` | 自动 | 同上 |
| C1.5 | 可开票门闸 | `ready_to_issue` = 门店 + FT + **FS** + 激活 + 操作员 | 自动 | 同上 |
| C1.6 | 可冲销 / 可借记 | `ready_to_credit` / `ready_to_debit` | 自动 | 同上 |

## 2. 签发（FT / FS）

| # | 检查项 | 通过标准 | 分类 | 验证方式 |
|---|--------|----------|------|----------|
| C2.1 | 默认单据类型 | 省略 `document_type` → **FS** | 自动 | d62 POST `/fiscal-documents` 无类型 |
| C2.2 | 产品类型限制 | API 拒 FR；UI 仅 FT、FS | 自动 + 手测 | d62 拒 FR；**手测** Admin 下拉 |
| C2.3 | 付款方式 | CASH / CARD / MBWAY / MULTIBANCO / MIXED / OTHER | 自动 | d62 六种 manual issue |
| C2.4 | Hash 链 | 同系列 `previous_hash` = 上张 `hash` | 自动 | d62 连开两张 FS + sqlite |
| C2.5 | ATCUD | `validation_code-n` 与详情/DB 一致 | 自动 | d62 |
| C2.6 | QR 内容 | `qr_content` 含 `H:` ATCUD 与税额字段 | 自动 + 手测 | d62 sqlite；**手测** 扫枪 |

## 3. NC（冲销）

| # | 检查项 | 通过标准 | 分类 | 验证方式 |
|---|--------|----------|------|----------|
| C3.1 | 原票类型 | FT、FS 可冲销 | 自动 | d62 NC on FS（FT 见 m3） |
| C3.2 | 原票状态 | `CREDITED_PARTIAL` / `CREDITED_FULL` | 自动 | d62 全额；部分见 `fiscal-m3-regression.mjs` |
| C3.3 | 剩余金额 | 详情 `remaining_gross_total` 正确 | 自动 | d62 全额后 remaining≈0 |
| C3.4 | 权限 | `can_issue_nc` enforce | 自动 | d62 `credit_not_allowed` |

## 4. ND（借记）

| # | 检查项 | 通过标准 | 分类 | 验证方式 |
|---|--------|----------|------|----------|
| C4.1 | 原票类型 | FT、FS 可借记 | 自动 | d62 ND on FT |
| C4.2 | 超额拒绝 | `debit_amount_exceeded` | 自动 | d62 |
| C4.3 | 原票状态 | `DEBITED_PARTIAL` / `DEBITED_FULL` | 自动 | d62 全额；部分见 m6-regression |

## 5. SAF-T 与打印

| # | 检查项 | 通过标准 | 分类 | 验证方式 |
|---|--------|----------|------|----------|
| C5.1 | 月导含 FT/FS/NC/ND | XML `InvoiceType` 齐全 + VALID | 自动 | d62 export + download |
| C5.2 | Windows-1252 | 葡语重音产品月导 VALID | 自动 | d62（Água）；单测见 `saft/build_test.go` |
| C5.3 | 原票打印 | ORIGINAL → `PRINTED` | 自动 + 手测 | d62 MemorySink；**手测** 真热敏 |
| C5.4 | 副本 Hash | 重打后详情 `hash` 不变 | 自动 | d62 reprint |

## 6. 账单同步（餐馆）

| # | 检查项 | 通过标准 | 分类 | 验证方式 |
|---|--------|----------|------|----------|
| C6.1 | 再同步挡重 | 已签 FT/FS → `already_invoiced` | 自动 | d62 `go test …AlreadyInvoiced…`；完整链路 `fiscal-bill-sync-regression.mjs` |
| C6.2 | 分单默认 FS | `IssueFromBillDraft` 省略类型 → FS | 自动 | d62 插草稿后 issue |

## 7. 运维（D6.3 / D6.4）

| # | 检查项 | 通过标准 | 分类 | 验证方式 |
|---|--------|----------|------|----------|
| C7.1 | 备份恢复校验 | backup 成功；`last_number`/`last_hash` 与票库一致（失配 block） | 自动 | d62 / `fiscal-d63-d64-regression.mjs` |
| C7.2 | 换机本机停用 | `prepare-swap` → `activated_ok=false`；新机重新 wrap | 自动 + 手测 | d62 API；**手测** 真机拷库+运营激活 |

---

## 你（产品）只需手测的项

逐步操作见 [`fiscal-m6-manual-uat.zh.md`](fiscal-m6-manual-uat.zh.md)。摘要：

1. **C2.2-UI** — Admin 手工开票类型只见 FT / FS  
2. **C2.6-scan** — 扫枪能读出纸 QR  
3. **C5.3-hw** — 真热敏 ORIGINAL 出纸含 ATCUD  
4. **C7.2-hw** —（可选）真机换机：备份 → 新 PC 换库 → 校验 → 运营同步授权  
