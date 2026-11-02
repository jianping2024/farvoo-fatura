# 本次改动业务场景测试报告：A2 从 Admin 签发 GR（AT test）

日期：2026-11-02  
分支：`fix/gr-gt-movement-start-not-past`  
仓库：`farvoo-fatura`  
本机：`http://127.0.0.1:17881`（`FISCAL_AT_ENV=test`，`fiscal-test.db`，子用户 `519655290/1`）

## 1. 本次修改

- **内容：** AT 不可用（`at_transport_unavailable`）时，Admin 显示三语提示，不再弹英文原始报错。
- **文件：** `bootstrap/ui/admin-i18n.js`、本报告。
- **唯一写法：** 文案 `errors.at_transport_unavailable`，由 `FiscalUI.formatError` 按 `error` 码解析。

## 2. 核心业务规则

1. 【已实现】先通信 AT，后写库；AT 失败 → 零单据行、不占号。
2. 【本次新增】AT 不可用 → 提示「暂时连不上税务局（AT），单据未签发，请稍后重试」。

## 3. 详细测试场景

### 场景 1：Admin 开最小 GR（散客，收货地址里斯本，装/卸省略）

- **操作：** 开票 → 加商品 → 票种「发货单」→ 收货地址 `Rua Augusta 100 / Lisboa / 1100-053` → 取消「收货方是 IVA 纳税人」→ 签发。
- **预期：** AT 返回 `ATDocCodeID`，本地落 `GR GR2026T02/1`。
- **结果：未通过（AT 侧）**：三次（16:49、16:51、16:53 UTC）均 HTTP 500，fault `33`「mismatched tag, expected hr at offset 241」，`<detail>` 命名空间 `autenticacao…/ValidateCredencialService`——AT 认证后端在读单据前就失败（解析到 HTML 错误页）。请求体（`FISCAL_AT_SOAP_DUMP`）字段与成功探针同构。同店凭证 16:08 的系列调用亦失败（`Erro - Resposta do Servidor`）。

### 场景 2：AT 失败时失败关闭

- **预期：** 无 GR 行；`GR2026T02.last_number` 不变。
- **结果：【已验证】pass**（`invoices` 仅 FT 1 张；GR2026T02 仍 0）。

### 场景 3：AT 不可用提示

- **结果：【已验证】pass**：`formatError({error:'at_transport_unavailable'})` → 中文提示。

## 4. 实际测试结果

| 项 | 结果 |
|---|---|
| Admin GR → AT test 拿 `ATDocCodeID` | **阻塞**：AT 认证服务 fault 33 |
| 失败关闭（零行、不占号） | pass |
| 中文不可用提示 | pass |
| `go test ./internal/fiscal/bootstrap/...` | pass |

## 5. 风险与待确认

- 只读 `TestLiveConsultSeries`（16:56 UTC，同子用户）也失败：HTTP 500 `env:Server`「Erro - Resposta do Servidor」。同一凭证当天上午查询返回 2002，系列与运输两个服务同时 500，判断为 AT test 后端故障，非本地请求问题；待恢复后重试场景 1。
- 17:01 UTC 再跑只读查询（手输密码）：0.3s 返回 fault **1199**「Erro de Autenticação/Autorização - Pedido do Cliente」（本机时钟已核对无偏差）。17:02 Admin 再签 GR（库存凭证）仍为 fault 33。结论**未定**：1199 可能是手输密码有误，也可能该子用户密码/权限有变或被锁；需在 AT 门户核对 `519655290/1` 后再判。时间线：13:03 最后成功 → 16:08 系列 500 → 16:49–17:02 运输 33。
- 门户核对（acesso.gov.pt → Gestão de Utilizadores）：`RestaurantPos 519655290/1` **Ativo**，权限 **WFA, WDT, WSE**，Último Acesso **2026-11-02 17:02:43**（与 17:02 Admin 签 GR 同刻）。→ 库存凭证有效、未锁、权限齐；17:01 的 1199 归因手输密码有误。运输 33 发生在认证通过之后的 AT 后端（推断：AT 运输侧故障，或对我方报文的异常处理），下一步用上午成功过的探针报文（`TestLiveTransportGR`）同时段对照。
- **对照结果（17:06:28 UTC）：** `scripts/at-live-transport-probe.sh 3`（上午成功的报文，`GR GR2026T01/3`）同样 HTTP 500 fault 33，12.3s。→ **结论：AT test 运输服务侧故障，与 Admin 报文无关**；A2 代码侧无待改项，等 AT 恢复后重试场景 1。`GR2026T01/3` 未登记成功，下次探针仍用 3。
- 观察到的 GR 表单体验问题（未改）：显示付款方式 / 实收金额；按钮仍写「签发发票并打印」；「收货方是 IVA 纳税人」默认勾选但 NIF 为空，散客必先手动取消。
