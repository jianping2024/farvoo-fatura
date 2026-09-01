# Print Agent Release Notes

Each release section starts with `## X.Y.Z`. The release workflow reads the matching section and appends standard install instructions.

## 0.4.42

**重打门闩 + 撕口→店名留白**

- 重打：允许 `SIGNED` / `CREDITED_*` / `DEBITED_*`（唯一 `domain.IsReprintableDocumentStatus`）；API 错误码 `reprint_not_allowed`（不再误报 `issue_failed`）。
- 票面：正常票唯一 `receiptStreamBegin`（无 `ESC @`），减少撕口→店名固件进纸；`nil` payload 测试切刀仍用 `ESC @`。
- 回归：`fiscal-reprint-regression.mjs` 增补 ND 后重打；`TestCreateReprintPrintJobAfterND`。

## 0.4.41

**税票留白：顶 ×½、底 ×⅔（相对 0.4.40 软件进纸）**

- 店名后 `ESC J 7`（原 15 点之半）；QR 后合计 **56 点**（原 85 之 ⅔）：`writeQR` LF 30 + `GS V 66` 26。
- 常量由 v0.4.40 基线推导；单测 `TestReceiptVerticalPaddingConstants`。权威 `docs/fiscal-ft-receipt-layout.zh.md` P0 #9/#11。

## 0.4.40

**热修：店名后间距误用 ESC d（15 行）→ ESC J（15 点）**

- 0.4.39 用 `ESC d 15` 会进 **15 行**（约 6 cm 空白），非半行；改为 `ESC J 15` 点进纸。
- 单测禁止 `ESC d` + `receiptTopGapDots`；设计文 P0 #9/#11 已更正。

## 0.4.39

**Admin 发票按类型 Tab；税票上下留白减半**

- 发票列表：FT / FS / NC / ND 分 Tab；`GET /local/v1/fiscal-documents?document_type=` 服务端过滤；选中 Tab 记 `sessionStorage`。
- 税票打印：店名后 `ESC d 15`（半行进纸）；QR 后切前进纸 **85 点**（`writeQR` LF 30 + `GS V 66` 55，原 170 点之半）。权威 `docs/fiscal-ft-receipt-layout.zh.md` P0 #11。

## 0.4.38

**Admin 固定按钮：冲销/借记/已关联发票去掉 data-* 双绑风险**

- 详情 `#btnInvoiceDetailCredit` / `#btnInvoiceDetailDebit` 改用 `_creditDocId` / `_debitDocId`（不再写 `data-credit` / `data-debit`）。
- 手工开票 `#btnInvLinkedOpen` 改用 `_openOriginalDocId`（不再写 `data-open-original`）。
- NC/ND 详情正文内「原票」链接仍仅用 `#invoiceDetailBody` 委托 + `data-open-original`（动态行，无 onclick）。
- 单测：禁止固定 `#btn*` 再写会被委托匹配的 `data-*`。

## 0.4.37

**Admin 重打：去掉双重点击，详情/订单不再双打**

- 唯一 `handleReprintClick`（`withBusy` + `reprintInvoice`）；列表仍用 `data-reprint` 委托。
- 详情 `#btnInvoiceDetailReprint`、订单 `#btnOrderReprint` 改用 `_reprintDocId`，不再带 `data-reprint`，避免与委托重复触发。
- 修复：详情点重打 → 打两张 + 按钮卡在「重打中…」。

## 0.4.36

**Admin 冲销/借记按钮：进 App 即加载 setup；清单与按钮对齐**

- `ensureSetupStatus`：进入 App / 打开发票详情前保证已拉 setup（退出清空缓存）。
- `ready_to_credit` / `ready_to_debit` 含 `operator_can_issue_nc`；详情按钮唯一门闩 `invoiceCanCredit` / `invoiceCanDebit`。
- 修复：设置全绿但未进过设置页 → 借记按钮不出现。

## 0.4.35

**ND 纠正差额：取消原票金额天花板**

