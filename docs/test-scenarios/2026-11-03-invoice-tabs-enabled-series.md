# 本次改动业务场景测试报告：发票列表 Tab / 开票下拉仅展示已注册 ACTIVE 系列票种

日期：2026-11-03  
分支：`fix/invoice-tabs-enabled-series-only`  
仓库：`farvoo-fatura`  
方式：按已确认最佳实践终点落地。

## 1. 本次修改

- **内容：** 发票列表票种 Tab 与手工开票下拉改为只展示本店已注册 ACTIVE 系列的类型（`setup.status` 的 `*_series_ok`）；「全部」始终保留；非法 session 票种回落「全部」。
- **文件：** `apps/fiscal-agent/internal/fiscal/bootstrap/admin/index.html`、`ui_toast_test.go`、`ui_list_pagination_test.go`、`docs/fiscal-admin-ui-prototype/README.md`、本报告。
- **终点形状：** Admin「本店启用票种」= ACTIVE 系列已注册；列表 Tab = 全部 + 这些类型；手工开票下拉同口径（`MANUAL_DOC_TYPES` ∩ `*_series_ok`）。
- **权威依据：** `docs/fiscal-admin-ui-prototype/README.md`（2026-11-03 P0）；各可选票种文「无系列不挡 `ready_to_issue`」与「无系列则不展示」一致；真相源 `GET /setup/status` 的 `*_series_ok`。
- **唯一写法（自检）：**
  - `docTypeHasActiveSeries` — 唯一 `*_series_ok` 判定
  - `enabledInvoiceListDocTypes` — 唯一列表 Tab 集合
  - `enabledManualDocTypes` / `enabledBillSyncDocTypes` — 唯一开票下拉集合
  - `renderInvoiceTypeTabs` — 唯一 Tab DOM 渲染
  - `applySettingsStatusUI` 在 setup 刷新后调用上述渲染 + `refreshLocalizedSelects`

## 2. 核心业务规则

1. 【本次新增】列表 Tab 可见性 = `*_series_ok`，不绑 `ready_to_issue` / `fiscal_profile` / `activated_ok`。
2. 【本次新增】手工 / 分单票种下拉只列已注册 ACTIVE 系列的类型。
3. 【本次新增】session 记住的票种若已不可见 → 回落「全部」并刷新列表。
4. 【已实现】「全部」仍可看到任意历史单据（含已退系列类型）。
5. 【已实现】NC / ND / RG 仍从单据动作开具，不进手工下拉（与改前一致）。

## 3. 详细测试场景

### 场景 1：静态 HTML 不再写死全票种 Tab
- **期望：** 除「全部」外无硬编码 `data-invoice-type="FT"` 等；存在唯一 `renderInvoiceTypeTabs` / `enabledInvoiceListDocTypes`。
- **结果：【已验证】pass** — `TestAdminHTMLInvoiceHubLayout`、`TestAdminHTMLInvoiceListPaginationUnique`。

### 场景 2：bootstrap 包单测
- **操作：** `GOMODCACHE=$HOME/go/pkg/mod go test ./internal/fiscal/bootstrap/ -count=1`
- **结果：【已验证】pass**

### 场景 3：test DB 仅 FT/FS/GR/GT ACTIVE → Tab / 下拉仅这些 + 全部
- **前置：** `fiscal-test.db` ACTIVE = FT, FS, GR, GT；本分支重启 `:17881`。
- **期望：** Tab = 全部 / 完整发票 / 简式发票 / 发货单 / 运输单；无 FR / NC / ND / RG / PF；`#docType` 选项 = FT, FS, GR, GT。
- **结果：【已验证】pass** — Chrome DevTools：`tabTypes=["","FT","FS","GR","GT"]`，`docOpts=["FT","FS","GR","GT"]`。

### 场景 4：注册新系列后 Tab 出现
- **期望：** 设置里注册 FR 成功 → 列表出现「发票收据」；开票下拉出现 FR。
- **结果：【未验证】** — 需互动（会调 AT 注册，先问用户）。

### 场景 5：非法 session 票种回落
- **期望：** session 存 `ND` 且无 ND 系列 → 打开列表为「全部」。
- **结果：【未验证】** — 真界面手测。

## 4. mesa-local-product-test

| 项 | 结果 |
|----|------|
| mesa-local-product-test | N/A · 【未验证】（不经 Mesa `:3000`） |
| 替代验收 | bootstrap `go test` pass；`:17881` 真界面 Tab/下拉裁剪 pass |

## 5. 实际测试结果

| 项 | 结果 |
|----|------|
| 唯一写法钉死 | pass |
| bootstrap 全量 | pass |
| 真界面 Tab + 开票下拉裁剪 | pass |

## 6. 风险与待确认

- 嵌入 Admin 须重启 Agent 后生效（本轮已重启 `:17881`；请 Cmd+Shift+R）。
- 已退系列的历史票无独立 Tab，靠「全部」+ 搜索。
- 待用户说 **commit / merge** 后再合入 local `main`。
