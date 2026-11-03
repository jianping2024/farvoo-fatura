---
name: interactive-scenario-test
description: Full-scenario interactive task-style testing for fiscal-agent — Claude builds the complete scenario matrix for a change (happy, boundary, negative, fail-closed, cross-type, state carry-over, UI/i18n), starts the environment (mock or FISCAL_AT_ENV=test), hands the user one task card at a time to perform in the real Admin, verifies each result from DB/SOAP dump/logs, tracks coverage until every scenario is pass/blocked, then writes the test report. Use when the user wants to test a change or continue testing ("我来测你验证", "继续 GT 测试", "全场景测一下", "AT 恢复了接着测").
---

# 全场景互动任务式测试（我出任务 → 用户点 → 我验证 → 覆盖全）

**重点是场景覆盖全**：先列全场景矩阵，再一张张任务卡推进，每个场景都落到【已验证 pass / fail】或【阻塞（原因）】，不许静默跳过。

权威来源：场景口径 `docs/fiscal-doc-type-test-scenarios.zh.md`（各票种 `XX-01…` 表）、用途 `docs/fiscal-doc-types-usage.zh.md`、各票种设计文（如 `fiscal-gt.zh.md`）；AT live 细则 `.cursor/rules/fiscal-at-live-testing.mdc`（失败分类、只读探针、报告结构）。范例报告：`docs/test-scenarios/2026-11-03-a2-admin-gt-test.md`。

## 分工（P0）

- **用户**在自己的浏览器里操作 Admin 真界面（登录、开票、选票种、填表、签发、注册系列、作废……）。
- **Claude** 不替用户点 Admin；负责：列场景、起环境、发任务卡、核对结果、修缺陷、记覆盖、写报告。
- **先确认阶段**：读 `docs/fiscal-at-live-integration.zh.md` 刀表、`docs/fiscal-dev-plan.zh.md` 摘要和最近的 `docs/test-scenarios/*`，弄清这次测的是哪一刀 / 哪张票，按依赖顺序排（例：A2 先 GR 再 GT）。
- 不可逆操作（AT 注册系列、真发运输单耗号、改测试库）先问用户；密码永不进对话 / 文件 / 命令行。

## 1. 列全场景矩阵（开测前，一次给全）

从权威场景表取该票种全部 ID，再按下面维度补齐本次改动涉及的场景，每条写「前置 / 操作 / 期望 / 类型」：

| 维度 | 要问自己的 | 例（GR/GT） |
|------|-----------|-------------|
| 正路径 | 每个业务场景的最小合法单 | GR 发客户；GT 调拨；GT 外送 |
| 边界 | 规则临界值 | 开运时间 = 当前分钟 / 早一分钟；邮编格式 |
| 默认值 | 留空时系统补什么 | 装货地留空 = 门店、卸货地留空 = 收货地址 |
| 负路径 / 校验 | 每条校验都触发一次，提示本地化、焦点正确 | 缺收货地址；开运时间已过；调拨 NIF ≠ 本店 |
| 外部失败 | AT 不可用 / 拒绝时 | 失败关闭：零新行、不占号；三语提示 |
| 状态残留 | 切换票种 / 场景 / 返回上一步后旧值是否清干净 | 调拨→GR 购方回散客；GT→FT 地址块隐藏 |
| 关联 | 能挂 / 不能挂谁，金额是否对齐 | GT 无需 FT；FT→GR 软关联 |
| 后续动作 | 作废、重打、列表筛选、SAF-T 导出 | GT 作废需理由；MovementOfGoods 含 GT |
| 幂等 / 并发 | 重复点击、同 request_id | 不重复通信 AT、不跳号 |

矩阵写进当天报告草稿（`docs/test-scenarios/YYYY-MM-DD-<主题>.md` 的「详细测试场景」），并在对话里给用户一张**覆盖表**（ID / 场景 / 状态：待测）。能用 `go test` / `scripts/fiscal-*-regression.mjs` 自动覆盖的，Claude 自己跑并标出，只把**必须真界面 / 真 AT** 的留给用户。

## 2. 起环境

- **mock**：`.claude/launch.json` 的 `fiscal-local-mock-demo`（每次新库，端口 17882）。
- **AT test**：先看 `tail ~/Library/Logs/farvoo-at-watch.log`（2002 = 系列服务正常），再起 `fiscal-local-at-test`（端口 17881）；无 launch.json 时用 cursor rule §1 的命令。日志须见 `at_env=test`、`db=…fiscal-test.db`。
- 改了 Admin HTML/JS（`go:embed`）→ **重启** Agent，并让用户 **Cmd+Shift+R**。
- 记基线：

```sh
cd apps/fiscal-agent/data
ls -t at-soap-dump/*meta.txt | head -1
sqlite3 fiscal-test.db "select document_type,count(*) from invoices group by 1;" \
  "select series_code,document_type,last_number,validation_code,status from series;"
```

## 3. 任务卡（一次一张，或一组互不影响的小卡）

每张卡包含：

