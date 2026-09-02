# 配置/身份五表：实例串讲（逐字段来源）

> **状态：定稿（示例）**  
> **权威表结构：** [`fiscal-sqlite-schema.zh.md`](fiscal-sqlite-schema.zh.md) §6.1–6.5 + `migrations/001_init.sql`  
> **写法：** 每个字段写明「值从哪来」；禁止含糊。

## 共用场景

| 项 | 本例取值 | 来源 |
|----|----------|------|
| 门店 | Farvoo 餐厅「Pirata Wok Lisboa」 | Farvoo `restaurants` |
| `store_id` | `a1b2c3d4-e5f6-7890-abcd-ef1234567890` | claim 返回的 `restaurant_id`，写入 `config.json` 后再写入各表 |
| 主机 | 店内唯一 Windows 收银主机 | 物理机 |
| admin | 王五 | 空库 bootstrap（登录页），`role=admin` |
| owner | 张三 | **admin** 在设置 §operators 创建，`role=owner` |
| cashier | 李四 | **admin** 或 **owner** 创建，`role=cashier` |

时间一律举 UTC 例：`2026-08-20T13:05:00Z`。

---

## 1. `taxpayer_settings`（管理员或店长在 Agent「商家资料」页保存）

**谁写：** **admin** 或 **owner** 在 Agent 本地表单提交（`owner` 无 AT/系列/激活权限，见 [`fiscal-m3-2-operators.zh.md`](fiscal-m3-2-operators.zh.md) §3.3.3）。  
**谁读：** 开票票头、QR 字段 A、SAF-T Header、算 `InvoiceDate` 用的时区。

| 列 | 本例值 | 明确来源 |
|----|--------|----------|
| id | `ts-001-uuid` | Agent 生成 UUID |
| store_id | `a1b2c3d4-…7890` | 本机 `config.json` 的 `restaurant_id`（来自 claim） |
| tax_registration_number | `517535009` | 手填；来自公司税务登记 NIF（与 Portal 一致） |
| legal_name | `ESTRELAS OGIVAIS LDA` | 手填；商业登记法定名 |
| business_name | `Pirata Wok` | 手填；门店招牌（可空则票面可用 legal_name） |
| address_detail | `Av. Exemplo 12` | 手填；注册地址 |
| city | `Lisboa` | 手填 |
| postal_code | `1000-001` | 手填 |
| country | `PT` | 表单默认；可改（P0 大陆店固定 PT） |
| timezone | `Europe/Lisbon` | 表单默认；可改；**唯一**用于 InvoiceDate/票面本地时 |
| phone | `218437250` | 手填；可空 |
| software_certificate_number | `0` | **产品默认**；Modelo 24 通过前固定 `0`；通过后由**运营下发/发版配置**改为真号，一般不手改 |
| product_id | `Farvoo/InvoiceEngine` | **产品常量**（安装包/代码内嵌） |
| product_version | `0.1.0` | **运行中 Agent 版本号**（与 `VERSION` 文件一致） |
| fs_amount_threshold | `100.00` | 表单默认；可改；会计确认后的门店配置 |
| tax_country_region | `PT` | 表单默认；P0 大陆 |
| created_at | `2026-08-20T13:00:00Z` | Agent 首次插入时 `time.Now().UTC()` |
| updated_at | `2026-08-20T13:00:00Z` | 每次保存刷新 |

---

## 2. `at_credentials`（**管理员**在 Agent「AT 凭证」页保存）

**谁写：** **admin** 粘贴 Portal 子用户账号密码（**owner** 不可见该分区）。  
**谁读：** 仅调用 AT Series SOAP 时（注册/查询系列）；日常开 FT **不读**。

| 列 | 本例值 | 明确来源 |
|----|--------|----------|
| id | `atc-001-uuid` | Agent 生成 UUID |
| store_id | `a1b2c3d4-…7890` | 同 `config.json.restaurant_id` |
| username | `517535009/37` | admin 手填；Portal「NIF/子用户编号」 |
| password_ciphertext | `<DPAPI blob>` | admin 输入明文密码 → Agent 调用 Windows `CryptProtectData` → 只存返回字节 |
| salt | `NULL` | **P0 定法写死 NULL**（不用 PBKDF2）；不是手填 |
| wrap_meta | `{"scheme":"dpapi","v":1}` | **Agent 代码常量写入**；不是店长填写 |
| last_ok_at | `2026-08-20T13:10:00Z` | 最近一次 `registarSerie`/`consultarSeries` **成功**时 Agent 写入 |
| last_error | `NULL` | 失败时 Agent 写入 AT/SOAP 错误摘要（脱敏）；成功则清空或保留上次策略由实现定——**P0：成功时置 NULL** |
| created_at | `2026-08-20T13:02:00Z` | 首次保存 |
| updated_at | `2026-08-20T13:02:00Z` | 每次改密码/用户名刷新 |

