# 本次改动业务场景测试报告：出品联恢复大类名称行

日期：2026-10-22  
分支：`feat/station-slip-category-header`  
版本：`0.5.50`

## 1. 本次修改

- **内容：** 档口出品联在 0.5.44 起不再打印 `(BEBIDAS)` / `(PRATOS)` 等大类行；门店仍需要同类归组。恢复打印，并受服务端开关控制。
- **改动点：** 恢复 `station_slip_options.show_category_group` + `lines[].category_group_header`；`writeStationMenuLines` 同类只打一次、原样输出；大类行上下不加分隔线；VERSION/NOTES。
- **唯一写法（自检，diff 各一份）：**
  - 是否打印大类行：**仅** `jobPayload.stationSlipShowCategoryGroup()`
  - 大类文案：**仅** `strings.TrimSpace(ln.CategoryGroupHeader)`（在 `writeStationMenuLines` 内）
- **不动：** 菜品排序（仍 `category_group_sort` + `item_index`）；菜与菜虚线分隔；预结单/账单版式。

## 2. 核心业务规则

1. 【已实现】`show_category_group=true` 且 header 非空 → 同类第一道菜前打一行大类名。
2. 【已实现】同类 `category_group_header` 相同只打一次；换类再打。
3. 【已实现】header 原样打印（不加本地括号/翻译）。
4. 【已实现】缺省 / `false` / 无 `station_slip_options` → 不打大类行。
5. 【已实现】大类行上下无实线分隔；菜间虚线照旧。

## 3. 详细测试场景

### 场景：开关开时打印大类且同类一次
- **操作：** `TestStationSlipPrintsCategoryHeaderWhenEnabled`（两类三道菜）。
- **预期：** 出现一次 `(BEBIDAS)`、一次 `(PRATOS)`；顺序为大类 → 菜 → 大类 → 菜；大类旁无连续实线分隔。
- 结果：【已验证】

### 场景：开关关不打印
- **操作：** `TestStationSlipSkipsCategoryHeaderWhenDisabled`。
- **预期：** 有 header 字段也不出现 `Bebidas`。
- 结果：【已验证】

### 场景：无 options 缺省不打印
- **操作：** `TestStationSlipSkipsCategoryHeaderWhenOptionsNil`。
- **预期：** 不出现 `BEBIDAS`。
- 结果：【已验证】

### 场景：回归葡文出品联主路径
- **操作：** `TestBuildStationTicketPortuguesePrintLocale` + `./scripts/check-fiscal-agent.sh`。
- **预期：** 桌号/数量编号行/菜名仍正确；全包测试与 Windows 交叉编译通过。
- 结果：【已验证】

### 场景：门店实打（云端开 show_category_group）
- **操作：** 装 0.5.50 后，点多类菜打档口联，目视大类行与旧票一致、无上下实线。
- 结果：【未验证】（待门店装包试打）

## 4. mesa-local-product-test

| 项 | 结果 |
|----|------|
| `mesa-local-product-test`（Mesa 餐厅仓技能） | **N/A · 【未验证】** — 本仓为 `farvoo-fatura` fiscal-agent，无 Mesa Next/Supabase localhost 产品栈 |
| 替代验收 | `go test` + `./scripts/check-fiscal-agent.sh`（见下） |

## 5. 结论

| 检查 | 结果 |
|------|------|
| 唯一写法各一份 | pass |
| 单元 / check-fiscal-agent | pass |
| mesa-local-product-test | N/A · 【未验证】 |
| 门店实打 | 【未验证】 |
| 可合 main | 是 |
