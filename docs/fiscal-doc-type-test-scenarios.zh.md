# 葡萄牙单据类型：开票场景与测试方案

> **状态：草稿**（已有 5 种对齐现网行为；新增 4 种为顺序开发前的场景/验收口径）  
> **权威：否**（业务规则以本文 + 各里程碑设计文为准；库列以 [`fiscal-sqlite-schema.zh.md`](fiscal-sqlite-schema.zh.md) + `migrations/*.sql` 为准）  
> **对应实现：**  
> - 已落地：`FT` / `FS` / `FR` / `NC` / `ND`（见 [`fiscal-dev-plan.zh.md`](fiscal-dev-plan.zh.md) M0–M6）  
> - 尚未落地：`RG` / `PF` / `GR` / `GT`（实现前须另开里程碑设计文）  
> **写作规范：** [`design-doc-standards.zh.md`](design-doc-standards.zh.md)

本文把 **9 种**单据的使用场景、关联规则与**可执行测试场景**写在一处，供顺序开发与回归对照。

---

## 1. 范围与总原则

### 1.1 九种单据一览

| # | 代码 | 葡语名 | 中文 | 产品状态 | SAF-T 表 |
|---|------|--------|------|----------|----------|
| 1 | FT | Fatura | 正式发票 | **已有** | 4.1 SalesInvoices |
| 2 | FS | Fatura simplificada | 简化发票 | **已有** | 4.1 |
| 3 | FR | Fatura-recibo | 发票-收据 | **已有** | 4.1 |
| 4 | NC | Nota de crédito | 贷记/冲销 | **已有** | 4.1 |
| 5 | ND | Nota de débito | 借记/补收 | **已有** | 4.1 |
| 6 | RG | Recibo | 收款收据 | **已有**（0.5.18） | 4.4 Payments |
| 7 | PF | Fatura pró-forma | 形式发票 | **未做** | 4.3 WorkingDocuments |
| 8 | GR | Guia de remessa | 发货/交货单 | **未做** | 4.2 MovementOfGoods |
| 9 | GT | Guia de transporte | 运输单 | **未做** | 4.2 |

### 1.2 P0 总原则（拍板）

1. **销售税票只有 FT / FS / FR**；NC / ND 改税基；RG / PF / GR / GT **不能**替代销售税票。  
2. **堂食 / 账单同步主路径**仍直接开 FS 或 FT；P0 **不**在结账流强制插入 PF / GR / GT / RG。  
3. **硬关联**（无原票不可开）：NC / ND → FT|FS|FR；RG → **仅 FT**（P0）。  
4. **软关联**（可选）：真票 → PF；FT → GR。  
5. **无销售关联**：GT（调拨可永远无 FT）。  
6. **金额**：仅 NC / ND / RG 须与原销售票余额对齐；PF / GR / GT **不要求**与 FT/FS 数字相等。  
7. **新增票开发顺序（定法）：** `RG` → `PF` → `GR` → `GT`（GT 须先有 AT 运输通信方案，否则不做）。

### 1.3 关联总图

```text
PF ──(可选)──► FT / FS / FR ──(强制)──► NC / ND
                     │
                     └──(强制, 仅 FT)──► RG

GR ──(可选)──► FT
GT ──(通常无)──► （可无销售票）
```

### 1.4 测试写法约定

每个场景含：

| 项 | 含义 |
|----|------|
| **前置** | 系列、原票、权限等 |
| **步骤** | 操作顺序 |
| **期望** | 状态、金额、错误码、票面/SAF-T |
| **类型** | 正路径 / 负路径 / 边界 |

自动化优先挂现有脚本风格（`scripts/fiscal-*-regression.mjs` + `go test ./internal/fiscal/...`）；未落地类型以本文为验收清单，落地时再补脚本名。

---

## 2. FT — Fatura（正式发票）

### 2.1 使用场景

- B2B、要完整购方资料、超 FS 限额、或需挂账后收款。  
- 账单同步可选 FT；手工开票可选 FT。

