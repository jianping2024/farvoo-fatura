# GR：发货 / 交货单（Guia de remessa）

> **状态：定稿**（G1–G6 已落地；G7 live AT 仍 blocked）  
> **权威：是**（GR 行为、实现步骤、AT 运输通信与测试接入；库列与 [`fiscal-sqlite-schema.zh.md`](fiscal-sqlite-schema.zh.md) + `migrations/014_gr_delivery.sql` 对齐）  
> **对应实现：** `store.IssueGR` / `AnnulGR`、`store.linkDeliveryNotesInTx`、`service.IssueDeliveryNote`、`at.TransportClient`（默认 mock）  
> **test/prod 统一接入：** 见 [`fiscal-at-live-integration.zh.md`](fiscal-at-live-integration.zh.md)（A2 = 原 G7）  
> **用途/当事人：** [`fiscal-doc-types-usage.zh.md`](fiscal-doc-types-usage.zh.md) §8  
> **测试场景：** [`fiscal-doc-type-test-scenarios.zh.md`](fiscal-doc-type-test-scenarios.zh.md) §9  
> **计划顺序：** RG → PF → **GR** → GT  
> **法律依据（摘要）：** DL 147/2003（RBC）art. 4.º–5.º；Portaria 161/2013（通信方式）；AT FAQ Documentos de Transporte；SAF-T 表 4.2 `MovementType=GR`

---

## 1. 目标

签发 **GR**：证明向收货客户交货；独立 GR 系列与 Hash 链；含 RBC 要素（装/卸货地、开运时间）；**上路前向 AT 通信**取得文档识别码；可选被后续 **FT** 引用（1..N）；可作废（仅未挂真票）；进 SAF-T MovementOfGoods；幂等。

## 2. 非目标

| 项 | 说明 |
|----|------|
| 用 GR 替代 FT/FS/FR | 禁止 |
| 堂食 / 收银账单强制插 GR | P0 不做 |
| 全局运输单（未知收货人 / FAQ 5436） | P0 不做 |
| 行级「已开票数量」进度 | P0 不做（仅单据级 `N`/`A`/`F`） |
| GD、对 GR 开 NC | 不做；退货冲 FT |
| FS/FR 挂 GR | P0 禁止（仅 FT） |
| GT | 见 [`fiscal-gt.zh.md`](fiscal-gt.zh.md)；复用同一 `TransportClient` |

## 3. P0 定法

| 项 | 定法 |
|----|------|
| 性质 | 货物移动单据（RBC）；**非**销售税票 |
| 入口 | **仅手工** Admin |
| 系列 | 独立 `series.document_type='GR'`；须 AT `validation_code` |
| 门闸 | 有 ACTIVE GR 系列才可开；无 GR 系列 **不**挡 FT `ready_to_issue` |
| AT 通信 | **签发前必须成功**（与销售票「本机签」不同）；失败整单失败 |
| 通信豁免 | 收货方勾选「非 IVA 纳税人」时 **P0 仍通信**（简化；备选：豁免且不取文档码——非 P0） |
| 合规要素 | ATCUD、Hash、QR、装货地、卸货地、`movement_start_at`、AT 文档码 |
| 收货方 | 名称+地址 **必填**；IVA 纳税人 → 真实 NIF；非纳税人 → 无 NIF + 声明文案 |
| 禁止散客默认 | 禁止空 NIF 静默变 `999999990`（与 FS 不同） |
| 与 FT | 开 **FT** 时可选挂 1..N 张 GR；金额不轧平 |
| 一张 GR 多张 FT | **允许**；首挂 → `F`，其后保持 `F` |
| 作废 | 仅 `N`→`A`（须 reason）；`F`/`A` 禁止 |
| 签后补挂/解挂 | **禁止** |
| 权限 | 已登录可开票操作员（不要求 `can_issue_nc`） |
| 列表 / 营收 | GR 独立 Tab；**不计入**销售营收 |
| `at_env` | `mock`（CI/默认）\| `test` \| `prod`（与系列注册同开关语义；运输客户端独立实现） |

