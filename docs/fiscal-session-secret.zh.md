# Admin 会话密钥（fiscal_session HMAC）

> **状态：定稿**  
> **权威：是**（Retail 方案 A；与 [`fiscal-p1-security-hardening.zh.md`](fiscal-p1-security-hardening.zh.md) §2.2 一致）  
> **对应实现：** `internal/fiscal/api/session.go`、`fiscal_embed.go`

## Admin 会话密钥（`fiscal_session` HMAC）

| 列 | 类型 | 必填 | 说明 |
|----|------|------|------|
| session_hmac.key | 文件 | Retail 首启自动 | 路径 `{DataDir}/session_hmac.key`；32 字节随机，base64 一行；权限 0600 |
| FISCAL_SESSION_SECRET | env | 否 | 若设则 **覆盖** 文件；≥32 字节 UTF-8；运维轮换用 |
| FISCAL_ALLOW_DEV_KEY | env | 否 | `1` 时 UAT 可无 env/文件，走 dataDir 派生 |

**唯一写法：** `api.NewSessionManager(dataDir, autoFile)`；Retail `autoFile=true` **仅**经 `fiscal_embed.go` → `bootstrap.StartCore`。

**不进** `config.json`。
