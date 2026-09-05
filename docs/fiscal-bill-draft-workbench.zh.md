# 收银账单 → 分单开票（整桌 / 按人）

> **状态：定稿**（实现已落地，见 Agent `IssueFromBillDraft` / 正式 Admin）  
> **权威：是**（本仓「从同步账单开票 / 分单补票」以本文为准；**库表/API 内部名不变**）  
> **对应实现：** 已落地（`mode`/`scope_id`/NIF/`discard`/详情已开标记）；开票员 PIN 见 **M3.2** [`fiscal-m3-2-operators.zh.md`](fiscal-m3-2-operators.zh.md)  
> **写作规范：** [`design-doc-standards.zh.md`](design-doc-standards.zh.md)  
> **库表权威：** [`fiscal-sqlite-schema.zh.md`](fiscal-sqlite-schema.zh.md) §6.21  
> **只读依据（不改对方仓文档）：** restaurant-ordering `farvoo-fiscal-agent-integration.zh.md` §3.1（整桌/按人互斥、`scope_id` 稳定 UUID）；挂单载荷见同仓 `farvoo-fiscal-bill-sync-api.zh.md` §5  
> **工程进度：** [`fiscal-dev-plan.zh.md`](fiscal-dev-plan.zh.md) — **M2.5 已完成**；店员 UI 见 **M2.6** + [`fiscal-admin-ui-prototype/README.md`](fiscal-admin-ui-prototype/README.md)  
> **UI 用语权威：** 方案 A 以原型 README「业务用语」为准（收银账单 / 手工开票）  
> **按菜分单 UX 权威：** [`fiscal-bill-split-workbench-ux.zh.md`](fiscal-bill-split-workbench-ux.zh.md)（main view + 同页开票条；本机分配；开票消账）

---

## 0. 业务用语 vs 内部名（P0）

**产品界面、培训、对外说明**只用左列；右列仅出现在本仓设计文、代码、日志。  
侧栏主名 **禁止**再用「订单」「待开票账单」（与「待开票」语义撞车）；旧称仅作别名。

| 界面 / 业务用语 | 内部（代码 / 表 / API） | 旧称（别名） |
|-----------------|-------------------------|--------------|
| 收银账单 | `bill_sync_drafts`；路由前缀 `/local/v1/bill-drafts` | 待开票账单 |
| 进入开票 | 进入签发流（或 M2.6 四步）；载荷仍来自同步或映射 | 转订单 |
| 手工开票 | 开票前销售聚合（手动为 snapshot）；页标题 | 订单；新建开票 |
| ＋ 手工开票 / 收银账单 | 发票 hub 入口 CTA（收银与侧栏同名） | 新建开票 / 处理收银账单 |
| 签发发票 | `IssueDocument` → FT | — |
| 发票 | `invoices` + 打印作业 | — |
| 重打 | `print_purpose=REPRINT` 新打印作业（**M2.6b**） | — |
| 作废账单 | `POST .../discard` → `DeleteBillDraftsBySale`（**不删**已开发票） | — |
| 商品 | `fiscal_products`（LOCAL 维护见 M2.6a） | — |
| 客户 | `customers` / 开票时 NIF 覆盖 | — |

**禁止**在店员可见 UI 出现：草稿、LOCAL、bill-draft、M3、API、scope_id（可展示桌号/人名）。

---

## 1. 一句话

Farvoo 结账只负责「同步账单」进本机 `bill_sync_drafts`；**整桌或按人开 FT、补票、作废收银账单**一律在本机 Fiscal Agent 完成（餐馆：**收银账单** → 进入开票 → 签发）。税务签发仍唯一走 `IssueDocument` → `IssueFT`。

---

## 2. 与现 MVP 的关系