### 3.1 状态机（存 `invoices.document_status`，仅 `document_type=GR`）

| 值 | 含义 | 何时 |
|----|------|------|
| `N` | 正常未作废、尚未被 FT 引用 | `IssueGR` |
| `F` | 已开票（含部分） | 第一张挂本 GR 的 FT 签发成功 |
| `A` | 作废 | `AnnulGR` |

规则同 PF：`F`→`A` 禁止；`A` 不可被 FT 引用。

---

## 4. 唯一写路径

```text
签发 GR：
  Admin 手工 document_type=GR
    → service.IssueDeliveryNote
      → at.TransportClient.RegisterDocument（mock|test|prod）
      → 成功拿到 ATDocCodeID
      → store.IssueGR（同事务：系列序号 + Hash + ATCUD + 行 + 客户 + 装卸货地 + at_transport_doc_code）

作废：
  service.AnnulDeliveryNote → store.AnnulGR（仅 N→A）

开 FT 挂 GR：
  IssueDocument / IssueManual（document_type=FT）+ delivery_note_ids[]
    → store.IssueFT 事务内 linkDeliveryNotesInTx
```

| 层 | 唯一入口 | 禁止 |
|----|----------|------|
| 签发 | `service.IssueDeliveryNote` → `store.IssueGR` | 复用 `IssueFT` 冒充 GR |
| AT 通信 | `at.TransportClient` | handler 直连 SOAP；通信失败仍 INSERT |
| 引用 | `invoice_delivery_references` | 签后 UPDATE 补挂 |
| 打印 | `print.BuildPayload` + `RenderESCPOS`（GR 前缀） | |

---

## 5. 库表（落地迁移 `014_gr_delivery.sql`）

### 5.1 `invoices` 新增列

| 列 | 类型 | 必填 | 说明 |
|----|------|------|------|
| ship_from_address | TEXT | 否 | GR/GT 装货地全文；GR 签发时必填（应用层） |
| ship_to_address | TEXT | 否 | 卸货地全文；GR 签发时必填 |
| movement_start_at | TEXT | 否 | 运输开始 ISO；GR 签发时必填；不得早于签发时刻（AT 规则） |
| at_transport_doc_code | TEXT | 否 | AT 返回文档识别码；GR 签发成功后必有（mock 亦写伪码） |
| recipient_is_vat_payer | INTEGER | 否 | `1`=IVA 纳税人（须 NIF）；`0`=非纳税人（须声明）；仅 GR/GT |

复用已有：`status_reason`、`status_changed_at`（作废 / 状态变更）。

### 5.2 `invoice_delivery_references`

| 列 | 类型 | 必填 | 说明 |
|----|------|------|------|
| id | TEXT PK | 是 | |
| sale_invoice_id | TEXT FK | 是 | → invoices（**仅 FT**） |
| delivery_invoice_id | TEXT FK | 是 | → invoices（GR） |
| created_at | TEXT | 是 | UTC |

唯一：`(sale_invoice_id, delivery_invoice_id)`。  
一张 FT 可挂多 GR；一张 GR 可被多 FT 引用。

**唯一写法：** `linkDeliveryNotesInTx`（仅 `IssueFT` 且 `document_type=FT`）。

---

## 6. API（P0 形状）

### 6.1 签发

`POST /local/v1/fiscal-documents/manual`，`document_type=GR`。

| 字段 | 必填 | 说明 |
|------|------|------|
| request_id | 是 | 幂等 |
| document_type | 是 | `GR` |
| lines | 是 | 同行结构；金额可有 |
| customer | 是 | 名称+地址必填；NIF 见 §3 |
| recipient_is_vat_payer | 是 | bool |
| ship_from_address | 是 | 非空 |
| ship_to_address | 是 | 非空 |
| movement_start_at | 是 | 未来或「现在起允许的短窗」；禁止早于当前（对齐 AT） |
| station_id | 否 | |

