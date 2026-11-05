# 本次改动业务场景测试报告：setup/configure 表单控件排版

日期：2026-11-05  
分支：`fix/setup-ui-form-control-layout`

## 1. 本次修改

- **内容**：打印代理 setup / configure 页，勾选框与「数字 + px」复合行不再吃整行 `input` 样式；说明文字改用正向间距。
- **文件**：
  - `apps/fiscal-agent/setup_ui.html`
  - `apps/fiscal-agent/configure_ui.html`
  - `apps/fiscal-agent/station_slip_one_line_test.go`（回归断言）
- **终点形状**：文本/下拉用整行框样式；checkbox 紧凑；「数字 + 单位」边距挂在 `.inline-unit` 容器上；hint 不再负 margin 硬贴。
- **权威依据**：纯 UI 排版；不适用 fiscal schema / SAF-T / AT。对齐已确认方案（本会话「先分析」→「按方案改」）。
- **唯一写法自检**：
  - 字段样式：`select, input:not([type="checkbox"])`（两页各一份）
  - checkbox：`input[type="checkbox"]` + `.check-label`
  - 数字+单位：`.inline-unit`
  - diff 中已无 `select, input {` 全量套用；无平行第二套补丁样式

## 2. 核心业务规则

- 【已实现】勾选/字号保存逻辑未改；仅 CSS/markup 排版。
- 【本次新增】UI 字符串回归：两页须含 `inline-unit` / `check-label` / `:not([type="checkbox"])`，且禁止 `select, input {`。

## 3. 详细测试场景

| ID | 场景 | 前置 | 操作 | 预期 | 结果 |
|----|------|------|------|------|------|
| L1 | setup 勾选框左右不拉开 | 打开 setup 页含强制一行 | 目视「菜名/备注强制一行」 | 方块紧挨标题左侧 | 【已验证】pass（实码 CSS 截图） |
| L2 | setup `22` 与 `px` 垂直居中 | 同上 | 目视中文位图字号行 | 数字框与 `px` 齐平 | 【已验证】pass（实码 CSS 截图） |
| L3 | setup 说明不贴死控件 | 同上 | 目视 hint 间距 | 正向间距、不挤不歪 | 【已验证】pass（实码 CSS 截图） |
| L4 | configure 同构 markup | embed HTML | `go test` 断言 | 含唯一写法标记、无全量 `select, input {` | 【已验证】pass |
| L5 | i18n 键仍在 | — | `TestStationSlipPrintOptsI18nKeys` | zh/en/pt 齐全 | 【已验证】pass |
| L6 | 真 Agent 浏览器点选保存 | 本机已配对 setup/configure | 勾选并保存 | 配置落盘、下一张票生效 | 【未验证】需本机跑 Agent 真页（本改不动保存逻辑） |

## 4. mesa-local-product-test

N/A ·【未验证】— 不经 Mesa `:3000`。  
**替代验收**：`GOMODCACHE=$HOME/go/pkg/mod go test -count=1 -run 'TestWizardUIHasStationSlipPrintOpts|TestStationSlipPrintOptsI18nKeys' .` → pass；实码 CSS 截图对照终点形状。

## 5. 实际测试结果

| 项 | 结果 |
|----|------|
| 相关 go test | pass |
| 实码排版截图 | pass |
| 真 Agent 界面保存 | 未验证（见 L6） |

未覆盖：生产/已安装 Agent 二进制需重编后才能看到；当前仅源码。

## 6. 风险与待确认

- 收窄 `input` 选择器后，若日后加 `type=radio` 等，需同样排除或专用类。
- 待用户说 **commit** / **merge** 后再合进 main；本报告完成时仍停在 feature 分支。
