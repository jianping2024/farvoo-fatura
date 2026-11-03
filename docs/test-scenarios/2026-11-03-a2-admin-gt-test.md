# 本次改动业务场景测试报告：A2 从 Admin 签发 GR / GT（AT test）

日期：2026-11-03  
分支：`fix/gr-gt-movement-start-not-past`  
仓库：`farvoo-fatura`  
本机：`http://127.0.0.1:17881`（`FISCAL_AT_ENV=test`，`fiscal-test.db`，子用户 `519655290/1`）  
方式：Agent 起环境，用户在 Admin 真界面操作，Agent 读报文转储与本地库核对。

## 1. 本次修改

- **内容：** GT「调拨」会把本店 NIF / 名称预填进购方；离开调拨（票种切走 GT，或场景改「外送客户」）时原样保留，导致 GR 购方变成本店自己。现在离开调拨时撤回预填，回到散客（空 NIF、`Consumidor Final`）；用户改过的值不动。
- **文件：** `internal/fiscal/bootstrap/admin/index.html`、本报告。
- **唯一写法：** 预填记在 `transferPrefill`，只由 `clearTransferPrefill` 撤回（`syncGtPurposeUI` 非调拨分支 + `syncGrFieldsVisibility` 非 GT 分支）。

## 2. 核心业务规则

1. 【已实现】先通信 AT，后写库；AT 失败 → 零单据行、不占号。
2. 【已实现】三个地址只有「收货地址」必填：装货地留空 = 门店地址，卸货地留空 = 收货地址（AT 报文三者齐全）。
3. 【已实现】GT 调拨：购方 = 本店，`recipient_is_vat_payer = 1`。
4. 【本次新增】离开调拨即撤回本店购方预填。

## 3. 详细测试场景

### 场景 0：AT test 是否恢复

- **结果：【已验证】** `at-recovery-watch` 16:31 UTC 起返回 2002（13:31 仍 HTTP 500）。

### 场景 1：GT 首签撞号（旧 GT 系列 `GR2026T01`）

- **结果：** 17:08:49 UTC，HTTP 200，`ReturnCode -3`「Já foi inserido um Documento de Transporte com o número fornecido」。`GT GR2026T01/1` 撞上探针在同码 GR 系列发过的 `GR GR2026T01/1`（推断：AT 按「系列码 + 号」判重，不看类型）。失败关闭【已验证】：GT 0 行，`last_number` 0。
- **处理：** 产品无「换系列」入口（每票种仅一个 ACTIVE 系列，GT 卡变灰）；测试库备份为 `fiscal-test.db.bak-20261103-before-gt-retire` 后把该行（0 张单）标 `RETIRED`；用户在 Admin 注册 `GT2026T01` → 验证码 `AAJFJP8X9D`（17:15:31 UTC）。

### 场景 2：Admin 开 GR（重跑 2026-11-02 场景 1）

- **操作：** 发货单 → 只填收货地址 `Rua São José Nº9 Ribafria Alenquer / Lisboa / 2580-557` → 装/卸留空 → 签发。
- **结果：【已验证】pass** 17:17:19 UTC：`ReturnCode 0 OK`，`ATDocCodeID 1108720320`；本地 `GR GR2026T02/1`，`ATCUD AAJFJB8H8B-1`，`N`，已打印；`last_number` 0 → 1，`previous_hash` 空。`AddressFrom` = 门店 `Rua Demo 1`，`AddressTo` = 收货地址。
- **发现：** 购方是本店 `519655290 / Farvoo Demo Lda`（此前切过 GT 调拨留下，用户确认非本意）→ 本次修复。

### 场景 3：Admin 开 GT（调拨）

- **操作：** 运输单 → 调拨（购方自动 = 本店）→ 只填收货地址 → 签发。
- **结果：【已验证】pass** 17:23:56 UTC：`ReturnCode 0 OK`，`ATDocCodeID 1108720347`；本地 `GT GT2026T01/1`，`ATCUD AAJFJP8X9D-1`，`N`，`movement_purpose=transfer`，`recipient_is_vat_payer=1`；`last_number` 0 → 1。开运时间 `2026-11-03T18:18:00`。

### 场景 4：离开调拨撤回本店购方

- **结果：** `go test ./internal/fiscal/bootstrap/...` pass；Admin 手测【未单独记录】（修复后用户直接签了 GT，GR 未再签）。

## 4. 实际测试结果

| 项 | 结果 |
|---|---|
| AT test 系列 / 运输服务恢复 | pass |
| Admin GR → 真 `ATDocCodeID` | **pass**（1108720320） |
| Admin GT → 真 `ATDocCodeID` | **pass**（1108720347） |
| 撞号时失败关闭 | pass |
| 切票种撤回本店购方 | 单测 pass；界面待手测 |

## 5. 风险与待确认

- `ReturnCode -3` 未在 `transportReturnCodes` 映射，界面显示葡语原文；生产出现即说明本地号与 AT 已登记号脱节，宜单独给三语提示（未改）。
- 无停用 / 更换系列入口：系列一旦出问题只能改库。生产前需定产品方案（AT 侧 `finalizarSerie` + 本地新系列）。
- 城市与邮编不校验一致（`Lisboa` + `2580-557` Alenquer，AT 照收）。
- 测试中误签两张 FT（`FT FT2026DEMO01/2`、`/3`，17:21 / 17:22 UTC），票种下拉未切到「运输单」——test 数据，保留。
- 遗留：`fiscal-gt-regression.mjs` / GR mock 回归本次未跑（A2 关闭条件之一）。
