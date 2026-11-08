# 本次改动业务场景测试报告

日期：2026-11-08  
分支：`fix/admin-product-row-modal-close`（未合进 main）  
环境：mock `http://127.0.0.1:17883`，`at_env=mock`，临时库 `fiscal-row-uat-1794146374.db`。未动正在听的 `17881`。

## 1. 本次修改

- **内容：** 商品列表点一行打开现有商品工作台。本机商品可编辑；收银同步（`REMOTE_SYNC`）同一窗口只读。去掉商品行上只含「编辑」的「⋯」。9 个 Admin 弹窗标题栏右上角同一颗关闭钮；点它、点遮罩、按 Esc（只关最上层）都走该弹窗原来的关闭函数。底栏「保存 / 关闭 / 取消 / 返回」仍在。开票页里的「本单合计」「发票信息」不加关闭钮。
- **文件：** `apps/fiscal-agent/internal/fiscal/bootstrap/admin/index.html`、`ui/admin-i18n.js`、`ui_toast_test.go`
- **终点形状：** 商品只有一个工作台弹窗；能不能改由 `source` 决定。弹窗只有一条关闭路径 `closeModalById`。
- **权威依据：** `docs/fiscal-sqlite-schema.zh.md` §6.8（`REMOTE_SYNC` 只读、`LOCAL` 可改）；`docs/fiscal-admin-ui-prototype/README.md`（唯一商品工作台）；`docs/test-scenarios/2026-11-03-order-invoice-sheet-foot.md`（壳仍是 `sheet-head` / `sheet-body` / `sheet-foot`）。纯本地界面，不报 AT。作废原因输入与 ≤50 字校验未改。
- **唯一写法：** `openProductWorkbench` 仍是打开商品弹窗的唯一函数；列表进入只走 `openProductFromList`。`setProductFieldsLocked` 只在这一处按 `source === 'REMOTE_SYNC'` 锁字段。`closeModalById` / `topOpenModalBackdrop` 各一处。`data-sheet-close` 的按钮 9 个，每个弹窗一颗。自检：`go test ./internal/fiscal/bootstrap/` 通过，上述符号计数由 `TestAdminHTMLProductWorkbenchUnique` 钉住。

## 2. 核心业务规则

- 本机商品可改可存。【已实现】
- 收银同步商品只能看，保存按钮隐藏，字段不可改。服务端拒绝改 `REMOTE_SYNC` 的原逻辑未动。【已实现】
- 关闭弹窗不提交、不保存。【已实现】
- 纠正发票确认层在表单上面时，关闭只关掉确认层（`closeAdjustConfirmModal`），不签发。【已实现】代码路径；本轮未在界面叠两层点过，见场景 6。
- 发票、账单点行进详情，以及它们的「⋯」，未改。【已实现】

## 3. 详细测试场景

| ID | 前置 | 操作 | 预期 | 结果 |
|----|------|------|------|------|
| UI-01 | mock 已登录，商品页有本机商品 TEA-ROW | 点该行 | 弹出「编辑商品」，字段可改，有保存，右上角有关闭 | 【已验证】pass |
| UI-02 | 编辑弹窗已开，名称改成别的字但未保存 | 点右上角关闭，再点开同一行 | 名称仍是原来的「行点测试」 | 【已验证】pass |
| UI-03 | 编辑弹窗已开 | 按 Esc；再打开后点遮罩 | 两次都关掉 | 【已验证】pass |
| UI-04 | 底栏「关闭」 | 点底栏关闭 | 弹窗关掉 | 【已验证】pass |
| UI-05 | 库里插入 `REMOTE_SYNC` 商品 POS-TEA 后刷新 | 点该行 | 标题「商品详情」，提示只能查看，五个字段禁用，无保存 | 【已验证】pass |
| UI-06 | 刚看完只读商品 | 再点本机商品，再点「新建商品」 | 字段恢复可编辑，新建是空表 | 【已验证】pass |
| UI-07 | 已登录 | 打开「修改 PIN」 | 右上角有关闭，点它关掉 | 【已验证】pass |
| UI-08 | 页面 DOM | 数 `.modal` 与关闭钮；看开票「本单合计」 | 9 个弹窗都有关闭钮；页内合计栏没有 | 【已验证】pass |
| UI-09 | 商品列表 | 看行尾 | 没有「⋯」 | 【已验证】pass |
| UI-10 | 纠正发票：表单层 + 确认层同时开着 | 点确认层关闭 | 只关确认层，不签发 | 【未验证】未开票，没叠出这两层。关闭函数仍是原来的 `closeAdjustConfirmModal` |

## 4. mesa-local-product-test

N/A ·【未验证】。这次只改 fiscal-agent Admin 嵌入页，不经 Mesa `:3000`。

替代验收：`go test ./internal/fiscal/bootstrap/` pass。未跑 `scripts/fiscal-*-regression.mjs`（那些脚本会 `pkill` 掉本机所有 `fiscal-local`，包括正在听的 `17881`）。商品写入接口没改。

## 5. 实际测试结果

| 项 | 结果 |
|----|------|
| bootstrap 单测 | pass |
| 商品行点击、只读、关闭钮、Esc、遮罩、底栏关闭 | pass |
| 纠正发票双层弹窗的关闭 | 【未验证】 |
| 17881 上的现有界面 | 未重启，仍是旧页面 |

## 6. 风险与待确认

- 未保存的内容，点关闭、遮罩或 Esc 会丢掉，和以前点底栏「关闭」一样。
- 你现在开着的 `127.0.0.1:17881` 要切到这个分支并重启后才能看到新界面。
- 未 commit，未合进 main。
