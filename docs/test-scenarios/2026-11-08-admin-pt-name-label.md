# 本次改动业务场景测试报告 — Admin「葡语名称」文案

日期：2026-11-08　分支：`fix/admin-pt-name-label`（未 commit / 未 merge）

## 1. 本次修改

**终点形状**：商品资料与 SAF-T 文本错误提示里，对人展示的字段名统一为「葡语名称」（en: Portuguese name，pt: Nome em português）；不再出现「税票名称」。

**权威依据**：现网 `product.fiscal_name_hint` 已写「葡语名称」；字段语义即 `saft_name` / SAF-T `ProductDescription`。

**唯一写法**

| 规则 | 唯一位置 |
|------|----------|
| 三语文案 | `bootstrap/ui/admin-i18n.js` 的 `product.field.fiscal_name`、`saft.field.saft_name`、`saft.saft_name_hint` |
| HTML 回退文案 | `admin/index.html` 商品表单 label |
| 回归门禁 | `ui_saft_text_test.go` 禁止再出现「税票名称」 |

未改 API 字段名 `saft_name`、未改校验逻辑。

## 2. 核心业务规则

- 【本次】Admin 中文界面称「葡语名称」；错误 toast 经同一 i18n 键自动带新称呼。
- 【已实现，不变】显示名仍可中文；`saft_name` 仍须 Windows-1252。

## 3. 详细测试场景

| ID | 场景 | 结果 |
|----|------|------|
| T1 | `go test ./internal/fiscal/bootstrap/ -run SAFTText` | 【已验证】pass |
| T2 | 源码无残留「税票名称」于 Admin embed | 【已验证】pass（测试断言） |
| T3 | Admin 真界面目视（商品编辑 / 中文开票拒收 toast） | 【未验证】待用户硬刷后看 |

## 4. mesa-local-product-test

N/A · 仅 Admin 文案。

## 5. 实际测试结果

| 项 | 结果 |
|----|------|
| bootstrap 相关 go test | pass |

## 6. 风险与待确认

1. 历史测试报告文档仍写「税票名称」（当时实录），未改写。
2. 待 merge / 重启 Agent 后硬刷，旧 embed 才会换成新文案。
