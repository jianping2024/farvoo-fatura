# 本次改动业务场景测试报告 — 发票挂发货单核对购方、运输单作废先报税务局

日期：2026-11-08　分支：`fix/ft-gr-buyer-and-movement-annul`（工作区 `/Users/chenjianping/Documents/farvoo-fatura-wt-cert-pre`，基于 main `a770741`；未 commit / 未 merge）

## 1. 本次修改

**终点形状：**

1. 一张 FT 只能挂同一个购方的 GR。有真实 NIF 时两边税号相同；都没有真实 NIF 时名称相同。对不上不写引用。
2. GR/GT 作废：开运时刻已过则拒绝且不通信；未过则用原单号再报 `MovementStatus=A`，税务局接受后本地才改为作废。原 `ATDocCodeID` 不改。店名和地址用当前门店资料。
3. 认证清单范围改为当前程序能开的全部单据。出具命令仍是 `fiscal-d62-cert-regression.mjs`，并调用已有票种回归和 SAF-T XSD。证书号保持 `0`。

**权威依据：** `docs/fiscal-gr.zh.md`、`docs/fiscal-gt.zh.md`、`docs/fiscal-doc-types-usage.zh.md` §8.5、运输 WSDL `MovementStatus` 含 `A`、SAF-T 作废原因 ≤50。

**唯一写法：**

| 规则 | 唯一位置 |
|------|----------|
| 购方是否相同 | `store.sameDeliveryBuyer`，只在 `linkDeliveryNotesInTx` 调用 |
| 空税号时是否保留名称 | `billsync.ApplyCustomerOverride` |
| 作废编排 | `service.annulMovement`（GR/GT 入口都进这里） |
| 报给税务局 | `TransportClient.RegisterDocument`（作废状态为 A，不占新号） |
| 本地改为作废 | `store.AnnulGR` / `store.AnnulGT` |

## 2. 场景结果

| ID | 场景 | 结果 |
|----|------|------|
| T1 | 税号不同的 FT 挂 GR | 【已验证】pass：`TestIssueGR_Annul_LinkFT` 返回 `ErrDeliveryNotAllowed`，GR 仍为 N |
| T2 | 无税号、名称相同可挂；名称不同拒绝 | 【已验证】pass：同上 |
| T3 | 作废先发状态 A，原文档码不变；同一请求再点不重复通信 | 【已验证】pass：`TestAnnulDeliveryNote_TellsATThenLocal`（mock） |
| T4 | 开运时刻已过、税务局拒绝：本地仍是 N | 【已验证】pass：`TestAnnulDeliveryNote_PastOrRejectedDoesNotChangeLocal` |
| T5 | GR / GT / RG / PF 回归 | 【已验证】pass |
| T6 | SAF-T 全票种样本过 XSD | 【已验证】pass：`SAF-T XSD OK` |
| T7 | 税务局测试口真实作废 | 【未验证】工作区没有测试库；未对已有测试单发状态 A |
| T8 | 扫码枪、真热敏、产品签名钥匙样本 | 【未验证】自动脚本仍用开发私钥 |

## 3. 风险

作废重发的店名和地址来自当前门店资料。测试口还没打过，若因资料对不上被拒绝，再补开单时的快照。同名无税号收货人仍可能挂错。
