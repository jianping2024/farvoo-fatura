# CI 与 Fiscal Agent 发布

> 约定来源：与 `restaurant-ordering` 的 print-agent 发布流程对齐（校验脚本 + tag 触发 Windows CI）。本仓库无 Vercel / on-prem 包。

## 现状

| 事件 | 工作流 / 工具 | 作用 |
|------|----------------|------|
| **push / PR → `main`（改 agent）** | [`.github/workflows/fiscal-agent-ci.yml`](../.github/workflows/fiscal-agent-ci.yml) | `go test` + vet + 交叉编译 |
| **push tag `fiscal-agent-v*`** | [`.github/workflows/fiscal-agent-release.yml`](../.github/workflows/fiscal-agent-release.yml) | 先 **test-linux**，再 Windows 安装包 + GitHub Release + **verify-release** |
| **推 main** | [scripts/push-to-main.sh](../scripts/push-to-main.sh) | 可选：自动提交、推 main、校验后发 tag |

**正式发布产物唯一来源**：GitHub Actions 在 Windows 上构建的 `FarvooFiscalAgent-Setup-amd64.exe` / zip / `SHA256SUMS`。  
**禁止**在 macOS 上 `go build` + zip 当作「已发版」交付。

---

## 推送到 main（`./scripts/push-to-main.sh`）

```bash
./scripts/push-to-main.sh
# 跳过自动打 tag：
PUSH_SKIP_FISCAL_AGENT_TAG=1 ./scripts/push-to-main.sh
```

若本次提交改动了 **`apps/fiscal-agent/` 业务代码**（相对上一个 `fiscal-agent-v*` tag；**不含** `VERSION` / `RELEASE_NOTES.md` / `README.md` / `dev/`），同一批提交须含：

1. **`apps/fiscal-agent/VERSION` 递增**
2. **`apps/fiscal-agent/RELEASE_NOTES.md`** 对应 `## X.Y.Z` 段落

校验在 **push main 之前** 执行（`validate-fiscal-agent-release.sh`）；失败则 **main 不会先推上去**。通过后 `apply-fiscal-agent-tag.sh` 会：

1. `go test` + `go vet`（同 CI）
2. `git tag fiscal-agent-v{VERSION}` && `git push origin` 该 tag → 触发 Windows 安装包构建

仅改 `RELEASE_NOTES.md`（同步旧版说明）不算业务代码变更，不会要求 bump VERSION。

---

## 发 Fiscal Agent 版本

**原则**：改业务逻辑 ≠ 改打包脚本；打包路径固定（Inno + `dist/`）。失败多半是 **没跑测试就 tag**，或 **workflow YAML 写坏**。

### 发版前（本地，推荐）

```bash
./scripts/check-fiscal-agent.sh
# 或一步：改 VERSION 后
./scripts/tag-fiscal-agent.sh 0.4.45
git push origin main   # 若 VERSION 有新提交
```

### Release 说明（自动）

| 时机 | 行为 |
|------|------|
| **打新 tag `fiscal-agent-v*`** | [fiscal-agent-release.yml](../.github/workflows/fiscal-agent-release.yml) 从 `RELEASE_NOTES.md` 经 `scripts/fiscal-agent-release-body.sh` 写入 Release 正文 |
| **已发版后只改了 `RELEASE_NOTES.md`** | push `main` 触发 [sync-fiscal-agent-release-notes.yml](../.github/workflows/sync-fiscal-agent-release-notes.yml) |
| **手动刷新某一版** | GitHub → Actions → **Sync fiscal agent release notes** → Run workflow，可选填版本号 |

### 发版步骤

1. 改 **`apps/fiscal-agent/VERSION`**（与将要打的 tag 一致）
2. 在 **`apps/fiscal-agent/RELEASE_NOTES.md`** 增加对应 `## X.Y.Z` 段落
3. **`./scripts/push-to-main.sh`**（或 merge + push）— 确认 **fiscal-agent CI** 在 main 上为绿；若 agent 有改动会自动打 tag
4. 等 **fiscal-agent-release** 全绿（`test-linux` → Windows 打包 → `verify-release`）
5. 在 [Releases](https://github.com/jianping2024/farvoo-fatura/releases) 确认有 **`FarvooFiscalAgent-Setup-amd64.exe`**
6. 可选：`./scripts/wait-for-github-release.sh fiscal-agent-vX.Y.Z`

### Tag 格式

- Tag：`fiscal-agent-v{VERSION}`
- `VERSION` = `apps/fiscal-agent/VERSION` 文件内容（必须一致）

---

## 给 Agent / 协作者

- **Fiscal agent**：必须 push `fiscal-agent-v*` tag 才会打安装包；merge main **不等于** 可下载。
- **不要**在 Mac 上本地打 Windows 安装包冒充 Release。
- Tag 前跑 `./scripts/check-fiscal-agent.sh`。
- **不要改** `fiscal-agent-release.yml` 里 Inno/路径，除非真要改安装方式。
