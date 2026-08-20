package main

import (
	"errors"
	"strings"
	"testing"
)

func TestPairClaimUserError_invalidCode(t *testing.T) {
	msg := pairClaimUserError(errors.New(`claim 404 Not Found: {"error":"invalid_or_expired_code"}`))
	if !strings.Contains(msg, "配对码无效或已过期") {
		t.Fatalf("unexpected message: %q", msg)
	}
}

func TestPairClaimUserError_rateLimited(t *testing.T) {
	msg := pairClaimUserError(errors.New(`claim 429 Too Many Requests: {"error":"rate_limited"}`))
	if !strings.Contains(msg, "请求过于频繁") {
		t.Fatalf("unexpected message: %q", msg)
	}
}
