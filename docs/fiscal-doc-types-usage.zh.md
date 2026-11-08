# 九种单据：用途、当事人与开具方法

> **状态：定稿**  
> **权威：是**（用途、当事人「谁开给谁」、开具方法与跨类型挂靠；实现细节以各里程碑设计文 + [`fiscal-sqlite-schema.zh.md`](fiscal-sqlite-schema.zh.md) + DDL 为准）  
> **对应实现：** 九种**均已落地**；GR / GT 在 `FISCAL_AT_ENV=test` 已对 AT 测试环境真通信（2026-11-03，见 [`fiscal-at-live-integration.zh.md`](fiscal-at-live-integration.zh.md) A2），默认环境仍为 mock  
> **测试对照：** [`fiscal-doc-type-test-scenarios.zh.md`](fiscal-doc-type-test-scenarios.zh.md)  
> **写作规范：** [`design-doc-standards.zh.md`](design-doc-standards.zh.md)

本文把九种单据的**业务用途、当事人、开具方法**写在一处，供产品与实现对照。  
库列 / API / 唯一写路径仍以各类型权威设计文为准（见文末索引）。

---

## 0. 总览

| 代码 | 中文 | 产品状态 | 性质 | 开给谁（收件方） | 主入口 |
|------|------|----------|------|------------------|--------|
| FT | 正式发票 | 已有 | 销售税票 | 购方客户 | 手工 / 收银账单 |
| FS | 简化发票 | 已有（默认） | 销售税票 | 购方客户（可散客） | 手工 / 收银账单 |
| FR | 发票-收据 | 已有 | 销售税票 + 当场收讫 | 购方客户 | **仅手工**（账单同步不可选） |
| NC | 贷记/冲销 | 已有 | 改税基 | 原票购方（复制） | 原票详情「冲销」 |
| ND | 借记/补收 | 已有 | 改税基 | 原票购方（复制） | 原票详情「借记」 |
| RG | 收款收据 | 已有 | 证明事后收款 | 原 FT 购方 | FT 详情「登记收款」 |
| PF | 形式发票 | 已有 | 报价/确认（非税票） | 潜在购方 | **仅手工** |
| GR | 发货/交货单 | 已有（AT test 已通） | 货物移动（交货） | 收货客户 | **仅手工** |
| GT | 运输单 | 已有（AT test 已通） | 货物移动（在途） | 卸货方（调拨=本店；外送=客户） | **仅手工** |

### 0.1 总原则（P0）

1. **销售税票只有 FT / FS / FR**；NC / ND 改税基；RG / PF / GR / GT **不能**替代销售税票。  
2. **堂食 / 账单同步主路径**直接开 FS 或 FT；**不**强制插入 PF / GR / GT / RG。  
3. **硬关联**（无原票不可开）：NC / ND → FT|FS|FR；RG → **仅 FT**。  
4. **软关联**（可选）：开真票时可挂 PF；开 **FT** 时可挂**同一购方**的 1..N 张 GR（§8.5）。  
5. **GT** 与销售票无强制关联（调拨可永远无 FT）。  
6. **金额对齐**：仅 NC / ND / RG 须与原销售票余额规则对齐；PF / GR / GT 与真票金额**不必**相等。  
7. **开发顺序：** RG → PF → GR → GT（已全部落地；GR/GT live 随 A2）。  
8. **实时报 AT 的只有 GR / GT**（上路前取 `ATDocCodeID`）；其余七种只需系列在 AT 注册（ATCUD），不逐张实时报送。

### 0.2 关联总图

```text
PF ──(可选)──► FT / FS / FR ──(强制)──► NC / ND
                     │
                     └──(强制, 仅 FT)──► RG

GR ──(可选, 开 FT 时挂)──► FT
GT ──(通常无)──► （调拨可永远无销售票）
```

### 0.3 挂靠矩阵（谁挂谁）

| 子单据 / 引用 | 合法前置 | 强制？ | 金额对齐？ |
|---------------|----------|--------|------------|
| NC | FT / FS / FR | 是 | 是（≤ 可冲余额） |
| ND | FT / FS / FR | 是 | 行引用；累计无原票天花板（现 P0） |
| RG | **仅 FT** | 是 | 是（≤ 未收余额） |
| 真票引用 PF | PF | 否 | 否 |
| FT 引用 GR | GR（1..N） | 否 | 否 |
| GT | — | — | — |

---

## 1. FT — Fatura（正式发票）

