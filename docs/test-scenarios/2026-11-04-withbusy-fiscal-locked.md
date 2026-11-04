# 本次改动业务场景测试报告：系列注册成功后按钮保持灰态

日期：2026-11-04  
分支：`fix/withbusy-respect-locked-buttons`  
仓库：`farvoo-fatura`  
方式：按方案改 → Admin HTML + `ui_toast_test` + mock Admin 真界面（临时 `127.0.0.1:17893`）

## 1. 本次修改

| 项 | 内容 |
|----|------|
| 终点形状 | 忙碌结束只恢复文案；最终能否点击服从状态门禁。系列已注册 → 输入只读 + 按钮灰；激活同步门禁同理。 |
| 权威依据 | RELEASE_NOTES 0.4.33 Admin §3「已注册输入只读并禁用按钮」；0.4.55 内嵌 SetupStatus 更新 UI |
| 唯一写法 | **`setFiscalBtnLocked`**：状态门禁按钮的 `data-fiscal-locked` + `disabled`；**`applySeriesForm`** / **`applySettingsStatusUI`（#btnActCloud）** 写锁；**`withBusy.finally`** 仅按 `data-fiscal-locked` 恢复 disabled |
| 文件 | `admin/index.html`、`ui_toast_test.go` |

**自检：** 无 `registerSeries` 内手写二次 `btn.disabled=true`；无成功后再 GET `/setup/status`；`withBusy` 不再无条件 `disabled=false`。

## 2. 核心业务规则

1. 【本次修复】注册系列成功后，按钮立即保持禁用，输入只读，pill/hint 已注册态不变。
2. 【已实现】同码再点：按钮不可点；后端仍幂等（防御）。
3. 【本次修复】云端激活成功 / 未满足同步前置时，`#btnActCloud` 不被 `withBusy` 冲成可点。
4. 【已实现】普通非门禁按钮：`withBusy` 结束后仍恢复可点（无 `data-fiscal-locked`）。

## 3. 详细测试场景

| ID | 场景 | 前置 | 期望 | 结果 |
|----|------|------|------|------|
| T1 | withBusy 不得无条件 re-enable | 源码钉死 | finally 认 `data-fiscal-locked` | 【已验证】pass（`TestAdminErrorToastSinglePath`） |
| T2 | 系列按钮经 setFiscalBtnLocked | 源码钉死 | lock/unlock 各一处 | 【已验证】pass |
| T3 | cloud 同步按钮经 setFiscalBtnLocked | 源码钉死 | `setFiscalBtnLocked(cloudBtn,` | 【已验证】pass |
| T4 | bootstrap 包全测 | — | pass | 【已验证】pass（`go test ./internal/fiscal/bootstrap/`） |
| T5 | Admin 注册 NC → 成功后立刻灰 | mock `17893` seed + AT 凭证已存 | `disabled`、`data-fiscal-locked=1`、输入只读、pill「已注册」 | 【已验证】pass（Chrome DevTools） |
| T6 | 刷新后已注册系列仍灰 | 同页 FT/FS/FR | 按钮 disabled | 【已验证】pass（快照：注册 FT/FS/FR 均为 disabled） |
| T7 | 激活同步成功后「从运营同步」灰 | seed `activated_ok` | `#btnActCloud` disabled | 【已验证】pass（快照；非本次点击路径，属门禁同源锁） |

## 4. mesa-local-product-test（本仓适配）

| 项 | 结果 |
|----|------|
| Mesa `:3000` | **N/A**【未验证】— 不经 Mesa；本改仅 fiscal-agent Admin embed |
| 替代验收 | `go test ./internal/fiscal/bootstrap/ -count=1` → **pass**；mock Admin 注册 NC 灰态 → **pass** |

## 5. 实际测试结果

| 项 | 结果 |
|----|------|
| `go test ./internal/fiscal/bootstrap/` | pass |
| Admin 真界面系列注册灰态（NC） | pass |
| commit / merge | **未做** — 待用户说 commit / merge |

## 6. 风险与待确认

1. 改了 `go:embed` Admin HTML → 须**重启** fiscal-local，浏览器 **Cmd+Shift+R**。
2. 其它按钮若误写 `data-fiscal-locked` 会永久灰；目前仅系列按钮与 `#btnActCloud` 走 `setFiscalBtnLocked`。
