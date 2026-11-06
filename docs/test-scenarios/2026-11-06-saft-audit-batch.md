# 本次改动业务场景测试报告 — SAF-T 审核批次（Header / 状态操作人 / 合计 / 上限 / 归档 / 区间导出）

日期：2026-11-06　分支：`fix/saft-audit-batch`（未 commit / 未 merge）
前序：`2026-11-06-saft-line-amounts-net.md`（行金额 + XSD 1.1，已合入 main）

## 1. 本次修改

| 组 | 内容 | 文件 |
|----|------|------|
| A 软件身份 | 新包 `internal/fiscal/software`：`Farvoo` / `FAINA VAIDOSA, LDA` / NIF `519655290` / 证书号 `0`；`ProductID()` = `Farvoo/FAINA VAIDOSA, LDA`；`Version()` 由 bootstrap 注入 `main.Version`（release = VERSION via ldflags，dev = `dev`）。签发 7 路径、AT 系列注册、SAF-T Header 只读此处；`taxpayer_settings` 的证书号/产品字段不再读写，`TaxpayerInput`/`SeedDemoParams`/`bootstrap.Options` 去掉证书号，Admin 不再发送，`FISCAL_CERT_NO` 删除 | `software/identity.go`、`store/issue*.go`、`store/setup.go`、`store/seed.go`、`store/taxpayer_read.go`、`service/service.go`、`api/routes.go`、`bootstrap/bootstrap.go`、`fiscal_embed.go`、`cmd/fiscal-local/main.go`、`admin/index.html` |
| A' Header | `TaxAccountingBasis=F`；`DateCreated` = 导出当天（门店时区）；`FiscalYear` = 起始年 | `saft/build.go`、`service/saft.go` |
| B 状态操作人 | 迁移 019：`invoices.status_source_id`（回填 = source_id，触发器为新单初始化）；GR/GT/PF 作废与 PF 转开票只写 `status_source_id`；loader 的 DocumentStatus/SourceID 读它 | `migrations/019_invoice_status_source_id.sql`、`store/issue_gr.go`、`issue_gt.go`、`issue_pf.go`、`store/saft_load.go` |
| C 运输单合计 | `TotalQuantityIssued` 排除 A；`NumberOfMovementLines` 含 A | `saft/build.go` |
| D 上限 | `domain/saft_limits.go`：SAF-T 字段上限表（唯一来源）+ `ValidateStatusReason`（1–50 字符）；NC/ND（原 200 字节）与 GR/GT/PF（原 3 份副本）统一调用；Admin NC/ND 原因 maxlength 200→50；导出 `saft.writeText` 按同表截断全部自由文本；ProductDescription < 2 字符 → INVALID | `domain/saft_limits.go`、`service/credit.go`、`debit.go`、`delivery.go`、`transport.go`、`proforma.go`、`saft/build.go`、`admin/index.html` |
| E 归档 | 文件名 `saft_{nif}_{start}_{end}_{id8}.xml`，每次导出一份不可变文件；`Period` 输出整数（无前导零）；`validation_status` 语义写入设计文（builder 层，不等于 XSD） | `service/saft.go`、`store/saft_export.go`、`saft/build.go` |
| F 区间导出 | `ExportSAFTInput{StartDate, EndDate}`（同一财年、start ≤ end）；API 收 `start_date`/`end_date`，`year`/`month` 为自然月简写；Admin §7 改用 `FiscalUI.createDateRangeFilter`（默认「本月」），列表显示起止日期 | `service/saft.go`、`api/saft_handlers.go`、`admin/index.html`、`ui/admin-i18n.js`、`ui_date_range_test.go`、`admin_i18n_test.go` |
| 文档 | 设计文 §3/§5/§7.1/§8/§9 | `docs/fiscal-m5-saft.zh.md` |

**终点形状**：SAF-T 每个字段只有一个权威来源、一个写入点——软件身份属于软件（build 时决定），创建人与状态操作人分列，原因上限在录入时按 Portaria 收紧、导出由同一张上限表兜底，每次导出一份不可变文件，期间为任意同财年区间。

