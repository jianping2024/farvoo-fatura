# Fiscal Agent SQLite 数据库设计（P0）

> **状态：定稿**  
> **权威：是**（库表设计；与 DDL 冲突时以 DDL 为准并回改本文）  
> **对应实现：** `apps/fiscal-agent/internal/fiscal/store/migrations/001_init.sql` + `002_bill_sync_drafts.sql`  
> **写作规范：** [`docs/design-doc-standards.zh.md`](design-doc-standards.zh.md)

> 权威库：门店本机 Agent 唯一 SQLite。  
> 范围：纯打发票 + 税务打印队列；业务云端 `print_jobs` 不进本库  
> 依据：需求 v0.17、Farvoo Fiscal 对接说明、华人零售 POS V0.1（边界对齐、不建库存）

配套：

- 实现迁移：`apps/fiscal-agent/internal/fiscal/store/migrations/001_init.sql`、`002_bill_sync_drafts.sql`
- 包内摘要：`apps/fiscal-agent/internal/fiscal/store/SCHEMA.md`

---

## 1. 目标与非目标

### 目标

- 门店唯一税务权威：系列号、InvoiceNo、Hash、ATCUD、QR、签后快照  
- 签发与 ORIGINAL 打印任务同事务  
- 手动开票所需薄商品/客户主档  
- SAF-T 月报导出归档  
- 操作员本地 PIN（离线开票）与审计  

### 非目标（禁止建表）

| 不做 | 权威在哪 |
|------|----------|
| 库存 / 流水 / 预留 | 零售 POS / Farvoo 业务 |
| 条码 / 采购价 / 供应商 | 同上 |
| Order / Payment 业务主档 / 履约 | Farvoo / POS |
| 云端 `print_jobs` 权威队列 | Farvoo（仅业务热敏） |

---

## 2. 全局约定

| 项 | 定法 |
|----|------|
| 主键 | `TEXT` UUID，应用层生成 |
| 金额 / 数量 / 税率 | `TEXT` 十进制字符串（金额两位如 `"12.50"`；税率如 `"0.23"`）；**禁止 `REAL`/`FLOAT`** |
| 时间 | 见下节 **§2.1**；库内时刻 UTC；`InvoiceDate` / 票面按门店时区 |
| 布尔 | `INTEGER` 0/1 |
| JSON | `TEXT`（如 `print_payload`、`display_meta`） |
| 外键 | `PRAGMA foreign_keys=ON` |
| 并发 | 签发：`BEGIN IMMEDIATE` + 系列行锁；WAL + `busy_timeout` |
| 签后不可变 | `invoices` / `invoice_lines` / `invoice_customer_snapshots` / Hash / ATCUD / QR / 金额 **禁止业务 UPDATE**；冲销只开 NC |
| 与 `config.json` 分工 | 配对 JWT、档口打印机、Realtime → JSON；商家 NIF、系列、密钥、开票人 PIN → **本 SQLite** |

### 2.1 时间：UTC 存储 vs 门店时区

**库内存 UTC，不等于票面显示 UTC。** 首发市场里斯本；扩展其他欧洲国家时仍用同一套规则。

| 用途 | 定法 |
|------|------|
| `created_at` / `printed_at` / `audit_log.at` / sync 时间 | `TEXT` ISO-8601，**UTC**（建议带 `Z`） |
| 门店时区 | `taxpayer_settings.timezone`，IANA 名；P0 默认 `Europe/Lisbon` |
| **InvoiceDate** | 按门店时区的**日历日** `YYYY-MM-DD`（里斯本的「今天」，不是 UTC 日期） |
| **SystemEntryDate** | 签发瞬间；与 Hash 拼接、SAF-T **同一字符串**；实现可存 UTC 再格式化，或存带偏移的本地时刻，但三者必须一致 |
| 票面 / Admin UI | 展示时转到门店时区 |

**禁止：**

- 把里斯本墙钟当 UTC 直接写入（会偏 1h，夏令时更糟）
- 用 UTC 日期当 `InvoiceDate`（UTC 已过午夜、里斯本可能仍是前一天）
- 签名用一种时间格式、SAF-T 用另一种

**扩展其他国家：** 只改门店 `timezone`（如 `Europe/Madrid`），不必改存库策略。

### 2.2 双钥模型：产品 RSA vs 设备绑定钥（TPM）

