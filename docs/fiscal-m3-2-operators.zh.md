# M3.2：开票员身份（Agent 本地创建）

> **状态：定稿**（M3.2 + M3.2b + **M3.2c 三档 RBAC 已实施**）  
> **权威：是**（开票员名册、PIN 登录、会话、角色与 `operators` 列口径；DDL 以 [`fiscal-sqlite-schema.zh.md`](fiscal-sqlite-schema.zh.md) + `migrations/*.sql` 为准）  
> **对应实现：** `apps/fiscal-agent` M3.2（会话 Cookie）；M3.2b（`session_epoch`、开票员管理 UI、停用吊销）**已实施**；M3.2c（`admin` / `owner` / `cashier` 三档、设置页分区权限、bootstrap-admin）**已实施**  
> **计划：** [`fiscal-dev-plan.zh.md`](fiscal-dev-plan.zh.md) M3.2  
> **前置：** M2.6 开票 Web、M3.1 `can_issue_nc` enforce（0.4.26）

## 1. 目标

| 项 | 定法 |
|----|------|
| 人员名册 | **仅在开票 Web 界面** 创建/编辑/禁用；**不同步** Farvoo 员工 API |
| 角色 | **三档**：`admin`、`owner`、`cashier`（DB 值）；UI 分别显示 **管理员**、**店长**、**店员** |
| 登录 | 进门校验 **PIN**（`operators.pin_hash`，argon2id）；会话绑定 **`operators.id`** |
| 开票归属 | 签发 FT/NC/ND 等写操作：`operator_id` **仅**来自服务端会话，禁止客户端自填 |
| 冲销权限 | **按账号** `operators.can_issue_nc`；新建 `admin` / `owner` 默认 1、`cashier` 默认 0；`admin` / `owner` 在设置页按人开关（§3.1.1） |
| 多端开票 | **一 Agent、多台开票电脑**（§3.8）；端数 **纯 Ops**（§3.8.1） |
| 发票模式 | **餐馆 / 商超** 由 **Ops** 下发（§3.9）；登录页 **不**选业态 |
| 安全边界 | 写 API **会话中间件**（§3.7）；PIN = 店员互防 + 审计；网络见 §3.7 / §3.8（单端本机 vs 店内局域网） |

**用语：** 「开票 Web」指店员使用的开票界面（工程路径 `bootstrap/admin`），与 DB 角色名 **不是**同一概念。

## 2. 非目标

| 项 | 说明 |
|----|------|
| Farvoo `fiscal-operators` 同步 | **P0 不做**；`mesa_user_id` 仅占位满足 DDL |
| Mesa / Farvoo 账号密码复用 | Agent 不存云端登录密码；产品用语统一 **PIN**，不叫「密码」 |
| `operator_token`（每设备长期密钥） | **P0 不做**；M3.2 用 **PIN + 会话 Cookie** 覆盖多端浏览器；列/方案保留为以后加强 |
| Mesa 桌台不经 Admin 直连接 API 签发 | 仍不做（旧 §13 桌台路径）；**店内其它 PC 打开同一套开票 Web** = M3.2 多端（§3.8） |
| 第四档及以上 RBAC | 督导、frontdesk 等不再细分；一律映射为 `cashier`、`owner` 或 `admin` |
| 云侧操作员副本 | 与 §1.1 一致：票库本地权威，人员名册亦仅本机 SQLite |
| 云端 PIN 重置 | 忘 PIN → 本机 `admin` / `owner` 重置或运维恢复备份；P0 不做运营远程改 PIN |
| UI 再增「操作员 / 管理员」等第四套角色名 | 产品界面只用 **管理员 / 店长 / 店员**；DB 只用 §3.1 三值 |

## 3. P0 定法

### 3.1 角色

| DB `role` | UI 显示 | 创建时默认 `can_issue_nc` | 日常开票 | 设置页 |
|-----------|---------|---------------------------|----------|--------|
| `admin` | 管理员 | 1 | **是** | **全部分区**（§3.3.3） |
| `owner` | 店长 | 1 | **是** | **子集**（§3.3.3）；**不可**见开票授权链 |
| `cashier` | 店员 | 0 | **是** | **不可见、不可进入** |

**创建权限（P0 定法）：**