**调用 AT 成功后（本例）：** 明文密码仅在内存；库中仍是 ciphertext。验证码进 **`series.validation_code`**，不进本表。

---

## 3. `agent_installations` + `signing_keys`（两步激活：**管理员**点「激活开票」）

顺序固定：

1. 本机生成设备钥 → 写 `agent_installations`（先可 pending，或 provisioned 在收到 C 后一次写齐）  
2. 用 `agentjwt` 向运营申请  
3. 运营下发 C → 写 `signing_keys`，并填齐 installation

### 3.1 `agent_installations` 一行

| 列 | 本例值 | 明确来源 |
|----|--------|----------|
| installation_id | `inst-9f3a-…` | Agent 首次激活时 `uuid` 生成，之后不变直至换机 |
| store_id | `a1b2c3d4-…7890` | `config.json.restaurant_id` |
| taxpayer_nif | `517535009` | 读 **`taxpayer_settings.tax_registration_number`**（激活前必须已保存商家资料） |
| device_id | `dev-主机原有-uuid` | **`config.json.device_id`**（claim 时生成/沿用，与打印配对同一值） |
| device_public_key | `-----BEGIN PUBLIC KEY-----…` | 本机 TPM/CNG（或 SOFTWARE）生成设备密钥对后 **导出的公钥 PEM** |
| hardware_fingerprint | `NULL` 或机器指纹串 | P0 可 NULL；若填则来自 Agent 采集的主板/磁盘等哈希（实现定算法后写死） |
| key_protection_level | `TPM` | Agent 检测：有可用 TPM 2.0 则 `TPM`，否则 `SOFTWARE` |
| signing_key_version | `1` | 与下发的产品钥版本一致；来自运营响应 `signing_key_version` |
| provisioned_at | `2026-08-20T14:00:00Z` | Agent 成功写入 `wrapped_private_key` 并自检签名通过的时刻 |
| revoked_at | `NULL` | 仅运营吊销/换机流程写入；正常为 NULL |

设备**私钥**不在本表，在 TPM（或 DPAPI）内。

### 3.2 `signing_keys` 一行（版本 1）

| 列 | 本例值 | 明确来源 |
|----|--------|----------|
| id | `sk-001-uuid` | Agent 生成 UUID |
| key_version | `1` | 运营下发包中的版本号；MVP 固定从 1 起 |
| public_key_pem | `-----BEGIN PUBLIC KEY-----…` | 运营下发；与 Modelo 24 交给 AT 的产品公钥相同 |
| wrapped_private_key | `<密文 C>` | **运营方**用本机 `device_public_key` 封装产品私钥 A 后下发；Agent **原样落库** |
| status | `ACTIVE` | 写入时 Agent 置 ACTIVE；换钥时旧行改 RETIRED |
| created_at | `2026-08-20T14:00:00Z` | 本行插入时间 |
| retired_at | `NULL` | 换钥退役时写入 |
| submitted_to_at_at | `2026-03-01T00:00:00Z` | 运营下发元数据（该公钥提交 AT 的时间）；若响应未带则 **P0 允许 NULL** |

开票时：本机用设备私钥解 `wrapped_private_key` → 内存中产品 RSA → 写 `invoices.hash`。

---

## 4. `operators`（Agent 本地创建 + PIN）

**名册来源：** 开票 Web 界面 **设置 §5**；**不同步** Farvoo。  
**PIN：** 6 位数字；argon2id 写 `pin_hash`；不上云。  
**权威：** [`fiscal-m3-2-operators.zh.md`](fiscal-m3-2-operators.zh.md)

### 4.1 admin 王五（bootstrap · 登录页）