**签发票用的 RSA，和绑设备用的密钥，是两套东西。** 不要混成「整张表 RSA 加密」。

#### 密钥有几份？

| # | 名称 | 谁持有 | 存哪里 | 算法 / 形态 | 干什么 |
|---|------|--------|--------|-------------|--------|
| A | **产品税务签名私钥** | 软件运营方（Farvoo） | 运营方密钥库（**不进**门店安装包明文） | AT 强制：**RSA-1024 + SHA-1** | 对发票文本做 Hash 签名 |
| A' | **产品公钥** | 运营方 + AT（Modelo 24 已交）+ 门店可读副本 | AT 备案；门店 `signing_keys.public_key_pem` | RSA 公钥 | AT/第三方验签；本机自检 |
| B | **设备私钥** | **仅本机** | **TPM 芯片内**（优先）或 Windows DPAPI 保护的软件密钥 | 由 TPM/CNG 决定，**不要求是 RSA-SHA1** | 解开 A 的封装；私钥理想状态 **不可导出** |
| B' | **设备公钥** | 本机生成后发给运营方 | `agent_installations.device_public_key` | 与 B 成对 | 运营方用来把 A **封装成只给这台机解得开的密文** |
| C | **wrapped_fiscal_key** | 门店 Agent | `signing_keys.wrapped_private_key`（SQLite） | 「用 B' 包起来的 A」密文 | 日常离线签发时，用本机 B 解封 → **仅内存**得到 A 来签名 |

同一产品私钥版本（如 `signing_key_version=1`）可下发到很多店；**每店每台主机**各有自己的 B/B' 和一份 **针对该机** 的 C。

```text
运营方保管：A（产品私钥母本）
     │
     │ 用各店的 B'（设备公钥）分别封装
     ▼
门店 SQLite：C = wrapped_fiscal_key（密文，拷走也解不开，除非有 B）
门店 TPM：   B = 设备私钥（不出芯片）
开票瞬间：   B 解 C → 内存中的 A → RSA-SHA1 签 Hash → A 不写回明文盘
```

#### TPM 是什么、怎么跑？

**TPM（Trusted Platform Module）** 是主板上的安全芯片（或固件 TPM）。在 Windows 上通过 **CNG**（Cryptography API: Next Generation）访问，常用 Provider：`Microsoft Platform Crypto Provider`。

运行机制（概念）：

1. Agent 首次激活时请求 TPM：**在芯片里生成设备密钥对**，私钥标记为 **non-exportable**（导不出来）。
2. 公钥（B'）导出 → 写入 `agent_installations` → 发给运营方。
3. 运营方用 B' 把产品私钥 A **wrap** 成密文 C，下发到门店。
4. 开票时 Agent 调 TPM：「用芯片里的 B 解开 C」→ 得到 A 只在内存用一次（或短时缓存策略由实现定，**禁止明文落盘**）。
5. 用 A 做 **RSA-SHA1** 签出发票 Hash → 写入 `invoices.hash`。

**无 TPM 时（降级）：** `key_protection_level=SOFTWARE`，设备钥用软件 KSP + **DPAPI**（绑 Windows 用户/机器）。仍比明文好，但拷走整机用户配置的风险高于 TPM。记录在 `agent_installations.key_protection_level`。

#### 两张表怎么分工？

| 表 | 记什么 | 不记什么 |
|----|--------|----------|
| **`signing_keys`** | 产品钥**版本**、公钥、**wrapped 密文 C**、ACTIVE/RETIRED | 不负责「哪台机器」；不存设备私钥 |
| **`agent_installations`** | **哪次安装/哪台机**、设备公钥 B'、TPM 还是 SOFTWARE、绑了哪版产品钥、是否吊销 | 不存产品私钥明文；**不是**用 RSA 加密整张表 |

`agent_installations` **不是**「RSA 加密表」；它是 **设备授权档案**。RSA 只出现在「用产品私钥 A 签发票」这一步。

#### 换机时发生什么？

1. 旧机 `revoked_at` 置上 → 旧 B 即使还在也不能再被运营信任。  
2. 新机生成新的 B/B'，新 `installation_id`。  
3. 运营方审批后，用 **新 B'** 重新封装 **同一 `signing_key_version` 的 A** → 新 C。  
4. 数据库备份可恢复发票与系列；**旧 C 不能在新机解开**，必须重新授权。

