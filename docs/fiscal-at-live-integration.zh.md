# AT 测试 / 正式环境统一接入规划

> **状态：定稿（规划）**  
> **权威：是**（本仓接 AT **真实** test/prod SOAP 的分刀顺序与共享底座；**不**改现有 mock 行为）  
> **对应实现：** 尚未落地（目标：`at.SOAPClient` 系列 + `at.SOAPTransportClient` 运输）  
> **现状：** `FISCAL_AT_ENV=mock`（默认/CI）全绿；`test`/`prod` 两套客户端均为骨架，失败关闭  
> **相关：** [`fiscal-m1-identity-series.zh.md`](fiscal-m1-identity-series.zh.md)（系列）、[`fiscal-gr.zh.md`](fiscal-gr.zh.md) §7–8（运输）、[`fiscal-config-boundary.zh.md`](fiscal-config-boundary.zh.md)

---

## 1. 目标

一次规划、分刀落地，把 **两套 AT webservice** 从 mock 接到 **同一套** `test` → `prod` 开关与证书/加密底座：

| 能力 | 接口（逻辑名） | 本仓入口 | 成功产物 |
|------|----------------|----------|----------|
| 系列注册 / 查询 | `registarSerie` / `consultarSeries` | `at.Client` → `RegisterSeries` | `series.validation_code` |
| 运输单据通信 | `envioDocumentoTransporte` | `at.TransportClient` → `RegisterDocument` | `ATDocCodeID` → `invoices.at_transport_doc_code` |

**非目标（本规划）**

| 项 | 说明 |
|----|------|
| 改 mock 语义 | CI / 本地默认仍 mock；不得强制联网 |
| 合并两个 SOAP 成一个 Client 类型 | 分文件保留；只共享 TLS / 密码加密 / 配置 |
| 门店自动切 prod | 须显式配置 + 正式通信证；禁止「有测试证就当生产」 |
| GT 产品化 | 复用运输客户端；GT 另文 |

---

## 2. 现状（2026-10-15）

| 层 | 系列 `at/client.go` | 运输 `at/transport.go` |
|----|---------------------|------------------------|
| mock | 固定 8 位验证码 | `MOCK`+hash 伪码 |
| test/prod | `SOAPClient` 返回 not configured | `SOAPTransportClient` 返回 not wired |
| 材料 | 运维用 AT 子用户（库内 Seal） | 测试证 + `ChavePublicaAT.cer` 已落 `testdata/transport/` |
| 业务依赖 | 开 FT/FS/… **前**须 ACTIVE 系列 | 开 GR **当场**须通信成功 |

探测结论（运输）：TLS 可达 `:701`；错误 xmlns → schema fault；正确包后 **Auth 500**（加密/Token/证绑定未闭环）。系列真 SOAP 尚未做同等探测。

---

## 3. P0 定法

| 项 | 定法 |
|----|------|
| 环境开关 | **唯一** `FISCAL_AT_ENV` = `mock` \| `test` \| `prod`（`config.json` → `fiscal_at_env`）；两套客户端同读 |
| 失败策略 | test/prod 缺证、加密失败、HTTP/SOAP 业务错 → **整单/整次注册失败**；禁止回落 mock |
| CI | 永远 mock；`-tags=at_live` / `at_transport_live` 默认 skip |
| 共享底座 | mTLS 客户端证、密码用 AT 公钥加密、SOAP Envelope 组装、超时/审计日志 |
| 凭证分离 | **子用户**用户名/密码：继续 `at_credentials`（Seal）；**通信证**（pfx）与 **加密公钥**（cer）：路径/口令走 env 或 `config.json`，**不**进 SQLite 明文 |
| 端点 | 以 AT 门户现行为准；代码内常量 + 可选 URL override（仅运维） |
| 顺序 | **先共享底座 → 系列 live → 运输 live → prod 门禁**（见 §5） |

### 3.1 配置一览（落地时写齐；一列一名）

| 名 | 适用范围 | 说明 |
|----|----------|------|
| `FISCAL_AT_ENV` | 共用 | mock / test / prod |
| `FISCAL_AT_SERIES_URL` | 系列（可选） | 覆盖默认 registarSerie URL |
| `FISCAL_AT_TRANSPORT_URL` | 运输（可选） | 覆盖默认 documentosTransporte URL |
| `FISCAL_AT_TRANSPORT_CERT` | 运输（及若系列亦要 mTLS） | 客户端 pfx/pem 路径 |
| `FISCAL_AT_TRANSPORT_CERT_PASS` | 同上 | pfx 密码 |
| `FISCAL_AT_TRANSPORT_PUBKEY` | 共用加密 | `ChavePublicaAT.cer` 路径 |
| `FISCAL_AT_TRANSPORT_USER` / `_PASS` | 可选覆盖 | 默认仍用 `at_credentials` 解封后的子用户 |

系列若官方要求同一套客户端证：复用 `FISCAL_AT_TRANSPORT_CERT*` 命名或落地时改名为 `FISCAL_AT_CLIENT_CERT*`（**P0：实现前拍板一名，文档与 env 同步改**）。

