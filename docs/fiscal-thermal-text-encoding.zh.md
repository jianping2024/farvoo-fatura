# 热敏票文字编码（葡语 Font A，汉语固件 B+FS!，位图逃生）

> **状态：定稿（已实现 0.5.36，待门店试打）**  
> **权威：是**  
> **对应实现：** `internal/escposenc`（policy + 固件汉字行）+ `internal/escposbitmap`（位图逃生）+ Mesa `escposWriter.text` + `print.emitThermalLine`  
> **写作规范：** [`design-doc-standards.zh.md`](design-doc-standards.zh.md)

---

## 0. 背景与门店实测

### 0.1 演变

- 0.3.83～0.5.28：葡语重音走打印机 Font A + `ESC t 16`（WPC1252），中文整行位图——门店出纸正常，但中文无硬件倍高。
- 0.5.29：凡非 ASCII 整行位图 → 葡语丢倍高、Qtd 错位；已回退。
- 0.5.32：`auto` = 仅 WPC1252 装不下的字（含汉字）→ 位图；葡语恢复真 `GS !` 1×2。中文仍无硬件 1×2。

### 0.2 pirata 码页自检（权威读数）

| 段 | 结果 |
|----|------|
| 葡语码页 | CP850 / CP860 / WPC1252 / CP858 **全部正确** |
| 中文段 | **B `FS &` + GBK + `FS .`** 与参照一致；A 直发 GBK、C UTF-8 **乱码** |
| SIZE 1×2 | **ZH B FS!1x2**（`FS ! 0x04`）与 **PT 1x2**（`GS ! 0x01`）同高；ZH B GS!1x2 **偏矮/畸变，不采用** |

结论：汉语不乱码 = B；汉语真 1×2 = **`FS !`，不是 `GS !`**。最优解 = 能走固件就走固件，位图退居逃生。

---

## 1. 范围

| 做 | 不做 |
|----|------|
| 厨打 / 预结 / 结账 / 正式 FT 热敏**字形**与字号 | SAF-T XML（仍 Windows-1252，分轨） |
| 葡语 Font A + `GS !`；汉语固件 B + `FS !`；位图逃生 | 自检结果自动写配置 |
| `text_encoding` 四态（见 §2）；同行葡语重音 + 汉字按段切换 | 整票无条件 GBK |

---

## 2. P0 定法

| # | 定法 |
|---|------|
| 1 | **葡语/英文（整行可编码进 WPC1252）**：Font A + `ESC t 16` + Windows-1252 + **`GS !` 字号**（正文 1×2 = `GS ! 0x01`，与现 `writeBody1x2` 同） |
| 2 | **汉语行（含 ≥1 汉字，其余字符为 GBK 内的中文标点/全角字符或 WPC1252 字符）**：按段输出——中文段 `GS ! 0` + `FS &` + `FS ! n` + GBK + `FS ! 0` + `FS .` + 恢复 `GS !`（与自检 ZH B FS!1x2 同字节：中文段内不留 `GS !`）；其余段（ASCII、葡语重音）照常 WPC1252 + `GS !`。如 `Observação: 少放盐`、`Água 宫保`。正文 1×2 = `FS ! 0x04`；1×1 = `FS ! 0`；标题级 2×2 = `FS ! 0x0C`（倍宽+倍高）；下划线行（备注）再或 `0x80`。中文标点/全角按 2 列计（`displayCols`） |
| 3 | **位图（整行 GS v 0）**：行内有既不在 GBK 中文、又装不进 WPC1252 的字（西里尔、emoji）；或 `text_encoding=bitmap` 强制逃生 |
| 4 | **禁止半行混排**：禁止「固件汉字标签 + Font A Qty/金额」若列宽会漂。出品联 Items/Qty：固件汉字模式用 **列宽模型**（汉字 2 列，与现 `displayWidth` 一致）空格垫 Qty；**仅**位图逃生时走 Han 列画布 |
| 5 | **`text_encoding ∈ {auto, utf8, latin, bitmap}`**；旧 `gbk`→`auto`。**唯一** normalize：`escposenc.NormalizeThermalEncoding` |
| 6 | **`auto`（默认）**：按 §3 逐行分流。Mesa 与 FT **同一判定** |
| 7 | **`utf8`**：仅手册写明 UTF-8 的机型；`ESC 9` + UTF-8 |
| 8 | **`latin`**：整票 Windows-1252，中文丢弃——**仅**排障；UI 必须标明 |
| 9 | **`bitmap`**：含汉字（及凡 `NeedsRaster`）行走旧位图路径——门店中文段非 B、或固件倍高异常时用 |
| 10 | 原生 ESC/POS：初始化 / 切刀 / 钱箱 / QR / 对齐；**QR 禁止**渲成字位图 |
| 11 | 葡语票面用正确 pt-PT 重音；**禁止**去重音 ASCII 化 |
| 12 | 新机：先打 §5 自检。中文非 B → 开 `bitmap`；SIZE 段 FS! 不同高 → 开 `bitmap` 或议机型。自检**不**自动改配置 |
| 13 | 已冻结旧 payload 重打保持冻结文案——可接受 |

---

## 3. 决策流