#### 禁止

- 产品私钥 A 明文进 Git、安装包、日志、普通配置文件  
- 提供通用 `sign(任意数据)` API（只允许 Fiscal Core 对合规签名字符串调用）  
- 把「设备绑定」和「发票 RSA-SHA1」混成一种算法一种表  

---

## 3. 表清单

```text
配置 / 身份
  taxpayer_settings
  at_credentials
  signing_keys
  agent_installations
  operators

主档（可改；不影响已签发票）
  customers
  fiscal_product_categories
  fiscal_products

税务权威（签后不可变）
  series
  invoices
  invoice_lines
  invoice_customer_snapshots
  invoice_payments
  invoice_line_references

幂等 / 打印 / 审计
  idempotency_keys
  local_print_jobs
  print_attempts
  audit_log

导出 / 同步
  saft_exports
  sync_outbox

账单同步草稿（云端 bill_sync_jobs → 本地；未开票）
  bill_sync_drafts
```

---

## 4. ER（核心）

```text
series 1───N invoices 1───N invoice_lines
                 │              └──0..1 invoice_line_references → 原 invoice_lines
                 ├──1──1 invoice_customer_snapshots
                 └──1──N invoice_payments

idempotency_keys ──► invoices
invoices 1───N local_print_jobs 1───N print_attempts
operators.id = invoices.source_id
fiscal_products.product_code ◄── invoice_lines.product_code（历史行靠快照，不强制 FK）
bill_sync_drafts ──(upsert by item_code)──► fiscal_products
```

---

## 5. 枚举

| 字段 | 取值 |
|------|------|
| `document_type` | `FT` `FS` `FR` `NC` `ND` |
| `series.status` | `PENDING` `ACTIVE` `FAILED` `TERMINATED` |
| `document_status` | `SIGNED` `CREDITED_PARTIAL` `CREDITED_FULL` |
| `print_status`（单据） | `NOT_PRINTED` `PENDING` `PROCESSING` `PRINTED` `PRINT_FAILED` `REPRINTED` |
| `job_status`（任务） | `PENDING` `PROCESSING` `PRINTED` `FAILED_BEFORE_WRITE` `UNKNOWN_AFTER_WRITE` `FAILED` |
| `print_purpose` | `ORIGINAL` `REPRINT` |
| `product_type` | `P` `S` `O` |
| `product_source` | `REMOTE_SYNC` `LOCAL` |
| `payment_method` | `CASH` `CARD` `MBWAY` `MULTIBANCO` `MIXED` `OTHER` |
| `completeness_status` | `SYSTEM_DEFAULT` `COMPLETE` `INCOMPLETE` `INVALID` |
| `operator.role` | `owner` `frontdesk` `cashier` |
| `signing_keys.status` | `ACTIVE` `RETIRED` `COMPROMISED` |
| `sync_outbox.status` | `PENDING` `SENT` `FAILED` |
| `saft validation_status` | `PENDING` `VALID` `INVALID` |
| `bill_sync_drafts.status` | `open` `discarded`（开票成功后**硬删**该 `source_sale_id` 全部行，不保留 `invoiced`） |

---

## 6. 表字段设计

书写规则：一列一行；与 `migrations/001_init.sql` 对齐。见 `docs/design-doc-standards.zh.md`。

### 6.1 `taxpayer_settings`（开票主体，单行/按 store）

| 列 | 类型 | 必填 | 说明 |
|----|------|------|------|
| id | TEXT PK | 是 | UUID |
| store_id | TEXT UQ | 是 | 与 Farvoo restaurant / 门店对应 |
| tax_registration_number | TEXT | 是 | 商家 NIF 9 位 |
| legal_name | TEXT | 是 | 法律名称 → SAF-T CompanyName |
| business_name | TEXT | 否 | 商业名称 |
| address_detail | TEXT | 是 | |
| city | TEXT | 是 | |
| postal_code | TEXT | 是 | |
| country | TEXT | 是 | 默认 `PT` |
| timezone | TEXT | 是 | IANA，默认 `Europe/Lisbon`；算 InvoiceDate 与票面展示 |
| phone | TEXT | 否 | 票面 |
| software_certificate_number | TEXT | 是 | 未认证前可用 `0` |
| product_id | TEXT | 是 | 如 `Farvoo/InvoiceEngine` |
| product_version | TEXT | 是 | |
| fs_amount_threshold | TEXT | 是 | 默认 `"100.00"`，可配置 |
| tax_country_region | TEXT | 是 | 默认 `PT` |
| created_at | TEXT | 是 | UTC |
| updated_at | TEXT | 是 | UTC |

