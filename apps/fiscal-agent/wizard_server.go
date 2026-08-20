package main

import (
	"context"
	"net/http"
	"sync"
	"time"
)

var (
	wizardServersMu sync.Mutex
	wizardServers   []*http.Server
)

func trackWizardServer(srv *http.Server) {
	if srv == nil {
		return
	}
	wizardServersMu.Lock()
	wizardServers = append(wizardServers, srv)
	wizardServersMu.Unlock()
}

func untrackWizardServer(srv *http.Server) {
	if srv == nil {
		return
	}
	wizardServersMu.Lock()
	defer wizardServersMu.Unlock()
	for i, s := range wizardServers {
		if s == srv {
			wizardServers = append(wizardServers[:i], wizardServers[i+1:]...)
			return
		}
	}
}

// shutdownHTTPServer stops accepting and closes the listener (does not wait on long handlers).
func shutdownHTTPServer(srv *http.Server, timeout time.Duration) {
	if srv == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	_ = srv.Shutdown(ctx)
	cancel()
	_ = srv.Close()
}

// shutdownAllWizardServers stops local pair/configure/setup HTTP listeners (tray exit).
func shutdownAllWizardServers() {
	wizardServersMu.Lock()
	list := append([]*http.Server(nil), wizardServers...)
	wizardServersMu.Unlock()
	for _, srv := range list {
		shutdownHTTPServer(srv, 2*time.Second)
	}
}

func waitLocalWizard(ctx context.Context, srv *http.Server, done <-chan error) error {
	trackWizardServer(srv)
	defer untrackWizardServer(srv)
	select {
	case <-ctx.Done():
		shutdownHTTPServer(srv, 3*time.Second)
		return ctx.Err()
	case err := <-done:
		shutdownHTTPServer(srv, 3*time.Second)
		return err
	}
}
