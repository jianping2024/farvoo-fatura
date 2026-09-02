# 操作记录（审计日志 UI）

> **状态：定稿**  
> **权威：是**（Admin「操作记录」读路径、`audit_log` 展示口径；DDL 仍以 [`fiscal-sqlite-schema.zh.md`](fiscal-sqlite-schema.zh.md) §6.18 为准）  
> **对应实现：** `apps/fiscal-agent`（`store.ListAuditLog`、`GET /local/v1/audit-log`、Admin §audit）；读表 `audit_log`（`migrations/001_init.sql`）；写路径 `store.InsertAuditLog`  
> **计划：** [`fiscal-dev-plan.zh.md`](fiscal-dev-plan.zh.md) §P1（按需）  
> **关联：** [`fiscal-m3-2-operators.zh.md`](fiscal-m3-2-operators.zh.md) §3.7（会话安全；PIN 不写明文）

---

## 1. 目标

| 项 | 定法 |
|----|------|
| 产品名 | **操作记录**（禁止 UI 写「audit_log」「审计表」） |
| 用途 | **admin / owner** 在 Admin 内查看本机已写入的安全与运维事件（谁、何时、做了什么） |
| 非用途 | 替代 **发票** 列表查票；替代 Farvoo 云端日志；实时告警 |

**P0 定法：** 本里程碑 **只读展示** 现有 `InsertAuditLog` 已写入的行；**不**扩展开票/冲销写 audit（见 §6 备选）。

---

## 2. 非目标

| 项 | 说明 |
|----|------|
| `cashier` 可见 | 店员 **不可**进入设置，故 **不可**见操作记录 |
| 导出 CSV / 邮件 | P0 不做 |
| 删改 audit 行 | 禁止；UI 只读 |
| PIN / 密钥 / AT 密码明文 | 禁止出现在 `detail_json` 与 UI |
| 登录页匿名查 audit | 禁止 |
| 云端同步 audit | 禁止；与 §1.1 票库本地权威一致 |
| 按来源 IP 列 | 表无列；P0 不补 DDL（见 §7 备选） |

---

## 3. 入口与信息架构（P0 定法）

### 3.1 放哪里

**P0 定法：放在「设置」内，独立分区「操作记录」。**

| 项 | 定法 |
|----|------|
| 视图 | **不**新增侧栏一级导航（避免与「发票」并列增加噪音） |
| 壳层 | 与「人员 / SAF-T / 高级」相同：`settings-shell` + `data-settings-section="audit"` |
| 设置左侧导航 | 在 **「人员」之后、「设备」之前** 插入链接 **「操作记录」** |
| 移动导航 | 同步插入 `#settingsNavMobile` |
| 折叠 | `<details class="settings-panel">`；默认 **收起**；折叠状态记入 `SETTINGS_SECTIONS_KEY`（与其它分区一致） |
| 进入路径 | 侧栏「设置」→ 左侧「操作记录」；**无**深链到单条 |

**为何不放在「高级」内：** 「高级」= 备份/换机（破坏性运维）；操作记录是 **只读查账**，且 **owner 需可见**（高级对 owner 不可见）。

**为何不放在「发票」旁：** 发票 = 业务票证；操作记录 = 账号与设备事件；用户心智不同。

### 3.2 RBAC 与 CSS 门控

| 角色 | 侧栏「设置」 | 分区「操作记录」 | API |
|------|:------------:|:----------------:|-----|
| `admin` | ✓ | ✓ 全部分类 | `GET /local/v1/audit-log` **authAdmin**（无 action 过滤） |
| `owner` | ✓ | ✓ **子集**（§3.3） | 同上 **authManager**；**服务端**过滤 action |
| `cashier` | ✗ | ✗ | 403 |

| 项 | 定法 |
|----|------|
| CSS | 分区容器加 `settings-manager-only`（与 SAFT 同级；**禁止** `settings-admin-only`） |
| JS 守卫 | `showView('settings')` 已有；加载 audit 前 `canAccessSettings()` |
| 禁止 | owner 仅前端藏行；**必须** API 过滤 |

### 3.3 owner 可见 action 子集（P0 定法）

