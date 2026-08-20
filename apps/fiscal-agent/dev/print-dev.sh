#!/usr/bin/env bash
# Mesa print-agent local dev helper (Docker fake printers + agent).
# Run from anywhere:  bash apps/print-agent/dev/print-dev.sh <cmd>
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
COMPOSE_FILE="$SCRIPT_DIR/docker-compose.yml"
CONFIG_FILE="$SCRIPT_DIR/config.docker.json"
MESA_API="${MESA_API:-http://host.docker.internal:3000}"

dc() {
  docker compose -f "$COMPOSE_FILE" "$@"
}

usage() {
  cat <<EOF
Mesa print dev — shortcuts for fake printers + print agent

Usage:  bash $SCRIPT_DIR/print-dev.sh <command> [args]

Commands:
  up              Build & start 3 fake printers (detached)
  down            Stop printers + agent stack
  logs            Follow all fake-printer logs (ESC/POS decode)
  logs-kitchen    Follow kitchen printer only
  agent-build     Rebuild print-agent image only
  agent           Run print agent (foreground; Ctrl+C to stop)
  pair <code>     One-time pairing (6-digit code from dashboard)
  rebuild         up --build + agent-build

Before first use:
  1. npm run dev   (Mesa on :3000, PRINT_AGENT_JWT_SECRET in .env.local)
  2. cp apps/print-agent/dev/config.example.json config.docker.json
     — set agentjwt/device_id after pair, and print_station UUIDs
  3. print-dev.sh pair 123456  then edit config.docker.json if needed

Examples:
  bash $SCRIPT_DIR/print-dev.sh up
  bash $SCRIPT_DIR/print-dev.sh logs
  bash $SCRIPT_DIR/print-dev.sh agent
EOF
}

cmd="${1:-help}"
shift || true

case "$cmd" in
  up)
    dc up --build -d
    echo "Printers: receipt :19100  kitchen :19101  bar :19102 (on host)"
    ;;
  down)
    dc --profile agent down --remove-orphans 2>/dev/null || dc down --remove-orphans
    ;;
  logs)
    dc logs -f printer-kitchen printer-receipt printer-bar
    ;;
  logs-kitchen)
    dc logs -f printer-kitchen
    ;;
  agent-build)
    dc --profile agent build print-agent
    ;;
  agent)
    if [[ ! -f "$CONFIG_FILE" ]]; then
      echo "Missing $CONFIG_FILE — copy from config.example.json" >&2
      exit 1
    fi
    echo "Building print-agent image (picks up Go + config changes)..."
    dc --profile agent build print-agent
    dc --profile agent run --rm \
      -v "$CONFIG_FILE:/data/config.json" \
      print-agent -api "$MESA_API" -config /data/config.json
    ;;
  pair)
    code="${1:-}"
    if [[ ! "$code" =~ ^[0-9]{6}$ ]]; then
      echo "Usage: print-dev.sh pair <6-digit-code>" >&2
      exit 1
    fi
    dc --profile agent run --rm \
      print-agent -api "$MESA_API" -code "$code" -config /data/config.json
    echo "Copy agentjwt & device_id from ~/.config/... or volume into $CONFIG_FILE"
    ;;
  rebuild)
    dc up --build -d
    dc --profile agent build print-agent
    echo "Done. Run: print-dev.sh agent   and   print-dev.sh logs"
    ;;
  help|-h|--help)
    usage
    ;;
  *)
    echo "Unknown command: $cmd" >&2
    usage
    exit 1
    ;;
esac
