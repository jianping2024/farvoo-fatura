# P1-S — Admin 安全加固（登录与会话）

> **状态：定稿**（P1-S 已实施）  
> **权威：否**（拍板后为本里程碑交付口径；会话/RBAC 基线仍以 [`fiscal-m3-2-operators.zh.md`](fiscal-m3-2-operators.zh.md) 为准）  
> **对应实现：** `apps/fiscal-agent`（`store/login_security.go`、`api/session_production.go`、`api/setup_status_public.go`）  
> **计划：** [`fiscal-dev-plan.zh.md`](fiscal-dev-plan.zh.md) §P1 待选型  
> **关联：** [`fiscal-m3-2-operators.zh.md`](fiscal-m3-2-operators.zh.md) §3.6、§3.7、§7

---

## 1. 目标与非目标

### 1.1 目标

在 **M3.2 已有 PIN + 会话 + 三档 RBAC** 之上，补齐店内 **局域网多 PC 开票** 场景下的撞库与会话伪造风险；**不改变** AT 票证签发规则。

### 1.2 非目标

| 项 | 说明 |
|----|------|
| AT 认证新检查项 | 本里程碑 **不是** 认证清单条目；不写入 `fiscal-certification-checklist.zh.md` |
| 操作记录 UI | 见 [`fiscal-audit-log-ui.zh.md`](fiscal-audit-log-ui.zh.md)（独立 P1 项） |
| 开票写 audit | `ISSUE` / `NC` / `ND` 记入 `audit_log` |
| 云端 WAF / 门店防火墙 | 店网边界不在 Agent 范围 |
| WebAuthn / 硬件密钥 | P0 不做 |
| 改 PIN 策略（长度、复杂度） | 仍 6 位数字；见 §4 备选 |

---

## 2. P0 定法（本里程碑必须遵守）

### 2.1 登录按来源 IP 限速

| 项 | 定法 |
|----|------|
| 与现有关系 | **保留** 按 `operator_id`：连续 **5** 次错 PIN → 该开票员锁 **15** 分钟（§3.6） |
| 新增维度 | 按 **客户端 IP**（`RemoteAddr` 去端口；`X-Forwarded-For` **不信任**） |
| 阈值 | 同一 IP **15 分钟**内累计 **30** 次 `login_failed`（任意 `operator_id`）→ 该 IP 拒绝登录 **15 分钟** |
| 响应 | HTTP **429**；`error` = `ip_rate_limited`；文案不暴露剩余次数 |
| 存储 | **不** 新 DDL；用 `audit_log`：`action=LOGIN_FAILED`，`entity_type=client_ip`，`entity_id` = IP 字符串 |
| 清理 | 与现有 `LOGIN_FAILED` 清理同策略：按 `entity_id` 删 **15 分钟**前行（登录成功 **不** 清 IP 计数，仅时间窗滑动） |
| 适用范围 | `POST /local/v1/setup/login`；**不** 对 `bootstrap-owner` 加 IP 限速（仍仅 loopback） |
| 唯一实现 | `store.CheckLoginIPRateLimit` + `store.RecordLoginFailureIP`（或并入现有 `InsertLoginFailureAudit` 扩展） |

### 2.2 生产强制 `FISCAL_SESSION_SECRET`

| 项 | 定法 |
|----|------|
| 「生产」判定 | `FISCAL_ALLOW_DEV_KEY` **未**设置为 `1`（与 dev 门铃、DEV_PLAIN 钥门禁一致） |
| 强制规则 | 生产启动时 `FISCAL_SESSION_SECRET` 须非空且长度 **≥ 32** 字节（UTF-8 字符串） |
| 失败行为 | Fiscal HTTP 注册前 **Fatal 日志 + 进程退出**（或拒绝挂 Fiscal 路由并托盘提示「配置错误」——**P0 定法：进程退出**，避免半开 Admin） |
| 开发/UAT | `FISCAL_ALLOW_DEV_KEY=1` 时允许沿用 `dataDir` 派生密钥（现有 `NewSessionManager` 行为） |
| 运维文档 | `apps/fiscal-agent/README.md` + 安装向导补充：生产店机须设用户级或系统 env |
| 唯一读路径 | 仍仅 `session.go` `NewSessionManager`；禁止第二处读 env |