**错误码：** `validation_failed`、`series_missing`、`at_transport_unavailable`、`at_transport_rejected`、`idempotency_conflict`。

### 6.2 作废

`POST /local/v1/fiscal-documents/{id}/annul`（与 PF 同路由；按 `document_type` 分流）。

### 6.3 开 FT 挂 GR

| 字段 | 必填 | 说明 |
|------|------|------|
| delivery_note_ids | 否 | `string[]`；仅当签发 **FT** 时合法 |

校验：每张存在、`document_type=GR`、`status∈{N,F}`、未作废；FS/FR 带该字段 → `validation_failed`。

---

## 7. AT 运输通信（与「开票审批」的区别）

| | 销售票（FT…） | 运输单据（GR/GT） |
|--|---------------|-------------------|
| 开票时 | **本机**占号/Hash/ATCUD | 本机签 **之前**须向 AT **注册在途单据** |
| 接口 | 系列注册 SOAP（已有 M1） | **另一套** `documentosTransporte` webservice |
| 结果 | 系列 `validation_code`（事先） | 每张单的 `ATDocCodeID`（当场） |

### 7.1 客户端

```text
at.TransportClient
  RegisterDocument(ctx, env, creds, doc) → (ATDocCodeID, raw)
```

| `FISCAL_AT_ENV` / 运输专用开关 | 行为 |
|-------------------------------|------|
| `mock`（默认） | 不访问网络；返回稳定伪码 `MOCK`+hash 前 8；CI 全绿 |
| `test` | HTTPS + 测试证 → `:701`（或门户现行测试港） |
| `prod` | 正式证 + 生产港 `:401`（或门户现行） |

系列注册客户端与运输客户端 **分文件**：`at/client.go`（系列）vs `at/transport.go`（本刀新增）。

### 7.2 官方端点（手册常见值；落地前再核门户）

| 环境 | URL |
|------|-----|
| 测试 | `https://servicos.portaldasfinancas.gov.pt:701/sgdtws/documentosTransporte` |
| 生产 | `https://servicos.portaldasfinancas.gov.pt:401/sgdtws/documentosTransporte` |

方法名：`envioDocumentoTransporte`（以 WSDL 为准）。

---

## 8. AT 测试接入 — 详细步骤

### 8.1 材料从哪里来

| 材料 | 是否要申请 | 怎么拿 |
|------|------------|--------|
| 测试客户端证书 `TesteWebservices.pfx` | **通常不用申请**（FAQ 4994：公开） | 下载官方 zip（见下） |
| 密码加密公钥 `ChavePublicaAT.cer` | 同上 | 同包 |
| 测试子用户 | 手册给定例 | 常用 `599999993/37` + `testes1234`（以现行手册为准） |
| WSDL | 公开 | Portal Apoio → Documentos de Transporte |
| **新 CA / RSA-4096 测试证** | 旧公开包过期后须换新 | Portal「Produtores de Software」或按手册邮件向 AT 索取 |

**下载结论（2026-10-15）：**