| action（DB 值） | admin | owner | UI 摘要 |
|-----------------|:-----:|:-----:|---------|
| `LOGIN` | ✓ | ✓ | 登录 |
| `LOGIN_FAILED` | ✓ | ✓ | 登录失败 |
| `LOGOUT` | ✓ | ✓ | 退出 |
| `PIN_CHANGE` | ✓ | ✓ | 修改 PIN |
| `PIN_RESET` | ✓ | ✓ | 重置 PIN |
| `OPERATOR_ACTIVATE` | ✓ | ✓ | 启用开票员 |
| `OPERATOR_DEACTIVATE` | ✓ | ✓ | 停用开票员 |
| `EXPORT_SAFT` | ✓ | ✓ | 导出 SAF-T |
| `fiscal_db_backup` | ✓ | ✗ | 备份税务库 |
| `prepare_machine_swap` | ✓ | ✗ | 换机准备 |
| `series_integrity_failed` | ✓ | ✗ | 系列校验失败 |
| `series_integrity_healed` | ✓ | ✗ | 系列校验修复 |

未列出 action：admin 可见；owner **不可见**（API 不返回）。

---

## 4. UI（P0 定法）

### 4.1 布局

复用 **`admin-list-panel`**（与商品/客户/发票列表同壳）：

| 区域 | 内容 |
|------|------|
| 筛选行 | 日期范围（可选）、**操作类型**下拉（「全部」+ §3.3 映射）、**开票员**下拉（active 名册 +「全部」） |
| 表格 | 列见 §4.2 |
| 底部分页 | `FiscalUI.createListPaginationBar`；默认 `page_size=50` |

**用语：** 表头用业务词（时间、开票员、操作、说明）；**禁止** `action`、`entity_type` 原样暴露给店员。

### 4.2 表格列

| 列 | 数据来源 | 说明 |
|----|----------|------|
| 时间 | `at` | 本机展示 **Europe/Lisbon**（与门店 `taxpayer_settings.timezone` 一致）；存库仍 UTC RFC3339 |
| 开票员 | `operator_id` → `operators.display_name` | 空 operator → 「系统」 |
| 操作 | `action` | §5.1 映射中文 |
| 说明 | `entity_type` + `entity_id` + `detail_json` | §5.2 格式化；过长 ellipsis + title |

**禁止：** 点击行跳转发票详情（P0）；`entity_id` 为 UUID 时不整行展示，用摘要（如「开票员：张三」）。

### 4.3 空态与错误

| 状态 | 文案 |
|------|------|
| 无数据 | 「暂无操作记录」 |
| 加载失败 | Toast；保留筛选条件 |
| owner 无权限 action | 不出现在下拉选项中 |

---

## 5. 数据与展示规则

### 5.1 action → UI 文案（P0 定法）

| action | UI 文案 |
|--------|---------|
| `LOGIN` | 登录 |
| `LOGIN_FAILED` | 登录失败 |
| `LOGOUT` | 退出 |
| `PIN_CHANGE` | 修改 PIN |
| `PIN_RESET` | 重置 PIN |
| `OPERATOR_ACTIVATE` | 启用开票员 |
| `OPERATOR_DEACTIVATE` | 停用开票员 |
| `EXPORT_SAFT` | 导出 SAF-T |
| `fiscal_db_backup` | 备份税务库 |
| `prepare_machine_swap` | 换机准备 |
| `series_integrity_failed` | 系列校验失败 |
| `series_integrity_healed` | 系列校验修复 |

未知 action：admin 列表显示原文；**禁止**崩溃。

### 5.2 说明列格式化（P0 定法）

| entity_type | 规则 |
|-------------|------|
| `operator` | 「开票员：{display_name}」；`entity_id` 解析不到名称时用「未知开票员」 |
| `saft_exports` | 「期间：{year}-{month}」；从 `detail_json` 或二次查 `saft_exports`（P0 允许 export 时写 `detail_json` 含 `period_year` / `period_month`） |
| `series` | 「系列：{series_code}」；来自 `detail_json` |
| `sqlite` | 「路径：{file_name}」；`detail_json.path`  basename |
| `installation` | 「本机换机准备」 |
| 其它 | `detail_json` 键值只读展示；**过滤** 含 `password` / `pin` / `key` 键 |

**P0 不写新 DDL。** 若现有 `detail_json` 不足，实现时 **仅补 InsertAuditLog 的 detail**（不改表）。

### 5.3 物理表（只读引用）

列定义以 [`fiscal-sqlite-schema.zh.md`](fiscal-sqlite-schema.zh.md) §6.18 为准：

