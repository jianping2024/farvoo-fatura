# Print Agent Release Notes

Each release section starts with `## X.Y.Z`. The release workflow reads the matching section and appends standard install instructions.

## 0.3.94

**收据菜品行不加粗（列头 / 应付仍加粗）**

- `writeReceiptMenuBodyLine`：列头 `bold=true`，菜品行 `bold=false`（Han 位图与拉丁 `ESC E` 一致）；避免含汉字菜名比拉丁行更黑。
- 应付等其它票面加粗不变。

## 0.3.93

**FT 脚对齐样票：认证在 QR 上、TOTAL 突出、无 Hash 行**

- `Fatura No.:`；TOTAL 加粗倍高；认证=`{四字}-Processado…`（无 `Hash:`）；ATCUD+QR 居中 module=6；QR 下只留白+`GS V 66` 切。
- 依据 VOZ/Pingo 样票原文 + 0.3.92 真机横切。见 `docs/fiscal-ft-receipt-layout.zh.md`。

## 0.3.92

**FT 票面：居中抬头 + 单行明细**

- 抬头 `ESC a 1`（法定名加粗）后回左；明细表头 + 单行 `qtyx price vat%-name … soma`（废两行品名模式）。
- 店名仍仅 `merchant.legal_name`；唯一 `RenderESCPOS`。见 `docs/fiscal-ft-receipt-layout.zh.md`。

## 0.3.91

**FT 票面 P1：1252 编码、列宽 32、QR 居中缩小、脚留白**

- 拉丁编码唯一 `internal/escposenc.Windows1252` + `ESC t 16`（厨打复用）；禁 UTF-8 直出。
- 列宽 32 + rune 计宽，避免付款行孤儿金额；联次 `1a Via - Original`；QR 居中 module=4，认证/Hash 后留白再切。
- 权威：`docs/fiscal-ft-receipt-layout.zh.md`。

## 0.3.90

**FT 热敏票面按零售参考重整（唯一 RenderESCPOS）**

- 抬头 → 票号/日期/联次 → 客户 → 明细（税率%）→ 合计+付款 → Resumo IVA → ATCUD/QR/认证/Hash。
- 禁止 `FT: FT FT…` 重复前缀；出纸仍 `printToTarget`。见 `docs/fiscal-ft-receipt-layout.zh.md`。

## 0.3.89

**税票 FT 出纸复用档口 `printToTarget`（TCP/USB）**

- 开票必填 `station_id`；`local_print_jobs.station_id`；worker 经注入 `PrintBytesFn`→`parsePrinterTarget`+`printToTarget`（与厨打同一出口）。
- `GET /local/v1/printers`；Admin §7 档口下拉 + 链 `17892/configure`；去掉 fiscal 专用 TCPSink / Memory 假成功生产路径。

## 0.3.88

**Admin 反馈对齐 restaurant Toast**

- 唯一组件 `FiscalUI.showToast`（`/fiscal-ui/toast.js`）；Admin 保存/开票成败右下角 toast，禁止页内再造 banner。
- 测试锁死 Admin 不得内联第二套 `showToast`。

## 0.3.87

**M2.5 草稿工作台：整桌/按人 + NIF**

- `POST .../issue`：`mode`/`scope_id`/`customer_nif`；按人 `DraftPartToSaleSnapshot`；互斥 `scope_mutex`；按人开完才硬删草稿。
- `GET .../bill-drafts/{id}` 已开标记；`POST .../discard`；签票成功不因清草稿失败失败（`cleanup_pending`）。
- Admin §7 演进为工作台（每人独立 NIF）。

## 0.3.86

**账单同步进本地草稿（不自动开票）+ 整桌从草稿开 FT**

- 打印 Realtime/Polling 同管道加订 `bill_sync_jobs` → `pending-bill-syncs` → 本地 `bill_sync_drafts` + 按 `item_code` upsert 商品，再 ack。
- Admin §7：open 草稿一键整桌开 FT（散客+全额现金）；`POST /local/v1/bill-drafts/{id}/issue`；映射唯一 `DraftToSaleSnapshot` → `IssueFromBillDraft` → `IssueDocument`。
- 开票成功 **硬删** 该 `source_sale_id` 全部草稿；再同步靠税务库 `HasSignedFTForSale` → `already_invoiced`（不保留 `invoiced` 行）。
- `vat_rate` 同步用百分数串（如 `"13.00"`）；开票映射为小数串。`split` 草稿本刀拒绝开票。