| 能力 | 现状（已落地） | 本文工作台 |
|------|----------------|------------|
| 同步入草稿 | `PullAndIngest` → `UpsertBillDraftOpen` | 不变 |
| 整桌开 FT | Admin §7 一键；`mode` 隐式 whole_table；成功立刻硬删草稿 | 保留；API 显式 `mode=whole_table` |
| 按人 / `split` 草稿 | `DraftToSaleSnapshot` **拒绝** | **开放**：`mode=person` + `scope_id` |
| NIF / 客户名 | MVP 固定散客 | **P0**：开票前可编辑；默认可散客；按人**各自**指定 |
| 再同步挡重 | `HasSignedFTForSale`（按 `source_sale_id` 有任一张 FT 即挡） | 实现刀须收紧：见 §5.2（按业务键区分整桌/按人） |
| UI | Admin 调试页 §7（工程） | **M2.6** 正式 Admin「收银账单」→ 进入开票 → 签发 |
| §13 鉴权 | Admin `/issue` **无**真 PIN（本机信任） | **M3.2** 落地 Agent 本地操作员 + PIN（[`fiscal-m3-2-operators.zh.md`](fiscal-m3-2-operators.zh.md)） |

---

## 3. 职责拆分（P0）

| 侧 | 管 | 不管 |
|----|----|------|
| Farvoo 结账 | 功能开关；点「同步账单」；挂 `bill_sync_jobs`；示「同步完成」 | 分单开票 UI、税票、ATCUD、改草稿、填 NIF |
| Agent | 列 open 收银账单；选整桌或按人 scope；**填 NIF（可选）**；签发发票；进度展示；作废账单 | 改云端菜单、关台、替 Restaurant 结账 |

发票开票人 = 打票本机 **当前登录操作员**（M3.2 前可暂用 `op-demo-cashier`）。  
**开票员 PIN / 本地名册：** **M3.2** [`fiscal-m3-2-operators.zh.md`](fiscal-m3-2-operators.zh.md)（Agent 创建，**不同步** Farvoo）。本里程碑继续本机 Admin 信任模型直至 M3.2 落地。

---

## 4. 用户流程

### 4.1 整桌（`payload.scope_type = whole_table`）

**两条店员路径（P0）：**

| 路径 | 何时 | 开票 |
|------|------|------|
| 整桌一张 | 客人不要分、店员也不代分 | `mode=whole_table`（既有） |
| **本机代分后按人** | 云端整桌进来，店员在 Agent 按菜分单（客人懒得在收银侧操作） | 写 `allocation` 后 `mode=person`（见分单 UX） |

```text
收银账单 → 进入分单工作台（main view）
  → 可选：不分配，直接整桌签发（mode=whole_table）
  → 或：添加用餐人、按菜分配（含几分之几）→ 按人签发（mode=person）
  → 互斥与清草稿见 §5 / §6；壳层见 fiscal-bill-split-workbench-ux
```

整桌签发（不分配）时：

```text
（可选）填本票 NIF / 客户名；空则散客
  → DraftToSaleSnapshot（whole_table）+ 本次客户覆盖
  → IssueFromBillDraft(mode=whole_table)
  → IssueDocument / IssueFT
  → DeleteBillDraftsBySale（尽力；见 §6）
```

### 4.2 按人（同步已是 `scope_type = split`，或本机 allocation 已有人）

```text
打开收银账单 → 分单工作台（与 whole 同一 UI；初值来自云端 splits 或本机 allocation）
  → 未开：可改分配后签发；已开：只读票号
  → 选一人 →（可选）为该人填 NIF / 客户名；空则该人散客；不沿用其他人的 NIF
  → mode=person + scope_id=allocation 中该人 UUID
  → 由 source_lines + allocation 派生 SaleSnapshot（唯一映射出口，见 §7）+ 本次客户覆盖
  → IssueDocument / IssueFT
  → 清草稿条件见 §6（人开完且池空）
```

**禁止：** 对已存在任一张 **按人** FT 的 sale 再发 `mode=whole_table`（`scope_mutex`）。  
**禁止：** 对同步即为 `split` 且从未意图整桌的草稿发 `mode=whole_table`（API 拒绝）。  
**允许（定稿修订）：** 同步为 `whole_table` 的草稿，在本机完成按菜分配后发 `mode=person`（店员代分主路径）。**旧禁止「whole 草稿不得 person」作废。**

