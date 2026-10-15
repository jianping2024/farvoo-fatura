# GT：运输单（Guia de transporte）

> **状态：定稿**（实现未落地）  
> **权威：是**（GT 行为、实现分刀、与 GR 边界；库列复用 GR 迁移列，落地时同步 [`fiscal-sqlite-schema.zh.md`](fiscal-sqlite-schema.zh.md)）  
> **对应实现：** 尚未落地（目标：`store.IssueGT` / `AnnulGT`、`service.IssueTransportNote`；复用 `at.TransportClient`）  
> **用途/当事人：** [`fiscal-doc-types-usage.zh.md`](fiscal-doc-types-usage.zh.md) §9  
> **测试场景：** [`fiscal-doc-type-test-scenarios.zh.md`](fiscal-doc-type-test-scenarios.zh.md) §10  
> **计划顺序：** RG → PF → GR → **GT**  
> **AT live：** [`fiscal-at-live-integration.zh.md`](fiscal-at-live-integration.zh.md)（运输 = A2；与 GR 共用）  
> **法律依据（摘要）：** 同 GR（DL 147/2003 RBC；Portaria 161/2013；AT FAQ Documentos de Transporte）；SAF-T 4.2 `MovementType=GT`

---

## 1. 目标

签发 **GT**：证明货物**在途**合规（调拨为主，也可外送）；独立 GT 系列与 Hash 链；RBC 要素（装/卸货地、开运时间）；**上路前向 AT 通信**取得文档识别码；**不**挂销售票；可作废（仅 `N`）；进 SAF-T MovementOfGoods；幂等。

## 2. 非目标

| 项 | 说明 |
|----|------|
| 用 GT 替代 FT/FS/FR | 禁止 |
| 用 FS「兼」运输单 | 禁止 |
| 堂食 / 账单强制插 GT | P0 不做 |
| GT ↔ FT/FS 软关联表 | P0 不做（外送销售另开税票，无强制引用） |
| 全局运输单（未知收货人） | P0 不做 |
| 行级在途进度 / 签收确认 | P0 不做 |
| 已上路补开（事后补录） | P0 默认禁止 |
| 把 GR 系列/写路径复用成 GT | 禁止；仅复用 `TransportClient` 与发票运输列 |

---

## 3. 与 GR 的边界（必须分清）

| 项 | GR | GT |
|----|----|----|
| 商业意思 | **交货给客户** | **货在途**（调拨 / 外送运输） |
| 收件方 | 收货客户 | 调拨=本店另一场所；外送=客户 |
| 与 FT | 开 FT 时可挂 1..N | **无**销售挂靠 |
| `document_status` | `N` / `F` / `A`（`F`=已挂 FT） | **仅 `N` / `A`**（P0 无 `F`） |
| 服务入口 | `IssueDeliveryNote` → `IssueGR` | `IssueTransportNote` → `IssueGT` |
| AT `MovementType` | `GR` | `GT` |
| 打印前缀 | Guia de remessa | Guia de transporte |

法定要素（装/卸、开运时间、上路前通信、NIF 规则）**相同**；差异在用途、当事人与挂票。

---

## 4. P0 定法

| 项 | 定法 |
|----|------|
| 性质 | 货物移动单据（RBC）；**非**销售税票 |
| 入口 | **仅手工** Admin |
| 系列 | 独立 `series.document_type='GT'`；须 `validation_code` |
| 门闸 | 有 ACTIVE GT 系列才可开；无 GT 系列 **不**挡 `ready_to_issue` |
| AT 通信 | **签发前必须成功**；失败整单失败；mock 写伪码 |
| 通信客户端 | **复用** `at.TransportClient`（`MovementType=GT`） |
| 合规要素 | ATCUD、Hash、QR、装货地、卸货地、`movement_start_at`、AT 文档码 |
| 场景 | 必选 `transfer` \| `customer`（见 §4.1） |
| 时点 | **上路前**；`movement_start_at` 不得早于签发时刻（与 AT 规则对齐）；UI 不提供「已上路补开」 |
| 与销售票 | **无强制、无可选挂靠** |
| 作废 | **允许**；仅 `N`→`A`（须 reason，1–50 字，与 PF/GR 同弹窗）；`A` 禁止再操作。GT 非销售发票，但是运输单据，可作废。**P0 mock：** 只按状态门禁（与 GR 同），**不**按开运时刻拒作废；live（A2）后再对齐 AT「开运时刻之后不可作废」 |
| 权限 | 已登录可开票操作员即可作废（**不**要求 `admin` / `can_issue_nc`；与 GR 同） |
| 列表 / 营收 | GT 独立 Tab；**不计入**销售营收 |
| setup 字段 | **`ready_to_gt`**（短名；JSON `ready_to_gt` / `gt_series_ok`） |
| 排期 | **先 T1–T4（mock）**；live 随 A0→A1→A2，不挡 GT 产品化 |
| `at_env` | 同 GR：`mock` \| `test` \| `prod` |