- `buildNDLines` 唯一构建 ND 行；金额须 >0，**可大于原行/原票**；废止累计 ≤ 原票 gross。
- 任一次 ND 后原票均为 `DEBITED_PARTIAL`（可继续借记；含历史 `DEBITED_FULL`）。
- 读模型：`DebitLinesForInvoice`（废止 `DebitRemainingForInvoice` / `remaining_debit_gross_total`）。
- Admin：借记按行无 max；详情只显示「已借记」。
- 回归：m6 / m6-product / d62 C4.2 改为允许超额。

## 0.4.34

**ND 部分借记 UI + 原票回链；手工开票已开关联**

- ND/NC 原票：唯一读 `CorrectiveOriginalForDocument`（废止 `CreditOriginalForNC`）；原票借记余额唯一读 `DebitRemainingForInvoice`。
- Admin：单一 `adjustModal` / `openAdjustModal`（NC+ND）；借记默认按行；`debitInvoice` 支持部分/全额；ND 详情原票按钮。
- 手工开票已开：唯一 `syncOrderInvoicePanel` — 显示关联票号、签发隐藏、主 CTA「重打」走 `reprintInvoice`；`openInvoiceDetail` 可按 id GET（不依赖列表缓存）。
- 回归：`fiscal-m6-product-regression.mjs`（部分 ND + ND 详情原票）；单测 `TestCorrectiveOriginalForDocument_*`。

## 0.4.33

**系列注册防重复：幂等同码 + 拒同类型新码 + Admin 唯一路径**

- `RegisterSeries`：同 ACTIVE `series_code` 幂等（不调 AT）；同类型已有不同 ACTIVE code → `series_already_active` (409)。
- Admin §3：单一 `registerSeries`；就绪清单展示系列号；已注册输入只读并禁用按钮；提示日常只需 FT+FS。
- 回归：`scripts/fiscal-series-register-regression.mjs`；单测 `TestRegisterSeries_IdempotentAndRejectSecondCode`。

## 0.4.32

**M6 D6.3 / D6.4：备份校验 + 换机最小集**

- 备份：`POST /local/v1/setup/backup`（`VACUUM INTO`）；校验：`POST .../integrity/verify`（失配将 ACTIVE→FAILED 阻断开票；可 heal）。
- 换机：`POST .../prepare-swap`（备份 + `ClearLocalActivation`）；Admin 设置 §8；设计 `docs/fiscal-m6-backup-swap.zh.md`。
- 回归：`scripts/fiscal-d63-d64-regression.mjs`；认证清单 C7.1/C7.2 可自动出具。

## 0.4.31

**M6 D6.1b：产品单据规则收口（唯一写法）**

- 默认销售单据 **FS**；产品 UI / 手工开票 / 分单仅 **FT + FS**；六种付款方式统一 `domain.NormalizePaymentMethod`。
- 账单同步挡重与已开 scope：`HasSignedSaleForSale` / `ListSignedSaleScopesForSale`（FT+FS，含 DEBITED_*）。
- NC/ND Admin：**FT + FS** 显示冲销/借记；`ready_to_issue` 须 FT **与** FS 系列齐备。
- 认证清单骨架：`docs/fiscal-certification-checklist.zh.md`；回归 `scripts/fiscal-m6-product-regression.mjs`。

## 0.4.30

**M6 D6.1：FS / FR / ND**

- `IssueDocument` 支持 `FT` / `FS` / `FR`；各自独立 ACTIVE 系列；手动开票可选类型。
- ND：`service.IssueDebitNote` → `store.IssueND`；API `POST .../debit-notes`；原票 `debited_gross_total` / `DEBITED_*` 状态。
- SAF-T 含 ND；Setup `nd_series_ok` / `fs_series_ok` / `fr_series_ok`；Admin 注册系列与全额借记按钮。
- 回归：`scripts/fiscal-m6-regression.mjs`；设计 `docs/fiscal-m6-fs-fr-nd.zh.md`。

## 0.4.29

**M5.1：SAF-T Windows-1252 校验修复**

- `validateWindows1252`：CP1252 编解码 roundtrip（修复葡语重音 `Água`、`Guarná` 等误报 INVALID）。
- 回归：`fiscal-m5-regression.mjs` 使用重音 `saft_name`；`saft/build_test.go` 增葡语 VALID + 中文 INVALID 断言。

## 0.4.28

