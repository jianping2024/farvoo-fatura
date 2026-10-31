# 热敏票文字编码（葡语 Font A，汉语位图默认，固件汉字库可选）

> **状态：定稿（已实现 0.5.43）**  
> **权威：是**  
> **对应实现：** `internal/escposenc`（policy + 固件汉字行）+ `internal/escposbitmap`（位图，默认）+ Mesa `escposWriter.text` + `print.emitThermalLine`  
> **写作规范：** [`design-doc-standards.zh.md`](design-doc-standards.zh.md)

---

## 0. 背景与门店实测

### 0.1 演变

- 0.3.83～0.5.28：葡语重音走打印机 Font A + `ESC t 16`（WPC1252），中文整行位图——门店出纸正常，但中文无硬件倍高。
- 0.5.29：凡非 ASCII 整行位图 → 葡语丢倍高、Qtd 错位；已回退。
- 0.5.32：`auto` = 仅 WPC1252 装不下的字（含汉字）→ 位图；葡语恢复真 `GS !` 1×2。中文仍无硬件 1×2。
- 0.5.36：中文改固件 B + `FS ! 0x04` 当「1×2」——门店出纸中文变扁（实为倍宽），列头换行；自检 `ZH B GS!1x2` 才是瘦长。
- 0.5.37：中文倍高改与葡语同一 `GS !`；废除用 `FS !` 当字号（仍留 `FS ! 0x80` 备注下划线）。
- 0.5.38： 备注中文变矮——`FS ! 0x80` 会清掉汉字倍高；汉字段**禁止**任何 `FS !`；下划线仅 Font A `ESC -`。
- 0.5.39： 出品联改版——每道菜行首「数量× 编号」Font A 1×2 加粗 + `GS B` 反白（bitmap 逃生口改为像素反转）；副名/备注缩进到菜名下；备注前缀 `» `、加粗不再下划线。
- 0.5.40： 出品联菜品区一律不加粗（`ESC E` 让 1×2 瘦高字发糊）；试打附「数量/编号」六种打法对比条。
- 0.5.41： 出品联编号「数量× 编号」2×2 不加粗、不反白（废除 `GS B`）；首个菜名 1×2，其他语言菜名曾为 1×1（汉字段不发 `GS !`），备注 1×2。现网：全部菜名行均为 1×2（与 [`station-slip-print-names.zh.md`](station-slip-print-names.zh.md) 一致）。
- **0.5.42（本版）：** 门店换新品牌热敏机，固件汉字（`FS &` + GBK）乱码——固件汉字库没有跨品牌标准，打印机也无法回报是否乱码。改为 **`auto`（标准）= 中文转位图**（只依赖 `GS v 0`）；固件汉字库降为可选 `han_firmware`（自检 B 正常才选）；旧 `bitmap` 并入 `auto`。位图按 `GS !` 字号复刻固件效果：1×1 = 24px 汉字格、1×2 = 同字号逐点行复制（与打印机倍高同法）、2×2 = 48px；上下按墨迹裁紧省纸；渲染失败退回 Font A 并以 `?` 占位，不丢行。
- 0.5.43：位图出品联菜名列 = max(Font A 缩进, 编号实测宽 + 24 点)（`hanStationNameX`），雅黑数字比 2×2 字宽，旧算法贴字；设置页加编码下拉框（§6）。

### 0.2 门店码页自检（权威读数）

| 段 | 结果 |
|----|------|
| 葡语码页 | CP850 / CP860 / WPC1252 / CP858 **全部正确** |
| 中文段 | **B `FS &` + GBK + `FS .`** 与参照一致；A 直发 GBK、C UTF-8 **乱码** |
| SIZE 1×2 | **ZH B GS!1x2**（`GS ! 0x01`）与 **PT 1x2** 同高瘦长；**ZH B FS!flat**（`FS ! 0x04`）变扁/倍宽，**不采用** |

结论（0.5.38 时）：该店汉语不乱码 = B；汉语真 1×2 = **`GS !`，与葡语同令**。0.5.42 起：B 只对部分品牌成立，默认改位图，固件汉字库仅对自检 B 通过的门店开放（`han_firmware`）。

---

## 1. 范围

| 做 | 不做 |
|----|------|
| 厨打 / 预结 / 结账 / 正式 FT 热敏**字形**与字号 | SAF-T XML（仍 Windows-1252，分轨） |
| 葡语 Font A + `GS !`；汉语位图（默认，字号对齐 `GS !`）；固件 B 可选 | 自检结果自动写配置；自动探测打印机是否支持汉字库（ESC/POS 无标准回报） |
| `text_encoding` 四态（见 §2） | 整票无条件 GBK |

---

## 2. P0 定法

