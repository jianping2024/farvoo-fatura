# fiscal-devtool

CLI 日结 FS tip。PIN 与已结算日存在 **exe 同目录** `fiscal-devtool-state.json`（可用 `--state` 覆盖），**不写** `fiscal.db` 旁路表。

```bash
cd apps/fiscal-agent
go test ./cmd/fiscal-devtool/ -count=1
go build -o fiscal-devtool ./cmd/fiscal-devtool
./fiscal-devtool settle --db /path/to/fiscal.db --i-am-dev
```

不进 Agent Inno 安装包。