**M5：SAF-T 月报导出**

- 唯一路径：`service.ExportSAFT` → `store.LoadSAFTInvoicesForPeriod` → `saft.Build` → `store.InsertSAFTExport`；Admin §7 `exportSAFT()`。
- API：`POST/GET /local/v1/saft/exports`、详情、下载；空月 `no_invoices`；重复导出追加新归档行。
- 同月 FT + NC 进同一 XML；NC References 与 `invoice_line_references` 一致；Windows-1252 校验。
- 回归：`scripts/fiscal-m5-regression.mjs`；设计定稿 `docs/fiscal-m5-saft.zh.md`。

## 0.4.27

**M3.1b：NC 详情原票回链（读模型收口）**

- `GET /local/v1/fiscal-documents/{id}`：NC 返回 `original_invoice_id`、`original_invoice_no`、`credit_reason`（唯一读 `CreditOriginalForNC`）；原票仍走 `CreditRemainingForInvoice`；NC 不再误返 `remaining_gross_total`。
- Admin NC 详情：原票按钮跳转原 FT 详情；显示冲销原因。
- 回归：`fiscal-m3-regression.mjs` 增加 `nc-detail-original-ref`、`nc-detail-no-credit-remaining`。

## 0.4.26

**M3.1：Admin NC 补强（部分冲销 + 权限 + Setup 可见性）**

- Setup：`nc_series_ok`、`ready_to_credit`；Admin §3「注册 NC 系列」；checklist 显示 NC / 可冲销。
- 发票详情扩展：`credited_gross_total`、`remaining_gross_total`、行级 `remaining_line_gross`（唯一读 `CreditRemainingForInvoice`）。
- Admin 冲销 modal：全额 / 按行（仅 `line_gross`）、确认框、幂等 toast；**仅 FT** 显示冲销按钮。
- `can_issue_nc` enforce（API 409）；设置 §5 checkbox；owner 默认 1、cashier 默认 0。
- 回归：`scripts/fiscal-m3-regression.mjs` 扩展 partial + permission；设计 `docs/fiscal-m3-nc.zh.md` §16。

## 0.4.25

**M3：NC 冲销（全额 P0）**

- 唯一写路径：`service.IssueCreditNote` → `store.IssueNC`；API `POST /local/v1/fiscal-documents/{id}/credit-notes`。
- 独立 NC 系列；原票更新 `credited_gross_total` / `document_status`；`invoice_line_references` 行级引用。
- Admin 发票详情「冲销」（P0 全额）；ESC/POS 打印原票号与原因。
- 回归：`scripts/fiscal-m3-regression.mjs`；设计定稿 `docs/fiscal-m3-nc.zh.md`。

## 0.4.24

**吊销后重启同步：清本机开票授权**

- Agent 启动 `TryPullCloudProvisionIfNeeded`：本机已激活且云端 provision=`not_active`（含 Ops 吊销）→ `ClearLocalActivation`（唯一本机废钥路径）。
- 开票仍不查云；重打不重签，吊销后仍可重打已开票。

## 0.4.23

**同步开票授权：一眼可见的结果态**

- Admin「从运营同步」下方固定状态条：已同步（绿）/ 等待运营激活（黄）/ 失败（红）。
- Ops 未激活改为独立错误码 `ops_activate_pending`（info，不再当硬失败）。

## 0.4.22

**Ops 激活开票 → Agent 自动领取封装签名钥**

- 生产默认关闭本地粘贴 PEM；主路径 `POST /local/v1/setup/activate-from-cloud`（register B' + pull C → `SaveActivation`）。
- 配对后自动尝试向云端注册设备公钥；Ops「激活开票」后可同步领钥。
- UAT/回归仍可用 `FISCAL_ALLOW_LOCAL_PROVISION=1` 粘贴 PEM。

## 0.4.20

**分单开票：同菜累加 + 本票预估金额**

- 「加入当前人」对同一 `line_key` **有理数累加**（如 `1/2+1/2→1`），不再报「已有该菜」。
- 服务端 `NormalizeAllocation` 当场合并重复份额（Parse / Save / Issue / ingest）；签发不会出现同菜多行。
- 当前人表增加只读 **小计** 与底部 **本票预估**（口径对齐 `line_gross` 比例）。

