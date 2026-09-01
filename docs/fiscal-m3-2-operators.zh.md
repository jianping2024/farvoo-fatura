# M3.2：开票员身份（Agent 本地创建）

> **状态：定稿后置**（方案已定；**暂不实施**，以免改动登录/回归；现网继续 demo 操作员 + PIN 占位）  
> **权威：是**（开票员名册、PIN 登录、会话、角色与 `operators` 列口径；DDL 仍以 [`fiscal-sqlite-schema.zh.md`](fiscal-sqlite-schema.zh.md) + `migrations/001_init.sql` 为准）  
> **对应实现：** 待 M3.2 刀；当前仍为 `op-demo-cashier` + Admin PIN 占位  
> **计划：** [`fiscal-dev-plan.zh.md`](fiscal-dev-plan.zh.md) M3.2  
> **前置：** M2.6 Admin、M3.1 `can_issue_nc` enforce（0.4.26）

## 1. 目标

| 项 | 定法 |
|----|------|
| 人员名册 | **仅在开票 Web 界面** 创建/编辑/禁用；**不同步** Farvoo 员工 API |
| 角色 | **两类**：`owner`、`cashier`（DB 与产品 UI **同名**，禁止再用 admin / 管理员 / 操作员 作角色名） |
| 登录 | 进门校验 **PIN**（`operators.pin_hash`，argon2id）；会话绑定 **`operators.id`** |
| 开票归属 | 签发 FT/NC/ND 等写操作：`operator_id` **仅**来自服务端会话，禁止客户端自填 |
| 冲销权限 | **按账号** `operators.can_issue_nc`；`owner` 创建时默认 1、`cashier` 默认 0；`owner` 在设置页按人开关（§3.1.1） |
| 多端开票 | **一 Agent、多台开票电脑**（§3.8）；端数 **纯 Ops**（§3.8.1） |
| 发票模式 | **餐馆 / 商超** 由 **Ops** 下发（§3.9）；登录页 **不**选业态 |
| 安全边界 | 写 API **会话中间件**（§3.7）；PIN = 店员互防 + 审计；网络见 §3.7 / §3.8（单端本机 vs 店内局域网） |

## 2. 非目标

| 项 | 说明 |
|----|------|
| Farvoo `fiscal-operators` 同步 | **P0 不做**；`mesa_user_id` 仅占位满足 DDL |
| Mesa / Farvoo 账号密码复用 | Agent 不存云端登录密码；产品用语统一 **PIN**，不叫「密码」 |
| `operator_token`（每设备长期密钥） | **P0 不做**；M3.2 用 **PIN + 会话 Cookie** 覆盖多端浏览器；列/方案保留为以后加强 |
| Mesa 桌台不经 Admin 直连接 API 签发 | 仍不做（旧 §13 桌台路径）；**店内其它 PC 打开同一套开票 Web** = M3.2 多端（§3.8） |
| 第三档及以上 RBAC | 督导、frontdesk 等不再细分；一律映射为 `cashier` 或 `owner` |
| 云侧操作员副本 | 与 §1.1 一致：票库本地权威，人员名册亦仅本机 SQLite |
| 云端 PIN 重置 | 忘 PIN → 本机 `owner` 重置或运维恢复备份；P0 不做运营远程改 PIN |

## 3. P0 定法

### 3.1 角色

| `role`（DB = UI） | 创建时默认 `can_issue_nc` | 能力 |
|-------------------|---------------------------|------|
| `owner` | 1 | 设置（门店/系列/激活）、管理开票员名册、开票；默认可冲销/借记 |
| `cashier` | 0 | 开票、收银账单、发票查询/重打；冲销/借记须 `can_issue_nc=1` |

**P0：** 每店至少 **1 名** `active=1` 的 `owner`；可有多名 `cashier`；`owner` 可再创建 `owner`。

**用语：** 「Fiscal Admin」仅指开票 **Web 界面**（工程/产品壳名），与角色 `owner` **不是**同一概念。

### 3.1.1 冲销权限：`can_issue_nc`（按账号，非按角色）

| 项 | 定法 |
|----|------|
| 权威列 | `operators.can_issue_nc`（**每个账号一行**） |
| 运行时判定 | 签发 NC/ND **只读** `can_issue_nc`；**不**在 service 层用 `role=cashier` 硬挡 |
| 角色作用 | **仅默认值**：新建 `owner` → 1；新建 `cashier` → 0 |
| 谁可改 | 已登录的 **`owner`** 在设置 §5 操作员列表，按人 checkbox |
| 可覆盖 | `owner` 可为某 `cashier` 开冲销；P0 **允许**把某 `owner` 的 `can_issue_nc` 关为 0 |
| 硬约束 | 全店至少保留 **1 名** `active=1` **且** `can_issue_nc=1` 的 `owner`；禁止禁用/删掉最后一个此类 `owner` |
| 唯一写路径 | `store.SetOperatorCanIssueNC`（已有） |