### 6.2 `at_credentials`（AT SOAP，加密）

| 列 | 类型 | 必填 | 说明 |
|----|------|------|------|
| id | TEXT PK | 是 | |
| store_id | TEXT UQ | 是 | |
| username | TEXT | 是 | 如 `NIF/子用户号` |
| password_ciphertext | TEXT | 是 | 密文；禁止明文 |
| salt | TEXT | 否 | 独立列。仅 KDF（如 PBKDF2）时使用；**P0（DPAPI）固定 NULL**。不是 AES IV。 |
| wrap_meta | TEXT | 否 | 独立列。JSON。**P0：** `{"scheme":"dpapi","v":1}`。自管 AES-GCM 时 IV 写在 JSON 的 `iv` 字段（非 P0）。 |
| last_ok_at | TEXT | 否 | 最近一次 AT 调用成功 |
| last_error | TEXT | 否 | 最近错误摘要；禁止含密码 |
| created_at | TEXT | 是 | UTC |
| updated_at | TEXT | 是 | UTC |

**P0 定法：** `salt=NULL`，`wrap_meta={"scheme":"dpapi","v":1}`，无单独 AES IV。

### 6.3 `signing_keys`（产品税务签名钥 — 版本与 wrapped 密文）

> 双钥模型见 **§2.2**。本表存产品 RSA 钥版本与门店侧密文，不是设备私钥。

| 列 | 类型 | 必填 | 说明 |
|----|------|------|------|
| id | TEXT PK | 是 | |
| key_version | INTEGER UQ | 是 | MVP=`1` → SAF-T `HashControl`；换钥递增 |
| public_key_pem | TEXT | 是 | 产品公钥（与 Modelo 24 / AT 备案一致） |
| wrapped_private_key | TEXT | 是 | **C**：设备绑定后的产品私钥密文；禁止明文私钥 |
| status | TEXT | 是 | ACTIVE / RETIRED / COMPROMISED |
| created_at | TEXT | 是 | UTC |
| retired_at | TEXT | 否 | |
| submitted_to_at_at | TEXT | 否 | 公钥提交 AT 的时间 |

**作用：** 开票时解封 → RSA-SHA1 签 Hash。历史票导出必须用开票当时的 `hash_control`，不得用新钥重算旧 Hash。

### 6.4 `agent_installations`（设备安装授权）

> 不是「RSA 加密表」。记哪台机器、哪次安装被授权；设备私钥在 TPM/DPAPI，不进本表明文。

| 列 | 类型 | 必填 | 说明 |
|----|------|------|------|
| installation_id | TEXT PK | 是 | 本次安装唯一 ID；换机则新 ID |
| store_id | TEXT | 是 | |
| taxpayer_nif | TEXT | 是 | |
| device_id | TEXT | 是 | 本机设备标识 |
| device_public_key | TEXT | 是 | **B'**：设备公钥 |
| hardware_fingerprint | TEXT | 否 | 辅助识别机器 |
| key_protection_level | TEXT | 是 | `TPM`（优先）或 `SOFTWARE` |
| signing_key_version | INTEGER | 是 | 对应 `signing_keys.key_version` |
| provisioned_at | TEXT | 是 | 激活完成时间 |
| revoked_at | TEXT | 否 | 非空则禁止继续解钥签发 |

### 6.5 `operators`（开票人）

| 列 | 类型 | 必填 | 说明 |
|----|------|------|------|
| id | TEXT PK | 是 | = SourceID |
| mesa_user_id | TEXT UQ | 是 | Farvoo user UUID |
| store_id | TEXT | 是 | |
| role | TEXT | 是 | P0：`owner`（店主）或 `cashier`；Farvoo `frontdesk` 同步时映射为 `cashier` |
| display_name | TEXT | 是 | |
| active | INTEGER | 是 | 默认 1 |
| pin_hash | TEXT | 否 | 未设则不能离线 PIN 登录 |
| can_issue_nc | INTEGER | 是 | 默认 0；店主可开 |
| synced_at | TEXT | 否 | |
| created_at | TEXT | 是 | UTC |
| updated_at | TEXT | 是 | UTC |

