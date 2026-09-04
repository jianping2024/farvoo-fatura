Farvoo Fiscal Client — LAN fiscal UI shell (Windows)
========================================================

FarvooFiscalClient.exe connects to a Farvoo Fiscal Agent on your store network
and opens the fiscal UI in a dedicated WebView2 window (no browser address bar).

Requirements
------------
- Windows 10/11 x64
- Microsoft Edge WebView2 Runtime (installer can install it if missing)
- Farvoo Fiscal Agent on another PC with LAN enabled in Admin:
  Settings → Multi-PC invoicing → allow other store PCs to connect → restart Agent
  (or ops env: FISCAL_ALLOW_LAN=1 and FISCAL_BIND=0.0.0.0:17880)

First run
---------
1. Install Farvoo Fiscal Client from Dashboard or GitHub Release.
2. On first launch, enter the Agent PC IP (store LAN) and port 17880.
3. Click Test connection, then Save and open.
4. Log in with operator PIN. If this PC is not registered, enter the one-time
   pairing code from Agent settings → Multi-PC invoicing → Add invoicing PC.

Change Agent IP
---------------
Start menu → Farvoo Fiscal Client Settings
(or run: FarvooFiscalClient.exe --settings)

Agent machine (same PC as Farvoo Fiscal Agent)
------------------------------------------------
Use the tray menu → 开票 / Fiscal…, or desktop shortcut Farvoo 开票
(FarvooFiscalAgent.exe fiscal). That opens http://127.0.0.1:17880/ in WebView2.

Support
-------
Logs: %LOCALAPPDATA%\Farvoo Fiscal Client\config.json
WebView2 session data: %LOCALAPPDATA%\Farvoo Fiscal Client\webview\
