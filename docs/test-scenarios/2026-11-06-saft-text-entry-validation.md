# 本次改动业务场景测试报告 — SAF-T 文本录入校验（签发前拦截）

日期：2026-11-06　分支：`fix/saft-text-entry-validation`（基于 main `b089b0f`；未 commit / 未 merge）
前序：`2026-11-06-saft-audit-batch.md`（上限表 `domain/saft_limits.go`、原因 1–50、导出截断）

## 1. 本次修改

**起因**：AT test 库商品 `jincha` 的税票名称留空，Admin 和 store 都拿显示名「金叉套餐」补上，签进 FT FT2026DEMO01/7、GT GT2026T01/5、GT GT2026T01/6。已签不可改，2026-11 的 SAF-T 只能判 INVALID，名称转 Windows-1252 后为空，过不了 XSD（ProductDescription 最少 2 字符）。审核批次之后，真实导出剩下的唯一 XSD 错误就是它。

**终点形状**：所有写进 SAF-T 的自由文本都由同一个函数按同一张上限表校验：NFC + 去空格 → 可编码 Windows-1252 → 长度合规。录入和签发前拒绝，返回带字段的本地化错误；导出端在成品 XML 上用同一个编码判断，问题写明单据和元素，不再静默删字。

**权威依据**：
- `docs/fiscal-sqlite-schema.zh.md` §7 第 2 步、`saft_name` 列「签前固化葡/英；Windows-1252」
- `docs/print-agent-lessons.zh.md:110`「签前 Windows-1252 校验」
- 官方 XSD `SAFTPT1.04_01.xsd`：`SAFPTProductDescription` 2–200；CompanyName 100、AddressDetail 210、City 50、PostalCode 20、ProductCode 60、Reason 50

| 组 | 内容 | 文件 |
|----|------|------|
| 规则 | `domain.ValidateSAFTText`（唯一规则）、`domain.EncodesWindows1252`（唯一编码判断）、`*domain.SAFTTextError{Field, Reason, Min, Max}`；`ValidateStatusReason` 改为调用它（原因多了编码检查） | `domain/saft_limits.go` |
| 组合 | `SaleSnapshot.ValidateSAFTText`（客户 + 行）、`CustomerInput.ValidateSAFTText`、`Address.ValidateSAFTText`；`SaleLine.EffectiveSaftName`（saft_name 空用 display_name 的唯一回退） | `domain/saft_text.go`、`store/issue.go`（buildLines 改用它） |
| 新单 | `service.validateSaleText`：IssueDocument（FT/FS/FR，含手工、整桌、云端自动）、IssueProforma、IssueDeliveryNote、IssueTransportNote，在签名、占号、AT 之前 | `service/service.go`、`proforma.go`、`delivery.go`、`transport.go` |
| 装卸货地 | `resolveShipPoints` 校验 ship_from / ship_to | `service/transport_at.go` |
| 主档 | `store.CleanProductText`（两条商品写入 + bill sync 预检）；`UpsertLocalCustomer`；`service.UpsertTaxpayer` | `store/products.go`、`store/customers.go`、`service/service.go` |
| 原因 | 5 个入口（NC、ND、GR/GT/PF 作废）直接返回规则错误 | `service/credit.go`、`debit.go`、`delivery.go`、`transport.go`、`proforma.go` |
| 错误出口 | HTTP `400 saft_text_invalid` + field/part/min/max（`api.writeSAFTTextErr`，`writeCoded` 与商品/客户保存共用）；bill sync ingest 在存草稿前拒绝整单，code `saft_text_invalid`；自动开票 `mapAutoIssueErr` 同码 | `api/routes.go`、`api/catalog_handlers.go`、`billsync/ingest.go`、`service/bill_sync_auto_issue.go` |
| 导出 | 删除 11 处 `check()` 与私有 `validateWindows1252`；`saft.encodingErrors` 扫描成品 XML 全部元素 | `saft/build.go` |
| Admin | 删除「税票名称留空用显示名」的 JS 回退；`focusSAFTTextField` / `clearSAFTTextField`（商品、客户、门店资料、开票四张表单 + 订单行标红）；税票名称下加提示；`formatError` 的 `saftTextMessage`；zh/en/pt 文案 | `admin/index.html`、`ui/toast.js`、`ui/admin-i18n.js` |
| 扫描 | `go run ./cmd/saft-text-scan <db>`：同一规则只读扫描，列出需修主档（fix）与已签不合格（signed） | `cmd/saft-text-scan/main.go` |
| 文档 | m5 设计文 §3.1 录入校验；schema §7 第 2 步 | `docs/fiscal-m5-saft.zh.md`、`docs/fiscal-sqlite-schema.zh.md` |
| 夹具 | 1 字符商品名（X/Y/B/C）改为合规名称（XSD 本就不允许） | billsync/store go test、`fiscal-bill-sync/m1/rg-regression.mjs` |

