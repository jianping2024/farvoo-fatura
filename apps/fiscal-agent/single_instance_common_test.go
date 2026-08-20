package main

import "testing"

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
	}
	for _, tc := range cases {
		if got := isMainAgentInvocation(tc.args); got != tc.want {
			t.Fatalf("%v: got %v want %v", tc.args, got, tc.want)
		}
	}
}