### 6.6 `customers`

| 列 | 类型 | 必填 | 说明 |
|----|------|------|------|
| id | TEXT PK | 是 | 内部 CustomerID |
| customer_tax_id | TEXT | 是 | NIF；散客 `999999990` |
| company_name | TEXT | 是 | |
| address_detail | TEXT | 是 | 默认 `Desconhecido` |
| city | TEXT | 是 | 默认 `Desconhecido` |
| postal_code | TEXT | 是 | 默认 `Desconhecido` |
| country | TEXT | 是 | 默认 `PT` |
| account_id | TEXT | 是 | 默认 `Desconhecido` |
| self_billing_indicator | INTEGER | 是 | 默认 0 |
| completeness_status | TEXT | 是 | |
| created_at | TEXT | 是 | UTC |
| updated_at | TEXT | 是 | UTC |

唯一：非散客 NIF 建议唯一（见 DDL 部分唯一索引）。  
种子行：`CONSUMIDOR_FINAL` / `999999990` / `Consumidor final` / `SYSTEM_DEFAULT`。

### 6.7 `fiscal_product_categories`

| 列 | 类型 | 必填 | 说明 |
|----|------|------|------|
| id | TEXT PK | 是 | |
| parent_id | TEXT | 否 | FK → 本表 |
| name_zh | TEXT | 否 | |
| name_pt | TEXT | 否 | |
| name_en | TEXT | 否 | |
| sort_order | INTEGER | 是 | 默认 0 |
| source | TEXT | 是 | REMOTE_SYNC 或 LOCAL |
| active | INTEGER | 是 | 默认 1 |

### 6.8 `fiscal_products`（薄投影）

| 列 | 类型 | 必填 | 说明 |
|----|------|------|------|
| id | TEXT PK | 是 | |
| product_code | TEXT UQ | 是 | → SAF-T ProductCode |
| category_id | TEXT | 否 | |
| display_name | TEXT | 否 | 中文等，可进小票位图 |
| name_pt | TEXT | 否 | 合规名来源之一 |
| name_en | TEXT | 否 | 合规名来源之一 |
| saft_name | TEXT | 是 | 签前固化葡/英；Windows-1252 |
| product_type | TEXT | 是 | 默认 P |
| unit_of_measure | TEXT | 是 | 默认 UN |
| unit_price_gross | TEXT | 是 | 含税售价 |
| vat_rate | TEXT | 是 | |
| tax_code | TEXT | 是 | RED / INT / NOR / ISE |
| source | TEXT | 是 | REMOTE_SYNC 只读；LOCAL 可改 |
| remote_item_id | TEXT | 否 | Farvoo 菜品 id |
| active | INTEGER | 是 | 默认 1 |
| created_at | TEXT | 是 | UTC |
| updated_at | TEXT | 是 | UTC |

**不含：** 库存数量、条码、采购价。

### 6.9 `series`

| 列 | 类型 | 必填 | 说明 |
|----|------|------|------|
| id | TEXT PK | 是 | |
| store_id | TEXT | 是 | |
| document_type | TEXT | 是 | FT / NC / … |
| series_code | TEXT | 是 | 如 `FT2026ABC01` |
| validation_code | TEXT | 否 | AT 8 位；未注册可空 |
| fiscal_year | INTEGER | 是 | |
| last_number | INTEGER | 是 | 已用最大序号，默认 0；下一张 = +1 |
| last_hash | TEXT | 是 | 上一张完整 Base64 Hash；首张空串 |
| status | TEXT | 是 | |
| registered_at | TEXT | 否 | |
| created_at | TEXT | 是 | UTC |
| updated_at | TEXT | 是 | UTC |

唯一：`(store_id, series_code)`。

### 6.10 `invoices`

