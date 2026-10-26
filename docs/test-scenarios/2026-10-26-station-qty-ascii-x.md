# 本次改动业务场景测试报告：出品联数量乘号改为 ASCII x

日期：2026-10-26  
分支：`fix/station-qty-ascii-x`  
版本：`0.5.55`

## 1. 本次修改

- **内容：** 门店 Sashimi 档口出品联上 `1× 01` 的乘号空白（Font A 对 Unicode `×` / WPC1252 `0xD7` 缺字形）；Frita 机能显示。改为 ASCII `x`，各机一致为 `1x 01`。
- **改动点：** `stationSlipQtyCodeLead`；单测期望；VERSION/NOTES。
- **唯一写法（自检，diff 各一份）：**
  - 出品联菜品行首「数量×编号」字符串：**仅** `stationSlipQtyCodeLead`（`%dx` / `%dx %s`）
- **不动：** 字号（lead 仍 2×2）；桌号/时间；菜名 1×2；入队 payload。

## 2. 核心业务规则

1. 【本次新增】纸面数量乘号必须为 ASCII `x`，不得用 Unicode `×`。
2. 【已实现】有编号：`{qty}x {code}`；无编号：`{qty}x`。
3. 【已实现】数量为 1 时仍打印 `1x`（不省略乘号）。
4. 【已实现】菜单体调用唯一经 `stationSlipQtyCodeLead`，禁止第二套格式化。

## 3. 详细测试场景

### 场景：有编号的 lead 为 ASCII x
- **前置：** 单元测试环境。
- **操作：** `TestStationSlipQtyCodeLead` — `stationSlipQtyCodeLead(1, "001")`。
- **预期：** 得到 `1x 001`（字节为 ASCII `x`，非 `0xD7`）。
- **数据验证：** 同测校验无码 `12x`；缩进列宽与改前一致（`x` 与 `×` 同为 1 显示列）。
- 结果：【已验证】pass

### 场景：葡语出品联主路径仍含 lead+菜名
- **前置：** 同批 pt locale fixture。
- **操作：** `TestBuildStationTicketPortuguesePrintLocale`（经 `leadRow` → `stationSlipQtyCodeLead`）。
- **预期：** 2×2 lead 后接 1×2 菜名；桌号/份数栏正常。
- 结果：【已验证】pass

### 场景：位图路径用 ASCII lead 测间距
- **操作：** `TestMergeBitmapRowsBottomAligns`、`TestHanStationNameXClearsMeasuredLead`（fixture `1x` / `2x 012`）。
- **预期：** 名称列仍在 lead 右侧空隙后。
- 结果：【已验证】pass

### 场景：全包门禁（含 Windows 交叉编译）
- **操作：** `./scripts/check-fiscal-agent.sh`
- **预期：** `go test ./...` + Windows build 通过。
- 结果：【已验证】pass

### 场景：门店双档口实打（Frita + 曾空白的 Sashimi）
- **操作：** 装 0.5.55 后同桌下单两档口，目视 `2x 43` / `1x 01` 乘号均可见。
- 结果：【未验证】（待门店装包）

## 4. mesa-local-product-test

| 项 | 结果 |
|----|------|
| `mesa-local-product-test`（Mesa 餐厅仓技能） | **N/A · 【未验证】** — 本改动在 fiscal-agent ESC/POS；不经 Mesa `:3000` UI。已跑 `mesa-local-uat stack-health`：web/supabase 可达，ops `:3001` 未起（与本改无关） |
| 替代验收 | `go test` + `./scripts/check-fiscal-agent.sh`（见上，【已验证】） |

## 5. 结论

| 检查 | 结果 |
|------|------|
| 唯一写法各一份 | pass |
| 单元 / check-fiscal-agent | pass |
| mesa-local-product-test | N/A · 【未验证】 |
| 门店实打 | 【未验证】 |
| 可合 main | 是 |
