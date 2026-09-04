package main

import (
	"testing"
	"time"
)

func TestIsMainAgentInvocation(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{[]string{"FarvooFiscalAgent.exe"}, true},
		{[]string{"FarvooFiscalAgent.exe", "-console"}, true},
		{[]string{"FarvooFiscalAgent.exe", "run"}, true},
		{[]string{"FarvooFiscalAgent.exe", "configure"}, false},
		{[]string{"FarvooFiscalAgent.exe", "pair"}, false},
		{[]string{"FarvooFiscalAgent.exe", "discover"}, false},
		{[]string{"FarvooFiscalAgent.exe", "version"}, false},
		{[]string{"FarvooFiscalAgent.exe", "fiscal"}, false},
		{[]string{"FarvooFiscalAgent.exe", "--restart-wait"}, false},
	}
	for _, tc := range cases {
		if got := isMainAgentInvocation(tc.args); got != tc.want {
			t.Fatalf("%v: got %v want %v", tc.args, got, tc.want)
		}
	}
}

func TestWaitAcquireAgentSingleInstancePoll_successAfterRetries(t *testing.T) {
	n := 0
	ok := waitAcquireAgentSingleInstancePoll(time.Second, func() bool {
		n++
		return n >= 3
	}, func(time.Duration) {})
	if !ok {
		t.Fatal("expected acquire success after retries")
	}
	if n != 3 {
		t.Fatalf("acquire calls=%d want 3", n)
	}
}

func TestWaitAcquireAgentSingleInstancePoll_timeout(t *testing.T) {
	n := 0
	ok := waitAcquireAgentSingleInstancePoll(2*agentRestartAcquirePoll, func() bool {
		n++
		return false
	}, func(d time.Duration) { time.Sleep(d) })
	if ok {
		t.Fatal("expected timeout failure")
	}
	if n < 2 {
		t.Fatalf("expected multiple acquire attempts, got %d", n)
	}
}
