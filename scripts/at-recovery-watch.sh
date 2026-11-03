#!/bin/sh
# Read-only AT TEST health watch: runs TestLiveConsultSeries (writes nothing at AT) and shows a
# macOS alert with the result (stays on screen; the next run replaces it). Meant for launchd every 30 min while AT test is down
# (install: scripts/at-recovery-watch-install.sh). The password comes from the login keychain, never
# from argv/env files, and is not logged:
#
#   security add-generic-password -s farvoo-at-test -a '519655290/1' -T /usr/bin/security -U -w
#
# Log: ${AT_WATCH_LOG:-$HOME/Library/Logs/farvoo-at-watch.log} (one line per run).
set -u
export PATH="/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:$PATH"
ROOT=$(cd "$(dirname "$0")/.." && pwd)
AT_LIVE_USER="${AT_LIVE_USER:-519655290/1}"
LOG="${AT_WATCH_LOG:-$HOME/Library/Logs/farvoo-at-watch.log}"

PIDFILE="${TMPDIR:-/tmp}/farvoo-at-watch.alert.pid"

notify() { # title, message — an alert that stays until clicked or replaced by the next run
	[ -f "$PIDFILE" ] && kill "$(cat "$PIDFILE")" 2>/dev/null
	afplay /System/Library/Sounds/Glass.aiff >/dev/null 2>&1 &
	nohup osascript -e 'on run argv' -e 'activate' \
		-e 'display alert (item 1 of argv) message (item 2 of argv) giving up after 1790' \
		-e 'end run' "$1" "$(date '+%H:%M') $2" >/dev/null 2>&1 &
	echo $! >"$PIDFILE"
	printf '%s %s | %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$1" "$2" >>"$LOG"
}

AT_LIVE_PASS=$(security find-generic-password -s farvoo-at-test -a "$AT_LIVE_USER" -w 2>/dev/null)
if [ -z "$AT_LIVE_PASS" ]; then
	notify "AT 探测：未运行" "钥匙串里没有 farvoo-at-test / $AT_LIVE_USER 的密码"
	exit 1
fi
export AT_LIVE_USER AT_LIVE_PASS

HERE=$(cd "$(dirname "$0")" && pwd)
if [ -x "$HERE/at.test" ]; then # installed copy (launchd cannot read ~/Documents)
	cd "$HERE" || exit 1
	OUT=$(./at.test -test.count=1 -test.run TestLiveConsultSeries -test.v 2>&1 \
		| grep -vE 'Password|Nonce|UsernameToken')
else
	cd "$ROOT/apps/fiscal-agent" || exit 1
	OUT=$(go test -count=1 -tags=at_series_live -run TestLiveConsultSeries -v ./internal/fiscal/at 2>&1 \
		| grep -vE 'Password|Nonce|UsernameToken')
fi
unset AT_LIVE_PASS

ok=$(printf '%s\n' "$OUT" | sed -n 's/.*AT \([0-9][0-9]*\): \(.*\)$/\1 \2/p' | head -1)
http=$(printf '%s\n' "$OUT" | sed -n 's/.* HTTP \([0-9][0-9][0-9]\)$/\1/p' | head -1)
fault=$(printf '%s\n' "$OUT" | sed -n 's/.*<faultstring[^>]*>\([^<]*\)<.*/\1/p' | head -1)

if [ -n "$ok" ] && printf '%s\n' "$OUT" | grep -q '^--- PASS'; then
	notify "AT 测试环境：系列服务已恢复" "返回 $ok。可以跑运输探针确认。"
elif printf '%s\n' "$fault $OUT" | grep -q '1199'; then
	notify "AT 探测：认证失败 1199" "检查钥匙串里的密码或门户子用户状态"
elif [ -n "$http" ]; then
	notify "AT 测试环境：仍故障" "HTTP $http ${fault:-（无 faultstring）}"
else
	last=$(printf '%s\n' "$OUT" | grep -vE '^\s*$' | tail -3 | tr '\n"' '  ' | cut -c1-160)
	notify "AT 探测：其他错误" "$last"
fi
