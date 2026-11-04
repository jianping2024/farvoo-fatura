package fiscalwebview

import "testing"

func TestUseAgentFiscalWindowIcon(t *testing.T) {
	prev := windowIconID
	t.Cleanup(func() { windowIconID = prev })

	windowIconID = ClientFiscalIconID
	if WindowIconID() != ClientFiscalIconID {
		t.Fatalf("default WindowIconID = %d, want %d", WindowIconID(), ClientFiscalIconID)
	}
	UseAgentFiscalWindowIcon()
	if WindowIconID() != AgentFiscalIconID {
		t.Fatalf("after UseAgentFiscalWindowIcon = %d, want %d", WindowIconID(), AgentFiscalIconID)
	}
	if AgentShellIconID != 1 || AgentFiscalIconID != 2 || ClientFiscalIconID != 1 {
		t.Fatalf("resource id constants drifted: shell=%d agentFiscal=%d client=%d", AgentShellIconID, AgentFiscalIconID, ClientFiscalIconID)
	}
}
