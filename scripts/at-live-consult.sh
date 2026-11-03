#!/bin/sh
# Read-only AT TEST series consult (A1.1): tells "AT down" apart from "our credentials/request".
# Writes nothing at AT. The password is read silently, never echoed, stored or logged.
#
#   sh scripts/at-live-consult.sh                 # user 519655290/1
#   AT_LIVE_USER='519655290/2' sh scripts/at-live-consult.sh
#
# Output is also saved to ${AT_CONSULT_LOG:-/tmp/at-consult.log} (auth lines filtered).
set -u
ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT/apps/fiscal-agent" || exit 1
AT_LIVE_USER="${AT_LIVE_USER:-519655290/1}"
LOG="${AT_CONSULT_LOG:-/tmp/at-consult.log}"
printf 'AT test password for %s: ' "$AT_LIVE_USER"
stty -echo; read -r AT_LIVE_PASS; stty echo; echo
export AT_LIVE_USER AT_LIVE_PASS
go test -count=1 -tags=at_series_live -run TestLiveConsultSeries -v ./internal/fiscal/at 2>&1 \
  | grep -vE 'Password|Nonce|UsernameToken' \
  | tee "$LOG"
unset AT_LIVE_PASS