## 0.3.85

**本机激活默认开：不用再设系统环境变量**

- Agent 启动时默认 `fiscal_allow_local_provision` / AT mock；可用 `config.json` 改，env 仍可覆盖。
- 装完托盘即可走 Admin 激活开票流程。

## 0.3.84

**产品身份收口：Farvoo Fiscal Agent 打包 / 安装 / 发布**

- 产物与 Inno：`FarvooFiscalAgent.exe`、`FarvooFiscalAgent-Setup-amd64.exe`、`installer/farvoo-fiscal-agent.iss`（AppId GUID 不变）。
- 升级：`PrepareToInstall` taskkill 新 exe + 迁移窗口 `MesaPrintAgent.exe`；无 AppMutex。
- Windows 数据目录：`%LOCALAPPDATA%\Farvoo Fiscal Agent\`；配置仍在 `.config/farvoo-fiscal-agent`。
- 本仓 CI：tag `fiscal-agent-v*` → `.github/workflows/fiscal-agent-release.yml`。

## 0.3.83

**Realtime 断网回落：本批打完后整进程重启再试推送；控制台/状态字色统一**

- Realtime→polling fallback 后，本批至少一单打印成功且本地队列打空 → **唯一**自动恢复路径：托盘 `requestTrayRestart`（与菜单「重启」同逻辑，不弹确认）；5 分钟冷却防重启死循环。
- 托盘状态首行不再 `Disable`（避免发灰）；调试控制台统一亮白字；设置页 `.status` 与正文同色。

## 0.3.82

**出品联固定壳跟打印语言（与预结同一规则）**

- 唯一入口 `printTicketLabels(locale)`：zh→中文壳，否则英文；出品联/预结/收据共用。
- 删除出品联强制 `stationTicketLabels()` 英文第二套。
- 票头/页脚 branding：`ticketBrandingWord`（zh `餐厅` / 其它 `restaurant`）。

## 0.3.81

**修复：中文位图废纸（档口行距 + 预结单拆行）**

- GS v 0 **不再尾随 LF**（图高已走纸）；收紧位图上下 padding。
- 预结/结账含汉字菜单行：唯一入口 `escposHanReceiptRow`（576 单行三栏）；合计 `escposHanPadRow`；禁止垫 48 列再 `wrapDisplay`。

## 0.3.80

**出品联：Qty 列 8→4，菜名/备注可贴得更近数字**

- 唯一列宽 `stationSlipQtyColWidth = 4`（仍右对齐贴右 4 列边距内侧）；正文折行右界随 `stationSlipQtyColStart` 自动右移。
- 菜名「列宽够则一行」、576 画布、备注跟语言：不变。

## 0.3.79

**修复：POS-80 576 满纸 + 备注跟打印语言 + 取消前缀半号**

- Han/GS v 0 画布 `bitmapTextMaxWidthPx = 576`（48×12）；禁止 384。
- 备注标签唯一来源 `labelsFor(locale).itemNote`（zh `备注: ` / en `Note: ` / pt `Observação: `）；删除死常量。
- 前缀与正文同字号；删除 `hanNotePrefixFontPx` / `escposHanNoteRow` / `renderBitmapNoteRow`。
- 菜名折行：保持 Font A 列宽闸门（不回退）。

## 0.3.78

**修复：站票左右边距对称 + 备注首行可写满**

- 菜名、备注左缘统一为 `Items` 表头左缘（col `stationSlipSideMargin` = 4）；删除 col 5 菜名 / col 1 备注第二套左距。
- 拉丁 Qty 表头/数量在 8 列 field 内 **右对齐**（`padFieldRight`），与 Han `hanQtyTextStartPx` 一致。
- 备注 `wrapHanNoteLines`：首行先扣前缀宽再折正文，避免 font 34 下「Observação:」占满首行只剩两字。

## 0.3.77

**修复：Qty 贴右 + 中文备注并入 Han canvas（唯一标尺）**

- Qty 表头/行内数量在 8 列 field 内 **右对齐**（`hanQtyTextStartPx`），不再 band 内居中留大块右侧空白。
- 备注与菜名同 canvas：`wrapHanNoteLines` + `escposHanLeftRow`；**禁止** `wrapDisplay` → `escposBitmapText` 二次折行。
- 续行 hanging indent；`wrapHanTextByPx` 优先在空格处断行。

## 0.3.76

**修复：中文站票 Qty 与表头同列（384px 画布 + 像素锚点）**

- 含汉字的 Items/Qty 区块：表头与菜品行同一张 384px 画布；数量 `TextOut` 钉在 Qty 列像素带，不再用空格假对齐。
- 纯拉丁行仍走 Font A 48 列；中文位图按角色字号（页脚 0.75×B / 正文 B / 标题 1.5×B）。

## 0.3.75

**修复：中文站票 Qty 掉行 / 与菜名粘在续行**

- 站票菜品行只走 `stationSlipItemLine`（按 display 列钉 Qty）；删除 `stationSlipItemBitmapLine` 拼接后再 wrap 的路径。
- 含汉字时按 `bitmapMaxDisplayCols` 折菜名；数量只在首行右侧，续行仅菜名。

## 0.3.74

**中文位图字号可配置（功能设置 → payload）**

- 读 `print_jobs.payload.han_bitmap_font_px`（缺省/越界回退 24）；折行列宽随字号变。
- 店级配置在功能设置「打印助手」；下一张云端打印任务即生效。

## 0.3.73

**中文菜单位图：固定 24px + 只折行不截断；清理 GBK 误导命名**

- Han 字号默认 `bitmapTextDefaultFontPx=24`（`DoubleH`/`DoubleW` 不再加倍 TrueType）。
- `escposBitmapText` / 出品联菜名与备注：`wrapDisplay` 折行，不截断；菜名续行无 Qty，首行保留数量。
- `localeUsesGBK` → `printLocaleIsZh`；删除 `text_encoding_gbk` 文案；无 GBK 出纸路径。

## 0.3.72

**回退：恢复 0.3.67 中文位图出纸（店端已验证可读）**

- print-agent 源码整体回退到 `7f9c1126`（当时 VERSION 为 0.3.67）：含「先清白再 TextOutW」位图、试打中英葡；不含 0.3.68+ 格子/`ESC J`/强制 FontPx/拉丁半行混打等后续改动。
- 本版号 **0.3.72**（不可复用已发的 0.3.69–0.3.71）；行为对齐本地已验证的 0.3.67 包。

## 0.3.67

**配对/设置页：试打可选中英葡**

- configure / setup 试打增加「试打语言」三选一（zh / en / pt），与托盘界面语言无关。
- `/api/test-print` 接受 `locale`，纸面标签与编码路径跟所选语言走。

**修复：中文位图试打空白**

- Windows GDI 渲染改为先清白再 `TextOutW` 再采样；画完后不再整帧刷白（此前会打出空白光栅）。

## 0.3.66

**隐藏托盘卸载入口**

- 托盘右键菜单不再显示卸载按钮。
- 安装器/Windows 系统卸载路径保留。

## 0.3.65

**中文票面改为位图输出**

- 默认中文打印不再依赖整票 GBK 模式，中文行转为 ESC/POS raster bitmap。
- 显式 UTF-8 配置继续保留；旧 GBK 配置按自动位图处理。
- 出品联、结账小票、连接测试统一走同一中文输出策略。

## 0.3.63

**档口冲突：向导内「接管并保存」；凭证失效提示重配**

- 云端 `POST /api/print-agent/routing` 支持 `force_takeover`：从其他活跃设备摘掉冲突档口再写入本机（不必误吊刚配对设备）。
- Setup/Configure：冲突时显示「接管并保存」；401/吊销失效时提示重新配对。
- Dashboard「已配对收银机」：无映射标「尚未映射档口」；吊销未映射设备时加强确认文案。

## 0.3.62

**托盘卸载：找对 Inno 卸载项并真正拉起 unins000**

- 注册表键认 Inno 实际写入的 `{GUID}}_is1`（外加 DisplayName 前缀匹配、exe 旁 `unins000.exe` 兜底）。
- 启动卸载器：解析 UninstallString → `ShellExecute`（不再 `cmd /C` + HideWindow）。
- Setup：`AppVerName` / `UninstallDisplayName` 固定为产品名，避免 DisplayName 带 `version X`。

## 0.3.61

**Cloud claim Realtime URL；重配热切换；托盘一键卸载**

- Connected 重配：进程内 `rebindTrayAgentWork`（不杀托盘 / `:17892`）；菜单「重启」仍整进程重启。
- 托盘新增「卸载…」：清除本机配置与日志，并拉起 Setup 卸载器（便携版仅清数据并提示手删）。
- 须配合 Web：cloud claim 的 `supabase_url` 固定 `getPublishedSupabaseUrl()`（`*.supabase.co`），Mode B 仍可优先 `api_base`。

## 0.3.64

**Setup 覆盖升级：运行中直接装，不要先退出 / yes-no**

- 去掉 `AppMutex`（会挡在「请先关闭再 OK/Cancel」）。
- 去掉 `CloseApplications=yes/force`（会问是否关闭应用）。
- 唯一关托盘路径：`PrepareToInstall` 静默 `taskkill /F /IM MesaPrintAgent.exe`；仍 `PrivilegesRequired=admin` + `UsePreviousAppDir` + `restartreplace`。
- 托盘 `agentMutexName` 只防第二进程启动，不参与 Setup。

## 0.3.60

**Setup 覆盖升级（管理员 + 关闭运行中进程）**

- Inno：`PrivilegesRequired=admin`（匹配 Program Files / HKLM 卸载项，升级而非“新装”）。
- （已被 0.3.64 取代）曾用 `AppMutex` + `CloseApplications`；exe 用 `restartreplace`；`UsePreviousAppDir=yes`。
- 向导前提示会请求管理员权限并在替换前关闭 `MesaPrintAgent.exe`。

## 0.3.59

**配对成功直接进入打印机设置**

- `/api/pair` 成功后浏览器唯一出口：`location.replace` 到 `/configure`（不再停在成功面板/可选链接，避免误以为失败）。
- Connected 重配触发的托盘重启延迟约 2s，先让跳转落地再杀本机 HTTP。

## 0.3.58

**配对成功后禁用「连接并保存」10 秒**

- 成功后清掉 URL/输入框里的配对码，避免刷新或连点复用一次性码。

## 0.3.57

**Realtime：`supabase_url` 跟 `api_base` 同 host 时对齐 scheme**

- claim 上报 `api_base`；服务端以该 origin 写 `supabase_url`（避免 Tunnel 把 proto 弄成 http → `ws://` bad handshake）。
- 启动/配对落盘：`alignSupabaseURLWithAPIBase` 纠正已有错误配置（同 host 的 http→https）；云端不同 host（Vercel vs `*.supabase.co`）不改。

