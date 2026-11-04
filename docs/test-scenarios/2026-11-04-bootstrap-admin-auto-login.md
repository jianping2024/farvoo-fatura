# 本次改动业务场景测试报告：冷启动「创建管理员并登录」自动进发票页

日期：2026-11-04  
分支：`fix/bootstrap-admin-auto-login`  
仓库：`farvoo-fatura`  
本机 UAT：`http://127.0.0.1:17883/admin`（`FISCAL_AT_ENV=mock`，空库 `FISCAL_SEED=0`，临时 `fiscal-bootstrap-uat-*.db`）  
方式：A0 确认方案 → 按方案改 → 钉死测 + 真界面自测

## 1. 本次修改

| 项 | 内容 |
|----|------|
| 终点形状 | 点「创建管理员并登录」且 API 成功后：清登录 Public 缓存 → 重载开票员 → 用刚设 PIN 调 `POST /login` → `enterApp` 落在发票主界面 |
| 权威依据 | `docs/fiscal-m3-2-operators.zh.md` §3.5（创建后立即要求 login） |
| 唯一写法 | 仅 `bootstrapOwnerAndLogin`（不平行第二套 bootstrap/登录流） |

| 文件 | 改动 |
|------|------|
| `apps/fiscal-agent/internal/fiscal/bootstrap/admin/index.html` | 形参改名 `pinInput`（不再遮蔽外层 `pin`）；创建成功后 `loginSetupPublicCache = null` |
| `apps/fiscal-agent/internal/fiscal/bootstrap/ui_toast_test.go` | `TestAdminBootstrapOwnerAndLoginAutoLogin` 钉死上述两点 |

## 2. 核心业务规则

1. 冷启动入口只在登录页；只认 `setup/status.bootstrap_required`。
2. `POST /bootstrap-owner` 成功后必须立即 `POST /login` 并进入 App（发票落地页）。
3. 登录页 Public 缓存不得在创建成功后仍假装 `bootstrap_required=true`。

## 3. 详细测试场景

| ID | 场景 | 前置 | 操作 | 期望 | 结果 |
|----|------|------|------|------|------|
| A1 | 钉死：唯一路径 + 清 cache + 不遮蔽 pin | — | `go test …TestAdminBootstrapOwnerAndLoginAutoLogin` | pass | 【已验证】pass |
| A2 | Public/Full 缓存分裂回归 | — | `TestAdminSetupStatusCacheSplit` | pass | 【已验证】pass |
| A3 | 后端 bootstrap 空库/错店/二次创建 | — | store/api `TestBootstrapOwner*` / `TestSetupStatusPublic*` | pass | 【已验证】pass |
| U1 | 空库真界面创建并登录 | mock Agent 空库 17883 | 填名+PIN → 点「创建管理员并登录」 | 直接进发票主界面，侧栏显示管理员名 | 【已验证】pass（侧栏「测试管理员」、发票空列表） |
| U2 | 退出后再登录 | U1 后 | 点退出 → 应见开票员+PIN，无创建表单 | 正常登录 UI | 【未验证】（本轮未点退出） |
| U3 | 非 loopback / 已有人再创建 | — | API 403 文案 | reportError | 【未验证】（本轮未覆盖） |

## 4. 产品验收（mesa-local-product-test · 本仓适配）

| 项 | 结果 |
|----|------|
| Mesa `:3000` | N/A ·【未验证】（不经 Mesa；纯 fiscal-agent Admin embed） |
| 替代验收 | `go test ./internal/fiscal/bootstrap/ -run TestAdmin`；`store`/`api` BootstrapOwner；真界面 U1 |

## 5. 实际测试结果

| 项 | 结果 |
|----|------|
| 创建后自动进发票页 | pass（U1） |
| 钉死测防回归 | pass（A1） |
| commit / merge | **未做**（等用户说 commit / merge） |

## 6. 风险与待确认

- 若本机库里**已经**建过管理员（上次半成功），再点创建会 `bootstrap_not_empty`；应改用正常 PIN 登录，或换空库测。
- U2（退出后是否回到正常登录）未在本轮点验；逻辑上清 cache + `loadLoginOperators` 应正确，建议合入前随手点一次。
