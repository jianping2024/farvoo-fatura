# AT 运输 SOAP 脱敏 fixtures（A2）

> **状态：定稿**（A2 关闭证据）  
> **来源：** Admin AT test 联调转储（`FISCAL_AT_SOAP_DUMP`），2026-11-03  
> **脱敏：** 无 UsernameToken / 密码 / 私钥；响应去掉 Oracle `WorkContext` 头

| 前缀 | 说明 | 原始 dump |
|------|------|-----------|
| `gr-success` | Admin GR GR2026T02/1 → ATDocCodeID | `20261103T171719Z` |
| `gt-success` | Admin GT GT2026T01/1 (transfer) → ATDocCodeID | `20261103T172356Z` |
| `gt-reject-number-taken` | ReturnCode -3 document number already registered | `20261103T170849Z` |
| `gt-reject-invalid-nif` | ReturnCode -1 recipient NIF not valid at AT | `20261103T180420Z` |

每个前缀含：`*-request-body.xml`、`*-response.xml`、`*-meta.txt`。

用途：对照 WSDL/手册、回归排障、修 Auth/schema 时与成功样例逐字段比对。**不**作为 CI 联网夹具。
