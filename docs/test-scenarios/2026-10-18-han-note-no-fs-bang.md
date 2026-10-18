# 本次改动业务场景测试报告：备注汉字禁止 FS !（与菜名同高）

日期：2026-10-18  
分支：`feat/han-note-no-fs-bang`  
权威设计：[`docs/fiscal-thermal-text-encoding.zh.md`](../fiscal-thermal-text-encoding.zh.md)  
版本：`0.5.38`

## 1. 本次修改

- **内容：** 门店出品联「中水」瘦长、「Observação: 多冰」中文变矮。根因是备注对汉字发 `FS ! 0x80` 清掉倍高。
- **改动点：** `HanFirmwareLine(s, gsBang)` 去掉 `hanMode`/`HanUnderline`；Mesa `escposWriter.text` 不再因 underline 拼 `FS !`；自检 MIX 1x2 U 仅 `ESC -` + 同一 `HanFirmwareLine`；VERSION/NOTES/定稿。
- **唯一写法（自检，diff 各一份）：**
  - 固件汉字行字节：**仅** `escposenc.HanFirmwareLine(s, FontASize(…))`
  - 备注下划线：**仅** Font A `ESC -`（`escposWriter.underline`）
  - Mesa：`escposWriter.text` → 上者；FT：`emitThermalLine` → 上者
- **不动：** `text_encoding=bitmap`；葡语纯 Font A；ZH B FS!flat 对比行（诊断）。

## 2. 核心业务规则

1. 【已实现】汉字段永不发 `FS !`（含下划线位）。
2. 【已实现】菜名与备注汉字同一 `FS &` + `GS !` + GBK + `FS .`。
3. 【已实现】备注行仍可对拉丁前缀下划线（`ESC -`）。

## 3. 详细测试场景

### 场景：HanFirmwareLine 无 FS !
- **操作：** `TestHanFirmwareLineBytes`。
- **预期：** 混打备注样例字节无 `1C 21 80`；汉段为 `FS &` + `GS ! 0x01` + GBK。
- 结果：【已验证】

### 场景：出品联备注与菜名同 GS!
- **操作：** `TestStationTicketPtNoteWithHanStaysFirmware`。
- **预期：** 有 `ESC -`；有 `FS &`+`GS ! 0x01`；无 `FS ! 0x80`。
- 结果：【已验证】

### 场景：样张备注路径
- **操作：** `TestBuildStationSampleSlipUsesFirmwareForAllLines`。
- **预期：** 含 `GS !` 汉段与 `ESC -`；无 `FS ! 0x80`。
- 结果：【已验证】

### 场景：自检 MIX 1x2 U
- **操作：** `TestSizeProbeRowsPTAndHanB`。
- **预期：** `MIX 1x2 U` = `ESC -` + `HanFirmwareLine(note)`；整单无 `FS ! 0x80`。
- 结果：【已验证】

### 场景：门店实打备注观感
- **操作：** 出品联含中文备注，目视与菜名同高瘦长。
- 结果：【未验证】（待门店装 0.5.38 试打）

## 4. mesa-local-product-test

| 项 | 结果 |
|----|------|
| `mesa-local-product-test`（Mesa 餐厅仓技能） | **N/A · 【未验证】** — 本仓为 `farvoo-fatura` fiscal-agent，无 Mesa Next/Supabase localhost 产品栈 |
| 替代验收 | `go test` + `./scripts/check-fiscal-agent.sh`（见下） |

## 5. 结论

| 检查 | 结果 |
|------|------|
| 唯一写法各一份 | pass |
| 单元 / 包 / check-fiscal-agent | pass |
| 门店实打 | 【未验证】 |
| 可合 main | 是 |
