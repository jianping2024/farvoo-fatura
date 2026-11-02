# AT 测试 / 正式环境统一接入规划

> **状态：定稿**（规划已齐；**A0、A1 已落地**；下一刀 = A2）  
> **权威：是**（本仓接 AT **真实** test/prod SOAP 的分刀顺序与共享底座；**不**改现有 mock 行为）  
> **对应实现：** A0：`at/crypto.go`、`at/tls.go`、`at/soaputil.go`、`at/live_config.go`；A1/A2 业务 SOAP 仍骨架  
> **现状：** `FISCAL_AT_ENV=mock`（默认/CI）全绿；`test`/`prod` 业务 Client 仍失败关闭（待 A1/A2）  
> **拍板（2026-11-02）：** 材料齐 → AT test 联调；**严格一刀一刀**（A0 → A1 → A2 → A3）  
> **相关：** [`fiscal-m1-identity-series.zh.md`](fiscal-m1-identity-series.zh.md)（系列）、[`fiscal-gr.zh.md`](fiscal-gr.zh.md) §7–8（运输）、[`fiscal-gt.zh.md`](fiscal-gt.zh.md)（T5=A2）、[`fiscal-config-boundary.zh.md`](fiscal-config-boundary.zh.md)、[`fiscal-dev-plan.zh.md`](fiscal-dev-plan.zh.md) §P1

---

## 0. 执行总览（2026-11-02）

| 刀 | 标题 | 状态 | 关闭条件（摘要） |
|----|------|------|------------------|
| **A0** | 共享底座（加密 + mTLS + SOAP 公共件） | **已完成**（2026-11-02；0.5.59） | 无网单测绿；mock 行为不变 |
| **A1** | 系列 SOAP live（test） | **已完成**（2026-11-02）：Admin 经 test 港注册 FT/FS/GR，真验证码入 `fiscal-test.db` | test 注册至少一种销售系列 + GR 系列，得真 `validation_code` |
| **A2** | 运输 SOAP live（test）= 原 G7 / GT T5 | **进行中**：探针真 ATDocCodeID ✅；待 Admin 开 GR 走通 | test 返回真 `ATDocCodeID`；mock 回归仍绿 |
| **A3** | prod 门禁 | 未开始 | 正式证；禁止测试 pfx 打 prod |
| A4 | 观测 / Admin 只读 `at_env`（可选） | 未开始 | 非本阶段硬门槛 |

```text
A0 共享底座（无网）
 └─► A1 系列 live（test 子用户 + mTLS）
      └─► A2 运输 live（修 Auth 500；GR/GT）
           └─► A3 prod 门禁（门店前）
```

**推荐：A1 先于 A2。** 真 `validation_code` 的 GR 系列 + 真 `ATDocCodeID` 才接近认证/门店路径。

---

## 1. 目标

一次规划、分刀落地，把 **两套 AT webservice** 从 mock 接到 **同一套** `test` → `prod` 开关与证书/加密底座：

| 能力 | 接口（逻辑名） | 本仓入口 | 成功产物 |
|------|----------------|----------|----------|
| 系列注册 / 查询 | `registarSerie` / `consultarSeries` | `at.Client` → `RegisterSeries` | `series.validation_code` |
| 运输单据通信 | `envioDocumentoTransporte` | `at.TransportClient` → `RegisterDocument` | `ATDocCodeID` → `invoices.at_transport_doc_code` |

**非目标（本规划）**

| 项 | 说明 |
|----|------|
| 改 mock 语义 | CI / 本地默认仍 mock；不得强制联网 |
| 合并两个 SOAP 成一个 Client 类型 | 分文件保留；只共享 TLS / 密码加密 / 配置 |
| 门店自动切 prod | 须显式配置 + 正式通信证；禁止「有测试证就当生产」 |
| GT 产品化 | mock（T1–T4）已完成；live 复用运输客户端，见 [`fiscal-gt.zh.md`](fiscal-gt.zh.md)（T5=A2） |
| 本阶段改 Hash/ATCUD 本机签 | 切换 `test`/`prod` **只**改验证码 / 运输文档码来源 |

---

## 2. 现状