| 执行者 | 可创建 / 管理的 `role` |
|--------|------------------------|
| **bootstrap**（空库一次） | 仅创建 **1 名** `admin`（§3.5） |
| **`admin`** | **`owner`、`cashier`**（**不得** bootstrap 以外新增第二名 `admin`） |
| **`owner`** | **仅** `cashier`（不得创建 / 改 role 为 `admin` 或 `owner`） |
| **`cashier`** | — |

**P0 人数约束：**

- 每店至少 **1 名** `active=1` 的 **`admin`**
- 每店至少 **1 名** `active=1` 的 **`owner`**
- 可有多名 `cashier`

**禁止：** bootstrap 以外通过 UI / API **新增**第二个 `admin`；`owner` **不得**创建或提升为 `owner` / `admin`。

### 3.1.1 冲销权限：`can_issue_nc`（按账号，非按角色）

| 项 | 定法 |
|----|------|
| 权威列 | `operators.can_issue_nc`（**每个账号一行**） |
| 运行时判定 | 签发 NC/ND **只读** `can_issue_nc`；**不**在 service 层用 `role=cashier` 硬挡 |
| 角色作用 | **仅默认值**：新建 `admin` → 1；新建 `owner` → 1；新建 `cashier` → 0 |
| 谁可改 | **`admin`**：任意开票员；**`owner`**：**仅** `cashier` 行的 checkbox |
| 可覆盖 | `admin` 可为某 `cashier` 开冲销；P0 **允许**把某 `owner` 的 `can_issue_nc` 关为 0 |
| 硬约束 | 全店至少保留 **1 名** `active=1` **且** `can_issue_nc=1` 的 **`owner`**（§3.7.3） |
| 唯一写路径 | `store.SetOperatorCanIssueNC` |

### 3.2 创建与字段

| 列 | P0 定法 |
|----|---------|
| `id` | Agent 生成 UUID；= 签发 `source_id` |
| `mesa_user_id` | **占位**：`local-{id}`（满足 NOT NULL UNIQUE；**不**表示 Farvoo 用户） |
| `display_name` | 手填（如「张三」「前台 1」） |
| `pin_hash` | 创建或改 PIN 时写入 argon2id；**未设则不可登录** |
| `synced_at` | 本地创建路径 **写 NULL**（列保留，不表示云同步） |
| `active` | 禁用 = 0；不可登录、不可作 `operator_id`；**须**同步吊销会话（§3.10） |
| `session_epoch` | 非负整数，默认 0；登录 Cookie 携带当前值；吊销时 +1（§3.7.2） |

**PIN 格式（P0 定法）：** **6 位数字**；禁止字母与可变长度。

**唯一写路径（增量）：**

| 操作 | 唯一入口 |
|------|----------|
| 创建/更新开票员（名称、角色） | `store.UpsertOperator` / `service.UpsertOperator` |
| 停用/启用 | `store.SetOperatorActive`（**禁止** `UpsertOperator` 隐式写 `active`） |
| 设/重置 PIN | `store.SetOperatorPIN`；**须**调用 `BumpOperatorSessionEpoch`（§3.7.2） |
| 本人改 PIN | `store.ChangeOperatorPIN`；**须**调用 `BumpOperatorSessionEpoch` |
| 改 `can_issue_nc` | `store.SetOperatorCanIssueNC` |
| 吊销会话代数 | `store.BumpOperatorSessionEpoch`（禁止 handler 直写 SQL） |
| 登录校验 | `store.VerifyOperatorPIN` → 建会话 |
| 会话中间件读库 | `store.GetOperatorSessionState`（`active`、`role`、`session_epoch`） |
| 审计 | `store.InsertAuditLog`（LOGIN / LOGIN_FAILED / PIN_RESET / PIN_CHANGE / LOGOUT / OPERATOR_DEACTIVATE / OPERATOR_ACTIVATE） |

### 3.3 开票 Web UI

**用语：** 产品界面称 **开票员** / **开票员管理**；**禁止**称 Farvoo「员工」或暗示与云端员工同步。

#### 3.3.1 导航与设置可见性（M3.2c · 待实现）

