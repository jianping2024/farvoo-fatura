#!/usr/bin/env bash
# Create and push fiscal-agent-v{VERSION} at HEAD (run after validate + push to main).
#
# Env:
#   PUSH_SKIP_FISCAL_AGENT_TAG=1 — skip
set -euo pipefail

if [[ "${PUSH_SKIP_FISCAL_AGENT_TAG:-}" == "1" ]]; then
  echo "Fiscal agent auto-tag skipped (PUSH_SKIP_FISCAL_AGENT_TAG=1)."
  exit 0
fi

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=scripts/fiscal-agent-release-lib.sh
source "$ROOT/scripts/fiscal-agent-release-lib.sh"

VERSION_FILE="$ROOT/$FISCAL_AGENT_DIR/VERSION"

if [[ ! -f "$VERSION_FILE" ]]; then
  exit 0
fi

VER="$(tr -d '\r\n' < "$VERSION_FILE")"
TAG="fiscal-agent-v${VER}"

git fetch origin --tags --quiet 2>/dev/null || true

latest_tag="$(fiscal_agent_latest_tag)"
tree="$(fiscal_agent_tree_for_mode "")"

if [[ -n "$latest_tag" ]] && ! fiscal_agent_code_changed "$latest_tag" "$tree"; then
  echo "Fiscal agent code unchanged since ${latest_tag}; no new tag."
  exit 0
fi

if git rev-parse "refs/tags/${TAG}" >/dev/null 2>&1; then
  tag_commit="$(git rev-parse "refs/tags/${TAG}^{commit}")"
  head_commit="$(git rev-parse HEAD)"
  if [[ "$tag_commit" == "$head_commit" ]]; then
    echo "Tag ${TAG} already points at this commit."
    exit 0
  fi
  echo "ERROR: Tag ${TAG} exists but does not point at HEAD; bump VERSION first." >&2
  exit 1
fi

if git ls-remote --exit-code origin "refs/tags/${TAG}" >/dev/null 2>&1; then
  remote_commit="$(git ls-remote origin "refs/tags/${TAG}" | awk '{print $1}')"
  head_commit="$(git rev-parse HEAD)"
  if [[ "$remote_commit" == "$head_commit" ]]; then
    echo "Tag ${TAG} already on origin for this commit."
    exit 0
  fi
  echo "ERROR: Tag ${TAG} already exists on origin for another commit." >&2
  exit 1
fi

echo "Running fiscal-agent checks before tagging ${TAG}..."
"$ROOT/scripts/check-fiscal-agent.sh"

git tag "$TAG"
echo "Pushing tag ${TAG}..."
git push origin "$TAG" 2>&1 | grep -v 'could not write config file' || true

echo ""
echo "Tagged ${TAG}. Wait for GitHub Actions → fiscal-agent-release (green)."
echo "Then confirm Release has FarvooFiscalAgent-Setup-amd64.exe."
echo "Optional: ./scripts/wait-for-github-release.sh ${TAG}"
