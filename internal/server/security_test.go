package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// okHandler is the trivial next handler enforceLocalOnly wraps in
// these tests, so a pass-through is unambiguous: 200 with no body
// means the request cleared the middleware and reached the handler.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

func TestEnforceLocalOnly_HostChecks(t *testing.T) {
	cases := []struct {
		name       string
		host       string
		wantStatus int
	}{
		{"bare loopback IPv4", "127.0.0.1", http.StatusOK},
		{"loopback IPv4 with port", "127.0.0.1:54321", http.StatusOK},
		{"localhost", "localhost", http.StatusOK},
		{"localhost with port", "localhost:8080", http.StatusOK},
		{"IPv6 loopback with port (bracketed)", "[::1]:8080", http.StatusOK},
		{"a real domain", "evil.example.com", http.StatusForbidden},
		{"a real domain with port", "evil.example.com:80", http.StatusForbidden},
		{"httptest's own default host", "example.com", http.StatusForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Host = tc.host
			rec := httptest.NewRecorder()

			enforceLocalOnly(okHandler).ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d, body = %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestEnforceLocalOnly_OriginChecks(t *testing.T) {
	cases := []struct {
		name       string
		origin     string // "" means no Origin header at all
		wantStatus int
	}{
		{"no Origin header (a top-level GET navigation)", "", http.StatusOK},
		{"matching loopback origin", "http://127.0.0.1:54321", http.StatusOK},
		{"matching localhost origin", "http://localhost:54321", http.StatusOK},
		{"a real domain", "http://evil.example.com", http.StatusForbidden},
		{"opaque null origin", "null", http.StatusForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Host = "127.0.0.1" // valid, so only the Origin check is under test
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			rec := httptest.NewRecorder()

			enforceLocalOnly(okHandler).ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d, body = %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestEnforceLocalOnly_RejectionBodyIsJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "evil.example.com"
	rec := httptest.NewRecorder()

	enforceLocalOnly(okHandler).ServeHTTP(rec, req)

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json (client should never need to guess the error shape)", ct)
	}
}

// TestRoutes_RejectsNonLocalHost is the integration check: confirms
// enforceLocalOnly is actually wired into app.routes(), not just
// correct in isolation.
func TestRoutes_RejectsNonLocalHost(t *testing.T) {
	app := testApp()
	req := httptest.NewRequest(http.MethodGet, "/api/registry", nil)
	req.Host = "evil.example.com"
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d, body = %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}
