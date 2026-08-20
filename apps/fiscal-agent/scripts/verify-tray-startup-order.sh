#!/usr/bin/env bash
# Evidence check: tray must start before blocking init; local HTTP before unpaired wait (Windows agent_entry_windows.go).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FILE="$ROOT/agent_entry_windows.go"
BOOTSTRAP="$ROOT/agent_bootstrap.go"
if ! grep -q 'runAgentTrayFirst' "$FILE"; then
  echo "FAIL: missing runAgentTrayFirst"
  exit 1
fi
if ! grep -q 'systray.Run' "$FILE"; then
  echo "FAIL: missing systray.Run"
  exit 1
fi
python3 <<PY
from pathlib import Path
text = Path("$FILE").read_text()
# Tray path: after console early-return, runAgent must go straight to runAgentTrayFirst.
ra = text[text.index("func runAgent(") : text.index("func runAgentTrayFirst")]
tray_tail = ra.split("runAgentTrayFirst", 1)[0]
# Only the block after the console branch should be checked.
if "if agentArgsWantConsole" in tray_tail:
    tray_tail = tray_tail.split("if agentArgsWantConsole", 1)[1]
    tray_tail = tray_tail.split("return", 1)[-1]  # after console return
if "initAgentSession" in tray_tail:
    print("FAIL: tray path still calls initAgentSession before runAgentTrayFirst")
    raise SystemExit(1)
block = text[text.index("func runAgentTrayFirst") : text.index("func onTrayReady")]
if "go func()" not in block or block.find("initAgentSession") < block.find("go func()"):
    print("FAIL: initAgentSession must run inside go func in runAgentTrayFirst")
    raise SystemExit(1)
if "systray.Run" not in block:
    print("FAIL: missing systray.Run in runAgentTrayFirst")
    raise SystemExit(1)
if block.count("startTrayLocalHTTP") != 1:
    print("FAIL: expected exactly one startTrayLocalHTTP in runAgentTrayFirst")
    raise SystemExit(1)
if block.find("startTrayLocalHTTP") > block.find("initAgentSession"):
    print("FAIL: startTrayLocalHTTP must run before initAgentSession (unpaired /pair on 17892)")
    raise SystemExit(1)
boot = Path("$BOOTSTRAP").read_text()
if "waitForAgentPairing" not in boot:
    print("FAIL: unpaired bootstrap must call waitForAgentPairing")
    raise SystemExit(1)
if "runPairingWizard(" in boot:
    print("FAIL: agent_bootstrap must not call runPairingWizard directly (use waitForAgentPairing)")
    raise SystemExit(1)
print("OK: tray HTTP before init; unpaired wait prefers tray 17892")
PY
if ! grep -q 'acquireAgentSingleInstance' "$ROOT/single_instance_windows.go"; then
  echo "FAIL: missing single-instance mutex"
  exit 1
fi
echo "OK: tray startup order + single instance present"
