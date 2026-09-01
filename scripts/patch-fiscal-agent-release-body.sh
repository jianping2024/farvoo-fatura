#!/usr/bin/env bash
# Patch an existing GitHub Release body from RELEASE_NOTES.md.
# Normally CI runs this via sync-fiscal-agent-release-notes.yml (no local token needed).
# Local usage: GH_TOKEN=... ./scripts/patch-fiscal-agent-release-body.sh [VERSION]

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="${1:-$(tr -d '\r\n' < "$ROOT/apps/fiscal-agent/VERSION")}"
TAG="fiscal-agent-v$VERSION"
REPO="${GITHUB_REPOSITORY:-jianping2024/farvoo-fatura}"
TOKEN="${GH_TOKEN:-${GITHUB_TOKEN:-}}"

if [[ -z "$TOKEN" ]]; then
  echo "Set GH_TOKEN or GITHUB_TOKEN to patch GitHub Release." >&2
  exit 1
fi

BODY_FILE=$(mktemp)
PAYLOAD_FILE=$(mktemp)
trap 'rm -f "$BODY_FILE" "$PAYLOAD_FILE"' EXIT

"$ROOT/scripts/fiscal-agent-release-body.sh" "$VERSION" > "$BODY_FILE"

RELEASE_JSON=$(curl -sS -H "Authorization: Bearer $TOKEN" \
  "https://api.github.com/repos/$REPO/releases/tags/$TAG")

RELEASE_ID=$(python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('id') or '')" <<< "$RELEASE_JSON")
if [[ -z "$RELEASE_ID" ]]; then
  echo "Release not found for tag $TAG" >&2
  exit 1
fi

python3 - "$BODY_FILE" "$PAYLOAD_FILE" <<'PY'
import json, pathlib, sys
body = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8")
pathlib.Path(sys.argv[2]).write_text(json.dumps({"body": body}), encoding="utf-8")
PY

curl -sS -X PATCH \
  -H "Authorization: Bearer $TOKEN" \
  -H "Accept: application/vnd.github+json" \
  "https://api.github.com/repos/$REPO/releases/$RELEASE_ID" \
  -d @"$PAYLOAD_FILE" >/dev/null

echo "Patched release body for $TAG"
echo "https://github.com/$REPO/releases/tag/$TAG"
