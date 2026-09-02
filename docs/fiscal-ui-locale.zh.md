# 界面语言与税票语言（方案 A）

> **状态：定稿**  
> **权威：是**  
> **对应实现：** `internal/fiscal/locale`、`print.receiptLabels`、`GET|PUT /local/v1/setup/ui-locale`、Admin 设置概览、托盘镜像

---

## 1. P0 定法

| # | 定法 |
|---|------|
| 1 | `ui_locale ∈ {zh, en, pt}` 唯一开关；**不**另开「发票语言」开关 |
| 2 | 派生（**唯一** `locale.InvoiceLocaleFromUI`）：`zh→pt`，`en→en`，`pt→pt` |
| 3 | 主入口：Admin **设置 → 概览**；次入口：托盘「界面语言」（同值） |
| 4 | 持久化：Agent → `config.json` `ui_locale`（`loadAgentUILocale` / `setAgentUILocale`）；fiscal-local → `DataDir/ui_locale.json`（`locale.PrefsFile`） |
| 5 | 税票业务标签：**唯一** `print.receiptLabels(invoiceLocale)`；认证句始终葡语 |
| 6 | 开票时把 `locale` 冻入 print Payload；重打沿用冻结值；空旧票 → pt |
| 7 | 厨打/Mesa `payload.locale` 与本开关无关 |

---

## 2. 唯一写法

| 职责 | 唯一入口 |
|------|----------|
| UI normalize | `locale.NormalizeUILocale` |
| 发票语言派生 | `locale.InvoiceLocaleFromUI` |
| 票面业务标签 | `print.receiptLabels` |
| Admin 读/写 | `GET\|PUT /local/v1/setup/ui-locale` |
| Agent config 读写 | `loadAgentUILocale` / `setAgentUILocale` |
| Admin 文案字典 | `FiscalAdminI18n`（`admin-i18n.js`） |

---

## 3. 文案原则

短、通用、准；葡语 **pt-PT**；认证 / ATCUD / NIF / IVA 缩写不跟 UI 变。