## 0.3.56

**claim-on-fetch；JWT renew 强制 refresh**

- `GET pending-jobs` 服务端已 claim-on-fetch 时，agent 不再对已非 `pending` 的任务重复 PATCH `processing`；仍兼容旧服务端（`pending`/空状态时照旧 PATCH）。
- Realtime 因 token 临近过期退出后，重连前**必定** refresh（不再因仍在 skew 内跳过），避免同秒连续 renew。
- `connect` 分步日志：ensuring token → dial → subscribe → connected。

## 0.3.55

**重配后重建 Realtime；店内 CDC 含 print_jobs**

- Connected 后再配对成功会自动重启托盘进程，避免旧 Realtime 会话 + 新 `api_base` PATCH 分裂（`job_not_found`）。
- 首次未配对配对仍走原 bootstrap，不额外重启。
- 配合 Web：Mode B claim 的 `supabase_url` 跟请求边沿 origin；on-prem ensure 将 `print_jobs` 加入 `supabase_realtime`。

## 0.3.54

**未配对即可用本机 17892 配对页**

- 托盘启动即监听 `127.0.0.1:17892`（`/pair`、`/configure`、`/api/health`），不再等 Connected 后才起本地 HTTP。
- 未配对 bootstrap 优先打开同一端口的 `/pair` 并等 JWT 落盘；托盘路径不再并行起 17890。
- Dashboard「在本机打开设置」与托盘「打印机设置」未配对时也可 probe/打开；CLI `pair` 仍可在无托盘时用 17890。

