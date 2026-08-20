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
| 开票员 PIN | SQLite `operators` | Fiscal setup / 同步 |

## 税务打印机解析（唯一）

`worker.ResolveFiscalPrinterTCP`：

1. 环境变量 `FISCAL_PRINTER_TCP`（host:port 或 `tcp:host:port`）  
2. 否则 `config.station_printers["fiscal_receipt_printer"]`  

未配置时 Worker 仅 MemorySink（开发可开票，不出网）。

## 进程内端口

| 服务 | 默认 |
|------|------|
| 托盘配对/打印机设置 | `:17892`（既有） |
| Fiscal Local API + Admin | `FISCAL_BIND` 默认 `127.0.0.1:17880` |
| SQLite 路径（可选覆盖） | `FISCAL_DB` / `FISCAL_DATA_DIR`（UAT；默认在 Agent 数据目录下） |

二者同进程；Fiscal **不**占用配对端口。

## 禁止

- 把 NIF / 系列验证码 / Hash 写入 `config.json`  
- 云端 `print_jobs` 作为正式税务出纸权威  
- 第二套「解析财政打印机」函数  