**唯一写法自检**

| 规则 | 唯一位置 | 结论 |
|------|----------|------|
| 文本规则 | `domain.ValidateSAFTText` | 所有调用方都经它（含扫描命令）；无第二套 |
| 编码判断 | `domain.EncodesWindows1252` | 录入与导出共用；`saft.validateWindows1252` 已删 |
| saft_name 回退 | `SaleLine.EffectiveSaftName`（签发）、`store.CleanProductText`（主档） | Admin 的 `body.saft_name = body.display_name` 与 store 两处 `in.SaftName = in.DisplayName` 已删 |
| HTTP 形状 | `api.writeSAFTTextErr` | 一处 |
| Admin 标红/聚焦 | `focusSAFTTextField` | 一处，四张表单各传自己的字段表 |
| Admin 文案 | `FiscalUI.formatError` → `saftTextMessage` | 一处 |

**与已确认方案的差异**：扫描工具从计划的 `scripts/fiscal-saft-text-scan.mjs` 改成 Go 命令。JS 版需要另写一份规则，违反「唯一写法」。

## 2. 核心业务规则

- 【本次新增】SAF-T 文本先 NFC、再去首尾空格，然后必须可编码 Windows-1252，长度合规；存入的是清洗后的值。
- 【本次新增】商品：编码 1–60、税票名称 2–200；税票名称留空时用显示名，显示名不合格同样被拒。
- 【本次新增】新单（FT/FS/FR/PF/GR/GT）在签名、占号、通信 AT 之前校验客户与每一行；被拒不占号、不调 AT。
- 【本次新增】客户主档、门店资料、GR/GT 装卸货地址按 XSD 上限校验。
- 【本次新增】作废与 NC/ND 原因多了编码检查（中文原因被拒）；错误码统一为 `saft_text_invalid`（原来超长是 `validation_failed`）。
- 【本次新增】云端 bill sync：菜名或 item_code 不合格 → 整单 ack failed（`saft_text_invalid`），草稿和商品都不落库（Will 已接受）。
- 【本次新增】导出：成品 XML 任一元素不可编码 → INVALID，错误写明「单据号 元素 "原文"」。
- 【已实现，不变】NC/ND/RG 沿用原单客户与商品行；已签数据不改。

## 3. 详细测试场景