| 列 | 类型 | 必填 | 说明 |
|----|------|------|------|
| id | TEXT PK | 是 | |
| at | TEXT | 是 | UTC RFC3339 |
| operator_id | TEXT | 否 | → `operators.id` |
| action | TEXT | 是 | §3.3 |
| entity_type | TEXT | 否 | |
| entity_id | TEXT | 否 | |
| detail_json | TEXT | 否 | 无密钥明文 |

索引：`idx_audit_log_at`（已有）。P0 **不**新增索引；数据量按单店年量级可分页扫。

---

## 6. API（P0 定法）

### 6.1 列表

| 方法 | 路径 | 会话档位 | 说明 |
|------|------|----------|------|
| GET | `/local/v1/audit-log` | **authManager**（owner 可调用；admin 亦可） | 分页 + 筛选 |

**Query（均可选）：**

| 参数 | 类型 | 说明 |
|------|------|------|
| page | int | 默认 1 |
| page_size | int | 默认 50；最大 100 |
| action | string | 精确匹配；owner 若请求不可见 action → **403** |
| operator_id | string | 精确匹配 |
| from | string | ISO8601 下限（含） |
| to | string | ISO8601 上限（含） |

**Response 200：**

```json
{
  "items": [
    {
      "id": "…",
      "at": "2026-09-02T10:00:00Z",
      "operator_id": "…",
      "operator_display_name": "张三",
      "action": "LOGIN",
      "action_label": "登录",
      "summary": "开票员：张三"
    }
  ],
  "page": 1,
  "page_size": 50,
  "total": 123
}
```

| 项 | 定法 |
|----|------|
| 唯一读路径 | `store.ListAuditLog` |
| 排序 | `at DESC` |
| owner 过滤 | SQL `WHERE action IN (...)` 白名单（§3.3） |
| 401 / 403 | 与 M3.2 会话一致 |

**authAdmin 专路径：** P0 **不做**第二条 URL；admin 与 owner 同路径，admin 不传 action 白名单限制。

### 6.2 与现有写路径关系

| 写路径 | 现状 | P0 UI |
|--------|------|-------|
| `store.InsertAuditLog` | 唯一 INSERT | 只读 |
| `operators_auth` LOGIN_FAILED | 直写 SQL + 15min 清理 | 展示 |
| 开票 ISSUE / NC / ND | schema 列了 action；**代码未写** | **不展示**（无行） |

---

## 7. 交付物与验收

| # | 交付物 | 定义「完成」 |
|---|--------|----------------|
| D-A.1 | **本文** | P0 入口、RBAC、API、UI 列定稿 |
| D-A.2 | `store.ListAuditLog` | 分页 + owner 白名单 + 联表 display_name |
| D-A.3 | `GET /local/v1/audit-log` | authManager；403 测 owner 查 admin-only action |
| D-A.4 | Admin §audit | 设置 Nav + 列表 + 筛选分页 |
| D-A.5 | `detail_json` 补强（可选最小） | EXPORT_SAFT / series 行可读摘要 |
| D-A.6 | 回归 | `scripts/fiscal-audit-log-regression.mjs` |

### 验收清单

1. **admin** 登录 → 设置 → 操作记录 → 可见 LOGIN / EXPORT_SAFT / backup 等。  
2. **owner** 同路径 → **不可见** backup / 换机 / series_integrity_*。  
3. **owner** `?action=fiscal_db_backup` → **403**。  
4. **cashier** → 侧栏无设置；直 GET audit-log → **403**。  
5. 表格 **无** PIN / 密码字段；`LOGIN_FAILED` 不暴露尝试 PIN。  
6. 分页：`total` 与 sqlite `COUNT` 一致。  

---

## 8. 备选（P1，非 P0 阻塞）

| 项 | 说明 |
|----|------|
| 开票写 audit | `ISSUE` / `NC` / `ND` / `REPRINT` 在 `IssueDocument` 成功后 `InsertAuditLog` |
| 保留策略 | 如 LOGIN_FAILED 已有 15min 清理；全表保留 365 天 DELETE job |
| 导出 CSV | owner 导出可见子集 |
| `client_ip` 列 | 登录限速配套；需 migration |
| 侧栏一级「操作记录」 | 仅当设置内点击率过低再评估 |

---

## 9. 修订记录

| 日期 | 变更 |
|------|------|
| 2026-09-02 | 草稿：入口=设置分区「操作记录」；admin/owner 分 action 可见；GET API + 列表 UI |
