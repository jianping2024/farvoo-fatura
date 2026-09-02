#!/usr/bin/env bash
# Regenerate assets/app_icon.ico and Windows rsrc_windows_*.syso for Agent + Client.
# Requires: python3, go install github.com/akavel/rsrc@latest
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
python3 scripts/gen-app-icon.py
RSRC="$(go env GOPATH)/bin/rsrc"
if [[ ! -x "$RSRC" ]]; then
  echo "installing akavel/rsrc..."
  go install github.com/akavel/rsrc@latest
fi
"$RSRC" -arch amd64 -ico assets/app_icon.ico -o rsrc_windows_amd64.syso
"$RSRC" -arch arm64 -ico assets/app_icon.ico -o rsrc_windows_arm64.syso
"$RSRC" -arch amd64 -ico assets/app_icon.ico -o cmd/fiscal-client/rsrc_windows_amd64.syso
echo "ok: app_icon.ico + rsrc_windows_*.syso"
