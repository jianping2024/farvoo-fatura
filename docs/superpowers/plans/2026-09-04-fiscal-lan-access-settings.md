# Fiscal 多台电脑开票（LAN + 登记 UX）Implementation Plan

> **状态：定稿（实现中）**  
> **For agentic workers:** 按任务勾选；改完自检「唯一写法」在 diff 里真的只有一份。

**Goal:** Admin「多台电脑开票」傻瓜页：① 开关是否允许其它电脑连接；② 「添加开票电脑」发大号配对码。`fiscal_allow_lan` 进 `config.json`，不再手敲系统 env。

**Architecture:** `config.json` → `applyFiscalRuntimeFromConfig`（唯一灌 env）→ 现有 bind 校验。API `GET|PUT /local/v1/setup/lan-access`；PUT 仅 admin + loopback。登记仍用 `allow-next` API，UI 改成向导文案。

## P0 定法

| 项 | 定法 |
|----|------|
| 侧栏/面板名 | **多台电脑开票**（`settings.nav.terminals`） |
| ① 网络 | 开关「允许店内其它电脑连接本机」；状态条 + 可复制 `IP:17880`；防火墙折叠 |
| ② 登记 | 主按钮「添加开票电脑」→ 大号配对码卡片（1/2 步说明）；禁止「允许下一台」文案 |
| 存盘 | `fiscal_allow_lan`；开 → bind `0.0.0.0:17880`；关 → 默认 loopback |
| Env | 进程启动时已设的 `FISCAL_ALLOW_LAN`/`FISCAL_BIND` 优先；UI `env_locked` |
| 生效 | 写盘后 `restart_required`；托盘重启；不热换 listener |
| PUT | `authAdmin` + loopback |
| 安装器 | 仍不自动改 bind |

## 唯一写法（自检清单）

| 唯一 | 符号 / 位置 |
|------|-------------|
| config→env 灌 LAN | `applyFiscalRuntimeFromConfig` |
| config.json 读写 `fiscal_allow_lan` | `loadAgentFiscalAllowLAN` / `setAgentFiscalAllowLAN`（`ui_lan_config.go`） |
| LAN 状态快照 | `buildAgentLanAccessSnapshot` |
| LAN HTTP | `handleGetLanAccess` / `handlePutLanAccess` |
| 店内 IPv4 列表 | `api.AgentLANIPv4` |
| Admin 刷新 LAN 块 | `refreshLanAccessPanel` |
| Admin 保存 LAN | `saveLanAccess` |
| Admin 添加电脑（发卡） | `startAddTerminalWizard`（唯一调 allow-next） |
| 登录页登记提示 | i18n `settings.terminals.login_need` 等（无第二套硬编码中文） |

## File map

见实现；权威产品口径同步：`fiscal-m3-2-operators` §3.8、`fiscal-config-boundary`、`fiscal-client-webview`、`CLIENT-README`、`RELEASE_NOTES`。
