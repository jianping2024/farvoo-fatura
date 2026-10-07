# 热敏票文字编码（固件无关）

> **状态：定稿**  
> **权威：是**  
> **对应实现：** `internal/escposenc`（policy）+ `internal/escposbitmap`（行级 GS v 0）+ Mesa `escposWriter.text` + `print.emitThermalLine`  
> **写作规范：** [`design-doc-standards.zh.md`](design-doc-standards.zh.md)

---

## 1. 范围

| 做 | 不做 |
|----|------|
| 厨打 / 预结 / 结账 / 正式 FT 热敏**字形** | SAF-T XML（仍 Windows-1252） |
| 中文、葡语重音、其他非 ASCII | 依赖杂牌机 `ESC t`≈Epson WPC1252 |
| `text_encoding` 三态 | 按语种各写补丁编码器 |

---

## 2. P0 定法

| # | 定法 |
|---|------|
| 1 | **默认不信任打印机码页。** `auto` 下：含非 ASCII 的**整行** → TrueType → **GS v 0**（与中文同位策略）。禁止半行「标签位图 + 金额 CP1252」混排。 |
| 2 | **纯 ASCII 行**走 ESC/POS Font A（各码页一致）。 |
| 3 | `text_encoding ∈ {auto, utf8, latin}`；`gbk`→`auto`。**唯一** normalize：`escposenc.NormalizeThermalEncoding`。 |
| 4 | **`auto`（默认）**：非 ASCII 整行位图。Mesa 与 FT **同一规则**。Han/列画布本身已是位图，继续唯一走画布，不夹 CP1252 重音。 |
| 5 | **`utf8`**：仅手册写明 UTF-8 的机型；`ESC 9` + UTF-8。**禁止**当葡语默认解法。 |
| 6 | **`latin`**：整票 `ESC t 16` + Windows-1252。**仅**确认固件认 WPC1252 的 Epson 类 / 排障对照；UI 文案必须标明，**不是**门店默认。 |
| 7 | 原生 ESC/POS：初始化 / 切刀 / 钱箱 / QR / 对齐；**QR 禁止**渲成字位图。 |
| 8 | 葡语票面用正确 pt-PT 重音；**禁止**去重音 ASCII 化掩盖码页问题。 |
| 9 | SAF-T / 合规校验仍 Windows-1252；与热敏**分轨**。 |
| 10 | 生产字形依赖 Windows GDI（与现中文路径同）；非 Windows stub 只锁指令流。 |
| 11 | 已冻结旧 payload 重打保持冻结文案（可能仍无重音）——可接受。 |

---

## 3. 决策流

```text
text_encoding
├─ utf8  → ESC 9 + UTF-8
├─ latin → ESC t 16 + CP1252（排障）
└─ auto  → 逐行：
           ├─ 含非 ASCII → escposbitmap.Line（整行 GS v 0）
           └─ 仅 ASCII  → Font A + LF
```

---

## 4. 唯一写法

| 职责 | 入口 |
|------|------|
| encoding normalize | `escposenc.NormalizeThermalEncoding` |
| 非 ASCII 判定 | `escposenc.HasNonASCII` |
| auto 下行是否位图 | `escposenc.LineUsesRaster` |
| 行级位图 | **仅** `escposbitmap.Line`（Mesa `escposBitmapText` / FT `emitThermalLine` 只包一层） |
| Mesa 模式选择 | **仅** `textModeForThermal` |
| Mesa 写字 | **仅** `escposWriter.text` |
| FT 写字 | **仅** `emitThermalLine` |
| FT 流前缀 | **仅** `receiptStreamBegin`（跟 `thermalEncoding`） |
| 配置写入 FT | **仅** `SetThermalEncoding` / `applyThermalEncodingFromConfig` |

列对齐 Han 画布（`escposHanColumnRow` 等）是**版式**能力，不是第二套拉丁编码器；禁止在画布路径再塞 CP1252 重音。

---

## 5. 码页自检单（诊断，不改出纸策略）

**目的：** 确认某台打印机固件到底认哪个 `ESC t` 码页。历史上只试过 `ESC t 16`（WPC1252）、`ESC 9`（UTF-8）、GBK，门店杂牌机全部乱码；CP850 / CP860 / CP858 未试过。

| 项 | 定法 |
|----|------|
| 入口 | 设置页「打印码页自检单」→ **复用** `/api/test-print`，`kind: "code_page"`；不另开路由 |
| 票面 | 标题/说明纯 ASCII；**参照行**＝`CodePageProbeSample` 的位图（固件无关）；下面每行 `ESC t n` + `n=… 名称` + 同一串葡语字母按该码页编码 |
| 候选 | `escposenc.CodePageCandidates`：2 CP850 / 3 CP860 / 16 WPC1252 / 19 CP858 |
| 读法 | 与参照图片字母完全一致的那一行即该机可用码页；全部不一致 → 该机继续走 §2 位图 |
| 不做 | 自检结果**不**自动写配置、**不**改变 `auto` 出纸路径 |

| 职责 | 唯一入口 |
|------|----------|
| 自检单票体（各码页行） | `escposenc.CodePageProbeRows` |
| 自检单整票 | `buildCodePageProbe` |
| 试打类型分流 | `runTestPrintForStation(…, kind)` |
| 设置页发送 | `configure_ui.html` 的 `sendTestPrint(kind, sentKey)` |
