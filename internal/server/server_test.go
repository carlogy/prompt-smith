package server

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/carlogy/prompt-smith/internal/registry"
)

// TestServe_SmokeTest binds a real (OS-assigned) loopback port, makes
// a real HTTP round-trip against it, then cancels ctx and confirms
// Serve shuts down cleanly. NoBrowser is always true in tests - this
// must never actually launch a browser in CI.
func TestServe_SmokeTest(t *testing.T) {
	reg := &registry.Registry{
		Categories: []string{"debugging"},
		Skills:     []registry.Skill{{ID: "diagnose", Category: "debugging", Body: "Build a feedback loop first."}},
		Targets:    map[string]registry.TargetConfig{"generic": {ID: "generic", SkillMode: "inline"}},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	urlCh := make(chan string, 1)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- Serve(ctx, reg, Options{
			Port:      0, // OS-assigned free port
			NoBrowser: true,
			Logger:    logger,
			Stdout:    io.Discard,
			Ready:     func(url string) { urlCh <- url },
		})
	}()

	var url string
	select {
	case url = <-urlCh:
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not become ready within the deadline")
	}

	resp, err := http.Get(url + "/api/registry")
	if err != nil {
		t.Fatalf("GET %s/api/registry error = %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	cancel() // trigger graceful shutdown

	select {
	case err := <-serveErr:
		if err != nil {
			t.Errorf("Serve() error = %v, want nil on a clean shutdown", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return within the deadline after cancel")
	}
}

func TestServe_ListenErrorIsReturned(t *testing.T) {
	// Bind the same fixed port twice: the second Serve call must fail
	// with a listen error rather than hanging or panicking.
	reg := &registry.Registry{Targets: map[string]registry.TargetConfig{}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	urlCh := make(chan string, 1)
	serveErr1 := make(chan error, 1)
	go func() {
		serveErr1 <- Serve(ctx1, reg, Options{Port: 0, NoBrowser: true, Logger: logger, Stdout: io.Discard, Ready: func(url string) { urlCh <- url }})
	}()

	var url string
	select {
	case url = <-urlCh:
	case <-time.After(5 * time.Second):
		t.Fatal("first Serve did not become ready within the deadline")
	}

	_, portStr, err := net.SplitHostPort(strings.TrimPrefix(url, "http://"))
	if err != nil {
		t.Fatalf("SplitHostPort(%q) error = %v", url, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("Atoi(%q) error = %v", portStr, err)
	}

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	if err := Serve(ctx2, reg, Options{Port: port, NoBrowser: true, Logger: logger, Stdout: io.Discard}); err == nil {
		t.Error("second Serve() on an already-bound port: error = nil, want a listen error")
	}

	cancel1()
	select {
	case <-serveErr1:
	case <-time.After(5 * time.Second):
		t.Fatal("first Serve did not return within the deadline after cancel")
	}
}
