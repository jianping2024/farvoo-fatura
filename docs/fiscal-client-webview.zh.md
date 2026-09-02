# Fiscal 开票客户端壳（WebView2）

> **状态：** 定稿  
> **权威：** 是（开票桌面壳、Client 配置、入口分工；LAN 多端网络仍以 [`fiscal-m3-2-operators.zh.md`](fiscal-m3-2-operators.zh.md) §3.8 为准）  
> **对应实现：** `apps/fiscal-agent/internal/fiscalwebview`、`cmd/fiscal-client`、`fiscal_shell_windows.go`

---

## 1. 背景

店员开票界面由 Agent 内嵌 HTTP Admin（`:17880`）提供。当前托盘「开票」调用系统默认浏览器，体验像「打开网页」，不像 POS 产品。

**P0 定法：** 直接做 **WebView2 专用窗口**；**不做** Chrome `--app=` 过渡。

---

## 2. 目标

| 目标 | 说明 |
|------|------|
| 产品感 | 无地址栏/标签栏；Farvoo 标题与图标；任务栏独立图标 |
| 窗口图标 | **P0 定法：** `assets/app_icon.ico`（深绿底 + 奶油色衬线 F + 右下橙点）经 `rsrc` 嵌入 Agent/Client EXE；WebView2 `IconId` = `fiscalwebview.WindowIconID`（1）；Inno `SetupIconFile` 同文件 |
| 架构不变 | UI 仍为现有 `bootstrap/admin`；会话仍为 Cookie + PIN；**不改** Fiscal API |
| 一 Agent · 多端 | Agent 本机 + LAN 其它 PC 共用同一 WebView2 模块；**两个 binary 入口** |
| 运维简单 | LAN 端 **设置里改 Agent IP**；P0 **不做** mDNS / Dashboard 预填安装包 |

---

## 3. 非目标（P0）

| 项 | 说明 |
|----|------|
| Admin HTML / API 改动 | 壳只加载 URL |
| Electron / Tauri | 过重 |
| macOS / Linux Client | P0 仅 Windows |
| mDNS / 自动发现 Agent | P1 |
| Dashboard 生成带 IP 的安装包 | P1 |
| 修改 `FISCAL_ALLOW_LAN` / 安装器自动开 LAN | 仍见 [`print-agent-ux-packaging.zh.md`](print-agent-ux-packaging.zh.md) |
| Agent 安装器捆绑 Client | **独立安装包**，并列发布 |

---

## 4. 交付物

| # | 交付物 | 说明 |
|---|--------|------|
| D1 | **共用模块** `internal/fiscalwebview` | WebView2 窗口：打开 `baseURL`、单例聚焦、Cookie 持久化目录 |
| D2 | **Agent 集成** | 托盘「开票 / Fiscal…」→ WebView2（`fiscalAdminBaseURL()`），不再 `ShellExecute` 调系统浏览器 |
| D3 | **`FarvooFiscalClient.exe`** | 独立 LAN 开票端；读本地 config；首次 / 设置改 Agent 地址 |
| D4 | **Client 配置** | `%LOCALAPPDATA%\Farvoo Fiscal Client\config.json` |
| D5 | **Client 设置 UI** | 无有效 `agent_base` 或用户点「更改 Agent 地址」→ 小窗：IP + 端口（默认 `17880`）+ 测试连接 |
| D6 | **Client 安装包** | `FarvooFiscalClient-Setup-amd64.exe` + zip；默认桌面快捷方式「Farvoo 开票」 |
| D7 | **Release CI** | 与 Agent 同 tag 流水线产出 Client 产物（或同 workflow 第二 build 步） |
| D8 | **文档** | README、[`fiscal-m3-2-operators.zh.md`](fiscal-m3-2-operators.zh.md) §3.8 补 Client 入口一句 |

---

## 5. 入口分工（P0 定法）

```text
【Agent 机 · 已开通 Fiscal】
  托盘 ^ → FARVOO Fiscal → 开票 / Fiscal…  → WebView2 → http://127.0.0.1:17880/
  桌面（默认勾选）→ Farvoo 开票            → FarvooFiscalAgent.exe fiscal（IPC 或随 Agent 启动）
  托盘 → 打印机设置…                        → 17892（不变）

【LAN 其它开票 PC】
  桌面 / 开始菜单 → Farvoo 开票（Client）  → WebView2 → http://<Agent-IP>:17880/
  开始菜单 → Agent settings…               → FarvooFiscalClient.exe --settings
```

---

## 6. URL 与配置

### 6.1 Agent 本机

| 项 | 定法 |
|----|------|
| baseURL | **`http://127.0.0.1:17880`**（`fiscalAdminBaseURL()`，去掉尾部 `/` 后加载 `{base}/`） |
| 配置文件 | **无**；不暴露给店员 |
| 与 `FISCAL_BIND` | 本机壳 **始终 loopback**；LAN 监听 `0.0.0.0` 不影响本机 URL |

### 6.2 LAN Client

**配置文件路径：** `%LOCALAPPDATA%\Farvoo Fiscal Client\config.json`

| 列 | 类型 | 必填 | 说明 |
|----|------|------|------|
| agent_base | string | 是 | 形如 `http://192.168.1.10:17880`；**禁止**尾部 `/`；存盘前 normalize |

