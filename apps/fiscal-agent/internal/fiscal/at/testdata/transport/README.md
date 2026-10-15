# AT 运输单据（Documentos de Transporte）测试材料

> **用途：** GR/GT 签发前 AT webservice 联调。  
> **权威步骤：** [`docs/fiscal-gr.zh.md`](../../../../../../docs/fiscal-gr.zh.md) §8。

## 当前可用（核查日 2026-10-15）

| 文件 | 来源 | 有效期 |
|------|------|--------|
| `certificados/TesteWebservices.pfx` | 社区镜像 [downloads.elsif.pt/AT_Certificados](http://downloads.elsif.pt/AT_Certificados/) → `TesteWebservices_20270106.pfx` | **2026-07-10 → 2027-01-06** |
| `certificados/ChavePublicaAT.cer` | 同镜像 → `ChaveCifraPublicaAT20250415.cer` | **2025-04-15 → 2027-04-15** |
| 密码 | `TESTEwebservice`（与历史公开测试证相同；镜像 README 称导出时保持原密码） | |

副本亦在 `certificados/current/`。

**说明：** AT FAQ 4994 写明测试证公开可分享。官方 `faturas.../certificados.zip` 仍是旧包（已挪到 `stale-official-zip/`，2021/2023 过期）。门户部分 Documents 链接 404。本仓以镜像中**未过期**文件做联调；上生产仍须厂商正式证。

## 其它文件

| 路径 | 说明 |
|------|------|
| `certificados.zip` | 官方旧包原文（勿用于联调） |
| `certificados/stale-official-zip/` | 从官方 zip 解出的过期证 |
| `documentosTransporte.wsdl` | 社区镜像（官方 info WSDL 当日 404） |

## 刷新

```bash
./scripts/fetch-at-transport-testdata.sh
openssl pkcs12 -in apps/fiscal-agent/internal/fiscal/at/testdata/transport/certificados/TesteWebservices.pfx \
  -passin pass:TESTEwebservice -nokeys -clcerts -legacy | openssl x509 -noout -dates
```

`notAfter` 过期后，到上述镜像取更新的 `TesteWebservices_YYYYMMDD.pfx`，或从 e-Fatura Produtores / AT 邮件换新。

**禁止**提交生产门店私钥或正式 CSR 结果。
