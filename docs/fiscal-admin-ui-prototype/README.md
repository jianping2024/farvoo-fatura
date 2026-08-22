# Fiscal Admin UI 原型（可点击）

> **状态：草稿（仅交互稿，未接 API）**  
> **权威：否** — 正式产品 UI 定稿前以此为讨论稿；实现仍以 `docs/fiscal-dev-plan.zh.md` 里程碑为准。

## 打开

```bash
open docs/fiscal-admin-ui-prototype/index.html
```

或在 Finder / 浏览器中直接打开该文件。

## 可点流程

| 屏 | 交互 |
|----|------|
| 登录 | PIN 键盘；演示 `1234`（满 4 位自动进） |
| 主页 | 摘要卡片 + 快捷入口 |
| 草稿开票 | 列表 → 详情 → 开 FT / 丢弃 |
| 查票 | 列表 → 详情 → **再打一张（2a Via）** |
| 手动开 FT | 加行 / 签发 |
| 商品主档 | LOCAL 编辑（REMOTE 只读示意） |
| 设置 | 纳税人 / AT / 系列 / 打印机 Tab |
| 冲销 NC | M3 占位 |

## 非目标

- 不接真实 Local API  
- 不是最终视觉规范（可再改色/字/信息架构）  
- 不替代现有 `admin_html.go` 调试页
