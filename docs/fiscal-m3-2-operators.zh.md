# M3.2：开票员身份（Agent 本地创建）

> **状态：定稿后置**（方案已定；**暂不实施**，以免改动登录/回归；现网继续 demo 操作员 + PIN 占位）  
> **权威：是**（开票员名册、PIN 登录、角色与 `operators` 列口径；DDL 仍以 [`fiscal-sqlite-schema.zh.md`](fiscal-sqlite-schema.zh.md) + `migrations/001_init.sql` 为准）  
> **对应实现：** 待 M3.2 刀；当前仍为 `op-demo-cashier` + Admin PIN 占位  
> **计划：** [`fiscal-dev-plan.zh.md`](fiscal-dev-plan.zh.md) M3.2  
> **前置：** M2.6 Admin、M3.1 `can_issue_nc` enforce（0.4.26）

## 1. 目标

| 项 | 定法 |
|----|------|
| 人员名册 | **仅在 Agent Admin 创建/编辑/禁用**；**不同步** Farvoo 员工 API |
| 角色 | **两类**：`owner`（产品 UI：**管理员**）、`cashier`（产品 UI：**操作员**） |
| 登录 | Admin 进门校验 **PIN**（存 `operators.pin_hash`）；登录会话绑定 **`operators.id`** |
| 开票归属 | 签发 FT/NC 时 `operator_id` = 当前登录操作员；禁止再写死 demo id |
| 冲销权限 | `can_issue_nc`：`owner` 默认 1；`cashier` 默认 0；管理员在设置页按人开关 |

## 2. 非目标

| 项 | 说明 |
|----|------|
| Farvoo `fiscal-operators` 同步 | **P0 不做**；`mesa_user_id` 仅占位满足 DDL |
| Mesa / Farvoo 账号密码复用 | Agent 不存云端登录密码 |
| `operator_token` / LAN 终端鉴权 | 餐馆路径不做桌台直连接 API；列/方案保留为以后备选 |
| 第三档及以上 RBAC | 督导、frontdesk 等不再细分；一律映射为 `cashier` 或 `owner` |
| 云侧操作员副本 | 与 §1.1 一致：票库本地权威，人员名册亦仅本机 SQLite |

## 3. P0 定法

### 3.1 角色

| DB `role` | 产品 UI | 默认 `can_issue_nc` | 能力 |
|-----------|---------|---------------------|------|
| `owner` | 管理员 | 1 | 设置（门店/系列/激活）、管理操作员、开票、可冲销 |
| `cashier` | 操作员 | 0 | 开票、收银账单、发票查询/重打；冲销需管理员开权限 |

**P0：** 每店至少 **1 名 `owner`**（首次激活或引导流程创建）；可有多名 `cashier`。

### 3.2 创建与字段

| 列 | P0 定法 |
|----|---------|
| `id` | Agent 生成 UUID；= 签发 `source_id` |
| `mesa_user_id` | **占位**：`local-{id}`（满足 NOT NULL UNIQUE；**不**表示 Farvoo 用户） |
| `display_name` | 管理员手填（如「张三」「前台 1」） |
| `pin_hash` | 创建或改 PIN 时写入 argon2id；**未设则不可登录** |
| `synced_at` | 本地创建路径 **写 NULL**（列保留，不表示云同步） |
| `active` | 禁用 = 0；不可登录、不可作 `operator_id` |

**唯一写路径（增量）：**

| 操作 | 唯一入口 |
|------|----------|
| 创建/更新操作员 | `store.UpsertOperator` / `service.UpsertOperator`（扩展 PIN、禁用） |
| 改 PIN | `store.SetOperatorPIN`（或 Upsert 内原子更新，二选一实现，禁止 handler 直写 SQL） |
| 改 `can_issue_nc` | 已有 `store.SetOperatorCanIssueNC` |
| 登录校验 | `store.VerifyOperatorPIN` → session 存 `operator_id` |

### 3.3 Admin UI（设置 §5 重做）

| 区域 | 行为 |
|------|------|
| 操作员列表 | 展示 `display_name`、角色（管理员/操作员）、是否可冲销、是否已设 PIN |
| 添加 | 名称 + 角色 + 初始 PIN（4–6 位，D3.2 实现时拍板具体长度） |
| 编辑 | 改名称/角色/禁用；重置 PIN |
| 冲销开关 | 每行 checkbox → `can_issue_nc`（仅 `owner` 登录可见或可操作） |
| 登录页 | 选操作员（或输入工号/名称）+ PIN；校验 `pin_hash` |

**禁止：** 仍用全局 `OPERATOR_ID = 'op-demo-cashier'` 作生产默认（可保留 seed/UAT 专用）。

### 3.4 与 M3.1 checkbox 的关系

M3.1 单 checkbox 改 **`op-demo-cashier`** 为 **临时 demo**；M3.2 落地后 **移除** 该单用户 checkbox，改为 **列表按人管理**。

## 4. API（Local）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/local/v1/setup/operators` | 列表（不含 `pin_hash`） |
| PUT | `/local/v1/setup/operator` | 创建/更新（body：`id` 可选新建、`display_name`、`role`、`pin` 可选、`active`、`can_issue_nc`） |
| POST | `/local/v1/setup/login` | body：`operator_id` + `pin` → 会话 token 或 Set-Cookie（实现刀拍板一种） |

**P0：** Local API 仍绑定本机信任；登录主要约束 **Admin UI** 与 **`operator_id` 真实性**，不引入对外 LAN 鉴权。

## 5. 验收清单

1. 管理员在设置页创建 2 名操作员（1 owner + 1 cashier），各设 PIN。  
2. cashier 用 PIN 登录 → 可开票 → **不可**冲销（无权限且 API 409）。  
3. owner 登录 → 可开票 → 可为 cashier 开 `can_issue_nc` → cashier 再登录可冲销。  
4. 禁用操作员 → PIN 登录失败；以其 `id` 调 API 409/403。  
5. 签发的 FT `source_id` = 登录操作员 `operators.id`（非 demo 常量）。  
6. **无**对 Farvoo `fiscal-operators` 的调用（回归/单测可 grep 断言）。

## 6. 参考

| 文档 | 用途 |
|------|------|
| [`fiscal-sqlite-schema.zh.md`](fiscal-sqlite-schema.zh.md) §6.5 | `operators` 列 |
| [`fiscal-schema-worked-example-identity.zh.md`](fiscal-schema-worked-example-identity.zh.md) §4 | 本地创建示例 |
| [`fiscal-m3-nc.zh.md`](fiscal-m3-nc.zh.md) §16 | `can_issue_nc` 与冲销 UI |