| 层 | 系列 `at/client.go` | 运输 `at/transport.go` |
|----|---------------------|------------------------|
| mock | 固定 8 位验证码 | `MOCK`+hash 伪码 |
| test/prod | `SOAPClient` 返回 not configured | `SOAPTransportClient` 返回 not wired |
| 材料 | 见 §3；测试子用户填 Admin `at_credentials` | 测试 pfx/cer 已落 `testdata/transport/`；厂商 adesão 公开证已落 `testdata/producer/` |
| 业务依赖 | 开 FT/FS/… **前**须 ACTIVE 系列 | 开 GR/GT **当场**须通信成功 |

探测结论（运输，历史）：TLS 可达 `:701`；错误 xmlns → schema fault；正确包后 **Auth 500**（加密/Token/证绑定未闭环）。系列真 SOAP 尚未做同等探测。→ **归 A2 修**；A0/A1 不挡。

GR mock（G1–G6）与 GT mock（T1–T4）**已完成**；缺的只是 live AT。

**系列连通探测（2026-11-02，`go test -tags=at_series_live -run TestLiveConsultSeries`，只读 `consultarSeries`）：**

| 试验 | 结果 | 结论 |
|------|------|------|
| `TesteWebservices.pfx` → `:722` | mTLS 通；SOAP Fault **1199** 认证/授权 | 证与加密链路 OK，账号不对 |
| 同上 + 2020 旧 `ChavePublicaAT` | Fault **118** | 现行公钥正确（Nonce 能被 AT 解开） |
| `519655290.pfx` → `:722` | TLS 被拒（`bad record MAC`） | `:722` 只收 AT/DGITA Issuing **CA1** |
| `599999993/37` | 1199 | 手册示例账号，不可用 → 须本司 NIF 子用户（§3.2） |
| **`519655290/1`（WSE+WFA）+ `TesteWebservices.pfx`** | **HTTP 200，`codResultOper=2002`**（查询成功，暂无系列） | **A1.1 连通 ✅**（2026-11-02） |
| 注册 FT2026T01 / GR2026T01 | 2001；验证码 `AAJFJG8HRJ` / `AAJFJ48HR6`（10 位） | **A1.2 AT 侧注册 ✅**（2026-11-02） |

修正：A0 `wss` 命名空间改为手册 `http://schemas.xmlsoap.org/ws/2002/12/secext`；`LoadClientTLS` 改用 `go-pkcs12`（OpenSSL 3 / SHA-256 pfx）。

---

## 3. 材料与账号口径（P0）

### 3.1 三套东西，禁止混用

| 材料 | 路径 / 来源 | 用途 | 有效期（核对日 2026-11-02） |
|------|-------------|------|------------------------------|
| 运输/mTLS 客户端 pfx | `apps/fiscal-agent/internal/fiscal/at/testdata/transport/certificados/current/TesteWebservices.pfx`（口令 `TESTEwebservice`） | HTTPS 客户端证（**有私钥**） | → 2027-01-06 |
| AT 密码加密公钥 | 同目录 `ChavePublicaAT.cer` | SOAP 内加密子用户密码 | → 2027-04-15 |
| 厂商 adesão 证 | `…/testdata/producer/519655290.cer`（私钥 + pfx 在仓外 `~/at-certificates/`，**禁止**入仓） | mTLS 客户端证（AT Issuing **CA2**）。实测：test `:722` 只收 CA1 → 测试联调**不用**本证，用公共 `TesteWebservices.pfx` | → 2028-10-28 |

刷新运输公开包：`./scripts/fetch-at-transport-testdata.sh`。

### 3.2 测试子用户 ≠ 软件公司账号 ≠ 门店真账号

| 身份 | 谁提供 | 联调怎么用 |
|------|--------|------------|
| **AT 测试子用户** | **本司 NIF（519655290）下自建子用户**，并授予 “WSE - Comunicação e Gestão de Séries por webservice” 权限（手册 Genéricos §2.3：test 用该子用户**生产**口令校验） | `FISCAL_AT_ENV=test` 时填 Admin「AT 凭证」 |
| **厂商 adesão** | 软件公司在门户申请（已有 `519655290`） | `taxpayer_settings.software_certificate_number`；系列注册字段 |
| **门店真子用户** | 门店在自家 NIF 下开 utilizador | **仅** `prod` / A3；禁止拿真门店账号打 test |

**⚠ 下表仅为手册示例，实测 test 港返回 1199（认证/授权失败），不可用；以本司 NIF 子用户替代：**

| 项 | 值 |
|----|-----|
| 用户名 | `599999993/37` |
| 密码 | `testes1234` |
| 配套纳税人 NIF | `599999993`（测试纳税人；与门店真 NIF 分开） |

