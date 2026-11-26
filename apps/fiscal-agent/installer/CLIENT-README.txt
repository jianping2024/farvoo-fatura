OULIN Fiscal Client — LAN fiscal UI shell (Windows)
========================================================

FarvooFiscalClient.exe connects to an OULIN Fiscal Agent on your store network
and opens the fiscal UI in a dedicated WebView2 window (no browser address bar).

Requirements
------------
- Windows 10/11 x64
- Microsoft Edge WebView2 Runtime (installer can install it if missing)
- OULIN Fiscal Agent on another PC with LAN enabled in Admin:
  Settings → Invoice PCs → allow other store PCs to connect → Save
  (do not set FISCAL_ALLOW_LAN / FISCAL_BIND system environment variables)

First run
---------
1. Install OULIN Fiscal Client from Dashboard or GitHub Release.
2. On first launch, enter the Agent PC IP (store LAN) and port 17880.
3. Click Test connection, then Save and open.
4. Log in with operator PIN. If this PC is not registered, enter the one-time
   pairing code from Agent settings → Invoice PCs → Add invoicing PC.

Change Agent IP
---------------
Start menu → OULIN Fiscal Client Settings
(or run: FarvooFiscalClient.exe --settings)

Agent machine (same PC as OULIN Fiscal Agent)
------------------------------------------------
Use the tray menu → 开票 / Fiscal…, or desktop shortcut OULIN 开票
(FarvooFiscalAgent.exe fiscal). That opens http://127.0.0.1:17880/ in WebView2.

Support
-------
Logs: %LOCALAPPDATA%\OULIN Fiscal Client\config.json
WebView2 session data: %LOCALAPPDATA%\OULIN Fiscal Client\webview\
