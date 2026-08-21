# Farvoo Fiscal Agent

Local **print + fiscal** agent for Farvoo: pulls `print_jobs` and prints via **LAN TCP :9100** or **Windows USB** (WinSpool RAW, UNYKA UK56009), and embeds Fiscal Core for AT invoices. ESC/POS.

## Job payload — table fields

All job types that reference a table (`station_ticket`, `order_receipt`, `pre_bill`) include **both**:

| Field | On paper | Purpose |
|-------|----------|---------|
| `display_name` | **Yes** — printed label (e.g. `A-05`) | Snapshot at enqueue time; stable on reprint even if table renamed |
| `table_id` | **No** — never print UUID | Logs, queue filtering, correlation with orders/sessions |

See [`docs/restaurant-tables-design.zh.md`](../../docs/restaurant-tables-design.zh.md) §8 and [`docs/print-agent-plan.md`](../../docs/print-agent-plan.md). Legacy `table_number` in payload is **not** supported after the table migration.

**Product UX / packaging roadmap (tray, onboarding, heartbeat, i18n):** [`docs/print-agent-ux-packaging.zh.md`](../../docs/print-agent-ux-packaging.zh.md).

**End-to-end print flow + Windows spooler rules (do not use `PRINTER_STATUS_*` preflight):** [`docs/print-agent-flow.zh.md`](../../docs/print-agent-flow.zh.md) — especially **§1 开发约束**.

## Commands

### Discover printers (LAN scan)

Scans local IPv4 subnets (from active NICs) for hosts with **TCP 9100** open:

```bash
go run . discover
go run . discover -json
go run . discover -timeout-ms 600 -workers 128
```

Use the printed addresses in `config.json` (see below). Map each **print station** in `configure` / first-run setup. Mapped stations appear on Mesa checkout/bill pages; staff must pick a printer there (default **do not print**).

### Run agent (poll + print)

**Windows (release installer / zip)** — normal use:

1. Install from Dashboard download (or GitHub Release `FarvooFiscalAgent-Setup-amd64.exe`). On **Select Additional Tasks**: optional **desktop shortcut** and **sign-in startup** (both off by default; portable zip has neither).
2. **Double-click `FarvooFiscalAgent`** (or let it start at logon). The agent runs in the **system tray** (taskbar **^** → **FARVOO Fiscal**). **No black console window** — you do **not** need to keep a command prompt open while printing.
3. **First pairing:** generate a 6-digit code in Farvoo **打印助手**, then **Open settings on this PC** (or tray → **Printer settings…**). Tray serves **`/pair` and `/configure` on port 17892 from startup** (including before Connected). Map stations, **Save** (test print optional).
4. **Troubleshooting only:** `FarvooFiscalAgent.exe -console`, or tray → **Show debug console**; log file under `%LOCALAPPDATA%\Farvoo Fiscal Agent\agent.log`. Optional advanced: `FarvooFiscalAgent.exe -api URL -code 123456` or `FarvooFiscalAgent pair` (standalone **17890** only when tray is not running).
5. **Fiscal:** tray → **开票 / Fiscal…** or `http://127.0.0.1:17880/`.

Local HTTP (tray): `http://127.0.0.1:17892/configure` and **`/pair`** on the same port from tray start. CLI-only pairing: `http://127.0.0.1:17890/pair`. Legacy setup-only: `http://127.0.0.1:17891/` (`setup` subcommand).

**Development** (`go run` from this directory):

```bash
go run . -api http://localhost:3000 -code 123456
go run . -api http://localhost:3000 -default-printer 192.168.1.50:9100
```

Config path: `~/.config/farvoo-fiscal-agent/config.json` (Windows: `%USERPROFILE%\.config\farvoo-fiscal-agent\config.json`).

## Config (`config.json`)

```json
{
  "api_base": "http://localhost:3000",
  "agentjwt": "...",
  "device_id": "...",
  "station_printers": {
    "uuid-of-kitchen-station": "192.168.1.51:9100",
    "uuid-of-bar-station": "192.168.1.52:9100"
  },
  "schedule": {
    "timezone": "Europe/Lisbon",
    "weekday": {
      "windows": [
        { "start": "12:00", "end": "15:00" },
        { "start": "19:30", "end": "23:00" }
      ]
    }
  },
  "poll": {
    "idle_interval_sec": 20,
    "busy_interval_sec": 5,
    "after_print_interval_sec": 5,
    "warm_interval_sec": 5,
    "warm_after_activity_sec": 1800,
    "closed_check_sec": 60
  }
}
```

- **`station_ticket`** → `station_printers[payload.print_station_id]` only (no fallback)
- **`order_receipt` / `pre_bill`** → `payload.receipt_printer_id` = `station:{uuid}` when chosen on checkout/bill; if omitted, Mesa defaults to the first mapped station by `print_stations.sort_order`, and the agent falls back to its first mapped station

