# 本次改动业务场景测试报告：开票页发票信息卡三段壳 + 底栏固定

日期：2026-11-03  
分支：`ui/compact-invoice-info-card`  
仓库：`farvoo-fatura`  
本机：`http://127.0.0.1:17882`（`FISCAL_AT_ENV=mock`，临时 seed DB）  
方式：Agent 改 Admin HTML/CSS → `go test` → Chrome DevTools 真界面量布局。

## 1. 本次修改

- **内容：** 开票第 1–3 步「本单合计」与第 4 步「发票信息」改为 `panel-sheet` + `sheet-head` / `sheet-body` / `sheet-foot`；主 CTA（确认 / 签发·重打·取消）钉在 foot。Admin 全部 modal 同步改用同一套 `sheet-*`（去掉 `modal-head/body/foot`）。GT 字段略压密（NIF/名称、付款/打印机两列；`#grFields` 底距收紧）。
- **文件：** `apps/fiscal-agent/internal/fiscal/bootstrap/admin/index.html`、`ui_toast_test.go`、本报告。
- **唯一写法：** 仅一处注释 `ONLY pinned head/body/foot shell`；滚动与钉栏类名仅 `sheet-head` / `sheet-body` / `sheet-foot`（modal 与 order panel 共用）。

## 2. 核心业务规则

1. 【已实现】长表单时只有 `sheet-body` 滚动；`sheet-foot` 内主按钮始终可见。
2. 【已实现】开钱箱仍走既有 `.main-chrome` fixed，不进 panel-foot。
3. 【已实现】GT/GR 字段集合与校验逻辑不变，仅布局密度与壳层变化。

## 3. 详细测试场景

### 场景 1：唯一写法 / 冗余清理

- **操作：** `git diff` 统计 `ONLY pinned head/body/foot shell`；搜遗留 `modal-head|modal-body|modal-foot`。
- **结果：【已验证】pass** — ONLY 标记恰好 1 处；无 `modal-*` 壳类；settings 的 `panel-footer` / `settings-panel-body` 保留（非本壳）。

### 场景 2：单测钉死壳结构

- **操作：** `go test ./internal/fiscal/bootstrap/ -count=1`
- **结果：【已验证】pass** — 含 `btnIssueInvoice` / `btnConfirmOrder` 必须在 `sheet-foot`、`panel-sheet` 恰好两处。

### 场景 3：第 1–3 步确认钮在 foot

- **前置：** mock Agent `:17882`，PIN `123456`，新建商品入单。
- **操作：** 进入开票页，量 `#btnConfirmOrder`。
- **结果：【已验证】pass** — `btnInFoot=true`，`panelOverflow=hidden`，`bodyOverflow=auto`，`footVisible=true`（viewport 720）。

### 场景 4：第 4 步 GT 长表单滚动后签发钮仍钉住

- **操作：** 确认进第 4 步 → 票种切 GT → 填收货地址 → 将 `.sheet-body` 滚到底 → 再量 `#btnIssueInvoice`。
- **结果：【已验证】pass** — `bodyScroll.scrollTop≈648`、`scrollHeight=1039`、`clientHeight=391`；签发钮仍在 `sheet-foot` 且 `footVisible=true`。

### 场景 5：开钱箱仍独立 fixed

- **结果：【已验证】** CSS 未改 `.main-chrome`；本刀未把开钱箱移入 sheet-foot。

### 场景 6：分单页签发轨

- **结果：【未验证】** 本刀明确不改 `view-bill-split`（方案约定第二刀）。

### 场景 7：真签发 FT/GT（业务落库）

- **结果：【未验证】** 本刀纯布局；未跑签发路径（mock/AT）。

## 4. mesa-local-product-test

| 项 | 结果 |
|---|---|
| `mesa-local-product-test`（Mesa 餐厅仓技能） | **N/A · 【未验证】** — 本改动在 fiscal-agent Admin embed HTML；不经 Mesa `:3000` UI / 开票路径 |

## 5. 实际测试结果

| 项 | 结果 |
|---|---|
| 唯一写法仅一份 + 无 modal-* 壳 | **pass** |
| `go test ./internal/fiscal/bootstrap/` | **pass** |
| 确认钮钉在 sheet-foot | **pass** |
| GT 滚动后签发钮仍可见 | **pass** |
| mesa-local-product-test | N/A · 【未验证】 |
| 分单页 / 真签发 | 【未验证】 |

## 6. 风险与待确认

- `max-height: calc(100vh - 12.5rem)` 在极矮窗口或放大字号时可能偏紧；若现场反馈再调常数。
- 窄屏（≤860px）`.split` 变单列后，NIF/名称并排会叠成竖排——预期行为。
- 分单「签发此人」尚未套同一壳，长表单时仍可能滚丢按钮。