| 项 | 定法 |
|----|------|
| **用途** | B2B、要完整购方资料、超 FS 限额、或挂账后收款。 |
| **当事人** | **本店 → 购方客户**（可为消费者 / 散客）。开具方=本店纳税人。 |
| **开具方法** | ① 注册 ACTIVE **FT** 系列。② **收银账单**整桌/按人选 FT 签发；或 **手工开票**四步选 FT；或 PF / GR 详情「开真票 / 开发票」预填（§7、§8.5）。③ 可选挂一张 PF；可选挂同一购方的 1..N 张 GR（§8.5）。④ 签发 → 独立 FT Hash 链 → 打印 ORIGINAL。 |
| **NIF** | 依 CIVA art. 36.º n.º 5 a)（DL 28/2019 后）：购方为 **IVA 纳税人**（sujeito passivo）→ 名称、地址、NIF 必填；购方为**消费者** → NIF **可选**，顾客要求时必须打印（n.º 16），名称地址不要求；无 NIF 时 SAF-T 用 `999999990` / 票面「Consumidor final」（Despacho 8632/2014 §2.2.5；AT 信息 10376）。系统现状：**FT 允许散客**（含 FS > €100 自动升 FT）；有 NIF 时未强制名称 / 地址（缺名称时以 NIF 代填、地址 `Desconhecido`）——对纳税人购方不合规，**待产品定**是否加「公司客户」标记（见测试报告 2026-11-03 §5）。 |
| **可被** | NC / ND；RG（仅挂账事后收款）。 |
| **权威** | M0–M2.6 / M4；票面 [`fiscal-ft-receipt-layout.zh.md`](fiscal-ft-receipt-layout.zh.md) |

---

## 2. FS — Fatura simplificada（简化发票）

| 项 | 定法 |
|----|------|
| **用途** | 餐厅/零售堂食、当场付款、金额在简化票限额内。**产品默认**销售类型。 |
| **当事人** | **本店 → 购方客户**（可为散客）。 |
| **开具方法** | ① ACTIVE **FS** 系列。② 账单同步 / 手工默认多为 FS；含税总额 > €100 时签发路径升为 FT（见现网 `ApplyFSGrossLimit`）。③ **不可**经账单同步选 FR。④ 签发 → 打印。⑤ Admin 软推荐**只升不降**：填有效 NIF 或非现金付款时 FS → FT；从不把已选的 FT 降回 FS，也不改 FR / PF / GR / GT。 |
| **NIF** | **可选**；空 → 散客 `999999990` / Consumidor Final。 |
| **不可当** | 运输单；P0 不做「FS 兼 GT」。 |
| **可被** | NC / ND；**不可**开 RG（P0 RG 仅 FT）。 |
| **权威** | [`fiscal-m6-fs-fr-nd.zh.md`](fiscal-m6-fs-fr-nd.zh.md) |

---

## 3. FR — Fatura-recibo（发票-收据）

| 项 | 定法 |
|----|------|
| **用途** | 开票同时证明已收讫（销售发票 + 收据语义）。 |
| **当事人** | **本店 → 购方客户**。 |
| **开具方法** | ① ACTIVE **FR** 系列（可选注册；不挡 `ready_to_issue`）。② **仅手工 / Local API** 选 FR；账单同步 **拒绝** FR。③ 签发同 `IssueDocument`。 |
| **NIF** | 与手工销售票同规则。 |
| **可被** | NC / ND；不可 RG。 |
| **权威** | [`fiscal-m6-fs-fr-nd.zh.md`](fiscal-m6-fs-fr-nd.zh.md) |

---

## 4. NC — Nota de crédito（贷记 / 冲销）

| 项 | 定法 |
|----|------|
| **用途** | 冲减已开销售票（退货、折扣、开错等）。 |
| **当事人** | **本店 → 原票购方**（客户快照复制自原票，冲销 UI 不可改客户）。 |
| **开具方法** | ① ACTIVE **NC** 系列。② 原票（FT/FS/FR）详情 → 冲销（全额/按行）。③ `IssueCreditNote` → `IssueNC`；金额 ≤ 行/票可冲余额。④ 打印展示原票引用。 |
| **权限** | 须 `can_issue_nc`。 |
| **权威** | [`fiscal-m3-nc.zh.md`](fiscal-m3-nc.zh.md) |

---

## 5. ND — Nota de débito（借记 / 补收）

