package main

// Win32 JOB_STATUS_* (subset) for post-submit verify on Windows.
const (
	jobStatusError    = 0x00000002
	jobStatusComplete = 0x00001000
)

func winspoolJobStatusIsProblem(flags uint32) bool {
	// JOB_STATUS_BLOCKED is unreliable on USB POS (same class as PRINTER_STATUS_OFFLINE).
	return flags&jobStatusError != 0
}
