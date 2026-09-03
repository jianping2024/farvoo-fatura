package fiscalclient

import "testing"

func TestClientMutexNameStable(t *testing.T) {
	const want = `Global\FarvooFiscalClient-SingleInstance-v1`
	if ClientMutexName != want {
		t.Fatalf("ClientMutexName = %q want %q", ClientMutexName, want)
	}
}