## 0.4.19

**账单同步：按菜小数份额 qty 不再误判为 0**

- `ParseQtyString` 唯一解析入口改为整串 `big.Rat.SetString`（+ `"N n/d"` 混合形）；去掉 `fmt.Sscanf("%d")` 半截匹配。
- 修复 Restaurant 契约十进制份额（如 `"0.33"`）被读成 `0` → ack `share qty must be positive`、同步关台失败。

## 0.4.18

**打印：LAN 连接复用，避免每张票 SOCKA 事件**

- `tcp:9100` 按 host:port **保持一条长连接**，多张票（厨打 + 税票）复用同一 socket，不再每 job connect/disconnect。
- 写失败时自动重连；Agent 重启后首次打单仍可能见 1 对 `+EVENT=SOCKA_*`（固件限制）。

## 0.4.17

**打印：TCP 单次连接 + 中文标题画布居中**

- LAN `tcp:9100` 预检不再单独 dial，避免部分网口机把 `+EVENT=SOCKA_*` 打到小票顶栏。
- Han 位图（`GS v 0`）在 576px 画布内对齐；满宽光栅不再依赖 `ESC a` 居中（兼容新型号热敏机）。

## 0.4.16

**打印机档口下拉：显示档口名称**

- `GET /local/v1/printers` 合并云端 `print_stations`，返回 `label` / `sort_order`。
- Admin「默认打印档口」与开票「打印机」下拉显示 **`档口名 · 打印机`**（如 `厨房 · 172.20.10.3:9100`），不再只有打印机地址 + UUID 片段。
- 唯一格式化：`FiscalUI.formatPrinterStationOption`（`bootstrap/ui/printer-station.js`）。

## 0.4.15

**发票列表：翻页 + 单卡片查询区（对齐 restaurant ListPaginationBar）**

- 共享 `FiscalUI.createListPaginationBar`（第 N / M 页 · 共 X 张、每页 10/20、上一页/下一页）。
- `GET /local/v1/fiscal-documents` 支持 `page` / `page_size`，返回 `total` / `gross_total_sum`；移除 `limit`。
- 发票页合并为单 panel：筛选 + 表 + 底栏；去掉 topbar hint 与 `#invoiceListMeta`。
- 筛选变更回第 1 页；`buildInvoicesQueryPath` / `refreshInvoices` / `refreshHomeStats` 各唯一一份。

## 0.4.14

**发票列表 Phase 2：日期预设 + 搜索**

- 共享 `FiscalUI.createDateRangeFilter`（今天/昨天/近7天/本月/自定义 + 原生 date 起止）。
- `GET /local/v1/fiscal-documents` 支持 `from`/`to`（`invoice_date`）与 `q`（票号/NIF/桌号）。
- 发票页搜索框；空态区分「该时段暂无」/「未找到匹配」；工作台今日统计走 `refreshHomeStats` 唯一路径。

## 0.4.13

**发票列表 Phase 1：主表瘦身 + 详情抽屉**

- 主表 6 列：签发时刻、票号、金额、购方（NIF+名称合并）、来源、重打。
- Hash / ATCUD / 类型 / 状态 / 打印状态移入点行详情；Hash 仅详情内截断预览 +「复制全文」。
- `refreshInvoices` / `renderInvoiceDetailModal` / `reprintInvoice` / `formatInvoiceBuyerCell` 各唯一一份。

## 0.4.12

**发票列表：Hash 入参可见列（无发票日/打印状态）**

- 列：签发时刻、票号、金额、上张 Hash、Hash、ATCUD、类型、单据状态、购方 NIF/名称、来源、重打。
- `ListInvoices` 唯一列表读：JOIN 客户快照；`truncateHash` / `refreshInvoices` 各一份。
- 来源优先桌号 `display_meta`（`orderLabelFromMeta` 唯一）。

## 0.4.11

**开票成功后清空当前槽**

- 有下一未开人 → 自动选中；否则中间/开票槽清空（禁止 fallback 到已开人当 current）。
- 已开人仅保留顶上 chip（✓，可点只读）。

## 0.4.10