同步载荷默认**不带**每人 NIF；工作台必须允许开票前逐人指定。

---

## 5. 开票范围与业务键（P0）

### 5.1 术语

| 载荷 / 开票 | 含义 |
|-------------|------|
| 同步 `scope_type=whole_table` | 草稿整桌一行清单 |
| 同步 `scope_type=split` | 草稿含多人初稿；**不是**开票用的 `scope_type` |
| 开票 `scope_type=whole_table` | 一张整桌 FT；`scope_id = source_sale_id`（与现 MVP 一致） |
| 开票 `scope_type=person` | 一张按人 FT；`scope_id = splits[].scope_id`（须非空 UUID） |

`fiscal_purpose` 固定 `"sale"`。`source_system` 固定 `"farvoo"`。

业务幂等键（与对接文一致）：

```text
store_id | source_system | source_sale_id | scope_type | scope_id | fiscal_purpose
```

### 5.2 互斥与补票（P0）

| 规则 | 定法 |
|------|------|
| 整桌 vs 按人 | 同 `source_sale_id`：已有 **整桌** FT → 禁任何按人；已有 **任意一张按人** FT → 禁整桌 |
| 按人补票 | 允许：未开的 `scope_id` 可继续开；已开 scope 幂等返回原票，不得重签 |
| 改范围 | 已走按人不能改回整桌；已走整桌不能改按人；纠错靠 NC（后置） |
| 关台 | 不影响本机对已落草稿/已有快照的开票 |

### 5.3 再同步 `already_invoiced`（相对现实现须收紧）

**现实现：** `HasSignedFTForSale` 按 `source_sale_id` 有任一张 FT 即整单失败。  

**工作台落地后 P0：**

| 情况 | 再同步 |
|------|--------|
| 已开 **整桌** FT | ack `already_invoiced` |
| 已开 **至少一张按人** FT | ack `already_invoiced`（禁止用同步覆盖未开完的分单草稿意图；未开人在工作台补票） |
| 无任何 FT | 允许覆盖 `open` 草稿 |

判定唯一入口扩展为（实现刀命名可保留 `HasSignedFTForSale`）：查 `invoices` 是否存在该 `source_system`+`source_sale_id` 的已签行。与「有票则禁再同步」一致；进度细节只在工作台用业务键查询。

---

## 6. 开票成功清草稿（P0）

| 情况 | 定法 |
|------|------|
| `mode=whole_table` 成功 | **立刻** `DeleteBillDraftsBySale(source_sale_id)` |
| `mode=person` 成功且仍有未开 person scope | **不删**草稿；UI 标已开 `scope_id` |
| `mode=person` 成功且所有 person scope 均已有 FT，**但**相对冻结源行（`source_lines`）剩余池非空 | **不删**草稿；须继续分配剩余或用户丢弃 |
| `mode=person` 成功且所有 person scope 均已有 FT，**且**剩余池为空 | **自动** `DeleteBillDraftsBySale` |
| 用户丢弃 | `POST .../discard` → 硬删该 sale 全部草稿；**不删**已开发票 |

> **相对旧定法：** 不再仅凭「全部 `splits[].scope_id` 已有 FT」清草稿。池空判定与分配模型见 [`fiscal-bill-split-workbench-ux.zh.md`](fiscal-bill-split-workbench-ux.zh.md) §0/§5/§6。

`fiscal_products` **永不**因开票/丢弃而删。

**不采用：** 每开一张按人票就删草稿（会丢失未开人的初稿）；**不采用：** 人开完但池非空时静默删草稿。

**签发 vs 清草稿（P0 定法）：** 已签 FT **不得**因删草稿失败而回滚。`IssueDocument` 成功后删草稿失败 → HTTP 仍按开票成功返回（含票号），并带 `cleanup_pending=true`（或等价字段）；记日志；允许补偿重试删。禁止「票已开却让客户端以为失败」导致盲目连点（幂等可挡双开，体验仍差）。

---

## 7. 唯一写路径（P0）