```text
text_encoding
├─ utf8    → ESC 9 + UTF-8
├─ latin   → ESC t 16 + CP1252（排障，中文丢弃）
├─ bitmap  → 凡 NeedsRaster → escposbitmap.Line；否则 Font A + CP1252
└─ auto    → 逐行：
             ├─ 整行可 WPC1252
             │    → Font A + ESC t 16 + CP1252 + GS! 字号 + LF
             ├─ UsesHanFirmware（含汉字，其余皆 GBK 中文或 WPC1252）
             │    → 按段：中文段 GS!0 + FS & + FS! 字号 + GBK + FS!0 + FS . + 恢复 GS!；
             │      其余段 CP1252；+ LF
             └─ 其余（西里尔、emoji…）
                  → escposbitmap.Line（GS v 0，无尾随 LF）
```

字号映射（角色 → 指令）：

| 角色 | 拉丁（Font A） | 固件汉字（B） |
|------|----------------|---------------|
| body / 1×1 | `GS ! 0x00` | `FS ! 0x00` |
| body / 1×2 | `GS ! 0x01` | `FS ! 0x04` |
| masthead / 2×2 | `GS ! 0x11` | `FS ! 0x0C` |

---

## 4. 唯一写法

| 职责 | 入口 |
|------|------|
| encoding normalize | **仅** `escposenc.NormalizeThermalEncoding` |
| auto 行路径 | **仅** `escposenc.LinePathFor` → `PathLatin1252` / `PathHanFirmware` / `PathRaster` |
| 行是否超 1252 | **仅** `escposenc.NeedsRaster`（语义不变；`bitmap` 逃生按它转位图，auto 再经 `LinePathFor` 细分） |
| 行是否固件汉字 | **仅** `escposenc.UsesHanFirmware`（字符集 `IsHanFirmwareRune`） |
| 固件汉字行字节 | **仅** `escposenc.HanFirmwareLine(s, n)`；字号 **仅** `escposenc.HanFirmwareSize(doubleW, doubleH)` |
| 拉丁编码 | **仅** `escposenc.Windows1252` |
| 行级位图 | **仅** `escposbitmap.Line` |
| Mesa 票级模式 | **仅** `textModeForThermal`（`bitmap` 配置或票内需位图逃生时进 bitmap 模式能力） |
| Mesa 写字 | **仅** `escposWriter.text` |
| FT 写字 | **仅** `emitThermalLine` |
| FT 流前缀 | **仅** `receiptStreamBegin` |
| 配置写入 FT | **仅** `SetThermalEncoding` / `applyThermalEncodingFromConfig` |

Mesa：`escposWriter.text` 在 `escposTextAuto` 下按 `LinePathFor` 分流，字号取当前 `GS !` 状态。FT：`emitThermalLine` 的 `thermalLineStyle.DoubleH` 跟 `RenderESCPOS` 的 `setDoubleH`。Han 列画布（`escposHan*Row`）只在 `escposTextBitmap` 下使用。

---

## 5. 码页自检单（诊断，不改出纸策略）

**目的：** 确认本机葡语码页、汉语 B、汉语倍高指令。入口不变：`kind: "code_page"`。

| 段 | 定法 |
|----|------|
| 葡语 | 参照位图 + CP850/860/WPC1252/CP858 |
| 中文 | 参照位图 + A / **B** / C；pirata 认 **B** |
| SIZE 1×2 + MIX | PT 1x1 / PT 1x2 / ZH B 1x1 / ZH B GS!1x2 / **ZH B FS!1x2**；认与 PT 1x2 同高者（pirata：**FS!**）。MIX 行用 auto 混排真实字节（`HanFirmwareLine`）：**MIX 1x1** / **MIX 1x2**（`Água 少放盐`）/ **MIX 1x2 U**（`Observação: 少放盐`，备注下划线）/ **MIX 2x2**（`Mesa 桌 01`，抬头）；每行葡中同高、不乱码即通过（待测） |
| 不做 | 自检结果不自动写配置 |

| 职责 | 唯一入口 |
|------|----------|
| 葡语行 | `escposenc.CodePageProbeRows` |
| 中文行 | `escposenc.HanProbeRows` |
| SIZE 行 | `escposenc.SizeProbeRows` |
| 整票 | `buildCodePageProbe` |

---

## 6. 设置页文案（实现时）

| key 语义 | 中文意图 |
|----------|----------|
| `auto` | 自动（葡语打印机字体；中文固件汉字+倍高；其它转图片） |
| `bitmap` | 中文强制图片（逃生：自检中文非 B 或倍高异常） |
| `utf8` | UTF-8（手册写明才选） |
| `latin` | 仅拉丁 / 排障（丢中文） |

---

## 7. 验收（实现后门店）

- 厨打中文菜名高度 ≈ 同票葡语 1×2  
- 中文不乱码（B）  
- 纯葡语票与现网一致（仍 `GS !`）  
- `bitmap` 逃生仍可用  
- 自检单仍可诊断新机  

---

## 8. 实现顺序（0.5.36 已完成 1–6 的代码部分；门店试打待做）

1. 本文已定稿口径（本文件）  
2. `escposenc`：`UsesHanFirmware` / `HanFirmwareLine` / 收窄 `NeedsRaster` / normalize 含 `bitmap`  
3. Mesa `escposWriter.text` + FT `emitThermalLine`  
4. 出品联列宽：固件汉字空格垫；位图逃生保留 Han canvas  
5. 设置页 + i18n  
6. 单测 + VERSION/NOTES + `./scripts/check-fiscal-agent.sh` + 门店试打  

`FS ! 0x08`（仅倍宽）未在门店验证；目前没有票面用到。
