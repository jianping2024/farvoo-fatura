package main

import "testing"

func TestIsMainAgentInvocation(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{[]string{"MesaPrintAgent.exe"}, true},
		{[]string{"MesaPrintAgent.exe", "-console"}, true},
		{[]string{"MesaPrintAgent.exe", "run"}, true},
		{[]string{"MesaPrintAgent.exe", "configure"}, false},
		{[]string{"MesaPrintAgent.exe", "pair"}, false},
		{[]string{"MesaPrintAgent.exe", "discover"}, false},
		{[]string{"MesaPrintAgent.exe", "version"}, false},
	}
	for _, tc := range cases {
		if got := isMainAgentInvocation(tc.args); got != tc.want {
			t.Fatalf("%v: got %v want %v", tc.args, got, tc.want)
		}
	}
}