| 动作 | 唯一入口 |
|------|----------|
| 同步写入/覆盖草稿 | `store.UpsertBillDraftOpen` |
| 载荷 → `SaleSnapshot` | **唯一** `billsync.DraftToSaleSnapshot` 族：整桌走现函数；按人走同包 **`DraftPartToSaleSnapshot(snap, scopeID)`**（或等价单出口，禁止第二套拼装） |
| 客户覆盖 | 映射后再写本次 `customer`（来自 issue body；空则散客 `999999990`） |
| 从草稿开票 | `service.IssueFromBillDraft`（扩展 mode/scope_id/客户）→ `IssueDocument` → `IssueFT` |
| 清草稿 | 仅 `store.DeleteBillDraftsBySale` |
| 丢弃草稿 | `service.DiscardBillDrafts`（或同名）→ `DeleteBillDraftsBySale` |
| 再同步挡重 | 仅税务库查询（见 §5.3） |

签发禁止第二套插票。Cloud→Agent 的 Realtime/Polling 仍只门铃同步入库；**Agent→Admin 工作台**见下节 SSE（不是浏览器空转轮询）。

按人幂等 `request_id` **必须**含 `scope_id`（禁止仅用 `draft-issue:{draft.id}`，否则多人互撞）。

---

## 7.1 Admin 实时提示（P0 定法）

**体感：** 收银账单入库后约 1 秒内，餐馆侧栏角标更新；在工作台/收银账单列表时列表自刷新；open 数增加时轻 toast（含桌号若有）。不打断当前填表。商超无此菜单、无角标。

| 项 | 定法 |
|----|------|
| 推送通道 | **唯一** `GET /local/v1/events`（SSE）；事件名 `bill_drafts_changed` |
| 何时推 | `UpsertBillDraftOpen` / `DeleteBillDraftsBySale` **成功后** 各调一次 fan-out（幂等命中已有 `request_id` **不**推） |
| Hub | **唯一** `uievents.Hub`：`NotifyBillDraftsChanged` → 所有 SSE 客户端 |
| 浏览器 | **唯一** `EventSource('/local/v1/events')`；收到后只调现有 `refreshBills()`；角标 **唯一** `updateBillsNavBadge` |
| Toast | 仅 `open_count` **增加**时；文案业务用语（桌号 / 「有新的收银账单」） |
| 轮询 | **禁止**作为主路径；SSE 断线靠浏览器自动重连 |
| UAT 门铃 | **唯一** `POST /local/v1/dev/bill-sync/pull`（`FISCAL_ALLOW_DEV_KEY=1`）→ 同进程 `PullAndIngest`；禁止另起进程写 SQLite 冒充推送 |
| 非目标 | 第二套 WS、Cloud Realtime 直连网页、强制跳转待开票页、强模态 |

**依据：** 店员不刷新也应知道有活；门铃由 Agent 按，网页不空转敲门。

## 8. API（P0）

```text
GET  /local/v1/bill-drafts
GET  /local/v1/bill-drafts/{id}          # 含完整 payload（行 / splits）与已开标记
POST /local/v1/bill-drafts/{id}/issue
POST /local/v1/bill-drafts/{id}/discard  # 硬删该 sale 全部草稿
```

本里程碑上述 Local 路由：**不**强制真 PIN；与现 Admin 本机信任一致。**M3.2** 落地后 `operator_id` 须为登录操作员。

### `POST .../issue` body

| 字段 | 必填 | 说明 |
|------|------|------|
| operator_id | 是 | 第一实现刀可默认 `op-demo-cashier` |
| mode | 是 | `"whole_table"` \| `"person"` |
| scope_id | person 时是 | 对应 `splits[].scope_id`（UUID） |
| customer_nif | 否 | 空或省略 → 散客 `999999990`；有则写入本票客户快照 |
| customer_name | 否 | 有 NIF 时可填抬头；散客可用固定 Consumidor Final |

错误码（稳定字符串，实现刀落代码）：

