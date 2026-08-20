#!/usr/bin/env bash
# Static checks for single-instance guard (main agent only).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

grep -q 'guardMainAgentSingleInstance' "$ROOT/main.go" || {
  echo "FAIL: main.go must call guardMainAgentSingleInstance()"
  exit 1
}

grep -q 'func guardMainAgentSingleInstance' "$ROOT/single_instance_common.go" || exit 1
grep -q 'Global\\\\MesaPrintAgent-SingleInstance' "$ROOT/single_instance_windows.go" || exit 1
grep -q 'errorAlreadyExists' "$ROOT/single_instance_windows.go" || exit 1
grep -q 'OpenMutexW' "$ROOT/single_instance_windows.go" || exit 1
grep -q 'exitAlreadyRunning' "$ROOT/single_instance_windows.go" || exit 1

python3 <<PY
from pathlib import Path
import subprocess
subprocess.check_call(["go", "test", "-run", "TestIsMainAgentInvocation", "."], cwd="$ROOT")
PY

echo "OK: single-instance guard wired (main entry + Windows mutex + tests)"
