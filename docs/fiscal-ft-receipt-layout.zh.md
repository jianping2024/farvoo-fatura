# 税票 FT 小票版式（对照零售参考票）

> **状态：定稿**  
> **权威：是**  
> **对应实现：** `print.RenderESCPOS`（唯一）；拉丁编码 `escposenc`  
> **参考样张：** VOZ / Pingo Doce（版式习惯）；店名等数据仍用本店 payload  
> **写作规范：** [`design-doc-standards.zh.md`](design-doc-standards.zh.md)

---

## 1. 范围

| 做 | 不做 |
|----|------|
| 居中抬头、单行明细、票号标签、合规脚（认证+ATCUD+QR） | 单独 `Hash:` 行；QR 下再印业务字；第二套 Render |
| 只用冻结 Payload | 写死店名/对方证号 |

---

## 2. P0 定法（本刀依据：样票原文 + 0.3.92 真机切坏）

| # | 定法 | 依据 |
|---|------|------|
| 1 | 票号：`Fatura No.: ` + `invoice_no` | VOZ：`Fatura No.: …` |
| 2 | 认证句：`{QRHashChars}-` + `Processado por… n. {证号}/AT`（票面无 `º`）；**禁止** `Hash:` 行 | VOZ `/IJ6 -- Processado…`；Pingo `XLM/-Processado…` |
| 3 | 顺序：认证 → ATCUD → QR → 留白切；QR 下无业务字 | 样票 QR 垫底；0.3.92 Hash 被横切 |
| 4 | ATCUD + QR **居中**；QR module **6** | VOZ 居中大 QR |
| 5 | 抬头居中 + 单行明细 | 既有定法，保留 |
| 6 | **TOTAL 突出**：加粗 + 倍高（`ESC E` + `GS ! 0x01`）；Liquido/IVA/付款普通 | VOZ：`TOTAL: … Euro` 明显大于邻行 |
| 7 | **票面宽 48 列**（与厨打 `escposWidth` 一致）；虚线/金额行铺满 80mm；**禁止**再缩到 32 | VOZ/Pingo 样票铺满；0.3.95 真机 32 列右边空白丑 |
| 8 | **明细表头**：`formatItemLinesHeader` 唯一；Qtd/Preco 分列宽；**虚线夹住表头**（上虚线→表头→下虚线→明细）；**上下虚线与表头各紧贴一行、中间不插空行**（间距对称） | 零售样票隔离列头；避免 Qtd Preco 贴死、表头贴明细、上下疏密不一 |
| 9 | **店名**仅 `LegalName`：加粗 + 1×2（`GS ! 0x01`），印完恢复 1×1；店名下空一行；地址 / NIF / BusinessName 仍 1×1 | 王氏抬头观感；禁止把整段抬头都放大 |

四字 = `compliance.QRHashChars`（已有）；斜杠是否出现取决于 Hash，不硬插。

---

## 3. 票面顺序

```text
① 抬头（居中）：店名 1×2 加粗 → 空一行 → 地址/NIF（1×1）
② Fatura No. / 日期 / 联次（左）
③ 客户（左）
④ 明细表：虚线 → 列头 → 虚线 → 单行明细（上下虚线与列头紧贴、不插空行）
⑤ Liquido / IVA（普通）→ **TOTAL（加粗倍高）** → 付款 → Resumo IVA
⑥ 认证句（居中，含四字前缀）
⑦ ATCUD（居中）
⑧ QR module=6（居中）
⑨ 留白 + 切纸
```

---

## 4. 唯一写法

| 项 | 定法 |
|----|------|
| 渲染 | 仅 `RenderESCPOS` |
| 票面列宽 | 仅 `receiptWidth`（=48，与厨打一致） |
| 明细表头 | 仅 `formatItemLinesHeader` |
| 明细行 | 仅 `formatItemLine` |
| 认证票面拼装 | 仅 `formatCertificationFace` |
| 票号标签行 | 仅 `formatFaturaNoLine` |
| 拉丁编码 | 仅 `escposenc.Windows1252` |

---

## 5. 验收

1. 输出含 `Fatura No.:`；无独立 `Hash:`  
2. 认证在 ATCUD/QR 之前；含四字 + `Processado`  
3. QR 后至 cut 无业务 ASCII 行  
4. `rg 'func RenderESCPOS'`=1；`Hash:` 拼接在 Render 中不存在  
5. `receiptWidth=48`；虚线与 `moneyRow` 行宽均为 48  
