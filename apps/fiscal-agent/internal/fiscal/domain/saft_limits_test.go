package domain

import (
	"strings"
	"testing"
)

func TestValidateStatusReason(t *testing.T) {
	if _, err := ValidateStatusReason("  "); err != ErrReasonRequired {
		t.Fatalf("blank: %v", err)
	}
	ok := strings.Repeat("é", SAFTMaxReason) // 50 characters, 100 bytes
	if r, err := ValidateStatusReason(" " + ok + " "); err != nil || r != ok {
		t.Fatalf("50 chars: %q %v", r, err)
	}
	if _, err := ValidateStatusReason(ok + "x"); err != ErrReasonTooLong {
		t.Fatalf("51 chars: %v", err)
	}
}
