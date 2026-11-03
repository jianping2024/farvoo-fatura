---
name: interactive-scenario-test
description: >-
  Fiscal-agent analyze gate + full delivery + full-scenario interactive testing.
  Use when the user asks to analyze first ("先分析", "先方案"), confirms a plan
  ("按方案改", "确认，开始改", "go ahead", "implement it"), wants interactive
  Admin testing ("我来测你验证", "继续 GT 测试", "全场景测一下", "AT 恢复了接着测",
  "做完了，帮我验证"), or asks to land (commit / merge / push). Covers
  best-practice end-state, unique-write-path self-check, redundancy cleanup,
  Chinese business-scenario report, fiscal regressions (mesa-local-product-test
  adapted). "按方案改" stops on the feature branch; merge to main only when the
  user says merge.
---

# 税务改动：方案门 → 按方案改 → 全场景测 →（另说）合进 main

本 skill = **A0 方案门** + **编码交付环** + **全场景互动测试环** + **C 合入（另授权）**。

权威来源：场景口径 `docs/fiscal-doc-type-test-scenarios.zh.md`、用途 `docs/fiscal-doc-types-usage.zh.md`、各票种设计文；方案质量 `.cursor/rules/best-practice-not-patches.mdc` + `.cursor/rules/analysis-before-code.mdc`；AT live `.cursor/rules/fiscal-at-live-testing.mdc`；合入 `.cursor/rules/git-local-merge-push.mdc`；开分支 `.cursor/rules/feature-branch-before-code.mdc`。范例报告：`docs/test-scenarios/2026-11-03-a2-admin-gt-test.md`、`docs/test-scenarios/2026-11-02-at-a0-shared-base.md`。

**不声称能数学保证「最佳实践」**——只保证：默认只推一种终点形状、须引用权威口径、确认后才改、diff 对照终点与唯一写法自检。

## 何时走哪条环

| 用户说法 | 走哪条 |
|----------|--------|
| 「先分析」/「先方案」/ 非平凡缺陷尚无确认方案 | **A0. 方案门**（停等确认） |
| 「按方案改」/ 确认方案后改 | **A. 编码交付环**（可内嵌互动测；**不合 main**） |
| 「我来测你验证」/ 继续测某票种 | **B. 互动测试环**（代码已在或边测边修） |
| 「做完了，帮我验证」 | B 验证；缺陷先 A0 分析再等同意 |
| 「commit」/「提交」 | feature 分支上 commit（仍**不合** main） |
| 「merge」/「合进 main」 | **C. 收尾合入** |
| 「push」/ ship | 推 `origin/main`（另遵 release-and-ci） |

用户确认方案后：**不要逐步请示**，自动跑完 **A（+ 需要时的 B）**；**停在 feature 分支**，汇报「待 merge」。除非用户本线程说了 commit / merge / push。

## 停问（硬）

先问用户再动手：

- AT 注册系列、真发运输单耗号、改/清测试库、`db reset`、硬删生产数据
- `git commit` / `git merge` 进 main / `git push` / force push / `reset --hard` / 删非临时目录（「按方案改」**不**授权 commit 或 merge）
- 缺陷：**先走 A0**（根因 + 终点形状方案），等用户同意再改（提问只答，不顺手修）

密码永不进对话、文件、命令行参数或日志。

## 分工（互动测 P0）

- **用户**在自己的浏览器操作 Admin（登录、开票、选票种、填表、签发、注册系列、作废……）。
- **Agent** 不替用户点 Admin；负责：列场景、起环境、发任务卡、核对 DB/SOAP/日志、修缺陷（同意后）、记覆盖、写报告；合入仅在用户说 merge 之后。

---

# A0. 方案门（未确认终点形状不得编码）

适用：本轮尚无「已确认终点形状」；或用户说先分析 / 发现缺陷要改。完整硬门见 `.cursor/rules/analysis-before-code.mdc`；终点形状口径见 `.cursor/rules/best-practice-not-patches.mdc`。

1. **禁止**改产品代码、开实现分支、mutating 修复。
2. 按五段回复（标题一字不差）：当前业务流程 → 问题根因（【已证实】/【未证实】）→ 推荐方案 → 文件/组件 → 风险与验证。
3. **Heading 3 第一行**必须写最佳实践标记（一眼可见，禁止省略）：
   - `**最佳实践标记：** ✅ 已是最佳实践终点`
   - 或 `**最佳实践标记：** ⚠️ 待 Will 在最佳方案中选择`（仅当 ≥2 个最佳实践备选；禁止把补丁当选项）
   随后才是 **终点形状** + **唯一写法** + **权威核对清单**。