### 2.2 规则摘要

| 项 | 定法 |
|----|------|
| 系列 | 独立 ACTIVE `FT` 系列 |
| 默认结账 | 否（产品默认 FS） |
| 可被 | NC / ND 调整；未来 RG 收款（仅 FT） |
| 可选引用 | 未来 PF、GR |

### 2.3 测试场景

| ID | 类型 | 前置 | 步骤 | 期望 |
|----|------|------|------|------|
| FT-01 | 正 | FT 系列 ACTIVE | 手工开 FT，购方含 NIF | 签发 SIGNED；票号 `FT …/n`；打印 ORIGINAL；有 ATCUD/Hash/QR |
| FT-02 | 正 | 账单草稿 | 分单/整桌选 FT 签发 | 同 FT-01；草稿消账；不可重复开同 scope |
| FT-03 | 正 | 已签 FT | 重打 | 新打印任务；Hash/ATCUD **不变** |
| FT-04 | 负 | 无 FT 系列 | 尝试签发 | 失败，稳定错误（系列缺失） |
| FT-05 | 正 | 同 request_id | 重复提交签发 | 幂等，不双开 |
| FT-06 | 边界 | 未来有 PF | 开 FT 时引用 PF | 可选成功；金额可与 PF 不等（见 PF-05） |
| FT-07 | 边界 | 未来有 GR | 开 FT 引用 1..N 张 GR | 可选成功；金额不必轧平（见 GR-04） |

**回归锚点（已有）：** `fiscal-local-regression.mjs`、`fiscal-manual-ft-regression.mjs`、`fiscal-bill-sync-regression.mjs`、`fiscal-reprint-regression.mjs`。

---

## 3. FS — Fatura simplificada（简化发票）

### 3.1 使用场景

- 餐厅/零售堂食、当场付款、金额在简化票限额内。  
- **产品默认**销售类型；账单同步默认 FS。

### 3.2 规则摘要

| 项 | 定法 |
|----|------|
| 系列 | 独立 ACTIVE `FS` |
| 账单同步 | 允许 FT / FS；**不允许 FR** |
| 不可当 | 运输单（法规）；P0 不做「FS 兼 GT」 |
| 可被 | NC / ND；**P0 RG 不挂 FS** |

### 3.3 测试场景

| ID | 类型 | 前置 | 步骤 | 期望 |
|----|------|------|------|------|
| FS-01 | 正 | FS 系列 ACTIVE | 不选类型直接手工开票 | 签发为 **FS** |
| FS-02 | 正 | 账单草稿 | 默认可开票类型签发 | FS；打印与 ATCUD 正常 |
| FS-03 | 正 | 已签 FS | NC 全额冲销 | 原票 CREDITED_FULL；NC 引用正确 |
| FS-04 | 正 | 已签 FS | ND 借记 | 原票 DEBITED_*；ND 引用正确 |
| FS-05 | 负 | （未来 RG 落地后） | 对 FS 开 RG | **拒绝**（P0 仅 FT） |
| FS-06 | UI | Admin | 账单分单类型下拉 | 仅 FT、FS；无 FR |

**回归锚点（已有）：** `fiscal-m6-regression.mjs`、`fiscal-d62-cert-regression.mjs`、手测 [`fiscal-m6-manual-uat.zh.md`](fiscal-m6-manual-uat.zh.md) H1。

---

## 4. FR — Fatura-recibo（发票-收据）

### 4.1 使用场景

- 开票当日全额收讫，一张票同时表示销售+收款。  
- **仅**手工 / Local API；**账单同步不可选 FR**。

### 4.2 规则摘要

| 项 | 定法 |
|----|------|
| 系列 | 独立 `FR`；`fr_series_ok` **不**阻塞 `ready_to_issue` |
| 与 RG | 互斥语义：已 FR **不应**再开 RG |
| 可被 | NC / ND |

### 4.3 测试场景

