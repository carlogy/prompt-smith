//go:build e2e

// e2e_test.go drives a real headless Chrome (via chromedp) against a
// real bound HTTP server, for exactly the interactions that live
// entirely in the browser - clipboard writes, file downloads, and DOM
// mutations from index.html's inline JS - none of which a Go-only
// httptest.NewRecorder request (used by every other test in this
// package) can exercise.
//
// Excluded from the default `go test ./...` and the -race CI matrix:
// these need a real Chrome/Chromium binary on PATH and are slower and
// less deterministic than the rest of the suite. Run explicitly via
// `make test-e2e`; see .github/workflows/e2e.yml for the opt-in CI
// job that installs Chrome and runs them.
package server

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/chromedp"

	"github.com/carlogy/prompt-smith/internal/prompt"
)

// newChromeContext starts a headless Chrome/Chromium instance and
// returns a context bound to one browser tab, torn down automatically
// at test end via t.Cleanup. Chromium's binary path comes from
// CHROMEDP_EXEC_PATH when set - see Dockerfile.e2e, which pins an
// exact Chromium build (chromedp/headless-shell) rather than
// depending on whatever browser happens to be on PATH; that ambiguity
// is what caused this suite's first real CI failure (it passed
// against every Chrome version tested locally, but never against
// whatever ubuntu-latest's runner image actually had preinstalled).
// Falls back to chromedp's own PATH-based auto-detection when unset,
// so this still works for local ad-hoc runs outside the container.
// NoSandbox is added on top of chromedp's own defaults (which already
// include Headless) since this typically runs as root in a container,
// where Chrome's sandbox refuses to start otherwise; harmless
// everywhere else. The 30s deadline exists so a chromedp/Chrome hang
// fails this test in seconds rather than running until go test's own
// much longer default timeout.
func newChromeContext(t *testing.T) context.Context {
	t.Helper()

	opts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.NoSandbox)
	if p := os.Getenv("CHROMEDP_EXEC_PATH"); p != "" {
		opts = append(opts, chromedp.ExecPath(p))
	}
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	t.Cleanup(cancelAlloc)

	ctx, cancelCtx := chromedp.NewContext(allocCtx)
	t.Cleanup(cancelCtx)

	ctx, cancelTimeout := context.WithTimeout(ctx, 30*time.Second)
	t.Cleanup(cancelTimeout)
	return ctx
}

// startTestServer binds a real loopback HTTP server for a headless
// Chrome to navigate to. httptest.NewRecorder (what every other test
// in this package uses) has no real socket for a separate browser
// process to connect to; httptest.NewServer does.
func startTestServer(t *testing.T, initial prompt.Inputs) string {
	t.Helper()
	srv := httptest.NewServer(testAppWithInitial(initial).routes())
	t.Cleanup(srv.Close)
	return srv.URL
}

// dispatchChange sets sel's .value via JS and then manually dispatches
// a bubbling "change" event. Setting .value programmatically - unlike
// a real user picking an <option> - does not fire one on its own, and
// index.html's target-filter listener (applyTargetFilter, wired to
// #target's "change" event) depends on one arriving.
func dispatchChange(sel, value string) chromedp.Action {
	return chromedp.Tasks{
		chromedp.SetValue(sel, value, chromedp.ByQuery),
		chromedp.Evaluate(fmt.Sprintf(
			`document.querySelector(%q).dispatchEvent(new Event("change", {bubbles: true}))`, sel),
			nil),
	}
}

// waitForDownload polls dir for a file that isn't still mid-download
// (Chrome names an in-progress download "<name>.crdownload" until it
// completes) and returns its path, or fails the test after timeout.
// chromedp has no built-in "wait for download" action; CDP does expose
// download events, but polling the filesystem for the real completed
// file is simpler and avoids wiring up an event listener just for
// this.
func waitForDownload(t *testing.T, dir string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("reading download dir %s: %v", dir, err)
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".crdownload") {
				return filepath.Join(dir, e.Name())
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("no completed download appeared in %s within %s", dir, timeout)
	return ""
}