4. 权威核对清单须点名或标【待核实】并停问：设计文/schema、法规或 SAF-T 上限、AT vs 仅本地、Admin 门禁是否误绑。
5. 只推一种最佳实践终点；不并列「小补丁 vs 最佳」。
6. **停等 Will 确认**。确认后才进 **A**。

「按方案改」仅在方案已含 **终点形状 + 唯一写法**（本轮或刚确认的分析里）时生效；缺则先补 A0。

---

# A. 编码交付环（按方案改）

按方案改。
自检：方案里的「唯一写法」在 diff 里只有一份；清冗余后再测。
按 business-scenario-testing：写中文《本次改动业务场景测试报告》，落盘 docs/test-scenarios/YYYY-MM-DD-<功能>.md；再跑 mesa-local-product-test。报告测了什么 + pass/fail，场景标【已验证】/【未验证】。
**只改代码（+ 测 + 报告）。不要 commit、不要 merge 进 main**——等用户说 commit / merge。

展开为固定顺序（不得跳步、不得「先合再测」）：

0. **A0 已过**：本轮有已确认终点形状 + 唯一写法；否则回 A0。
1. **开本任务分支**（`feature-branch-before-code`）：从 `main` `git checkout -b` / 脏树则 `git worktree add`；禁止在 `main` 或无关分支上改产品代码。
2. **按已确认方案改**：唯一写入口；不扩 scope；对齐设计文 / DDL；实现必须匹配终点形状。
3. **自检「唯一写法」+ 终点对照**：方案点名的函数/组件/CSS/错误码在 `git diff` 里各只有一份；禁止平行第二套；**diff 不得出现方案已拒绝的补丁形态**（例：继续写死 `Cancelado`、用藏按钮掩盖错门禁）。报告 §1 写明终点形状、权威依据、唯一写法清单。
4. **清冗余**：删死代码、重复分支、废弃平行实现、无用注释/开关；清完再测（不要带着重复实现去 UAT）。
5. **自动测**：相关 `go test`、触及的 `scripts/fiscal-*-regression.mjs`；改 AT/签发路径时按需 `./scripts/check-fiscal-agent.sh`（push/tag 前必跑，见 `release-and-ci`）。
6. **真界面**：UI/签发/AT 路径 → 转 **B**（或 mock Agent + 浏览器自测）；纯库内无网改动可标互动项【未验证】并说明原因。
7. **写报告**（§报告）→ **mesa-local-product-test 适配**（§产品验收）→ 汇报：分支名、测通/残余风险、**待用户说 commit / merge**。**禁止**自动进入 C。

---

# B. 全场景互动测试环

**重点是场景覆盖全**：先列全场景矩阵，再一张张任务卡推进，每个场景落到【已验证 pass / fail】或【阻塞（原因）】，不许静默跳过。

## B1. 先确认阶段

读 `docs/fiscal-at-live-integration.zh.md` 刀表、`docs/fiscal-dev-plan.zh.md` 摘要、最近 `docs/test-scenarios/*`，确认测的是哪一刀 / 哪张票，按依赖排（例：A2 先 GR 再 GT）。

## B2. 列全场景矩阵（开测前一次给全）

从权威场景表取该票种全部 ID，再按维度补齐本次改动场景，每条写「前置 / 操作 / 期望 / 类型」：

| 维度 | 要问自己的 | 例（GR/GT） |
|------|-----------|-------------|
| 正路径 | 每个业务场景的最小合法单 | GR 发客户；GT 调拨；GT 外送 |
| 边界 | 规则临界值 | 开运时间 = 当前分钟 / 早一分钟；邮编格式 |
| 默认值 | 留空时系统补什么 | 装货地留空 = 门店、卸货地留空 = 收货地址 |
| 负路径 / 校验 | 每条校验都触发一次，提示本地化、焦点正确 | 缺收货地址；开运时间已过；调拨 NIF ≠ 本店 |
| 外部失败 | AT 不可用 / 拒绝时 | 失败关闭：零新行、不占号；三语提示 |
| 状态残留 | 切换票种 / 场景 / 返回上一步后旧值是否清干净 | 调拨→GR 购方回散客；GT→FT 地址块隐藏 |
| 关联 | 能挂 / 不能挂谁，金额是否对齐 | GT 无需 FT；FT→GR 软关联 |
| 后续动作 | 作废、重打、列表筛选、SAF-T 导出 | GT 作废须 reason（≤50，SAF-T）；MovementOfGoods 含 GT |
| 幂等 / 并发 | 重复点击、同 request_id | 不重复通信 AT、不跳号 |

