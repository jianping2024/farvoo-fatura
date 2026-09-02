# config.json vs Fiscal SQLite 边界

> **状态：定稿**  
> **权威：是**（配置存放边界；列定义仍以 schema/DDL 为准）  
> **对应实现：** Agent `config.json` + `internal/fiscal` SQLite  
> **计划：** [`fiscal-dev-plan.zh.md`](fiscal-dev-plan.zh.md) M2 D2.4

## 分工

| 数据 | 存放 | 谁写 |
|------|------|------|
| `api_base`、`agentjwt`、Realtime/Supabase 会话 | `config.json` | 配对 / 云端刷新 |
| `station_printers`（含 `fiscal_receipt_printer`） | `config.json` | 打印机设置向导 |
| `default_printer` / legacy `printer_host` | `config.json` | 向导 / 参数 |
| 纳税人、AT 凭证、系列、签名钥、发票、税务打印队列 | **SQLite** | Fiscal Local API / Core |
| 开票员 PIN | SQLite `operators` | Admin 设置（本地创建 + 设 PIN） |
| `fiscal_allow_local_provision` / `fiscal_at_env` | `config.json` | 本机开关（默认允许粘贴 PEM 激活；AT 默认 mock）。环境变量若已设则优先 |

## 税务打印机解析（唯一）

`worker.ResolveFiscalPrinterTCP`：

1. 环境变量 `FISCAL_PRINTER_TCP`（host:port 或 `tcp:host:port`）  
2. 否则 `config.station_printers["fiscal_receipt_printer"]`  

未配置时 Worker 仅 MemorySink（开发可开票，不出网）。

| 项 | 定法 |
|----|------|
| `FISCAL_SESSION_SECRET` | 生产店机 **必填**（≥32 字节 UTF-8）；`FISCAL_ALLOW_DEV_KEY=1` 的 UAT 可省略（派生密钥） |
| 未设且非 dev | `NewSessionManager` 失败 → 进程退出（`MustNewSessionManager`） |

## 进程内端口

| 服务 | 默认 |
|------|------|
| 托盘配对/打印机设置 | `:17892`（既有） |
| Fiscal Local API + Admin | `FISCAL_BIND` 默认 `127.0.0.1:17880`；**店内多端**见 [`fiscal-m3-2-operators.zh.md`](fiscal-m3-2-operators.zh.md) §3.8：`FISCAL_ALLOW_LAN=1` + 非 loopback bind |
| SQLite 路径（可选覆盖） | `FISCAL_DB` / `FISCAL_DATA_DIR`（UAT；默认在 Agent 数据目录下） |

二者同进程；Fiscal **不**占用配对端口。

## 安装目录 vs 数据

| 路径 | 内容 |
|------|------|
| `C:\Program Files\Farvoo Fiscal Agent\` | Setup 覆盖的 exe / VERSION.txt；**升级可整夹替换** |
| `%USERPROFILE%\.config\farvoo-fiscal-agent\config.json` | 配对 / 打印机映射 |
| `%LOCALAPPDATA%\Farvoo Fiscal Agent\`（默认） | `fiscal.db`、日志等；**升级 Setup 不删** |

不要把 SQLite 或密钥放进 Program Files。

## 禁止

- 把 NIF / 系列验证码 / Hash 写入 `config.json`  
- 云端 `print_jobs` 作为正式税务出纸权威  
- 第二套「解析财政打印机」函数  