| ID | 场景 | 操作 / 数据 | 预期 | 结果 |
|----|------|-------------|------|------|
| S1 | 规则边界 | `TestValidateSAFTText`：葡语重音、€ + 不换行空格、NFD「Pão」、2 / 200 字、可空、中文、单个汉字、emoji、1 字、必填空、201 字 | 合格的返回清洗值；不合格给 encoding / too_short / too_long | 【已验证】pass |
| S2 | 原因 | `TestValidateStatusReason`：空、50 个 é、51 字、「退货」 | too_short / 通过 / too_long / encoding | 【已验证】pass |
| S3 | 快照组合 | `TestSaleSnapshotValidateSAFTText`：显示名回退、输入不被改写、显示名为中文、客户城市中文 | 字段 `lines.2.saft_name`、`customer.city` | 【已验证】pass |
| S4 | 装卸货地 | `TestAddressValidateSAFTText` | `ship_from.address_detail` encoding | 【已验证】pass |
| S5 | FT 拒绝不占号 | `TestIssueDocument_RefusesNonSAFTText`：中文名、1 字名、中文编码、客户城市中文，之后正常开一张 | 四次全拒；正常单号为 `/1`；冻结名为 NFC 去空格的「Chá」 | 【已验证】pass |
| S6 | GR 不触 AT | `TestIssueDeliveryNote_SAFTTextNeverReachesAT`：行名中文、卸货地中文 | 两次都拒，AT 调用 0 次 | 【已验证】pass |
| S7 | 主档 | `TestMasterData_RefusesNonSAFTText`：商品显示名中文且税票名空 → 拒；补葡语名后通过（显示名仍中文）；客户名中文；门店店名中文 | 字段 `product.saft_name` / `customer.company_name` / `taxpayer.legal_name` | 【已验证】pass |
| S8 | bill sync 整单拒绝 | `TestIngest_RejectsNonSAFTDishName`：一行「金叉套餐」 | ingest code `saft_text_invalid`；0 草稿、0 商品 | 【已验证】pass |
| S9 | 导出端全元素 | `TestBuildRejectsNon1252Characters`：行名中文；客户城市中文 + 商品编码中文 | INVALID；错误含 `City "里斯本"`、`FT FT2026DEMO01/1 ProductCode "茶1"` | 【已验证】pass |
| S10 | HTTP 形状 | `TestUpsertProduct_SAFTTextErrorShape` | 400，`saft_text_invalid`、field、part、min 2、max 200 | 【已验证】pass |
| S11 | Admin 唯一路径 | `TestAdminSAFTTextErrorSinglePath`：无 JS 回退、helper 一份、四张表单接入、toast 分支、15 个 key 三语齐 | — | 【已验证】pass |
| S12 | 三语文案 | node 载入 i18n + toast，调 `formatError`：订单行编码、卸货地城市超长、商品名太短、原因编码 | zh/en/pt 句子完整，订单行带「第 n 行」与补葡语名提示 | 【已验证】pass |
| S13 | 扫描命令 | `saft-text-scan` 扫 AT test 库与 hub-inspect 库（只读，data 目录不产生 -wal/-shm） | test 库：fix 1（jincha）、signed 3（FT/7、GT/5、GT/6）；hub-inspect：fix 1（TEA-01「花茶」）；退出码 1 | 【已验证】pass |
| S14 | 真界面·AT test：修遗留坏商品 | 编辑 `jincha`，税票名称保持「金叉套餐」→ 保存；再把同一框改成「Menu garfo dourado」→ 保存 | 第一次：toast「税票名称只能用葡语/拉丁字符（不能含中文、emoji 等）」，税票名称框标红、聚焦、全选；第二次「商品已保存」，库内 saft_name=`Menu garfo dourado`、display_name 仍为「金叉套餐」 | 【已验证】pass |
| S15 | 真界面·AT test：坏商品开票被拒、修好后开票 | 修好前用 `jincha` 开 FS；修好后再开 | 修好前 toast「第 1 行：税票名称只能用葡语/拉丁字符…请在商品资料里填写葡语『税票名称』」，FS 仍为 0、单据数不变、无新 AT SOAP；修好后得 **FS FS2026DEMO01/1**（ATCUD AAJFJ28HRZ-1，首号即证明被拒未占号），冻结 product_description=`Menu garfo dourado` | 【已验证】pass |
| S15b | 开票步订单行标红 | 同 S15 被拒时 | 开票步可见的是 `#invSummaryLines`，原实现只标 `#orderLines`（该步隐藏）→ 看不到标红。已修：摘要行加 `data-line`，`focusSAFTTextField` 同时标两张表 | 【已验证】缺陷已修（`TestAdminSAFTTextErrorSinglePath` 钉住）；修后画面【未验证】：mock 缺「开票电脑→档口」绑定发不出请求，AT test 已无坏商品可复现 |
| S16 | 真界面·AT test：客户 / 门店资料 / GR 卸货地 | 新建客户 NIF 509442013 名称「王氏贸易」；店名改「金叉餐厅」保存门店；GR（非 IVA 收货方，Porto 收货地址）卸货地城市「里斯本」签发 | 三处 toast 分别为「客户名称…」「店名…」「卸货地 · 城市只能用葡语/拉丁字符…」，对应输入框标红、聚焦、全选；库内客户仍 5 条、无 509442013；taxpayer 未更新（updated_at 仍 11-02）；GR 仍为 8、无新 AT SOAP（最后一次仍 14:32） | 【已验证】pass |
| S17 | 真界面·AT test：作废原因中文 | 作废 GR GR2026T02/7，原因「客户取消」→ 确认 | 响应 `400 saft_text_invalid` field=reason part=encoding；toast「原因只能用葡语/拉丁字符…」；GR/7 仍为 N、无 status_reason、无 AT 调用。弹窗已关，无焦点（风险 3） | 【已验证】pass |

