# Fiscal Agent 开发计划（里程碑与交付物）

> **状态：定稿**（里程碑顺序与每步交付物；排期日期未定）  
> **权威：是**（本仓工程推进顺序以本文为准）  
> **对应实现：** 按里程碑落地；**当前完成至 M2.5**；下一步 M3（NC）/ M4（Farvoo Local API + §13）  
> **写作规范：** [`design-doc-standards.zh.md`](design-doc-standards.zh.md)

## 依据（只读，不替代本文交付定义）

| 文档 | 用途 |
|------|------|
| [`fiscal-sqlite-schema.zh.md`](fiscal-sqlite-schema.zh.md) | 库表权威 |
| [`fiscal-bill-draft-workbench.zh.md`](fiscal-bill-draft-workbench.zh.md) | 草稿工作台（整桌/按人）Agent 侧定稿 |
| [`fiscal-ft-receipt-layout.zh.md`](fiscal-ft-receipt-layout.zh.md) | FT 热敏票面版式（草稿；RenderESCPOS） |
| 需求 v0.17（`document/`，本仓 gitignore） | 业务/合规规则 |
| restaurant-ordering `docs/technical/farvoo-fiscal-agent-integration.zh.md` | Farvoo ↔ Agent 对接（只读） |
| [`print-agent-lessons.zh.md`](print-agent-lessons.zh.md) | 打印/安装器踩坑 |

冲突时：DDL / schema 定列；需求定业务规则；**谁先做哪一刀以本文为准**。

---

## 总原则

1. **一刀一切口可验收**：每里程碑必须有「交付物清单」+「验收命令/场景」；没有交付物不算完成。  
2. **唯一签发写路径不变**：`service.IssueDocument` → `store.IssueFT`（及后续 NC 的唯一写入口）；禁止第二套插票。  
3. **种子仅用于 M0/开发**：M1 完成后，正式路径禁止依赖 `SeedDemo` 验证码与 `DEV_PLAIN` 私钥。  
4. **本仓回归门禁**：`go test ./internal/fiscal/...` + `node scripts/fiscal-local-regression.mjs`（及里程碑新增脚本）；无 Mesa/`npm run dev` 时以此为准。  
5. **不做清单**写在各里程碑「非目标」；禁止把非目标偷塞进当刀。

---

## 进度总览

| 里程碑 | 标题 | 状态 |
|--------|------|------|
| **M0** | 纯打发票 FT（本地权威闭环） | **已完成**（`ebec039`） |
| **M1** | 身份、AT 系列、激活开票 | **已完成**（`ba9c95f`） |
| **M2** | 并入主 Agent + 真机税务打印 | **已完成** |
| **M2.5** | 账单草稿打票工作台（整桌/按人/NIF） | **本提交实现**（回归见 `fiscal-bill-sync-regression.mjs`） |
| **M3** | NC（冲销） | 未开始 |
| **M4** | Farvoo Local API 联调（含 §13 鉴权子集） | 未开始 |
| **M5** | SAF-T 月报导出 | 未开始 |
| **M6** | FS/FR/ND + 加固（认证扫尾） | 未开始（可与认证窗口并行细化） |

```text
M0 开 FT（seed）
 └─► M1 真系列/真钥
      └─► M2 托盘进程 + 真打印机
           ├─► M2.5 草稿工作台（整桌/按人）← M4 前置体验
           ├─► M3 NC
           ├─► M4 Farvoo 收银打进来
           └─► M5 SAF-T
                └─► M6 其余单据类型 / 认证材料
```

---

## M0 — 纯打发票 FT（本地权威闭环）

> **状态：已完成**

### 目标

不依赖 AT/Farvoo，本机完成：开 FT → SQLite 同事务 → `local_print_jobs` → 出纸（MemorySink/回归）。

### 非目标

AT SOAP、TPM、NC、SAF-T、Farvoo 对接、主进程托盘集成。

### 交付物（已具备）