### 4.1 场景与收件方（P0）

| `movement_purpose` | 含义 | 收件方 TaxID | 名称 / 地址 |
|--------------------|------|--------------|-------------|
| `transfer` | 仓↔店、店↔店调拨 | **必须等于**本店 `taxpayer_settings.tax_registration_number` | 名称：**允许场所别名**（UI 默认可预填 `legal_name`）；**卸货地址必填** |
| `customer` | 外送到客户 | 同 GR：IVA 纳税人 → 真实 NIF；非纳税人 → 空 NIF + 声明 | 名称+地址必填；禁止静默 `999999990` |

禁止：`transfer` 却填客人 NIF；`customer` 却填本店 NIF 冒充外送（应用层校验）。

**装货地 vs 卸货地（P0 定法）：** **不硬拦**相同地址（法律只要求两处都标明，未强制字符串不同）。Admin 在 `transfer` 时 **轻提示**：「调拨时装卸地址宜不同」。

**外送与销售票（P0 定法）：** **无挂靠、不自动开票**。`customer` 签发成功后 Admin **一句轻提示**：「外送销售请另开发票」；可不提示关闭路径，不得强制跳转开 FT/FS。

### 4.2 状态机（仅 `document_type=GT`）

| 值 | 含义 | 何时 |
|----|------|------|
| `N` | 正常未作废 | `IssueGT` |
| `A` | 作废 | `AnnulGT` |

**不做 `F`：** GT 不挂 FT，无需「已开票」态。若以后要「运输完成」态，另开里程碑，禁止偷用 GR 的 `F` 语义。

---

## 5. 唯一写路径

```text
签发 GT：
  Admin 手工 document_type=GT
    → service.IssueTransportNote
      → 校验 movement_purpose + 收件方规则
      → at.TransportClient.RegisterDocument（MovementType=GT）
      → 成功拿到 ATDocCodeID
      → store.IssueGT

作废：
  service.AnnulTransportNote → store.AnnulGT（仅 N→A）
```

| 层 | 唯一入口 | 禁止 |
|----|----------|------|
| 签发 | `IssueTransportNote` → `IssueGT` | 走 `IssueDeliveryNote`/`IssueGR`；走 `IssueFT` |
| AT 通信 | `TransportClient.RegisterDocument` | handler 直连；失败仍 INSERT |
| 打印 | `BuildPayload` + `RenderESCPOS`（GT 前缀） | |

**与 GR 编排并列、不合并为一个 service 函数**（避免 `document_type` 分支泄漏进 GR 唯一路径）。共享的仅：运输列写入、`TransportClient`、打印运输块。

Annul HTTP：复用 `POST …/fiscal-documents/{id}/annul`，按 `document_type` 分流（已有 PF/GR；加 GT → `AnnulTransportNote`）。

---

## 6. 库表

### 6.1 复用（已有 `014_gr_delivery.sql`）

| 列 | 说明（GT） |
|----|------------|
| ship_from_address | 装货地；签发必填 |
| ship_to_address | 卸货地；签发必填 |
| movement_start_at | 开运时间；签发必填 |
| at_transport_doc_code | AT 文档码；签发成功后必有 |
| recipient_is_vat_payer | `customer` 场景用；`transfer` 固定按纳税人处理（存 `1`） |
| status_reason | 作废原因 |
| status_changed_at | 作废时刻 |

**不**使用 `invoice_delivery_references`（该表仅 FT→GR）。

### 6.2 新增列（落地迁移建议 `015_gt_transport.sql`）

