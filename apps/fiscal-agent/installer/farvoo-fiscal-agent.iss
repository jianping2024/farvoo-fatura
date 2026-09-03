; Farvoo Fiscal Agent — Inno Setup (x64). From apps/fiscal-agent:
;   ISCC /DMyAppVersion=0.1.0 installer\farvoo-fiscal-agent.iss
;
; Upgrade story (sole path): elevated Setup → PrepareToInstall taskkill of
; FarvooFiscalAgent.exe then legacy MesaPrintAgent.exe (no AppMutex, no
; CloseApplications yes/no) → overwrite Program Files → optional launch.
; Tray single-instance mutex stays in Go only.

#ifndef MyAppVersion
  #define MyAppVersion "0.1.0"
#endif

#define MyAppName "Farvoo Fiscal Agent"
#define MyAppExe "FarvooFiscalAgent.exe"
#define MyAppPublisher "Farvoo"
#define MyAppURL "https://github.com/jianping2024/farvoo-fatura"
; Legacy exe name — kill during one-release Mesa → Farvoo migration only.
#define MyLegacyExe "MesaPrintAgent.exe"

[Setup]
; AppId={{GUID}} → Uninstall registry subkey "{GUID}}_is1" (extra "}"). Tray find derives
; keys from fiscalAgentInnoGUID in tray_uninstall_common.go — keep GUID in lockstep.
AppId={{A3B8F2E1-9C4D-4A2B-8E1F-0D5C6B7A8E9F}}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
; Wizard title shows version (AppVerName). ARP / tray uninstall keep stem-only DisplayName.
AppVerName={#MyAppName} {#MyAppVersion}
UninstallDisplayName={#MyAppName}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
DefaultDirName={autopf}\{#MyAppName}
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
; Admin: match Program Files + HKLM uninstall so Setup detects prior install (overwrite, not "new").
PrivilegesRequired=admin
UsePreviousAppDir=yes
; No AppMutex (that blocks with "please close then OK/Cancel"). No CloseApplications
; (that asks yes/no to close). PrepareToInstall kills the tray quietly before file copy.
CloseApplications=no
RestartApplications=no
OutputDir=..\dist
OutputBaseFilename=FarvooFiscalAgent-Setup-amd64
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
InfoBeforeFile=wizard-before.txt
InfoAfterFile=wizard-after.txt
SetupIconFile=..\assets\app_icon.ico
UninstallDisplayIcon={app}\{#MyAppExe}

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "Create a Farvoo Fiscal Agent shortcut on the desktop"; GroupDescription: "Desktop shortcut:"; Flags: unchecked
Name: "desktopfiscal"; Description: "Create a Farvoo Fiscal (WebView2) shortcut on the desktop"; GroupDescription: "Desktop shortcut:"; Flags: unchecked
Name: "autostart"; Description: "Start Farvoo Fiscal Agent when you sign in to Windows"; GroupDescription: "Sign-in startup:"; Flags: unchecked
Name: "webview2"; Description: "Install Microsoft Edge WebView2 Runtime if missing (recommended for fiscal UI)"; GroupDescription: "Prerequisites:"

[Files]
Source: "..\dist\amd64\{#MyAppExe}"; DestDir: "{app}"; Flags: ignoreversion restartreplace
Source: "..\dist\amd64\VERSION.txt"; DestDir: "{app}"; Flags: ignoreversion
Source: "WINDOWS-README.txt"; DestDir: "{app}"; Flags: ignoreversion
Source: "deps\MicrosoftEdgeWebview2Setup.exe"; DestDir: "{tmp}"; Flags: deleteafterinstall; Tasks: webview2

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExe}"
Name: "{group}\Farvoo 开票"; Filename: "{app}\{#MyAppExe}"; Parameters: "fiscal"
Name: "{group}\Printer settings"; Filename: "{app}\{#MyAppExe}"; Parameters: "configure"
Name: "{group}\Read me"; Filename: "{app}\WINDOWS-README.txt"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExe}"; Tasks: desktopicon
Name: "{autodesktop}\Farvoo 开票"; Filename: "{app}\{#MyAppExe}"; Parameters: "fiscal"; Tasks: desktopfiscal
Name: "{userstartup}\{#MyAppName}"; Filename: "{app}\{#MyAppExe}"; Tasks: autostart

[Run]
Filename: "{tmp}\MicrosoftEdgeWebview2Setup.exe"; Parameters: "/silent /install"; StatusMsg: "Installing WebView2 Runtime..."; Tasks: webview2; Check: NeedsWebView2; Flags: waituntilterminated
Filename: "{app}\{#MyAppExe}"; Description: "Launch Farvoo Fiscal Agent now"; Flags: nowait postinstall skipifsilent

[Code]
function NeedsWebView2: Boolean;
begin
  Result := not RegKeyExists(HKLM, 'SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}');
  if Result then
    Result := not RegKeyExists(HKLM, 'SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}');
end;

function PrepareToInstall(var NeedsRestart: Boolean): String;
var
  ResultCode: Integer;
begin
  { Quiet stop so Program Files exe can be replaced; ignore non-zero if not running. }
  Exec(ExpandConstant('{sys}\taskkill.exe'), '/F /IM {#MyAppExe} /T', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  { Migration window: also stop legacy Mesa Print Agent process name. }
  Exec(ExpandConstant('{sys}\taskkill.exe'), '/F /IM {#MyLegacyExe} /T', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Result := '';
end;