| 列 | 类型 | 必填 | 说明 |
|----|------|------|------|
| id | TEXT PK | 是 | document_id |
| store_id | TEXT | 是 | |
| document_type | TEXT | 是 | |
| series_id | TEXT FK | 是 | → series |
| series_code | TEXT | 是 | 冗余快照 |
| sequence_number | INTEGER | 是 | |
| invoice_no | TEXT UQ | 是 | 如 `FT FT2026ABC01/1` |
| atcud | TEXT | 是 | `validation-sequence`，无前缀 |
| hash | TEXT | 是 | Base64 RSA-SHA1，≤172 |
| hash_control | INTEGER | 是 | 密钥版本 |
| signing_key_version | INTEGER | 是 | |
| previous_hash | TEXT | 是 | 默认空串 |
| qr_content | TEXT | 是 | 完整 QR 字符串 |
| invoice_date | TEXT | 是 | YYYY-MM-DD（门店时区日历日） |
| system_entry_date | TEXT | 是 | 与 Hash / SAF-T 同源 |
| document_status | TEXT | 是 | |
| print_status | TEXT | 是 | 汇总态 |
| gross_total | TEXT | 是 | |
| net_total | TEXT | 是 | |
| tax_payable | TEXT | 是 | |
| customer_id | TEXT | 否 | → customers |
| source_id | TEXT | 是 | → operators.id |
| software_certificate_number | TEXT | 是 | 开票时冻结 |
| source_system | TEXT | 否 | 业务幂等 |
| source_sale_id | TEXT | 否 | 业务幂等 |
| scope_type | TEXT | 否 | 业务幂等 |
| scope_id | TEXT | 否 | 业务幂等 |
| fiscal_purpose | TEXT | 否 | 业务幂等 |
| external_bill_id | TEXT | 否 | |
| display_meta_json | TEXT | 否 | 桌号展示名等，不进 SAF-T |
| credited_gross_total | TEXT | 是 | 默认 `"0.00"` |
| created_at | TEXT | 是 | 签发提交时间 UTC |

### 6.11 `invoice_lines`

| 列 | 类型 | 必填 | 说明 |
|----|------|------|------|
| id | TEXT PK | 是 | |
| invoice_id | TEXT FK | 是 | |
| line_number | INTEGER | 是 | 从 1 |
| product_code | TEXT | 是 | |
| product_description | TEXT | 是 | = saft_name 冻结 |
| display_name | TEXT | 否 | 小票中文 |
| quantity | TEXT | 是 | |
| unit_of_measure | TEXT | 是 | 默认 UN |
| unit_price_gross | TEXT | 是 | |
| unit_price_net | TEXT | 是 | |
| line_gross | TEXT | 是 | |
| line_net | TEXT | 是 | |
| line_tax | TEXT | 是 | |
| vat_rate | TEXT | 是 | |
| tax_type | TEXT | 是 | 默认 IVA |
| tax_country_region | TEXT | 是 | 默认 PT |
| tax_code | TEXT | 是 | |
| tax_exemption_code | TEXT | 否 | 0% 时与 reason 成对 |
| tax_exemption_reason | TEXT | 否 | 0% 时与 code 成对 |
| product_type | TEXT | 是 | 默认 P |

唯一：`(invoice_id, line_number)`。

### 6.12 `invoice_customer_snapshots`

| 列 | 类型 | 必填 | 说明 |
|----|------|------|------|
| invoice_id | TEXT PK FK | 是 | |
| customer_tax_id | TEXT | 是 | |
| company_name | TEXT | 是 | |
| address_detail | TEXT | 是 | |
| city | TEXT | 是 | |
| postal_code | TEXT | 是 | |
| country | TEXT | 是 | |
| account_id | TEXT | 是 | 默认 `Desconhecido` |
| self_billing_indicator | INTEGER | 是 | 默认 0 |

### 6.13 `invoice_payments`

| 列 | 类型 | 必填 | 说明 |
|----|------|------|------|
| id | TEXT PK | 是 | |
| invoice_id | TEXT FK | 是 | |
| method | TEXT | 是 | |
| amount | TEXT | 是 | |
| paid_at | TEXT | 是 | |
| operator_id | TEXT | 否 | |

MVP 不进 SAF-T `DocumentTotals/Payment`。

### 6.14 `invoice_line_references`（NC）

| 列 | 类型 | 必填 | 说明 |
|----|------|------|------|
| id | TEXT PK | 是 | |
| credit_line_id | TEXT UQ FK | 是 | → invoice_lines（NC 行） |
| original_invoice_id | TEXT FK | 是 | |
| original_invoice_no | TEXT | 是 | 完整 InvoiceNo |
| original_line_id | TEXT | 是 | |
| original_line_number | INTEGER | 是 | |
| reason | TEXT | 是 | References.Reason |

