# 热敏票文字编码（葡语走打印机字体，中文走位图）

> **状态：定稿**  
> **权威：是**  
> **对应实现：** `internal/escposenc`（policy）+ `internal/escposbitmap`（行级 GS v 0）+ Mesa `escposWriter.text` + `print.emitThermalLine`  
> **写作规范：** [`design-doc-standards.zh.md`](design-doc-standards.zh.md)

---

## 0. 背景（0.5.32 回退）

- 0.3.83～0.5.28：葡语重音走打印机 Font A + `ESC t 16`（WPC1252），只有中文转位图——门店出纸正常。
- 0.5.29 改成「含非 ASCII 整行转位图」：重音行改用微软雅黑位图，**丢了倍高**（厨打菜品行只剩一半高）、**比例字体导致 Qtd 列错位**、字形与其他行不同。
- 0.5.31 码页自检单在 pirata 门店打印机上实测：CP850 / CP860 / WPC1252 / CP858 **全部正确**。0.5.29 当时的「葡语乱码」已无法复现与定位（可能是别台机器或当时 `text_encoding` 配置），故回退为只有 WPC1252 装不下的字才转位图。

---

## 1. 范围

| 做 | 不做 |
|----|------|
| 厨打 / 预结 / 结账 / 正式 FT 热敏**字形** | SAF-T XML（仍 Windows-1252，分轨） |
| 葡语/英文（WPC1252 内）走 Font A；中文等（WPC1252 外）走位图 | 按语种各写补丁编码器 |
| `text_encoding` 三态 | 自检结果自动写配置 |

---

## 2. P0 定法

| # | 定法 |
|---|------|
| 1 | **`auto`（默认）逐行：** 行内有 WPC1252 装不下的字（中文、西里尔…）→ 整行 `escposbitmap.Line`（GS v 0）；否则 Font A + `ESC t 16` + Windows-1252。葡语重音与 ASCII **同字体、同倍高、同列宽**。Mesa 与 FT **同一判定**。 |
| 2 | 禁止半行「标签位图 + 金额 CP1252」混排；Han 列画布（`escposHanColumnRow` 等）是版式能力，继续唯一走画布。 |
| 3 | `text_encoding ∈ {auto, utf8, latin}`；`gbk`→`auto`。**唯一** normalize：`escposenc.NormalizeThermalEncoding`。 |
| 4 | **`utf8`**：仅手册写明 UTF-8 的机型；`ESC 9` + UTF-8。 |
| 5 | **`latin`**：整票 Windows-1252，中文被丢弃——**仅**排障；UI 文案必须标明。 |
| 6 | 原生 ESC/POS：初始化 / 切刀 / 钱箱 / QR / 对齐；**QR 禁止**渲成字位图。 |
| 7 | 葡语票面用正确 pt-PT 重音；**禁止**去重音 ASCII 化。 |
| 8 | 某店打印机 WPC1252 不认 → 先打 §5 自检单定位，再议码页切换；不要退回整行位图。 |
| 9 | 已冻结旧 payload 重打保持冻结文案——可接受。 |

---

## 3. 决策流

```text
text_encoding
├─ utf8  → ESC 9 + UTF-8
├─ latin → ESC t 16 + CP1252（排障，中文丢弃）
└─ auto  → 逐行 escposenc.NeedsRaster(s)：
           ├─ true（WPC1252 装不下：中文…）→ escposbitmap.Line（整行 GS v 0）
           └─ false（ASCII + 葡语重音） → Font A + ESC t 16 + CP1252 + LF
```

---

## 4. 唯一写法

| 职责 | 入口 |
|------|------|
| encoding normalize | `escposenc.NormalizeThermalEncoding` |
| auto 下行是否位图 | **仅** `escposenc.NeedsRaster` |
| 拉丁编码 | **仅** `escposenc.Windows1252` |
| 行级位图 | **仅** `escposbitmap.Line`（Mesa `escposBitmapText` / FT `emitThermalLine` 只包一层） |
| Mesa 模式选择 | **仅** `textModeForThermal`（票级判定 `*TicketNeedsBitmap` 也只用 `NeedsRaster`） |
| Mesa 写字 | **仅** `escposWriter.text` |
| FT 写字 | **仅** `emitThermalLine` |
| FT 流前缀 | **仅** `receiptStreamBegin`（跟 `thermalEncoding`） |
| 配置写入 FT | **仅** `SetThermalEncoding` / `applyThermalEncodingFromConfig` |

---

## 5. 码页自检单（诊断，不改出纸策略）

**目的：** 确认某台打印机固件到底认哪个 `ESC t` 码页。0.5.31 前只试过 `ESC t 16`（WPC1252）、`ESC 9`（UTF-8）、GBK；CP850 / CP860 / CP858 未试过。pirata 门店实测四种全部正确（见 §0）。

| 项 | 定法 |
|----|------|
| 入口 | 设置页「打印码页自检单」→ **复用** `/api/test-print`，`kind: "code_page"`；不另开路由 |
| 票面 | 标题/说明纯 ASCII；**参照行**＝`CodePageProbeSample` 的位图（固件无关）；下面每行 `ESC t n` + `n=… 名称` + 同一串葡语字母按该码页编码 |
| 候选 | `escposenc.CodePageCandidates`：2 CP850 / 3 CP860 / 16 WPC1252 / 19 CP858 |
| 中文段 | 第二张参照位图（`HanProbeSample`）+ 三行：**A** 直发 GBK（开机即汉字模式的机型）/ **B** `FS &` + GBK + `FS .` / **C** `ESC 9 1` + UTF-8；末尾 `ESC @` + `ESC t 16` 复位，避免卡在汉字/UTF-8 模式 |
| 读法 | 葡语段：与参照一致的 `n=` 即该机可用码页。中文段：与参照一致的 A/B/C 即该机可用汉字指令；全部不一致 → 中文继续走位图 |
| 不做 | 自检结果**不**自动写配置、**不**改变 `auto` 出纸路径 |

| 职责 | 唯一入口 |
|------|----------|
| 自检单票体（各码页行） | `escposenc.CodePageProbeRows` |
| 自检单票体（中文行） | `escposenc.HanProbeRows` |
| 自检单整票 | `buildCodePageProbe` |
| 试打类型分流 | `runTestPrintForStation(…, kind)` |
| 设置页发送 | `configure_ui.html` 的 `sendTestPrint(kind, sentKey)` |
