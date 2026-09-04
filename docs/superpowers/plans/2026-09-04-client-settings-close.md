# Client settings dialog close (Terminate)

> **状态：定稿（0.4.89）**

**Goal:** 「保存并打开」/「取消」关闭设置 WebView，主流程才能继续开开票页。

## 唯一写法

| 唯一 | 符号 |
|------|------|
| HTML 对话框退出 | `closeHTMLDialog`（`runHTMLWindowOnThread` → `Dispatch`+`Terminate`） |
| Bind 注入 | `HTMLWindowOptions.Bind func(closeDialog func())` |
| 设置结果+关窗 | `finishSettings`（`settings_windows.go`） |

## 已收掉

- `finish` 返回 `(bool, error)` 假装关窗
- JS `save` 再 `closeSettings(base)` 双关