| # | 交付物 | 路径 / 说明 |
|---|--------|-------------|
| D0.1 | DDL + 迁移嵌入 | `store/migrations/001_init.sql`，`store.Open`/`Migrate` |
| D0.2 | 合规原语（唯一） | `compliance`: `FormatSequence` / `BuildSignPayload` / `BuildQR` / money |
| D0.3 | PEM 签名器（P0/dev） | `signer.PEMSigner` + `testdata/dev_signing_key.pem` |
| D0.4 | 唯一签发写路径 | `store.IssueFT` + `service.IssueDocument` |
| D0.5 | Local API | `POST/GET /local/v1/fiscal-documents*`，`GET /local/v1/print-jobs/{id}`，`GET /local/v1/health` |
| D0.6 | 打印快照 + ESC/POS + Worker | `print.BuildPayload` / `RenderESCPOS` / `worker.Worker` |
| D0.7 | 本地进程 | `cmd/fiscal-local` + 最小 Admin 页 |
| D0.8 | 回归脚本 | `scripts/fiscal-local-uat.mjs`，`scripts/fiscal-local-regression.mjs` |
| D0.9 | 单测 | `compliance` + `store` IssueFT/幂等/打印 |

### 验收（已通过）

- `go test ./internal/fiscal/...`  
- `node scripts/fiscal-local-regression.mjs`（health / issue / sqlite / PRINTED / idempotent / by-request）  
- Admin UI 点「开 FT」返回 `SIGNED`

---

## M1 — 身份、AT 系列、激活开票

### 目标

去掉「开发者灌库才能开票」：店长可配置纳税人与 AT 子用户，注册系列拿到真 `validation_code`，完成「激活开票」拿到可解封的产品签名钥。

### 非目标

NC、SAF-T、Farvoo 桌台联调、FS/FR/ND、换机完整运维手册实现（可先写设计小节）。

### P0 定法（本里程碑必须遵守）

| 项 | 定法 |
|----|------|
| AT 密码存放 | DPAPI；`at_credentials.salt=NULL`，`wrap_meta={"scheme":"dpapi","v":1}` |
| 系列 | SOAP `registarSerie` / `consultarSeries`；验证码只进 `series.validation_code` |
| 产品钥 | 运营封装下发 `signing_keys.wrapped_private_key`；本机设备钥优先 TPM，否则 SOFTWARE+DPAPI |
| 开票 | 仍只走 `IssueFT`；日常开 FT **不读** `at_credentials` |

### 交付物

| # | 交付物 | 定义「完成」 |
|---|--------|----------------|
| D1.1 | **设计补篇** `docs/fiscal-m1-identity-series.zh.md` | 表单字段→列映射、SOAP 环境（test/prod）、错误码、与 schema 一致；状态头齐全 |
| D1.2 | **AT SOAP 客户端** `internal/fiscal/at/` | 可调用注册/查询系列；配置 `at_env`；单测用 mock transport（无官方材料时） |
| D1.3 | **凭证仓储** | `at_credentials` 读写 + DPAPI wrap/unwrap（Windows）；非 Windows stub 明确失败信息 |
| D1.4 | **系列用例** | `RegisterSeries` / `BindExistingSeries`：成功则写 `series`（含 `validation_code`、`ACTIVE`）；更新 `last_ok_at` |
| D1.5 | **激活开票用例** | 生成设备钥 → 上报公钥（可先 HTTP stub/合同）→ 写入 `agent_installations` + `signing_keys`；解封接口供签发使用 |
| D1.6 | **Local Admin：身份页** | 至少：纳税人设置、AT 凭证、系列注册/列表、激活状态；禁止再依赖手工 SQL |
| D1.7 | **签发改接真钥** | `IssueFT` 经统一 `Signer` 解封产品钥；删除或门禁 `DEV_PLAIN`（仅 `FISCAL_ALLOW_DEV_KEY=1`） |
| D1.8 | **回归** `scripts/fiscal-m1-regression.mjs`（或扩展现有脚本） | 场景：seed 关闭 → 配置纳税人 → mock AT 注册系列 → 激活 → 开 FT → 打印 PRINTED |
| D1.9 | **文档** | README「当前阶段」改为 M1；SCHEMA/计划进度表更新为 M1 完成 |

### 验收清单（必须全过，不准 skip）

1. 无 `SeedDemo` 验证码时，未注册系列 → 开 FT **失败**且错误码稳定。  
2. mock AT 返回验证码后，`series.validation_code` 非空且 `status=ACTIVE`。  
3. 激活后 `signing_keys` 有 ACTIVE 行；开 FT Hash 长度合规（Base64 ~172）。  
4. 重复 `request_id` 仍幂等。  
5. `fiscal-local-regression` 在「M1 模式」下通过（或 M1 专用脚本全绿）。

