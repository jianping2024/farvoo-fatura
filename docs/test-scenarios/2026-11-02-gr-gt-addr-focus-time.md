# 本次改动业务场景测试报告：GR/GT 地址精确提示 + focus + 开运时间

日期：2026-11-02  
分支：`feat/at-a0-shared-base`  
仓库：`farvoo-fatura`  
本机：`http://127.0.0.1:17881`（`FISCAL_AT_ENV=test`）

## 1. 本次修改

- **内容：** 签发 GR/GT 时，地址缺哪一项就说哪一项，并把光标放到那个输入框；开运时间改为与日期同一风格的 24 小时选择（不再用浏览器自带 `type=time`）。
- **文件：** `domain/address.go`、`service/{service,delivery,transport,transport_at}.go`、`api/routes.go`、`bootstrap/ui/{time-picker.js,time-picker.css,toast.js,admin-i18n.js}`、`bootstrap/ui_assets.go`、`bootstrap/admin/index.html`、相关测试。
- **角色 / 页面：** Admin 手工开票（发货单 GR / 运输单 GT）。
- **唯一写法（自检）：**
  - 地址缺项：`domain.Address.ValidatePT` → `IncompleteAddressError{Field,Part}`（服务端）；Admin `ptAddressDefect` + `reportTransportAddressDefect`（客户端预检，与 part 顺序一致）
  - API 错误：`transport_address_incomplete` + `field` + `part`
  - 用户可见句：`gr.addr.incomplete` × `gr.addr.role.*` × `gr.addr.part.*`（toast / formatError）
  - 开运时间：sole `FiscalUI.createTimePicker` + `readMovementStart`

## 2. 核心业务规则

1. 【已实现】装货地三项都空 → 用本店地址；卸货地三项都空 → 用收货地址。
2. 【本次新增】半填的装/卸货地必须补全；提示点名「哪一组 + 缺街道/城市/邮编」。
3. 【本次新增】收货地址缺项也走同一套 `field=customer` + `part`，不再用笼统「须填地址/城市/邮编」。
4. 【本次新增】开运时间为 Lisbon 墙钟日期 + 24h `HH:MM`；缺日期 / 缺时间分开提示并尽量 focus。
5. 【已实现】已签发成功的单据不因本校验改动而回写。

## 3. 详细测试场景

### 场景 1：卸货地有街道和邮编、城市空（对标截图）

- **前置：** Admin 已登录；文档类型 GR 或 GT；收货地址完整。
- **操作：** 卸货地填街道与 `1000-001`，城市留空；点签发。
- **预期：** 提示「卸货地须填写城市」（或等价 i18n）；光标在卸货城市框；不签发。
- **数据验证：** 无新发票行。
- **异常恢复：** 填城市后再签可继续。
- 结果：API【已验证】pass（`field=ship_to` `part=city`）；Admin UI focus【未验证】（主仓旁路 dirty 挡住浏览器 MCP）

### 场景 2：装货地邮编格式错误

- **前置：** 收货地址完整。
- **操作：** 装货地填街道、城市、邮编 `10001`（非 NNNN-NNN）；签发。
- **预期：** `field=ship_from` `part=postal_code`；提示须葡萄牙邮编。
- 结果：API【已验证】pass

### 场景 3：收货地址缺城市

- **操作：** 收货街道+邮编有值，城市空；签发。
- **预期：** `transport_address_incomplete` + `field=customer` + `part=city`（不再是笼统 `validation_failed` 地址句）。
- 结果：API【已验证】pass（`field=customer` `part=city`）

### 场景 4：装/卸三项都空

- **操作：** 不填装货地、卸货地；收货完整；有开运时间；签发。
- **预期：** 客户端不因装/卸报错；服务端用店址/收货地址（系列/AT 等其他条件另论）。
- 结果：【未验证】（本刀未强制打通完整 AT 签发；规则未改）

### 场景 5：缺开运日期或时间

- **操作：** 清空日期或时间后签发。
- **预期：** 分别提示「请选择开运日期」/「请填写开运时间」；时间用 TimePicker（无 `type=time`）。
- 结果：HTML/资源【已验证】pass（sole TimePicker、无 native time）；交互 focus【未验证】

### 场景 6：服务端结构化错误可被 toast 映射

- **操作：** 绕过客户端预检（直接 API）返回 `field`+`part`。
- **预期：** `formatError` 用 `gr.addr.*` 拼精确中文，不回「三处地址」总句。
- 结果：代码路径【已验证】；浏览器 toast 文案【未验证】

### 场景 7：回归 — 旧笼统文案已删除

- **操作：** 检查 embed 的 `admin-i18n.js` / Admin HTML。
- **预期：** 无 `gr.need_postal_pt` / `gr.need_address` / `gt.need_address`；无 `type="time"`。
- 结果：【已验证】pass

### 场景 8：单测

- **操作：** `domain` Address / `bootstrap` TimePicker+DateRange / `service` IncompleteAddress。
- 结果：【已验证】pass

## 4. 实际测试结果

| 项 | 结果 |
|---|---|
| Go 单测（Address / TimePicker / IncompleteAddress） | pass |
| `GET /` 无 `type=time`；sole `createTimePicker` | pass |
| i18n 无旧笼统键；有 `gr.addr.part.city` | pass |
| API 卸货缺城市 → `ship_to`/`city` | pass |
| API 装货坏邮编 → `ship_from`/`postal_code` | pass |
| API 收货缺城市 → `customer`/`city` | pass |
| Admin UI 精确 toast + focus | **未跑** — 浏览器 MCP 被 `restaurant-ordering` 旁路分支 `fix/waiter-board-dish-qty-title-gap` 未提交改动门禁挡住；本任务未 stash/restore 该 WIP |
| 完整 GR live 签发 | 未跑（本刀范围=校验与时间 UX） |

## 5. 风险与待确认

1. Admin 页面上的 focus/toast 需在浏览器门禁可测时补一眼（同一操作：卸货城市空 → 光标进城市框）。
2. Store 层仍有「customer address (detail/city/postal) required」底线文案；正常签发路径应在 service 被结构化错误拦住，进不到 store。
3. 主仓旁路 dirty 不属本任务；合进 `farvoo-fatura` main 不依赖该 WIP。