// TestE2E_CopyButtonCopiesPreviewText proves the Copy button's actual
// clipboard write succeeds end-to-end: #copy-status only ever reads
// "Copied" inside the navigator.clipboard.writeText().then callback
// (see index.html), so its appearance is proof the write promise
// resolved, not just that the click handler ran.
func TestE2E_CopyButtonCopiesPreviewText(t *testing.T) {
	url := startTestServer(t, prompt.Inputs{Target: "generic", Skills: []string{"diagnose"}, Goal: "fix the flaky test"})
	ctx := newChromeContext(t)

	var copied bool
	err := chromedp.Run(ctx,
		// Headless Chrome otherwise silently refuses clipboard access.
		browser.SetPermission(&browser.PermissionDescriptor{Name: "clipboard-read"}, browser.PermissionSettingGranted),
		browser.SetPermission(&browser.PermissionDescriptor{Name: "clipboard-write"}, browser.PermissionSettingGranted),
		chromedp.Navigate(url),
		chromedp.WaitVisible("#preview-text", chromedp.ByQuery), // the seeded goal+skill built once, async via htmx's "load" trigger
		chromedp.Click("#copy-button", chromedp.ByQuery),
		chromedp.Poll(`document.getElementById("copy-status").textContent === "Copied"`, &copied,
			chromedp.WithPollingTimeout(5*time.Second)),
	)
	if err != nil {
		t.Fatalf("chromedp.Run: %v", err)
	}
	if !copied {
		t.Error("#copy-status never showed \"Copied\" after clicking Copy")
	}
}

// TestE2E_DownloadButtonSavesPromptText proves the Download button
// saves a file matching both the previewed text and the server-
// supplied filename (data-filename, from naming.SuggestFilename - see
// preview.go), by pointing Chrome's real download machinery at a temp
// directory and reading back what actually landed on disk.
func TestE2E_DownloadButtonSavesPromptText(t *testing.T) {
	url := startTestServer(t, prompt.Inputs{Target: "generic", Skills: []string{"diagnose"}, Goal: "fix the flaky test"})
	ctx := newChromeContext(t)
	downloadDir := t.TempDir()

	var wantText, wantFilename string
	err := chromedp.Run(ctx,
		browser.SetDownloadBehavior(browser.SetDownloadBehaviorBehaviorAllow).WithDownloadPath(downloadDir),
		chromedp.Navigate(url),
		chromedp.WaitVisible("#preview-text", chromedp.ByQuery),
		// Read back exactly what the app's own download handler reads
		// (pre.textContent / pre.dataset.filename - see index.html) so
		// this compares apples to apples, not some other DOM-text
		// extraction with different whitespace-normalization rules.
		chromedp.Evaluate(`document.getElementById("preview-text").textContent`, &wantText),
		chromedp.Evaluate(`document.getElementById("preview-text").dataset.filename`, &wantFilename),
		chromedp.Click("#download-button", chromedp.ByQuery),
	)
	if err != nil {
		t.Fatalf("chromedp.Run: %v", err)
	}

	gotPath := waitForDownload(t, downloadDir, 5*time.Second)
	gotBytes, err := os.ReadFile(gotPath)
	if err != nil {
		t.Fatalf("reading downloaded file: %v", err)
	}
	if got := string(gotBytes); got != wantText {
		t.Errorf("downloaded file content = %q, want %q (the previewed text)", got, wantText)
	}
	if gotName := filepath.Base(gotPath); gotName != wantFilename {
		t.Errorf("downloaded filename = %q, want %q (from data-filename)", gotName, wantFilename)
	}
}

