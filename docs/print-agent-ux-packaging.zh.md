# P1-U 拍板记录 — 托盘 / 安装 / 打票可选

> **状态：定稿**（P1-U **已关闭**；**无代码交付**；仅保留产品拍板，供引用）  
> **权威：是**（装机 UX 口径；工程禁区见 [`print-agent-lessons.zh.md`](print-agent-lessons.zh.md)）  
> **总原则：** [`fiscal-dev-plan.zh.md`](fiscal-dev-plan.zh.md) §9（打票为可选产品）

---

## 产品拍板（P0）

| 项 | 定法 |
|----|------|
| 打票（Fiscal） | **可选产品**；店可 **仅厨打**（配对 + 映射打印机即完成） |
| 安装包 | 同进程嵌入 Fiscal ≠ 每店必配；未开通店忽略托盘「开票」与 `17880` |
| `17892/configure` | **仅**打印机设置；**不做**顶部开店 / Fiscal checklist |
| 托盘 | **厨打运行时**（Connected / Realtime / 轮询 / 打印中）；**不**汇总开店步骤或开票就绪 |
| 开票就绪 | **仅 Admin** `17880`（wizard + `setup/status`） |
| Inno wizard | 维持 `wizard-after.txt` **一行 optional** Fiscal；不扩激活长文 |
| LAN 开票 | Ops / Admin / 运维文档；安装器 **不**自动改 `FISCAL_BIND` |
| 文档禁令 | **禁止**将上述分工写为「缺口」或 backlog（含 P1-U 草案所列项） |

**P1-U 草案已否决（勿再做）：** configure checklist、`Readiness` 聚合、托盘 Fiscal 进度、Inno Fiscal 长引导、云端 Fiscal 就绪灯。

---

## 用户路径（引用用）

**厨打（默认）：** Setup → Dashboard 配对码 → `17892` 配对 → 扫打印机 → 映射 → 保存。

**打票（可选）：** 托盘「开票」或 `17880` → Admin wizard → `ready_to_issue`。手测：[`fiscal-m6-manual-uat.zh.md`](fiscal-m6-manual-uat.zh.md)。LAN：[`fiscal-m3-2-operators.zh.md`](fiscal-m3-2-operators.zh.md) §3.8。

---

## 修订记录

| 日期 | 变更 |
|------|------|
| 2026-09-02 | 草稿（checklist 等）→ 产品否决 → **本文瘦身定稿** |