### 2.3 匿名 `GET /setup/status` 收紧（最小集）

| 项 | 定法 |
|----|------|
| 问题 | 登录页匿名可读字段过多，便于枚举门店配置 |
| P0 定法 | 匿名 `GET /local/v1/setup/status` **移除** 以下字段：`at_env`、`software_certificate_number`、`signing_key_version`、`cloud_jwt_hint`（若存在） |
| 保留 | `bootstrap_required`、`operators_count`、`fiscal_profile`、`store_display_name`（店名用于登录页展示）、`ready_to_issue` 布尔 **不** 暴露系列明细 |
| 已登录 | 会话下 `GET /setup/status` 行为 **不变**（各角色仍按 M3.2c 可见子集） |
| 唯一出口 | 现有 status handler 分匿名/会话两套 JSON builder |

---

## 3. 交付物

| # | 交付物 | 定义「完成」 |
|---|--------|----------------|
| D-S.1 | **本文定稿** | 状态头改为定稿；与 `fiscal-dev-plan.zh.md` 里程碑表一致 |
| D-S.2 | **Store** | IP 限速查询/记录；`LOGIN_FAILED` IP 行写入 |
| D-S.3 | **Login handler** | 先 IP 限速 → 再 operator 锁定 → 再验 PIN；429/401 稳定 |
| D-S.4 | **Session secret 门禁** | 生产无 secret 启动失败；UAT 仍可用派生密钥 |
| D-S.5 | **Setup status 拆分** | 匿名响应字段按 §2.3；单测断言字段缺失 |
| D-S.6 | **回归** | `scripts/fiscal-p1-security-regression.mjs` |
| D-S.7 | **文档** | README 生产 env 表；`fiscal-config-boundary.zh.md` 补 `FISCAL_SESSION_SECRET` |

---

## 4. 验收清单

1. 同一 IP 对 3 个不同开票员各错 PIN 10 次 → 第 31 次起 **429** `ip_rate_limited`（15 分钟内）。  
2. 15 分钟后同 IP 可再试；单 `operator_id` 仍受 5 次锁定。  
3. `FISCAL_ALLOW_DEV_KEY` 未设且无 `FISCAL_SESSION_SECRET` → Agent **不**提供可登录 Admin。  
4. `FISCAL_ALLOW_DEV_KEY=1` 且无 secret → 行为与现网 UAT 一致。  
5. 匿名 `GET /setup/status` 无 `at_env` 等敏感字段；已登录 admin 仍可见完整 status。  
6. `go test ./internal/fiscal/...` + `fiscal-p1-security-regression.mjs` 全绿。  
7. **不**回归：`fiscal-m3-operators-regression.mjs`、`fiscal-d62-cert-regression.mjs`。

---

## 5. 依赖与顺序

| 依赖 | 说明 |
|------|------|
| M3.2 | 会话、PIN、`LOGIN_FAILED` 写路径 |
| 建议先于 | 店内 `FISCAL_ALLOW_LAN=1` 大规模 rollout |

---

## 6. 备选（P1+，非本里程碑阻塞）

| 项 | 说明 |
|----|------|
| `change-pin` 失败锁定 | 与登录同 IP + operator 双维 |
| `client_ip` 列 | migration + 操作记录 UI 展示来源 IP |
| Cookie `Secure` | 仅 HTTPS 终止于 Agent 时启用 |
| 会话固定防护 | 登录成功后轮换 session id |

---

## 7. 修订记录

| 日期 | 变更 |
|------|------|
| 2026-09-02 | 草稿：IP 限速、SESSION_SECRET、匿名 status 三刀 + 交付物/验收 |
