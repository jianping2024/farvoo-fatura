#!/usr/bin/env bash
# Decide whether a fiscal-agent release is required and whether the tree is ready.
# Call before push/commit when agent code changed; shared by push-to-main and tag-fiscal-agent.sh.
#
# Usage:
#   ./scripts/validate-fiscal-agent-release.sh          # validate HEAD
#   ./scripts/validate-fiscal-agent-release.sh --staged # validate index (after git add, before commit)
#
# Env:
#   PUSH_SKIP_FISCAL_AGENT_TAG=1 — skip release requirements (exit 0)
set -euo pipefail

if [[ "${PUSH_SKIP_FISCAL_AGENT_TAG:-}" == "1" ]]; then
  echo "Fiscal agent release validation skipped (PUSH_SKIP_FISCAL_AGENT_TAG=1)."
  exit 0
fi

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=scripts/fiscal-agent-release-lib.sh
source "$ROOT/scripts/fiscal-agent-release-lib.sh"

MODE="${1:-}"
VERSION_FILE="$ROOT/$FISCAL_AGENT_DIR/VERSION"

if [[ ! -f "$VERSION_FILE" ]]; then
  exit 0
fi

git fetch origin --tags --quiet 2>/dev/null || true

latest_tag="$(fiscal_agent_latest_tag)"
tree="$(fiscal_agent_tree_for_mode "$MODE")"

if fiscal_agent_code_changed "$latest_tag" "$tree"; then
  if [[ -n "$latest_tag" ]]; then
    echo "Fiscal agent code changed since ${latest_tag}; release bundle required."
  else
    echo "No prior fiscal-agent-v* tag; first release bundle required."
  fi
else
  echo "Fiscal agent code unchanged since ${latest_tag:-<none>}; no release required."
  exit 0
fi

VER="$(fiscal_agent_version_from_tree "$tree")"
TAG="fiscal-agent-v${VER}"

if git rev-parse "refs/tags/${TAG}" >/dev/null 2>&1; then
  echo "ERROR: Fiscal agent code changed but VERSION is still ${VER} (tag ${TAG} already exists)." >&2
  echo "Bump ${VERSION_FILE} and add ## ${VER} to apps/fiscal-agent/RELEASE_NOTES.md in the same push." >&2
  exit 1
fi

if git ls-remote --exit-code origin "refs/tags/${TAG}" >/dev/null 2>&1; then
  echo "ERROR: Fiscal agent code changed but tag ${TAG} already exists on origin." >&2
  echo "Bump ${VERSION_FILE} and add ## ${VER} to apps/fiscal-agent/RELEASE_NOTES.md in the same push." >&2
  exit 1
fi

if ! bash "$ROOT/scripts/fiscal-agent-release-body.sh" "$VER" >/dev/null; then
  echo "ERROR: Add '## ${VER}' to apps/fiscal-agent/RELEASE_NOTES.md before pushing." >&2
  exit 1
fi

echo "Fiscal agent release ready: will tag ${TAG} after push."