| 码 | 何时 |
|----|------|
| `draft_not_found` / `draft_not_open` | 无草稿或非 open |
| `validation_failed` | mode/载荷不匹配、缺 scope_id、split 缺 UUID、NIF 格式非法 |
| `scope_mutex` | 整桌/按人互斥冲突 |
| `already_invoiced` | 该 scope 业务键已有票（幂等命中除外由 Issue 返回原票） |

列表/详情只读模型须能标：**每个 `scope_id` 是否已有 FT**（查税务库业务键，不在草稿行存 `invoiced` 状态）。

---

## 9. UI（P0）

- **店员路径（M2.6）：** 正式 Admin，流程见 [`fiscal-admin-ui-prototype/README.md`](fiscal-admin-ui-prototype/README.md)。餐馆侧栏 **收银账单** → **进入开票** → **签发发票**。  
- **工程调试：** 本机 `http://127.0.0.1:17880` Admin §7 保留至 M2.6 正式页替代；**不对店员暴露** § 编号与「草稿」字样。  
- **不做** Farvoo 云端开票页。  
- **工作台双 CTA（P0）：** 与 [`fiscal-admin-ui-prototype/README.md`](fiscal-admin-ui-prototype/README.md)「发票 hub 双 CTA」一致：`收银账单` 与 `＋ 手工开票` **同级视觉**；有 open 收银账单时前者 **优先焦点**。禁止灰边弱化收银账单入口。  
- 列表：桌号、金额、同步时间、整桌/分单、未开/已开人数。  
- **进入开票 / 分单：** 列表点入 → **专用 main view**（非列表下钻小块、非小弹窗分配器）；交互与壳层以 [`fiscal-bill-split-workbench-ux.zh.md`](fiscal-bill-split-workbench-ux.zh.md) 为准（左剩余池 / 右当前人份额 / 同页开票条；`whole_table` 与已 `split` 同一界面可实时改；本机持久化、不回写云）。  
- **签发：** 按人在开票条签发；整桌仅在尚未走按人开票路径时保留「整桌签发」（互斥见 §5）。  
- **NIF：** 整桌开票前一个输入框；按人则**每个人一块独立输入**（默认空=散客）；不得把 A 的 NIF 默认填给 B。  
- 成功：展示 `InvoiceNo`、ATCUD；已开份额从总池消去；按人刷新已开标记；整桌或全员开完且池空后收银账单从列表消失（若 `cleanup_pending` 则提示稍后清理，票号仍展示）。

### 第一实现刀默认（文档锁定）

| 项 | 定法 |
|----|------|
| 客户 | 未填 NIF → `999999990` / Consumidor Final；填了则用本次输入 |
| 付款 | 该 scope 全额 `CASH` |
| 操作员 | M3.2 前可暂 `op-demo-cashier`；M3.2 后为登录 `operators.id` |

### P1（不挡本里程碑关闭）

| 项 | 说明 |
|----|------|
| 混合付款 | 多 `payments[]` |
| 开票员 PIN / 本地名册 | **M3.2** [`fiscal-m3-2-operators.zh.md`](fiscal-m3-2-operators.zh.md)（Agent 创建，不同步 Farvoo） |
| 重打 / NC 入口 | 发票详情：**重打 M2.6b**；**NC M3** |
| 新 NIF 写入本地客户主档 | 对接文 A7；本刀至少写入本票快照即可 |

---

## 10. 数据

物理列以 [`fiscal-sqlite-schema.zh.md`](fiscal-sqlite-schema.zh.md) §6.21 为准。运行时草稿 `status` 仅 `open` / `discarded`；**不**用草稿行表示「已开票」。已开进度 = `invoices` / `idempotency_keys` 业务键查询。

同步载荷中 `splits[]`（与 `farvoo-fiscal-bill-sync-api` §5.3 一致；Restaurant `by_item` 入队可能尚未交付，Agent 按此形状消费）：

| 字段 | 要求 |
|------|------|
| scope_id | 必填非空 UUID（开票 `person` 的 scope_id） |
| name | 展示用（如人名/座位） |
| lines | 同整桌行字段；`vat_rate` 百分数串 |
| gross_total | 该人合计；缺则由行汇总 |