### 依赖

M0；Windows 上 DPAPI/TPM 真机测可分「本机 Mac mock + CI」与「门店 Windows 手测」两栏，手测项写入 D1.1。

---

## M2 — 并入主 Agent + 真机税务打印

> **状态：已完成**

### 目标

Fiscal Core 跑在 **正式托盘 Agent 进程**内；税务小票走既有打印机绑定与 ESC/POS，不再只靠 `fiscal-local` + MemorySink。

### 非目标

NC、SAF-T、改云端 `print_jobs` 语义、新安装器大改（若 mutex/配置名已定可只做小补丁）。

### 交付物

| # | 交付物 | 定义「完成」 |
|---|--------|----------------|
| D2.1 | **bootstrap 挂接** | 主进程启动：Open SQLite、RegisterHTTP、启动 fiscal Worker；与配对/JWT 共存 |
| D2.2 | **打印机 Sink** | Worker 按 `station_printers` / `fiscal_receipt_printer` 逻辑角色出纸；失败只改 print 状态 |
| D2.3 | **托盘入口** | 打开 Local Admin（身份/开票）的菜单项或 URL |
| D2.4 | **配置约定** | `config.json` vs SQLite 边界写进短文 `docs/fiscal-config-boundary.zh.md`（或并入 schema §8 扩写） |
| D2.5 | **回归** | Windows 或 fake-printer：`fiscal-local` 可保留；新增「主进程 + fake TCP :9100」脚本 `scripts/fiscal-m2-print-smoke.mjs` |
| D2.6 | **单测护栏** | 不破坏 `print-agent-lessons` 已有测试；新增 fiscal mount 烟测 |

### 验收清单

1. 只起主 Agent（不起 `fiscal-local`）→ `GET /local/v1/health` ok。  
2. 开 FT → `local_print_jobs` → fake-printer 或真机出现 ATCUD/QR 相关内容。  
3. 打印失败后发票行仍在且 `document_status=SIGNED`。  
4. 云端业务 `print_jobs` 路径仍可认领（不回归打印模块）。

### 依赖

M1（至少能 ACTIVE 系列 + Signer）；打印机 fake 可在 M1 未完成时用 seed 并行冒烟，但 **M2 关闭条件仍要求 M1 主路径**。

---

## M2.5 — 账单草稿打票工作台（整桌 / 按人）

> **状态：已完成**（本仓实现 + `fiscal-bill-sync-regression.mjs`）  
> **权威方案：** [`fiscal-bill-draft-workbench.zh.md`](fiscal-bill-draft-workbench.zh.md)

### 目标

本机工作台对 `bill_sync_drafts`：整桌一键开 FT（收编现有 MVP）；`split` 按人开/补票；**开票前可填 NIF（整桌一处 / 按人各自，默认可散客）**；互斥与清草稿规则按方案文；签发仍唯一 `IssueDocument` → `IssueFT`。

### 非目标

- 改 restaurant-ordering 契约文档或同步载荷  
- NC/重打完整工作台、云端打票页  
- **§13 PIN / `operator_token` / 开票终端**（完整方案有；**归 M4**，本刀继续本机 Admin 信任）  
- 混合付款（P1）  
- 第二套插票 / 第二套 Realtime  

### 交付物（实现刀）

| # | 交付物 | 定义「完成」 |
|---|--------|----------------|
| D2.5.0 | **方案文** | 工作台文已定稿（含 NIF P0、`cleanup_pending`） |
| D2.5.1 | **映射** | `DraftPartToSaleSnapshot`（或同包单出口）；禁止第二套拼装；按人 `request_id` 含 `scope_id` |
| D2.5.2 | **Issue API** | `POST .../issue` 带 `mode`/`scope_id`/`customer_nif`/`customer_name`；互斥；按人清草稿；签票成功与删草稿失败解耦 |
| D2.5.3 | **Discard** | `POST .../discard` → `DeleteBillDraftsBySale` |
| D2.5.4 | **详情 + 已开标记** | `GET .../{id}`；按业务键标已开 scope |
| D2.5.5 | **UI** | `17880` 草稿工作台；整桌/每人独立 NIF 输入 |
| D2.5.6 | **回归** | 方案文 §11（含 NIF 与 `cleanup_pending`）；无 `t.Skip` |

