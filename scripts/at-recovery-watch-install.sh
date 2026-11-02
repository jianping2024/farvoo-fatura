#!/bin/sh
# Installs the AT TEST health watch (scripts/at-recovery-watch.sh) as a launchd job every 30 min.
# launchd jobs cannot read ~/Documents (TCC), so the probe is compiled and copied, with the public
# AT test certs, to ~/Library/Application Support/farvoo-at-watch. Re-run after changing the probe.
#
#   sh scripts/at-recovery-watch-install.sh            # install / reinstall
#   sh scripts/at-recovery-watch-install.sh uninstall
set -eu
ROOT=$(cd "$(dirname "$0")/.." && pwd)
DEST="$HOME/Library/Application Support/farvoo-at-watch"
LABEL=com.farvoo.at-recovery-watch
PLIST="$HOME/Library/LaunchAgents/$LABEL.plist"

launchctl bootout "gui/$(id -u)/$LABEL" 2>/dev/null || true
if [ "${1:-}" = uninstall ]; then
	rm -rf "$DEST" "$PLIST"
	echo "uninstalled"
	exit 0
fi

CERTS="testdata/transport/certificados/current"
mkdir -p "$DEST/$CERTS"
(cd "$ROOT/apps/fiscal-agent" && go test -c -tags=at_series_live -o "$DEST/at.test" ./internal/fiscal/at)
cp "$ROOT/apps/fiscal-agent/internal/fiscal/at/$CERTS/TesteWebservices.pfx" \
	"$ROOT/apps/fiscal-agent/internal/fiscal/at/$CERTS/ChavePublicaAT.cer" "$DEST/$CERTS/"
cp "$ROOT/scripts/at-recovery-watch.sh" "$DEST/"
sed "s|/Users/chenjianping/Documents/farvoo-fatura/scripts/at-recovery-watch.sh|$DEST/at-recovery-watch.sh|" \
	"$ROOT/scripts/at-recovery-watch.plist" >"$PLIST"
launchctl bootstrap "gui/$(id -u)" "$PLIST"
echo "installed: $DEST (runs now, then every 30 min)"
