// promptsmith - generate portable, skill-aware prompts for any LLM or agent harness.
// Copyright (C) 2026 carlogy
// SPDX-License-Identifier: AGPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/carlogy/prompt-smith/internal/prompt"
	"github.com/carlogy/prompt-smith/internal/registry"
)

// shutdownTimeout bounds how long Serve waits for in-flight requests
// to finish once ctx is done, before giving up and returning anyway.
const shutdownTimeout = 5 * time.Second

// Options configures Serve.
type Options struct {
	// Port to bind, or 0 (the default) to let the OS assign a free one.
	Port int
	// NoBrowser skips the best-effort browser auto-open.
	NoBrowser bool
	// NoHints mirrors the CLI's --no-hints flag: it suppresses this
	// server's promptlint findings (see previewData.Findings in
	// preview.go) the same way --no-hints suppresses warnLintFindings's
	// stderr output in internal/cli/generate.go. Without this, --ui
	// would give --no-hints one meaning on the command line and a
	// silent no-op under the web UI - the same flag would stop meaning
	// "I don't want hints" depending on which mode it's combined with.
	NoHints bool
	// Initial seeds the page's form (target/skills/goal/etc.) - see
	// --ui's flag seeding in cli, which populates this the same way
	// --tui pre-populates the picker.
	Initial prompt.Inputs
	// Logger receives structured diagnostic events (browser-open
	// failures, shutdown, request-handling errors). Defaults to
	// slog.Default() if nil. Deliberately separate from Stdout: this
	// is for operational log lines, not the primary human-facing
	// message (see Stdout).
	Logger *slog.Logger
	// Stdout receives the one human-facing "here's the URL" banner.
	// Defaults to os.Stdout if nil.
	Stdout io.Writer
	// Ready, if non-nil, is called with this server's URL once it's
	// bound and about to start serving - before opening a browser or
	// blocking. Exists so tests can learn the OS-assigned port
	// deterministically, without parsing printed output or polling on
	// a fixed sleep; real callers have no need to set it.
	Ready func(url string)
}

// Serve runs promptsmith's local web UI until ctx is done, then shuts
// down gracefully (waiting up to shutdownTimeout for in-flight
// requests). Binds loopback-only - never a wildcard address -
// regardless of Options.Port.
//
// Serve has no OS signal handling of its own: the caller decides how
// ctx gets canceled (the CLI wiring uses signal.NotifyContext for
// Ctrl-C). That's what makes shutdown deterministic in a test - a
// plain context.WithCancel - rather than needing to send a real signal
// to the test process, which would affect the whole `go test` run, not
// just one goroutine.
func Serve(ctx context.Context, reg *registry.Registry, opts Options) error {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", opts.Port))
	if err != nil {
		return fmt.Errorf("promptsmith: listen: %w", err)
	}

	app, err := newApplication(reg, logger, opts.Initial)
	if err != nil {
		return err
	}
	// Set directly on app rather than threaded through newApplication
	// as a parameter (the way opts.Initial is): newApplication is also
	// called by this package's own tests via a small fixture registry
	// (see testAppWithInitial in testhelpers_test.go), and those call
	// sites have no Options value to draw a NoHints from. Assigning it
	// here, once Options is already in scope, avoids changing that
	// constructor's signature for every existing caller just to plumb
	// one field only Serve itself has.
	app.noHints = opts.NoHints
	srv := &http.Server{
		Handler:           app.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	url := fmt.Sprintf("http://%s", listener.Addr())
	if opts.Ready != nil {
		opts.Ready(url)
	}
	fmt.Fprintf(stdout, "promptsmith: web UI listening on %s (press Ctrl-C to stop)\n", url)

	if !opts.NoBrowser {
		if err := openBrowser(url); err != nil {
			logger.Warn("could not open a browser automatically", "error", err, "url", url)
		}
	}

	shutdownErr := make(chan error, 1)
	go func() {
		<-ctx.Done()
		logger.Info("shutting down")
		shutdownErr <- gracefulShutdown(srv)
	}()

	if err := srv.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("promptsmith: serve: %w", err)
	}
	return <-shutdownErr
}

// gracefulShutdown calls srv.Shutdown with a fresh, independently-timed
// context. By the time this runs, Serve's own ctx parameter has
// already fired Done() - deriving the shutdown deadline from it would
// hand srv.Shutdown an already-expired context, giving in-flight
// requests zero grace period and defeating the entire point of
// shutdownTimeout. A goroutine calling this always terminates within
// shutdownTimeout of being invoked, regardless of what triggered the
// shutdown, so there's no unbounded-lifetime risk in using a fresh
// context.Background() here.
func gracefulShutdown(srv *http.Server) error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
