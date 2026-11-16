# 本次改动业务场景测试报告：Admin 壳滚动 A + 开钱箱仅发票页

日期：2026-11-10  
分支：`fix/admin-shell-scroll-a-drawer-invoices-only`

## 1. 本次修改

- **内容**：
  1. Admin 桌面壳按方案 **A**：`#app-shell` 锁视口高，**只有** `#mainScroll` 滚内容；左侧菜单与脚栏钉住。
  2. 「开钱箱」浮钮仍只有 `#mainChrome` / `#btnOpenCashDrawer` 一处；由 `syncMainChrome` 控制，**仅发票 hub（`invoices`）显示**。
- **文件**：
  - `apps/fiscal-agent/internal/fiscal/bootstrap/admin/index.html`
  - `apps/fiscal-agent/internal/fiscal/bootstrap/ui_toast_test.go`
  - `apps/fiscal-agent/internal/fiscal/bootstrap/ui_cash_drawer_test.go`
- **终点形状**：锁定壳 + 右列全宽 scrollport（滚动条在窗口右缘）；钱箱 dock 唯一节点，显隐唯一写入口 `syncMainChrome`（`showView` 调用）。窄屏 ≤860px 解锁为自然堆叠滚动。
- **权威依据**：本会话已确认方案 A；钱箱「只在发票界面」为同确认附加需求。纯 UI；不适用 fiscal schema / SAF-T / AT。
- **唯一写法自检**：
  - 内容滚动：`#mainScroll` / `.main-scroll`（一份）
  - 壳锁定：`#app-shell` `height:100%; overflow:hidden`
  - 钱箱显隐：`function syncMainChrome` + `syncMainChrome(name);` 仅自 `showView`
  - 开箱动作仍：`openCashDrawer` / `#btnOpenCashDrawer` 各一份
  - diff 无 sticky+`min-height:100vh` 旧侧栏；无按页 CSS 藏钮平行补丁

## 2. 核心业务规则

- 【已实现】桌面宽：滚设置长页时侧栏顶与「退出」不离开视口。
- 【已实现】切页时 `#mainScroll.scrollTop` 归零。
- 【已实现】`invoices` 显示开钱箱；`products` / `customers` / `settings` / 开票流等隐藏。
- 【已实现】设置里「测试开箱」仍可用（配置路径，不是浮钮）。

## 3. 详细测试场景

| ID | 场景 | 前置 | 操作 | 预期 | 结果 |
|----|------|------|------|------|------|
| S1 | 发票页见开钱箱 | mock `:17884` 登录 | 落在发票 hub | `#mainChrome.hidden=false`，按钮「开钱箱」 | 【已验证】pass |
| S2 | 设置页无开钱箱 | 同上 | 点设置 | `hidden=true` | 【已验证】pass |
| S3 | 商品/客户无开钱箱 | 同上 | 切商品、客户 | 均为 hidden | 【已验证】pass |
| S4 | 设置长内容侧栏钉住 | 设置页拉长内容 | `#mainScroll` 滚到底 | 侧栏 top/脚栏 bottom 不变 | 【已验证】pass |
| S5 | 壳为锁定 scrollport | 桌面 1280×800 | 读 computed style | shell `800px`+`overflow:hidden`；mainScroll `overflow-y:auto` | 【已验证】pass |
| S6 | 单测钉唯一写法 | — | `go test` Admin/CashDrawer/Shell | pass | 【已验证】pass |
| S7 | ≤860 窄屏堆叠 | 窄窗 | 整页可滚 | 侧栏不占死 100vh 压扁内容 | 【未验证】未做窄屏模拟 |

## 4. mesa-local-product-test

N/A ·【未验证】— 不经 Mesa `:3000`。  
**替代验收**：mock Admin `http://127.0.0.1:17884` + `go test ./internal/fiscal/bootstrap/ -run 'TestAdminCashDrawer|TestAdminHTMLNoPageXScroll|TestAdminHTMLShellContentScroll|TestAdmin'`。

## 5. 实际测试结果

| 项 | 结果 |
|----|------|
| 相关 go test（TestAdmin* 等） | pass |
| mock 浏览器 UAT（S1–S6） | pass |
| 窄屏 S7 | 未验证 |
| 用户本机 `:17881` 旧进程 | 未吃到本分支 embed；需用本分支重编/重启后才可见 |

## 6. 风险与待确认

- 「发票界面」定法为 **仅 `invoices` hub**；开票四步流（`order-new` / `order-invoice`）不显示浮钮。若要在开票流也显示，改 `syncMainChrome` 一处即可。
- 用户当前 `:17881` 若仍是旧二进制，刷新看不到改动。
- 待用户说 **commit** / **merge**；本报告完成时停在 feature 分支。