**权威依据**：Portaria 302/2016 Anexo I（1.4、1.12–1.16、4.1.4.3.4、4.1.4.6、4.1.4.10、4.2.1、4.2.2、4.3.2/4.3.3、4.4.2/4.4.3，本地 `pdftotext` 原文核对）；官方 XSD 1.1（Reason ≤50、SourceID ≤30、ProductDescription 2–200、ProductID `[^/]+/[^/]+`）；三家友商 Header（`F`、真实 DateCreated、厂商 NIF ≠ CompanyID）。

**唯一写法自检**

| 规则 | 唯一位置 | 结论 |
|------|----------|------|
| 软件身份 | `software` 包 | `software_certificate_number`/`product_id`/`product_version` 在 Go 代码中已无读写（DB 列保留默认值，不再使用） |
| 原因上限 | `domain.ValidateStatusReason` | 5 个 service 入口共用；原 3 份 `RuneCountInString(reason) > 50` 与 2 份 `len(reason) > 200` 已删除 |
| 文本上限 | `domain` 上限表 + `saft.writeText` | 自由文本字段均经 `writeText`；地址经 `writeAddress`，行商品块经 `writeLineProduct` |
| 状态操作人 | `status_source_id`（触发器初始化 + 3 个状态 UPDATE） | `source_id` 只在 INSERT 写入 |
| 期间 | `service.ExportSAFT` 区间校验 | 月简写仅在 API 层展开 |

## 2. 核心业务规则

- 【本次新增】Header 软件身份来自 `software` 包；证书号拿到后只改 `software.CertificateNumber` 一处。
- 【本次新增】DocumentStatus/SourceID = 当前状态操作人；单据 SourceID = 创建人。019 之前已改状态的行创建人不可恢复（audit_log 无开票事件），回填后两者相同。
- 【本次新增】NC/ND/GR/GT/PF 原因 1–50 字符（按字符计）。
- 【本次新增】`TotalQuantityIssued` 不含 A；`NumberOfMovementLines` 含 A。
- 【本次新增】同期间重导出生成新文件，旧文件与旧行 sha256 保持一致。
- 【本次新增】导出期间：同一财年内任意闭区间。
- 【已实现，不变】Hash 签名口径；SAF-T 行金额口径；`TaxEntity=Global`（一个 NIF 一家店）。

## 3. 详细测试场景

| ID | 场景 | 操作 / 数据 | 预期 | 结果 |
|----|------|-------------|------|------|
| T1 | Header 软件身份与日期 | `TestBuildLineAmountsNetAndSided` 断言 | F / 2026 / CreatedDate / 519655290 / 0 / `Farvoo/FAINA VAIDOSA, LDA` / Version | 【已验证】pass |
| T2 | Period 无前导零 | 同上，8 月样本 | `8` | 【已验证】pass |
| T3 | 运输单合计含作废 GR | 样本加 A 状态 GR | 行数含 A，数量不含 A | 【已验证】pass |
| T4 | 作废不覆盖创建人 | `TestIssueGR_Annul_LinkFT`：op-annul 作废 op-demo-cashier 开的 GR | `source_id` 不变、`status_source_id=op-annul`；loader 两个 SourceID 不同 | 【已验证】pass |
| T5 | 原因上限 | `TestValidateStatusReason`：50 个「é」通过、51 拒、空拒；`TestExportSAFT…`：NC 51 字 → `validation_failed` | 按字符计 | 【已验证】pass |
| T6 | 文本截断按字符 | `TestWriteTextCapsCharacters` | 60 个「ã」截为 50 | 【已验证】pass |
| T7 | 重导出不覆盖 + 区间 | `TestExportSAFTArchivesEachExportAndTakesDateRange`：08-15..08-25 两次 | 两个文件、各自 sha 一致、Header 期间正确；跨财年/倒序/格式错拒绝 | 【已验证】pass |
| T8 | 全票种样本过官方 XSD 1.1 | `scripts/fiscal-saft-xsd-check.sh` | `SAF-T XSD OK` | 【已验证】pass |
| T9 | Admin 日期控件规则 | `TestAdminHTMLInvoiceFiltersUnique`、`TestAdminSettingsI18nUniqueWriters` | 用 `createDateRangeFilter`、纳入 relabel 循环 | 【已验证】pass |
| T10 | 迁移 019 上 AT test 库 | 重启 17881 test Agent（备份 `fiscal-test.db.bak-20261106-before-019`） | 019 生效；`status_source_id` 无 NULL | 【已验证】pass |
| T11 | Admin 真界面区间导出 ×2 | 用户在 §7 选 2026-11-01..2026-11-30 导出两次 | 两行、两个文件（`…_380378e8.xml` / `…_61e3e041.xml`），期间 `2026-11-01 ~ 2026-11-30` | 【已验证】pass |
| T12 | 真实导出 Header | 同上文件 | `F`、`DateCreated 2026-11-06`、`ProductID Farvoo/FAINA VAIDOSA, LDA`、证书 0、`ProductVersion dev`（fiscal-local 为 dev 构建） | 【已验证】pass |
| T13 | 真实导出 MovementOfGoods | 8 GR（含 A、F）+ 6 GT | `NumberOfMovementLines 19`（含 GR/8 A）、`TotalQuantityIssued 21.00`（不含 A） | 【已验证】pass |
| T14 | 真实导出 Hash 链 | DB 公钥逐张验签 | 21 张、3 系列、0 问题 | 【已验证】pass |
| T15 | 真实导出 XSD | 官方 XSD 1.1 | 仅中文商品名「金叉套餐」（FT/7、GT/5、GT/6）共 7 错；SourceID/CustomerID/Header 无错 | 【已验证】pass（剩余错误为已知开票端问题） |
| T16 | 回归 | GR 40、GT 36、PF 40、RG 13、d62 27、m3 28 | 全 PASS | 【已验证】pass |
| T17 | 回归 m5 / m6 | `fiscal-m5/m6-regression.mjs` | 启动即 `FISCAL_SESSION_SECRET required` | 【阻塞】脚本陈旧（m5 由独立会话修，m6 同因） |
| T18 | release 构建 ProductVersion = VERSION | Windows release | — | 【未验证】需 CI release 构建 |

