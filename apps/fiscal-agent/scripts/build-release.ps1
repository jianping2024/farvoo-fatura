# Build Farvoo Fiscal Agent + Client Windows release artifacts (Windows + Go + Inno Setup 6).
# Usage: .\scripts\build-release.ps1 [-Version 0.4.58] [-Amd64Only]
#
# Artifacts in dist/:
#   FarvooFiscalAgent-windows-amd64.zip
#   FarvooFiscalClient-windows-amd64.zip
#   FarvooFiscalAgent-Setup-amd64.exe
#   FarvooFiscalClient-Setup-amd64.exe
#   SHA256SUMS

param(
  [string]$Version = "",
  [switch]$Amd64Only
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
if (-not $Version) {
  $Version = (Get-Content (Join-Path $Root "VERSION") -Raw).Trim()
}

$Dist = Join-Path $Root "dist"
if (Test-Path $Dist) {
  Remove-Item -Recurse -Force $Dist
}
New-Item -ItemType Directory -Force -Path $Dist | Out-Null

$depsDir = Join-Path $Root "installer\deps"
New-Item -ItemType Directory -Force -Path $depsDir | Out-Null
$bootstrapper = Join-Path $depsDir "MicrosoftEdgeWebview2Setup.exe"
if (-not (Test-Path $bootstrapper)) {
  Write-Host "Downloading WebView2 Evergreen Bootstrapper..."
  Invoke-WebRequest -Uri "https://go.microsoft.com/fwlink/p/?LinkId=2124703" -OutFile $bootstrapper
}

$archs = @(
  @{ Name = "amd64"; GoArch = "amd64" }
)
if (-not $Amd64Only) {
  $archs += @{ Name = "arm64"; GoArch = "arm64" }
}

$ldflags = "-s -w -H windowsgui -X main.Version=$Version"

foreach ($a in $archs) {
  $outDir = Join-Path $Dist $a.Name
  New-Item -ItemType Directory -Force -Path $outDir | Out-Null
  $env:GOOS = "windows"
  $env:GOARCH = $a.GoArch
  Push-Location $Root
  try {
    $agentExe = Join-Path $outDir "FarvooFiscalAgent.exe"
    go build -ldflags $ldflags -o $agentExe .
    if ($LASTEXITCODE -ne 0) { throw "go build agent failed for $($a.Name) (exit $LASTEXITCODE)" }

    if ($a.Name -eq "amd64") {
      $clientExe = Join-Path $outDir "FarvooFiscalClient.exe"
      go build -ldflags $ldflags -o $clientExe ./cmd/fiscal-client
      if ($LASTEXITCODE -ne 0) { throw "go build client failed (exit $LASTEXITCODE)" }
      Set-Content -Path (Join-Path $outDir "FarvooFiscalClient-VERSION.txt") -Value $Version -NoNewline
    }

    Set-Content -Path (Join-Path $outDir "VERSION.txt") -Value $Version -NoNewline
  } finally {
    Pop-Location
  }

  Copy-Item (Join-Path $Root "installer\WINDOWS-README.txt") $outDir -Force
  $agentZip = Join-Path $Dist "FarvooFiscalAgent-windows-$($a.Name).zip"
  if (Test-Path $agentZip) { Remove-Item -Force $agentZip }
  Compress-Archive -Path (Join-Path $outDir "FarvooFiscalAgent.exe"), (Join-Path $outDir "VERSION.txt"), (Join-Path $outDir "WINDOWS-README.txt") -DestinationPath $agentZip -Force
  Write-Host "zip: $agentZip"

  if ($a.Name -eq "amd64") {
    Copy-Item (Join-Path $Root "installer\CLIENT-README.txt") $outDir -Force
    $clientZip = Join-Path $Dist "FarvooFiscalClient-windows-amd64.zip"
    if (Test-Path $clientZip) { Remove-Item -Force $clientZip }
    Compress-Archive -Path (Join-Path $outDir "FarvooFiscalClient.exe"), (Join-Path $outDir "FarvooFiscalClient-VERSION.txt"), (Join-Path $outDir "CLIENT-README.txt") -DestinationPath $clientZip -Force
    Write-Host "zip: $clientZip"
  }
}

$iscc = $null
foreach ($candidate in @(
    "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe",
    "${env:ProgramFiles}\Inno Setup 6\ISCC.exe"
  )) {
  if (Test-Path $candidate) {
    $iscc = $candidate
    break
  }
}
if (-not $iscc) {
  $cmd = Get-Command ISCC.exe -ErrorAction SilentlyContinue
  if ($cmd) { $iscc = $cmd.Source }
}
if (-not $iscc) {
  throw "Inno Setup ISCC.exe not found. Install Inno Setup 6 or run: choco install innosetup -y"
}

if ($Amd64Only) {
  foreach ($issName in @("farvoo-fiscal-agent.iss", "farvoo-fiscal-client.iss")) {
    $iss = Join-Path $Root "installer\$issName"
    Write-Host "ISCC $issName"
    & $iscc "/DMyAppVersion=$Version" $iss
    if ($LASTEXITCODE -ne 0) { throw "ISCC failed for $issName (exit $LASTEXITCODE)" }
  }
} else {
  throw "arm64 installer not configured; use -Amd64Only"
}

$hashFile = Join-Path $Dist "SHA256SUMS"
$lines = Get-ChildItem $Dist -File | ForEach-Object {
  $h = Get-FileHash $_.FullName -Algorithm SHA256
  "$($h.Hash.ToLower())  $($_.Name)"
}
$lines | Set-Content $hashFile -Encoding ascii

Write-Host "Done. Version $Version — artifacts in $Dist"
Get-ChildItem $Dist -File | Format-Table Name, Length