| 列 | 类型 | 必填 | 说明 |
|----|------|------|------|
| movement_purpose | TEXT | 否 | `transfer` \| `customer`；**仅 GT 签发时应用层必填** |

一列一行；与 DDL 同步进 schema 文后再标「已落地」。

---

## 7. API（P0 形状）

### 7.1 签发

`POST /local/v1/fiscal-documents/manual`，`document_type=GT`。

| 字段 | 必填 | 说明 |
|------|------|------|
| request_id | 是 | 幂等 |
| document_type | 是 | `GT` |
| movement_purpose | 是 | `transfer` \| `customer` |
| lines | 是 | 同行结构 |
| customer_nif / name / address… | 视场景 | `transfer`：NIF=本店；`customer`：同 GR |
| recipient_is_vat_payer | `customer` 时是 | bool |
| ship_from_address | 是 | |
| ship_to_address | 是 | |
| movement_start_at | 是 | |

**错误码：** `validation_failed`、`series_missing`、`at_transport_unavailable`、`at_transport_rejected`、`idempotency_conflict`。

### 7.2 作废

同 GR：`POST /local/v1/fiscal-documents/{id}/annul`。

---

## 8. Admin / 打印 / SAF-T

| 项 | 定法 |
|----|------|
| 系列卡 | 可选注册 GT（同 PF/GR） |
| 手工类型 | 下拉含 GT；选中后出装卸货地 + `movement_purpose` |
| Tab | 发票列表 `document_type=GT` |
| 票号前缀 | `Guia de transporte:` / `Transport guide:` |
| 票面 | 装/卸、开运时间、AT 文档码；调拨可标「Transferência」类短句（业务词） |
| 调拨地址提示 | `transfer`：装卸相同不拦；轻提示宜不同 |
| 外送销售提示 | `customer` 签发成功后一句：「外送销售请另开发票」 |
| SAF-T | 4.2 MovementOfGoods，`MovementType=GT`；**不**进 4.1 |
| 营收 | 排除 GT（与 GR/PF/RG 同） |

---

## 9. 实现分刀

### 刀 T0 — 本文定稿

- [x] 本文件  
- [x] 用途文 §9 / 场景文 §10 / 计划索引交叉链接

### 刀 T1 — DDL + 领域

1. `015_gt_transport.sql`：`movement_purpose`  
2. schema：`document_type` 含 `GT`；GT 状态仅 `N`/`A`  
3. `domain.DocumentGT`；列表 Tab；`ParseSaleDocumentType` **不**含 GT  

### 刀 T2 — `IssueGT` / `AnnulGT` + service

1. `store.IssueGT` / `AnnulGT`（照抄 GR 结构，`MovementType`/`document_type`=GT；无 delivery refs）  
2. `service.IssueTransportNote` / `AnnulTransportNote`  
3. TransportClient 传 `MovementType=GT`  
4. 打印前缀 GT  

### 刀 T3 — API / Admin / 营收 / SAF-T

1. manual 分流 `document_type=GT`  
2. annul 分流  
3. setup：`gt_series_ok` + **`ready_to_gt`**（短名已拍板）  
4. SAF-T load/build 含 GT（可与 GR 同 reader，按 `document_type` 写 MovementType）  
5. 营收 `NOT IN (…,'GT')`  
6. Admin：`transfer` 装卸提示；`customer` 签发成功轻提示另开发票

### 刀 T4 — 回归

`scripts/fiscal-gt-regression.mjs`：GT-01…08（mock）；调拨 NIF=本店；外送 NIF 门禁；作废；SAF-T；无 FT 关联。

### 刀 T5 — live（非 GT 独有）

并入 [`fiscal-at-live-integration.zh.md`](fiscal-at-live-integration.zh.md) **A2**；不单开运输 SOAP。

**关闭条件：** T0–T4 全绿；T5 随 A2。

---

## 10. 修订

| 日期 | 说明 |
|------|------|
| 2026-10-15 | 定稿：与 GR 边界、transfer/customer、状态仅 N/A、唯一路径、分刀 T0–T5 |
| 2026-10-15 | 钉死：`ready_to_gt`；装卸不硬拦+提示；调拨允许别名；外送轻提示另开票；作废=N→A 与 GR 同权限；先 mock T1–T4 |
