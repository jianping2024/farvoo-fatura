#!/bin/sh
# AT TEST transport probe (A2): sends the known-good GR from TestLiveTransportGR on series
# GR2026T01 (registered at AT, never issued locally). WRITES to AT test and consumes one
# number of GR2026T01 there — ask before running. Password read silently, never stored.
#
#   sh scripts/at-live-transport-probe.sh <next unused GR2026T01 number at AT>
#
# Output is also saved to ${AT_PROBE_LOG:-/tmp/at-transport-probe.log} (auth lines filtered).
set -u
SEQ="${1:-}"
if [ -z "$SEQ" ]; then echo "usage: $0 <seq>"; exit 2; fi
ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT/apps/fiscal-agent" || exit 1
AT_LIVE_USER="${AT_LIVE_USER:-519655290/1}"
LOG="${AT_PROBE_LOG:-/tmp/at-transport-probe.log}"
printf 'AT test password for %s: ' "$AT_LIVE_USER"
stty -echo; read -r AT_LIVE_PASS; stty echo; echo
export AT_LIVE_USER AT_LIVE_PASS AT_LIVE_GR_SEQ="$SEQ"
go test -count=1 -tags=at_series_live -run TestLiveTransportGR -v ./internal/fiscal/at 2>&1 \
  | grep -vE 'Password|Nonce|UsernameToken' \
  | tee "$LOG"
unset AT_LIVE_PASS