| 角色 | 侧栏（工作台 / 开票 / 账单 / 发票 / 商品 / 客户） | 侧栏「设置」 | 设置页 |
|------|---------------------------------------------------|--------------|--------|
| `admin` | ✓ | ✓ | **全部分区**（§3.3.3） |
| `owner` | ✓ | ✓ | **子集**（§3.3.3） |
| `cashier` | ✓ | **不可见** | **不可进入**（`showView('settings')` 守卫） |

| 项 | 定法 |
|----|------|
| 修改我的 PIN | **侧栏**入口（三档均可）；**不**放在设置内 |
| 角色判断唯一写法 | Admin JS：`operatorRole()` + `canAccessSettings()` + `canManageProvisioning()` + `canManageOperators()` + `applyOperatorAccess()`（**禁止**回退 `isLoggedInOwner()` 二元写法） |

#### 3.3.3 设置页分区可见性（M3.2c · P0 定法）

分区 id 与现 Admin HTML `data-settings-section` 一致。

| 分区 | `data-settings-section` | `admin` | `owner` | `cashier` |
|------|-------------------------|:-------:|:-------:|:---------:|
| 就绪概览 | `overview` | 完整卡片 | **简化**（隐藏系列/授权/激活相关卡片） | — |
| 门店与税务 · **门店信息** | `store`（纳税人字段） | ✓ 读写 | ✓ 读写 | — |
| 门店与税务 · **税务平台凭证** | `store`（AT 字段） | ✓ 读写 | **不可见** | — |
| 系列与开票授权 | `series` | ✓ | **不可见** | — |
| 开票员名册 | `operators` | ✓ 全表 | ✓ **仅 `cashier` 行** | — |
| 打印机 | `printers` | ✓ | ✓ | — |
| SAF-T 月导 | `saft` | ✓ | ✓ | — |
| 备份与换机 | `advanced` | ✓ | **不可见** | — |

| 项 | 定法 |
|----|------|
| 配置向导 | **仅 `admin`**；`ready_to_issue=false` 时展示 |
| CSS 门控 | `settings-admin-only` / `settings-manager-only`（**禁止**复用 M3.2b 的 `owner-only` 表示「全设置权限」） |
| `owner` 名册 UI | 添加开票员弹窗 **无**角色下拉（固定创建 `cashier`）；表格 **不展示** `admin` / `owner` 行 |

**「开票授权等一系列」= `admin` 独占（P0）：** AT 凭证、系列注册（FT/FS/NC/ND）、`activate-from-cloud`、本地 PEM 激活、备份/换机/完整性校验、就绪概览中的授权/激活卡片。

#### 3.3.2 开票员管理（设置 §operators · M3.2b + M3.2c）

与商品/客户同级：`admin-list-panel` + 表格（P0 不分页）。

| 列 | 内容 |
|----|------|
| 姓名 | `display_name` |
| 角色 | UI：**管理员** / **店长** / **店员**（DB：`admin` / `owner` / `cashier`） |
| PIN | 已设置 / 未设置 |
| 可冲销 | 开关 → `can_issue_nc`（`owner` 仅对 `cashier` 行） |
| 状态 | 启用 / 已停用 |
| 操作 | 编辑 · 重置 PIN · 停用/启用 |

| 操作 | `admin` | `owner` |
|------|---------|---------|
| 添加开票员 | 弹窗：姓名、**角色**（`owner` / `cashier`）、初始 PIN | 弹窗：姓名、初始 PIN（**固定 `cashier`**） |
| 编辑 | 任意行（**不得**改 `admin` 的 `role`；不得新增 `admin`） | **仅** `cashier`：改姓名 |
| 重置 PIN | 任意非自身或含自身 | **仅** `cashier` |
| 停用/启用 | 任意行（受 §3.7.3 约束） | **仅** `cashier` |

| 区域 | 行为 |
|------|------|
| 登录页 | 选开票员（仅 `active=1` 且已设 PIN）+ 6 位 PIN → `POST /setup/login` |
| 侧栏 | 当前 `display_name`；**退出**；**修改 PIN** |

**禁止：** 生产路径写死 `OPERATOR_ID = 'op-demo-cashier'`（可保留 seed/UAT）。

### 3.4 与 M3.1 checkbox 的关系

M3.1 单 checkbox 改 **`op-demo-cashier`** 为 **临时 demo**；M3.2 落地后 **移除** 该 checkbox，改为 §5 **按人列表**。

### 3.5 冷启动：首个 `admin`