| ID | 类型 | 前置 | 步骤 | 期望 |
|----|------|------|------|------|
| FR-01 | 正 | FR 系列 ACTIVE | 手工选 FR 签发 | SIGNED；票号 FR 前缀；进 SAF-T InvoiceType=FR |
| FR-02 | 负 | 账单同步 | document_type=FR | API 拒绝（仅 FT/FS） |
| FR-03 | 正 | 无 FR 系列 | 仅开 FT/FS | `ready_to_issue` 仍可为真 |
| FR-04 | 正 | 已签 FR | NC / ND | 与 FT/FS 相同可调整 |
| FR-05 | 负 | （未来 RG） | 对 FR 开 RG | **拒绝** |
| FR-06 | UI | Admin | 手工下拉 | 可见 FR；发票列表有 FR Tab |

**回归锚点（已有）：** `fiscal-m6-regression.mjs`、`fiscal-d62-cert-regression.mjs`。

---

## 5. NC — Nota de crédito（冲销）

### 5.1 使用场景

- 退菜、退货、折扣、开错需冲减已签销售票。

### 5.2 规则摘要

| 项 | 定法 |
|----|------|
| 原票 | **必须** FT / FS / FR |
| 金额 | ≤ 行/票剩余可冲额度；全额或部分 |
| 系列 | 独立 `NC` |
| 权限 | `can_issue_nc` |

### 5.3 测试场景

| ID | 类型 | 前置 | 步骤 | 期望 |
|----|------|------|------|------|
| NC-01 | 正 | 已签 FT | 全额 NC | 原票 CREDITED_FULL；NC 含原票引用+原因；打印 |
| NC-02 | 正 | 已签 FS | 部分 NC（按行） | CREDITED_PARTIAL；剩余可继续冲 |
| NC-03 | 正 | 已签 FR | NC | 同 NC-01/02 |
| NC-04 | 负 | 无可冲余额 | 再开 NC | 失败（超额/不允许） |
| NC-05 | 负 | 无 NC 系列 | 开 NC | series_missing 类错误 |
| NC-06 | 负 | 无 can_issue_nc | 开 NC | 权限拒绝 |
| NC-07 | 正 | 同月 FT+NC | 导出 SAF-T | 两者均在；NC References 正确 |
| NC-08 | 幂等 | 同 request_id | 重复 NC | 不双开 |

**回归锚点（已有）：** `fiscal-m3-regression.mjs`、[`fiscal-m3-nc.zh.md`](fiscal-m3-nc.zh.md)、`fiscal-m5-regression.mjs`。

---

## 6. ND — Nota de débito（借记）

### 6.1 使用场景

- 原票少收、需补加金额（服务费漏收等）。

### 6.2 规则摘要

| 项 | 定法 |
|----|------|
| 原票 | **必须** FT / FS / FR |
| 金额上限 | P0：**不以**原票 gross 为天花板（可继续借记）；见 [`fiscal-m6-fs-fr-nd.zh.md`](fiscal-m6-fs-fr-nd.zh.md) |
| 系列 | 独立 `ND` |
| 权限 | 复用 `can_issue_nc` |

### 6.3 测试场景

| ID | 类型 | 前置 | 步骤 | 期望 |
|----|------|------|------|------|
| ND-01 | 正 | 已签 FT | 全额 ND（按产品「全额借记」语义） | ND 签发；原票 debited 累加；状态 DEBITED_PARTIAL |
| ND-02 | 正 | 已签 FS | 部分按行 ND | 行引用正确；可再次 ND |
| ND-03 | 正 | 已签 FR | ND | 同可调整原票白名单 |
| ND-04 | 正 | 已 ND 一次 | 再 ND 且金额 > 原票 gross | **允许**（现行 P0） |
| ND-05 | 负 | 无 ND 系列 | 开 ND | series_missing |
| ND-06 | 负 | 无权限 | 开 ND | 拒绝 |
| ND-07 | 正 | 同月含 ND | SAF-T | InvoiceType=ND + 行引用 |
| ND-08 | UI | Admin | 与 NC 同一调整入口 | 默认按行；详情回链原票 |

