# 本次改动业务场景测试报告：A2 GR / GT 全场景互动测试（AT test）

日期：2026-11-03  
分支：`main`（`fix/gr-gt-movement-start-not-past` 已合入，`a7c13cc`）  
仓库：`farvoo-fatura`  
本机：`http://127.0.0.1:17881`（`FISCAL_AT_ENV=test`，`fiscal-test.db`，子用户 `519655290/1`）  
方式：全场景互动任务式（`.claude/skills/interactive-scenario-test`）——Agent 列矩阵、起环境、发任务卡；用户在 Admin 真界面操作；Agent 读 SOAP 转储 / 本地库 / 日志核对；发现缺陷先分析、用户同意后当场修，再复测。

## 1. 本次修改

| 提交 | 内容 |
|------|------|
| `2eef4b1` | GT 调拨预填本店购方，离开调拨即撤回 |
| `cfe1e01` → `e83f5df` | FT/FS 软推荐：先不再覆盖 FR/PF/GR/GT，最终定为**只升不降**（FS→FT；从不降回 FS） |
| `0a6142a` | 「收货方是 IVA 纳税人」由 NIF 决定（`syncRecipientVatFromNif`；共用 `isIdentifiedNif`） |
| `917c45a` | GR/GT 非纳税人收货方保留所填名称（`applyManualRecipient`）；空 / `Consumidor Final` → `recipient_name_required` |
| `214ce6d` | 购方名称 `Consumidor Final` 改为占位提示，不再预填 |
| `f3ceaff` | GT 两条 NIF 规则独立错误码（`transfer_nif_not_store` / `customer_nif_is_store`）+ 签发前预检 |
| `54fb78e` | AT 拒绝可读：`at_transport_rejected` 只带 AT 原文，Admin 前缀「税务局（AT）拒绝」；`-3` → `at_number_taken` |
| `bc82832` | 用户未改过的开运时间签发时顺延到现在 + 1 分钟 |
| `2e882d4` → `f8aba76` | FT 挂 GR：GR 详情「开发票」+ FT 只挂同一购方 GR（折叠列表）；详情 API `customer` / `item_lines` / `can_issue_sale_from`；顺带修好「从 PF 开真票」（白屏、明细不渲染、税率固定 23%） |
| `9173f17` | 单测：手填装 / 卸货地发到 AT |

唯一写法：收货方 `applyManualRecipient`；名称 / NIF 校验 `validateGRCustomer`；可挂 GR `domain.LinkableDeliveryNoteStatuses` + `ListLinkableDeliveryNotes`；从单据开票 `startOrderFromDocument`；票种程序改值 `setSelectValue`；软推荐 `syncSaleDocTypeSuggestion`。

## 2. 核心业务规则

1. 【已实现】GR/GT 先通信 AT，后写库；失败 → 零单据行、不占号。
2. 【已实现】地址只有收货地址必填；装货地省略 = 门店地址，卸货地省略 = 收货地址（报文三者齐全）。
3. 【本次】GR/GT 收货方须有名称（`Consumidor Final` 不算）；非纳税人 NIF 留空。
4. 【本次】FT 只挂同一购方的 GR；散客 GR 只能经 GR 详情「开发票」挂。
5. 【本次】FT/FS 软推荐只升不降。
6. 【已查法规】FT 不必有 NIF：消费者 NIF 可选、要求时须打印；纳税人购方须名称 / 地址 / NIF（CIVA art. 36.º n.º 5 a)，DL 28/2019；AT 信息 10376）。
7. 【已确认】实时报 AT 的只有 GR/GT；其余七种仅系列注册（ATCUD）。

## 3. 详细测试场景（覆盖表）

### 3.1 自动化（mock）

| ID | 场景 | 结果 |
|----|------|------|
| A1 | GR-07 / GT-07 无系列 `series_missing` | pass |
| A2 | GR-02/03/04/06 FT 挂多张、金额不等、部分开票、无 GR 开 FT | pass |
| A3 | GR 作废 N→A；已挂 FT 禁作废 | pass |
| A4 | GT 调拨填客人 NIF、外送填本店 NIF 拒绝（新错误码） | pass |
| A5 | 纳税人无 NIF 拒绝（GR/GT） | pass |
| A6 | GT 作废与重复作废幂等 | pass |
| A7 | 独立 Tab、不计营收 | pass |
| A8 | SAF-T MovementOfGoods 含 GR/GT | pass |
| A9 | 打印类型与运输块 | pass |
| A10 | 开运时间已过 / AT `-100` 同码 | pass |
| A11 | AT `-3` / `-1` 映射且不占号；手填装卸地到 AT；可挂 GR 按 NIF；原明细 / 收货方快照；`FromDecimal` | pass（单测） |