载荷**不要求**带 NIF；NIF 仅工作台开票时提供。

---

## 11. 验收场景（实现刀用）

1. `whole_table` 草稿 → 开整桌 → 有票号 → 草稿硬删 → 再同步 `already_invoiced`。  
2. `split` 两人草稿 → 开 A → 草稿仍在且 A 已开、B 可开 → 开 B → 草稿自动硬删。  
3. 已开 A 后再开 A → 幂等原票，不重签。  
4. 已开任一人后点整桌 → `scope_mutex`。  
5. 同步 `split` 草稿 `mode=whole_table` → 拒绝。同步 `whole_table` 本机分配后 `mode=person` → **允许**（代分主路径）。  
6. 丢弃草稿 → 行消失；已开发票仍在。  
7. 按人 A 填 NIF、B 不填 → A 票面非散客、B 为散客；两人 NIF 互不串。  
8. 整桌不填 NIF → 散客；整桌填 NIF → 票面用该 NIF。  
9. 签票成功但删草稿失败 → 仍返回成功 + `cleanup_pending`；库中有票。  
10. `rg`：映射 / `IssueFromBillDraft` / `DeleteBillDraftsBySale` 仍各唯一入口。  
11. `whole_table` → 本机分两人 → 开 A → 池与草稿仍在 → 开 B 且池空 → 草稿删。

---

## 12. 非目标

- NC / 重打完整工作台、SAF-T、FS/FR/ND  
- 开票员 PIN / 本地名册（归 **M3.2**；非「方案没有」）  
- 修改 Restaurant 同步载荷契约文或 API（本仓消费已有 `split`；**本机按菜再分配不回写云**，见分单 UX 定稿）  
- 第二套 Realtime / 第二套插票  
- 云端打票页、手机直连签发  
- 混合付款  
- Agent 上 even / custom 金额分单（只做按菜 by_item 对齐，见分单 UX）

---

## 13. 实现刀建议顺序

1. 扩展映射 `DraftPartToSaleSnapshot` + issue body `mode`/`scope_id`/客户字段 + 互斥 + 按人 `request_id`  
2. 按人清草稿策略（§6）+ `cleanup_pending` + discard API  
3. `GET .../{id}` + 已开标记  
4. Admin → 正式 Admin 收银账单页（M2.6；含每人/整桌 NIF 输入）  
5. 单测与回归：方案文 §11；无 `t.Skip`  

---

## 修订记录

| 日期 | 变更 |
|------|------|
| 2026-08-21 | 首版定稿：整桌/按人工作台、API、清草稿 |
| 2026-08-21 | NIF/客户名编辑升为 **P0**；签票成功与清草稿失败解耦；明确 §13 归 M4 非本刀；按人 `request_id` 须含 `scope_id` |
| 2026-08-22 | 增补 §0 业务用语；UI 指向 M2.6 + v2 原型；「待开票账单」替代界面「草稿」 |
| 2026-08-25 | §9：工作台双 CTA 同级 + 有待开票时优先焦点（挂原型 README） |
| 2026-08-25 | §7.1：Agent→Admin SSE `bill_drafts_changed` + 侧栏角标（禁浏览器空转轮询主路径） |
| 2026-08-25 | §0/§9：**方案 A** 用语：收银账单 / 手工开票 / 新建开票 / 处理收银账单（Will 确认） |
| 2026-09-05 | §0/§9：hub CTA 收口为「收银账单」+「＋ 手工开票」；废止「处理收银账单」「新建开票」主文案 |
| 2026-08-25 | §9/§12：挂接按菜分单 UX 定稿（main view + 本机分配消账）；细节以该文为准 |
| 2026-08-25 | 与分单 UX 对齐：同单只同步一次（后续关台）；不做开票前再同步覆盖本机分单的冲突策略 |
| 2026-08-25 | §6：按人清草稿改为「全部 person 已开 **且** 剩余池空」；挂分单 UX |
| 2026-08-25 | §4/§11：废止「whole 禁 person」；整桌同步可本机代分后按人开票 |
