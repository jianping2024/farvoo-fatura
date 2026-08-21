package worker

// Package worker: fiscal local_print_jobs drain.
// Physical out on Agent = injected PrintBytesFn → main parsePrinterTarget + printToTarget (tcp|winspool).
// Do NOT add a second TCP/WinSpool implementation here.