### 3.2 AT test 真界面（用户操作，Agent 核对）

| 卡 | 场景 | 结果 | 证据 |
|----|------|------|------|
| L1 | AT 不可用失败关闭（11-02） | pass | 见 [`2026-11-02-a2-admin-gr-test.md`](2026-11-02-a2-admin-gr-test.md) |
| L2 | 旧 GT 系列撞号 `-3` 失败关闭 | pass | 17:08:49 UTC |
| L3 | GR 只填收货地址 | pass | `GR GR2026T02/1` → `1108720320` |
| L4 | GT 调拨 | pass | `GT GT2026T01/1` → `1108720347`、`/2` → `1108720452` |
| 1 | 切票种 / 场景购方撤回 + GT 外送纳税人 | pass（首签 `-1` NIF 无效，换真实 NIF） | `GT GT2026T01/3` → `1108720495` |
| 2 | GT 外送非纳税人 | pass（名称缺陷已修） | `GT GT2026T01/4` → `1108720517` |
| 3 | GR 散客 + 名称拦截 | pass | `GR GR2026T02/3` → `1108720797`，名称 `Maria Costa` |
| 4 | GR 手填装 / 卸货地 | pass | `GR GR2026T02/6` → `1108721190`（Loures / Rua Augusta） |
| 5a–e | 本地拦截：调拨他人 NIF、外送本店 NIF、缺收货地址、邮编格式、开运时间已过 | pass（5b 提示缺陷已修） | 期间无报文、不占号 |
| 6 | FT 挂 GR + 作废（GR-10/12、GT 作废） | **待测** | 页面已按新设计重做，待用户真界面执行 |
| — | 列表 Tab / 重打票面抬头 | **未测** | 未发卡 |

## 4. 实际测试结果

| 项 | 结果 |
|----|------|
| Admin GR → 真 `ATDocCodeID` | pass（/1–/7） |
| Admin GT → 真 `ATDocCodeID` | pass（/1–/4） |
| AT 拒绝 / 不可用失败关闭 | pass |
| 本地拦截 5 项 | pass |
| 本轮发现并修复缺陷 | 10 项（§1） |
| `go test ./internal/fiscal/...` | pass |
| 回归 GR / GT / PF / 手工 FT / FT-FS 推荐 / 界面语言 | pass；其余回归脚本未跑 |
| FT 挂 GR 真界面 | 未验证（页面走查以调试方式触发，见 §5） |

## 5. 风险与待确认

**待产品定：**
- 纳税人购方 FT 的名称 / 地址：有 NIF 时未强制（缺名称以 NIF 代填、地址 `Desconhecido`）；系统分不清「公司」与「要求印 NIF 的消费者」，需定是否加「公司客户」标记。FT 表单无地址栏，从 GR 开发票时收货地址不带入。
- 详情页动作门禁 `ready_to_issue` 含云端 `fiscal_profile`：本测试 Agent 未配对云端 → 「开发票」「开真票」「冲销」「借记」「登记收款」在此环境都不显示（签发本身不受影响）。可选：测试库设门店类型，或改按钮门禁。
- GR 收货方用本店 NIF 目前允许（GT 外送已禁）。
- 无停用 / 更换系列入口（本次改库 `RETIRED`，备份 `fiscal-test.db.bak-20261103-before-gt-retire`）。
- 已知 NIF 自动带名称会覆盖已填名称。
- 城市与邮编不校验一致性。
- GR/GT 作废只改本地，不报 AT。

**已知实现缺口：**
- 「只挂同一购方 GR」仅由列表接口与 Admin 保证，`linkDeliveryNotesInTx` 未校验。
- 非纳税人收货方 AT 报文 `CustomerTaxID=999999990`（AT test 接受），待与运输 WS 手册核对。
- A2 关闭：剩其余回归脚本与 §7.2 脱敏 fixtures。

**测试数据：** 误签 FT `FT2026DEMO01/2`、`/3`、`/4`（FT/FS 推荐覆盖 GR/GT 所致，已修）、意外 GR `/7`（本店 NIF，用户有意）；GR `/2`、GT `/4` 收货方名称为 `Consumidor Final`（修复前签出）。均为 test 数据，保留。