| 来源 | 结果 |
|------|------|
| 官方 `faturas.../certificados.zip` | 可下，但证已过期（2021 / 2023）→ 存 `stale-official-zip/` |
| 官方 Documents 下 `TesteWebservices.zip` / WSDL / `ChaveCifraPublicaAT2025.zip` | 当日 **404** |
| 社区镜像 [downloads.elsif.pt/AT_Certificados](http://downloads.elsif.pt/AT_Certificados/) | **有现行证**（FAQ 4994：测试证公开可分享） |

**本仓已采用的现行材料：**

| 文件 | 有效期 | 密码 |
|------|--------|------|
| `TesteWebservices.pfx`（= `TesteWebservices_20270106.pfx`） | **2026-07-10 → 2027-01-06** | `TESTEwebservice` |
| `ChavePublicaAT.cer`（= `ChaveCifraPublicaAT20250415.cer`） | **2025-04-15 → 2027-04-15** | — |

→ **不必再向 AT 申请测试证即可开始联调**（用本仓 `testdata`）。证过期后重跑刷新脚本或从镜像取更新文件名。生产环境仍须厂商正式通信证。

### 8.2 本仓已落盘位置

```text
apps/fiscal-agent/internal/fiscal/at/testdata/transport/
  README.md
  certificados/
    TesteWebservices.pfx      ← 现行（至 2027-01-06）
    ChavePublicaAT.cer        ← 现行（至 2027-04-15）
    current/                  ← 同上副本 + 镜像 README
    stale-official-zip/       ← 官方旧包（勿用）
  certificados.zip            ← 官方旧包原文
  documentosTransporte.wsdl   ← 社区镜像
```

刷新：

```bash
./scripts/fetch-at-transport-testdata.sh
```

### 8.3 联调清单（材料已齐）

1. **核对有效期**  
   ```bash
   openssl pkcs12 -in apps/fiscal-agent/internal/fiscal/at/testdata/transport/certificados/TesteWebservices.pfx \
     -passin pass:TESTEwebservice -nokeys -clcerts -legacy \
     | openssl x509 -noout -dates
   ```  
   `notAfter` 须晚于今天。

2. **转 PEM（可选）**  
   ```bash
   openssl pkcs12 -in .../TesteWebservices.pfx -out TesteWebservices.pem \
     -nodes -passin pass:TESTEwebservice -legacy
   ```

3. **配置运行参数（建议）**  
   | 变量 | 例 |
   |------|-----|
   | `FISCAL_AT_ENV` | `test` |
   | `FISCAL_AT_TRANSPORT_CERT` | `.../testdata/transport/certificados/TesteWebservices.pfx` |
   | `FISCAL_AT_TRANSPORT_CERT_PASS` | `TESTEwebservice` |
   | `FISCAL_AT_TRANSPORT_PUBKEY` | `.../ChavePublicaAT.cer` |
   | `FISCAL_AT_TRANSPORT_USER` | `599999993/37` |
   | `FISCAL_AT_TRANSPORT_PASS` | `testes1234` |
   | `FISCAL_AT_TRANSPORT_URL` | `https://servicos.portaldasfinancas.gov.pt:701/sgdtws/documentosTransporte` |

4. **连通性**  
   - 门户自检：`https://faturas.portaldasfinancas.gov.pt/testarLigacaoWebService.action`  
   - 本仓：实现后 `-tags=at_transport_live`（默认 skip）

5. **发一笔最小 SOAP**  
   - Header：UsernameToken；密码用 `ChavePublicaAT.cer` 按手册加密。  
   - Body：`MovementType=GR`、发/收货方、行、装卸货地、开运时间（**不得早于当前**）。  
   - 期望：响应含 `ATDocCodeID`。

6. **接到 `IssueDeliveryNote`**  
   通信成功 → 写库；失败 → `at_transport_rejected` / `at_transport_unavailable`，零发票行。

### 8.4 CI / 日常开发

| 场景 | 定法 |
|------|------|
| `go test ./...` / regression | **仅 mock** TransportClient |
| 开发者有有效测试证 | 可选 live 标签打真实 `:701` |
| 无有效证 | 不阻塞 GR 代码合并；`test` 环境启动时明确报「证书过期/缺失」 |

---

## 9. 实现步骤（分刀，可验收）

### 刀 G0 — 文档与材料（本刀）

- [x] 用途合订 [`fiscal-doc-types-usage.zh.md`](fiscal-doc-types-usage.zh.md)  
- [x] 本文定稿  
- [x] 官方 zip（过期）+ 社区镜像**现行**测试证（至 2027-01）落盘  
- [x] `scripts/fetch-at-transport-testdata.sh`（优先拉镜像最新 `TesteWebservices_*.pfx`）

### 刀 G1 — DDL + 领域类型

1. `migrations/014_gr_delivery.sql`（§5 列 + 引用表）  
2. 同步 `fiscal-sqlite-schema.zh.md`：`document_type` 含 `GR`；`document_status` 注明 GR 用 N/A/F  
3. `domain.DocumentGR`；`ParseSaleDocumentType` **不**把 GR 当销售类型；手工路径单独解析  
4. 验收：`go test` migrate 空库通过  

### 刀 G2 — `TransportClient` mock + SOAP 骨架

1. `at/transport.go`：接口 + Mock  
2. SOAP 骨架：读 WSDL、组 Header/Body（可先录制 golden XML）  
3. live 测试默认 skip  
4. 验收：mock 单测；无网络  

### 刀 G3 — `IssueGR` / `AnnulGR`

1. `store.IssueGR` / `AnnulGR`  
2. `service.IssueDeliveryNote`：先通信再写库  
3. 打印前缀 `Guia de remessa:`（对齐 `documentNoPrefix`）  
4. 回归：`scripts/fiscal-gr-regression.mjs`（GR-01、作废、NIF 门禁）  

### 刀 G4 — FT 挂 GR

1. `linkDeliveryNotesInTx`  
2. manual/IssueDocument 接受 `delivery_note_ids`（仅 FT）  
3. GR→`F`；多 FT 再挂  
4. 场景 GR-02…04、09  

### 刀 G5 — SAF-T MovementOfGoods

1. `saft_load` + `saft/build` 导出 GR  
2. 营收汇总排除 GR  
3. M5 regression 扩一条含 GR 月  

### 刀 G6 — Admin UI

1. 系列注册可选 GR  
2. 手工开票类型 GR + 装卸货地/开运时间/是否 IVA 纳税人  
3. GR Tab；详情作废 / 开 FT（预填+多选挂）  
4. 用语：业务词，不出现工程词  

### 刀 G7 — AT test 实打（有证后）

> **并入统一规划：** [`fiscal-at-live-integration.zh.md`](fiscal-at-live-integration.zh.md) **刀 A2**（建议先完成 A0 共享底座 + A1 系列 live）。

1. 换新证（若过期）  
2. `-tags=at_transport_live` 打通 `:701`  
3. 记录请求/响应样例（脱敏）入 `testdata/transport/fixtures/`  

**关闭条件：** G1–G6 全绿；G7/A2 有证则必须绿，无证则文档标明 blocked + mock 覆盖。

---

## 10. Admin / 打印 / SAF-T（摘要）

| 项 | 定法 |
|----|------|
| 票号前缀 | `Guia de remessa:` / 英文等价（`documentNoPrefix`） |
| 票面 | 须含装/卸货地、开运时间、AT 文档码；不可主标题成「Fatura」 |
| SAF-T | 4.2 MovementOfGoods，`MovementType=GR`；**不**进 4.1 |
| 未开票列表 | 可选：`document_type=GR AND document_status=N` |

---

## 11. 回归

| 脚本 / 测试 | 覆盖 |
|-------------|------|
| `go test ./internal/fiscal/...` | IssueGR、Annul、link、NIF、mock AT |
| `node scripts/fiscal-gr-regression.mjs` | 场景 GR-01…10（对齐场景文） |
| `-tags=at_transport_live` | 真实测试港（可选） |

---

## 12. 修订

| 日期 | 说明 |
|------|------|
| 2026-10-15 | G7 并入 [`fiscal-at-live-integration.zh.md`](fiscal-at-live-integration.zh.md) A2 |
| 2026-10-15 | 定稿：行为 + 实现分刀 + AT 通信 P0；下载公开 zip 并记录证书已过期；测试接入步骤与刷新脚本 |
| 2026-10-15 | 从 elsif 镜像取得现行 `TesteWebservices`（至 2027-01-06）与 `ChaveCifraPublicaAT`（至 2027-04-15）；更新 §8 |
