package vatpercent

import "testing"

func TestNormalize(t *testing.T) {
	cases := []struct {
		in, want string
		ok       bool
	}{
		{"23", "23.00", true},
		{"23.0", "23.00", true},
		{"23.00", "23.00", true},
		{" 13 ", "13.00", true},
		{"0", "0.00", true},
		{"0.00", "0.00", true},
		{"0.23", "", false},
		{"", "", false},
		{"abc", "", false},
		{"101", "", false},
	}
	for _, c := range cases {
		got, err := Normalize(c.in)
		if c.ok {
			if err != nil || got != c.want {
				t.Fatalf("Normalize(%q)=%q,%v want %q", c.in, got, err, c.want)
			}
		} else if err == nil {
			t.Fatalf("Normalize(%q) expected error, got %q", c.in, got)
		}
	}
}

func TestFromDecimal(t *testing.T) {
	cases := []struct {
		in, want string
		ok       bool
	}{
		{"0.23", "23.00", true},
		{"0.13", "13.00", true},
		{"0.06", "6.00", true},
		{"0", "0.00", true},
		{"23", "", false},
		{"", "", false},
		{"x", "", false},
	}
	for _, c := range cases {
		got, err := FromDecimal(c.in)
		if c.ok {
			if err != nil || got != c.want {
				t.Fatalf("FromDecimal(%q)=%q,%v want %q", c.in, got, err, c.want)
			}
		} else if err == nil {
			t.Fatalf("FromDecimal(%q) expected error, got %q", c.in, got)
		}
	}
}