### 验收清单

见 [`fiscal-bill-draft-workbench.zh.md`](fiscal-bill-draft-workbench.zh.md) §11。

### 依赖

M2（主进程 + 开票可用）；账单同步入草稿（已有）。宜在 **M4 大联调前**完成，避免收银只能整桌 demo。

---

## M3 — NC（冲销）

### 目标

独立 NC 系列；对已签 FT 开具 NC；打印展示原票引用；幂等。

### 非目标

ND、FS、FR；自动「NC 冲 NC」产品化；会计完整对账 UI。

### 交付物

| # | 交付物 | 定义「完成」 |
|---|--------|----------------|
| D3.1 | **设计补篇** `docs/fiscal-m3-nc.zh.md` | 引用字段、金额规则、系列命名、权限 `can_issue_nc` |
| D3.2 | **唯一写路径** `store.IssueNC`（或统一 `IssueDocument` 内分支，但仍单一入口） | 写 `invoice_line_references`；更新原票 `document_status` / `credited_gross_total` |
| D3.3 | **API** | `POST /local/v1/fiscal-documents/{id}/credit-notes` 按对接说明 |
| D3.4 | **Admin UI** | 选原 FT → 填原因/行 → 开 NC（需登录/PIN，P0 可简化为已同步 operator） |
| D3.5 | **打印** | NC payload 含原 `InvoiceNo`、原因；Render 路径仍唯一 |
| D3.6 | **回归** | `scripts/fiscal-m3-regression.mjs`：FT→NC→幂等→打印；禁止第二张重复 NC（同业务键） |

### 验收清单

1. NC 使用 **独立** `series`（不能蹭 FT 序号）。  
2. ATCUD/Hash/InvoiceNo 均经 `compliance.Format*` 唯一函数。  
3. 原 FT 状态变为 `CREDITED_PARTIAL` 或 `CREDITED_FULL`（与金额一致）。  
4. 重复 credit 请求幂等，不双开。

### 依赖

M1（NC 系列也要 validation_code）；M2 建议已完成以便真打 NC，否则允许 MemorySink 验收逻辑、打印版式在 M2 后补测。

---

## M4 — Farvoo Local API 联调

### 目标

Farvoo 桌台「打印发票」经 Agent Local API 只开 FT；业务幂等键与对接说明一致；`sync_outbox` 异步副本（云不可用时不阻断开票）。

### 非目标

手机/PWA 直连 Agent；云端分配发票号；收款自动开票。

### 交付物

| # | 交付物 | 定义「完成」 |
|---|--------|----------------|
| D4.1 | **契约冻结** | 请求/响应 JSON 示例落入 `docs/fiscal-local-api.zh.md`（与 v0.17 / 对接说明字段对齐） |
| D4.2 | **鉴权（§13 子集）** | Local API：本机默认可保留开发；LAN / 正式收银路径：开票终端凭证 + 操作员（`operator_token` / PIN，按对接说明 P0 子集）；**含**工作台 `/bill-drafts/.../issue` 与 `fiscal-documents` |
| D4.3 | **业务幂等** | `store_id+source_system+source_sale_id+scope_*+fiscal_purpose` 已实现并有测试 |
| D4.4 | **sync_outbox** | 签发成功写 outbox；worker 推送（可先 stub Farvoo endpoint + 重试字段） |
| D4.5 | **联调记录** | `docs/fiscal-m4-farvoo-uat.zh.md`：用 mesa/Farvoo 测账号打一单 FT 的步骤与结果 |
| D4.6 | **回归** | 扩展 UAT：模拟 Farvoo body → 开 FT → outbox `PENDING`→（mock）`SENT` |

### 验收清单

1. 前端不传 InvoiceNo/Hash/ATCUD 仍能开票。  
2. 同业务键不同 `request_id` 返回原票。  
3. 断网（mock 推送失败）本地仍 `SIGNED` + 可打印。  
4. Farvoo 侧能看到约定副本字段（若云端尚未就绪：本里程碑以 Agent 契约 + mock 收口，并在 D4.5 标明阻塞项）。

### 依赖

M2（收银打本机 Agent）；M1。

---

## M5 — SAF-T 月报导出

