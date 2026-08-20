Mesa Print Agent (Windows) — v0.2+
===================================

Thermal print agent for Mesa (UNYKA UK56009). One installer for LAN and USB.

Installer optional tasks (Setup .exe only)
------------------------------------------
On the wizard step "Select Additional Tasks" you can check:

  Desktop shortcut — add Mesa Print Agent to your desktop (off by default).
  Sign-in startup — run the agent when you log on to Windows (off by default).

Checked shortcuts are removed when you uninstall. Portable zip has no wizard tasks.

First-time setup (no command line)
----------------------------------
1. Install UNYKA UK56009 driver if using USB (Start menu -> UNYKA driver (web), or https://unykach.com/).
2. Keep Mesa Print Agent running after install (finish page "Launch now", or sign in if autostart was enabled).
3. Return to Mesa Dashboard -> Print assistant.
4. Click "Generate pairing code", copy the code on the Dashboard.
5. On the POS PC tray → Printer settings: open /pair if needed, paste the code; then Scan printers,
   map each print station, Save. Test print is optional. (Tray serves http://127.0.0.1:17892 from start.)
6. The agent stays in the Windows system tray (near the clock, click ^ if hidden).
   Icon color: green = OK, yellow = outside hours or setup, red = error.
   Right-click: printer settings (includes test print), open log folder, exit.
   No need to keep a black console window open.

Debug (show console logs): MesaPrintAgent.exe -console

Re-open later: MesaPrintAgent.exe configure   (printer mapping; /pair on same port 17892)
   Or: tray icon -> Printer settings…
Re-pair only: use /pair on 17892 while tray is running; or MesaPrintAgent.exe pair when tray is not running (17890)
Printer only (legacy first-run): MesaPrintAgent.exe setup

Version check
-------------
Installed folder contains VERSION.txt (same as MesaPrintAgent.exe -version).
Default: C:\Program Files\Mesa Print Agent\

Config file
-----------
%USERPROFILE%\.config\farvoo-fiscal-agent\config.json

Examples:
  "default_printer": "tcp:192.168.1.50:9100"
  "default_printer": "winspool:UK56009"
  "station_printers": {
    "<kitchen-station-uuid>": "tcp:192.168.1.51:9100",
    "<bar-station-uuid>": "winspool:Bar"
  }

Discover printers: MesaPrintAgent.exe discover

SmartScreen
-----------
Unsigned build: More info -> Run anyway, or Unblock in file Properties.

Support
-------
https://github.com/jianping2024/restaurant-ordering
