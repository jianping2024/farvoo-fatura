#!/usr/bin/env bash
# Run before tagging fiscal-agent-v* — same checks as fiscal-agent-ci.yml / release test-linux job.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
AGENT="$ROOT/apps/fiscal-agent"
VERSION="$(tr -d '\r\n' < "$AGENT/VERSION")"

agent_go_works() {
  command -v go >/dev/null 2>&1 && (cd "$AGENT" && go version >/dev/null 2>&1)
}

docker_available() {
  command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1
}

go_version_from_mod() {
  sed -n 's/^go //p' "$AGENT/go.mod" | head -1
}

run_go_checks() {
  if agent_go_works; then
    echo "Using local Go..."
    (cd "$AGENT" && go test ./... && go vet ./... && GOOS=windows GOARCH=amd64 go build -o /dev/null .)
    return
  fi
  if docker_available; then
    local gv
    gv="$(go_version_from_mod)"
    echo "Using Docker (golang:${gv}-bookworm)..."
    docker run --rm \
      -v "$AGENT:/app" \
      -w /app \
      "golang:${gv}-bookworm" \
      sh -ce 'go test ./... && go vet ./... && GOOS=windows GOARCH=amd64 go build -o /dev/null .'
    return
  fi
  echo "Need Go (see apps/fiscal-agent/go.mod) or Docker to run fiscal-agent checks." >&2
  exit 1
}

echo "fiscal-agent VERSION=$VERSION"
run_go_checks
echo "OK — safe to tag: git tag fiscal-agent-v$VERSION && git push origin fiscal-agent-v$VERSION"
echo "Then wait for Actions → fiscal-agent-release (green) before telling users to download."
echo "Never build Setup/zip locally on macOS as a substitute for GitHub Release."
