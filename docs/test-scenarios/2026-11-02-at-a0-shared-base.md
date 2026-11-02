# 本次改动业务场景测试报告：AT live A0 共享底座

日期：2026-11-02  
分支：`feat/at-a0-shared-base`  
版本：`0.5.59`  
权威设计：[`docs/fiscal-at-live-integration.zh.md`](../fiscal-at-live-integration.zh.md) §5

## 1. 本次修改

- **内容：** 落地 AT test/prod 共用底座（无网）：密码加密、mTLS 加载、SOAP UsernameToken/Envelope、环境变量材料读取。不调用 AT；不改 mock 开票/运输行为。
- **改动点：** `internal/fiscal/at/crypto.go`、`tls.go`、`soaputil.go`、`live_config.go`、`a0_test.go`；VERSION/RELEASE_NOTES；计划文进度头。
- **唯一写法（自检，diff 各一份）：**
  - AT SOAP 密码加密：`EncryptATAuth`（+ `LoadPasswordPublicKey`）
  - mTLS：`LoadClientTLS`
  - UsernameToken XML：`BuildUsernameTokenXML`
  - Envelope：`WrapSOAPEnvelope`
  - 材料 env：`LiveMaterialsFromEnv`
- **不动：** `SOAPClient` / `SOAPTransportClient` 真 HTTP（A1/A2）；Admin UI；开票 Hash/ATCUD；mock 验证码语义。

## 2. 核心业务规则

1. 【本次新增】`FISCAL_AT_ENV=mock` 时仍走 `MockClient` / `MockTransportClient`，零网络。
2. 【本次新增】Password/Created 用 AES-128-ECB；Nonce 为 RSA-PKCS1 加密的 16 字节 AES 钥（ChavePublicaAT）。
3. 【本次新增】坏 pfx 口令 / 缺文件 → 明确错误，不 panic、不回落 mock。
4. 【本次新增】配置优先 `FISCAL_AT_CLIENT_*` / `FISCAL_AT_PASSWORD_PUBKEY`，兼容旧 `FISCAL_AT_TRANSPORT_*` 别名。
5. 【已实现】日常开 FT 不读本底座；仅未来 A1/A2 live 路径调用。

## 3. 详细测试场景

### 场景 A0-01：固定密钥加密后可回解密码与 Created
- **前置：** 仓内 `ChavePublicaAT.cer`；测试密码 `testes1234`。
- **操作：** `EncryptATAuth` 注入 16 字节 AES key + 固定 UTC；Base64 解码后 AES 解密。
- **预期：** 明文密码与 `2026-11-02T10:00:00.000Z` 一致；同 key 再加密 Password/Created 密文稳定；Nonce 长度 = RSA 模长。
- **数据验证：** 无写库。
- 结果：【已验证】pass（`TestEncryptATAuth_RoundTripAES`）

### 场景 A0-02：缺密码 / 缺公钥 / 短 AES key 拒绝
- **操作：** 空密码、nil pub、AES key≠16。
- **预期：** 返回错误。
- 结果：【已验证】pass

### 场景 A0-03：加载现行 TesteWebservices.pfx 得 TLS 配置
- **操作：** `LoadClientTLS(…/TesteWebservices.pfx, TESTEwebservice)`。
- **预期：** `Certificates` 非空；口令错误 / 文件缺失明确失败。
- 结果：【已验证】pass

### 场景 A0-04：UsernameToken + Envelope 组装
- **操作：** `BuildUsernameTokenXML` → `BuildSecurityHeaderXML` → `WrapSOAPEnvelope`。
- **预期：** 含用户名、wsse 命名空间、Header/Body。
- 结果：【已验证】pass

### 场景 A0-05：env 别名与新名优先
- **操作：** 仅设旧 `FISCAL_AT_TRANSPORT_*`；再设新 `FISCAL_AT_CLIENT_*` 覆盖。
- **预期：** 别名可读；新名优先；`MissingRequired` 在齐套时为空。
- 结果：【已验证】pass

### 场景 A0-06：默认 mock 注册系列与运输不变
- **前置：** 未设 `FISCAL_AT_ENV`。
- **操作：** `NewFromEnv` / `NewTransportFromEnv` → Register。
- **预期：** Mock 类型；8 位验证码；`ATDocCodeID` 以 `MOCK` 开头。
- 结果：【已验证】pass

### 场景 A0-07：fiscal 包回归门禁（含 Windows 交叉编译）
- **操作：** `./scripts/check-fiscal-agent.sh`
- **预期：** go test / vet / Windows build 通过。
- 结果：【已验证】pass

### 场景 A0-08：mock 本地开 FT 主路径仍绿
- **操作：** `node scripts/fiscal-local-regression.mjs`
- **预期：** issue-ft / print / idempotent 全 pass（不依赖 A0 live）。
- 结果：【已验证】pass

### 场景 A0-09：Admin 配测试子用户后真打 registarSerie
- **说明：** 属 A1；本刀未接线。
- 结果：【未验证】（下一刀）

### 场景 A0-10：真打运输 SOAP 得 ATDocCodeID
- **说明：** 属 A2；本刀未接线。
- 结果：【未验证】（下一刀）

## 4. mesa-local-product-test

| 项 | 结果 |
|----|------|
| `mesa-local-product-test`（Mesa 餐厅仓技能） | **N/A · 【未验证】** — 本改动为 fiscal-agent `at` 包无网底座；不经 Mesa `:3000` UI / 开票路径 |
| 替代验收 | `go test ./internal/fiscal/at` + `./scripts/check-fiscal-agent.sh` + `fiscal-local-regression.mjs`（【已验证】） |
| `node scripts/mesa-local-uat.mjs stack-health`（restaurant-ordering） | web/ops 未起（fetch failed）；supabase 可达；`ready=false` — **与本改无关**，标【未验证】产品 UI |

## 5. 结论

| 检查 | 结果 |
|------|------|
| 唯一写法各一份 | pass |
| A0 无网单测 | pass |
| check-fiscal-agent | pass |
| fiscal-local-regression（mock FT） | pass |
| mesa-local-product-test | N/A · 【未验证】 |
| A1/A2 live SOAP | 【未验证】 |
| 可合 main | 是 |