### 3.2 创建与字段

| 列 | P0 定法 |
|----|---------|
| `id` | Agent 生成 UUID；= 签发 `source_id` |
| `mesa_user_id` | **占位**：`local-{id}`（满足 NOT NULL UNIQUE；**不**表示 Farvoo 用户） |
| `display_name` | 手填（如「张三」「前台 1」） |
| `pin_hash` | 创建或改 PIN 时写入 argon2id；**未设则不可登录** |
| `synced_at` | 本地创建路径 **写 NULL**（列保留，不表示云同步） |
| `active` | 禁用 = 0；不可登录、不可作 `operator_id` |

**PIN 格式（P0 定法）：** **6 位数字**；禁止字母与可变长度。

**唯一写路径（增量）：**

| 操作 | 唯一入口 |
|------|----------|
| 创建/更新开票员 | `store.UpsertOperator` / `service.UpsertOperator`（扩展 PIN、`active`） |
| 设/重置 PIN | `store.SetOperatorPIN`（禁止 handler 直写 SQL） |
| 本人改 PIN | `store.ChangeOperatorPIN`（验旧 PIN + 写新 hash） |
| 改 `can_issue_nc` | `store.SetOperatorCanIssueNC`（已有） |
| 登录校验 | `store.VerifyOperatorPIN` → 建会话 |
| 审计 | `store.InsertAuditLog`（LOGIN / LOGIN_FAILED / PIN_RESET / PIN_CHANGE / LOGOUT） |

### 3.3 开票 Web UI

| 区域 | 行为 |
|------|------|
| 登录页 | 选开票员（`display_name` 列表）+ 6 位 PIN → `POST /setup/login` |
| 侧栏 | 显示当前 `display_name`；**锁定**（清会话，回登录页） |
| 设置 §5 名册 | `display_name`、角色（`owner` / `cashier`）、是否可冲销、是否已设 PIN |
| 添加 | 名称 + 角色 + 初始 PIN（仅 **`owner`**） |
| 编辑 | 改名称/角色/禁用；**重置 PIN**（仅 `owner`，不验被重置者旧 PIN） |
| 冲销开关 | 每行 checkbox → `can_issue_nc`（仅 `owner`） |
| 修改我的 PIN | 旧 PIN + 新 PIN × 2（**`owner` / `cashier` 均可**）；`cashier` **无**自改以外路径 |

**禁止：** 生产路径写死 `OPERATOR_ID = 'op-demo-cashier'`（可保留 seed/UAT）。

### 3.4 与 M3.1 checkbox 的关系

M3.1 单 checkbox 改 **`op-demo-cashier`** 为 **临时 demo**；M3.2 落地后 **移除** 该 checkbox，改为 §5 **按人列表**。

### 3.5 冷启动：首个 `owner`

| 项 | 定法 |
|----|------|
| 触发条件 | `operators` 表 **0 行**（本店） |
| 入口 | **设置 → 激活/门店就绪** 向导 **最后一步**（与纳税人/系列/激活同流） |
| 表单 | `display_name` + 6 位 PIN × 2 |
| API | `POST /local/v1/setup/bootstrap-owner`（**无会话**；handler 内断言 `COUNT(operators)=0`，否则 403） |
| 写入 | 1 行 `role=owner`、`can_issue_nc=1`、`pin_hash` 已设 |
| 完成后 | 立即要求 `POST /login`；禁止长期无 PIN / demo 操作员生产路径 |
| 换机 | 拷库则名册随库；**新机空库**再走本流程 |

**禁止：** 独立的、可随时调用的「免登录创建 owner」API（除 `COUNT=0` 这一次性 bootstrap）。

### 3.6 PIN 策略

| 操作 | 执行者 | 验旧 PIN | 说明 |
|------|--------|----------|------|
| **重置 PIN** | 已登录 `owner` | 否 | 对任意开票员（含其他 `owner`、`cashier`）设新 6 位 PIN；写 `audit_log` `PIN_RESET` |
| **修改我的 PIN** | 当前会话本人 | **是** | 旧 PIN + 新 PIN × 2；写 `PIN_CHANGE` |
| **cashier 自改** | — | P0 **仅**「修改我的 PIN」 | 忘 PIN → 找 `owner` 重置 |
| **登录** | 任何人知 PIN | — | 连续 **5 次**失败 → 该 `operator_id` 锁定 **15 分钟**；写 `LOGIN_FAILED` |

