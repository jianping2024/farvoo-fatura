# M4 餐馆联调 UAT — 白云饭店（cloud）

> **状态：已通过**（联调执行记录；工程扫尾见 0.4.21）  
> **权威：否** — 手测清单；契约见 [`fiscal-local-api.zh.md`](fiscal-local-api.zh.md) / [`fiscal-bill-draft-workbench.zh.md`](fiscal-bill-draft-workbench.zh.md)  
> **环境：** Farvoo **cloud** · 门店 **白云饭店**  
> **Agent：** 托盘主进程（非 `fiscal-local` seed）；Admin 0.4.x 餐馆模式（联调时约 0.4.20；扫尾 0.4.21）

## 1. 范围

| 用例 | 说明 |
|------|------|
| 整桌 | 同步载荷 `scope_type=whole_table` → 收银账单 → 签发一张 FT |
| 按人 | 同步载荷 `scope_type=split` + `splits[]` → 选人 → 各签一张 FT；互斥规则见 workbench §5.2 |

## 2. 前置（P0）

| # | 检查项 | 通过 |
|---|--------|------|
| P1 | Cloud 该店 fiscal / bill_sync 已开 | ✓ |
| P2 | Agent 已配对 store，Realtime 收到 `bill_sync_jobs` | ✓ |
| P3 | 本机 M1：纳税人 / 系列 / 激活（非 seed demo） | ✓ |
| P4 | 打印机档口已绑（17892/configure） | ✓ |
| P5 | Admin 登录 **餐馆模式** → 可见「收银账单」 | ✓ |

## 3. 整桌用例

```text
Farvoo 结账 → 同步账单
  → Agent PullAndIngest → Admin 收银账单出现
  → 选择开票 → 整桌 →（可选 NIF）→ 签发 FT → 热敏出纸
  → 收银账单消失；再点同步 → cloud ack already_invoiced
```

| 步骤 | 预期 | 结果 |
|------|------|------|
| T1 同步 | cloud job succeeded | ✓ |
| T2 本地列表 | 桌号、整桌、金额正确 | ✓ |
| T3 开票 | FT 票号 + ATCUD；print PRINTED | ✓ |
| T4 重打 | 发票页「重打」成功（2ª Via） | ✓ |

## 4. 按人用例

```text
Cloud 下发 scope_type=split（含 scope_id UUID 的 splits[]）
  → Admin 收银账单显示「按人」
  → 详情：列出各位（如 Ana / Bruno）；已开显示票号
  → 选一人 → NIF（可选）→ 签发 → 草稿保留直至全员开完
  → 第二人同理；全员完成后收银账单消失
  → 禁止：已开任一人后再开整桌（scope_mutex）
```

| 步骤 | 预期 | 结果 |
|------|------|------|
| S1 同步 split 载荷 | 本地 open 草稿含 splits | ✓ |
| S2 Admin UI | 显示按人选择（非误走整桌） | ✓ |
| S3 开 A | FT-A；草稿仍在 | ✓ |
| S4 开 B | FT-B；草稿删除 | ✓ |
| S5 互斥 | 已有按人票后整桌 issue 拒绝 | ✓ |

## 5. 已知缺口（不挡联调；工程扫尾状态）

| 项 | 说明 |
|----|------|
| sync_outbox | **0.4.21 已落地**（IssueFT 同事务入队 + worker；云端副本接口可 stub） |
| §13 鉴权 | **0.4.21 P0 已落地**（loopback trust / required + 终端 + operator_token）；离线 PIN 登录 UI 仍属后续 |
| Cloud by_item | 联调以整桌 / 按人（split）为主；本机 by-item 分单工作台另轨 |

## 6. 执行记录

| 日期 | 执行人 | 整桌 | 按人 | 备注 |
|------|--------|------|------|------|
| 2026-08-29 | Will | 通过 | 通过 | 白云饭店 cloud；Agent ~0.4.20；D4.5 关闭 |

## 修订记录

| 日期 | 变更 |
|------|------|
| 2026-08-22 | 首版：白云饭店 cloud 联调清单；整桌+按人 |
| 2026-08-29 | **联调通过**：前置与整桌/按人步骤标 ✓；D4.5 关闭 |