// TestE2E_ClearButtonResetsForm proves Clear's DOM-mutation side (see
// index.html's clear-button handler) actually happens in a real
// browser: textarea emptied, skill unchecked, target reset to the
// first option, and - since Clear also fires promptsmith:refresh -
// the live preview rebuilding back down to its empty-state
// placeholder.
func TestE2E_ClearButtonResetsForm(t *testing.T) {
	url := startTestServer(t, prompt.Inputs{Target: "opencode", Skills: []string{"verify"}, Goal: "some seeded goal"})
	ctx := newChromeContext(t)

	var goalVal, targetVal string
	var verifyChecked, placeholderShown bool
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible("#preview-text", chromedp.ByQuery), // the seeded prompt built once
		chromedp.Click("#clear-button", chromedp.ByQuery),
		chromedp.Poll(`document.getElementById("preview-text") === null`, &placeholderShown,
			chromedp.WithPollingTimeout(5*time.Second)),
		chromedp.Value("#goal", &goalVal, chromedp.ByQuery),
		chromedp.Value("#target", &targetVal, chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelector('input[name="skills"][value="verify"]').checked`, &verifyChecked),
	)
	if err != nil {
		t.Fatalf("chromedp.Run: %v", err)
	}

	if goalVal != "" {
		t.Errorf("goal textarea = %q, want empty after Clear", goalVal)
	}
	if targetVal != "generic" {
		t.Errorf(`target select = %q, want "generic" (this fixture's first target alphabetically) after Clear`, targetVal)
	}
	if verifyChecked {
		t.Error("verify checkbox is still checked after Clear")
	}
	if !placeholderShown {
		t.Error("preview did not return to the empty-state placeholder after Clear")
	}
}

// TestE2E_TargetChangeFiltersUnsupportedSkills proves index.html's
// applyTargetFilter actually runs in a real browser: agent-only (the
// fixture's Body-less skill - see testhelpers_test.go) starts disabled
// and dimmed on the default target (generic, inline mode, requires a
// Body), becomes selectable once switched to opencode (reference mode,
// supports every skill), and - if it was checked while enabled -
// auto-unchecks itself when switched back to a target that doesn't
// support it.
func TestE2E_TargetChangeFiltersUnsupportedSkills(t *testing.T) {
	url := startTestServer(t, prompt.Inputs{}) // unseeded: the <select> defaults to its first <option>, "generic"
	ctx := newChromeContext(t)

	const agentOnly = `input[name="skills"][value="agent-only"]`
	dimmed := func(sel string) string {
		return fmt.Sprintf(`document.querySelector(%q).closest("[data-skill-row]").classList.contains("opacity-50")`, sel)
	}
	disabled := func(sel string) string { return fmt.Sprintf(`document.querySelector(%q).disabled`, sel) }
	checked := func(sel string) string { return fmt.Sprintf(`document.querySelector(%q).checked`, sel) }

	var disabledOnGeneric, dimmedOnGeneric bool
	var disabledOnOpencode, dimmedOnOpencode bool
	var checkedBeforeSwitchBack bool
	var disabledAfterSwitchBack, checkedAfterSwitchBack bool

	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible("#target", chromedp.ByQuery),
		chromedp.Evaluate(disabled(agentOnly), &disabledOnGeneric),
		chromedp.Evaluate(dimmed(agentOnly), &dimmedOnGeneric),

		dispatchChange("#target", "opencode"),
		chromedp.Evaluate(disabled(agentOnly), &disabledOnOpencode),
		chromedp.Evaluate(dimmed(agentOnly), &dimmedOnOpencode),
		chromedp.Click(agentOnly, chromedp.ByQuery),
		chromedp.Evaluate(checked(agentOnly), &checkedBeforeSwitchBack),

		dispatchChange("#target", "generic"),
		chromedp.Evaluate(disabled(agentOnly), &disabledAfterSwitchBack),
		chromedp.Evaluate(checked(agentOnly), &checkedAfterSwitchBack),
	)
	if err != nil {
		t.Fatalf("chromedp.Run: %v", err)
	}

	if !disabledOnGeneric || !dimmedOnGeneric {
		t.Errorf("agent-only should start disabled+dimmed on generic (unsupported): disabled=%v dimmed=%v", disabledOnGeneric, dimmedOnGeneric)
	}
	if disabledOnOpencode || dimmedOnOpencode {
		t.Errorf("agent-only should be enabled+undimmed on opencode (reference mode supports every skill): disabled=%v dimmed=%v", disabledOnOpencode, dimmedOnOpencode)
	}
	if !checkedBeforeSwitchBack {
		t.Fatal("clicking the (now-enabled) agent-only checkbox on opencode did not check it")
	}
	if !disabledAfterSwitchBack {
		t.Error("agent-only should be disabled again after switching back to generic")
	}
	if checkedAfterSwitchBack {
		t.Error("agent-only should have been auto-unchecked when it became unsupported again")
	}
}
