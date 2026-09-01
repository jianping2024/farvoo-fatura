#!/usr/bin/env bash
# Bump-safe tag helper: run tests, ensure VERSION matches tag, push tag.
#
# Usage:
#   ./scripts/tag-fiscal-agent.sh              # tag fiscal-agent-v$(cat VERSION)
#   ./scripts/tag-fiscal-agent.sh 0.4.45       # set VERSION + tag (commits VERSION only)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION_FILE="$ROOT/apps/fiscal-agent/VERSION"
TAG_VER="${1:-}"

if [[ -n "$TAG_VER" ]]; then
  echo "$TAG_VER" > "$VERSION_FILE"
  git add "$VERSION_FILE"
  git commit -m "Bump fiscal-agent to v$TAG_VER."
fi

"$ROOT/scripts/validate-fiscal-agent-release.sh"
"$ROOT/scripts/apply-fiscal-agent-tag.sh"