## 0.3.53

**启动排障日志；撤回定时补偿与启动总超时**

- 保留启动阶段日志（拉配置 / 同步档口 + 耗时）、托盘 `ready (UI only)` vs `Connected`、入队等待时长与打印耗时，便于分辨网络卡住。
- 撤回 Realtime 健康时定时拉 `pending-jobs`、开门 reconcile，以及启动路径统一 HTTP 总超时；启动恢复行为与线上一致（网通后同一次请求可成功）。
- Token 刷新仍使用原有短超时；仅启动/连接/重连等原有时机补拉 pending。

## 0.3.51

**打印代理通知模式可观测性（Realtime / Polling）**
- Heartbeat 上报并写入 `print_agent_devices.notification_mode`，当 Realtime 不可用/失败后自动降级到 Polling 时也会同步体现。
- 后台 `PrintAgentDevicesPanel` 增加“运行方式”展示每台设备当前实际使用的 notifier 模式。

## 0.3.50

**清理死路径；本地队列生命周期收口**

- 删除不可达的单体 `runPollLoop` 及仅为其服务的队首轮转辅助。
- 正式路径统一为 Notifier + JobProcessor；暂不可打 / claim 失败用 `Requeue`，终态与 Retry 用 `Forget`。
- Realtime 推送、补偿拉取、Polling 共用同一 `jobEligibleForQueue` 入队规则。