### 6.15 `idempotency_keys`

| 列 | 类型 | 必填 | 说明 |
|----|------|------|------|
| id | TEXT PK | 是 | |
| store_id | TEXT | 是 | |
| request_id | TEXT | 是 | |
| request_payload_hash | TEXT | 是 | 同 request 不同 payload → 冲突 |
| business_key | TEXT | 是 | `source_system|source_sale_id|scope_type|scope_id|fiscal_purpose` |
| invoice_id | TEXT | 否 | 成功后绑定 |
| created_at | TEXT | 是 | |

唯一：`(store_id, request_id)`；`(store_id, business_key)`。

### 6.16 `local_print_jobs`（税务打印权威队列）

| 列 | 类型 | 必填 | 说明 |
|----|------|------|------|
| id | TEXT PK | 是 | print_job_id |
| invoice_id | TEXT FK | 是 | |
| document_type | TEXT | 是 | |
| print_purpose | TEXT | 是 | ORIGINAL / REPRINT |
| job_status | TEXT | 是 | |
| logical_role | TEXT | 是 | 默认 `fiscal_receipt_printer` |
| payload_json | TEXT | 是 | 冻结 print_payload v1 |
| payload_hash | TEXT | 是 | |
| attempts | INTEGER | 是 | 默认 0 |
| last_error | TEXT | 否 | |
| created_at | TEXT | 是 | UTC |
| updated_at | TEXT | 是 | UTC |
| printed_at | TEXT | 否 | |
| created_by | TEXT | 否 | 重打操作员 |

### 6.17 `print_attempts`

| 列 | 类型 | 必填 | 说明 |
|----|------|------|------|
| id | TEXT PK | 是 | |
| print_job_id | TEXT FK | 是 | |
| attempted_at | TEXT | 是 | |
| result | TEXT | 是 | |
| error_code | TEXT | 否 | |
| error_message | TEXT | 否 | |
| device_hint | TEXT | 否 | 打印机目标摘要 |

### 6.18 `audit_log`

| 列 | 类型 | 必填 | 说明 |
|----|------|------|------|
| id | TEXT PK | 是 | |
| at | TEXT | 是 | |
| operator_id | TEXT | 否 | |
| action | TEXT | 是 | ISSUE / REPRINT / NC / EXPORT_SAFT / LOGIN … |
| entity_type | TEXT | 否 | |
| entity_id | TEXT | 否 | |
| detail_json | TEXT | 否 | 无密钥明文 |

### 6.19 `saft_exports`

| 列 | 类型 | 必填 | 说明 |
|----|------|------|------|
| id | TEXT PK | 是 | |
| store_id | TEXT | 是 | |
| taxpayer_nif | TEXT | 是 | |
| period_year | INTEGER | 是 | |
| period_month | INTEGER | 是 | |
| start_date | TEXT | 是 | |
| end_date | TEXT | 是 | |
| file_name | TEXT | 是 | |
| file_path | TEXT | 否 | 本地路径 |
| file_sha256 | TEXT | 否 | |
| invoice_count | INTEGER | 是 | 默认 0 |
| total_net | TEXT | 否 | |
| total_tax | TEXT | 否 | |
| total_gross | TEXT | 否 | |
| validation_status | TEXT | 是 | |
| validation_errors | TEXT | 否 | |
| created_by | TEXT | 否 | |
| created_at | TEXT | 是 | |
| submitted_at | TEXT | 否 | |
| at_receipt_number | TEXT | 否 | |
| at_receipt_file_path | TEXT | 否 | |

### 6.20 `sync_outbox`

| 列 | 类型 | 必填 | 说明 |
|----|------|------|------|
| id | TEXT PK | 是 | |
| store_id | TEXT | 是 | |
| event_type | TEXT | 是 | INVOICE_ISSUED / PRINT_STATUS / NC … |
| payload_json | TEXT | 是 | 副本字段，无私钥 |
| status | TEXT | 是 | |
| attempts | INTEGER | 是 | 默认 0 |
| next_attempt_at | TEXT | 否 | |
| last_error | TEXT | 否 | |
| created_at | TEXT | 是 | |
| sent_at | TEXT | 否 | |