**回归锚点（已有）：** `fiscal-m6-regression.mjs`、store `issue_m6_test.go`。

---

## 7. RG — Recibo（收款收据）【未做 · 建议 M-A】

### 7.1 使用场景

- 已开 **FT** 挂账，客户事后付款 → 开 RG 证明收款。  
- **不是**发票；不改变销售额/VAT。

### 7.2 P0 规则（拍板）

| 项 | 定法 |
|----|------|
| 原票 | **必须** ≥1 张已签 **FT** |
| 禁止原票 | FS、FR（FR 已表示收讫） |
| 金额 | 必填；单次 ≤ 该 FT **未收余额**；允许多张 RG 分期 |
| 与 NC | 冲销后按剩余应收计算；应收为 0 则不可再 RG |
| 系列 | 独立 `RG` |
| SAF-T | 4.4，`PaymentType=RG` |
| 入口 | FT 详情「登记收款」；不进堂食默认结账 |

### 7.3 测试场景

| ID | 类型 | 前置 | 步骤 | 期望 |
|----|------|------|------|------|
| RG-01 | 正 | FT €100，未收款 | 开 RG €100 | RG SIGNED；FT 未收余额 0；SAF-T 含 RG |
| RG-02 | 正 | FT €100 | RG €40 再 RG €60 | 两张 RG；余额 0；累计=100 |
| RG-03 | 负 | FT €100，已收 €100 | 再 RG €1 | 失败（超额） |
| RG-04 | 负 | 仅有 FS | 开 RG | 失败（原票类型非法） |
| RG-05 | 负 | 仅有 FR | 开 RG | 失败 |
| RG-06 | 负 | 无原票 | 裸开 RG | 失败 |
| RG-07 | 正 | FT €100，NC 冲 €30 | 开 RG | 最多可收 €70；收 €71 失败 |
| RG-08 | 负 | 无 RG 系列 | 开 RG | series_missing |
| RG-09 | 幂等 | 同 request_id | 重复 | 不双开 |
| RG-10 | UI | Admin | FT 详情 | 有收款入口；FS/FR 详情无此入口（或禁用） |
| RG-11 | 打印 | RG-01 | 出纸 | 票面标明收据/关联 FT 号；非「Fatura」税票文案 |

**落地后回归（建议名）：** `scripts/fiscal-rg-regression.mjs`。

---

## 8. PF — Fatura pró-forma（形式发票）【未做 · 建议 M-B】

### 8.1 使用场景

- 宴会/企业询价：先报价，确认后再开 FT/FS/FR。  
- **非正式发票**；可不转真票。

### 8.2 P0 规则（拍板）

| 项 | 定法 |
|----|------|
| 金额 | 有；与最终真票 **不必**相等 |
| 转真票 | **可选**；开 FT/FS/FR 时可引用 1 张 PF |
| 强制 | 无 PF 也可直接开真票 |
| 系列 | 独立 `PF` |
| SAF-T | 4.3，`WorkType=PF`；转真票后可标已开票 `F` |
| 票面 | 必须可见 **Pró-forma / 非发票** 字样 |

### 8.3 测试场景