**分单：付一张清一张（单人槽 + 人条进度）**

- 顶上人条可点切换；已开 ✓ 可点只读；中间+开票永远只服务当前一人。
- 中间名为分单标记（不进票面购方）；开票条 NIF/名称为唯一购方；跨票无身份约束。
- 去掉「添加用餐人 / 保存分单」；标记名失焦/回车建人；**签发 = 唯一 allocation PUT**（`commitSplitAllocationForIssue`）。
- 切人丢弃未提交改动并清空开票条；提交时已开人份额强制用 committed 副本，避免误改已开人。

## 0.4.9

**分单左右数量守恒 + 超分实时提示**

- 左边剩余 = 源行 − 本机全部分配（`localRemainingMap` 唯一）；改右边整/分子/分母即时刷新左边（`onSplitQtyEdited` → `paintSplitPoolAndHint`，不重建输入框）。
- 超分：红字 `#splitAllocHint` + toast；保存/开票前 `assertSplitNotOverAllocated` 阻断。
- 分完（剩余≤0）不再出现在左边。

## 0.4.8

**收银账单本机按菜分单工作台**

- Admin：`view-bill-split` 主区（左剩余池 / 右当前人份额 / 同页开票条）；废止列表下钻 `#billDetail`。
- 本机 `allocation_json` + OCC `allocation_revision`；入库冻结 `source_lines`；不回写云。
- `whole_table` 可本机代分后 `mode=person`；签发校验超分；清草稿须人开完且池空。
- 唯一路径：`DraftPersonFromAllocation` / `SaveBillDraftAllocation`；`DraftPartToSaleSnapshot` 仅适配器。

## 0.4.7

**NIF 失焦即时校验**

- 唯一绑定 `bindCustomerNifAutofill`：`blur` → Mod-11 toast；签发/保存仍走 `assertCustomerNifOrToast`。
- 覆盖手工开票、收银账单（含按人）、客户表单。

## 0.4.6

**Admin 用语方案 A：收银账单 / 手工开票**

- 侧栏：「收银账单」（待开票 · 来自收银）、「手工开票」（本机新建）；废止侧栏主名「待开票账单」「订单」。
- 工作台 CTA：「新建开票」「处理收银账单」；SSE toast「有新的收银账单」。
- 文档：原型 README / workbench §0 / M2.6 定法同步；`TestAdminHTMLSchemeACopyUnique` 锁唯一文案。

## 0.4.5

**待开票实时提示 + 税率/NIF 收口 + 票面 MESA/认证折行 + 按人 NIF**

- **SSE**：草稿写入/删除 → `uievents.Hub` → `GET /local/v1/events`；Admin 侧栏角标 + `refreshBills`；禁空转轮询主路径。UAT 门铃：`POST /local/v1/dev/bill-sync/pull`（同进程 `PullAndIngest`）。
- **税率**：唯一 `vatpercent.Normalize`（`23`→`23.00`）；商品保存/同步/手动开票统一；清晰中文错误。
- **NIF**：唯一 `ptnif` Mod-11（空=散客；`999999990` 放行）；Admin + `ApplyCustomerOverride` + 客户保存。
- **票面**：有桌号印 `MESA:`；认证句超宽故意两行（`formatCertificationFaceLines`）。
- **按人**：每人独立 NIF/名称 +「签发此人发票」。

## 0.4.4

**订单添加商品：检索 + 新建一体弹窗**

- 去掉「添加一行」；唯一入口「添加商品」→ `openProductWorkbench`。
- 弹窗：名称/编码检索选用；无命中可新建（同一套 `#pCode`… 字段）；保存走 `saveProductFromFields` 并加入订单。
- 商品页新建/编辑复用同一工作台。
- **P0 定法（文档）**：餐馆不加扫码；商超扫码为主路径（实现待后续刀）。

## 0.4.3

**Admin 工作台双 CTA + FT 票号加粗**

- 餐馆工作台：`新建订单` 与 `处理待开票账单` 同级 `.cta-big`；有 open 待开票时 `focusHomePrimaryCta` 优先后者。
- FT 票面：`Fatura No.` 整行 `ESC E` 加粗（不倍高）；唯一写在 `RenderESCPOS`。

