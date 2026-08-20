# Print agent — local Docker dev (isolated)

Dev-only fake printers + agent. Does not affect Mesa production deploy.

## Quick start (3 terminals)

**Terminal 1 — Mesa**

```bash
npm run dev
```

**Terminal 2 — fake printers + ticket logs**

```bash
npm run print          # rebuild images & start printers (background)
npm run printlog         # watch decoded tickets (all printers)
```

Stop printers: `npm run printstop`

**Terminal 3 — print agent**

```bash
# first time only:
cp apps/print-agent/dev/config.example.json apps/print-agent/dev/config.docker.json
npm run print:dev -- pair 123456   # code from 餐厅设置 → 打印助手
# paste agentjwt + device_id into config.docker.json, set station_printers UUIDs

npm run printagent
```

After changing agent code (`schedule` / `poll`) or `config.docker.json` schedule, **rebuild the agent image**:

```bash
npm run print:dev -- agent-build
npm run printagent
```

Startup should log `schedule enabled` or `outside schedule — sleeping` when closed.

Then place an order (guest menu `?table_id=` or waiter **加菜**); station tickets enqueue automatically. Watch terminal 2 (`npm run printlog`) for decoded ticket text — table line should show **`display_name`**, not UUID.

---

## One script for everything

From repo root (or any directory):

```bash
bash apps/print-agent/dev/print-dev.sh help
```

| Command | What it does |
|---------|----------------|
| `up` | Build & start fake printers (background) |
| `down` | Stop stack |
| `logs` | All printer logs (`-f`) |
| `logs-kitchen` | Kitchen printer only |
| `agent` | Run agent (foreground) |
| `agent-build` | Rebuild agent image |
| `pair <code>` | Pair once (6 digits) |
| `rebuild` | `up --build` + `agent-build` |

**npm shortcuts:** `npm run print` · `npm run printstop` · `npm run printlog` · `npm run printagent` · `npm run print:dev -- pair 123456`

---

## Config (`config.docker.json`)

Inside Docker network use **service names**, not `127.0.0.1:1910x`:

```json
{
  "default_printer": "printer-receipt:9100",
  "station_printers": {
    "<kitchen-station-uuid>": "printer-kitchen:9100",
    "<bar-station-uuid>": "printer-bar:9100"
  }
}
```

UUIDs: Dashboard → **出品档口**. Copy `agentjwt` / `device_id` after `pair`.

---

## Host ports (optional)

| Printer | Host |
|---------|------|
| receipt | `127.0.0.1:19100` |
| kitchen | `127.0.0.1:19101` |
| bar | `127.0.0.1:19102` |

Use these only when the agent runs **on the host** with Go, not inside Docker.

---

## File layout

```text
apps/print-agent/dev/
  print-dev.sh          ← run this
  docker-compose.yml
  config.docker.json    ← gitignored (your secrets)
  config.example.json
  fake-printer/
  Dockerfile.agent
```
