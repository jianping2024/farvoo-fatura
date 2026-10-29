# AT 软件厂商 adesão 证书（测试）

> **用途：** Portal「Pedidos de adesão ao serviço」签发的厂商公开证，后续系列/运输 live 对接用。  
> **不是** `transport/certificados/TesteWebservices.pfx`（公开运输测试客户端证）。

## 当前文件

| 文件 | 主体 | 有效期 | 来源 |
|------|------|--------|------|
| `519655290.cer` | CN=`519655290` / O=FAINA VAIDOSA LDA | **2026-10-29 → 2028-10-28** | e-Fatura 门户下载（Estado=Ativo） |

```bash
openssl x509 -in apps/fiscal-agent/internal/fiscal/at/testdata/producer/519655290.cer \
  -noout -subject -issuer -dates
```

## 注意

- 本文件为 **PEM 公钥证书**（无私钥）。mTLS 若还要私钥，须另行保管，**禁止**把生产私钥提交进仓。
- 与 `transport/` 下 AT 公开测试材料分开存放，避免混用。