## 4. mesa-local-product-test

N/A ·【未验证】：只改 fiscal-agent，不经 Mesa `:3000`。替代验收：`go test ./...`、`go vet ./...`、`GOOS=windows go build`、`fiscal-saft-xsd-check.sh`、上表 6 项 node 回归、AT test 真界面导出。

## 5. 实际测试结果

| 项 | 结果 |
|----|------|
| `go test ./...` / `go vet` / Windows 交叉编译 | pass |
| XSD 门禁（样本） | pass |
| node 回归 6 项 | pass |
| AT test 真导出（XSD / 审计 / 验签） | 仅剩中文名；其余 pass |
| m5 / m6 回归 | 阻塞（与本改动无关） |

## 6. 风险与待确认

1. **中文商品名**：FT/7、GT/5、GT/6 已签名不可改，2026-11 在 AT test 库永远 INVALID。开票端校验由独立会话处理（方案待确认）。
2. **历史创建人**：019 之前状态已变的 GR/GT/PF（AT test 库：GR/6 F、GR/8 A）创建人已丢失，两个 SourceID 相同。
3. **证书号来源变更**：setup 页原本就不可编辑（Admin 写死 `'0'`），实际无影响；拿到证书后改 `software.CertificateNumber` 并发版。
4. **NC/ND 原因上限收紧为 50 字符**：用户输入超过 50 将被拒；Admin 输入框已同步 maxlength。
5. **API 兼容**：`POST /local/v1/saft/exports` 仍接受 `year`/`month`；列表 API 的 `year`/`month` 过滤不变；Admin 列表改为不过滤、显示全部。
6. **旧文件**：改名前生成的 `saft_{nif}_{yyyy-mm}.xml` 保留，旧行仍指向它（被覆盖过的旧行无法追溯）。
7. **未改**：`MovementStartTime` 截到分钟（早于 SystemEntryDate 数秒，AT 已接受）；Admin 列表「N 张」只计发票+收据；同 NIF 不同名称主档只留一条；`fiscal-rg-regression.mjs` 跑完不退出（子进程未回收）。
8. **审计脚本口径**：初版审计脚本把 `NumberOfMovementLines` 也按排除 A 检查，与 Portaria 4.2.1 不符，以原文为准。
