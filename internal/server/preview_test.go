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