| 项 | 定法 |
|----|------|
| **用途** | 在已开销售票上补记加收（运费、差额等）。 |
| **当事人** | **本店 → 原票购方**（同 NC，复制快照）。 |
| **开具方法** | ① ACTIVE **ND** 系列。② 原票详情 → 借记（与 NC 同调整 UI）。③ `IssueDebitNote` → `IssueND`。④ P0 **不以**原票 gross 为累计天花板。 |
| **权限** | 与 NC 同（`can_issue_nc`）。 |
| **权威** | [`fiscal-m6-fs-fr-nd.zh.md`](fiscal-m6-fs-fr-nd.zh.md) |

---

## 6. RG — Recibo（收款收据）

| 项 | 定法 |
|----|------|
| **用途** | 已开 **FT** 挂账后，客户事后付款 → 开 RG 证明收款（可分期多张）。 |
| **当事人** | **本店 → 原 FT 购方**。 |
| **开具方法** | ① ACTIVE **RG** 系列。② 仅针对 **FT**；FT 详情「登记收款」或 `POST .../receipts`。③ 单次金额 ≤ 未收余额；付款方式禁止 ACCOUNT。④ 进 SAF-T Payments，**不**进 SalesInvoices。 |
| **不可** | 对 FS/FR 开 RG；无原票裸收据；用 RG 替代 FR。 |
| **权威** | [`fiscal-rg.zh.md`](fiscal-rg.zh.md) |

---

## 7. PF — Fatura pró-forma（形式发票）

| 项 | 定法 |
|----|------|
| **用途** | 报价 / 形式确认；**无纳税义务**；客户确认后可再开真票。 |
| **当事人** | **本店 → 潜在购方**。 |
| **开具方法** | ① ACTIVE **PF** 系列。② **仅手工**开 PF（不接收银账单）。③ 票面须含 Pró-forma +「Este documento não serve de fatura」。④ 可选：开 FT/FS/FR 时挂该 PF（或从 PF 详情「开真票」预填并自动挂：按 PF **原明细**数量 / 含税单价 / 税率与购方预填，票种默认 FT 可改）。⑤ 未挂真票可作废（`N`→`A`）；已 `F` 禁止作废。 |
| **NIF** | 与现网手工 FT 同规则。 |
| **关联** | 1 PF → 多张真票；1 真票 → 最多 1 PF；金额不必相等。 |
| **权威** | [`fiscal-pf.zh.md`](fiscal-pf.zh.md) |

---

## 8. GR — Guia de remessa（发货 / 交货单）【已落地 · AT test 已通】

> **法律依据（摘要）：** DL 147/2003（Regime de Bens em Circulação）art. 4.º；AT FAQ「Documentos de Transporte」（GR 与 GT **内容要素相同**，名称随商业习惯）。  
> **实现权威设计文：** [`fiscal-gr.zh.md`](fiscal-gr.zh.md)（定稿；含实现分刀与 AT 测试接入）。

### 8.1 用途

- **批发 / 送货：** 货先交给客户，当场开 GR 证明交货；税票（FT）可以后开。  
- 典型：先开一张或多张 GR，再开一张 FT，并在开 FT 时**可选**挂上这些 GR。  
- **堂食不使用** GR；结账流不强制插 GR。

### 8.2 当事人（谁开给谁）

| 角色 | 是谁 |
|------|------|
| 开具方（remetente） | **本店**（纳税人；名称/地址/NIF 必填） |
| 收件方（destinatário） | **收货客户**（名称与住所/总部 **必填**） |

不是开给税务局当销售发票；也不是收款收据。

### 8.3 收货方 NIF（官方定法 → 产品 P0）

