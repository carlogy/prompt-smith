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
	if strings.Contains(body, "preview-error") {
		t.Errorf("fragment rendered an error class on a successful build, got:\n%s", body)
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
	// "preview-error" is a stable semantic hook independent of the
	// Tailwind utility classes that style it - checked as a class-name
	// substring, not an exact class="..." boundary, since it co-exists
	// with those utilities in the same attribute.
	if !strings.Contains(body, "preview-error") {
		t.Errorf("fragment missing the error partial, got:\n%s", body)
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
