package goxpress

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

// TestServeDrainsInFlight verifies that a request already inside a handler when
// shutdown is triggered runs to completion rather than being cut off.
func TestServeDrainsInFlight(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})

	r := New()
	r.Get("/slow", func(c *Context) error {
		close(started)
		<-release
		return c.String(http.StatusOK, "done")
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := r.Server("")
	ctx, cancel := context.WithCancel(context.Background())

	serveErr := make(chan error, 1)
	go func() { serveErr <- r.serve(ctx, srv, ln) }()

	type result struct {
		body string
		err  error
	}
	resp := make(chan result, 1)
	go func() {
		res, err := http.Get("http://" + ln.Addr().String() + "/slow")
		if err != nil {
			resp <- result{err: err}
			return
		}
		b, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		resp <- result{body: string(b)}
	}()

	<-started // request is now in-flight inside the handler
	cancel()  // simulate SIGINT/SIGTERM
	close(release)

	got := <-resp
	if got.err != nil {
		t.Fatalf("in-flight request failed: %v", got.err)
	}
	if got.body != "done" {
		t.Errorf("body = %q, want %q", got.body, "done")
	}
	if err := <-serveErr; err != nil {
		t.Errorf("serve returned %v, want nil", err)
	}
}

// TestServeRejectsNewAfterShutdown verifies new connections are refused once
// shutdown has completed.
func TestServeRejectsNewAfterShutdown(t *testing.T) {
	r := New()
	r.Get("/", func(c *Context) error { return c.String(http.StatusOK, "ok") })

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ctx, cancel := context.WithCancel(context.Background())
	serveErr := make(chan error, 1)
	go func() { serveErr <- r.serve(ctx, r.Server(""), ln) }()

	// Confirm the server is up.
	if res, err := http.Get("http://" + addr + "/"); err != nil {
		t.Fatalf("pre-shutdown request failed: %v", err)
	} else {
		_ = res.Body.Close()
	}

	cancel()
	if err := <-serveErr; err != nil {
		t.Fatalf("serve returned %v", err)
	}

	client := &http.Client{Timeout: 500 * time.Millisecond}
	res, err := client.Get("http://" + addr + "/")
	if err == nil {
		_ = res.Body.Close()
		t.Error("expected request to fail after shutdown")
	}
}

func TestServerHelper(t *testing.T) {
	r := New()
	r.ShutdownTimeout = 3 * time.Second
	srv := r.Server(":8080")
	if srv.Addr != ":8080" {
		t.Errorf("Addr = %q, want :8080", srv.Addr)
	}
	if srv.Handler != http.Handler(r) {
		t.Error("Handler should be the router")
	}
}