依据 RBC art. 4.º n.º 2 c) 与 [AT FAQ 5423](https://info.portaldasfinancas.gov.pt/pt/faturas/Pages/faqs-01005.aspx)：

| 收货方 | NIF |
|--------|-----|
| IVA 纳税人（CIVA art. 2.º） | **必填真实 NIF** |
| 非 IVA 纳税人 / 消费者 | **可不填 NIF**，但单据须写明「非 IVA 纳税人」类声明；名称+地址仍必填 |
| 禁止 | 把「空 NIF 默认散客 `999999990`」当 GR 主路径（与堂食 FS 不同）；收货方名称为空或填 `Consumidor Final` 一律拒绝（`recipient_name_required`，本地与服务端同一规则） |
| `999999990` | 本地单据快照：非纳税人收货方 **NIF 留空**，不落 `999999990`。AT 运输报文：`CustomerTaxID` 是必填字段，现实现对非纳税人填 `999999990`（`at/transport_soap.go`），AT test 2026-11-03 接受（GT /4、GR /3）——**待与 AT 运输 WS 手册核对**是否为官方口径 |

### 8.4 开具方法（产品）

```text
1. 注册独立 ACTIVE GR 系列（AT validation_code）
2. Admin 手工开 GR（不接收银账单）
3. 填：行（品名/数量；金额可有）、收货客户、装货地、卸货地、运输开始日期时间（RBC 要素）
4. 签发 → 独立 GR 系列与 Hash 链 → 进 SAF-T 4.2 MovementOfGoods（MovementType=GR）→ 可打印
5. （可选）之后开 FT 时挂 1..N 张 GR；也可永不转 FT
```

| 项 | P0 定法 |
|----|---------|
| 与 FT | **开 FT 时可选关联** 1..N 张 GR；可不选；无 GR 也可直接开 FT |
| 金额 | 与后续 FT **不必**相等、不必一张对一张 |
| 一张 GR 多张 FT | **允许**（部分开票）；第一张挂上后 GR 标已开票态（如 `F`），之后仍可再挂 |
| 未开 FT | **允许**长期存在；可做「未开票 GR」提醒；**不**自动开 FT、**不**跨月自动作废 |
| 作废 | **仅尚未挂任何真票**时可作废（须 reason）；已挂 FT 禁止作废 |
| 退货 | 已开 FT → **NC** 冲 FT；不做 GD |
| AT 运输实时通信 | **签发前必须成功**（见 [`fiscal-gr.zh.md`](fiscal-gr.zh.md) §7–§8）；与 GT 共用通信客户端 |
| 全局运输单 | **P0 不做** |

### 8.5 开 FT 时如何「选挂」GR

- **规则：只挂同一购方的 GR**。有真实 NIF 时，FT 购方 NIF = GR 收货方 NIF。都没有真实 NIF 时，FT 购方名称 = GR 收货方名称。签发写路径 `linkDeliveryNotesInTx` 核对；列表仍只按 NIF 过滤（`GET /local/v1/delivery-notes/linkable?customer_nif=`，空 NIF 不返回）。  
- **入口 ①（主）：GR 详情「开发票」** → 新开 FT，按 GR **原明细**（数量 / 含税单价 / 税率）与收货方预填，默认已挂本 GR（可移除）。  
- **入口 ②：签发 FT 时补挂** → 仅当票种 FT 且购方 NIF 有效时出现；该客户的可挂 GR 默认**折叠**（「该客户有 N 张可挂发货单」），行显示签发时刻 · 金额 · 状态（未开票 / 已部分开票）；改 NIF 时清掉不属于新购方的已挂项。  
- 可挂状态：`N`、`F`（`domain.LinkableDeliveryNoteStatuses`）；不选也可开 FT。  
- 落库：FT → GR 引用表（多对多）；签后禁止补挂/解挂（与 PF 同思路）。  
- 金额/行 **不做**与 GR 轧平校验。  
- 已知缺口：FT 表单无购方地址栏，从 GR 开发票时 GR 收货地址**不带入** FT（与 §1 纳税人购方地址问题同源，待设计）。

---

## 9. GT — Guia de transporte（运输单）【已落地 · AT test 已通】

> **法律依据（摘要）：** 同 §8（RBC）；与 GR 法定要素相同。  
> **实现权威设计文：** [`fiscal-gt.zh.md`](fiscal-gt.zh.md)（定稿；mock 可开发；live 见 [`fiscal-at-live-integration.zh.md`](fiscal-at-live-integration.zh.md) A2）。  
> **落地状态：** mock T1–T4 完成；live 2026-11-03 在 AT test 签发调拨（GT /1–/2）与外送（GT /3 纳税人、/4 非纳税人）拿到真 `ATDocCodeID`。

### 9.1 用途

- **货在途合规：** 仓→店、店→店调拨；外送需要独立运输单时。  
- **调拨可永远无 FT**（货仍属本店，无销售）。  
- **不能**用 FS 冒充 GT。

### 9.2 当事人（谁开给谁）

| 场景 | 开具方 | 收件方 / 卸货方 |
|------|--------|-----------------|
| 仓↔店、店↔店调拨 | 本店 | **本店另一场所**（同一纳税人；NIF=本店，不是客人） |
| 外送到客户 | 本店 | **客户地址**（名称+地址必填；NIF 规则同 §8.3） |

GT 证明「在途」，不是销售发票；外送时销售另开 FS/FT，两张**无强制引用**。

### 9.3 开具方法（产品）

```text
1. 注册独立 ACTIVE GT 系列
2. AT 运输通信可用（否则拒绝签发）
3. Admin 手工开 GT：行 + 装货地 + 卸货地 + 开运时间 + 收货方身份
4. 上路前签发 → 先通信 AT → 成功拿到文档码再落库 → 打印
5. 通信失败 → 整单失败（不得静默成功）
```

| 项 | P0 定法 |
|----|---------|
| 与销售票 | **无强制关联**；外送签发后可轻提示「另开发票」（见 [`fiscal-gt.zh.md`](fiscal-gt.zh.md)） |
| 时点 | **上路前**；默认禁止「已上路补开」 |
| 金额 | 非验收重点 |
| 作废 | **可作废**：仅 `N`→`A`（须 reason）；权限同 GR；细则见 [`fiscal-gt.zh.md`](fiscal-gt.zh.md) |
| 调拨装卸地址 | 两处都必填；**允许相同**（仅提示宜不同） |
| 全局运输单 | **P0 不做**（同 GR） |
| SAF-T | 4.2，`MovementType=GT` |

### 9.4 NIF

与 §8.3 相同（RBC 对 GR/GT 要素一致）。调拨填本店 NIF，不填「客人散客」。

| 校验 | 错误码（本地预检 + 服务端同码） |
|------|------|
| 调拨 NIF ≠ 本店 | `transfer_nif_not_store` |
| 外送用本店 NIF | `customer_nif_is_store`（提示改选「调拨」） |
| 收货方名称空 / `Consumidor Final` | `recipient_name_required` |

Admin：选「调拨」自动带出本店 NIF / 名称；离开调拨（切票种或改外送）撤回预填。「收货方是 IVA 纳税人」由 NIF 决定：有效 NIF 自动勾选，空或无效自动取消。

---

## 10. 堂食主路径（对照）

| 步骤 | P0 |
|------|-----|
| 默认真票 | **FS**（可改 FT；超限额升 FT） |
| 结账强制 PF / GR / GT / RG | **否** |
| 冲销 / 借记 | 有原销售票后，在发票详情操作 |
| 挂账收款 | 仅 FT → RG |

---

## 11. 权威实现文索引

| 类型 | 权威设计文 |
|------|------------|
| FT 票面 | [`fiscal-ft-receipt-layout.zh.md`](fiscal-ft-receipt-layout.zh.md) |
| NC | [`fiscal-m3-nc.zh.md`](fiscal-m3-nc.zh.md) |
| FS / FR / ND | [`fiscal-m6-fs-fr-nd.zh.md`](fiscal-m6-fs-fr-nd.zh.md) |
| RG | [`fiscal-rg.zh.md`](fiscal-rg.zh.md) |
| PF | [`fiscal-pf.zh.md`](fiscal-pf.zh.md) |
| GR | [`fiscal-gr.zh.md`](fiscal-gr.zh.md)（实现）；本节（用途） |
| GT | [`fiscal-gt.zh.md`](fiscal-gt.zh.md)；场景 [`fiscal-doc-type-test-scenarios.zh.md`](fiscal-doc-type-test-scenarios.zh.md) §10 |
| 工程顺序 | [`fiscal-dev-plan.zh.md`](fiscal-dev-plan.zh.md) |

---

## 12. 修订

| 日期 | 说明 |
|------|------|
| 2026-10-15 | 初稿定稿：九种单据用途/当事人/开具方法合订；GR NIF 与选挂 FT、作废/部分开票 P0；GT 通信门槛；全局运输单 P0 不做 |
| 2026-10-15 | GR 实现权威改挂 [`fiscal-gr.zh.md`](fiscal-gr.zh.md)；GR 签发前须 AT 运输通信 |
| 2026-10-15 | GT 实现权威改挂 [`fiscal-gt.zh.md`](fiscal-gt.zh.md)；取消「无通信方案不开刀」 |
| 2026-11-03 | GR/GT live（AT test）落地；FT NIF 改按 CIVA art. 36 写明（纳税人 vs 消费者）；FS→FT 软推荐只升不降；FT 只挂同一购方 GR + GR 详情「开发票」；GR/GT 收货方名称与 NIF 错误码；非纳税人 `999999990` 报文口径标注待核 |
