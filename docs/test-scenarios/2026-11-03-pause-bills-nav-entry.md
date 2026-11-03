# 本次改动业务场景测试报告：暂停账单同步 → 去掉 Admin「账单」入口

日期：2026-11-03  
分支：`ui/pause-bills-nav-entry`  
仓库：`farvoo-fatura`  
方式：按已确认最佳实践终点落地（账单同步暂时废弃，店员侧不暴露账单入口）。

## 1. 本次修改

- **内容：** 去掉餐馆侧栏「账单」、发票 hub「账单」CTA/角标；静默店员侧 SSE（不订阅 `bill_drafts_changed`、无新账单 toast）；发票 hub 焦点仅「＋ 开票」；空态文案去掉「有待开账单」；UI 定稿与原型对齐临时暂停口径。后端 `bill_sync_drafts` / 分单视图代码暂留。
- **文件：** `admin/index.html`、`ui/admin-i18n.js`、`ui_toast_test.go`、`docs/fiscal-admin-ui-prototype/README.md`、`docs/fiscal-admin-ui-prototype/index.html`、方案门规则（最佳实践标记）、本报告。
- **终点形状：** 同步暂停期间餐馆开票入口与商超一致，唯一可见为「＋ 开票」；无账单菜单/CTA/角标/toast。
- **权威依据：** `docs/fiscal-admin-ui-prototype/README.md`（2026-11-03 临时定法）；后端保留见同文「账单实时提示 · 临时暂停」。
- **唯一写法：**
  - 入口：无 `#navBills` / `#ctaPendingBills`（测试钉死禁止残留）
  - 焦点：`focusHomePrimaryCta` 只聚焦 `#ctaNewOrder`
  - SSE：`startBillDraftEvents` 仅 `stopBillDraftEvents`（无 EventSource）
  - hub 加载：`loadViewData('invoices')` 不再 `refreshBills`

## 2. 核心业务规则

1. 【本次新增】账单同步暂停 → Admin 不暴露「账单」导航与 hub CTA。
2. 【本次新增】店员侧不因账单草稿 SSE 角标/toast。
3. 【已实现】手建「＋ 开票」四步流仍为开票入口。
4. 【已实现】后端 bill-draft / ingest API **未拆除**（待恢复同步）。

## 3. 详细测试场景

### 场景 1：无账单入口 DOM
- **期望：** HTML 无 `navBills` / `ctaPendingBills` / `data-nav="bills"`。
- **结果：【已验证】pass** — `TestAdminHTMLSchemeACopyUnique` / hub layout / home CTA 测试。

### 场景 2：焦点与 SSE 静默
- **期望：** `focusHomePrimaryCta` 不优先账单；无 EventSource / `bill_drafts_changed`。
- **结果：【已验证】pass** — `TestAdminHTMLHomeCtaPeerAndFocus`、`TestAdminHTMLBillDraftEventsUnique`。

### 场景 3：bootstrap 包单测
- **操作：** `GOMODCACHE=$HOME/go/pkg/mod go test ./internal/fiscal/bootstrap/ -count=1`
- **结果：【已验证】pass**

### 场景 4：餐馆真界面无「账单」菜单、仅「＋ 开票」
- **结果：【未验证】** — 需重启 Agent + Cmd+Shift+R 后目视侧栏与 hub。

### 场景 5：＋开票四步流仍可用
- **结果：【未验证】** — 真界面手测。

## 4. mesa-local-product-test

| 项 | 结果 |
|----|------|
| mesa-local-product-test | N/A · 【未验证】（不经 Mesa `:3000`） |
| 替代验收 | bootstrap `go test` pass |

## 5. 实际测试结果

| 项 | 结果 |
|----|------|
| 唯一写法 / 入口钉死 | pass |
| bootstrap 全量 | pass |
| 真界面 | 【未验证】 |

## 6. 风险与待确认

- 嵌入 Admin 须重启 Agent 后生效。
- 若某店仍有 open `bill_sync_drafts`，Admin 无法消账（与「同步已废弃」前提一致）；清理另立任务。
- 待用户说 **commit / merge** 后再合入 local `main`。
