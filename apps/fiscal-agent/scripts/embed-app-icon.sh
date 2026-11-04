#!/usr/bin/env bash
# Regenerate Agent + 开票 .ico and Windows rsrc_windows_*.syso.
# Agent EXE: agent (id 1, shell) + fiscal (id 2, WebView) — comma order matters for akavel/rsrc.
# Client EXE: fiscal only (id 1, shell + WebView).
# Requires: python3, go install github.com/akavel/rsrc@latest
set -euo pipefail
cd "$(dirname "$0")/.."
python3 scripts/gen-app-icon.py
RSRC="$(go env GOPATH)/bin/rsrc"
if [[ ! -x "$RSRC" ]]; then
  echo "installing akavel/rsrc..."
  go install github.com/akavel/rsrc@latest
fi
"$RSRC" -arch amd64 -ico assets/agent_app_icon.ico,assets/fiscal_app_icon.ico -o rsrc_windows_amd64.syso
"$RSRC" -arch arm64 -ico assets/agent_app_icon.ico,assets/fiscal_app_icon.ico -o rsrc_windows_arm64.syso
"$RSRC" -arch amd64 -ico assets/fiscal_app_icon.ico -o cmd/fiscal-client/rsrc_windows_amd64.syso
echo "ok: agent_app_icon.ico + fiscal_app_icon.ico + rsrc_windows_*.syso"
