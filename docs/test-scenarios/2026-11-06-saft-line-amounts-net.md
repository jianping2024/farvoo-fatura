# 本次改动业务场景测试报告 — SAF-T 行金额未税 + 官方 XSD 合规

日期：2026-11-06　分支：`fix/saft-line-amounts-net`（未 commit / 未 merge）

## 1. 本次修改

**内容**

| 文件 | 改动 |
|------|------|
| `apps/fiscal-agent/internal/fiscal/saft/build.go` | 行金额改写未税 `line_net`；NC→`DebitAmount`，其余→`CreditAmount`；删除 `SettlementAmount`；`TotalDebit/TotalCredit` = Σ 行金额；`DocumentStatus` 复合节点（Invoice / WorkDocument / StockMovement 补 `SourceBilling`，Payment 补 `SourcePayment`）；`TaxTable` → `TaxTableEntry`；`References` 移到 XSD 位置；`SourceDocuments` 按 XSD 顺序；Payment 节点顺序；0% 行输出免税原因/代码，缺失 → `INVALID` |
| `apps/fiscal-agent/internal/fiscal/compliance/money.go` | `LineFromGross` 的 unitNet 由 2 位改 6 位（`UnitPriceDecimals`）；gross/net/tax 不变 |
| `apps/fiscal-agent/internal/fiscal/saft/amounts_test.go`（新） | 全票种样本 + 金额不变量断言 + 0% 无免税原因 → INVALID |
| `apps/fiscal-agent/internal/fiscal/saft/testdata/SAFTPT1.04_01.xsd`（新） | AT 官方 XSD 原样（sha256 `292c0ab4…9350`） |
| `scripts/fiscal-saft-xsd-check.sh`（新） | 官方 XSD 1.1 校验门禁（`xmlschema==4.2.0` 私有 venv） |
| `scripts/check-fiscal-agent.sh`、`.github/workflows/fiscal-agent-ci.yml` | 接入 XSD 门禁 |
| `apps/fiscal-agent/internal/fiscal/service/transport_at_test.go` | GR 报文 UnitPrice 期望改 `10.000000`（计划内变化） |
| `docs/fiscal-m5-saft.zh.md` | 改正 §2/§3/§8/§10 口径 |
| `store/migrations/018_operator_saft_user_code.sql`（新） | `operators.saft_user_code`（U001…，门店内唯一，触发器为唯一写入点，存量回填） |
| `store/saft_load.go` | 4 个 loader 的 SourceID 改 join `operators.saft_user_code` |
| `saft/build.go`（第二轮） | `saftCustomer` 唯一客户身份映射，替换 `customerKey` 与 movement 999999990 兜底；`unknown` 常量 |
| `saft/customer_test.go`、`store/operator_saft_user_code_test.go`（新） | 客户身份规则、操作员短码 |

**终点形状**：导出文件通过官方 SAF-T(PT) 1.04_01 XSD 1.1；发票 / PF / GR/GT 行写未税 `LineNet`，借贷侧按票种唯一规则决定，控制合计由同一套行金额累加；UnitPrice 在开票时唯一写入点存 6 位，导出原样写出；RG 金额口径不变、结构按 XSD。

**权威依据**：AT 官方 XSD `SAFTPT1.04_01.xsd`（info.portaldasfinancas.gov.pt/apps/saft-pt04/）；Portaria 302/2016 4.1.4.20.6/7（行金额未税、借贷侧）；三家友商真实 SAF-T（宏翔 HX cert 1549、腾龙 TalentoRos cert 1175、ifume cert 2985）均过同一 XSD，行金额全部未税、FT/FR 全在 Credit、DocumentStatus 复合节点。

**唯一写法自检**

| 规则 | 唯一位置 | 结论 |
|------|----------|------|
| 行借贷侧 | `saft.lineIsDebit` | 发票行、PF 行、GR/GT 行、SalesInvoices 合计均调用；diff 中无第二处 `"NC"` 判断 |
| 行金额 | `saft.writeLineAmount`（写 `LineNet`） | build.go 中 `LineGross` 已无引用；`DebitAmount/CreditAmount` 仅此处 + RG Payment 行（收款额，按方案不变） |
| Tax + 免税对 | `saft.writeLineTax` | 三类行共用 |
| 单价精度 | `compliance.UnitPriceDecimals` / `LineFromGross` | 开票唯一写入点；NC 复制原单 unitNet，自动一致 |
| 方案已拒绝的补丁形态 | 「导出时临时重算单价」「排除作废销售单」「有折扣才输出 Settlement 的开关」 | diff 中均未出现 |

## 2. 核心业务规则

