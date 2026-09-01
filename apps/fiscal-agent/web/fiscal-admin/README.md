# Fiscal Admin UI

> **Feedback（定稿）:** 操作成功/失败 **只**用 `FiscalUI.showToast(message, type)`，契约对齐 restaurant-ordering `components/ui/Toast.tsx`（`success` | `error` | `info`，右下角）。  
> **实现：** `internal/fiscal/bootstrap/ui/toast.js` + `toast.css`，经 `/fiscal-ui/toast.*` 嵌入；**禁止**在 Admin 页内再写一套 banner / flash / toast。

## 现状

- **店员路径：** 按 [`docs/fiscal-admin-ui-prototype/README.md`](../../../docs/fiscal-admin-ui-prototype/README.md) v2（**方案 A 用语**）落地正式 Admin。  
- **当前入口：** bootstrap 嵌入的 `admin/index.html`（工作台 / 手工开票 / 收银账单 / 发票 / 商品 / 客户 / 设置）。  
- **本目录：** 可选静态 React 构建；与 bootstrap 嵌入二选一，以 [`fiscal-dev-plan.zh.md`](../../../docs/fiscal-dev-plan.zh.md) M2.6 验收为准。

## P0 屏幕（与 v2 原型一致）

| # | 屏幕 | 餐馆 | 商超 | 里程碑 |
|---|------|------|------|--------|
| 1 | 登录（业态 + PIN 占位） | ✓ | ✓ | M2.6 |
| 2 | 工作台 | ✓ | ✓ | M2.6 |
| 3 | 手工开票列表 / 新建开票 / 四步进度 | ✓ | ✓ | M2.6 |
| 4 | 收银账单 → 进入开票 | ✓ | — | M2.6（API：M2.5） |
| 5 | 开票（购方、付款、签发 FT） | ✓ | ✓ | M2.6 |
| 6 | 发票列表 / 详情 / 重打 | ✓ | ✓ | M2.6b–e；**按类型 Tab**（FT / FS / NC / ND） |
| 7 | 商品维护 | ✓ | ✓ | M2.6a ✓ |
| 8 | 客户维护 | ✓ | ✓ | M2.6a ✓ |
| 9 | 设置（纳税人、系列、打印机） | ✓ | ✓ | M2.6（迁入 M1 页） |

## P1（后续里程碑）

| 屏幕 | 里程碑 |
|------|--------|
| 冲销 NC | M3 |
| 系列状态 / SAF-T 导出 | M5 |
| 测试打印 | M2 已有能力，正式 Admin 设置内入口 |

## 用语

界面禁止「草稿、LOCAL、§7、bill-draft」等；侧栏主名禁止「订单」「待开票账单」。  
对照表见 [`fiscal-admin-ui-prototype/README.md`](../../../docs/fiscal-admin-ui-prototype/README.md)「业务用语」与 [`fiscal-bill-draft-workbench.zh.md`](../../../docs/fiscal-bill-draft-workbench.zh.md) §0。