## 0.3.49

**省流：云端配置只在启动拉取；心跳 5 分钟**

- 运行中不再定时请求 `runtime-config`；改 Dashboard 营业时段/轮询后需托盘「重启」。
- 设备心跳改为每 5 分钟；Dashboard 约 10 分钟无心跳视为离线。
- 营业时间闸门仍每 15 秒在本地判定（不上网）。

## 0.3.48

**营业时间支持跨午夜**

- 窗口仍为 `{start,end}`：若结束钟点早于开始（如 `19:30–02:00`），按跨天半开区间判定。
- 与 Dashboard 保存规则对齐；开始与结束相同仍非法。

## 0.3.47

**营业时间闸门与托盘状态对齐**

- 关店时统一黄灯「非营业时间」，空闲 Ready 不再盖成绿灯。
- 关店清空待打印队列并清除去重，开店后同一任务可重新入队。
- 定时热更新云端营业时间/轮询配置，改时段无需手动重启 agent。
- Polling / Realtime 入队与打印前均过同一 `scheduleOpen` 闸门。

## 0.3.46

**Realtime：JWT 到期前主动续期**

- 连接与订阅前按 `exp` 提前刷新 session，减少中途断线。
- JWT 过期辅助函数与测试去重，行为与 supabase-js 对齐。

## 0.3.45

**Realtime session 契约对齐 GoTrue / supabase-js**

- Auth refresh 使用官方 JSON body（修复生产 `bad_json`）。
- 连接前按 JWT `exp` 决定是否 refresh；刷新失败不再假装成功，走既有 Realtime→Polling 回退。
- WebSocket 握手只带 URL `apikey`；用户身份仅在 subscribe 的 `access_token`。
- 已配对时「打印机设置」露出「重新配对」→ 同一 `/pair` 页。

## 0.3.44

**Realtime 使用店内 print_agent 员工 session**

- 配对 claim 若返回 `access_token` / `refresh_token` / `anon_key`，默认用 Realtime 听本店 `print_jobs`；失败自动回退 polling（兼容仅有 `agentjwt` 的旧配置）。
- 控制台与托盘显示当前模式（Realtime / Polling）；打烊时段与 polling 一样不入队、不出票。
- 需配合 Web：系统 `print_agent` 员工、claim 签发 session、print_jobs RLS。单独升级 Agent 而无 Web/migration 时仍走 polling。

## 0.3.43

**出品联备注折行**

- 菜品备注（`Observação:`）超长时按 Items 区宽度折行完整打印，不再截断加省略号；续行不进入 Qty 列。
- 业务侧单条备注上限仍由 Web `APPEND_CART_NOTE_MAX_LEN`（120）约束。

## 0.3.42

**账单菜单区排版**

- 列头（Items / Qty / Pri）、菜品行、Amount Due 统一 Font A 1×2 加粗；英文价格列头由 Original Price 改为 Pri。
- 费用明细与时间戳仍为 1×1 普通；出品联不受影响。

## 0.3.41

**出品联 / 账单纸面版式**

- 出品联：Items 表头左缩进 4（对齐 Guest 的 t），菜品行再缩进 1；Qty 列内居中；左右边距对称。
- 账单：菜品行改为 1×2 字号（表头/费用/时间戳不变）；不印备注行。
- 需配合 Web 端：账单菜品标签与出品联一致（`{编号}-{菜名}`，无类别前缀）。

## 0.3.40

**出品联（Guest Order）版式**

- 菜品区改为 1×2 字号；列模型左/右各缩进 1，Qty 列头偏左对齐。
- 菜品行仅 `{编号}-{菜名}`；可选居中分类分组行由 Web 配置开关控制（默认关）。
- 出品联固定 Windows-1252（葡语/英语重音）；页脚 `Printed By:restaurant`。

## 0.3.39

**换店重新配对**

- 配对成功后持久化 `restaurant_id`；切换到另一家餐厅重新配对时，自动清空旧店的档口打印机映射，引导重新配置。
- 需配合 Web 端 claim 设备转移（同次发布）；单独升级 Agent 无法解决换店 409 冲突。

