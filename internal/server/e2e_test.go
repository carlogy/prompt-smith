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

// appendAndDispatchInput appends suffix to sel's current value via JS
// and dispatches a real, bubbling "input" event - the same event
// index.html's hx-trigger="input changed delay:300ms" depends on to
// start its debounce. Used instead of chromedp.Click+SendKeys where a
// test needs the edit to land at a specific, deterministic spot (the
// end of already-seeded content): clicking into a multi-row textarea
// (see #goal's rows="3") places the caret wherever the click
// coordinates happen to land relative to existing text, which is the
// kind of layout-dependent guesswork this sidesteps entirely - the
// same reasoning as dispatchChange above for the "change" event.
func appendAndDispatchInput(sel, suffix string) chromedp.Action {
	return chromedp.Evaluate(fmt.Sprintf(
		`(function(){var el=document.querySelector(%q);el.value+=%q;el.dispatchEvent(new Event("input",{bubbles:true}));})()`,
		sel, suffix), nil)
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

// TestE2E_LivePreviewUpdatesAfterDebounce proves the live preview
// still updates correctly now that the form swaps via
// hx-swap="morph:innerHTML" instead of plain innerHTML replacement
// (see index.html): typing into #examples, after waiting out htmx's
// real 300ms debounce (hx-trigger="input changed delay:300ms"),
// produces a preview whose rendered content includes what was typed.
// This deliberately asserts only on rendered *content*: under
// morphing, #preview-text is a reused node rather than a fresh one on
// every update, so any assertion resting on node identity or on the
// node having been replaced would be the wrong thing to check here -
// see TestE2E_SelectionSurvivesLivePreviewMorph below for the test
// that actually exercises that reuse.
func TestE2E_LivePreviewUpdatesAfterDebounce(t *testing.T) {
	url := startTestServer(t, prompt.Inputs{Target: "generic", Skills: []string{"diagnose"}, Goal: "fix the flaky test"})
	ctx := newChromeContext(t)

	const marker = "debounced-marker-42"
	var containsMarker bool
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible("#preview-text", chromedp.ByQuery), // the seeded goal built once, async via htmx's "load" trigger
		// #examples lives inside the "Optional fields" <details> (see
		// index.html), collapsed by default since this fixture seeds no
		// optional fields - and a collapsed <details>'s non-summary
		// content isn't focusable, so Click below would fail without
		// this. A real user would click the <summary> to get here;
		// clicking it directly (rather than just forcing .open via JS)
		// keeps this exercising the same toggle a user's click does.
		chromedp.Click("main details summary", chromedp.ByQuery),
		chromedp.Click("#examples", chromedp.ByQuery),
		chromedp.SendKeys("#examples", marker, chromedp.ByQuery),
		// Polls rather than a fixed Sleep past the 300ms debounce: this
		// waits exactly as long as the real request+render+swap takes,
		// no more, no less, and would time out (not false-pass) if the
		// debounce were ever changed to something longer.
		chromedp.Poll(fmt.Sprintf(`document.getElementById("preview-text").textContent.includes(%q)`, marker), &containsMarker,
			chromedp.WithPollingTimeout(5*time.Second)),
	)
	if err != nil {
		t.Fatalf("chromedp.Run: %v", err)
	}
	if !containsMarker {
		t.Errorf("preview did not include %q after typing into #examples and waiting out the debounce", marker)
	}
}

// selectSubstringJS is injected into the page by
// TestE2E_SelectionSurvivesLivePreviewMorph to build a real
// window.Selection over a substring of #preview-text's rendered
// content. There is no chromedp action for "select this text" (that's
// a purely in-page DOM operation, not an input-device gesture chromedp
// otherwise models), so this walks #preview-text's text nodes directly
// with a TreeWalker to find the one containing the target substring,
// then builds a Range over exactly that substring - the same DOM APIs
// a user's real text-drag selection produces.
const selectSubstringJS = `function(rootId, substr) {
	var root = document.getElementById(rootId);
	var walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
	var node;
	while ((node = walker.nextNode())) {
		var idx = node.nodeValue.indexOf(substr);
		if (idx !== -1) {
			var range = document.createRange();
			range.setStart(node, idx);
			range.setEnd(node, idx + substr.length);
			var sel = window.getSelection();
			sel.removeAllRanges();
			sel.addRange(range);
			return true;
		}
	}
	return false;
}`

// TestE2E_SelectionSurvivesLivePreviewMorph is the headline regression
// test for this phase: it proves a user's text selection inside the
// live preview survives a re-render, which is the entire reason the
// preview swaps via idiomorph (hx-swap="morph:innerHTML") instead of
// htmx's default innerHTML replacement (see index.html). Plain
// innerHTML replacement tears down and rebuilds every node under
// #preview on each swap - including nodes whose rendered text didn't
// even change - which unconditionally collapses any live
// window.Selection pointing into the old tree. Idiomorph instead
// reuses matching nodes across a swap, so a Selection anchored in a
// part of the preview that didn't change (here, the seeded examples
// text, while only the goal changes) keeps pointing at a still-live,
// still-attached node afterward.
//
// This seeds Goal and Examples with distinct content on purpose: the
// selection is made in the examples text specifically so the test
// isn't merely checking that *something* under #preview survived -
// selecting text that itself is left untouched by the edit is what
// makes node-reuse (rather than incidental luck) the reason the
// selection survives.
func TestE2E_SelectionSurvivesLivePreviewMorph(t *testing.T) {
	url := startTestServer(t, prompt.Inputs{
		Target:   "generic",
		Skills:   []string{"diagnose"},
		Goal:     "original goal text",
		Examples: []string{"stable example marker text"},
	})
	ctx := newChromeContext(t)

	var selected, edited bool
	var selectionBefore, selectionAfter string
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitVisible("#preview-text", chromedp.ByQuery), // the seeded goal+examples built once
		chromedp.Evaluate(fmt.Sprintf(`(%s)("preview-text", "example marker")`, selectSubstringJS), &selected),
		chromedp.Evaluate(`window.getSelection().toString()`, &selectionBefore),
		appendAndDispatchInput("#goal", "!"),
		// Poll for the edit to actually land in the preview (proof the
		// debounced re-render this test cares about really happened)
		// before reading the selection back out.
		chromedp.Poll(`document.getElementById("preview-text").textContent.includes("original goal text!")`, &edited,
			chromedp.WithPollingTimeout(5*time.Second)),
		chromedp.Evaluate(`window.getSelection().toString()`, &selectionAfter),
	)
	if err != nil {
		t.Fatalf("chromedp.Run: %v", err)
	}
	if !selected {
		t.Fatal("could not build a selection over \"example marker\" inside #preview-text")
	}
	if selectionBefore == "" {
		t.Fatal("selection was empty immediately after being made, before any re-render")
	}
	if selectionAfter == "" {
		t.Error("selection was cleared by the live-preview re-render; want it to survive the morph swap")
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
