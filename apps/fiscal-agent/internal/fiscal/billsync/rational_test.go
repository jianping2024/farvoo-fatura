package billsync_test

import (
	"testing"

	"farvoo-fiscal-agent/internal/fiscal/billsync"
)

func TestParseQtyString(t *testing.T) {
	cases := []struct {
		in      string
		wantNum int64
		wantDen int64
		ok      bool
	}{
		{"", 1, 1, true},
		{"2", 2, 1, true},
		{"0", 0, 1, true},
		{"1/3", 1, 3, true},
		{"1/2", 1, 2, true},
		{"2 1/3", 7, 3, true},
		{"0.33", 33, 100, true},
		{"0.01", 1, 100, true},
		{"1.00", 1, 1, true},
		{"2.50", 5, 2, true},
		{"3.00", 3, 1, true},
		{"-1", 0, 0, false},
		{"-0.5", 0, 0, false},
		{"abc", 0, 0, false},
		{"1 2", 0, 0, false},
		{"1/", 0, 0, false},
	}
	for _, tc := range cases {
		got, err := billsync.ParseQtyString(tc.in)
		if tc.ok {
			if err != nil {
				t.Fatalf("ParseQtyString(%q): %v", tc.in, err)
			}
			n := billsync.NormalizeRational(got)
			if n.Num != tc.wantNum || n.Den != tc.wantDen {
				t.Fatalf("ParseQtyString(%q)=%d/%d want %d/%d", tc.in, n.Num, n.Den, tc.wantNum, tc.wantDen)
			}
			continue
		}
		if err == nil {
			t.Fatalf("ParseQtyString(%q) want error, got %d/%d", tc.in, got.Num, got.Den)
		}
	}
}