| 项 | 定法 |
|----|------|
| 触发条件 | `operators` 表 **0 行**（本店） |
| 入口 | **登录页**（仅 Agent 本机 `127.0.0.1` / `::1`）；设置 **无** bootstrap |
| 表单 | `display_name` + 6 位 PIN；UI 文案 **「创建管理员并登录」** |
| API | `POST /local/v1/setup/bootstrap-owner`（**路径名保留**；**无会话**；**仅 loopback**；handler 内断言 `COUNT(operators)=0`，否则 403） |
| 写入 | 1 行 `role=admin`、`can_issue_nc=1`、`pin_hash` 已设 |
| 完成后 | 立即要求 `POST /login`；由 **admin** 再创建 `owner` / `cashier` |
| 换机 | 拷库则名册随库；**新机空库**再走本流程 |

**禁止：** 独立的、可随时调用的「免登录创建 admin」API（除 `COUNT=0` 这一次性 bootstrap）；bootstrap **不得**再写 `role=owner`。

### 3.6 PIN 策略

| 操作 | 执行者 | 验旧 PIN | 说明 |
|------|--------|----------|------|
| **重置 PIN** | 已登录 **`admin`** 或 **`owner`**（仅其可管行，§3.3.2） | 否 | 设新 6 位 PIN；`PIN_RESET`；**须** `BumpOperatorSessionEpoch` |
| **修改我的 PIN** | 当前会话本人 | **是** | 旧 PIN + 新 PIN × 2；`PIN_CHANGE`；**须** `BumpOperatorSessionEpoch` |
| **`cashier` 自改** | — | P0 **仅**「修改我的 PIN」 | 忘 PIN → 找 **`admin` / `owner`** 重置 |
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
- `POST /setup/login`、`POST /setup/bootstrap-owner`（仅空库 + **仅 loopback**）
- bill-sync ingest

**会话：**

| 项 | 定法 |
|----|------|
| 载体 | **HttpOnly** Cookie（`SameSite=Lax`；绑 Agent 主机，本机与局域网 IP 访问通用） |
| Payload | `operator_id`、`role`、`display_name`、`issued_at`、`last_seen`、**`epoch`**（= 登录时 `operators.session_epoch`） |
| 多端 | 本机与 `http://<AgentIP>:17880` **同一套** login/logout；禁止仅 `sessionStorage` 手填 token |
| 绝对 / 空闲过期 | 8h / 30min |
| 注销 | `POST /setup/logout`；侧栏「退出」 |
| 运行时校验 | 每次受保护请求 **查库** `active`、`session_epoch`；middleware **以库中 `role` 为准**（§3.7.2、§3.7.4） |

**登录限速：** 按 `operator_id`（5 失败 / 15min）。按 **来源 IP** 防撞库 → **P1**（§7 备选）。

**备份：** `pin_hash` 离线可撞 6 位 PIN；备份 ACL + 店网不对公网。

#### 3.7.4 API 鉴权档位（M3.2c · P0 定法）

middleware 在 M3.2b 二元 `authOwner` 上拆为三档（命名实现时可调整，**语义**如下）：

| 档位 | 允许 `role` | 路径（P0） |
|------|-------------|------------|
| `authPublic` | — | §3.7 白名单 |
| `authSession` | `admin`、`owner`、`cashier` | 开票、商品/客户、改 PIN、读打印档口等 |
| `authManager` | **`admin`、`owner`** | `PUT /setup/taxpayer`；`GET/POST /local/v1/saft/exports*`；`GET/PUT /setup/operators/manage`；`PUT /setup/operator`（受 §3.1 角色写入限制） |
| `authAdmin` | **`admin`  only** | `PUT /setup/at-credentials`；`POST /setup/series/register`；`POST /setup/activate`；`POST /setup/activate-from-cloud`；`POST /setup/backup`；`POST /setup/integrity/verify`；`POST /setup/prepare-swap` |

**`PUT /setup/operator` service 约束（P0）：**

| 会话 `role` | 允许 body.`role` | 允许目标 |
|-------------|------------------|----------|
| `admin` | `owner`、`cashier` | 任意行；**禁止**新建 / 提升为 `admin` |
| `owner` | **`cashier` only** | **仅** `cashier` 行；403 若目标为 `admin` / `owner` |
| `cashier` | — | 403 |