## 0.4.2

**Admin：待开票账单列表金额 + NIF 体验**

- 列表 API / UI 显示 `gross_total`（整桌直读；按人汇总 splits）。
- **待开票账单** 与 **订单开票** 共用客户 datalist + 选 NIF 自动填名；登录即预拉 customers。
- 签发前 UI 校验 NIF 须 9 位数字（空=散客）；唯一 helper：`billSyncPayloadAmount` / `assertCustomerNifOrToast` / `renderCustomerNifDatalist`。

## 0.4.1

**Admin：按人账单 scope 对齐 cloud**

- 同步载荷 `scope_type=split` 与开票 API `mode=person` 分离；唯一 helper：`syncScopeType` / `isSplitSyncScope` / `billScopeLabel`。
- 待开票账单列表/详情正确显示「按人」并列出 splits 人选。
- 回归：`TestAdminHTMLSplitSyncScopeHelper`；`scripts/fiscal-admin-split-ui-prep.mjs`。
- M4 联调清单：`docs/fiscal-m4-farvoo-uat.zh.md`（白云饭店 · 整桌+按人）。

## 0.4.0

**M2.6：正式 Admin + 重打 API**

- 重打唯一路径：`print.ClonePayloadForReprint` → `store.CreateReprintPrintJob` → `POST /local/v1/fiscal-documents/{id}/reprints`（不重签）。
- 发票列表/详情：`GET /local/v1/fiscal-documents`、`GET /local/v1/fiscal-documents/{id}`。
- 正式 Admin（v2 原型）：订单四步、餐馆/商超、待开票账单、商品/客户/设置；替换调试 § 页。
- 回归：`scripts/fiscal-reprint-regression.mjs`。

## 0.3.99

**FT 收口：薄商品 / 客户 + 手动开 FT**

- LOCAL 商品：`UpsertLocalFiscalProduct`（REMOTE 同步 `UpsertFiscalProductByCode` 不覆盖 LOCAL）。
- LOCAL 客户：`UpsertLocalCustomer`；开票 `ensureCustomerIDTx` 绑定 `invoices.customer_id`。
- 手动 FT 唯一入口：`POST /local/v1/fiscal-documents/manual` → `catalog.BuildManualSaleSnapshot` → `IssueDocument`。
- Admin §6–8：商品 / 客户 / 手动开票（去掉旧 demo 直开 FT）。

## 0.3.98

**FT 明细表头 + 店名抬头**

- 唯一 `formatItemLinesHeader` / `formatItemLine`：Qtd、Preco 分列宽；明细区 `rule` → 表头+Soma → `rule` → 行（上下虚线紧贴、不插空行）。
- 店名仅 `LegalName`：加粗 + 1×2（`GS ! 0x01`），下空一行；地址/NIF 仍 1×1。

## 0.3.97

**中文出品联：表头与数量同一套列画布**

- `stationSlipColumnBlockUsesHanCanvas`：`print_locale=zh` 时强制 Items/Qty 走 Han 画布（含西文菜名）；禁止中文表头位图 + 拉丁等宽数量混排导致「数量」与数字错位。
- 葡/英出品联、收据、FT 不动。唯一门闸仍该函数。

## 0.3.96

**FT 票面铺满 80mm（48 列，对齐样票）**

- `receiptWidth` 32→48（与厨打 `escposWidth` 一致）；虚线/`moneyRow`/Resumo IVA 铺满；禁止再缩窄。
- 唯一渲染仍 `RenderESCPOS`。权威：`docs/fiscal-ft-receipt-layout.zh.md` P0#7。
- `POST /local/v1/fiscal-documents` 贯通 `station_id`；M2 smoke 用 `-config` + `station_printers`（去掉失效的 `FISCAL_PRINTER_TCP`）。

## 0.3.95

**Setup 向导显示版本号**

- `AppVerName` = 产品名 + 版本（安装窗口标题可见，例如 `Farvoo Fiscal Agent 0.3.95`）。
- `UninstallDisplayName` 仍为产品名（托盘卸载前缀匹配不变）。
- `wizard-before` 提示对照窗口标题确认版本。

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
