#!/usr/bin/env bash
# Static checks for single-instance guard (main agent + fiscal + Client).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

grep -q 'guardMainAgentSingleInstance' "$ROOT/main.go" || {
  echo "FAIL: main.go must call guardMainAgentSingleInstance()"
  exit 1
}

grep -q 'func guardMainAgentSingleInstance' "$ROOT/single_instance_common.go" || exit 1
grep -q 'FarvooFiscalAgent-SingleInstance-v1' "$ROOT/internal/fiscalipc/constants.go" || {
  echo "FAIL: AgentMutexName must be FarvooFiscalAgent-SingleInstance-v1"
  exit 1
}
grep -q 'errorAlreadyExists' "$ROOT/single_instance_windows.go" || exit 1
grep -q 'OpenMutexW' "$ROOT/single_instance_windows.go" || exit 1
grep -q 'exitAlreadyRunning' "$ROOT/single_instance_windows.go" || exit 1

# Sole silent duplicate exit — no UI dialog helpers
if grep -nE 'messageBoxOK|MessageBoxW|instance_running' "$ROOT/single_instance_windows.go"; then
  echo "FAIL: exitAlreadyRunning must be silent (no dialog / instance_running)"
  exit 1
fi
grep -q 'silent exit' "$ROOT/single_instance_windows.go" || {
  echo "FAIL: exitAlreadyRunning must log silent exit"
  exit 1
}

# Fail-closed CreateMutex
if grep -A2 'if handle == 0' "$ROOT/single_instance_windows.go" | grep -q 'return true'; then
  echo "FAIL: CreateMutex failure must fail-closed (return false)"
  exit 1
fi

# Sole fiscal desktop entry — no log.Fatal on IPC
grep -q 'func runFiscalCommand' "$ROOT/fiscal_shell_windows.go" || exit 1
if grep -n 'log.Fatal' "$ROOT/fiscal_shell_windows.go"; then
  echo "FAIL: runFiscalCommand must not Fatal on IPC (silent / log only)"
  exit 1
fi
n_req=$(grep -c 'func RequestOpenFiscal' "$ROOT/internal/fiscalipc/pipe_windows.go" || true)
if [[ "$n_req" != "1" ]]; then
  echo "FAIL: want exactly 1 RequestOpenFiscal in pipe_windows.go, got $n_req"
  exit 1
fi
grep -q 'openFiscalAttempts' "$ROOT/internal/fiscalipc/pipe_windows.go" || {
  echo "FAIL: RequestOpenFiscal must retry (openFiscalAttempts)"
  exit 1
}

# Console: show/focus only — no toggle (exclude tests that assert absence)
if grep -rn 'func toggleConsoleWindow\|toggleConsoleWindow()' "$ROOT" --include='*.go' | grep -v '_test.go'; then
  echo "FAIL: toggleConsoleWindow must be removed (use showOrFocusConsoleWindow)"
  exit 1
fi
grep -q 'func showOrFocusConsoleWindow' "$ROOT/console_windows.go" || exit 1

# Client sole mutex
grep -q 'FarvooFiscalClient-SingleInstance-v1' "$ROOT/internal/fiscalclient/single_instance_windows.go" || exit 1
grep -q 'AcquireClientSingleInstance' "$ROOT/cmd/fiscal-client/main.go" || exit 1
grep -q 'FocusExistingByTitle' "$ROOT/cmd/fiscal-client/main.go" || exit 1

# Dead i18n keys removed
if grep -n 'instance_running_' "$ROOT/ui_i18n.go"; then
  echo "FAIL: instance_running_* i18n keys must be removed"
  exit 1
fi

(
  cd "$ROOT"
  go test -count=1 -run 'TestIsMainAgentInvocation' .
  go test -count=1 -run 'TestAgentMutexNameStable|TestCommandOpenFiscalStable|TestSoleSingleInstanceWritings' ./internal/fiscalipc
  go test -count=1 -run 'TestClientMutexNameStable' ./internal/fiscalclient
)

echo "OK: single-instance guard wired (silent exit + fail-closed + fiscal IPC + Client + no toggle)"
