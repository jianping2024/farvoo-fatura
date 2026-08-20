# Print / Fiscal Agent — 继承经验与禁止项

> 来源：`restaurant-ordering/apps/print-agent` 实战踩坑（RELEASE_NOTES + 代码约束）+ Fiscal 合规定稿。  
> **本仓是打印模块主线**；发票签发后也走本进程出纸。改代码前先扫一遍本文件。

---

## 0. 总原则

1. **一个进程**：业务热敏 + 税务小票同一 Agent；不另起第二套打印服务。  
2. **权威分离**：云端队列 = 业务票；本地 SQLite 队列 = 正式发票。税务编号/Hash/ATCUD **永不**由云端分配。  
3. **已验证路径不要乱改**：中文位图、安装升级、托盘退出等已有回归测试与门店验证，改动必须带测试。  
4. **用测试锁死禁区**：安装器 `AppMutex`、GS v 0 尾随 LF、桌号 UUID 上等，已有 `*_test.go` 会 `Fatal`。

---

## 1. Windows 安装 / 升级（高优先级）

| 禁止 | 原因 | 正确做法 |
|------|------|----------|
| Inno `AppMutex=` | 升级弹「请先关闭再 OK/Cancel」 | Mutex **只**在 Go 托盘防第二实例 |
| `CloseApplications=yes/force` | 问用户是否关应用 | 不要 |
| `PrivilegesRequired=lowest` | 升级装成「新装」、卸载项错乱 | 固定 `admin` |
| 让用户先手动退出再装 | 体验差、易失败 | `PrepareToInstall` 静默 `taskkill /F /IM <exe>` |
| 换升级故事却不改测试 | 旧坑回潮 | 保持 `installer_iss_test.go` 约束 |

仍需要：`UsePreviousAppDir` + `Flags: ignoreversion restartreplace`。

**本仓定法（Farvoo Fiscal Agent）：**

| 项 | 值 |
|----|-----|
| 显示名 / exe | `Farvoo Fiscal Agent` / `FarvooFiscalAgent.exe` |
| Inno 脚本 | `installer/farvoo-fiscal-agent.iss` |
| AppId GUID | 保持 `A3B8F2E1-…`（与旧 Mesa 卸载项同一键，原地升级） |
| taskkill | 先 `FarvooFiscalAgent.exe`，再迁移窗口 `MesaPrintAgent.exe` |
| 发布 | tag `fiscal-agent-v*` → `.github/workflows/fiscal-agent-release.yml` |
| 下载源 | `jianping2024/farvoo-fatura` Releases（Dashboard 改链另仓） |

---

## 2. 托盘 / 单实例 / 退出

| 经验 | 定法 |
|------|------|
| 单实例 | `agentMutexName` 仅防第二进程；**不**进 Inno AppMutex |
| 退出 | `systray.Quit` **禁止**在菜单回调同线程同步调用（会卡住退不出）→ `go systray.Quit()` + 超时 `os.Exit` |
| 重启 | 先 `spawn` 成功再停当前进程；spawn 失败则保持原进程 |
| Connected 重配 | 用 `rebindTrayAgentWork`（不杀托盘 / 本地 HTTP）；菜单「重启」才整进程重启 |
| Realtime→polling 恢复 | **唯一**自动路径 = 队列打空且至少一单成功后托盘重启；**禁止**进程内再挂第二套 notifier 切换；5 分钟冷却防死循环 |

---

## 3. ESC/POS / 中文位图（门店已验证）

| 禁止 | 原因 |
|------|------|
| 画布宽度 **384** | POS-80 应用 **576**（48×12）；384 会废纸/对不齐 |
| `GS v 0` 后尾随 **LF** | 图高已走纸 → 双倍行距废纸 |
| 中文行 `wrapDisplay` → 再 `escposBitmapText` 二次折行 | Qty/备注错位；唯一路径走 Han canvas |
| 预结含汉字垫 48 列再 wrap | 废纸；用 `escposHanReceiptRow` / `escposHanPadRow` |
| 位图截断加省略号 | 只折行不截断 |
| Windows GDI 画完再整帧刷白 | 打出空白光栅；先清白 → `TextOutW` → 采样 |
| 用 GBK 整票当默认中文路径 | 已改位图；无 GBK 出纸主路径 |
| 桌号打 `table_id` UUID | 纸面只用 `display_name` |

