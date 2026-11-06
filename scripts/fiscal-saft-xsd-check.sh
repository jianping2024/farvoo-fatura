#!/usr/bin/env bash
# Validate a SAF-T(PT) export covering every section (FT, NC, PF, GR, GT, RG, 0% exempt line)
# against the official AT schema apps/fiscal-agent/internal/fiscal/saft/testdata/SAFTPT1.04_01.xsd.
#
# The schema is XSD 1.1 (xs:assert), which xmllint cannot evaluate, so validation uses the
# pinned Python xmlschema package in a private venv. Source of the schema:
#   https://info.portaldasfinancas.gov.pt/apps/saft-pt04/SAFTPT1.04_01.xsd
# Optional: SAFT_XSD_CHECK_FILES="a.xml b.xml" also validates those files (e.g. a real monthly export).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
AGENT="$ROOT/apps/fiscal-agent"
XSD="$AGENT/internal/fiscal/saft/testdata/SAFTPT1.04_01.xsd"
XSD_SHA256="292c0ab4611e3f5a0cdf2abb4e62d9bd41254dc2e76a1fae4d35a8b132d79350"
XMLSCHEMA_VERSION="4.2.0"
VENV="${SAFT_XSD_VENV:-${XDG_CACHE_HOME:-$HOME/.cache}/farvoo/saft-xsd-venv-$XMLSCHEMA_VERSION}"

command -v python3 >/dev/null 2>&1 || { echo "fiscal-saft-xsd-check: python3 required" >&2; exit 1; }
command -v go >/dev/null 2>&1 || { echo "fiscal-saft-xsd-check: go required" >&2; exit 1; }

actual="$(shasum -a 256 "$XSD" 2>/dev/null || sha256sum "$XSD")"
[[ "${actual%% *}" == "$XSD_SHA256" ]] || { echo "fiscal-saft-xsd-check: $XSD is not the official AT schema (sha256 mismatch)" >&2; exit 1; }

if [[ ! -x "$VENV/bin/python" ]]; then
  python3 -m venv "$VENV"
  "$VENV/bin/pip" install --quiet --disable-pip-version-check "xmlschema==$XMLSCHEMA_VERSION"
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
fixture="$tmp/saft-fixture.xml"
(cd "$AGENT" && SAFT_XSD_FIXTURE_OUT="$fixture" go test ./internal/fiscal/saft/ -run '^TestBuildLineAmountsNetAndSided$' -count=1 >/dev/null)
[[ -s "$fixture" ]] || { echo "fiscal-saft-xsd-check: fixture not written" >&2; exit 1; }

# shellcheck disable=SC2086
"$VENV/bin/python" -I - "$XSD" "$fixture" ${SAFT_XSD_CHECK_FILES:-} <<'PY'
import sys
import xmlschema

schema = xmlschema.XMLSchema11(sys.argv[1])
failed = False
for path in sys.argv[2:]:
    errors = list(schema.iter_errors(path))
    if not errors:
        print(f"SAF-T XSD OK: {path}")
        continue
    failed = True
    print(f"SAF-T XSD FAIL: {path} ({len(errors)} errors)", file=sys.stderr)
    for e in errors[:20]:
        print(f"  {e.path}: {e.reason}", file=sys.stderr)
sys.exit(1 if failed else 0)
PY
