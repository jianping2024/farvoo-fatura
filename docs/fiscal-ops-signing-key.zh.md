# Ops 产品签名钥：激活下发与吊销

> **状态：定稿**  
> **权威：是**（本能力产品与云/Agent 边界；库列以本文云表 + `fiscal-sqlite-schema.zh.md` 本机表为准）  
> **对应实现：** restaurant-ordering `fiscal_signing_installations` + `@mesa/shared` fiscal-signing；Agent `ActivateFromCloud` → `store.SaveActivation`  
> **计划：** 替代生产路径手动粘贴 PEM

## 1. 目标

Ops 按店点「激活开票」后，已配对 Agent **自动领取** 针对本机设备公钥封装的产品签名私钥密文 C。  
Ops「吊销」后该 installation 终态，不得复活；再开票须新注册 + 再激活。

## 2. 非目标

- 发票功能开/关激活码  
- Ops 管 NIF / AT / 系列 / bill-sync  
- 云端存正式税票正文  

## 3. 密钥

| 代号 | 含义 | 存放 |
|------|------|------|
| A | 产品签名私钥（全平台同 `signing_key_version` 共用） | 仅服务端 Secret：`FISCAL_PRODUCT_PRIVATE_KEY_PEM`（P0）；以后可迁 Secrets Manager |
| A' | 产品公钥 | 下发进 C 包元数据 / 本机 `signing_keys.public_key_pem` |
| B / B' | 设备钥 | 本机；B' 上报云 |
| C | Wrap(A, B') | 云表 `wrapped_private_key` + 本机 `signing_keys.wrapped_private_key` |

**唯一 wrap 实现：** `@mesa/shared` `wrapFiscalProductPem`（与 Agent `signer.WrapProductPEM` 同算法）。

## 4. 云表 `fiscal_signing_installations`

| 列 | 类型 | 必填 | 说明 |
|----|------|------|------|
| id | UUID PK | 是 | 云端 installation id（下发时写入本机 `agent_installations.installation_id`） |
| restaurant_id | UUID | 是 | 门店 |
| device_id | UUID | 是 | = `print_agent_devices.id` / claim `device_id` |
| device_public_key | TEXT | 是 | B' PEM |
| signing_key_version | INTEGER | 是 | 默认 1 |
| product_public_key_pem | TEXT | 否 | 激活后填 A' |
| wrapped_private_key | TEXT | 否 | 激活后填 C；吊销后可清空 |
| status | TEXT | 是 | `registered` \| `active` \| `revoked` |
| activated_at | TIMESTAMPTZ | 否 | |
| revoked_at | TIMESTAMPTZ | 否 | |
| activated_by | UUID | 否 | platform admin user |
| revoked_by | UUID | 否 | |
| created_at | TIMESTAMPTZ | 是 | |
| updated_at | TIMESTAMPTZ | 是 | |

**约束：** 每店同时最多一行 `status=active`；`revoked` 不可改回 `active`。

## 5. API（唯一入口）

| 谁 | 方法 | 路径 | 作用 |
|----|------|------|------|
| Agent | POST | `/api/print-agent/fiscal-signing/register` | 上报 B'；写/更新 `registered` |
| Agent | GET | `/api/print-agent/fiscal-signing/provision` | 拉取本机 `active` 的 C |
| Ops | GET | `/api/ops/restaurants/[id]/fiscal-signing` | 状态 |
| Ops | POST | `/api/ops/restaurants/[id]/fiscal-signing/activate` | wrap + `active`（先吊销旧 active） |
| Ops | POST | `/api/ops/restaurants/[id]/fiscal-signing/revoke` | `revoked` 终态 |

Agent Bearer：`agentjwt`。Ops：`requirePlatformAdminRole('admin')`。

## 6. 本机唯一写路径

云领钥与本地 PEM 激活 **均只** 调用 `store.SaveActivation`。

| 入口 | 条件 |
|------|------|
| `service.ActivateFiscal` | `FISCAL_ALLOW_LOCAL_PROVISION=1` + PEM（UAT/回归） |
| `service.ActivateFromCloud` | 配对后 register + provision（**生产主路径**） |

生产默认 **关闭** 本地粘贴 PEM（`applyFiscalRuntimeFromConfig` 默认 `0`）。

## 7. 体感

店：配对 → 税务资料 → 等待 Ops 激活 → Agent 自动/点同步领 C → 开票。  
Ops：激活开票 / 吊销。
