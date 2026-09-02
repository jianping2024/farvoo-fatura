; Farvoo Fiscal Client — Inno Setup (x64). From apps/fiscal-agent:
;   ISCC /DMyAppVersion=0.1.0 installer\farvoo-fiscal-client.iss

#ifndef MyAppVersion
  #define MyAppVersion "0.1.0"
#endif

#define MyAppName "Farvoo Fiscal Client"
#define MyAppExe "FarvooFiscalClient.exe"
#define MyAppPublisher "Farvoo"
#define MyAppURL "https://github.com/jianping2024/farvoo-fatura"
#define WebView2Bootstrapper "MicrosoftEdgeWebview2Setup.exe"

[Setup]
AppId={{B7C4E2A1-5D3F-4A8B-9C1E-2F6D8A9B0C3E}}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppVerName={#MyAppName} {#MyAppVersion}
UninstallDisplayName={#MyAppName}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
DefaultDirName={autopf}\{#MyAppName}
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
PrivilegesRequired=admin
UsePreviousAppDir=yes
CloseApplications=no
RestartApplications=no
OutputDir=..\dist
OutputBaseFilename=FarvooFiscalClient-Setup-amd64
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
SetupIconFile=..\assets\app_icon.ico
UninstallDisplayIcon={app}\{#MyAppExe}

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "Create a Farvoo 开票 shortcut on the desktop"; GroupDescription: "Desktop shortcut:"; Flags: checked
Name: "desktopsettings"; Description: "Create Agent settings shortcut on the desktop"; GroupDescription: "Desktop shortcut:"; Flags: unchecked
Name: "webview2"; Description: "Install Microsoft Edge WebView2 Runtime if missing (recommended)"; GroupDescription: "Prerequisites:"; Flags: checked

[Files]
Source: "..\dist\amd64\{#MyAppExe}"; DestDir: "{app}"; Flags: ignoreversion restartreplace
Source: "..\dist\amd64\FarvooFiscalClient-VERSION.txt"; DestDir: "{app}"; DestName: "VERSION.txt"; Flags: ignoreversion
Source: "CLIENT-README.txt"; DestDir: "{app}"; Flags: ignoreversion
Source: "deps\{#WebView2Bootstrapper}"; DestDir: "{tmp}"; Flags: deleteafterinstall; Tasks: webview2

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExe}"
Name: "{group}\Agent settings…"; Filename: "{app}\{#MyAppExe}"; Parameters: "--settings"
Name: "{group}\Read me"; Filename: "{app}\CLIENT-README.txt"
Name: "{autodesktop}\Farvoo 开票"; Filename: "{app}\{#MyAppExe}"; Tasks: desktopicon
Name: "{autodesktop}\Farvoo Fiscal Client Settings"; Filename: "{app}\{#MyAppExe}"; Parameters: "--settings"; Tasks: desktopsettings

[Run]
Filename: "{tmp}\{#WebView2Bootstrapper}"; Parameters: "/silent /install"; StatusMsg: "Installing WebView2 Runtime..."; Tasks: webview2; Check: NeedsWebView2; Flags: waituntilterminated
Filename: "{app}\{#MyAppExe}"; Description: "Launch Farvoo Fiscal Client now"; Flags: nowait postinstall skipifsilent

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
  Exec(ExpandConstant('{sys}\taskkill.exe'), '/F /IM {#MyAppExe} /T', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Result := '';
end;
