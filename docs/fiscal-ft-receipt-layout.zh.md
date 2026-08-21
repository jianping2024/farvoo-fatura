# 税票 FT 小票版式（对照零售参考票）

> **状态：定稿**  
> **权威：是**（本仓 FT 热敏票面以本文为准；出纸通道仍见工作台/schema，不改）  
> **对应实现：** `apps/fiscal-agent/internal/fiscal/print/escpos.go` 的 `RenderESCPOS`（唯一税票 ESC/POS 出口）；拉丁编码唯一 `internal/escposenc`  
> **参考样张：** 店内实测 FT（Farvoo Demo）vs 超市零售票（Pingo Doce，仅作版式参考，不抄超市业务块）  
> **写作规范：** [`design-doc-standards.zh.md`](design-doc-standards.zh.md)

---

## 1. 范围

| 做 | 不做 |
|----|------|
| 调整 **FT ORIGINAL** 热敏票面文字/分段/对齐/编码/QR | 改厨打 `station_ticket` / 收据 / 预结版式 |
| 只用已有冻结 `print.Payload` 字段 | 新造第二套 Render；另写 TCP/WinSpool |
| 让票「像一张葡萄牙合规发票」 | Logo 位图、会员卡、银行卡回单、超市部门小计 |

**出纸：** 仍走 Agent `printToTarget`（档口映射）。

---

## 2. 实票反馈（0.3.90 后仍须修）

| # | 问题 | P0 定法 |
|---|------|---------|
| 1 | 重音/特殊字符乱码（`Guaraná`→花字；联次行花字） | 全文 **Windows-1252** 编码 + `ESC t 16`（与厨打同一编码器 `escposenc.Windows1252`）；票面破折号用 ASCII `-`，不用 Unicode em dash |
| 2 | 金额折到下一行（如 `Numerario` 下孤儿金额） | 排版列宽 **32**（按窄纸有效宽）；`moneyRow` 按 **rune 宽度**计列，禁止按 UTF-8 字节数撑爆 |
| 3 | 明细第二行挤 | 第二行：`x数量  单价` 左、`行合计` 右；税率单独短标签 `IVA xx%` 夹在中间或紧跟单价，整行 ≤32 |
| 4 | 联次行异常 | 固定文案 `1a Via - Original` / `2a Via - Reprint`（ASCII 安全；`ª` 用 `a` 避免部分机对上标支持差）。仍经 1252 输出 |
| 5 | QR 截断、未居中；脚可能被切 | QR 前 `ESC a 1` 居中，打完 `ESC a 0`；模块尺寸 **4**（小于旧 6）；QR 后空 2 行再认证+Hash；切纸前再空 3 行 |

---

## 3. 票面从上到下（不变）

```text
① 商家抬头
② 单据身份（票号、日期时间、联次）
③ 客户
④ 分隔线
⑤ 明细
⑥ 分隔线
⑦ 合计 + 付款
⑧ IVA 汇总表
⑨ ATCUD
⑩ QR（居中）
⑪ 认证行 + Hash
⑫ 留白 + 切纸
```

### 3.1–3.3 抬头 / 单据 / 客户

同前：法定名、地址、`NIF: PT…`；票号禁止 `FT: FT FT…`；日期 `DD/MM/YYYY HH:MM`；联次见 §2 #4。

### 3.4 明细

两行：

1. 品名（经 1252；超长截断）  
2. 左：`x{qty}  {unit_price}  IVA {pct}`；右：`{line_gross}`（`moneyRow`，宽 32）

税率票面为百分数。

### 3.5–3.6 合计 / 付款 / IVA 表

标签用 ASCII 折叠葡语以免个别机缺字：`Liquido` / `Numerario` / `Cartao`（编码器仍走 1252；若日后改回带重音文案，只改字符串不改通道）。

IVA 表：`Taxa | Base | IVA | Total`，宽适配 32。

### 3.7 合规脚

ATCUD → 居中 QR → 认证 → Hash → 留白 → 切。认证行禁止上移票头。

---

## 4. 明确不做

- Logo、超市会员块  
- 改金额算法 / 新 SQLite 列  
- 第二套 Render 或第二套拉丁编码器（厨打与税票共用 `escposenc`）

---

## 5. 实现约束

| 项 | 定法 |
|----|------|
| 唯一渲染 | `print.RenderESCPOS` |
| 唯一拉丁编码 | `escposenc.Windows1252`（`ESC t 16`） |
| 测 | 含 1252 字节（如 `á`）、无 orphan 宽行、QR 居中命令、切纸前有 LF；`RenderESCPOS` 仍仅一份 |

---

## 6. 验收

1. 联次行、品名重音在 POS-80 上可读（非花字）  
2. 合计/付款行无「下一行孤金额」  
3. 明细第二行 ≤ 纸宽、金额右齐  
4. QR 大致居中且完整，下方可见认证 + Hash  
5. `rg 'func RenderESCPOS'` = 1；`rg 'func Windows1252'` / 唯一编码入口在 `escposenc`

---

## 7. 关联

| 文档 | 关系 |
|------|------|
| [`fiscal-sqlite-schema.zh.md`](fiscal-sqlite-schema.zh.md) | 不改表 |
| `internal/escposenc` | 拉丁编码唯一包 |
| `internal/fiscal/print/payload.go` | 字段字典 |
