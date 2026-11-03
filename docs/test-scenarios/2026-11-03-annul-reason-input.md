# 本次改动业务场景测试报告：PF/GR/GT 作废原因真输入

日期：2026-11-03  
分支：`fix/annul-reason-input`  
仓库：`farvoo-fatura`  
本机：`http://127.0.0.1:17881`（`FISCAL_AT_ENV=test`，`fiscal-test.db`）  
方式：按确认方案改 Admin 确认框 → `go test` 钉死 → 真界面待用户复测。

## 1. 本次修改

- **内容：** 作废确认框增加真正的原因输入（必填、≤50 字、字数提示）；提交把用户输入作为 `reason`；去掉写死的 `Cancelado`（仅作 placeholder）。
- **文件：** `apps/fiscal-agent/internal/fiscal/bootstrap/admin/index.html`、`ui/admin-i18n.js`、`ui_toast_test.go`、本报告。
- **终点形状：** 作废走同一套 `openConfirmAction`；需要原因时显示 `#confirmActionReason`；服务端仍校验 1–50；其它确认操作不显示输入框。
- **权威依据：** SAF-T `DocumentStatus/Reason` = `SAFPTtextTypeMandatoryMax50Car`（≤50）；`docs/fiscal-pf.zh.md` / `docs/fiscal-gt.zh.md` 作废须 reason；后端 `AnnulGR`/`AnnulGT`/`AnnulPF` 已强制 reason。
- **唯一写法：** `openConfirmAction`（可选 `withReason`）+ `#confirmActionReason`；作废 handler 仅一处 `withReason: true`。

## 2. 核心业务规则

1. 【本次新增】PF/GR/GT 详情「作废」必须填原因才能确认；空原因留在弹窗并 toast。
2. 【本次新增】原因最长 50（Unicode 码点；`maxlength=50` + 提交再数一遍）；字数 `n/50`。
3. 【已实现】已挂 FT 的 GR（状态 `F`）仍不显示作废按钮。
4. 【已实现】服务端作废仍只改本地、不报 AT（既有待定项，本刀未改）。

## 3. 详细测试场景

### 场景 1：唯一写法 / 无写死 Cancelado

- **操作：** diff / `TestAdminHTMLConfirmActionSinglePath`。
- **结果：【已验证】pass** — `withReason: true`；作废块无 `reason: 'Cancelado'`；`confirmActionReason` 与 `CONFIRM_ACTION_REASON_MAX = 50` 各一份声明。

### 场景 2：其它确认框不出现原因框

- **操作：** 读 `openConfirmAction`：仅 `withReason` 时去掉 `#confirmActionReasonWrap` 的 `hidden`。
- **结果：【已验证】pass**（代码路径）；真界面删终端等【未验证】。

### 场景 3：空原因不可提交

- **操作：** 真界面作废 → 不填原因点确认。
- **结果：【未验证】** — 待用户；预期 toast `action.annul_pf_reason_need`，弹窗不关、不请求。

### 场景 4：填原因后 GR 状态 N→A 且 status_reason 一致

- **前置：** 状态 N 的 GR（如 `GR2026T02/8` 或 `/5`）。
- **操作：** 作废 → 输入例如 `Erro de emissao` → 确认。
- **结果：【未验证】** — 待用户；预期 `document_status=A`，`status_reason` 等于所填。

### 场景 5：bootstrap 单测

- **操作：** `go test ./internal/fiscal/bootstrap/ -count=1`
- **结果：【已验证】pass**

## 4. mesa-local-product-test

| 项 | 结果 |
|----|------|
| `mesa-local-product-test`（Mesa 餐厅仓） | **N/A · 【未验证】** — 不经 Mesa `:3000` |
| 替代验收 | `go test ./internal/fiscal/bootstrap/` 【已验证】pass |

## 5. 实际测试结果

| 项 | 结果 |
|----|------|
| 唯一写法 / 不写死 Cancelado | **pass** |
| bootstrap 单测 | **pass** |
| 真界面空原因 / 填原因作废 | 【未验证】（Agent 已重启 `:17881`，待 Cmd+Shift+R 后测） |
| mesa-local-product-test | N/A · 【未验证】 |

## 6. 风险与待确认

- 真界面作废尚未由用户点过；合入后请用状态 N 的 GR/GT 复测场景 3–4。
- GR/GT 作废仍不通知 AT（既有产品待定）。
