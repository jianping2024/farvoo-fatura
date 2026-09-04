# Fiscal 多台电脑开票（LAN + 登记 UX）Implementation Plan

> **状态：定稿（0.4.88：监听只认 config.json，无 LAN env 管道）**  
> **权威：**`resolveFiscalListenBind` + `validateBindAddr(addr, allowLAN)` + `loopbackAdminURL` + `startEmbeddedFiscal`

**Goal:** Admin「多台电脑开票」只靠 `config.json` 的 `fiscal_allow_lan`；本机壳永不打开 `0.0.0.0`；系统/进程 `FISCAL_ALLOW_LAN`/`FISCAL_BIND` 不参与 Agent 产品路径。

## 唯一写法（自检）

| 唯一 | 符号 |
|------|------|
| 监听地址 | `resolveFiscalListenBind` |
| H2 校验 | `validateBindAddr(addr, allowLAN)` |
| 本机打开 URL | `loopbackAdminURL`（经 `fiscalAdminBaseURL`） |
| 监听重绑 | `startEmbeddedFiscal` |
| 保存后重绑入口 | `agentLanAccessSet` |
| config 读写 | `loadAgentFiscalAllowLANState` / `setAgentFiscalAllowLAN` |
| 快照 | `buildAgentLanAccessSnapshot` |

## 已收掉（禁止复活）

- `captureLanOpsEnvLocksOnce` / `lanOpsLocked*`
- Agent 内 `Setenv`/`Getenv` `FISCAL_ALLOW_LAN` / `FISCAL_BIND`
- `env_locked` / `ErrLanEnvLocked` / Admin 锁定提示
