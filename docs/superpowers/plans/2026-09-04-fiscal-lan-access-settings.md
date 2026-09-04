# Fiscal 多台电脑开票（LAN + 登记 UX）Implementation Plan

> **状态：定稿（0.4.87 已落地 LAN 监听修复）**  
> **权威修复：**磁盘 `fiscal_allow_lan` + `captureLanOpsEnvLocksOnce` + `loopbackAdminURL` + `startEmbeddedFiscal` 重绑

**Goal:** Admin「多台电脑开票」可稳定打开 LAN 监听；本机壳永不打开 `0.0.0.0`。

## 唯一写法（自检）

| 唯一 | 符号 |
|------|------|
| ops env 锁定捕获 | `captureLanOpsEnvLocksOnce` |
| LAN env 灌入 | `applyFiscalRuntimeFromConfig` |
| 本机打开 URL | `loopbackAdminURL`（经 `fiscalAdminBaseURL`） |
| 监听重绑 | `startEmbeddedFiscal` |
| 保存后重绑入口 | `agentLanAccessSet` |
| config 读写 | `loadAgentFiscalAllowLANState` / `setAgentFiscalAllowLAN` |
| 快照 | `buildAgentLanAccessSnapshot` |
