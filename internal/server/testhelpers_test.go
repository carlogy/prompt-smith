package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"

	"github.com/carlogy/prompt-smith/internal/prompt"
	"github.com/carlogy/prompt-smith/internal/registry"
)

// testApp builds an application against a small synthetic registry -
// same fixture style as internal/registry's own tests - plus a
// discard logger so tests never spam output for expected error paths
// (e.g. a 500 test), and no seeded initial values.
func testApp() *application {
	return testAppWithInitial(prompt.Inputs{})
}

// testAppWithInitial is testApp, but with a custom seed for the index
// page - used by tests that verify --ui's initial-value seeding (see
// page_test.go).
func testAppWithInitial(initial prompt.Inputs) *application {
	reg := &registry.Registry{
		Categories: []string{"debugging", "testing"},
		Skills: []registry.Skill{
			{ID: "diagnose", Name: "Diagnose", Category: "debugging", Order: 10, WhenToUse: "Hard bugs.", Body: "Build a feedback loop first."},
			{ID: "verify", Name: "Verify", Category: "testing", Order: 10, WhenToUse: "Before done.", Body: "Run the checks."},
			{ID: "agent-only", Name: "Agent Only", Category: "testing", Order: 20, WhenToUse: "Agent harnesses only."}, // no Body
		},
		Targets: map[string]registry.TargetConfig{
			"generic":  {ID: "generic", SkillMode: "inline"},
			"opencode": {ID: "opencode", SkillMode: "reference"},
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	app, err := newApplication(reg, logger, initial)
	if err != nil {
		// The embedded template is committed alongside this code, so a
		// parse failure here is a build-time bug, not a runtime
		// condition any test is meant to exercise - every test in this
		// package would fail immediately regardless.
		panic(err)
	}
	return app
}

// newLocalRequest builds a request the way a real browser hitting this
// loopback server would: Host set to a loopback hostname, since
// enforceLocalOnly (security.go) rejects anything else, and
// httptest.NewRequest's own default ("example.com") is exactly the
// kind of host that middleware exists to reject. Tests that need to
// exercise enforceLocalOnly itself build a request directly instead of
// through this helper.
func newLocalRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	req.Host = "127.0.0.1"
	return req
}