- **场景 ID + 一句话目的**（如「GT-02 调拨，只开 GT 不开 FT」）。
- 地址 `http://127.0.0.1:<端口>`，PIN 见 `scripts/fiscal-session-helper.mjs` 的 `DEFAULT_PIN`。
- 编号操作步骤；票种**写中文名 + 代码**（发货单 = GR，运输单 = GT）；哪些字段要填、哪些留空。
- **预期**：单号（如 `GT GT2026T01/1`）、`ATCUD` = 验证码-序号、提示文案、界面状态——让用户能自己对照。
- **请用户留意**：提示是否本地化、焦点、按钮文案、默认值是否合理（体验问题记进报告，不在本刀擅改）。
- 已知坑提醒：签发前确认「发票类型」是目标票种（可能回默认 FT）；GR 散客要取消「收货方是 IVA 纳税人」。

发完用 `wrap_up` 留按钮「做完了，帮我验证」，然后停下等。

## 4. 验证（用户说做完了）

```sh
cd apps/fiscal-agent/data
ls -t at-soap-dump/*meta.txt | head -2
cat at-soap-dump/latest-meta.txt
sed 's/></>\n/g' at-soap-dump/latest-response.xml | grep -E "ReturnCode|ReturnMessage|ATDocCodeID|faultstring"
sed 's/></>\n/g' at-soap-dump/latest-request-body.xml | grep -E "DocumentNumber|ATCUD|MovementType|CustomerTaxID|CustomerName|Addressdetail|PostalCode|MovementStartTime|<Address"
sqlite3 fiscal-test.db "select document_type,invoice_no,atcud,at_transport_doc_code,document_status,created_at from invoices order by created_at desc limit 3;" \
  "select series_code,document_type,last_number,status from series;"
```

（mock 库换成启动日志里的 `db=` 路径；负路径另看 `read_network_requests` / Agent 日志里的错误码。）

1. **先确认做的是这张卡**：最新单据 / 报文的时间、类型对得上；对不上（如多了 FT、没有新报文）→ 如实告诉用户，该卡仍「待测」，重发卡。
2. 按期望逐项核对：成功 = `ReturnCode 0` + `ATDocCodeID`，本地单号 / `ATCUD` / `at_transport_doc_code` 一致，`last_number` +1，首张 `previous_hash` 空；失败 = cursor rule §3 分类 + 失败关闭核对。
3. 扫一遍报文与落库里**不合理的数据**（购方是本店自己、城市与邮编不符……），问用户是否本意。
4. **发现缺陷当场修，不攒到最后**：找根因 → 改唯一写入口 → 补测试 → 跑相关回归 → 重启 → 页面验证 → 提交 → 让用户复测，再发下一张卡。只有需要产品拍板的问题（规则取舍、新功能）才记下来问用户。
5. 更新覆盖表状态，发下一张卡。阻塞的写清原因和「代码侧已就绪 / 还差什么」，不卡住其余场景。

## 5. 收尾（覆盖表全部有结论后）

- 报告按 cursor rule §6 结构：环境、UTC 时间、ReturnCode / `ATDocCodeID`、失败关闭结果、证据等级【已验证】/【未验证】/【阻塞】、覆盖表、未改的体验问题。**未覆盖的场景必须列出并说明原因。**
- 同步进度：`docs/fiscal-at-live-integration.zh.md` 刀表与 live 记录、`docs/fiscal-dev-plan.zh.md` 摘要；关闭条件没全满足（如 mock 回归未跑）不写「已完成」。
- 改测试库前先 `cp fiscal-test.db fiscal-test.db.bak-<日期>-<原因>`，写进报告。
- 用户说提交再提交；`data/` 不入库，`.claude/launch.json` 是本机路径不提交。

## 已踩过的坑

- **AT 按「系列码 + 号」判重、不看类型**（推断）：探针 GR 系列 `GR2026T01` 与同码 GT 系列撞号 → `ReturnCode -3`。新系列码别与探针系列重名。
- 每票种只有一个 ACTIVE 系列、无换系列入口：GT 卡变灰时只能在测试库把旧行（0 张单）标 `RETIRED` 再注册新系列。
- GT「调拨」自动带出本店为购方；切走调拨必须撤回（2026-11-03 修复，属「状态残留」维度）。
- 地址只有「收货地址」必填；报文里装 / 卸 / 收三者齐全。
- `scripts/fiscal-*-regression.mjs` 启动前会 `pkill -f fiscal-local`，会把用户正在用的 test Agent 一起杀掉：**先跑回归，再起环境**；跑完回归要重启 Agent 并告诉用户刷新。
- **验证前先确认 Agent 跑的是哪份代码**：`git branch --show-current` + 关键改动 `grep` 在源码里；别的窗口切过分支时，Agent 重启会编译到别的分支。
- 「用户签的不是目标票种」先当缺陷查，别先怪操作：2026-11-03 三张误签 FT 的根因是 FT/FS 推荐在输入 NIF 时覆盖了已选的 GR/GT。