云端冲突以 Agent 为准回写；失败不阻断本地开票。

### 6.21 `bill_sync_drafts`（账单同步草稿）

Farvoo `bill_sync_jobs` 经 Agent 唯一路径 `billsync.PullAndIngest` → `IngestCloudJob` → `UpsertBillDraftOpen` 写入。不同时开票。

| 列 | 类型 | 必填 | 说明 |
|----|------|------|------|
| id | TEXT PK | 是 | UUID |
| request_id | TEXT | 是 | 幂等键；UNIQUE |
| source_sale_id | TEXT | 是 | Farvoo sale id |
| payload_json | TEXT | 是 | 完整 Snapshot JSON |
| status | TEXT | 是 | 仅 `open` / `discarded`（覆盖时旧 open→discarded） |
| cloud_job_id | TEXT | 否 | Farvoo job id |
| last_error | TEXT | 否 | |
| created_at | TEXT | 是 | UTC |
| updated_at | TEXT | 是 | UTC |

**唯一写路径：**

| 动作 | 唯一入口 |
|------|----------|
| 入站/覆盖草稿 | `store.UpsertBillDraftOpen` |
| 开票成功清临时数据 | `store.DeleteBillDraftsBySale`（硬删该 `source_sale_id` 全部行） |
| 商品 upsert | `UpsertFiscalProductByCode`（`vat_rate` 百分数串如 `"13.00"`） |

**再同步挡重（P0）：** 查税务库是否已有同 `source_system`+`source_sale_id` 的已签 FT（`store.HasSignedFTForSale`），有则 ingest ack `already_invoiced`。**不以**草稿行状态为准（开票后草稿已删）。

**整桌从草稿开 FT：** `billsync.DraftToSaleSnapshot`（唯一映射）→ `service.IssueFromBillDraft` → `IssueDocument`/`IssueFT` → `DeleteBillDraftsBySale`。`fiscal_products` 不删。

## 7. 签发事务写序（必须）

同事务 `BEGIN IMMEDIATE`：

1. 查 `idempotency_keys`（命中则返回原票，不占新号）  
2. Windows-1252 / 金额 / 系列 ACTIVE 校验  
3. 更新 `series.last_number` / `last_hash`  
4. 插入 `invoices` + `invoice_lines` + `invoice_customer_snapshots` + `invoice_payments`（NC 另写 `invoice_line_references`）  
5. 插入 `local_print_jobs`（ORIGINAL + 完整 payload）  
6. 写 `idempotency_keys.invoice_id`  
7. （可选）`sync_outbox`  
8. `COMMIT`  

之后 Print Worker 认领出纸；失败只改 `print_status` / job，**不删税务行**。

---

## 8. 与 `config.json` / Farvoo 的边界

| 数据 | 存放 |
|------|------|
| `api_base`、`agentjwt`、档口 `station_printers`、Realtime | `config.json`（打印配对） |
| 商家 NIF、系列、Hash 链、发票、税务打印队列 | **SQLite** |
| `han_bitmap_font_px`（业务票） | Farvoo 功能设置 → 云端 job payload |
| 发票小票字号（未来） | 建议写入冻结 `print_payload` 或 `taxpayer_settings`，不依赖云端 job |

---

## 9. 实现文件

| 文件 | 用途 |
|------|------|
| `docs/fiscal-sqlite-schema.zh.md` | 本文（设计权威） |
| `apps/fiscal-agent/internal/fiscal/store/SCHEMA.md` | 包内速查 |
| `apps/fiscal-agent/internal/fiscal/store/migrations/001_init.sql` | 首版 DDL |

---

## 10. 后续实现顺序

工程里程碑与**每步交付物**以 [`docs/fiscal-dev-plan.zh.md`](fiscal-dev-plan.zh.md) 为准（本文 §10 仅保留历史提示）。

已完成：迁移嵌入、seed（M0 开发用）、series 锁 + invoice insert、compliance Hash、service 签发（**M0**）；身份 / AT mock 系列 / 激活（**M1**）；主 Agent 嵌入 + TCP 税务打印（**M2**）。

下一刀：**M3** NC（冲销）。

**变更流程**：改表先改本文 + SQL，再写代码；已签发票列不改语义，只允许加可空列或新表。