存法：默认进 `at_credentials`（Seal）。**产品不提供**第二套 config/env 账号字段；CI 进程 env 覆盖仅开发用（见 §4.0）。

### 3.3 与开票路径

```text
日常开 FT/FS/FR/…     → 本机签；不读 AT（系列已有 validation_code）
注册系列（Admin）     → A1 起可真打 AT
开 GR / GT            → A2 起可真打运输 AT；此前 mock
```

---

## 4. P0 定法（配置与行为）

| 项 | 定法 |
|----|------|
| **产品配置面** | **仅** Agent `config.json`（+ 安装包内材料文件）。**禁止**要求客户改 Windows/系统环境变量 |
| 环境开关 | `config.json` → `fiscal_at_env` = `mock` \| `test` \| `prod`（缺省 `mock`）；系列与运输**同读** |
| 启动桥接 | 托盘 `applyFiscalRuntimeFromConfig`：把 `config.json` 写入**进程内** env，供 `at` 包读取；**不是**写客户机持久系统变量 |
| 失败策略 | test/prod 缺证、加密失败、HTTP/SOAP 业务错 → **整单/整次注册失败**；禁止回落 mock |
| CI / 开发脚本 | 可设进程 env（如 regression）；**不进**安装说明与门店交付清单 |
| 共享底座 | mTLS 客户端证、密码用 AT 公钥加密、SOAP Envelope / UsernameToken、超时 |
| 凭证分离 | 子用户 → SQLite `at_credentials`（Admin）；pfx/cer **文件**随包或安装目录，路径/口令在 `config.json`，**不**进 SQLite 明文 |
| 端点 | 代码内常量；可选 URL 覆盖也只写 `config.json`（运维/打包） |
| 顺序 | **A0 → A1 → A2 → A3**（禁止跳刀先接运输再补加密） |
| 非法 env | `at.ParseEnv` 唯一解析：空=mock；非 `mock\|test\|prod`（如 `production`）→ Fiscal 启动失败、客户端每次调用失败；**禁止**静默当 mock |
| 优先级 | `config.json` 的 `fiscal_at_env` 非空 → **覆盖**进程 env（防门店机残留 env）；为空才用进程 env（CI）或 mock |
| 环境隔离 | Agent 按 env 分库：mock=`fiscal.db`，test=`fiscal-test.db`，prod=`fiscal-prod.db`；每库首开写 `fiscal_meta.at_env` 印记，换 env 打开同一库 → 启动失败（含 `FISCAL_DB` 覆盖路径）。无印记但已有系列/发票的旧库视为 mock。`FISCAL_SEED=1` 仅 mock |

### 4.0 产品 vs CI（P0 硬门禁）

| 场景 | 怎么配 | 禁止 |
|------|--------|------|
| 门店安装包 / 正式交付 | Setup 写入或预置 `config.json`；证文件放安装目录（或相对路径） | 说明书要求设 `FISCAL_AT_*` 系统变量；让店员改「环境变量」面板 |
| 我方联调 / UAT 机 | 同上：改该机 `config.json`（或临时改进程 env 仅开发者本机） | 把开发者机器的系统 env 当成产品设计 |
| CI / `scripts/*-regression.mjs` | spawn 子进程时注入 `FISCAL_AT_ENV=mock` 等 | 依赖开发者 shell profile |

**拍板（2026-11-02）：** AT live 全链路以 **打包配置** 为准；环境变量只作「进程内桥接 / CI」，不对客户暴露。

### 4.1 `config.json` 字段（产品权威名）

| 列（JSON） | 必填（test/prod） | 说明 |
|------------|-------------------|------|
| `fiscal_at_env` | 是（可省略=mock） | `mock` \| `test` \| `prod` |
| `fiscal_at_client_cert` | test/prod 是 | 客户端 pfx 路径（建议相对安装目录或绝对路径） |
| `fiscal_at_client_cert_pass` | test/prod 是 | pfx 口令（测试公开口令可随包；prod 正式口令由运维写入，勿提交公开仓） |
| `fiscal_at_password_pubkey` | test/prod 是 | `ChavePublicaAT.cer` 路径 |
| `fiscal_at_series_url` | 否 | 覆盖默认系列 SOAP URL |
| `fiscal_at_transport_url` | 否 | 覆盖默认运输 SOAP URL |