| ID | 类型 | 前置 | 步骤 | 期望 |
|----|------|------|------|------|
| PF-01 | 正 | PF 系列 | 开 PF €800 | 签发成功；票面非发票；不进 SalesInvoices |
| PF-02 | 正 | 已有 PF | 客户取消，不转真票 | PF 保持未开票；系统允许 |
| PF-03 | 正 | PF €800 | 开 FS €800，引用该 PF | 真票成功；PF 状态可标已开票 |
| PF-04 | 正 | PF €800 | 开 FT €920，引用该 PF | **允许**金额不一致 |
| PF-05 | 正 | 无 PF | 直接开 FS | 成功（无强制前置） |
| PF-06 | 负 | 无 PF 系列 | 开 PF | series_missing |
| PF-07 | 负 | 误把 PF 当税票申报唯一依据 | （流程/培训） | 产品不提供「仅用 PF 完成结账报税」路径 |
| PF-08 | UI | 开真票 | 可选挂 PF | 不选也能签发 |
| PF-09 | SAF-T | 仅 PF、未转 | 月导 | WorkingDocuments 有 PF；SalesInvoices 无对应假发票 |
| PF-10 | 打印 | PF-01 | 出纸 | 含「Pró-forma」；无误导为 Fatura 的主标题 |

**落地后回归（建议名）：** `scripts/fiscal-pf-regression.mjs`。

---

## 9. GR — Guia de remessa（发货单）【未做 · 建议 M-C】

### 9.1 使用场景

- 批发/送货：先交货开 GR，后开 FT（可多张 GR 汇总一张 FT）。  
- 堂食 **不使用** GR。

### 9.2 P0 规则（拍板）

| 项 | 定法 |
|----|------|
| 金额 | 可有；与后续 FT **不必**相等、不必一张对一张 |
| 开 FT | **可选**引用 1..N 张 GR |
| 未开 FT | **允许**；可提供「未开票 GR」列表提醒，**不**自动开 FT |
| 系列 | 独立 `GR` |
| SAF-T | 4.2，`MovementType=GR` |
| 退货 | P0 仍用 **NC** 冲 FT；不做 GD |
| AT 运输实时通信 | **非**本阶段（见 GT） |

### 9.3 测试场景

| ID | 类型 | 前置 | 步骤 | 期望 |
|----|------|------|------|------|
| GR-01 | 正 | GR 系列 | 开 GR €500（含行项目） | 签发；进 MovementOfGoods |
| GR-02 | 正 | GR €300 + GR €200 | 月末开一张 FT €500，引用两张 GR | FT 成功；引用可选落库 |
| GR-03 | 正 | GR €500 | 开 FT €520（加运费） | **允许**；以 FT 为准 |
| GR-04 | 正 | GR €500 | 先 FT €200（部分开票） | 允许；剩余可再开 FT（若产品支持多次引用策略：见 TBD） |
| GR-05 | 正 | 有 GR 未开 FT | 跨月末仍未开 FT | GR 仍有效；提醒列表可见；**不**自动作废 |
| GR-06 | 正 | 无 GR | 直接开 FT | 成功 |
| GR-07 | 负 | 无 GR 系列 | 开 GR | series_missing |
| GR-08 | UI | 堂食结账 | — | **无**强制选 GR 步骤 |
| GR-09 | UI | 开 FT | 可选挂 GR | 多选 1..N；可不选 |
| GR-10 | 混用 | GR 交货后退货 | 已开 FT 则 NC；未开 FT | P0：业务取消/作废 GR 策略 **TBD**（须在 GR 里程碑设计文拍板） |

**GR-04 / GR-10 TBD：** 实现 GR 里程碑设计文时必须拍板「一张 GR 是否允许多次部分开票」与「未开票 GR 作废」；本文只锁定测试意图。

**落地后回归（建议名）：** `scripts/fiscal-gr-regression.mjs`。

---

## 10. GT — Guia de transporte（运输单）【未做 · 建议 M-D】

### 10.1 使用场景

- 货在途合规（仓→店、店→店、外送需独立运输单）。  
- **调拨可永远无 FT**。

### 10.2 P0 规则（拍板）

| 项 | 定法 |
|----|------|
| 与销售票 | **无强制关联** |
| 必填 | 品名/数量、装货地、卸货地、开始运输时间等（细则在 GT 设计文） |
| 金额 | 非验收重点 |
| 时点 | **上路前**签发；默认禁止「已上路补开」 |
| AT 通信 | **有通信方案才做 GT**；无方案则本里程碑不做 |
| 系列 | 独立 `GT` |
| SAF-T | 4.2，`MovementType=GT` |
| FS 兼运输单 | **禁止** |