**禁止：** UI 隐藏但 API 仍 200（须 middleware + service 双挡）。

#### 3.7.2 会话吊销：`session_epoch`（P0 定法 · M3.2b）

| 项 | 定法 |
|----|------|
| 存储 | `operators.session_epoch` INTEGER NOT NULL DEFAULT 0 |
| Cookie | 登录写入 `epoch`；middleware 比较 `cookie.epoch` 与库值 |
| `cookie.epoch < db.session_epoch` | **401** `session_revoked` |
| `active != 1` | **401** `operator_inactive` |
| 服务端 | **不能**主动删除他端浏览器 Cookie；靠下一请求 401 + 客户端 `forceLogout` |

**`session_epoch += 1`（唯一入口 `BumpOperatorSessionEpoch`）：**

| 事件 | bump |
|------|------|
| 停用 `active=0` | **是** |
| admin 重置他人 PIN | **是** |
| 本人 `change-pin` | **是** |
| 改 `role` | **是** |
| 启用 `active=1` | **否**（旧 Cookie 仍无效，须重登） |
| 改 `display_name` | **否** |
| 改 `can_issue_nc` | **否** |

#### 3.7.3 停用硬约束（M3.2c · P0 定法）

执行停用、降权或 `PUT /setup/operator` 改 `role` 前，service **必须**断言（违反 → **409** `last_owner_constraint` 或专用 `last_admin_constraint`）：

1. 全店至少 **1** 名 `active=1` 且 `role=admin`  
2. 全店至少 **1** 名 `active=1` 且 `role=owner`  
3. 全店至少 **1** 名 `active=1` 且 `role=owner` 且 `can_issue_nc=1`（§3.1.1）  

**附加：**

| 项 | 定法 |
|----|------|
| 停用 `admin` | **禁止**（409）；仅运维拷库 / 迁移场景处理 |
| `owner` 操作 `admin` / `owner` 行 | **403**（不得到达 store 层约束） |
| bootstrap 迁移 | 见 §3.11；迁移后旧 `admin` Cookie 因 `session_epoch` bump 须重登 |

**唯一判定：** `store` 层；禁止 UI 单独挡而不写库。

#### 3.7.1 实现硬约束

| # | 约束 | 说明 |
|---|------|------|
| H1 | 默认拒绝 middleware | 除白名单外 401；setup **写**路径必须在内 |
| H2 | bind 两档 | 非 loopback **须** `FISCAL_ALLOW_LAN=1`，否则拒绝启动（§3.8） |
| H3 | bootstrap 事务 | `BEGIN IMMEDIATE` + `COUNT(operators)=0` 再 INSERT **`role=admin`**，防双 admin |
| H4 | bootstrap 网络 | **仅 loopback** 可调 `bootstrap-owner`；LAN 其它 PC 只能登录已有账号 |
| H5 | 会话查库 | 受保护请求 **禁止** 仅信 Cookie 内 `role`；须 `GetOperatorSessionState` |
| H6 | 停用吊销 | `SetOperatorActive(false)` **必须** bump `session_epoch` |

### 3.10 停用与客户端清场（P0 定法 · M3.2b）

```text
admin 或 owner 停用 cashier
  → DB: active=0; session_epoch+=1; audit OPERATOR_DEACTIVATE
  → 目标端下一 API 请求: 401 (operator_inactive | session_revoked)
  → Admin forceLogout(): 清 Cookie + 本地缓存 + SSE
```

**`forceLogout` 唯一清场路径（Admin JS）：**

| 清理项 | 说明 |
|--------|------|
| `POST /setup/logout` | 清 `fiscal_session` Cookie |
| `loggedInOperator` | `null` |
| `setupStatusCache` | `null` |
| `document.body` 角色 class | 移除 `operator-admin`、`operator-manager`（M3.2b 的 `operator-owner` 一并移除） |
| 账单 SSE | `stopBillDraftEvents()` |
| PIN 垫 | 清空 |
| `#app-shell` | 隐藏；显示登录页 |
| 会话相关 `sessionStorage` | 清空（如发票 Tab） |

**`applyOperatorAccess` body class（M3.2c）：**

| class | 条件 |
|-------|------|
| `operator-admin` | `role=admin` |
| `operator-manager` | `role=owner` |