| 列 | 本例值 | 明确来源 |
|----|--------|----------|
| id | `op-wang-uuid` | Agent 生成 UUID（作 SourceID） |
| mesa_user_id | `local-op-wang-uuid` | 占位 `local-{id}`（非 Farvoo user） |
| store_id | `a1b2c3d4-…7890` | `config.json.restaurant_id` |
| role | `admin` | 空库 bootstrap（`POST /setup/bootstrap-owner`，路径名保留） |
| display_name | `王五` | 登录页手填 |
| active | `1` | bootstrap 默认启用 |
| pin_hash | `$argon2id$…` | bootstrap 设 6 位 PIN |
| can_issue_nc | `1` | `admin` 默认 1 |
| synced_at | NULL | P0 本地路径不写 |
| created_at | `2026-08-20T14:00:00Z` | bootstrap 插入 |
| updated_at | `2026-08-20T14:00:00Z` | 改 PIN 或权限时刷新 |

### 4.2 owner 张三（admin 在设置 §operators 创建）

| 列 | 本例值 | 明确来源 |
|----|--------|----------|
| id | `op-zhang-uuid` | Agent 生成 UUID（作 SourceID） |
| mesa_user_id | `local-op-zhang-uuid` | 占位 `local-{id}`（非 Farvoo user） |
| store_id | `a1b2c3d4-…7890` | `config.json.restaurant_id` |
| role | `owner` | **admin** 添加开票员时选择 `owner` |
| display_name | `张三` | 手填 |
| active | `1` | 创建时默认启用 |
| pin_hash | `$argon2id$…` | 创建时设 PIN |
| can_issue_nc | `1` | `owner` 默认 1 |
| synced_at | NULL | P0 本地路径不写 |
| created_at | `2026-08-20T14:05:00Z` | admin 创建 |
| updated_at | `2026-08-20T15:00:00Z` | 改 PIN 或权限时刷新 |

### 4.3 cashier 李四

| 列 | 本例值 | 明确来源 |
|----|--------|----------|
| id | `op-li-uuid` | Agent 生成 |
| mesa_user_id | `local-op-li-uuid` | 占位 `local-{id}` |
| store_id | `a1b2c3d4-…7890` | 同上 |
| role | `cashier` | 创建时选 `cashier` |
| display_name | `李四` | 手填 |
| active | `1` | 默认启用 |
| pin_hash | `$argon2id$…` | **admin** 或 **owner** 创建时设 PIN；或本人「修改我的 PIN」 |
| can_issue_nc | `0` | 默认 0；**admin** / **owner** 可在设置页打开 |
| synced_at | NULL | P0 本地路径不写 |
| created_at | `2026-08-20T14:05:00Z` | |
| updated_at | `2026-08-20T15:00:00Z` | |

李四开 FT 时：`invoices.source_id = op-li-uuid`（登录会话解析到该 `operators.id`）。

---

## 5. 五表在一条时间线上的写入顺序（本例）

```text
T0  claim 六位码
    → config.json: restaurant_id, device_id, agentjwt
    → 本例尚无写 SQLite 身份表

T0b 空库 bootstrap（仅 loopback 登录页）
    → INSERT operators（王五 admin + pin_hash）

T1  admin 登录后填商家资料
    → INSERT taxpayer_settings（上表全部列有来源）

T2  admin 填 AT 子用户
    → INSERT at_credentials（salt=NULL, wrap_meta=dpapi）

T3  admin 点激活开票
    → 生成本机设备钥
    → 申请运营 → 收到 C
    → INSERT agent_installations + signing_keys

T4  （可穿插）用 at_credentials 调 AT 注册系列
    → 更新 at_credentials.last_ok_at
    → 验证码写入 series（别表，本篇不展开）

T5  **admin** 在设置 §operators 创建开票员
    → INSERT operators（张三 owner、李四 cashier）
    → 各设 pin_hash

T6  日常开票
    → 读 taxpayer_settings + signing_keys + operators
    → 不读 at_credentials（除非又要管系列）
```

---

## 6. 字段来源类型汇总（禁止含糊）

| 来源类型 | 含义 | 本篇出现的列例 |
|----------|------|----------------|
| A. 手填（admin/owner 表单） | Agent 表单 | NIF、地址、AT username/密码明文（存前加密；AT 仅 admin） |
| B. claim/config.json | 打印配对已有 | store_id、device_id |
| C. 产品常量/版本 | 安装包 | product_id、software_certificate_number 初值、wrap_meta.scheme |
| D. 本机生成 | Agent/TPM | installation_id、device_public_key、password_ciphertext、operators.id、mesa_user_id 占位 |
| E. 运营下发 | Fiscal 激活 API | wrapped_private_key、public_key_pem、key_version |
| G. Agent 运行时写入 | 成功/失败回调 | last_ok_at、provisioned_at、created_at、updated_at |

任一列必须能归到 A–G 之一；不能写「系统自动」而不指明哪一步。
