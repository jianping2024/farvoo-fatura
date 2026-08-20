package main

import "testing"

func TestWinspoolJobStatusIsProblem(t *testing.T) {
	if winspoolJobStatusIsProblem(0) {
		t.Fatal("zero status should be OK")
	}
	if winspoolJobStatusIsProblem(0x20) {
		t.Fatal("job offline alone must not fail verify")
	}
	if !winspoolJobStatusIsProblem(jobStatusError) {
		t.Fatal("job error should fail verify")
	}
	if winspoolJobStatusIsProblem(0x200) {
		t.Fatal("job blocked alone must not fail verify")
	}
}
