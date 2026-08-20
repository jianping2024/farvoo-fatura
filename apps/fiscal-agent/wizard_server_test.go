package main

import (
	"net"
	"net/http"
	"testing"
	"time"
)

func TestShutdownHTTPServerNil(t *testing.T) {
	shutdownHTTPServer(nil, time.Second)
}

func TestShutdownAllWizardServersEmpty(t *testing.T) {
	shutdownAllWizardServers()
}

func TestShutdownHTTPServerStopsListener(t *testing.T) {
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv.Addr = ln.Addr().String()
	go func() { _ = srv.Serve(ln) }()

	shutdownHTTPServer(srv, 2*time.Second)

	req, err := http.NewRequest(http.MethodGet, "http://"+srv.Addr+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = http.DefaultClient.Do(req)
	if err == nil {
		t.Fatal("expected connection error after shutdown")
	}
}