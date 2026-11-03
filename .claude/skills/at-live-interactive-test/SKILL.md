---
name: at-live-interactive-test
description: Interactive AT test live testing for fiscal-agent — Claude starts the FISCAL_AT_ENV=test Agent, the user clicks through the real Admin (issue GR/GT/FT, register series), then Claude verifies the SOAP dump, fiscal-test.db and logs and writes the test report. Use when the user wants to test or continue testing against AT test ("继续 GT 测试", "我来测你验证", "AT 恢复了接着测").
---

# AT test 互动式联调（我起环境 → 用户点 → 我验证）

方法细则（失败分类表、只读探针、报告结构）以 `.cursor/rules/fiscal-at-live-testing.mdc` 为准；本 skill 只定**分工与节奏**。范例：`docs/test-scenarios/2026-11-03-a2-admin-gt-test.md`。

## 分工（P0）

- **用户**在自己的浏览器操作 Admin 真界面：登录、开票、选票种、填表、签发、注册系列。
- **Claude** 不替用户点 Admin（不用浏览器面板驱动表单）；只负责起环境、给步骤、核对结果、写报告、修代码。
- **先确认阶段再给步骤**：读 `docs/fiscal-at-live-integration.zh.md` 的刀表（A0–A4 状态）和最近的 `docs/test-scenarios/*a2*` 报告，弄清「下一步测的是哪张票」，别跳步（例：A2 先 Admin GR 走通，再 GT）。
- AT 侧不可逆操作（注册系列、改测试库）先问用户；密码永不进对话 / 文件 / 命令行。

## 1. 起环境

1. AT 是否可用：`tail ~/Library/Logs/farvoo-at-watch.log`（`at-recovery-watch` 只读探针）；返回 2002 = 系列服务正常。
2. 起 test Agent：有 `.claude/launch.json` 的 `fiscal-local-at-test` 就 `preview_start`；否则用 `.cursor/rules/fiscal-at-live-testing.mdc` §1 的命令（`FISCAL_AT_ENV=test`、`FISCAL_AT_SOAP_DUMP`、`fiscal-test.db`、端口 17881）。
3. 看启动日志确认 `at_env=test` 与 `db=…fiscal-test.db`。改了 Admin HTML/JS（`go:embed`）必须**重启**，并让用户 **Cmd+Shift+R** 硬刷新。
4. 记下基线：

```sh
cd apps/fiscal-agent/data
ls -t at-soap-dump/*meta.txt | head -1
sqlite3 fiscal-test.db "select document_type,count(*) from invoices group by 1;" \
  "select series_code,document_type,last_number,validation_code,status from series;"
```

## 2. 给用户的步骤（一条消息说完，然后等）

- 地址 `http://127.0.0.1:17881`，PIN 见 `scripts/fiscal-session-helper.mjs` 的 `DEFAULT_PIN`。
- 编号步骤：票种（**写中文名 + 代码**：发货单 = GR，运输单 = GT）、场景、要填的字段、留空即默认的字段、开运时间。
- 写出**预期**：单号（如 `GT GT2026T01/1`）、`ATCUD` = 验证码-序号，方便用户自己对照。
- 提醒已知坑：签发前看一眼「发票类型」确实是目标票种（下拉可能回到默认 FT）；GR 散客要取消「收货方是 IVA 纳税人」。
- 用 `wrap_up` 留一个按钮「签完了，帮我验证」，然后停下等用户。

## 3. 用户说「签好了」→ 验证

```sh
cd apps/fiscal-agent/data
ls -t at-soap-dump/*meta.txt | head -2          # 有没有新报文
cat at-soap-dump/latest-meta.txt
sed 's/></>\n/g' at-soap-dump/latest-response.xml | grep -E "ReturnCode|ReturnMessage|ATDocCodeID|faultstring"
sed 's/></>\n/g' at-soap-dump/latest-request-body.xml | grep -E "DocumentNumber|ATCUD|MovementType|CustomerTaxID|CustomerName|Addressdetail|PostalCode|MovementStartTime|<Address"
sqlite3 fiscal-test.db "select document_type,invoice_no,atcud,at_transport_doc_code,document_status,created_at from invoices order by created_at desc limit 3;" \
  "select series_code,document_type,last_number,status from series;"
```

- **先确认用户签的是不是目标票种**：最新一行 `invoices` 与最新报文的时间、类型对得上；没有新报文但多了 FT → 票种没切对，如实告诉用户重签。
- 成功：`ReturnCode 0` + `ATDocCodeID`；本地单号、`ATCUD`、`at_transport_doc_code` 与报文一致；`last_number` +1；首张 `previous_hash` 为空。
- 失败：按 cursor rule §3 分类（认证层 fault / ReturnCode / 本地拦截），并做**失败关闭核对**（零新行、`last_number` 不变）。
- 顺带看报文里有没有不合理的数据（购方是本店自己、城市与邮编不符等），问用户是否本意；确认是缺陷再修。
- 修了 UI → 跑 `go test ./internal/fiscal/bootstrap/...` → 重启 Agent → 让用户硬刷新复测。

## 4. 收尾

- 写 / 更新 `docs/test-scenarios/YYYY-MM-DD-<主题>.md`（cursor rule §6 结构）：环境、UTC 时间、ReturnCode / `ATDocCodeID`、失败关闭结果、证据等级【已验证】/【未验证】/【阻塞】、未改的体验问题。
- 同步进度：`docs/fiscal-at-live-integration.zh.md` 刀表与 live 记录、`docs/fiscal-dev-plan.zh.md` 摘要。关闭条件没全满足（如 mock 回归未跑）就不写「已完成」。
- 改测试库前先 `cp fiscal-test.db fiscal-test.db.bak-<日期>-<原因>`，并写进报告。
- 用户说提交再提交；`data/` 不入库，`.claude/launch.json` 是本机路径不提交。

## 已踩过的坑

- **AT 按「系列码 + 号」判重、不看类型**（推断）：探针用的 GR 系列 `GR2026T01` 与同码 GT 系列撞号 → `ReturnCode -3`。新系列码别与探针系列重名。
- 每票种只有一个 ACTIVE 系列、无换系列入口：GT 卡变灰时只能在测试库把旧行标 `RETIRED`（0 张单时）再注册新系列。
- GT「调拨」自动带出本店为购方；地址只有「收货地址」必填，装货地留空 = 门店、卸货地留空 = 收货地址（报文里三者齐全）。
