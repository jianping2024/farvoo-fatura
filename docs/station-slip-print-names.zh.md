# 出品联多语菜名（Agent 纸面契约）

> **状态：定稿**  
> **权威：是**（fiscal-agent 出品联出纸；产品与入队以 Mesa 为准）  
> **产品定稿：** restaurant-ordering `docs/product/station-slip-multilang-names.zh.md`  
> **对应实现：** `jobLine.PrintNames` + `stationSlipLabels` → `writeStationMenuItem` / `writeStationMenuLabel`  
> **写作规范：** [`design-doc-standards.zh.md`](design-doc-standards.zh.md)

---

## 0. 背景

Mesa 入队可在同一道菜上带多个语种菜名。Agent 须按序 1×2 打印；Qty / 编号只挂第一语言行；备注仍在全部菜名之后。预结/结账 **不**使用本契约。

---

## 1. P0 定法

| # | 定法 |
|---|------|
| 1 | **范围**仅 `station_ticket`（及同版式开台单若复用 `writeStationMenuLines` 且 payload 带 `print_names` 时同样遵守） |
| 2 | **payload 行字段** `print_names`：`string[]`，有序纯菜名（已 trim），**不含** `item_code` |
| 3 | **兼容**：`print_names` 缺省、`null` 或空 → 行为与现网相同：`stationSlipItemLabel`（`item_code` + `item_name`/`display_name`）单行 + Qty |
| 4 | **多行时**：第 1 个名：`format` = 有 `item_code` 则 `{code}-{name}` 否则 `name`；与 Qty 同列模型（或 Han 画布）打出。第 2…n 个名：仅菜名，同左边距，**不**打 Qty、**不**加编号 |
| 5 | **备注**：全部 `print_names` 行打完后，再 `writeStationItemNoteLine`（空 note 跳过） |
| 6 | **字号**：`print_names[0]`（通常葡语）由 agent 本机 `station_pt_name_size` 控制：**`1x2`（默认）** 或 **`2x2`**；第 2…n 名与备注固定 **1×2**；数量×编号 lead 固定 **2×2**。葡语 Font A `GS !`，汉语固件 `FS &` + `GS !` + GBK，见 [`fiscal-thermal-text-encoding.zh.md`](fiscal-thermal-text-encoding.zh.md) |
| 8 | **换行（默认）**：每个名字各自在菜名列内换行（汉字按 2 列）；续行同左边距、不打 Qty、不加编号 |
| 9 | **强制一行（本机）**：`station_name_one_line` / `station_note_one_line`（默认关）。开启后各 `print_names` / 备注只保留 wrap 首段，超出**硬丢弃**（不加 `…`）。闸门仅 `stationSlipKeepLines` |
| 7 | **结账 / 预结**：不读 `print_names`；保持现单语行逻辑 |

---

## 2. `jobLine` 相关列（JSON）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| print_names | string[] | 否 | 有序出品联菜名；空/缺省 → 兼容单行 |
| item_code | string | 否 | 仅缀于第一语言行 |
| item_name | string | 否 | 兼容/调试；多语时宜为 `print_names[0]` |
| display_name | string | 否 | 兼容；多语时宜为 `{code}-{print_names[0]}` |
| qty | number | 是 | 只印在第一语言行 |
| note | string | 否 | 全部菜名之后；空不印 |

---

## 3. 唯一写法

| 职责 | 入口 |
|------|------|
| 菜名标签列表 | **仅** `stationSlipLabels`（trim 后非空的 `print_names`，首名缀 `item_code`；为空 → `[stationSlipItemLabel]`） |
| 出品联菜单体 | **仅** `writeStationMenuLines` → `writeStationMenuItem` |
| 备注 | **仅** `writeStationItemNoteLine`（调用顺序：菜名循环之后） |

伪代码：

```text
labels = stationSlipLabels(line)   // 无 print_names → [stationSlipItemLabel]，与现网同一分支、同字节
for i, label in labels:
  writeStationMenuLabel(label, qty if i==0 else "")   // 1×2，按列宽换行
writeStationItemNoteLine(note)
```

---

## 4. 验收（Agent）

- 无 `print_names`：与现网字节路径一致（单测回归）  
- `["Água 500ml","中水"]` + code `001` + qty 1：第一行含 `001-` 与 Qty；第二行仅 `中水`  
- note 非空：出现在菜名之后且 1×2  
- receipt / pre_bill builder **不**消费 `print_names`  