税务小票复用同一套 ESC/POS / Han 能力时：**QR 走原生指令，不要把 QR 渲成低分辨率位图。**

---

## 4. 打印队列 / 云端交互

| 经验 | 定法 |
|------|------|
| claim-on-fetch | 服务端已 `processing` 时，Agent **不要**对非 pending 再 PATCH `processing` |
| 旧任务 | pending 过久（约 10 分钟）不要重放下厨；靠服务端 fail + 人工 Retry |
| Realtime URL | 同 host 时 `api_base` / `supabase_url` scheme 对齐（避免 `ws://` bad handshake） |
| Token 临近过期 | Realtime 重连前**必定** refresh，不要因 skew 跳过 |
| 营业时段配置 | 启动时拉一次；热改需重启（文档写清楚） |
| WinSpool 预检 | **不要**用 `PRINTER_STATUS_*` 一类预检挡打印（见原 flow 文档约束） |

**发票路径额外：**

- 正式票 **禁止**写入云端 `print_jobs` 当权威队列  
- 签发与 ORIGINAL 任务同 SQLite 事务；打印失败不回滚编号  
- 重打 = 新 job + 冻结 payload，**不重签**

---

## 5. 配对 / 本地 HTTP UX

| 经验 | 定法 |
|------|------|
| 配对成功 | `location.replace` 直达 `/configure`，别停在成功面板让人以为失败 |
| 一次性码 | 成功后清码并短暂禁用「连接并保存」，防刷新复用 |
| 重配后 Realtime | Connected 再配对需重建会话，避免旧 Realtime + 新 `api_base` 分裂 |
| 档口冲突 | 提供「接管并保存」，不要逼用户误吊设备 |
| 托盘卸载 | 认 Inno `{GUID}}_is1`；`ShellExecute` 拉起 unins，不要 `cmd /C` HideWindow |

---

## 6. Fiscal / 数据（本仓新增，必须继承合规坑）

| 禁止 | 原因 |
|------|------|
| 金额用 `float` / SQLite `REAL` | 舍入与 Hash/SAF-T 不一致 → 一律 decimal **字符串** |
| SAF-T 用 UTF-8 却声明 Windows-1252（或带 BOM） | AT 静默拒收 |
| 签名后再改金额/客户/Hash | 违法；只能 NC |
| 签名算法升级成 SHA-256 | AT 要求 RSA-SHA1 |
| 中文进 SAF-T `ProductDescription` | 必须葡/英 `saft_name`，签前 Windows-1252 校验 |
| Agent 自建开票账号体系 | 操作员从 Farvoo 同步 + 本地 PIN |
| Fiscal Core 读库存/结账表 | 只吃 `SaleSnapshot` |

---

## 7. 回归护栏（改相关代码必跑）

```text
apps/fiscal-agent/
  installer_iss_test.go          # 无 AppMutex / CloseApplications yes
  escpos_bitmap_text_test.go     # 576、无尾随 LF、不截断
  escpos_receipt_test.go         # 不打 UUID、Han 路径
  promote_restart_test.go        # Realtime 恢复唯一路径 + 冷却
  tray_uninstall_test.go         # 卸载注册表键形态
  single_instance_*              # Mutex 名稳定（现为 FarvooFiscal…）
```

发票落地后追加：Hash 官方样例、金额混税率、签发事务幂等、payload 冻结一致性。

---

## 8. 命名 / 同机共存

- 配置目录、Mutex、exe 名已与旧 `mesa-print-agent` **隔离**  
- 同机可临时并存；长期以本仓为打印主线后，迁移配置需显式工具，**不要**静默抢旧 Mutex  

---

## 一句话

**安装别用 AppMutex；中文位图 576 且 GS v 0 无尾随 LF；托盘退出别堵菜单线程；税务权威只在本地 SQLite；云端只打业务票副本。** 这些是原 Agent 用版本号和门店试打买来的，本仓必须继承。
