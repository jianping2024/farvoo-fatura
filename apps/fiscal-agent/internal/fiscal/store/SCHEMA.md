# Fiscal Agent SQLite Schema（P0）速查

> **完整字段与规则**：[`docs/fiscal-sqlite-schema.zh.md`](../../../../../../docs/fiscal-sqlite-schema.zh.md)  
> **DDL**：[`migrations/001_init.sql`](migrations/001_init.sql)

## 原则

| 原则 | 定法 |
|------|------|
| 金额 | `TEXT` 小数串，禁止 `REAL` |
| 时间 | 库内 UTC；`InvoiceDate`/票面按 `taxpayer_settings.timezone`（默认 `Europe/Lisbon`） |
| 密钥 | **双钥**：产品 RSA（签发票）≠ 设备 TPM/DPAPI（解 wrapped）；见设计文档 §2.2 |
| 签后 | invoices / lines / 客户快照 / Hash / ATCUD / QR 不可业务 UPDATE |
| 打印 | 税务权威队列 = `local_print_jobs`；云端 `print_jobs` 仅业务热敏 |
| 商品 | `fiscal_products` 薄投影，无库存 |

## 表分组

```text
taxpayer_settings, at_credentials, signing_keys, agent_installations, operators
customers, fiscal_product_categories, fiscal_products
series, invoices, invoice_lines, invoice_customer_snapshots,
  invoice_payments, invoice_line_references
idempotency_keys, local_print_jobs, print_attempts, audit_log
saft_exports, sync_outbox
```

## 签发同事务

idempotency → series 占号 → invoice(+lines/snapshot/payments) → ORIGINAL local_print_job → COMMIT  
打印失败不回滚税务行。
