# 本次改动业务场景测试报告：开运时间不能早于现在

日期：2026-11-02  
分支：`fix/gr-gt-movement-start-not-past`  
仓库：`farvoo-fatura`  
本机：`http://127.0.0.1:17881`

## 1. 本次修改

- **内容：** 签发 GR/GT 时，开运时间若早于 Lisbon「现在」，本地直接拦住并中文提示；打开 GR/GT 表单时把时间默认成约一分钟后；若仍打到 AT 的 `-100`，映射成同一错误码，不弹葡语原文。
- **文件：** `at/errors.go`、`service/{delivery,transport,transport_at}.go`、Admin HTML/i18n、测试与本报告。
- **唯一写法：**
  - 规则：`assertMovementStartNotPast`（服务端）+ `movementStartIsPast`（客户端，同一 Lisbon 墙钟比较）
  - 错误码：`movement_start_past`（含 AT `-100` → `mapTransportRegisterError`）
  - 文案：`errors.movement_start_past`
  - 默认刷新：`refreshMovementStartDefaults`（切入 GR/GT / 打开开票页时）

## 2. 核心业务规则

1. 【已实现 / 文档】开运时间不得早于签发时刻（AT）。
2. 【本次新增】比较粒度到 Lisbon「分」；等于当前分钟可通过。
3. 【本次新增】表单默认时间 = 当前时刻 + 1 分钟，减少填单拖久又过期。
4. 【本次新增】AT `-100` 与本地校验同一 `movement_start_past` 提示。

## 3. 详细测试场景

### 场景 1：开运时间已过（对标截图）

- **前置：** Admin 登录；GR；地址完整。
- **操作：** 开运时间设为比现在早几分钟；点签发。
- **预期：** 提示「开运时间不能早于现在…」；focus 时间；不调 AT。
- 结果：单测【已验证】pass；API【已验证】pass（`error=movement_start_past`）；Admin UI【未验证】（请本机硬刷新后手测）

### 场景 2：开运时间等于或晚于现在

- **操作：** 默认（约 +1 分钟）或手动选更晚时间后签发。
- **预期：** 不因「过期」被拦（其他 AT/系列条件另论）。
- 结果：【未验证】手测

### 场景 3：切入 GR/GT 刷新默认时间

- **操作：** 切到发货单/运输单或进入开票页。
- **预期：** 日期为今天，时间为约一分钟后。
- 结果：【未验证】手测

### 场景 4：AT `-100` 映射

- **操作：** SOAP 单测 ReturnCode -100。
- **预期：** Hint `movement_start_past`；服务映射码 `movement_start_past`。
- 结果：【已验证】pass

## 4. 实际测试结果

| 项 | 结果 |
|---|---|
| Go：PastMovement / TimePicker / -100 hint | pass |
| API 过去开运时间 → `movement_start_past` | pass |
| Admin 中文提示 + focus | 未跑（请硬刷新 http://127.0.0.1:17881 手测） |

## 5. 风险与待确认

- 跨日凌晨若长时间不关页，日期 min 仍是进程启动时的「今天」——既有 DatePicker 限制，本刀未扩 `setMin`。