### 3.7 安全与 API 会话（P0 必做）

**分层边界：**

| 层 | 定法 |
|----|------|
| 写 API | **必须**有效会话；`operator_id` 从会话注入，**忽略** body/query 中的 `operator_id` |
| PIN | 店员互防 + 审计（**谁**开票）；**哪台 PC** 用 `station_id` + 审计 client IP（§3.8） |
| 路由 | **默认拒绝** + 匿名白名单（§3.7.1） |

**须会话保护的写路径（非穷举）：**

- 开票、冲销/借记、重打、SAF-T、备份、换机
- 设置写：`PUT /setup/taxpayer`、`/setup/at-credentials`、`POST /setup/series/register`、`/setup/activate*`、`PUT /setup/operator`、改 PIN
- 商品/客户 LOCAL 写入

**可无会话：**

- `GET /health`、`GET /setup/status`、`GET /setup/operators`（登录页；无 `pin_hash`）
- `POST /setup/login`、`POST /setup/bootstrap-owner`（仅空库）
- bill-sync ingest

**会话：**

| 项 | 定法 |
|----|------|
| 载体 | **HttpOnly** Cookie（`SameSite=Lax`；绑 Agent 主机，本机与局域网 IP 访问通用） |
| 多端 | 本机与 `http://<AgentIP>:17880` **同一套** login/logout；禁止仅 `sessionStorage` 手填 token |
| 绝对 / 空闲过期 | 8h / 30min |
| 注销 | `POST /setup/logout`；侧栏「锁定」 |

**登录限速：** 按 `operator_id`（5 失败 / 15min）+ 按 **来源 IP**（防局域网撞库）。

**备份：** `pin_hash` 离线可撞 6 位 PIN；备份 ACL + 店网不对公网。

#### 3.7.1 实现硬约束

| # | 约束 | 说明 |
|---|------|------|
| H1 | 默认拒绝 middleware | 除白名单外 401；setup **写**路径必须在内 |
| H2 | bind 两档 | 非 loopback **须** `FISCAL_ALLOW_LAN=1`，否则拒绝启动（§3.8） |
| H3 | bootstrap 事务 | `BEGIN IMMEDIATE` + `COUNT=0` 再 INSERT，防双 owner |

### 3.8 多端开票（一 Agent · 多开票电脑）

**产品事实：** 一店常有多台 PC 都要开发票；票库与系列 **只有一份**（Agent SQLite）。其它 PC **不装第二套 Agent**，浏览器连 Agent 即可。

```text
  [Agent 机 192.168.1.10]  fiscal.db + 系列
        ▲              ▲
   PC-A 浏览器+PIN   PC-B 浏览器+PIN
   station_id=A      station_id=B  → 各映射税务打印机
```

| 项 | P0 定法 |
|----|---------|
| 人员 | `operators` **按人**；张三在 A 或 B 登录均为同一账号 |
| 客户端 | `http://<Agent局域网IP>:17880/` → 选开票员 + PIN（同 §3.3） |
| 打印机 | 每 PC 选 **档口 / `station_id`**（已有 `station_printers`） |
| 开启多端 | 安装向导或设置：「允许店内其它电脑开票」→ `FISCAL_ALLOW_LAN=1` + `FISCAL_BIND=0.0.0.0:17880`（或店网 IP） |
| 仅本机 | 默认 `127.0.0.1:17880`，不开 `FISCAL_ALLOW_LAN` |

**与 Mesa 区别：** Mesa **不**直连接 API 自动签发；多端 = 店员 **打开开票网页**，不是桌台静默开票。

**安全预期：** 店内网里、知道 PIN 的店员可开票——等同多个收银台共用一套税控；**须**店网隔离、不对公网；**不**替代 HTTPS / 每设备 token（后置）。

#### 3.8.1 限制开票端数（P0 定法 · 纯 Ops）

**可以。** 按 **「登记过的开票台」** 计数。**端数上限、登记、停用 — 仅 Ops**；门店 `owner` **无**管理入口；Agent **无**本地超级账号 / 支持密钥 / `/support/*`。

**谁管什么：**

| 能力 | 权威 | 门店 UI |
|------|------|---------|
| 上限 `max_fiscal_terminals` | **Ops**（激活 / 续费 / 改套餐） | 只读「已用 2/3」（可选） |
| 登记新开票台 | **Ops 配对码** | 页面上仅「输入配对码」；**无** owner 登记按钮 |
| 停用 / 腾出名额 | **Ops** 吊销终端 | **无**；满额文案「联系 Farvoo」 |
| 店员 PIN / 开票员名册 | 本机 `owner` | §3.3 / §5（与端数无关） |