**WebView2 用户数据（Cookie 会话）：** `%LOCALAPPDATA%\Farvoo Fiscal Client\webview\`

**设置 UI 字段：**

| 字段 | 默认 | 说明 |
|------|------|------|
| 主机 | 空 | IPv4 或 hostname；禁止路径 |
| 端口 | `17880` | 整数 1–65535 |

**测试连接：** `GET {agent_base}/local/v1/health` → HTTP 200 视为可达（公开路由，见 `auth_middleware.go`）。

**首次启动：** 无 config 或 `agent_base` 无效 → 必须先过设置 UI，再开主 WebView2。

---

## 7. 技术选型（P0 定法）

| 项 | 定法 |
|----|------|
| 宿主 | **Microsoft WebView2**（Win10+；Win11 通常已带 Runtime） |
| Go 绑定 | **`github.com/jchv/go-webview2`**（实现阶段若 ABI 问题可换等价 WebView2 封装，但保持「Go + WebView2、无 Electron」） |
| Client 设置 UI | **原生 Win32 对话框**（`syscall`/轻量 UI）；不另起 HTTP 服务 |
| 窗口单例 | 同一进程内第二次「开票」→ **聚焦已有窗口**，不叠多个主窗 |
| 失败回退 | WebView2 初始化失败 → 记录 `agent.log` / Client 日志 → 提示安装 [WebView2 Runtime](https://developer.microsoft.com/microsoft-edge/webview2/) |

**Client 安装器：** 检测 WebView2；缺失时可选运行 Evergreen Bootstrapper（Inno `[Run]` 或文档指引，实现时二选一，**P0 至少文档 + 明确错误提示**）。

---

## 8. 代码结构

```text
apps/fiscal-agent/
  internal/fiscalwebview/          # 新建：Open(opts), 单例, webview2 生命周期
    webview_windows.go             # //go:build windows
    stub.go                        # !windows no-op / 测试用
  cmd/fiscal-client/
    main.go                        # Client 入口：读 config → 设置 UI → Open
  client_config.go                 # Client config 读写（仅 Client 编译或 build tag）
  fiscal_open_windows.go           # openFiscalUI(baseURL) — Agent 托盘调用
  agent_entry_windows.go           # mFiscal → openFiscalUI（替换 openBrowser）
  setup_browser_windows.go         # 保留；configure/pair 等仍可用浏览器
  installer/
    farvoo-fiscal-client.iss       # 新建
  scripts/build-release.ps1        # 增 Client build + Client ISCC
```

**依赖：** `go.mod` 增加 `go-webview2`（仅 Windows build 链拉取）。

---

## 9. 修改范围清单

### 9.1 必改（P0）

| 区域 | 文件 / 动作 |
|------|-------------|
| WebView2 模块 | `internal/fiscalwebview/*` |
| Agent 托盘 | `agent_entry_windows.go`、`fiscal_open_windows.go` |
| Client 二进制 | `cmd/fiscal-client/main.go`、`client_config*.go`、Client 设置 UI |
| 构建 | `scripts/build-release.ps1` → 产出 `FarvooFiscalClient.exe` |
| Client 安装 | `installer/farvoo-fiscal-client.iss`、`installer/CLIENT-README.txt` |
| CI | `.github/workflows/fiscal-agent-release.yml` → Release 附 Client Setup/zip/SHA256 |
| 版本 | `RELEASE_NOTES.md`、安装说明 |
| 文档 | 本文定稿；§3.8 补 Client 一句 |

### 9.2 不改（P0）

| 区域 | 原因 |
|------|------|
| `internal/fiscal/bootstrap/admin/*` | UI 已满足；壳只加载 |
| Fiscal API / 会话 / 终端配对 | 已有 §3.8.1 |
| `17892` configure 流程 | 仍浏览器 / 现有本地 HTTP |
| `openBrowser` 非开票路径 | pair、文档链接等保留 |
| Agent 安装器 | P0 增「Farvoo 开票」桌面快捷方式（`fiscal` 子命令，默认勾选）+ WebView2 bootstrapper（默认勾选） |

### 9.3 可选（P1，不阻塞 P0）

| 项 | 说明 |
|----|------|
| ~~`FarvooFiscalAgent.exe fiscal`~~ | **已做（0.4.58）** |
| Dashboard 下载 Client + 预填 IP | Ops 生成带 `agent_base` 的配置片段 |
| Client 系统托盘 | 最小化到托盘、右键改 IP |
| ~~Inno 捆绑 WebView2 Bootstrapper~~ | **已做（0.4.58）** Agent + Client 安装器默认勾选 |

---

## 10. 验收

1. **Agent 本机：** 托盘「开票」→ WebView2 窗 → PIN 登录 → 工作台；无 Chrome 地址栏。  
2. **LAN：** 另一 PC 装 Client，设置 Agent IP，测试连接成功 → 登录 → 终端配对（若 Ops 要求）→ 开票。  
3. **改 IP：** Client 设置改 IP 后重启 Client → 连新地址。  
4. **会话：** Client 关闭再开，Cookie 有效则仍登录（直至过期 / `forceLogout`）。  
5. **回归：** `go test ./...` 全绿；Windows CI build Agent + Client + 双 Setup。  
6. **未装 WebView2 的 Win10：** 明确错误提示，不 silent fail。

---

## 11. 运维话术（引用）

1. Agent 机：Ops 开 `FISCAL_ALLOW_LAN=1`，固定店网 IP，防火墙 **17880**。  
2. LAN PC：安装 **Farvoo Fiscal Client**（不是 Agent）→ 填 Agent IP → 开票。  
3. Agent IP 变更：各 Client **设置里改 IP**。

---

## 12. 修订记录

| 日期 | 变更 |
|------|------|
| 2026-09-02 | **定稿落地（0.4.58）**：WebView2 壳、Client、IPC、`fiscal` 子命令、安装器 |
