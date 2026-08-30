# M1：身份、AT 系列、激活开票

> **状态：定稿**  
> **权威：是**（M1 行为；库列仍以 `fiscal-sqlite-schema.zh.md` + DDL 为准）  
> **对应实现：** `internal/fiscal/at`、`protect`、`service` setup、`api` `/local/v1/setup/*`  
> **计划：** [`fiscal-dev-plan.zh.md`](fiscal-dev-plan.zh.md) M1

## 1. 目标

店长经 Local Admin / API 完成：纳税人 → AT 子用户 → 注册/绑定系列（`validation_code`）→ 激活开票（设备钥 + 产品钥封装）→ 再开 FT。  
**禁止**依赖 `SeedDemo` 灌验证码与 `DEV_PLAIN` 私钥（除非显式开发开关）。

## 2. 环境

| 项 | 定法 |
|----|------|
| `at_env` | `mock`（默认，本地/CI）\| `test` \| `prod` |
| mock | 不访问 AT；`registarSerie` 返回固定 8 位验证码（可配置） |
| test/prod | 真实 SOAP（M1 实现客户端骨架；无证书材料时测试仍走 mock） |

## 3. 表单 → 列（一列一行）

### 3.1 纳税人 → `taxpayer_settings`

| 表单 | 列 |
|------|-----|
| store_id | store_id（可来自 config） |
| NIF | tax_registration_number |
| 法定名称 | legal_name |
| 商号 | business_name |
| 地址 | address_detail |
| 城市 | city |
| 邮编 | postal_code |
| 国家 | country（默认 PT） |
| 时区 | timezone（默认 Europe/Lisbon） |
| 软件认证号 | software_certificate_number |

### 3.2 AT 凭证 → `at_credentials`

| 表单 | 列 |
|------|-----|
| 用户名（NIF/nn） | username |
| 密码明文（仅提交瞬间） | → password_ciphertext |
| — | salt = NULL |
| — | wrap_meta = `{"scheme":"dpapi"|"file_aes","v":1}` |

**P0 保护方案：**

| 平台 | scheme | 说明 |
|------|--------|------|
| Windows | `dpapi` | CryptProtectData / Unprotect |
| Darwin/Linux（开发） | `file_aes` | AES-GCM；密钥文件 `wrap.key`（0600），**非生产门店方案** |

唯一入口：`protect.Seal` / `protect.Open`。

### 3.3 系列 → `series`

| 输入 | 列 |
|------|-----|
| document_type | document_type（M1：FT） |
| series_code | series_code |
| fiscal_year | fiscal_year |
| AT 返回 codValidacaoSerie | validation_code |
| — | status=ACTIVE；last_number=0；last_hash='' |

### 3.4 激活 → `agent_installations` + `signing_keys`

| 步骤 | 写入 |
|------|------|
| 本机生成设备 RSA | device_public_key；设备私钥 Seal 存 `device_key_store` 文件（不进 SQLite 明文） |
| 运营/本地供给产品 PEM | wrapped_private_key（用设备公钥封装）；public_key_pem；key_version；status=ACTIVE |
| installation | installation_id、key_protection_level=SOFTWARE（M1）、signing_key_version |

**M1 本地供给：** `POST /local/v1/setup/activate` 可带 `product_private_key_pem`（仅 `FISCAL_ALLOW_LOCAL_PROVISION=1`）。生产改为云端下发密文，同一 `signer.UnwrappingSigner` 解封。

## 4. API（唯一 setup 面）

| 方法 | 路径 | 作用 |
|------|------|------|
| PUT | `/local/v1/setup/taxpayer` | upsert 纳税人 |
| PUT | `/local/v1/setup/at-credentials` | 存 AT 用户/密码（Seal） |
| POST | `/local/v1/setup/series/register` | 调 AT（或 mock）并写 series |
| POST | `/local/v1/setup/activate` | 设备钥 + 产品钥封装 |
| PUT | `/local/v1/setup/operator` | upsert 开票员（M1 最小） |
| GET | `/local/v1/setup/status` | 纳税人/凭证/系列/激活是否就绪 |

签发仍：`POST /local/v1/fiscal-documents` → `IssueFT` only。

## 5. 错误码（稳定字符串）

| error | 何时 |
|-------|------|
| `taxpayer_missing` | 无纳税人 |
| `at_credentials_missing` | 无 AT 凭证 |
| `series_inactive` / `series_missing` | 无 ACTIVE 系列或无验证码 |
| `signer_not_ready` | 未激活 / 无法解封产品钥 |
| `at_soap_failed` | AT/mock 业务失败 |
| `idempotency_conflict` | 同 request_id 不同 payload |

## 6. 开发开关

| 变量 | 默认 | 含义 |
|------|------|------|
| `FISCAL_AT_ENV` | `mock` | AT 环境 |
| `FISCAL_SEED` | 未设置=关 | `1` 时允许 SeedDemo（仅 M0 兼容） |
| `FISCAL_ALLOW_DEV_KEY` | 关 | 允许 `DEV_PLAIN:` wrapped（禁止默认开票路径） |
| `FISCAL_ALLOW_LOCAL_PROVISION` | **默认关**（未设 env 时由 `config.json` / 缺省 false） | 允许 activate 明文产品 PEM；生产走 Ops 云端激活 |
| `FISCAL_MOCK_VALIDATION_CODE` | `CSDF7T5H` | mock 系列验证码 |

**装机不用再设系统环境变量。** 托盘 Agent 启动时 `applyFiscalRuntimeFromConfig`：未设 env 则默认 `FISCAL_ALLOW_LOCAL_PROVISION=1`、`FISCAL_AT_ENV=mock`；也可在 `config.json` 写：

```json
{
  "fiscal_allow_local_provision": true,
  "fiscal_at_env": "mock"
}
```

## 7. Windows 手测（本里程碑不阻塞 Mac CI）

- [ ] DPAPI Seal/Open 往返  
- [ ] 换 Windows 用户后旧密文不可解（预期）  

---

## 修订

| 日期 | 变更 |
|------|------|
| 2026-08-20 | M1 定稿：API、列映射、protect scheme、错误码 |