### 10.3 测试场景

| ID | 类型 | 前置 | 步骤 | 期望 |
|----|------|------|------|------|
| GT-01 | 正 | GT 系列 + AT 通信可用 | 上路前开 GT | 签发；获 AT 文档码（或约定成功回执）；可打印 |
| GT-02 | 正 | 店间调拨 | 仅 GT，不开 FT | **允许**；无销售票 |
| GT-03 | 正 | 外送 | GT + 另开 FS | 两张独立；无强制父子引用 |
| GT-04 | 负 | AT 通信失败 | 开 GT | 按设计失败或降级策略（须在 GT 文拍板）；不得静默假成功 |
| GT-05 | 负 | 无装货/卸货地 | 开 GT | 校验失败 |
| GT-06 | 负 | UI「补开已上路」 | 默认路径 | 拦截或强确认（定法在 GT 文） |
| GT-07 | 负 | 试图用 FS 当 GT | — | 产品无此能力 |
| GT-08 | SAF-T | 仅调拨 GT | 月导 | MovementOfGoods 有 GT；无对应假 FT |

**落地门槛：** 无 AT 运输通信设计文与验收环境 → **不开 GT 开发刀**。

**落地后回归（建议名）：** `scripts/fiscal-gt-regression.mjs`（含通信 mock）。

---

## 11. 跨类型矩阵（验收速查）

### 11.1 谁可以挂谁

| 子单据 | 合法原件 / 前置 | 强制？ | 金额对齐？ |
|--------|-----------------|--------|------------|
| NC | FT / FS / FR | 是 | 是（≤ 可冲余额） |
| ND | FT / FS / FR | 是 | 行引用；累计无原票天花板（现 P0） |
| RG | **仅 FT** | 是 | 是（≤ 未收） |
| 真票引用 PF | PF | 否 | 否 |
| FT 引用 GR | GR | 否 | 否 |
| GT | — | — | — |

### 11.2 堂食主路径不得出现的步骤

| 步骤 | P0 |
|------|-----|
| 结账强制 PF | 否 |
| 结账强制 GR / GT | 否 |
| 结账开 RG | 否 |
| 默认真票 | **FS** |

### 11.3 顺序开发与依赖

| 顺序 | 类型 | 依赖 | 阻塞条件 |
|------|------|------|----------|
| 1 | RG | 已有 FT、NC 余额语义 | 无 |
| 2 | PF | 已有 FT/FS/FR 签发 | 无 |
| 3 | GR | 已有 FT；可选引用 | GR-04/10 须先拍板 |
| 4 | GT | AT 运输通信方案 | **无方案则跳过** |

---

## 12. 与现有文档的关系

| 文档 | 关系 |
|------|------|
| [`fiscal-dev-plan.zh.md`](fiscal-dev-plan.zh.md) | 工程里程碑；新增 4 票须立项后再改计划表 |
| [`fiscal-m3-nc.zh.md`](fiscal-m3-nc.zh.md) | NC 实现权威 |
| [`fiscal-m6-fs-fr-nd.zh.md`](fiscal-m6-fs-fr-nd.zh.md) | FS/FR/ND 实现权威 |
| [`fiscal-m5-saft.zh.md`](fiscal-m5-saft.zh.md) | SAF-T；新类型落地须扩展导出范围 |
| [`fiscal-m6-manual-uat.zh.md`](fiscal-m6-manual-uat.zh.md) | 已有类型手测 |
| [`fiscal-certification-checklist.zh.md`](fiscal-certification-checklist.zh.md) | 认证项；新类型不自动纳入除非认证范围变更 |

---

## 13. 修订记录

| 日期 | 说明 |
|------|------|
| 2026-09-24 | 初稿：9 种单据场景 + 测试 ID；新增 RG→PF→GR→GT 顺序与关联定法 |