矩阵写入当天报告草稿「详细测试场景」，对话里给用户**覆盖表**（ID / 场景 / 待测）。`go test` / `fiscal-*-regression.mjs` 能覆盖的 Agent 自跑并标出，只把**必须真界面 / 真 AT**的留给用户。

## B3. 起环境

- **mock**：`.claude/launch.json` 的 `fiscal-local-mock-demo`（新库，端口 17882）。
- **AT test**：先 `tail ~/Library/Logs/farvoo-at-watch.log`（2002 = 系列服务正常），再起 `fiscal-local-at-test`（17881）；无 launch.json 用 `.cursor/rules/fiscal-at-live-testing.mdc` §1。日志须见 `at_env=test`、`db=…fiscal-test.db`。
- 改了 Admin HTML/JS（`go:embed`）→ **重启** Agent，用户 **Cmd+Shift+R**。
- 记基线：

```sh
cd apps/fiscal-agent/data
ls -t at-soap-dump/*meta.txt | head -1
sqlite3 fiscal-test.db "select document_type,count(*) from invoices group by 1;" \
  "select series_code,document_type,last_number,validation_code,status from series;"
```

## B4. 任务卡（一次一张）

每张卡：

- **场景 ID + 一句话目的**（如「GT-02 调拨，只开 GT 不开 FT」）。
- 地址 `http://127.0.0.1:<端口>`，PIN 见 `scripts/fiscal-session-helper.mjs` 的 `DEFAULT_PIN`。
- 编号步骤；票种**中文名 + 代码**（发货单 = GR，运输单 = GT）；哪些必填、哪些留空。
- **预期**：单号、`ATCUD`、提示、界面状态。
- **请用户留意**：本地化、焦点、按钮文案、默认值（体验问题进报告「风险」，本刀不擅改）。
- 已知坑：签发前确认票种仍是目标（可能被软推荐改掉）；GR 散客取消「收货方是 IVA 纳税人」；`ready_to_issue=false` 时详情「开发票 / 冲销」等可能不显示——发卡时写明替代路径。

发完停下等「做完了，帮我验证」。

## B5. 验证

```sh
cd apps/fiscal-agent/data
ls -t at-soap-dump/*meta.txt | head -2
cat at-soap-dump/latest-meta.txt
sed 's/></>\n/g' at-soap-dump/latest-response.xml | grep -E "ReturnCode|ReturnMessage|ATDocCodeID|faultstring"
sed 's/></>\n/g' at-soap-dump/latest-request-body.xml | grep -E "DocumentNumber|ATCUD|MovementType|CustomerTaxID|CustomerName|Addressdetail|PostalCode|MovementStartTime|<Address"
sqlite3 fiscal-test.db "select document_type,invoice_no,atcud,at_transport_doc_code,document_status,created_at from invoices order by created_at desc limit 3;" \
  "select series_code,document_type,last_number,status from series;"
```

1. **先确认是这张卡**：时间 / 类型对得上；不对 → 卡仍「待测」，重发。
2. 成功 = `ReturnCode 0` + `ATDocCodeID`，本地单号 / `ATCUD` / `at_transport_doc_code` 一致，`last_number` +1；失败 = fiscal-at-live-testing §3 分类 + 失败关闭。
3. 扫不合理数据（购方是本店、城邮不符）→ 问是否本意。
4. **缺陷当场处理但不攒着偷偷改**：复现 → 根因 → 查设计文判缺陷 vs 本意 → 说清影响与修法 → **等同意** → 唯一写入口 → 补测 → 相关回归 → 重启 → 用户复测 → 再下一张卡。
5. 更新覆盖表；阻塞写清「代码侧已就绪 / 还差什么」。

---

# 报告（business-scenario-testing · 落盘）

