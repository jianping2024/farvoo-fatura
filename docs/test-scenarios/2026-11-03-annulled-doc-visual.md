# 本次改动业务场景测试报告：作废单据列表/详情视觉区分

日期：2026-11-03  
分支：`ui/annulled-doc-visual`  
仓库：`farvoo-fatura`  
方式：收紧版终点形状落地 Admin + 静态稿对齐。

## 1. 本次修改

- **内容：** 列表对 `document_status=A` 行加 `is-annulled` +「已作废」pill；对 `F` 仅中性「已开票」pill；`N` 不加标。详情状态本地化（含 N/A/F），作废时展示 `status_reason`。无票号删线、无标题染色。
- **文件：** `admin/index.html`、`ui/admin-i18n.js`、`ui_toast_test.go`、`docs/design-mocks/annulled-document-ui.html`、本报告。
- **终点形状：** 只标异常态；状态文案唯一；原因仅详情。
- **权威依据：** `docs/fiscal-gr.zh.md` 状态机 N/A/F；SAF-T Reason 已在作废流程落地。
- **唯一写法：** `formatInvoiceDocStatus` / `invoiceListStatusPill` / `formatInvoiceDocStatusDetailHtml`；列表行 class `is-annulled`。

## 2. 核心业务规则

1. 【本次新增】列表不作废列；A → 浅红底 + pill；F → pill；N → 无 pill。
2. 【本次新增】详情 A 显示「已作废」pill + 作废原因。
3. 【已实现】A 不显示作废按钮。

## 3. 详细测试场景

### 场景 1：唯一写法钉死
- **结果：【已验证】pass** — `TestAdminHTMLAnnulledDocVisualSinglePath`

### 场景 2：bootstrap 单测
- **操作：** `go test ./internal/fiscal/bootstrap/ -count=1`
- **结果：【已验证】pass**

### 场景 3：真界面 GR/8 列表与详情
- **结果：【未验证】** — 需 Cmd+Shift+R；预期列表有「已作废」、详情有原因。

## 4. mesa-local-product-test

| 项 | 结果 |
|----|------|
| mesa-local-product-test | N/A · 【未验证】 |
| 替代 | bootstrap `go test` |

## 5. 实际测试结果

| 项 | 结果 |
|----|------|
| 唯一写法单测 | pass |
| bootstrap 全量 | pass |
| 真界面 | 【未验证】 |

## 6. 风险与待确认

- 嵌入资源需重启 Agent 后生效。
- 静态稿：http://127.0.0.1:17890/annulled-document-ui.html