### 目标

路线 A：按月生成 SAF-T(PT) 1.04_01 结构文件；归档 `saft_exports`；校验与票面/QR/库一致抽样。

### 非目标

e-Fatura 逐票实时上报；会计全量 SAF-T（另一套）。

### 交付物

| # | 交付物 | 定义「完成」 |
|---|--------|----------------|
| D5.1 | **设计补篇** `docs/fiscal-m5-saft.zh.md` | 节点映射表、期间边界、时区、与 schema `saft_exports` 列一致 |
| D5.2 | **导出器** `internal/fiscal/saft/` | 输入 store_id+年月 → XML 文件 + sha256 |
| D5.3 | **Admin** | 选择期间 → 导出 → 下载/路径展示 → validation_status |
| D5.4 | **金样/夹具** | 至少 1 张 FT（+可选 NC）导出后 XSD 或结构化断言（无 XSD 工具则固定节点存在性断言） |
| D5.5 | **回归** | `scripts/fiscal-m5-regression.mjs`：开票→导出→文件存在→关键字段=库 |

### 验收清单

1. 导出中的 InvoiceNo/ATCUD/Hash/GrossTotal 与库一致。  
2. 无票月份行为有定法（空文件或拒绝）并文档写明。  
3. 重复导出不破坏历史 `saft_exports` 行（新行或版本策略在 D5.1 拍板）。

### 依赖

M0+M1；有 NC 时 M3 票也进同一文件。

---

## M6 — FS/FR/ND + 认证扫尾

### 目标

认证范围内其余单据类型可注册系列并签发；补齐认证材料与运维（备份校验、换机）。

### 非目标

餐厅结账默认映射到 FS（需求：桌台默认 FT）。

### 交付物

| # | 交付物 | 定义「完成」 |
|---|--------|----------------|
| D6.1 | 各类型系列注册与签发（与 FT 同入口，类型可选） | API/UI 可选 `document_type`；各有独立 series |
| D6.2 | 认证检查清单 `docs/fiscal-certification-checklist.zh.md` | Hash 金样、QR、ATCUD、系列、SAF-T、打印、权限 |
| D6.3 | 备份/恢复校验工具或菜单 | 恢复后校验 last_number/last_hash；失败阻断系列 |
| D6.4 | 换机流程实现（与对接说明 §12 对齐的最小集） | revoked 旧 installation + 新 wrap |

### 验收清单

以 D6.2 清单逐项 pass/fail 出具报告；认证窗口前冻结行为变更。

### 依赖

M1–M5 主路径稳定。

---

## 跨里程碑工程规范（每刀都要）

| 项 | 要求 |
|----|------|
| 文档 | 新设计补篇带状态头；改表同步 schema + DDL |
| 唯一写法 | 序号/Hash/QR/签发/打印构建函数不得分叉 |
| 测试 | 里程碑关闭前：单测 + 对应 regression 脚本全绿 |
| 提交 | 用户要求时再 commit；默认按里程碑一次或逻辑清晰的多次 |
| 演示页 | `fiscal-local` Admin 可保留为开发页；正式能力以托盘/正式 Admin 为准 |

---

## 明确不在计划内（除非新开里程碑）

- 库存 / 采购 / 云端正式票队列权威化  
- 手机直连签发  
- e-Fatura 实时逐票 Webservice  
- 多 Agent 每店部署  

---

## 修订记录

| 日期 | 变更 |
|------|------|
| 2026-08-20 | 首版定稿：M0–M6、每步交付物与验收；M0 已完成 |
| 2026-08-20 | M1 完成：身份/系列/激活 + `fiscal-m1-regression.mjs` |
| 2026-08-20 | M2 完成：主进程 `-fiscal-standalone` 嵌入 + `ResolveFiscalPrinterTCP`/`TCPSink` + `fiscal-m2-print-smoke.mjs` |
| 2026-08-21 | 增补 **M2.5** 草稿工作台里程碑；挂 `fiscal-bill-draft-workbench.zh.md` |
| 2026-08-21 | M2.5：NIF 编辑升为 P0；§13 明确归 M4；进度头标明「下一步 = 实现 M2.5」 |
| 2026-08-21 | **M2.5 完成**：按人/NIF/`discard`/详情已开标记 + 回归全绿；VERSION 0.3.87 |
