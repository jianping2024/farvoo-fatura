// Package worker claims local_print_jobs and drives physical output.
//
// Separate from cloud JobProcessor (main package): cloud queue = business receipts;
// fiscal worker = SQLite local_print_jobs only.
package worker
