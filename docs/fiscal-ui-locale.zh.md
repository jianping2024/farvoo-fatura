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
| 8 | **纸面语言**（Mesa 功能管理 `print_locale` → job `payload.locale`）与 `ui_locale` **分轨**；Agent 热敏票固定栏（桌号/小计/预结单等）**唯一** `printTicketLabels` → `normalizePrintLocale` → `labelsFor` |

---

## 2. 三条语言轨（勿混）

| 轨 | 开关 | 作用范围 | Agent 唯一入口 |
|----|------|----------|----------------|
| **屏幕** | `config.json` `ui_locale` | 托盘、Admin 导航/设置页/支付文案 | `loadAgentUILocale` / `applyUILocaleToConfig` + `FiscalAdminI18n` |
| **纸面（Mesa 厨打/预结/收银小票）** | Farvoo Dashboard 功能管理 `restaurants.print_locale` → job `payload.locale` | 热敏票**固定栏**（桌号、Guest、Items、Pre-Bill 标题等）；**不**改菜品名 | `normalizePrintLocale` + `printTicketLabels` |
| **税票 FT（方案 A）** | `ui_locale` 派生 `invoiceLocale` | 认证 FT 票面业务标签 + 冻结进 print payload | `locale.InvoiceLocaleFromUI` + `print.receiptLabels` |

空 `payload.locale` → **pt**（与 Mesa 默认一致）。`zh` / `en` / `pt` 三档全走 `labelsFor`，禁止「zh 否则 English」。

预结单（`pre_bill` / Consulta de Mesa）票尾须印「非发票」声明 + 开票填写栏（姓名/NIF/地址，下划线空行）+ 致谢，**跟 `print_locale`**（空→pt）；**不**印在 FT 税票、厨打、结账收据上。标题：zh `预结账单` / en `Table Consultation` / pt `Consulta Mesa`。

---

## 3. 唯一写法

| 职责 | 唯一入口 |
|------|----------|
| UI normalize | `locale.NormalizeUILocale` |
| 发票语言派生 | `locale.InvoiceLocaleFromUI` |
| 票面业务标签 | `print.receiptLabels` |
| Admin 读/写 HTTP | `GET\|PUT /local/v1/setup/ui-locale` |
| Agent `config.json` 读写 | `loadAgentUILocale` / `setAgentUILocale`（默认路径） |
| `config.UILocale` 赋值 | **唯一** `applyUILocaleToConfig`（Admin/托盘/wizard 同函数） |
| Wizard 写 HTTP | `POST /api/ui-locale`（可同请求写 `text_encoding`；**不**与 Admin 端点合并） |
| fiscal-local 文件 | `locale.PrefsFile`（`DataDir/ui_locale.json`；无 Agent 回调时） |
| Admin 文案字典 | `FiscalAdminI18n`（`admin-i18n.js`） |
| 支付方式 code 表 | `domain.KnownPaymentMethods`（Admin `pay.*` / 票面 `Pay*` 分表，仅 key 对齐） |
| Mesa 热敏票固定栏 | `printTicketLabels`（`escpos_encoding.go`；仅 `payload.locale`） |
| Mesa `payload.locale` normalize | `normalizePrintLocale`（默认 pt；**不**读 `ui_locale`） |
| 预结单「非发票」声明 | **唯一** `writePreBillLegalBlock`（仅 `pre_bill`）；填写行 **唯一** `preBillFillLine`；文案在 `labelsFor` 的 `notAnInvoice` / `invoiceFill*` / `thankYouVisit`；跟 `print_locale`，空→pt |

---

## 4. 文案原则

短、通用、准；葡语 **pt-PT**；认证 / ATCUD / NIF / IVA 缩写不跟 UI 变。