| # | 定法 |
|---|------|
| 1 | **葡语/英文（整行可编码进 WPC1252）**：Font A + `ESC t 16` + Windows-1252 + **`GS !` 字号**（正文 1×2 = `GS ! 0x01`，与现 `writeBody1x2` 同） |
| 2 | **汉语行（仅 `han_firmware`；含 ≥1 汉字，其余字符为 GBK 内的中文标点/全角字符或 WPC1252 字符）**：按段输出——中文段 `FS &` + **`GS !` n** + GBK + `FS .`（与自检 ZH B GS!1x2 / 菜名 / 备注汉字同一 `HanFirmwareLine`）；其余段（ASCII、葡语重音）照常 WPC1252 + `GS !`。如 `Observação: 少放盐`、`Água 宫保`。正文 1×2 = `GS ! 0x01`；1×1 = 不发 / `GS ! 0`；标题级 2×2 = `GS ! 0x11`。备注下划线：**仅** Font A `ESC -`；汉字段**禁止** `FS !`（含下划线位——门店实测会清掉倍高）。中文标点/全角按 2 列计（`displayCols`） |
| 3 | **位图（整行 GS v 0，`auto` 默认）**：凡 `NeedsRaster` 行（汉字、西里尔、emoji…）。字号按 `escposbitmap.Scale`：基准 `han_bitmap_font_px`（Agent `config.json`，默认 **22**）= 1×1 汉字格；1×2 = 同字号逐点行 ×2；2×2 = 2× 基准渲染。整行图按墨迹上下裁紧（上 2 / 下 4 点）。渲染失败 → 本行 Font A + `?` 占位（`Windows1252Placeholder`），**禁止丢行**。云端 job payload 旧字段忽略 |
| 4 | **禁止半行混排**：位图是整行图，不能与 Font A 同行。`auto` 下出品联**按菜逐行**判断（`stationItemUsesHanCanvas`）：首菜名需位图时，编号 2×2 + 菜名 1×2 同一张 **Han 列画布**（576 点标尺，底部对齐）；否则编号/菜名走 Font A，副名/备注里需位图的行单独成图、按 Font A 名称列的点位左对齐（`writeStationHangLine`）。账单列仍走 Han 列画布。GDI 渲染用 `NONANTIALIASED_QUALITY`（抗锯齿边在 1-bit 阈值下会把笔画加粗）；`han_firmware` 用列宽模型空格垫 |
| 5 | **`text_encoding ∈ {auto, han_firmware, utf8, latin}`**；旧 `gbk` / `bitmap`→`auto`。**唯一** normalize：`escposenc.NormalizeThermalEncoding` |
| 6 | **`auto`（默认，界面名「标准」）**：按 §3 逐行分流。Mesa 与 FT **同一判定**、同一字号映射 |
| 7 | **`utf8`**：仅手册写明 UTF-8 的机型；`ESC 9` + UTF-8 |
| 8 | **`latin`**：整票 Windows-1252，中文丢弃——**仅**排障；UI 必须标明 |
| 9 | **`han_firmware`**：0.5.38–0.5.41 的固件汉字路径——仅门店自检中文 B 正常、且要打印机字形时选 |
| 10 | 原生 ESC/POS：初始化 / 切刀 / 钱箱 / QR / 对齐；**QR 禁止**渲成字位图 |
| 11 | 葡语票面用正确 pt-PT 重音；**禁止**去重音 ASCII 化 |
| 12 | 默认 `auto` 无需自检即可出中文。想改用打印机汉字库：先打 §5 自检，中文 B 正常且 SIZE 段 GS! 同高才开 `han_firmware`。自检**不**自动改配置 |
| 13 | 已冻结旧 payload 重打保持冻结文案——可接受 |

---

## 3. 决策流

```text
text_encoding
├─ utf8    → ESC 9 + UTF-8
├─ latin   → ESC t 16 + CP1252（排障，中文丢弃）
├─ auto（默认）→ 逐行：
│            ├─ 整行可 WPC1252 → Font A + ESC t 16 + CP1252 + GS! 字号 + LF
│            └─ NeedsRaster → escposbitmap.Line（Scale 字号，GS v 0，无尾随 LF）
│                 失败 → Windows1252Placeholder + LF
└─ han_firmware → 逐行：
             ├─ 整行可 WPC1252
             │    → Font A + ESC t 16 + CP1252 + GS! 字号 + LF
             ├─ UsesHanFirmware（含汉字，其余皆 GBK 中文或 WPC1252）
             │    → 按段：中文段 FS & + GS! 字号 + GBK + FS .；
             │      其余段 CP1252；+ LF
             └─ 其余（西里尔、emoji…）
                  → escposbitmap.Line（GS v 0，无尾随 LF）
```

字号映射（角色 → 指令；三条路径同一 `GS !` 语义）：