**不进 `config.json`：** 子用户用户名/密码（继续 Admin → `at_credentials`）。  
**不设字段：** `fiscal_at_client_user` / `_pass` 作为产品配置（避免第二套账号源）；CI 若需覆盖仍可用进程 env，不写进安装模板。

启动时 `applyFiscalRuntimeFromConfig` 映射（进程内，唯一桥接）：

| `config.json` | 进程 env（仅内存） |
|---------------|-------------------|
| `fiscal_at_env` | `FISCAL_AT_ENV` |
| `fiscal_at_client_cert` | `FISCAL_AT_CLIENT_CERT` |
| `fiscal_at_client_cert_pass` | `FISCAL_AT_CLIENT_CERT_PASS` |
| `fiscal_at_password_pubkey` | `FISCAL_AT_PASSWORD_PUBKEY` |
| `fiscal_at_series_url` | `FISCAL_AT_SERIES_URL` |
| `fiscal_at_transport_url` | `FISCAL_AT_TRANSPORT_URL` |

`fiscal_at_env` 例外：config 非空即覆盖进程 env（见 §4 优先级）；其余字段若进程 env 已有值（CI 脚本）则保留。产品路径下客户机不应预置这些系统变量。

A0 已有 `LiveMaterialsFromEnv`：A1 前须保证托盘路径经 `applyFiscalRuntimeFromConfig` 灌齐上表；缺字段时 test/prod 失败关闭并提示「检查 config.json」，**不**提示「请设置系统环境变量」。

**遗留别名（仅代码读，不写进安装模板）：** 旧 `FISCAL_AT_TRANSPORT_*` 见 [`fiscal-gr.zh.md`](fiscal-gr.zh.md) §8；一期后删。

### 4.2 默认端点（落地前再核门户）

| 用途 | 环境 | URL（手册常见值） |
|------|------|-------------------|
| 系列注册 | test | `https://servicos.portaldasfinancas.gov.pt:722/SeriesWSService`（具体 path 以 WSDL/门户为准） |
| 运输通信 | test | `https://servicos.portaldasfinancas.gov.pt:701/sgdtws/documentosTransporte` |
| 门户自检 | — | `https://faturas.portaldasfinancas.gov.pt/testarLigacaoWebService.action` |

prod URL 在 A3 写入；**禁止**用 `TesteWebservices.pfx` 打 prod。

### 4.3 打包落点（设计口径）

| 材料 | 建议位置 | 谁写路径 |
|------|----------|----------|
| `TesteWebservices.pfx` / 正式通信 pfx | 安装目录下固定子目录（如 `at-certs\`），或随 Setup 解压 | Setup / 内部装机清单写入 `config.json` |
| `ChavePublicaAT.cer` | 同上 | 同上 |
| `fiscal_at_env` | `config.json` | 测试包可预置 `test`；门店正式包预置 `prod` 或装机时改一次 |

店员日常：**只开 Admin 配纳税人 + AT 子用户 + 注册系列**；不碰证书路径与 `fiscal_at_env`。

---

## 5. 刀 A0 — 共享底座（**已完成** 2026-11-02）

### 5.1 目标

系列与运输 **共用** 的无网能力：密码加密、mTLS 加载、SOAP UsernameToken / Envelope 最小公共件。  
**不**调用 AT；**不**改 `MockClient` / `MockTransportClient` 语义。

### 5.2 交付物

| # | 交付物 | 定义「完成」 | 状态 |
|---|--------|----------------|------|
| A0.1 | `EncryptATAuth` + `LoadPasswordPublicKey` | 手册算法（AES-128-ECB + RSA-PKCS1）；仓内 cer；固定 AES key → Password/Created 稳定可回解 | **已完成** |
| A0.2 | `LoadClientTLS` | 仓内 `TesteWebservices.pfx`；坏口令/缺文件明确错误 | **已完成** |
| A0.3 | `BuildUsernameTokenXML` / `WrapSOAPEnvelope` | UsernameToken + Security + Envelope；错误解析留给 A1 | **已完成** |
| A0.4 | `LiveMaterialsFromEnv` | §4.1 名 + 旧 `FISCAL_AT_TRANSPORT_*` 别名 | **已完成** |
| A0.5 | 文档 | 本文 §5 关闭 | **已完成** |
| A0.6 | 回归护栏 | `go test ./internal/fiscal/at`；mock 客户端不变 | **已完成** |

**唯一写法（A0）：** `EncryptATAuth`｜`LoadClientTLS`｜`BuildUsernameTokenXML`｜`LiveMaterialsFromEnv` — 各仅一份。

### 5.3 非目标

| 项 | 说明 |
|----|------|
| `SOAPClient.RegisterSeries` 真 HTTP | → A1 |
| `SOAPTransportClient.RegisterDocument` 真 HTTP | → A2 |
| Admin UI 改动 | 不需要 |
| 写入 `series` / 开 GR | 不需要 |

### 5.4 验收清单（必须全过）

1. 底座单测 **零网络**。  
2. `FISCAL_AT_ENV=mock` 下现有系列/运输 mock 行为与回归不变。  
3. 用错误 pfx 口令 → 明确错误（非 panic）。  
4. 核对 pfx/cer `notAfter` 仍晚于执行日（命令见 [`fiscal-gr.zh.md`](fiscal-gr.zh.md) §8.3）。

### 5.5 建议实现路径

```text
apps/fiscal-agent/internal/fiscal/at/
  crypto.go      # EncryptATPassword
  tls.go         # LoadClientTLS
  soaputil.go    # UsernameToken + Envelope（最小）
  *_test.go      # 无网