**触发：** 用户点退出；或 `j()` 收到 401 且 `error` ∈ `session_revoked`、`operator_inactive`、`unauthorized`。

**P0 定法：** 禁止各页面分散清缓存；**唯一** HTTP 封装处理 401 吊销。

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
| 店员 PIN / 开票员名册 | 本机 **`admin` / `owner`** | §3.3 / §operators（与端数无关） |

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
| 3 | **`admin`** 填纳税人 / 系列等 | 与现 M1 相同 |
| 4 | **`admin`** 点 **「从运营同步开票授权」** | **仅当** 已打印配对且 §1 门店信息已保存；进入设置页会自动尝试一次；一次请求拉齐 `fiscal_profile` + 封装私钥 |
| 5 | `activate-from-cloud` 成功 | UI 按 `fiscal_profile` 定型（餐馆/商超菜单） |

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

**明确不做：** 登录页业态选择；**`owner` / `cashier`** 本地选模式或同步开票授权；无 `fiscal_profile` 时允许 `activate-from-cloud` 成功。

### 3.11 升级迁移：bootstrap `owner` → `admin`（方案 A · P0 定法）

**适用：** M3.2 / M3.2b 已部署、首位 bootstrap 账号仍为 `role=owner` 的库。

| 项 | 定法 |
|----|------|
| SQL | `migrations/007_operators_bootstrap_admin.sql` |
| 规则 | 对每个 `store_id`：若 **尚无** `role=admin`，则将 **`created_at` 最早** 且当前 `role=owner` 的那一行改为 `admin` |
| 副作用 | 同行 `can_issue_nc=1`；`session_epoch+=1`（旧 Cookie 须重登） |
| 新装空库 | 不命中 UPDATE；bootstrap 直接写 `admin` |
| 运维 | 迁移后 **admin** 须登录并 **创建至少 1 名 `owner`**，否则违反 §3.7.3 约束（仅当 admin 尝试停用末位 owner 时触发；新建 owner 为常规流程） |

**禁止：** 把全部 `owner` 批量改为 `admin`；禁止迁移脚本新增第二名 `admin`。

## 4. API（Local）

| 方法 | 路径 | 会话档位 | 说明 |
|------|------|----------|------|
| GET | `/local/v1/setup/status` | 否 | 含只读 `fiscal_profile`、`terminals_used`、`max_fiscal_terminals` |
| GET | `/local/v1/setup/operators` | 否 | **登录页专用**：`active=1` 且已设 PIN；`id`、`display_name`、`role`（**无** `pin_hash`） |
| GET | `/local/v1/setup/operators/manage` | **authManager** | **`admin`**：全表；**`owner`**：**仅 `cashier` 行** |
| POST | `/local/v1/setup/bootstrap-owner` | 否 | 仅 `operators` 为空；**仅 loopback**；写入 **`role=admin`** |
| POST | `/local/v1/setup/login` | 否 | `operator_id` + `pin` → Set-Cookie（含 `epoch`） |
| POST | `/local/v1/setup/logout` | 是 | 清本会话 Cookie |
| POST | `/local/v1/setup/change-pin` | 是 | `old_pin` + `new_pin`（本人）；成功后 bump `session_epoch` |
| PUT | `/local/v1/setup/taxpayer` | **authManager** | **`admin` / `owner`** |
| PUT | `/local/v1/setup/at-credentials` | **authAdmin** | **`admin` only** |
| POST | `/local/v1/setup/series/register` | **authAdmin** | **`admin` only** |
| POST | `/local/v1/setup/activate` | **authAdmin** | **`admin` only** |
| POST | `/local/v1/setup/activate-from-cloud` | **authAdmin** | **`admin` only** |
| PUT | `/local/v1/setup/operator` | **authManager** | §3.7.4 角色写入限制；`active` 经 `SetOperatorActive` |
| POST | `/local/v1/setup/backup` | **authAdmin** | **`admin` only** |
| POST | `/local/v1/saft/exports` | **authManager** | **`admin` / `owner`** |
| GET | `/local/v1/saft/exports` | **authManager** | **`admin` / `owner`** |
| GET | `/local/v1/saft/exports/{exportId}` | **authManager** | **`admin` / `owner`** |
| GET | `/local/v1/saft/exports/{exportId}/download` | **authManager** | **`admin` / `owner`** |
| *写业务* | 见 §3.7 | **authSession** | `operator_id` 由会话注入；middleware 校验 §3.7.2 |