| 角色 | 拉丁（Font A） | 位图（auto，基准 24） | 固件汉字（han_firmware） |
|------|----------------|------------------------|---------------|
| 1×1 | `GS ! 0x00` | 24px | （不发 / 同） |
| 1×2 | `GS ! 0x01` | 24px，逐点行 ×2 | `GS ! 0x01` |
| masthead / 2×2 | `GS ! 0x11` | 48px | `GS ! 0x11` |
| note underline | `ESC - 1` | 字体下划线 | （不发；与菜名同 `GS !` 字节） |

---

## 4. 唯一写法

| 职责 | 入口 |
|------|------|
| encoding normalize | **仅** `escposenc.NormalizeThermalEncoding` |
| auto 行路径 | **仅** `escposenc.LinePathFor` → `PathLatin1252` / `PathHanFirmware` / `PathRaster` |
| 行是否超 1252 | **仅** `escposenc.NeedsRaster` |
| 行是否固件汉字 | **仅** `escposenc.UsesHanFirmware`（字符集 `IsHanFirmwareRune`） |
| 固件汉字行字节 | **仅** `escposenc.HanFirmwareLine(s, gsBang)`；`gsBang`=`FontASize` |
| 拉丁编码 | **仅** `escposenc.Windows1252` |
| 行级位图 | **仅** `escposbitmap.Line` |
| 位图字号 | **仅** `escposbitmap.Scale`（Han 画布经 `hanFontPxForRole` / `hanStretchRow` 调用） |
| 位图失败兜底 | **仅** `escposenc.Windows1252Placeholder`（Mesa `writeRasterOr`、FT `emitThermalRaster`） |
| Mesa 票级模式 | **仅** `textModeForThermal` |
| Mesa 写字 | **仅** `escposWriter.text` |
| FT 写字 | **仅** `emitThermalLine` |
| FT 流前缀 | **仅** `receiptStreamBegin` |
| 配置写入 FT | **仅** `SetThermalEncoding` / `applyThermalEncodingFromConfig` |

禁止第二套「中文专用字号映射」或对汉字段发任何 `FS !`（含下划线）。Han 列画布（`escposHan*Row`）只在 `escposTextBitmap`（`auto`）下使用。

---

## 5. 码页自检单（诊断，不改出纸策略）

**目的：** 确认本机葡语码页、汉语 B、汉语倍高指令。入口不变：`kind: "code_page"`。

| 段 | 定法 |
|----|------|
| 葡语 | 参照位图 + CP850/860/WPC1252/CP858 |
| 中文 | 参照位图 + A / **B** / C；认 **B** |
| SIZE 1×2 + MIX | PT 1x1 / PT 1x2；**ZH B 1x1 / ZH B GS!1x2**（均 `HanFirmwareLine`，与出纸同字节）；**ZH B FS!flat**（对比：`FS ! 0x04`，应扁，勿当正道）。MIX 1x1 / 1x2 / **1x2 U**（`ESC -` + 同一 `HanFirmwareLine`，无 `FS !`）/ 2x2；葡中同高瘦长、不乱码即通过 |
| 不做 | 自检结果不自动写配置 |

| 职责 | 唯一入口 |
|------|----------|
| 葡语行 | `escposenc.CodePageProbeRows` |
| 中文行 | `escposenc.HanProbeRows` |
| SIZE 行 | `escposenc.SizeProbeRows`（ZH/MIX 出纸路径只调 `HanFirmwareLine`） |
| 整票 | `buildCodePageProbe` |

---

## 6. 设置页文案（实现时）

| key 语义 | 中文意图 |
|----------|----------|
| `auto` | 标准（推荐）：葡语打印机字体；中文及其它转图片 |
| `han_firmware` | 中文用打印机汉字库（需自检 B 正常） |
| `utf8` | UTF-8（手册写明才选） |
| `latin` | 仅拉丁 / 排障（丢中文） |

---

## 7. 验收（实现后门店）

- 厨打中文菜名高度 ≈ 同票葡语 1×2（瘦长，非扁宽）  
- 列头「菜品」「数量」同一行、Qty 右对齐  
- 中文不乱码（任意品牌，位图）  
- 纯葡语票与现网一致（仍 `GS !`）  
- `han_firmware` 仍可用  
- 自检单仍可诊断新机  

---

## 8. 实现顺序

1. 本文定稿口径（本文件）  
2. `escposenc.HanFirmwareLine(s, gsBang)`：字号仅 `GS !`；汉字段永不发 `FS !`  
3. Mesa `escposWriter.text` + FT `emitThermalLine`（同入口，无旁路）  
4. 自检 SIZE：ZH/MIX 只调 `HanFirmwareLine`；保留 FS!flat 对比行  
5. 单测 + VERSION/NOTES + `./scripts/check-fiscal-agent.sh` + 门店试打  
