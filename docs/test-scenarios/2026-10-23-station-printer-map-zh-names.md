# 本次改动业务场景测试报告：档口打印机映射显示中文名

**日期：** 2026-10-23  
**仓：** farvoo-fatura（`apps/fiscal-agent`）  
**分支：** `feat/station-map-zh-names`

## 1. 本次修改

打印助手「打印机设置」（`/configure`）与首次配对设置页（`setup`）里，出品档口主标题按界面语言显示；副标题列出与主标题不同的其它已填语言（含中文）。

| 项 | 说明 |
|---|---|
| 文件 | `wizard_ui_shared.js`（唯一 `stationPrimaryLabel` / `stationAltLine`）、`configure_ui.html`、`setup_ui.html`、`wizard_ui_shared_test.go` |
| 角色 | 店员 / 装机人员在收银机上配置打印机 |
| 页面 | 打印机映射列表、试打档口下拉 |
| 不改 | 云端 API、映射保存逻辑、小票内容 |

## 2. 核心业务规则

【本次新增】

1. 界面语言为中文时：主标题优先用档口中文名；没有中文名则用葡文，再没有用英文，再没有用档口 id。
2. 界面语言为葡语时：主标题优先葡文。
3. 其它语言（含英语）：主标题优先英文，再葡文、中文、id。
4. 副标题只写「与主标题不同的」已填语言，格式 `ZH … · EN … · PT …`；与主标题相同或未填的不写。
5. 列表主标题与试打档口下拉用同一套主标题规则。
6. 【已实现】档口名解析与优先级只在 `wizard_ui_shared.js` 一份；两页禁止再写 `name_en || name_pt || name_zh`。

## 3. 详细测试场景

### 场景 1：中文界面，档口有三语

- **前置状态：** Agent UI=`zh`；档口 `name_zh=后厨`，`name_en=Kitchen`，`name_pt=Cozinha`
- **操作过程：** 打开映射页，看该档口行与试打下拉
- **预期结果：** 主标题「后厨」；副标题 `EN Kitchen · PT Cozinha`；试打下拉同为「后厨」
- **数据验证：** 不改 `station_printers`；仅展示
- **标签：** 【已验证】（node + Chrome DOM：`后厨` / `EN Kitchen · PT Cozinha`）

### 场景 2：英文界面，同档口

- **前置状态：** UI=`en`；同上三语
- **操作过程：** 看映射行
- **预期结果：** 主标题 `Kitchen`；副标题 `ZH 后厨 · PT Cozinha`
- **标签：** 【已验证】（node + Chrome DOM）

### 场景 3：葡语界面

- **前置状态：** UI=`pt`；同上
- **预期结果：** 主标题 `Cozinha`；副标题含 ZH、EN
- **标签：** 【已验证】（node + Chrome DOM：`Cozinha` / `ZH 后厨 · EN Kitchen`）

### 场景 4：仅有 id 的本地已存映射行（云端名未拉到）

- **前置状态：** 行只有 `id`，无 name_*
- **预期结果：** 主标题为 id；无副标题
- **标签：** 【已验证】（node id fallback）

### 场景 5：configure 用 namesById 补全仅 id 的行

- **前置状态：** 渲染 `{id}`，`stationNamesById[id]` 有中文名
- **预期结果：** 中文 UI 下主标题为中文名
- **标签：** 【已验证】（node namesById）

### 场景 6：两页无第二套优先级

- **操作过程：** `go test` 断言 HTML 无 `function stationLabel` / `name_en ||`；shared 内函数各定义一次
- **标签：** 【已验证】

### 场景 7：保存映射仍可用（回归）

- **前置状态：** 已配对、可保存
- **操作过程：** 选打印机 → 保存（本机有 Agent 时）
- **预期结果：** 保存成功；展示改动不挡保存
- **标签：** 【未验证】（本机未起托盘 `:17892` 实机保存；逻辑未改保存路径）

### 场景 8：mesa-local-uat（本仓 web）

- **说明：** 本次无 `restaurant-ordering` 产品树改动；档口映射 UI 在 farvoo-fatura
- **标签：** 【未验证】（不适用；证据见 fiscal-agent go/node）

## 4. 实际测试结果

| 项 | 结果 |
|---|---|
| `go test`（WizardUIShared / ConfigureUIUses / SetupUIUses / ConfigureUIDOM） | **pass** |
| node：`stationPrimaryLabel` / `stationAltLine` 场景 1–5 | **pass** |
| 清冗余：两页无内联 `stationLabel` / `name_en \|\|`；shared 各 1 份 | **pass** |
| 浏览器：临时页加载同一 `wizard_ui_shared.js`，DOM 断言 zh/en/pt 主副标题 | **pass**（场景 1–3） |
| 本机托盘 `/configure`（`:17892`） | **blocked** — 未监听；用共享脚本临时页等价验展示 |
| 保存映射实机 | 【未验证】 |
| `mesa-local-uat`（restaurant-ordering） | 【未验证】不适用（无 web 改动） |

## 5. 风险与待确认

- 档口未填 `name_zh` 时中文界面仍可能只见葡/英名（数据问题，非展示 bug）。
- 托盘实机页未在本轮打开；上线前建议在已配对收银机用中文 UI 扫一眼列表与试打下拉。