```text
Ops Dashboard
  → 设 max、生成一次性「终端配对码」、吊销 terminal_id
       ↓
activate-from-cloud / 定期 pull → Agent 缓存 max + 同步终端吊销
       ↓
新 PC：开票页输入配对码 → Agent 向 Ops 校验 → Set-Cookie fiscal_terminal_id
```

| 项 | 定法 |
|----|------|
| 上限来源 | Ops 写入 Agent（`activate-from-cloud` 或 `PullTerminalPolicy` 类同步）；**Agent 无本地改 max API** |
| 默认 | Ops 未下发时 **1**（仅本机 `127.0.0.1`） |
| 登记 | 新 PC 无终端 Cookie → `POST /setup/terminals/pair` + `pairing_code` → **Agent 调 Ops 校验**（须联网）；成功且 `used<max` → Set-Cookie |
| 满额 | **403**「终端数已满，联系 Farvoo」 |
| 停用 | Ops 吊销 → 下次同步 `fiscal_terminals.active=0`；该 Cookie 失效 |
| 本机 | `127.0.0.1` **不占** LAN 端数 |
| 断网 | **不能**新登记终端（无本地兜底）；已登记终端在 LAN 内可继续用直至 Cookie/会话过期 |

**表 `fiscal_terminals`（M3.2 迁移）：**

| 列 | 说明 |
|----|------|
| id | UUID；Cookie `fiscal_terminal_id` |
| store_id | |
| label | Ops 配对响应可选带回 |
| active | 0 = Ops 已吊销（同步写入） |
| ops_terminal_ref | Ops 侧终端 id（吊销、对账） |
| registered_at | |
| last_seen_at | |

