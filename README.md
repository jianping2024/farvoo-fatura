# farvoo-fatura

**Farvoo Local Agent**：门店本机的 **打印 + 合规开票** 统一进程。

发票不是旁路系统——**正式发票也是打印模块的一部分**（税务签发后进本地打印队列出纸）。业务热敏（厨打/预结等）与税务小票共用同一 Agent、同一套打印机绑定与 ESC/POS 能力。

## 定位（定稿）

| 项 | 定法 |
|----|------|
| 本仓职责 | 承担门店 **打印模块**（含业务打印 + 发票打印） |
| 发票 | Fiscal Core 签发 → **同一进程**本地打印队列出纸 |
| 原 `restaurant-ordering/apps/print-agent` | 历史拷贝来源；后续以 **本仓** 为打印 Agent 演进主线 |

## 目录

```text
document/                 需求、SAF-T、WSDL、零售 POS 设计等
apps/fiscal-agent/        Go Agent（打印能力 + Fiscal）
  internal/fiscal/        税务签发 / SQLite / Local API / 税务打印队列
  *.go / escpos*          既有打印、配对、托盘、云端业务队列认领
```

## 本机身份

- Go module：`farvoo-fiscal-agent`
- Windows 产物：`FarvooFiscalAgent.exe` / `FarvooFiscalAgent-Setup-amd64.exe`
- 配置：`~/.config/farvoo-fiscal-agent/config.json`（Windows 同路径）
- 数据目录：`%LOCALAPPDATA%\Farvoo Fiscal Agent\`（含 `fiscal.db`；**不是** Program Files）
- Mutex：`Global\FarvooFiscalAgent-SingleInstance-v1`

## Windows 发布

见 [`apps/fiscal-agent/README.md`](apps/fiscal-agent/README.md) **Windows release**。摘要：

```bash
git tag fiscal-agent-v0.3.86   # 须与 apps/fiscal-agent/VERSION 一致
git push origin fiscal-agent-v0.3.86
```

制品源：本仓 GitHub Releases（`jianping2024/farvoo-fatura`）。Dashboard 下载 env 改指本仓属另仓跟进。

## 开发计划

里程碑与每步交付物（权威）：[`docs/fiscal-dev-plan.zh.md`](docs/fiscal-dev-plan.zh.md)

当前：**M0–M6 + M3.2 已完成**；**P0 认证阶段已关闭**（2026-09-02）。权威安装包：**v0.4.56**。下一步：**P1 待选型**（见开发计划 §P1 待选型）。

## 当前阶段

**P0 认证已关闭**：自动回归 + 手测 H1–H3 已通过；制品 **0.4.56**。日常开票路径：Farvoo 同步关台 → 收银账单 → Admin PIN 登录 → FS/FT 签发/打印。

```bash
cd apps/fiscal-agent
# 主进程内嵌 Fiscal（Mac UAT）
FISCAL_ALLOW_LOCAL_PROVISION=1 FISCAL_AT_ENV=mock go run . -fiscal-standalone
# 认证回归（SQLite 断言）
node scripts/fiscal-d62-cert-regression.mjs
node scripts/fiscal-m3-operators-regression.mjs
node scripts/fiscal-bill-sync-regression.mjs
```

- Setup API：`/local/v1/setup/*`（见 `docs/fiscal-m1-identity-series.zh.md`）
- 签发：`POST /local/v1/fiscal-documents`（唯一）
- Admin：`http://127.0.0.1:17880/`（`FISCAL_BIND`）
- 配置边界：`docs/fiscal-config-boundary.zh.md`
- DB 断言：SQLite（`assert-db`；本仓无 Supabase）

## 继承经验（必读）

原 `restaurant-ordering` print-agent 踩过的坑与定法已整理：

→ [`docs/print-agent-lessons.zh.md`](docs/print-agent-lessons.zh.md)

改安装器、ESC/POS、托盘、队列或 Fiscal 签发前先对照；相关约束已有单测护栏，禁止回潮。

## 数据库设计

- 库表设计（权威）：[`docs/fiscal-sqlite-schema.zh.md`](docs/fiscal-sqlite-schema.zh.md)
- DDL：`apps/fiscal-agent/internal/fiscal/store/migrations/001_init.sql`
- **设计文档写法规范（必遵）：** [`docs/design-doc-standards.zh.md`](docs/design-doc-standards.zh.md)
