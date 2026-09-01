#!/usr/bin/env bash
# Shared helpers for fiscal-agent release validation and tagging.
set -euo pipefail

FISCAL_AGENT_DIR="apps/fiscal-agent"

fiscal_agent_latest_tag() {
  git tag -l 'fiscal-agent-v*' --sort=-v:refname 2>/dev/null | head -1 || true
}

# True when non-metadata files under apps/fiscal-agent differ (or exist on first release).
fiscal_agent_code_changed() {
  local base_tag="${1:-}"
  local treeish="$2"

  if [[ -n "$base_tag" ]]; then
    ! git diff --quiet "$base_tag" "$treeish" -- "$FISCAL_AGENT_DIR" \
      ':(exclude)apps/fiscal-agent/VERSION' \
      ':(exclude)apps/fiscal-agent/RELEASE_NOTES.md'
    return
  fi

  local path
  while IFS= read -r path; do
    [[ -z "$path" ]] && continue
    case "$path" in
      apps/fiscal-agent/VERSION | apps/fiscal-agent/RELEASE_NOTES.md) continue ;;
      *) return 0 ;;
    esac
  done < <(git ls-tree -r --name-only "$treeish" -- "$FISCAL_AGENT_DIR" 2>/dev/null || true)
  return 1
}

fiscal_agent_version_from_tree() {
  local treeish="$1"
  local ver
  ver="$(git show "${treeish}:${FISCAL_AGENT_DIR}/VERSION" 2>/dev/null | tr -d '\r\n' || true)"
  if [[ -n "$ver" ]]; then
    echo "$ver"
    return 0
  fi
  tr -d '\r\n' < "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/${FISCAL_AGENT_DIR}/VERSION"
}

fiscal_agent_tree_for_mode() {
  local mode="${1:-}"
  if [[ "$mode" == "--staged" ]]; then
    git write-tree
  else
    git rev-parse 'HEAD^{tree}'
  fi
}