**401 错误码（会话）：** `session_revoked`、`operator_inactive`、`unauthorized`。

## 5. 验收清单

1. **冷启动：** 空库 → 登录页创建首个 **`admin`** + PIN → 登录成功。  
2. **名册：** **`admin`** 建 1 `owner` + 1 `cashier`；**`owner`** 仅能建 / 管 `cashier`；管理表权限符合 §3.3.2。  
3. **设置可见性：** **`owner`** 可见门店信息 / 店员 / 打印机 / SAF-T；**不可见** AT 凭证、系列、激活、备份；**`admin`** 全可见。  
4. **API 档位：** **`owner`** 调 `PUT at-credentials` / `activate-from-cloud` → **403**；调 `PUT taxpayer` / SAF-T → **200**。  
5. **权限：** **`cashier`** 登录 → 可开票 → **不可**见设置 → **不可**冲销（409）；**`admin` / `owner`** 为其开 `can_issue_nc` 后可冲销。  
6. **PIN：** **`admin` / `owner`** 重置 `cashier` PIN 成功且目标旧会话失效；「修改我的 PIN」须旧 PIN 且旧会话失效。  
7. **锁定：** 连续 5 次错误 PIN → 15 分钟内拒绝；审计有 `LOGIN_FAILED`。  
8. **会话：** 未登录 `POST /fiscal-documents` → 401；登录后签发 `source_id` = 会话 `operators.id`。  
9. **约束：** 禁止违反 §3.7.3（409）；禁止停用 **`admin`**；禁止 **`owner`** 改 **`admin` / `owner`** 行（403）。  
10. **迁移 A：** 跑 `007_operators_bootstrap_admin.sql` 后，每店最早 `owner` → **`admin`**，`session_epoch` bump，须重登。  
11. **停用吊销：** `active=0` + bump epoch → 401 + `forceLogout`。  
12. **回归：** `fiscal-m3-operators-regression.mjs` 扩展三档场景后全绿。  
13. **多端 / 端数 / 发票模式：** 仍按 §3.8、§3.8.1、§3.9（同步授权仅 **`admin`**）。

## 6. 参考

| 文档 | 用途 |
|------|------|
| [`fiscal-sqlite-schema.zh.md`](fiscal-sqlite-schema.zh.md) §6.5、§6.18 | `operators`、`audit_log` |
| [`fiscal-schema-worked-example-identity.zh.md`](fiscal-schema-worked-example-identity.zh.md) §4 | 本地创建示例 |
| [`fiscal-m3-nc.zh.md`](fiscal-m3-nc.zh.md) §16 | `can_issue_nc` 与冲销 UI |

## 7. 备选（P1，非 M3.2b 阻塞）

| 项 | 说明 |
|----|------|
| 登录按来源 IP 限速 | §3.7 原 P0 描述；实现后置 |
| `change-pin` 失败锁定 | 与登录锁定同策略 |
| 收紧 `GET /setup/status` 匿名字段 | 登录页与 owner 详情拆分 |
| 商品/客户写入是否仅 `owner` | 产品确认后单列 |
| 生产强制 `FISCAL_SESSION_SECRET` | 运维约束 |
| 审计日志查看 UI | 不在 M3.2b |

## 修订记录

| 日期 | 变更 |
|------|------|
| 2026-08-30 | 定稿：Agent 本地创建；废止 Farvoo 同步 |
| 2026-09-01 | 角色 UI 统一 `owner`/`cashier`；`can_issue_nc` 按账号 |
| 2026-09-01 | §3.9：**激活开票前**须 Ops 已配 `fiscal_profile`；`ready_to_issue` 增加校验 |
| 2026-09-02 | **M3.2c 定稿：** 三档 `admin`/`owner`/`cashier`；bootstrap-admin；设置分区权限；`authAdmin`/`authManager`；迁移 `007` 方案 A |
| 2026-09-01 | **M3.2b 定稿：** `session_epoch`、停用吊销、`forceLogout`、开票员管理 UI、`/operators/manage`；cashier 不可见设置；SAFT owner-only；§7 P1 备选 |
