#!/usr/bin/env bash
# Poll until a GitHub Release tag has expected fiscal-agent assets (or timeout).
# Usage: ./scripts/wait-for-github-release.sh fiscal-agent-v0.4.44
# Env: GITHUB_REPOSITORY (default: jianping2024/farvoo-fatura), GH_TOKEN optional for private repos.

set -euo pipefail

TAG="${1:?Usage: $0 fiscal-agent-vX.Y.Z}"
REPO="${GITHUB_REPOSITORY:-jianping2024/farvoo-fatura}"
MAX="${MAX_ATTEMPTS:-36}"
SLEEP="${SLEEP_SEC:-10}"

auth=()
if [[ -n "${GH_TOKEN:-}" ]]; then
  auth=(-H "Authorization: Bearer $GH_TOKEN")
fi

required=(
  FarvooFiscalAgent-Setup-amd64.exe
  FarvooFiscalAgent-windows-amd64.zip
  SHA256SUMS
)

echo "Waiting for release $TAG on $REPO ..."

for ((i = 1; i <= MAX; i++)); do
  json=$(curl -sS "${auth[@]}" "https://api.github.com/repos/$REPO/releases/tags/$TAG" || true)
  if echo "$json" | grep -q '"tag_name"'; then
    mapfile -t assets < <(echo "$json" | python3 -c "import sys,json; d=json.load(sys.stdin); print('\n'.join(a['name'] for a in d.get('assets',[])))")
    ok=1
    for r in "${required[@]}"; do
      found=0
      for a in "${assets[@]}"; do
        [[ "$a" == "$r" ]] && found=1 && break
      done
      if [[ $found -eq 0 ]]; then ok=0; break; fi
    done
    if [[ $ok -eq 1 ]]; then
      echo "OK: all assets present for $TAG"
      echo "https://github.com/$REPO/releases/tag/$TAG"
      exit 0
    fi
    echo "[$i/$MAX] Release exists but assets incomplete: ${assets[*]:-(none yet)}"
  else
    echo "[$i/$MAX] Release not found yet..."
  fi
  sleep "$SLEEP"
done

echo "Timeout: release $TAG not ready with required assets." >&2
exit 1
