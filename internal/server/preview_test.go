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
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestHandlePreview_Success(t *testing.T) {
	app := testApp()
	form := url.Values{
		"target": {"generic"},
		"skills": {"diagnose"},
		"goal":   {"fix the flaky test"},
	}
	req := newLocalRequest(http.MethodPost, "/preview", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html...", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "fix the flaky test") || !strings.Contains(body, "Build a feedback loop first.") {
		t.Errorf("fragment missing expected content, got:\n%s", body)
	}
	if strings.Contains(body, `role="alert"`) {
		t.Errorf("fragment rendered an assertive role=\"alert\" on a successful build, got:\n%s", body)
	}
}

// TestHandlePreview_HighlightsSectionTags guards the feedback-driven
// highlighting feature: opening tags get the cornflower accent,
// closing tags get dimmed - and, critically, it's the *shared*
// internal/prompthl classifier doing the classifying, the same one
// the TUI's live preview uses, so the two can never highlight
// differently for the same prompt.
func TestHandlePreview_HighlightsSectionTags(t *testing.T) {
	app := testApp()
	form := url.Values{
		"target": {"generic"},
		"skills": {"diagnose"},
		"goal":   {"fix the flaky test"},
	}
	req := newLocalRequest(http.MethodPost, "/preview", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	body := rec.Body.String()
	wantOpen := `<span class="text-cornflower-600 dark:text-cornflower-300">&lt;task&gt;</span>`
	wantClose := `<span class="text-slate-500 dark:text-slate-400">&lt;/task&gt;</span>`
	if !strings.Contains(body, wantOpen) {
		t.Errorf("fragment missing the highlighted opening tag %q, got:\n%s", wantOpen, body)
	}
	if !strings.Contains(body, wantClose) {
		t.Errorf("fragment missing the dimmed closing tag %q, got:\n%s", wantClose, body)
	}
	// The body line between them must stay plain - no span wrapping a
	// non-tag line.
	if strings.Contains(body, `<span class="text-cornflower-600 dark:text-cornflower-300">fix the flaky test</span>`) {
		t.Error("a content line was highlighted as if it were a tag")
	}
}

func TestHandlePreview_MultipleSkillsAllIncluded(t *testing.T) {
	// Checkboxes sharing a name submit as repeated form keys - proves
	// r.Form["skills"] (not r.FormValue, which only returns the first)
	// is what feeds prompt.Build.
	app := testApp()
	form := url.Values{
		"target": {"generic"},
		"skills": {"diagnose", "verify"},
		"goal":   {"x"},
	}
	req := newLocalRequest(http.MethodPost, "/preview", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "Build a feedback loop first.") || !strings.Contains(body, "Run the checks.") {
		t.Errorf("fragment missing content from both selected skills, got:\n%s", body)
	}
}

func TestHandlePreview_UnknownSkillIsA200WithInlineError(t *testing.T) {
	// A build-logic error (bad target/skill) is an expected, routine
	// outcome of live preview - not a malformed request - so it must
	// stay 200 with the error rendered inline: htmx does not swap
	// 4xx/5xx responses by default, so a non-200 here would leave the
	// preview pane stuck on stale content.
	app := testApp()
	form := url.Values{
		"target": {"generic"},
		"skills": {"does-not-exist"},
		"goal":   {"x"},
	}
	req := newLocalRequest(http.MethodPost, "/preview", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (build errors are not request errors), body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	body := rec.Body.String()
	// role="alert" is the single source of truth for a preview error:
	// it is both the semantic error marker and the element that
	// announces (assertively) to screen readers, and index.html's
	// htmx:afterSettle handler keys its announce-or-stay-silent
	// decision off this exact selector. Asserting on it here keeps the
	// template and that JS selector from silently drifting apart.
	if !strings.Contains(body, `role="alert"`) {
		t.Errorf("fragment missing the error alert, got:\n%s", body)
	}
	if !strings.Contains(body, "does-not-exist") {
		t.Errorf("fragment error doesn't mention the unknown skill, got:\n%s", body)
	}
}

func TestHandlePreview_OversizedBodyReturns400(t *testing.T) {
	app := testApp()
	form := url.Values{"goal": {strings.Repeat("x", maxRequestBody+1)}}
	req := newLocalRequest(http.MethodPost, "/preview", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestHandlePreview_EscapesUserSuppliedContent(t *testing.T) {
	// html/template auto-escapes by construction - proven empirically
	// here (same discipline as TestHandleIndex_EscapesUserSuppliedContent)
	// for the fragment endpoint specifically, since it's a separate
	// template execution path from the index page.
	app := testApp()
	form := url.Values{
		"target": {"generic"},
		"skills": {"diagnose"},
		"goal":   {`<script>alert(1)</script>`},
	}
	req := newLocalRequest(http.MethodPost, "/preview", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	body := rec.Body.String()
	if strings.Contains(body, `<script>alert(1)</script>`) {
		t.Errorf("goal was rendered unescaped - XSS risk, got:\n%s", body)
	}
	if !strings.Contains(body, `&lt;script&gt;alert(1)&lt;/script&gt;`) {
		t.Errorf("expected the goal to be HTML-escaped, got:\n%s", body)
	}
}

// TestHandlePreview_EmptyGoalShowsPlaceholder guards the third branch
// of preview.html (error / lines / neither): nothing built yet is
// distinct from a build that produced content, and must not render
// #preview-text at all - the empty-state placeholder takes its place.
func TestHandlePreview_EmptyGoalShowsPlaceholder(t *testing.T) {
	app := testApp()
	form := url.Values{"target": {"generic"}}
	req := newLocalRequest(http.MethodPost, "/preview", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "Enter a goal") {
		t.Errorf("fragment missing the empty-state placeholder, got:\n%s", body)
	}
	if strings.Contains(body, `id="preview-text"`) {
		t.Errorf("fragment rendered #preview-text with nothing built, got:\n%s", body)
	}
}

// TestHandlePreview_IncludesDownloadFilename guards the Download
// button's data source: the fragment must carry a suggested filename
// (from the shared internal/naming, the same one the TUI's save
// prompt uses) for the button's script to read.
func TestHandlePreview_IncludesDownloadFilename(t *testing.T) {
	app := testApp()
	form := url.Values{
		"target": {"generic"},
		"skills": {"diagnose"},
		"goal":   {"fix the bug"},
	}
	req := newLocalRequest(http.MethodPost, "/preview", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `data-filename="promptsmith-`) || !strings.Contains(body, `.txt"`) {
		t.Errorf("fragment missing a suggested download filename, got:\n%s", body)
	}
}

// TestHandlePreview_ExamplesFieldSplitsIntoMultipleExampleBlocks proves
// the one "examples" form key (a single textarea - see index.html)
// gets divided by prompt.SplitExamples (preview.go) into as many
// <example> children as "---"-separated pieces it contains, mirroring
// internal/prompt's own TestBuild_ExamplesSectionShape but exercised
// through the actual HTTP form-parsing path rather than calling
// prompt.Build directly.
func TestHandlePreview_ExamplesFieldSplitsIntoMultipleExampleBlocks(t *testing.T) {
	app := testApp()
	form := url.Values{
		"target":   {"generic"},
		"goal":     {"x"},
		"examples": {"first example\n---\nsecond example"},
	}
	req := newLocalRequest(http.MethodPost, "/preview", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	body := rec.Body.String()
	if n := strings.Count(body, "&lt;example&gt;"); n != 2 {
		t.Errorf("expected 2 <example> blocks, found %d, got:\n%s", n, body)
	}
	if !strings.Contains(body, "first example") || !strings.Contains(body, "second example") {
		t.Errorf("fragment missing one or both examples, got:\n%s", body)
	}
}

// TestHandlePreview_EmptyExamplesRendersNoExamplesSection proves an
// empty (or whitespace-only) examples textarea omits the whole
// <examples> wrapper, mirroring prompt.TestBuild_ExamplesOmittedWhenEmpty
// through the HTTP path.
//
// Asserts on the escaped "&lt;examples&gt;" tag specifically, not the
// bare substring "examples": a blank examples field also fires
// promptlint's no-examples finding (see Task 2/3), whose own message
// text ("No examples given; ...") legitimately contains the word
// "examples" - checking for the tag is what keeps this test asserting
// on the <examples> wrapper it's actually named for, rather than
// colliding with that unrelated, correct finding text.
func TestHandlePreview_EmptyExamplesRendersNoExamplesSection(t *testing.T) {
	app := testApp()
	form := url.Values{
		"target":   {"generic"},
		"goal":     {"x"},
		"examples": {"   "},
	}
	req := newLocalRequest(http.MethodPost, "/preview", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	body := rec.Body.String()
	if strings.Contains(body, "&lt;examples&gt;") {
		t.Errorf("expected no <examples> section for a blank examples field, got:\n%s", body)
	}
}

// TestHandlePreview_ExamplesFieldNormalizesCRLF proves the browser-
// realistic case: a <textarea> submits CRLF line endings, and
// prompt.SplitExamples's CRLF normalization (see build.go) must still
// recognize the "---" separator line and keep each example's own
// internal line breaks intact - the exact failure mode that
// normalization exists to prevent (see internal/prompt's own
// TestSplitExamples CRLF cases, exercised there without an HTTP round
// trip; this proves the same behavior survives actually going through
// url.Values encoding and r.FormValue).
func TestHandlePreview_ExamplesFieldNormalizesCRLF(t *testing.T) {
	app := testApp()
	form := url.Values{
		"target":   {"generic"},
		"goal":     {"x"},
		"examples": {"first example\r\n---\r\nsecond example"},
	}
	req := newLocalRequest(http.MethodPost, "/preview", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	body := rec.Body.String()
	if n := strings.Count(body, "&lt;example&gt;"); n != 2 {
		t.Errorf("expected 2 <example> blocks from a CRLF-separated field, found %d, got:\n%s", n, body)
	}
	if !strings.Contains(body, "first example") || !strings.Contains(body, "second example") {
		t.Errorf("fragment missing one or both examples, got:\n%s", body)
	}
	// A literal "\r" surviving into the rendered output would mean the
	// CRLF normalization didn't run before splitting.
	if strings.Contains(body, "\r") {
		t.Errorf("fragment contains an unnormalized carriage return, got:\n%q", body)
	}
}

// TestHandlePreview_GoalOnlyListsEveryFindingUncollapsed proves the
// web UI's deliberate divergence from the CLI (see previewData's
// Findings comment in preview.go): a goal-only prompt trips all three
// pure-absence rules (no-role, no-output-format, no-examples), and
// unlike internal/cli/generate.go's warnLintFindings - which collapses
// those three into a single stderr line - this pane renders each one
// as its own data-rule="..." list item.
func TestHandlePreview_GoalOnlyListsEveryFindingUncollapsed(t *testing.T) {
	app := testApp()
	form := url.Values{
		"target": {"generic"},
		"goal":   {"fix the flaky checkout test"},
	}
	req := newLocalRequest(http.MethodPost, "/preview", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `id="preview-hints"`) {
		t.Errorf("fragment missing the findings block, got:\n%s", body)
	}
	for _, rule := range []string{"no-role", "no-output-format", "no-examples"} {
		want := `data-rule="` + rule + `"`
		if !strings.Contains(body, want) {
			t.Errorf("fragment missing %s (the web UI must not collapse these), got:\n%s", want, body)
		}
	}
}

// TestHandlePreview_WellFormedPromptRendersNoFindingsBlock proves a
// clean prompt (role, output_format, 3+ examples, a goal at or over
// minGoalChars) renders no #preview-hints block at all - the
// {{if .Findings}} guard in preview.html.
func TestHandlePreview_WellFormedPromptRendersNoFindingsBlock(t *testing.T) {
	app := testApp()
	form := url.Values{
		"target":       {"generic"},
		"goal":         {"fix the flaky checkout test"},
		"role":         {"a senior engineer"},
		"outputFormat": {"a unified diff"},
		"examples":     {"input: 1 -> output: one\n---\ninput: 2 -> output: two\n---\ninput: 3 -> output: three"},
	}
	req := newLocalRequest(http.MethodPost, "/preview", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	body := rec.Body.String()
	if strings.Contains(body, `id="preview-hints"`) {
		t.Errorf("expected no findings block for a well-formed prompt, got:\n%s", body)
	}
}

// TestHandlePreview_BuildErrorRendersNoFindingsBlock proves the
// template's {{if .Error}} branch owns the pane exclusively: a build
// error must show the existing role="alert" error and must NOT also
// render #preview-hints underneath it (see handlePreview's own comment
// on why findings are suppressed on buildErr != nil).
func TestHandlePreview_BuildErrorRendersNoFindingsBlock(t *testing.T) {
	app := testApp()
	form := url.Values{
		"target": {"generic"},
		"skills": {"does-not-exist"},
		"goal":   {"x"},
	}
	req := newLocalRequest(http.MethodPost, "/preview", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, `role="alert"`) {
		t.Errorf("fragment missing the error alert, got:\n%s", body)
	}
	if strings.Contains(body, `id="preview-hints"`) {
		t.Errorf("expected no findings block alongside a build error, got:\n%s", body)
	}
}

// TestHandlePreview_NoHintsSuppressesFindingsBlock proves
// application.noHints (set from server.Options.NoHints, itself set
// from the CLI's --no-hints flag - see server.Options and
// internal/cli/generate.go's runUI) suppresses the findings block
// entirely while leaving the built prompt untouched.
func TestHandlePreview_NoHintsSuppressesFindingsBlock(t *testing.T) {
	app := testApp()
	app.noHints = true
	form := url.Values{
		"target": {"generic"},
		"goal":   {"fix the flaky checkout test"},
	}
	req := newLocalRequest(http.MethodPost, "/preview", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "fix the flaky checkout test") {
		t.Errorf("expected the prompt to still render normally under --no-hints, got:\n%s", body)
	}
	if strings.Contains(body, `id="preview-hints"`) {
		t.Errorf("expected --no-hints to suppress the findings block, got:\n%s", body)
	}
}

// TestHandlePreview_FindingsBlockHasNoLiveRegionRole pins the
// accessibility decision recorded in preview.html's own comment above
// #preview-hints: the form posts on hx-trigger="input changed
// delay:300ms", so an aria-live role here (role="alert" or
// role="status") would re-announce every suggestion to a screen-reader
// user on essentially every keystroke. This test exists so a future
// change adding one back has to consciously break a test, not
// silently regress.
func TestHandlePreview_FindingsBlockHasNoLiveRegionRole(t *testing.T) {
	app := testApp()
	form := url.Values{
		"target": {"generic"},
		"goal":   {"fix the flaky checkout test"},
	}
	req := newLocalRequest(http.MethodPost, "/preview", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	body := rec.Body.String()
	hintsStart := strings.Index(body, `id="preview-hints"`)
	if hintsStart == -1 {
		t.Fatalf("fragment missing the findings block, got:\n%s", body)
	}
	hintsBlock := body[hintsStart:]
	if strings.Contains(hintsBlock, `role="alert"`) || strings.Contains(hintsBlock, `role="status"`) {
		t.Errorf("findings block must not carry a live-region role, got:\n%s", hintsBlock)
	}
}
