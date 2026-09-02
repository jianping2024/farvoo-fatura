package fiscalipc

import "testing"

func TestAgentMutexNameStable(t *testing.T) {
	const want = `Global\FarvooFiscalAgent-SingleInstance-v1`
	if AgentMutexName != want {
		t.Fatalf("AgentMutexName = %q want %q", AgentMutexName, want)
	}
}

func TestCommandOpenFiscalStable(t *testing.T) {
	if CommandOpenFiscal != "open-fiscal" {
		t.Fatalf("CommandOpenFiscal = %q", CommandOpenFiscal)
	}
}
