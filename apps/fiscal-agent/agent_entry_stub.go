//go:build !windows

package main

import (
	"context"
	"log"
)

func runAgent(args []string) {
	sess, _, err := initAgentSession(context.Background(), args)
	if err != nil {
		log.Fatal(err)
	}
	runNotificationLoop(context.Background(), sess, &agentStatus{})
}
