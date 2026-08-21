# Fiscal Admin UI

> **Feedback（定稿）:** 操作成功/失败 **只**用 `FiscalUI.showToast(message, type)`，契约对齐 restaurant-ordering `components/ui/Toast.tsx`（`success` | `error` | `info`，右下角）。  
> **实现：** `internal/fiscal/bootstrap/ui/toast.js` + `toast.css`，经 `/fiscal-ui/toast.*` 嵌入；**禁止**在 Admin 页内再写一套 banner / flash / toast。

## 现状

当前入口仍是 bootstrap 嵌入的 `admin_html.go`（设置 + 草稿工作台）。静态 React 构建（本目录）为后续正式 Admin；新反馈组件继续扩展 `bootstrap/ui/`（或将来 `web/fiscal-admin` 内的同一 Toast 契约），勿在业务页内联。

## P0 screens（正式 Admin 以后）

- Manual FT issue
- Document search / detail
- Reprint
- NC (admin)
- Series status
- SAF-T export
- Test print