```

---

## 6. 刀 A1 — 系列 SOAP live（test）

> **前置：** A0 关闭。

### 6.1 目标

`SOAPClient` 打 AT **test** 港：`registarSerie` / `consultarSeries`；Admin 注册系列写入 **真** `validation_code`。

### 6.2 步骤

1. 实现 `SOAPClient.RegisterSeries` / `ConsultSeries`（复用 A0 加密 + mTLS + Token）。  
2. Admin / 现有 setup：纳税人 NIF=`599999993`（或手册现行测试 NIF）；AT 凭证=`599999993/37` + `testes1234`；`numCertSWFatur` = 软件认证号（≤4 位；未认证填 `0`）——**不是** NIF `519655290`。  
3. `config.json`：`fiscal_at_env=test` + §4.1 证路径（经 `applyFiscalRuntimeFromConfig`）；注册至少 **一种销售系列（FT 或 FS）** + **GR 系列**。  
4. 错误可诊断：Auth / schema / 业务码映射进 `at_soap_failed` 摘要（无密码）。  
5. 可选：`-tags=at_series_live` 集成测（默认 skip）；CI 仍 mock。  
6. 手测清单写入本节关闭记录或 `docs/test-scenarios/`。

### 6.2.1 实现口径（A1.2，`at/series_soap.go`）

| 项 | 定法 |
|----|------|
| `registarSerie` 固定值 | `tipoSerie=N`、`numInicialSeq=1`、`meioProcessamento=PI`、`dataInicioPrevUtiliz`=当天（Europe/Lisbon） |
| `classeDoc` | `SeriesDocClass` 唯一映射：FT/FS/FR/ND/NC→SI；GR/GT/GA/GC/GD→MG；PF 等→WD；RG/RC→PY |
| 本地预检（不打 AT） | 系列号 ≤35、`[A-Za-z0-9]` + 单个内部 `[._-]`、禁 `AT` 前缀；`numCertSWFatur` 0..9999 |
| 结果码 | 2001 → 取 `codValidacaoSerie`；**4001（AT 已有）→ `consultarSeries` 取回 ACTIVE 码**（本地库丢失可恢复）；其它 → `AT <码>: <消息>` |
| SOAP Fault | 1199 提示「子用户/密码、WSE 权限、电脑时钟」；118 提示「公钥」；错误串不含密码 |
| 材料 | 懒加载 `LiveMaterialsFromEnv`（托盘经 `applyFiscalRuntimeFromConfig` 从 `config.json` 灌入；相对路径按安装目录）；缺项报 config 字段名。非 test 环境未配 `fiscal_at_series_url` → 失败关闭 |
| live 探针 | `-tags=at_series_live`：`TestLiveConsultSeries`（只读）/ `TestLiveRegisterSeries`（需显式 `AT_LIVE_SERIES_FT` / `_GR`） |
| 验证码长度 | WSDL 写 `length=8`，**AT test 实际返回 10 位**（如 `AAJFJG8HRJ`）；按 Portaria 195/2020「≥8 位字母数字」校验，**禁止**写死 8 |
| 查询结果码 | 有结果时 AT test 也返回 **2002**（非 2000）；两者都按成功处理 |
| 注意 | AT 不可删系列，仅「当天或次日」可 `anularSerie`；test 系列号建议带 `T` 便于识别 |

### 6.3 关闭条件

| # | 条件 |
|---|------|
| 1 | test 环境成功拿到至少一笔销售系列真 `validation_code` |
| 2 | 同上，GR 系列真码 |
| 3 | mock / CI 全绿 |
| 4 | 失败路径不回落 mock |

### 6.3.1 关闭记录（2026-11-02）

环境：`fiscal-local`，`FISCAL_AT_ENV=test`，`fiscal-test.db`（`fiscal_meta.at_env=test`）；公共 `TesteWebservices.pfx`；子用户 `519655290/1`（WSE）；`numCertSWFatur=0`。

| 类型 | 系列 | 验证码 | 路径 |
|------|------|--------|------|
| FT | FT2026DEMO01 | AAJFJK8HRN | Admin → 2001 |
| FS | FS2026DEMO01 | AAJFJ28HRZ | Admin → 2001 |
| GR | GR2026T02 | AAJFJB8H8B | Admin → 2001 |
| GT | GR2026T01 | AAJFJW8H89 | Admin → 2001（误填入 GT 卡；test 数据，保留） |
| — | FT2026T01 / GR2026T01(GR) | AAJFJG8HRJ / AAJFJ48HR6 | live 探针（2001 + 4001→consult 取回） |

关闭条件 1–4 全过（FT + GR 真码；mock/CI 绿；失败不回落 mock）。

**遗留（已修，2026-11-02）：**
1. ~~同号不同类型误判「已注册」~~：`ActiveSeriesHasCode` 带 `document_type`；系列号已被其它类型占用 → **调 AT 前**拒绝（`series_code_taken`，三语提示），避免 AT 侧孤儿系列；`UpsertActiveSeries` **永不改写**已有行的 `document_type`（`ErrSeriesCodeOtherDocType`）。表约束仍为 `UNIQUE(store_id, series_code)`（`invoices` 外键引用 `series.id`，不做重建）；若产品要「同号跨类型」（如统一 `2026A`）须另立迁移。
2. ~~按钮停在「保存中…」~~：`markSaveSuccess` 把忙碌文案当原文、1.8s 后又写回；`withBusy` 以 `data-busy-prev` 交接原文。

### 6.4 非目标

运输通信、开 GR 真 `ATDocCodeID`、prod。

---

## 7. 刀 A2 — 运输 SOAP live（test）= 原 G7

> **前置：** A0 关闭；**建议** A1 已通（真 GR 系列码）。  
> **等同：** [`fiscal-gr.zh.md`](fiscal-gr.zh.md) G7；[`fiscal-gt.zh.md`](fiscal-gt.zh.md) T5。

### 7.1 目标

`SOAPTransportClient.RegisterDocument` → 真 `ATDocCodeID`；接现有 `IssueDeliveryNote` / `IssueTransportNote`（先通信再写库）。

### 7.2 步骤

1. 实现 `envioDocumentoTransporte`（复用 A0）。  
2. 对照门户「测试连接」与脱敏 fixtures，**修历史 Auth 500**。  
3. 脱敏请求/响应样例入 `testdata/transport/fixtures/`（无密码、无私钥）。  
4. `-tags=at_transport_live`；手测最小 GR；GT 复用同一客户端（`MovementType=GT`）。  
5. mock 回归（含 `fiscal-gt-regression.mjs` / GR 相关）仍绿。

### 7.2.1 实现口径（A2，`at/transport_soap.go` + `service/transport_at.go`）

| 项 | 定法 |
|----|------|
| 单据号 | **真实号**：`PrepareTransport` 读下一号（如 `GR GR2026T02/1`）+ ATCUD → AT → `IssueGR/IssueGT(ExpectedSeq)`；号变动 → `ErrTransportSeqMoved` 失败关闭。`transportMu` 串行（Agent 单进程独占 DB） |
| 幂等 | 同 `request_id` 已签发 → 直接返回，**不再**通信 AT |
| 地址 | `domain.Address` 结构化贯穿 API / DB（迁移 017）/ 打印 / SAF-T / AT；不做文本解析。省略装货地＝门店地址，省略卸货地＝客户地址；三者 `ValidatePT` 在 service 统一校验（mock 同 live） |
| ATCUD | 报文带 `<ATCUD>`（现行 AT schema；镜像 WSDL 无此字段，缺则 AT 告警） |
| 错误 | `at.ErrTransportInvalid`（本地拦截）/ `ErrTransportUnavailable`（网络、认证、配置，可重试）/ `ErrTransportRejected`（AT 拒绝）；SOAP fault 与 ReturnCode 统一在 `at/errors.go` 码表（1199、118 → unavailable；33 schema、54 缺 WDT → rejected） |
| 对账 | AT 已登记但本地落库失败 → `audit_log` `at_transport_unsaved`（单号 + ATDocCodeID）并报错 |
| 子用户权限 | 运输需 **WDT**（Webservice de comunicação de documentos de transporte）；系列需 WSE |
| SAF-T | `ShipTo`/`ShipFrom` 按 XSD 置于 `MovementStartTime` 前，写完整 `<Address>`（旧单据缺项填 `Desconhecido`） |

**live 记录（2026-11-02）：** 探针 `GR GR2026T01/2` → `ATDocCodeID 1108714649`（ReturnCode 0 / OK）。

**遗留（另立）：** SAF-T `DocumentStatus` 缺 `SourceBilling`、发票 `DocumentStatus` 非结构 —— 全单据类型 SAF-T 合规问题，未在本刀处理。

### 7.3 关闭条件

| # | 条件 |
|---|------|
| 1 | test 港返回真实 `ATDocCodeID` |
| 2 | 通信失败 → 零发票行（现有失败关闭语义） |
| 3 | mock / CI 全绿 |

---

## 8. 刀 A3 — prod 门禁

> **前置：** A1/A2 在 test 稳定；持有**厂商正式通信证**（非公开测试 pfx）。

1. prod URL + 正式证路径检查：`prod` 且证缺失 → 启动或首次调用即明确错误。  
2. 运维检查表：纳税人 NIF、子用户权限、证有效期、`fiscal_at_env=prod`。  
3. **禁止**用 `TesteWebservices.pfx` 打 prod。  
4. 门店用**自家** AT 子用户，不用 `599999993/37`。

---

## 9. 刀 A4（可选）— 观测与运维

- 审计：系列注册 / 运输通信成功失败写 `audit_log`（无密码、无完整 SOAP）。  
- Admin：设置页只读展示当前 `at_env`（不展示密钥）。

---

## 10. 风险与依赖

| 风险 | 缓解 |
|------|------|
| 测试证过期 | `fetch-at-transport-testdata.sh` + A0/A2 验收核对 `notAfter` |
| 手册测试账号失效 | 改 §3.2；以 AT 现行手册为准 |
| 系列与运输 Auth 规则不一致 | 共用 A0 加密；分别对照两份 WSDL/手册 |
| Auth 500 | A2：脱敏 fixtures + 门户自检对照 |
| 误开 prod | A3 门禁 + 默认 mock |
| 软件认证号与门户不一致 | A1 前核对 `519655290` 与 `numCertificado` 字段要求 |

---

## 11. 共享底座示意（A0）

```text
at/crypto.go
  EncryptATPassword(plain, pubkeyCER) → ciphertext