**实现前拍板（命名）：** 统一用 `FISCAL_AT_CLIENT_CERT` / `FISCAL_AT_CLIENT_CERT_PASS` / `FISCAL_AT_PASSWORD_PUBKEY`；运输专用 URL 仍分 `…_SERIES_URL` / `…_TRANSPORT_URL`。旧 `FISCAL_AT_TRANSPORT_*` 在 GR 文 §8 作别名过渡一期后删除。

---

## 4. 共享底座（两刀共用，只做一次）

```text
at/crypto.go（或 at/password_encrypt.go）
  EncryptATPassword(plain, pubkeyCER) → ciphertext（手册算法）

at/tls.go
  LoadClientTLS(certPath, pass) → *tls.Config（mTLS）

at/soaputil.go（可选）
  Envelope / UsernameToken / 错误解析公共件
```

| 验收 | 说明 |
|------|------|
| 单测 | 对固定明文 + 仓内 cer 出稳定密文（或与 golden XML 比对） |
| 无网 | 底座单测不访问 AT |
| 系列与运输 | 均调用同一 `EncryptATPassword` + 同一 TLS loader |

---

## 5. 分刀顺序

### 刀 A0 — 共享底座

1. 密码加密（`ChavePublicaAT.cer`）+ mTLS loader  
2. 文档：端点表、FAQ/手册引用、失败码映射  
3. 验收：单测；`FISCAL_AT_ENV=mock` 行为不变  

### 刀 A1 — 系列 SOAP live（test）

1. 实现 `SOAPClient.RegisterSeries` / `ConsultSeries`  
2. 用 test 子用户 + 客户端证打 AT 测试港  
3. Admin 注册 FT/FS/…/GR 系列拿 **真** `validation_code`  
4. 回归：`FISCAL_AT_ENV=test` 手测清单；CI 仍 mock  
5. 标签：`-tags=at_series_live` 可选集成测（默认 skip）  

**关闭条件：** test 环境成功注册至少一种销售系列 + GR 系列；错误信息可诊断（Auth / schema / 业务码）。

### 刀 A2 — 运输 SOAP live（test）= 原 G7

1. 实现 `SOAPTransportClient.RegisterDocument`（`envioDocumentoTransporte`）  
2. 接 `IssueDeliveryNote`（已有编排：先通信再 `IssueGR`）  
3. 用仓内现行 pfx/cer；修 Auth 500  
4. 脱敏 fixtures 入 `testdata/transport/fixtures/`  
5. `-tags=at_transport_live`  

**关闭条件：** test 港返回真实 `ATDocCodeID`；mock 回归全绿。

### 刀 A3 — prod 门禁

1. prod URL + **厂商正式通信证**（非公开测试 pfx）  
2. 配置检查：`prod` 且证路径缺失 → 启动或首次调用即明确错误  
3. 运维手册：切 `fiscal_at_env=prod` 的检查表（纳税人 NIF、子用户权限、证有效期）  
4. **禁止**用测试证打 prod  

### 刀 A4（可选）— 观测与运维

- 审计：系列注册 / 运输通信成功失败写 `audit_log`（无密码、无完整 SOAP）  
- Admin：设置页只读展示当前 `at_env`（不展示密钥）  

---

## 6. 与开票路径的关系

```text
日常开 FT/FS/FR/…     → 本机签；不读 AT（系列已有 validation_code）
注册系列（Admin）     → A1 起可真打 AT
开 GR / 将来 GT       → A2 起可真打运输 AT；此前 mock
```

切换 `test`/`prod` **不改变** Hash/ATCUD 本机签口径；只改变「拿验证码 / 拿运输文档码」的来源。

---

## 7. 风险与依赖

| 风险 | 缓解 |
|------|------|
| 测试证过期 | `scripts/fetch-at-transport-testdata.sh` + 有效期检查列入 A0/A2 清单 |
| 系列与运输 Auth 规则不一致 | 共用加密函数；分别对照两份 WSDL/手册 |
| Auth 500 难复现 | A2 保留脱敏请求/响应样例；门户「测试连接」对照 |
| 误开 prod | A3 门禁 + 文档红字；默认 mock |

---

## 8. 建议排期口径（不绑具体日期）

| 阶段 | 内容 | 前置 |
|------|------|------|
| 现在 | mock 产品化（已完成，含 GR） | — |
| 下一刀 AT | **A0 → A1**（系列真测） | 测试子用户可用；证有效 |
| 随后 | **A2**（GR 真测 = G7） | A0；A1 建议先通以便 GR 系列也是真码 |
| 门店前 | **A3** | 正式证 + 合同/软件认证号 |

**推荐：A1 先于 A2。** 真 `validation_code` 的 GR 系列 + 真 `ATDocCodeID` 才接近认证/门店路径；仅运输 live 而系列仍 mock 码，测试港可能拒单。

---

## 9. 修订

| 日期 | 说明 |
|------|------|
| 2026-10-15 | 初稿：系列 + 运输统一 test/prod；A0–A3；配置命名拍板 |