- 【本次新增】SAF-T 行金额 = 行未税额（2 位）；NC 在 Debit、FT/FS/FR/ND/PF/GR/GT 在 Credit。
- 【本次新增】每张单据 Σ 行金额 = `NetTotal`；`NetTotal + TaxPayable = GrossTotal`（原已成立，现有断言）。
- 【本次新增】SalesInvoices `TotalDebit` = Σ NC 行金额，`TotalCredit` = Σ 其他销售单行金额；WorkingDocuments `TotalCredit` = Σ 非作废 PF 行金额。
- 【本次新增】|Quantity × UnitPrice − 行金额| ≤ 0.01（UnitPrice 6 位）。
- 【本次新增】0% 税率行缺 `TaxExemptionReason/Code` → 导出 `INVALID`（XSD assert 要求）。
- 【已实现】销售单 SAF-T `InvoiceStatus` 恒为 `N`（系统无销售单作废；NC/ND 只改原票业务状态）。
- 【已实现，不变】Hash 签名口径（日期、录入时间、单号、GrossTotal 两位、前 Hash）不受影响。
- 【待核实】RG 行金额为收款额（含税、无 Tax 节点）：XSD 允许 RG 无 Tax，OCC 称 RG 不进 e-Fatura；Portaria 原文未找到 RG 必须未税的规定，需会计确认一次。

## 3. 详细测试场景

| ID | 场景 | 前置 | 操作 | 预期 | 数据验证 | 结果 |
|----|------|------|------|------|----------|------|
| S1 | 多税率多数量 FT | 种子门店 + FT 系列 | 真实 `IssueFT`：咖啡 3×0.85@13%、可乐 7×2.20@23%、面包 2×1.15@6% | 行写未税、Credit；UnitPrice 6 位 | XML：`UnitPrice 0.752212 / CreditAmount 2.26`，`1.788618 / 12.52`；Σ行=NetTotal | 【已验证】pass |
| S2 | NC 全额冲销 | S1 的 FT + NC 系列 | 真实 `IssueNC` CreditFull | NC 行在 Debit；References 在 TaxPointDate 与 Description 之间 | XML 片段核对；`TotalDebit 16.95` = Σ NC 行 | 【已验证】pass |
| S3 | SalesInvoices 控制合计 | S1+S2+种子 FT+0% FT | Build | `TotalDebit/TotalCredit` = Σ 对应侧行金额 | 断言通过（`TotalCredit 37.11`） | 【已验证】pass |
| S4 | PF 含作废单 | 两张 PF（N / A） | Build | `TotalCredit` 只计 N；DocumentStatus 含 `SourceBilling` | 断言 + XSD | 【已验证】pass |
| S5 | GR / GT | 两张移动单 | Build | 行未税、Credit；DocumentStatus 含 `SourceBilling` | 断言 + XSD | 【已验证】pass |
| S6 | RG 收据 | 一张 RG | Build | 节点顺序与 `SourcePayment` 符合 XSD | XSD | 【已验证】pass |
| S7 | 0% 免税行（带原因） | M07 免税行 | Build | 输出 `TaxExemptionReason/Code`；XSD assert 通过 | XSD | 【已验证】pass |
| S8 | 0% 行缺免税原因 | 无原因的 ISE 行 | Build | `INVALID` + 提示 TaxExemption | `TestBuildFlagsZeroRateWithoutExemption` | 【已验证】pass |
| S9 | 全票种样本对官方 XSD 1.1 | S1–S7 合成文件 | `scripts/fiscal-saft-xsd-check.sh` | VALID | `SAF-T XSD OK` | 【已验证】pass |
| S10 | 门禁能拦旧输出 | 修复前导出 XML | 同上 + `SAFT_XSD_CHECK_FILES` | FAIL（10 错） | TaxTable / DocumentStatus / References 报错 | 【已验证】pass（按预期失败） |
| S11 | 校验器基准 | 三家友商真实 SAF-T | 同一校验器 | 全部 VALID | 宏翔 / 腾龙 / ifume VALID | 【已验证】pass |
| S12 | GR 发 AT 报文单价 | mock AT | `TestIssueDeliveryNote_RealNumberToAT_NoDoubleSend` | UnitPrice `10.000000` | 断言 | 【已验证】pass |
| S13 | GR / GT 端到端（含 SAF-T 导出） | 本地 mock Agent | `fiscal-gr-regression.mjs`、`fiscal-gt-regression.mjs` | 全 PASS | 含「GR/GT MovementOfGoods export」 | 【已验证】pass |
| S14 | M5 SAF-T 端到端 | 本地 Agent | `fiscal-m5-regression.mjs` | 全 PASS | 启动即失败：`FISCAL_SESSION_SECRET required`（脚本未用 `fiscalAgentTestEnv`，与本次改动无关，外部注入 env 仍失败） | 【阻塞】脚本陈旧 |
| S15 | Admin §7 真界面导出（AT test 库，2026-11，第 1 次） | `fiscal-test.db`：6 FT + 用户新开 FT/7（3 行 6%/13%/23%）、8 GR（含 A/F）、4 GT | 用户在 Admin §7 导出 | 行金额口径正确；暴露 XSD 真实数据问题 | 金额：20 张单 Σ行=NetTotal，全 Credit，TotalCredit 2658.24 = total_net；FT/7 UnitPrice 6 位。XSD 43 错：SourceID 36 位 UUID >30（38 处）、无 NIF 收货方 CustomerID 空（3）、中文商品名（2） | 【已验证】金额 pass；XSD fail → 第二轮修 |
| S17 | 操作员短码迁移 | 同库 | Agent 重启跑 018 | admin → U001 | `operators.saft_user_code=U001` | 【已验证】pass |
| S18 | Admin §7 真界面重导出（第 2 次） | S17 后 | 用户在 Admin §7 导出 | SourceID/CustomerID 错误消失；仅剩中文名 | `SourceID U001`×38；客户主档：5 个 NIF + `CF`（Consumidor Final，GR/2、GT/4 等 5 处引用）+ `C93e0fe91b6ba`（Maria Costa）；XSD 仅 3 错，全部来自 FT/7「金叉套餐」（ProductDescription×2、Description×1）；状态 INVALID 原因同 | 【已验证】pass（INVALID 为预期，见风险 7） |
| S16 | GT 6 位单价发 AT test（GT-06） | AT test（`at_env=test`，`fiscal-test.db`，17881），GT2026T01 last_number=4 | 用户在 Admin 签发调拨 GT 两张（12:56:15Z、12:57:02Z UTC） | ReturnCode 0；报文 UnitPrice 6 位 | GT/5：ATCUD `AAJFJP8X9D-5`、ATDocCodeID `1108739978`，UnitPrice `41.509434`/`491.150442`/`53.658537`，数量各 1；GT/6：ATCUD `AAJFJP8X9D-6`、ATDocCodeID `1108739986`，同单价，数量 1/2/3；两张 `ReturnCode 0 OK`；库 `at_transport_doc_code` 一致，`last_number` 4→6 | 【已验证】pass |

