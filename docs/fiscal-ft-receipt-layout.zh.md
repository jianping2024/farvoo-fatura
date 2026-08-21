# 税票 FT 小票版式（对照零售参考票）

> **状态：定稿**  
> **权威：是**（本仓 FT 热敏票面以本文为准；出纸通道仍见工作台/schema，不改）  
> **对应实现：** `apps/fiscal-agent/internal/fiscal/print/escpos.go` 的 `RenderESCPOS`（唯一税票 ESC/POS 出口）；拉丁编码唯一 `internal/escposenc`  
> **参考样张：** 葡语零售热敏票（仅对齐/分段习惯；店名等数据仍用本店 payload）  
> **写作规范：** [`design-doc-standards.zh.md`](design-doc-standards.zh.md)

---

## 1. 范围

| 做 | 不做 |
|----|------|
| FT ORIGINAL：居中抬头、单行明细、编码/QR | 改厨打三票；第二套 Render / TCP |
| 只用冻结 `print.Payload` | 写死店名；Logo；手写 Nome 虚线；抄对方票号/证书号 |

**店名：** 已是参数 — `merchant.legal_name`（+ 可选 `business_name`），禁止写死参考票店名。

---

## 2. P0 定法（本刀）

| # | 定法 |
|---|------|
| 1 | **抬头居中**：法定名（可加粗）+ 经营名（若不同）+ 地址 + `NIF: PT…` 用 `ESC a 1`；打完 `ESC a 0` |
| 2 | **单行明细**：表头 `Qtd Preco IVA%-Desc` … `Soma`；每行 `qtyx unit vat%-name` 左、`line_gross` 右（`moneyRow`，宽 32）；**禁止**品名独占一行再跟金额行 |
| 3 | 票号/日期/联次、客户、合计/付款、IVA 表：仍 **左对齐**（客户位置本刀不挪） |
| 4 | 编码 / 联次 / QR / 切前留白：沿用既有（1252、`1a Via - Original`、QR 居中 module=4） |

---

## 3. 票面顺序

```text
① 商家抬头（居中）
② 单据身份（左：票号、日期、联次）
③ 客户（左）
④ 分隔线 + 明细表头 + 单行明细
⑤ 分隔线 + Liquido / IVA / TOTAL / 付款
⑥ Resumo IVA
⑦ ATCUD（左）
⑧ QR（居中）
⑨ 认证 + Hash（左；本刀不强制居中脚）
⑩ 留白 + 切纸
```

### 3.1 抬头

| 行 | 内容 | 对齐 |
|----|------|------|
| 法定名 | `merchant.legal_name` | 居中；可 `ESC E 1` |
| 经营名 | 与法定名不同才印 | 居中 |
| 地址 | `merchant.address` | 居中 |
| NIF | `NIF: PT` + 税号 | 居中 |

### 3.2 单据 / 客户

票号禁止 `FT: FT FT…`；日期 `DD/MM/YYYY HH:MM`；联次 `1a Via - Original` / `2a Via - Reprint`。

### 3.3 明细（单行）

| 列区 | 定法 |
|------|------|
| 数量 | payload `quantity` + 后缀 `x`（如 `2.00x`） |
| 单价 | `unit_price_gross` 原样 |
| 税+品名 | `{百分数}-` + 品名（`display_name`，空则 `description`）；超长截断 |
| 行合计 | `line_gross` **右齐** |

税率：payload 小数 → 票面百分数；禁止票面 `0.23`。

### 3.4 合计 / IVA / 合规

`Liquido` / `IVA` / `TOTAL` / 付款 `moneyRow`；IVA 表 `Taxa|Base|IVA|Tot`；ATCUD → QR → 认证 → Hash → 切。

---

## 4. 明确不做

- Logo、手写客户虚线、宣传语、对方 `FAC A/…` / 证书号  
- 第二套 `RenderESCPOS` 或第二套拉丁编码器  

---

## 5. 实现约束

| 项 | 定法 |
|----|------|
| 唯一渲染 | `print.RenderESCPOS` |
| 唯一拉丁 | `escposenc.Windows1252` + `ESC t 16` |
| 测 | 抬头含 `ESC a 1`；明细单行含 `23%-` 与右齐金额；无两行品名模式；`RenderESCPOS` = 1 |

---

## 6. 验收

1. 真机抬头大致居中  
2. 明细一行可读，金额右齐、无孤儿金额  
3. 店名为本店 setup 名，非样张店名  
4. `rg 'func RenderESCPOS'` = 1  

---

## 7. 关联

| 文档 | 关系 |
|------|------|
| `internal/escposenc` | 拉丁编码唯一包 |
| `internal/fiscal/print/payload.go` | 字段字典 |