### Schedule (optional)

When `schedule` is set, the agent **does not call** `pending-jobs` outside the configured windows (saves API traffic). Times are **local wall clock** in `timezone` (IANA name, e.g. `Europe/Lisbon`).

| Key | Meaning |
|-----|---------|
| `weekday` | Default for **every day** (Mon–Sun) unless overridden |
| `saturday`, `sunday` | Optional overrides for those days |
| `monday` … `friday` | Optional per-day override (wins over `weekday`) |

Each window is half-open `[start, end)` using `"HH:MM"` or `"HH:MM:SS"`. If `end` is earlier than `start` on the clock (e.g. `"19:30"`–`"02:00"`), the window crosses midnight. Omit `schedule` entirely to poll 24/7 while the process runs.

### Poll (optional, dynamic intervals)

While **inside** schedule:

| Phase | When | Default interval |
|-------|------|------------------|
| **after_print** | After the whole fetched batch is printed | `after_print_interval_sec` (default 5s) before next `pending-jobs` pull |
| **busy** | Failed to claim job on older servers (`processing` PATCH) | `busy_interval_sec` (default 5s) |
| **warm** | No pending jobs, but printed or saw pending within `warm_after_activity_sec` | 5s |
| **idle** | Open hours, no recent activity | 20s |

While **outside** schedule: sleep `closed_check_sec` (default 60s), no HTTP polls.

**Dashboard overrides:** Owners can edit lunch/dinner hours and poll seconds under **Dashboard → Print assistant** (saved to `restaurants.print_agent_config`). The agent fetches this once at **startup** via `GET /api/print-agent/runtime-config`; it is **not** hot-reloaded while running—restart the tray agent after saving. Dashboard defaults: after-batch **8s**, warm **9s**, quiet-period (`idle_interval_sec`) **10s**; allowed range for those three is **5–60** (idle max **120**). Lower values print sooner but use more API traffic.

**Job claim:** `GET /api/print-agent/pending-jobs` atomically claims matching `pending` rows (`processing` + `claimed_by`) before returning them. The agent PATCHes `done`/`failed` only; a `processing` PATCH remains idempotent for the same device (rolling upgrades). Older agents that still PATCH `processing` after fetch keep working.

**Job max age:** `GET /api/print-agent/pending-jobs` only claims `pending` rows **newer than 10 minutes**. Older `pending`/`processing` rows are marked `failed` by a **server cron (every 5 minutes)** and defensively skipped by the agent. Reconnecting after days offline will **not** replay old kitchen tickets (use dashboard **Retry** if you need a reprint).

Set `poll.fixed_interval_sec` to disable dynamic tiers and use a single interval (legacy behaviour).

See **[dev/config.example.json](./dev/config.example.json)** for a Pirata-style template.

Get station UUIDs from **Dashboard → 餐厅设置 → 出品档口** (`print_stations.id`).

## Windows release (installers)

Version is in **[VERSION](./VERSION)** (currently `0.3.86`). Production builds: **[`.github/workflows/fiscal-agent-release.yml`](../../.github/workflows/fiscal-agent-release.yml)**.

### Publish a release

```bash
# VERSION file must match the tag suffix
git tag fiscal-agent-v0.3.86
git push origin fiscal-agent-v0.3.86
```

Assets (stable names for Dashboard download links):

| File | Use |
|------|-----|
| `FarvooFiscalAgent-Setup-amd64.exe` | Inno installer, x64; optional **desktop shortcut** and **sign-in startup** (both unchecked by default) |
| `FarvooFiscalAgent-windows-amd64.zip` | Portable zip |
| `SHA256SUMS` | Hashes |

ARM64 Setup is not built yet (`build-release.ps1 -Amd64Only`).

### Build on Windows locally

Requires [Go](https://go.dev/) and [Inno Setup 6](https://jrsoftware.org/isinfo.php):

```powershell
cd apps/fiscal-agent
.\scripts\build-release.ps1 -Amd64Only
```

**POS first run:** install → tray icon → Farvoo Dashboard **打印助手** code → **Open settings** → pair → **Scan printers** → map → save. Fiscal Admin: tray **开票 / Fiscal…** or `http://127.0.0.1:17880/`. See **[installer/WINDOWS-README.txt](./installer/WINDOWS-README.txt)**.

Download source of truth: GitHub repo **`jianping2024/farvoo-fatura`** Releases. Farvoo Dashboard `NEXT_PUBLIC_PRINT_AGENT_GITHUB_REPO` should point here (wire in the Dashboard repo separately).

## Local Docker dev (optional, isolated)

Fake printers on host ports **19100–19102** — does not start Mesa or touch app code. See **[dev/README.md](./dev/README.md)**.