## 0.3.38

**预结/结账小票：表头与虚线间距**

- 去掉虚线与 `Items` 表头之间多余的一行空白（v0.3.36 误加）；菜品行之间的间距不变。

## 0.3.37

**预结/结账小票：自助餐 Qty 列**

- Qty 列加宽至 9 字符并居中，支持 `A4-C2`、`A9`、`C3` 等自助餐人数标签（由 Web 端 `share_qty_label` 下发）。

## 0.3.36

**预结/结账小票：菜品行间距**

- 表头 `Items` 与上方虚线之间增加一行空白。
- 每道菜品（含备注后）与下一行之间增加同等空白，便于核对长编号行。

## 0.3.35

**托盘：重启**

- 托盘右键菜单在「退出」上方新增 **重启**，可重新加载云端配置并恢复打印，无需手动退出再打开。
- 更新相关提示文案（已运行、首次启动等）。

## 0.3.34

**安装器：可选桌面快捷方式**

- Inno Setup「Select Additional Tasks」页新增 **Desktop shortcut**（默认不勾选），与 **Sign-in startup** 并列展示，便于用户按需勾选。
- 卸载时移除安装器创建的桌面与登录启动快捷方式。

## 0.3.33

**安装器：登录自启默认不勾选**

- Inno Setup 向导「当前用户登录时启动」改为默认**不勾选**；需要登录自启的用户可在安装时手动勾选。

## 0.3.32

**备注行前加 `Observação:` 标签**

- 出品联与预结/结账：菜品备注仍单独一行、下划线，前缀固定为 `Observação: `（与备注内容同一行）。

## 0.3.31

**统一打单排版：出品联 1×1 + 备注下划线**

- 出品联菜单项改回 **Font A 1×1**（与预结单菜品密度一致），票头/桌号仍为 2×2。
- 出品联与预结/结账单：菜品 **备注** 单独一行、**下划线**（ESC/POS `ESC -`）；菜品名不加粗。
- 预结/结账此前未输出的 `note` 字段现已打印。
- 合并票头与菜品行排版逻辑，移除 2×2 菜单专用冗余代码。

## 0.3.30

**厨房单菜单行距**

- 2×2 菜单块行距略加大（ESC/POS 行距），分类/菜名/数量行不再贴得过密；页眉页脚不变。

## 0.3.29

**修复托盘「打印机设置」保存无反应**

- 托盘本地 HTTP（`:17892`）未放行 `/wizard-ui-shared.js`，导致 `MesaWizardUI` 未加载、点击保存无反应；已修复。

**厨房单菜单字号加大一档**

- 厨房单 **仅菜单项**（分类标题、菜名、数量、备注）改为 **2×2**；`restaurant`、人数、`Items/Qty` 表头、页脚等其余行保持原字号。
- 菜单块倍宽下列宽按 24 字符排版，长菜名截断略早，纸长会略增。

## 0.3.28

**支持清除全部档口映射**

- 打印机设置中可将所有出品档口置空后保存，**解除本机全部打印职责**（本机不再接收任何打印任务）。
- 清空映射**仅影响当前这台 Agent** 的云端 `routing_snapshot`，**不会覆盖或删除其他 Agent** 已占用的档口。
- 保存流程仍为先同步 MesaGo、成功后再写本地配置；试打仍须先选择档口打印机。
- 配置/setup 向导共用 `wizard_ui_shared.js`，减少重复逻辑。

## 0.3.27

**多代理打印隔离（需 MesaGo Web 与 Print Agent 同时升级）**

- **按设备过滤待打印任务**：`pending-jobs` 只返回本机 `routing_snapshot` 已订阅档口的任务，避免多台 Agent 抢同一厨房单。
- **档口映射冲突校验**：保存映射时，若某档口已被另一台 Agent 占用，云端返回 **409**，本机配置**不会**保存。
- **先同步云端、再写本地**：配置向导改为校验 → 同步 MesaGo → 成功后再写入本机 config。
- **认领二次校验**：不属于本机档口的任务无法 claim（403）。
- **配置界面**：冲突时展示具体档口与占用设备名称。
- **Dashboard**：打印助手设备列表显示已映射档口名称。

**升级注意**

- 升级后请在各 Agent 上**重新保存一次档口映射**，云端 snapshot 才会生效。
- 若之前有多台 Agent 映射重叠，保存时会提示冲突，需手动拆分到不同设备。
