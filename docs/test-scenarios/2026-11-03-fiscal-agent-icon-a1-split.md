# 本次改动业务场景测试报告：Agent / 开票启动图标分标（A1）

日期：2026-11-03  
分支：`feat/fiscal-agent-icon-a1-split`  
仓库：`farvoo-fatura`  
方式：按已确认最佳实践终点落地（Agent 橙点 F；开票 A1 折角单据）。

## 1. 本次修改

- **内容：** 拆两套 Windows `.ico`：Agent 壳保持 F+橙点；开票 / Client / WebView 任务栏用 A1（F+折角单据）。Agent `rsrc` 嵌两套（id1=agent，id2=fiscal）；Client 只嵌 fiscal；Inno 分 `SetupIconFile`，Agent「Farvoo 开票」快捷方式 `IconFilename=fiscal_app_icon.ico`。
- **文件：** `scripts/gen-app-icon.py`、`scripts/embed-app-icon.sh`、`assets/agent_app_icon.ico`、`assets/fiscal_app_icon.ico`、预览 PNG、`rsrc_windows_*.syso`、`cmd/fiscal-client/rsrc_…`、`internal/fiscalwebview/options.go`、`open_windows.go`、`fiscal_shell_windows.go`、两个 `.iss`、`installer_iss_test.go`、`app_icons_test.go`、`icon_id_test.go`、`fiscal_shell_open_test.go`、`docs/fiscal-client-webview.zh.md`、`README.md`、本报告。
- **终点形状：** 桌面/开始菜单/安装包/EXE/开票窗按角色分标；托盘状态圆点不变。
- **权威依据：** `docs/fiscal-client-webview.zh.md` §窗口图标（2026-11-03 分标定法）。
- **唯一写法：**
  - 资源：仅 `agent_app_icon.ico` + `fiscal_app_icon.ico`（`embed-app-icon.sh` 再生）
  - Agent WebView：`UseAgentFiscalWindowIcon()` → id `AgentFiscalIconID`（2）
  - Client：默认 id `ClientFiscalIconID`（1）
  - Inno 开票快捷方式：仅 `IconFilename: "{app}\fiscal_app_icon.ico"`

## 2. 核心业务规则

1. 【本次】Agent 与开票启动标不得共用同一视觉稿。
2. 【本次】开票标固定为 A1 折角单据变体。
3. 【已实现】托盘 `tray_icon*.ico` 仍为状态色点，不并入启动标。

## 3. 详细测试场景

### 场景 1：两套 ICO 存在且内容不同
- **结果：【已验证】pass** — `TestAppIconAssetsSoleSplit`

### 场景 2：embed 脚本 Agent=agent,fiscal；Client=fiscal only
- **结果：【已验证】pass** — 同上

### 场景 3：Inno SetupIconFile + 开票 IconFilename
- **结果：【已验证】pass** — `TestAgentInstallerFiscalShortcut` / Client installer 测试

### 场景 4：Agent UseAgentFiscalWindowIcon → id 2
- **结果：【已验证】pass** — `TestUseAgentFiscalWindowIcon`、`TestSoleFiscalShellOpenPath`

### 场景 5：Windows 真机桌面 Agent vs 开票 vs Client 图标可辨
- **结果：【未验证】** — 需 Windows 安装/构建后目视（本机为 macOS 开发环境）

### 场景 6：开票 WebView 任务栏为 A1 而非 Agent 橙点
- **结果：【未验证】** — 需 Windows WebView2 真跑

## 4. mesa-local-product-test

| 项 | 结果 |
|----|------|
| mesa-local-product-test | N/A · 【未验证】（壳层图标，不经 Mesa `:3000`） |
| 替代验收 | `GOMODCACHE=$HOME/go/pkg/mod go test . -run 'TestAppIcon|TestAgentInstaller|TestClientInstaller|TestSoleFiscalShell' -count=1` + `go test ./internal/fiscalwebview/ -count=1` |

## 5. 实际测试结果

| 项 | 结果 |
|----|------|
| 资源 / embed / Inno / IconId 单测 | pass |
| fiscalwebview 包 | pass |
| Windows 真界面图标 | 【未验证】 |

## 6. 风险与待确认

- 须跑 `./scripts/embed-app-icon.sh` 后再交叉编译 Windows；已在本分支执行并更新 `.syso`。
- 旧安装若缓存快捷方式图标，升级后可能需重建快捷方式或清图标缓存。
- 待用户说 **commit / merge** 后再合入 local `main`。