## 4. mesa-local-product-test

N/A ·【未验证】：本次只改 fiscal-agent 库与导出，不经 Mesa `:3000`。替代验收：`go test ./...`、`go vet ./...`、Windows 交叉编译、`scripts/check-fiscal-agent.sh`（含 XSD 门禁）、`fiscal-gr-regression.mjs`、`fiscal-gt-regression.mjs` 全部 pass。

## 5. 实际测试结果

| 项 | 结果 |
|----|------|
| `go test ./...`（apps/fiscal-agent） | pass |
| `go vet ./...` + `GOOS=windows go build` | pass |
| `scripts/check-fiscal-agent.sh` | pass（含 `SAF-T XSD OK`） |
| `scripts/fiscal-saft-xsd-check.sh` 对修复前输出 | 按预期 FAIL |
| `fiscal-gr-regression.mjs` / `fiscal-gt-regression.mjs` | pass |
| `fiscal-m5-regression.mjs` | 阻塞（脚本 env 陈旧，与本改动无关） |

未覆盖：S14；CI 上 XSD 步骤尚未在 GitHub Actions 实跑（未 push）。

## 6. 风险与待确认

1. **历史月份重导出会与旧文件不同**（行金额 gross→net、方向、结构）。这是修正；已交会计 / 已提交 AT 的月份是否需重导出，由业务决定。历史行 UnitPrice 仍为 2 位（不迁移）。
2. **GR/GT 发 AT 报文 UnitPrice 变 6 位**：WSDL 为 `xsd:decimal` 无位数限制；AT test 已接受（S16，GT/5、GT/6 ReturnCode 0）。生产 AT 未验证。GT/6 含中文商品名「金叉套餐」，AT 运输报文接受，但同月 SAF-T 仍会因此 INVALID（风险 7）。
3. **0% 税率开票路径不收免税原因**：`issue.go` 写 `tax_exemption_code/reason` 为 NULL，`products.go` 可产出 `ISE`。若真开出 0% 销售行，导出将 `INVALID`（正确地失败而非产出违规文件）。开票端补免税原因是独立功能，本次未做。
4. **RG 金额口径**待会计确认（见 §2）。
5. **CI 新增 Python 依赖**（`xmlschema==4.2.0`，首次运行 pip 安装）；`check-fiscal-agent.sh` 的 Docker 回退路径下 XSD 步骤仍需本机 `go` 与 `python3`。
6. `fiscal-m5-regression.mjs` 陈旧需单独修（改用 `fiscalAgentTestEnv` / session helper）；已由独立会话处理。该脚本旧版全局 `pkill -f fiscal-local`，测试期间两次杀掉 17881 test Agent（已重启，测试库未被写）。
7. **开票端允许非 Windows-1252 的 SAF-T 商品名**（FT/7「金叉套餐」）：导出器正确判 INVALID，且 1252 转换后名称为空 → XSD 再报 3 错。根因在开票端（签发前未校验 SAF-T 名称可编码），属独立任务。
8. **历史单据重导出的 SourceID / CustomerID 变化**：UUID → `U001`；无 NIF 收货方 → `CF` / `C…`。不影响 Hash、不涉及 AT。
9. **待分析**：MovementOfGoods `NumberOfMovementLines` / `TotalQuantityIssued` 目前含作废（A）单（本月 13.00 含 GR 作废单 1 行），Portaria 口径需核对，本次未改。
10. **同 NIF 不同名称**（519655290：Farvoo Demo Lda / adadsf）在主档只保留一条，属数据质量，不影响 XSD。
11. 测试库已迁移至 018；备份 `data/fiscal-test.db.bak-20261106-saft-uat`。
