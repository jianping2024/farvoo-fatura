//go:build !windows

package main

import (
	"context"
	"log"
)

func runAgent(args []string) {
	if wantsFiscalStandalone(args) {
		runFiscalStandalone(args)
		return
	}
	sess, _, err := initAgentSession(context.Background(), args)
	if err != nil {
		log.Fatal(err)
	}
	ensureFiscalStarted(context.Background(), sess)
	runNotificationLoop(context.Background(), sess, &agentStatus{})
}