路径：`docs/test-scenarios/YYYY-MM-DD-<功能>.md`（中文《本次改动业务场景测试报告》）。聊天摘要与落盘对齐，禁止只聊不写。

固定结构：

1. **本次修改** — 内容、文件；**终点形状**（1–2 句）；**权威依据**（设计文 / SAF-T / 法规路径）；**唯一写法**（自检结论）
2. **核心业务规则** — 不变量；用【已实现】/【本次新增】/【待核实】
3. **详细测试场景** — 可复现（前置 / 操作 / 预期 / 数据验证）；每条结果标 **【已验证】pass|fail** 或 **【未验证】**（原因）或 **【阻塞】**
4. **mesa-local-product-test**（见下节）— 跑了什么 + pass/fail/N/A
5. **实际测试结果** — 总表；未覆盖必须列出
6. **风险与待确认**

AT live 报告另须：环境（env、DB、子用户、端口）、UTC、`ReturnCode` / `ATDocCodeID`、失败关闭核对（对齐 fiscal-at-live-testing §6）。

事实标签：禁止未跑测声称通过；推测标【待核实】。

---

# 产品验收（mesa-local-product-test · 本仓适配）

餐厅仓 skill `mesa-local-product-test`（`:3000` Mesa UI）**不替代** fiscal-agent 验收。本仓对应：

| 改动类型 | 必跑 | mesa-local-product-test |
|----------|------|-------------------------|
| fiscal-agent 库/服务/Admin embed | 相关 `go test` + 触及的 `scripts/fiscal-*-regression.mjs`；AT/发布路径另加 `./scripts/check-fiscal-agent.sh` | 多数 **N/A ·【未验证】**，报告写明「不经 Mesa `:3000`」+ **替代验收**列（已跑脚本） |
| 同时改 restaurant-ordering / 云开票路径 | 上表 + Mesa localhost UAT | 按餐厅仓 skill 真跑 |

回归会 `pkill -f fiscal-local`：**先跑回归，再起 test Agent**，并告诉用户刷新。

---

# C. 收尾合入（仅当用户说 merge / 合进 main）

**触发：** 本线程明确说 `merge` / `合进 main` / `land`（可同时说先 commit）。「按方案改」**不够**。

1. 若尚未 commit：先在本任务 feature 分支 commit（信息说清为什么）。`data/`、`.claude/launch.json` 不入库。
2. **merge 进本地 `main`**，然后按 `git-local-merge-push.mdc`：删本任务 feature 分支/worktree，主工作区回到 `main`；done-gate 全过再汇报。
3. **push `origin/main`** 仅当用户本线程明确说 push / ship / `./scripts/push-to-main.sh`；动了 `apps/fiscal-agent/` 先 `./scripts/check-fiscal-agent.sh`。
4. 同步进度文（`fiscal-at-live-integration` / `fiscal-dev-plan`）仅在关闭条件确实满足时写「完成」；mock 回归未跑、fixtures 未入库 → 不标 A2 完成。

改测试库前：`cp fiscal-test.db fiscal-test.db.bak-<日期>-<原因>`，写入报告。

---

# 已踩过的坑

- **AT 按「系列码 + 号」判重、不看类型**（推断）：探针 `GR2026T01` 与同码 GT 撞号 → `-3`。新系列别与探针重名。
- 每票种一个 ACTIVE 系列、无换系列入口：卡灰只能测库把 0 单旧行标 `RETIRED` 再注册。
- GT「调拨」预填本店购方；切走必须撤回。
- 地址仅收货地址必填；报文装/卸/收三者齐全。
- 验证前确认 Agent 代码：`git branch --show-current` + 关键改动在源码里；别窗切分支会导致重启编到错分支。
- 「用户签错票种」先当缺陷：FT/FS 软推荐曾覆盖已选 GR/GT；定法为 **只升不降**（FS→FT）。
- 作废原因：SAF-T `Reason` ≤50 字；UI 须真输入框，禁止写死 `Cancelado` 却展示「作废原因」空壳。
- 详情「开发票」门禁含 `ready_to_issue`（要云端 `fiscal_profile`）；测试库未配对时按钮不显示——发卡写清用 FT 表单挂同购方 GR 的替代路径。
- 「只挂同一购方 GR」若仅列表保证，服务端缺口须在报告风险里写明，不得假装已闭环。
