# 税票 FT 小票版式（对照零售参考票）

> **状态：定稿**（P0 版式已落地）  
> **权威：是**（本仓 FT 热敏票面以本文为准；出纸通道仍见工作台/schema，不改）  
> **对应实现：** `apps/fiscal-agent/internal/fiscal/print/escpos.go` 的 `RenderESCPOS`（唯一税票 ESC/POS 出口）  
> **参考样张：** 店内实测 FT（Farvoo Demo）vs 超市零售票（Pingo Doce，仅作版式参考，不抄超市业务块）  
> **写作规范：** [`design-doc-standards.zh.md`](design-doc-standards.zh.md)

---

## 1. 范围

| 做 | 不做 |
|----|------|
| 调整 **FT ORIGINAL** 热敏票面文字/分段/对齐 | 改厨打 `station_ticket` / 收据 / 预结版式 |
| 只用已有冻结 `print.Payload` 字段（能印的先印） | 新造第二套 Render；另写 TCP/WinSpool |
| 让票「像一张葡萄牙合规发票」 | Logo 位图、会员卡、银行卡签购单、超市部门小计 |

**出纸：** 仍走 Agent `printToTarget`（档口映射）；本文只定 **印什么、什么顺序**。

---

## 2. 现状问题（人话）

当前 `RenderESCPOS` 是调试小票，主要问题：

1. 没有开票日期时间  
2. 票号打成 `FT: FT FT…`，重复难看  
3. 明细不对齐，数量/单价/金额难对  
4. 税率印成 `0.23`，应给人看的是 **23%**  
5. 没有「按税率汇总」税表（算过、payload 有，没印）  
6. 没写付款方式  
7. 认证行 / Hash / QR 顺序别扭，不像正规票脚  

---

## 3. P0 定法：票面从上到下

唯一顺序（禁止再颠倒合规块与合计块）：

```text
① 商家抬头
② 单据身份（类型、票号、日期时间、联次）
③ 客户
④ 分隔线
⑤ 明细行（对齐）
⑥ 分隔线
⑦ 合计 + 付款
⑧ IVA 汇总表
⑨ ATCUD
⑩ QR
⑪ 认证行 + Hash 控制字符
⑫ 切纸
```

### 3.1 商家抬头

| 行 | 内容 | 来源 |
|----|------|------|
| 1 | 法定名称 | `merchant.legal_name` |
| 2 | 经营名称（与法定名不同才印） | `merchant.business_name` |
| 3 | 地址 | `merchant.address` |
| 4 | `NIF: PT` + 税号 | `merchant.tax_registration_number` |

### 3.2 单据身份

| 行 | 内容 | 定法 |
|----|------|------|
| 类型+票号 | 一行：`FT` + 空格 + `invoice_no` 中**已含系列的正式票号** | **禁止**再拼出 `FT: FT FT…`。若 `invoice_no` 已以 `FT ` 开头，票面只印该字符串；类型字样与票号不重复堆前缀。 |
| 日期时间 | 本地开票时分（可读） | 用 `issued_at`，按纳税人时区格式化为 `DD/MM/YYYY HH:MM`（与 PT 零售习惯一致） |
| 联次 | `1ª Via — Original`（ORIGINAL）；重打另文 | `print_purpose=ORIGINAL` 时固定此文案 |

### 3.3 客户

| 行 | 内容 |
|----|------|
| 客户名 | `Cliente: ` + `customer.company_name` |
| 客户 NIF | `NIF Cliente: ` + `customer.tax_id`（有则印） |

### 3.4 明细

每一品名占 **两行**（80mm 热敏可读优先）：

1. 品名（`display_name`，空则 `description`）  
2. 数量 × 单价(含税) + 税率% + 行合计(含税)，**金额右对齐倾向**（同列尽量竖齐）

| 字段展示 | 定法 |
|----------|------|
| 数量 | `quantity`，前缀 `x` 或 `×` |
| 单价 / 行合计 | 原文字符串，不改货币算法 |
| 税率 | payload 存小数（如 `0.23`）→ **票面印百分数**（`23%`）；禁止票面直接印 `0.23` |

本刀不印税码列（NOR/RED）；以后可加。

### 3.5 合计 + 付款

| 行 | 内容 |
|----|------|
| 净额 | `Líquido` + `totals.net_total` |
| 税额 | `IVA` + `totals.tax_payable` |
| 总额 | `TOTAL` + `totals.gross_total`（可略强调，仍 ESC/POS 文本） |
| 付款 | 每个 `payments[]`：`method` 文案 + `amount`；`CASH`→`Numerário`（或中英已有约定则跟 Agent UI 语言；**P0 默认葡语零售常用：Numerário / Cartão**） |

无付款数组时：不印付款块（不应发生；签发侧默认有全额支付）。

### 3.6 IVA 汇总表（P0 必做）

用 `tax_summary[]`，表头 + 每档一行：

| 列 | 含义 |
|----|------|
| Taxa | 百分数，如同明细 |
| Base | `tax_base`（不含税） |
| IVA | `tax_amount` |
| Total | `gross` |

无多档时仍印表（一行也要印），避免「只有总额看不出税率结构」。

### 3.7 合规脚

| 行 | 内容 |
|----|------|
| ATCUD | `ATCUD: ` + 值 |
| QR | ESC/POS 原生 QR，内容 = `compliance.qr.content` |
| 认证 | `compliance.certification_line`（已含 AT 证号句式） |
| Hash | `Hash: ` + `hash_control_chars`（控制字符，非全文 hash） |

**禁止**把认证行挪到票头。

---

## 4. 明确不做（本刀）

- 店招 Logo / 图形头  
- 抄 Pingo Doce 的会员、燃油积分、银行卡回单、部门小计  
- 改 `BuildPayload` 金额算法或 AT 字段含义  
- 为版式新增 SQLite 列（现有 payload 够用）  
- NC / 重打专用版式（可复用本文骨架，另开短文）

---

## 5. 实现约束

| 项 | 定法 |
|----|------|
| 唯一渲染出口 | `print.RenderESCPOS` |
| 输入 | 仅冻结 `Payload` v1 |
| 测 | 单测：固定 payload → 输出须含日期、IVA 表头、百分数税率、认证在 QR 之后；禁止出现 `FT FT FT` 式重复前缀 |
| 回归 | 开一张 demo FT，肉眼对 §3 顺序 |

---

## 6. 验收清单（落地后勾）

1. [x] 票上有 `DD/MM/YYYY HH:MM`  
2. [x] 票号行无不必要的双重 `FT`  
3. [x] 明细税率可见为 `xx%`  
4. [x] 有 IVA 汇总表（有表头）  
5. [x] 有付款行（有 payments 时）  
6. [x] 顺序：合计/税表 → ATCUD → QR → 认证 → Hash → 切纸  
7. [x] 厨打三票版式零改动；出纸仍 `printToTarget`  
8. [x] 单测 `TestRenderESCPOS_LayoutP0`

---

## 7. 关联

| 文档 | 关系 |
|------|------|
| [`fiscal-sqlite-schema.zh.md`](fiscal-sqlite-schema.zh.md) | 签发与 `local_print_jobs`；本文不改表 |
| [`fiscal-bill-draft-workbench.zh.md`](fiscal-bill-draft-workbench.zh.md) | 谁触发开票；本文只管印出来长什么样 |
| `internal/fiscal/print/payload.go` | 字段字典；渲染不得臆造 payload 没有的键 |