at/tls.go
  LoadClientTLS(certPath, pass) → *tls.Config

at/soaputil.go
  UsernameToken / Envelope / 错误解析（最小集）
```

| 验收 | 说明 |
|------|------|
| 单测 | 固定明文 + 仓内 cer |
| 无网 | 底座单测不访问 AT |
| 共用 | 系列与运输均调用同一加密 + 同一 TLS loader |

---

## 12. 修订

| 日期 | 说明 |
|------|------|
| 2026-10-15 | 初稿：系列 + 运输统一 test/prod；A0–A3；配置命名拍板 |
| 2026-11-02 | **执行计划落入本文**：§0 总览；材料/账号口径 §3；配置名 §4.1 定稿；A0–A3 步骤与关闭条件写全；下一刀 = A0 |
| 2026-11-02 | **A0 落地**（0.5.59）：`EncryptATAuth` / `LoadClientTLS` / SOAP 公共件 / `LiveMaterialsFromEnv`；下一刀 = A1 |
| 2026-11-02 | **P0 修复**：非法 `fiscal_at_env` 失败关闭；config 覆盖残留 env；按 env 分库 + `fiscal_meta.at_env` 印记 |
| 2026-11-02 | **P0：产品只走打包/`config.json`**，禁止要求客户改系统环境变量；§4.0–§4.3 |