**API（Agent，增量）：**

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/local/v1/setup/terminals/pair` | body `pairing_code`；**代理 Ops 校验**；成功 Set-Cookie |
| GET | `/local/v1/setup/terminals/summary` | 只读 `used` / `max`（登录页提示） |

**Ops 侧（本仓外，接口约定）：** `max_fiscal_terminals`；`POST` 生成配对码；`POST` 吊销终端；Agent pull 同步吊销状态。M3.2 Agent **只消费**。

**明确不做：** 本地支持模式、`FISCAL_SUPPORT_KEY`、`/support/terminals`；owner 管理端数；门店自填 `max`；断网本地绕过 Ops 登记。

#### 3.9 发票模式（餐馆 / 商超 · 纯 Ops）

**P0 定法：** 门店是 **餐馆** 还是 **商超**，只在 **Ops** 配置；Agent **只读** 展示，登录页 **去掉**「选餐馆 / 选商超」。

**硬门槛：激活开票前必须有模式**

| 顺序 | 步骤 | 说明 |
|------|------|------|
| 1 | Ops 为门店选定 `fiscal_profile` | **餐馆** 或 **商超**；未选则 Ops **不得**开放该店税务签名激活 |
| 2 | Agent 拉取门店策略 | `activate-from-cloud` 前或同次响应须含 `fiscal_profile` + `max_fiscal_terminals` → 写入 `taxpayer_settings` |
| 3 | 店长填纳税人 / 系列等 | 与现 M1 相同 |
| 4 | 店长点 **「激活开票」** | **仅当** `fiscal_profile` 已落库；否则按钮禁用 + 文案「联系 Farvoo 配置门店模式」 |
| 5 | `activate-from-cloud` 成功 | 与现网相同拉签名钥；**同时** UI 按 `fiscal_profile` 定型（餐馆/商超菜单） |

| 项 | 定法 |
|----|------|
| 权威 | Ops（开店 / 改套餐 / Dashboard） |
| 下发 | **激活开票前**须已下发；`activate-from-cloud` 响应 **必须**含 `fiscal_profile`（及 `max_fiscal_terminals`） |
| Agent 拒激活 | Ops 未配模式 → `POST /setup/activate-from-cloud` 返回 `fiscal_profile_missing`；**不**写 `signing_keys` |
| `setup.status` | 增加 `fiscal_profile_ok`；`ready_to_issue` **须** `fiscal_profile_ok`（与纳税人/系列/激活/操作员并列） |
| 取值 | `restaurant` 或 `retail`；与 UI `body.mode-*` 一一对应 |
| 门店 UI | 设置页展示当前模式（只读）；侧栏文案；**无**切换、**无**本地选择 |
| 行为差异 | 仍按 [`fiscal-admin-ui-prototype/README.md`](fiscal-admin-ui-prototype/README.md) |
| 变更 | 仅 Ops 改；已激活门店 pull 后 UI 随 pull 更新（少见，须运维协调） |

**与端数同属 Ops 门店策略（本仓外接口约定）：**

```json
{
  "fiscal_profile": "restaurant",
  "max_fiscal_terminals": 3
}
```

**Ops 硬规则：** 创建/开通税务能力时 **`fiscal_profile` 必填**；禁止「先激活、后补模式」。

**明确不做：** 登录页业态选择；owner 本地选模式；无 `fiscal_profile` 时允许 `activate-from-cloud` 成功。

## 4. API（Local）

| 方法 | 路径 | 会话 | 说明 |
|------|------|------|------|
| GET | `/local/v1/setup/status` | 否 | 含只读 `fiscal_profile`、`terminals_used`、`max_fiscal_terminals` |
| GET | `/local/v1/setup/operators` | 否 | 登录页列表：`id`、`display_name`、`role`（**无** `pin_hash`） |
| POST | `/local/v1/setup/bootstrap-owner` | 否 | 仅 `operators` 为空；`display_name` + `pin` |
| POST | `/local/v1/setup/login` | 否 | `operator_id` + `pin` → Set-Cookie |
| POST | `/local/v1/setup/logout` | 是 | 清会话 |
| POST | `/local/v1/setup/change-pin` | 是 | `old_pin` + `new_pin`（本人） |
| PUT | `/local/v1/setup/operator` | **owner** | 创建/更新：`display_name`、`role`、`pin`（可选重置）、`active`、`can_issue_nc` |
| *写业务* | 见 §3.7 | **是** | `operator_id` 由会话注入 |

## 5. 验收清单

1. **冷启动：** 空库走完激活向导 → 创建首个 `owner` + PIN → 登录成功。  
2. **名册：** `owner` 再建 1 `owner` + 1 `cashier`，各设 PIN。  
3. **权限：** `cashier` 登录 → 可开票 → **不可**冲销（409）；`owner` 为其开 `can_issue_nc` 后可冲销。  
4. **PIN：** `owner` 重置 `cashier` PIN 成功；`cashier`「修改我的 PIN」须旧 PIN；`cashier` **无**重置他人入口。  
5. **锁定：** 连续 5 次错误 PIN → 15 分钟内拒绝；审计有 `LOGIN_FAILED`。  
6. **会话：** 未登录 `POST /fiscal-documents` → 401；登录后签发 `source_id` = 会话 `operators.id`；body 伪造 `operator_id` **无效**。  
7. **约束：** 禁止禁用全店最后一个 `active` 且 `can_issue_nc=1` 的 `owner`。  
8. **禁用：** `active=0` → 登录失败；以其 id 调写 API 403。  
9. **回归：** 无 Farvoo `fiscal-operators` 调用；`fiscal-m3-operators-regression.mjs` 全绿。  
10. **多端：** `FISCAL_ALLOW_LAN=1`；另一 PC 连 Agent IP → PIN 登录开票；不同 `station_id`。  
11. **端数（纯 Ops）：** Ops 下发 `max=2`；配对码登记 2 台成功；第 3 台 403；Ops 吊销后 pull 可再配对；**无** `/support/*`。  
12. **发票模式（纯 Ops）：** Ops 未配 `fiscal_profile` → 激活按钮不可用 / `activate-from-cloud` 失败；Ops 配 `retail` 后再激活 → UI 商超形态；**登录页无业态选择**。

## 6. 参考

| 文档 | 用途 |
|------|------|
| [`fiscal-sqlite-schema.zh.md`](fiscal-sqlite-schema.zh.md) §6.5、§6.18 | `operators`、`audit_log` |
| [`fiscal-schema-worked-example-identity.zh.md`](fiscal-schema-worked-example-identity.zh.md) §4 | 本地创建示例 |
| [`fiscal-m3-nc.zh.md`](fiscal-m3-nc.zh.md) §16 | `can_issue_nc` 与冲销 UI |

## 修订记录

| 日期 | 变更 |
|------|------|
| 2026-08-30 | 定稿：Agent 本地创建；废止 Farvoo 同步 |
| 2026-09-01 | 角色 UI 统一 `owner`/`cashier`；`can_issue_nc` 按账号 |
| 2026-09-01 | §3.9：**激活开票前**须 Ops 已配 `fiscal_profile`；`ready_to_issue` 增加校验 |
