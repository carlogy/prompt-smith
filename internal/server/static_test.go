package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStaticHandler_ServesHTMX(t *testing.T) {
	app := testApp()
	req := newLocalRequest(http.MethodGet, "/static/htmx.min.js", nil)
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/javascript") {
		t.Errorf("Content-Type = %q, want text/javascript...", ct)
	}
	// A real check that the vendored file is intact, not just that
	// *some* 200 came back: htmx's own top-of-file banner comment
	// names the project and license.
	body := rec.Body.String()
	if !strings.Contains(body, "htmx") {
		t.Errorf("served body doesn't look like htmx.min.js (missing \"htmx\"), len=%d", len(body))
	}
	if rec.Body.Len() < 10000 {
		t.Errorf("served body suspiciously small (%d bytes) for htmx.min.js", rec.Body.Len())
	}
}

func TestStaticHandler_ServesAppCSS(t *testing.T) {
	app := testApp()
	req := newLocalRequest(http.MethodGet, "/static/app.css", nil)
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
		t.Errorf("Content-Type = %q, want text/css...", ct)
	}
	// A real check the built stylesheet actually contains our theme,
	// not just that some 200 came back with some CSS in it - if
	// make ui-css were run against a stale/wrong input, or the
	// template's class usage stopped matching the @source scan, this
	// class would be the first thing to silently disappear.
	body := rec.Body.String()
	if !strings.Contains(body, "clay") {
		t.Errorf("served app.css doesn't reference the clay theme color, got len=%d", len(body))
	}
}

func TestStaticHandler_UnknownFileReturns404(t *testing.T) {
	app := testApp()
	req := newLocalRequest(http.MethodGet, "/static/no-such-file.js", nil)
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// TestStaticHandler_RespectsLocalOnlyMiddleware guards against a
// regression where a future refactor moves static serving outside
// enforceLocalOnly - every route this server has must stay
// loopback-only, static assets included.
func TestStaticHandler_RespectsLocalOnlyMiddleware(t *testing.T) {
	app := testApp()
	req := httptest.NewRequest(http.MethodGet, "/static/htmx.min.js", nil)
	req.Host = "evil.example.com"
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}