## 4. mesa-local-product-test

N/A ·【未验证】：只改 fiscal-agent，不经 Mesa `:3000`。替代验收：`check-fiscal-agent.sh`（go test、vet、Windows build、XSD）、上表 go test、节点回归（见 §5）。

## 5. 实际测试结果

| 项 | 结果 |
|----|------|
| `./scripts/check-fiscal-agent.sh`（go test ./...、go vet、Windows amd64 build ×2、官方 XSD 1.1 样本） | pass（exit 0） |
| `fiscal-m6-product-regression.mjs` | pass |
| `fiscal-manual-ft-regression.mjs` | pass |
| `fiscal-gr-regression.mjs` / `fiscal-gt-regression.mjs` / `fiscal-pf-regression.mjs` | pass |
| `fiscal-rg-regression.mjs` | pass 13/13（脚本跑完不退出，已知问题，手动结束） |
| `fiscal-d62-cert-regression.mjs` | pass（`fail: 0`） |
| `fiscal-bill-sync-regression.mjs` / `fiscal-m1-regression.mjs` | 【阻塞】Agent 启动 / 登录即失败（`FISCAL_SESSION_SECRET required` / `session required`）。对照：在 main `b089b0f` 的干净副本上跑，失败相同（m1 在 main 上更早失败于 `setup-taxpayer`），属于与 m5/m6 同类的脚本陈旧，与本改动无关。bill sync 的新规则由 go test S8 覆盖 |
| 真界面 S14–S17（AT test 环境：本分支代码 + `fiscal-test.db`，`at_env=test`，17881） | pass；S15b 缺陷当场修复，修后画面未复看 |

**AT test 环境记录**：Agent 由本 worktree 代码启动（`at_env=test`、`db=apps/fiscal-agent/data/fiscal-test.db`、端口 17881）；AT watch 17:05 返回 2002；测试前备份 `fiscal-test.db.bak-20261106-before-saft-text`。库内变化仅有：`jincha` 税票名称改为 `Menu garfo dourado`（修复遗留坏数据，本意）、新增 FS FS2026DEMO01/1（本地签发，不报 AT）。全程无新的 AT SOAP 交互（最后一次仍为 14:32）。

## 6. 风险与待确认

1. **已签坏单**：AT test 库 FT/7、GT/5、GT/6 不可改，2026-11 在该库永远 INVALID（导出错误现在会写明这三张单）。
2. **店里已有的坏主档**：上线后用它们开票会被拒，店员需要补葡语税票名称。发版前用 `saft-text-scan` 扫生产库；生产库情况【待核实】。
3. **作废 / NC / ND 弹窗**：确认后弹窗先关、再请求，原因被拒时只有 toast，没有焦点。要做到聚焦需要改弹窗流程（失败时保持打开），本刀未改。
4. **Mesa 没填葡语菜名**：云端整单 failed（已接受）；Mesa 端展示失败原因是另一个仓的后续工作。
5. **错误码变化**：原因超长从 `validation_failed` 变为 `saft_text_invalid`（Admin 文案已覆盖）；外部调用方若按旧码判断需同步（目前只有 Admin）。
6. **主工作区**：未触碰另一会话的 fiscal-clear 未提交改动；所有改动在本 worktree 分支上。
