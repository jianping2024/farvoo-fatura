# 本次改动业务场景测试报告：bootstrap 须 Ops 已存 fiscal_profile（方案 A）

日期：2026-11-04  
分支：`feat/bootstrap-ops-fiscal-profile-gate`  
仓库：`farvoo-fatura`  
方式：按方案改 → 单测 + `fiscal-m3-operators-regression.mjs`；真界面配对/Ops 开通路径标【未验证】（待用户测）

## 1. 本次修改

| 项 | 内容 |
|----|------|
| 终点形状 | Ops 未保存 `fiscal_profile`（餐馆/商超）时，用户端不得创建首位管理员；UI 与 API 同一门禁。签名「激活开票」仍在 bootstrap 之后。 |
| 权威依据 | `docs/fiscal-m3-2-operators.zh.md` §3.5（本轮改定稿）、§3.9；`fiscal-dev-plan` §9 打票可选；对照 Mesa Ops 总闸习惯（方案 A 复用业态，不新造布尔总闸） |
| 唯一写法 | **开通判定**仅 `service.OpsFiscalEntitled`；**`bootstrap_required`** 仅 `BuildSetupStatusPublic`；**创建**仅 `BootstrapOwner`→同闸；登录页仅认 `bootstrap_required`，空名册等待仅 `setLoginWaitOpsMode` |
| 文件 | `operators_policy.go`、`setup_status_public.go`、`routes.go`、`auth_handlers.go`、Admin `index.html` + `admin-i18n.js`、§3.5 / 串讲时间线、相关测试 |

**自检：** 无平行第二套开通 if；未用 `ready_to_issue` / signing `active` 挡 bootstrap；未只藏按钮。

## 2. 核心业务规则

1. 【本次新增】空库可 bootstrap ⟺ 真 greenfield ∧ `OpsFiscalEntitled`（本地缓存或 live store-policy 的 `restaurant`\|`retail`，或 `FISCAL_ALLOW_LOCAL_PROVISION=1`）。
2. 【本次新增】未配对 → `cloud_not_paired`；已配对未设业态 → `ops_fiscal_not_entitled`；匿名 status 带 `cloud_paired_ok` / `ops_fiscal_entitled`。
3. 【已实现】仍仅 loopback 可调 bootstrap；他店已有 operators 仍禁。
4. 【已实现】签名激活 / `ops_activate_pending` 路径不变，仍在 admin 之后。

## 3. 详细测试场景

| ID | 场景 | 前置 | 期望 | 结果 |
|----|------|------|------|------|
| T1 | 未配对空库 status | `LOCAL_PROVISION=0` | `bootstrap_required=false`，`ops_fiscal_entitled=false` | 【已验证】pass（`TestSetupStatusPublic_Anonymous`） |
| T2 | UAT 本地旁路 | `LOCAL_PROVISION=1` | `bootstrap_required=true`，可建 admin | 【已验证】pass（status + service 单测） |
| T3 | 未配对调 BootstrapOwner | 同上 | 403 `cloud_not_paired` | 【已验证】pass |
| T4 | 他店已有 operators | entitled=true | 本店仍不可 bootstrap | 【已验证】pass |
| T5 | entitled=false 空库 | BuildSetupStatusPublic | `bootstrap_required=false` | 【已验证】pass |
| T6 | m3 operators 回归 | mock + LOCAL_PROVISION=1 | 28/28 含 bootstrap | 【已验证】pass |
| T7 | 真配对 + Ops 未存业态 → 登录页等待文案 | 真云 | 无创建表单；提示等待 Ops | 【未验证】需真 Ops/配对 |
| T8 | Ops 保存业态后可创建管理员 | 真云 | 出现创建表单；API 成功 | 【未验证】需真 Ops/配对 |
| T9 | 仅藏 UI、直打 API | 未开通 | API 仍 403 | 【已验证】pass（service/API 同闸；真机再扫一次更好） |

## 4. mesa-local-product-test（本仓适配）

| 项 | 结果 |
|----|------|
| Mesa `:3000` | **N/A**【未验证】— 不经 Mesa；本改仅 fiscal-agent |
| 替代验收 | `go test` api/service/bootstrap + 根包 embed；`node scripts/fiscal-m3-operators-regression.mjs` → **28/28 pass** |

## 5. 实际测试结果

| 项 | 结果 |
|----|------|
| `go test ./internal/fiscal/api|service|bootstrap` | pass |
| `go test .`（embed / bootstrap 门钉） | pass |
| `fiscal-m3-operators-regression.mjs` | 28/28 pass |
| 真界面 Ops 开通前后 | 【未验证】 |
| commit / merge | **未做** — 待用户说 commit / merge |

## 6. 风险与待确认

1. 生产冷启动顺序变为：**配对 → Ops 保存业态 → 建管理员**；旧习惯「先建管理员再等 Ops」会看到等待页。
2. live `PullStorePolicy` 失败时软判未开通（不 500）——网络抖时可能短暂看不到创建表单，刷新即可。
3. 在 taxpayer 行不存在时无法缓存 `fiscal_profile`（`SaveOpsFiscalProfile` best-effort）——开通判定仍以 live 响应为准，OK。
4. Mesa 云店「打印发票」是否跟同一开通信号：本任务未改 restaurant-ordering。
