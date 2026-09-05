# Fiscal Admin UI

> **Feedback（定稿）:** 操作成功/失败 **只**用 `FiscalUI.showToast(message, type)`，契约对齐 restaurant-ordering `components/ui/Toast.tsx`（`success` | `error` | `info`，右下角）。  
> **实现：** `internal/fiscal/bootstrap/ui/toast.js` + `toast.css`，经 `/fiscal-ui/toast.*` 嵌入；**禁止**在 Admin 页内再写一套 banner / flash / toast。

## 现状

- **店员路径：** 按 [`docs/fiscal-admin-ui-prototype/README.md`](../../../docs/fiscal-admin-ui-prototype/README.md) v2（**方案 A 用语**）落地正式 Admin。  
- **当前入口（P0 定法）：** bootstrap 嵌入的 `internal/fiscal/bootstrap/ui/admin/`（发票 hub / 账单 / 商品 / 客户 / 设置）。
- **本目录（`web/fiscal-admin/`）：** 早期 React 静态原型路径，**已废弃**；勿再维护或二选一部署。验收以 bootstrap Admin + [`docs/fiscal-admin-ui-prototype/README.md`](../../../docs/fiscal-admin-ui-prototype/README.md) 为准。

## P0 屏幕（与 v2 原型一致）

| # | 屏幕 | 餐馆 | 商超 | 里程碑 |
|---|------|------|------|--------|
| 1 | 登录（业态 + PIN 占位） | ✓ | ✓ | M2.6 |
| 2 | 发票 hub（统计 + 列表 + 开票入口） | ✓ | ✓ | M2.6 |
| 3 | 开票 / 四步进度（无草稿列表） | ✓ | ✓ | M2.6 |
| 4 | 账单 → 进入开票 | ✓ | — | M2.6（API：M2.5） |
| 5 | 开票（购方、付款、签发 FT） | ✓ | ✓ | M2.6 |
| 6 | 发票详情 / 重打（列表在 hub） | ✓ | ✓ | M2.6b–e；**按类型 Tab**（FT / FS / NC / ND） |
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
