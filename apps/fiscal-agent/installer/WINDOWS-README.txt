Farvoo Fiscal Agent (Windows)
=============================

Local print + fiscal agent for Farvoo (LAN TCP :9100 and Windows USB / WinSpool).

Installer optional tasks (Setup .exe only)
------------------------------------------
On the wizard step "Select Additional Tasks" you can check:

  Desktop shortcut — add Farvoo Fiscal Agent to your desktop (off by default).
  Sign-in startup — run the agent when you log on to Windows (off by default).

Checked shortcuts are removed when you uninstall. Portable zip has no wizard tasks.

First-time setup (no command line)
----------------------------------
1. Install UNYKA UK56009 driver if using USB (https://unykach.com/).
2. Keep Farvoo Fiscal Agent running after install (finish page "Launch now", or sign in if autostart was enabled).
3. Return to Farvoo Dashboard -> Print assistant.
4. Click "Generate pairing code", copy the code on the Dashboard.
5. On the POS PC tray → Printer settings: open /pair if needed, paste the code; then Scan printers,
   map each print station, Save. Test print is optional. (Tray serves http://127.0.0.1:17892 from start.)
6. Fiscal setup / issue FT: tray → 开票 / Fiscal…, or http://127.0.0.1:17880/
   Local activate (paste product PEM) is on by default — no system env vars required.
   Optional in config.json: "fiscal_allow_local_provision": true, "fiscal_at_env": "mock"
7. The agent stays in the Windows system tray (near the clock, click ^ if hidden).
   Icon color: green = OK, yellow = outside hours or setup, red = error.
   Right-click: printer settings, fiscal, open log folder, exit.
   No need to keep a black console window open.

Debug (show console logs): FarvooFiscalAgent.exe -console

Re-open later: FarvooFiscalAgent.exe configure   (printer mapping; /pair on same port 17892)
   Or: tray icon -> Printer settings…
Re-pair only: use /pair on 17892 while tray is running; or FarvooFiscalAgent.exe pair when tray is not running (17890)
Printer only (legacy first-run): FarvooFiscalAgent.exe setup

Version check
-------------
Installed folder contains VERSION.txt (same as FarvooFiscalAgent.exe -version).
Default: C:\Program Files\Farvoo Fiscal Agent\

Config file (pairing / printers — not fiscal SQLite)
----------------------------------------------------
%USERPROFILE%\.config\farvoo-fiscal-agent\config.json

Fiscal SQLite (invoices / series / keys) lives under the agent data directory
(%LOCALAPPDATA%\Farvoo Fiscal Agent\fiscal.db) — not under Program Files.
Upgrading the Setup does not wipe config or fiscal DB.

Examples:
  "default_printer": "tcp:192.168.1.50:9100"
  "default_printer": "winspool:UK56009"
  "station_printers": {
    "<kitchen-station-uuid>": "tcp:192.168.1.51:9100",
    "fiscal_receipt_printer": "tcp:192.168.1.50:9100"
  }

Discover printers: FarvooFiscalAgent.exe discover

SmartScreen
-----------
Unsigned build: More info -> Run anyway, or Unblock in file Properties.

Support
-------
https://github.com/jianping2024/farvoo-fatura
