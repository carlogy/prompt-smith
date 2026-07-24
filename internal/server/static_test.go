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
	"mime"
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

// TestStaticHandler_ForcesJavaScriptMIME proves newStaticHandler's
// mime.AddExtensionType override actually wins regardless of ambient
// mime state - this is what makes .js content type deterministic
// across OSes (see newStaticHandler), rather than that just happening
// to be true today on whatever OS runs this test. It seeds the exact
// hostile value real Windows registries commonly report for ".js"
// (see the comment in newStaticHandler) before building the app, so
// this fails without the override and passes with it, on any OS.
func TestStaticHandler_ForcesJavaScriptMIME(t *testing.T) {
	if err := mime.AddExtensionType(".js", "application/javascript"); err != nil {
		t.Fatalf("seeding a hostile .js mime type: %v", err)
	}

	app := testApp() // newApplication -> newStaticHandler must re-force text/javascript
	req := newLocalRequest(http.MethodGet, "/static/htmx.min.js", nil)
	rec := httptest.NewRecorder()

	app.routes().ServeHTTP(rec, req)

	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/javascript") {
		t.Errorf("Content-Type = %q, want text/javascript... even with a pre-seeded application/javascript mapping", ct)
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
	if !strings.Contains(body, "cornflower") {
		t.Errorf("served app.css doesn't reference the cornflower theme color, got len=%d", len(body))
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
