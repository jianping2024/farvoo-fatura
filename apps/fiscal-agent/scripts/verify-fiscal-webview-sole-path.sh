#!/usr/bin/env bash
# Sole-path self-check for fiscal WebView2 UI thread (0.4.67).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
cd "$ROOT/apps/fiscal-agent"
go test ./internal/fiscalwebview/ -run TestWebViewSoleConstructionPath -count=1

echo "verify-fiscal-webview-sole-path: OK"
