# 本次改动业务场景测试报告 — Agent 档口强制一行 + 中文位图字号本机化

日期：2026-10-31  
分支：`feat/station-slip-one-line-han-px`  
版本：0.5.58  

对应 Mesa 报告：`restaurant-ordering/docs/test-scenarios/2026-10-31-station-slip-one-line-han-px-move.md`

## 唯一写法（自检）

| 职责 | 唯一入口 | diff 内份数 |
|------|----------|-------------|
| 一行闸门 | `stationSlipKeepLines` | 1 定义；Font A / Han 共用 |
| 中文位图字号 | `hanBitmapFontPxFromConfig` | 1；`newEscposFor*` 均调它，不读 payload |

## 单测结果

- `go test ./...`：**pass**【已验证】
- 菜名一行 / 备注一行 / 默认 22 / 忽略 payload / wizard 往返：**pass**【已验证】
