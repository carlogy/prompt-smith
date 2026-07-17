package server

import (
	"net/http"

	"github.com/carlogy/prompt-smith/internal/prompt"
)

// registryResponse is the JSON shape of GET /api/registry: everything
// the page needs to render the skill picker and target selector.
type registryResponse struct {
	Categories []string    `json:"categories"`
	Targets    []targetDTO `json:"targets"`
	Skills     []skillDTO  `json:"skills"`
}

type targetDTO struct {
	ID string `json:"id"`
}

// skillDTO mirrors registry.Skill's picker-relevant fields, plus
// SupportedTargets precomputed via Registry.SupportsTarget - the same
// check `list -t` uses - so the page doesn't need target-support logic
// duplicated in JavaScript.
type skillDTO struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Category         string   `json:"category"`
	WhenToUse        string   `json:"whenToUse"`
	SupportedTargets []string `json:"supportedTargets"`
}

// handleRegistry serves the loaded registry (embedded skills merged
// with any user skills - see registry.Load) as JSON, with skills in
// the same canonical order SortSkills defines for every other surface
// (list, the TUI picker).
func (app *application) handleRegistry(w http.ResponseWriter, r *http.Request) {
	targetIDs := sortedTargetIDs(app.reg)
	skills := sortedSkills(app.reg)

	skillDTOs := make([]skillDTO, 0, len(skills))
	for _, sk := range skills {
		supported := make([]string, 0, len(targetIDs))
		for _, tid := range targetIDs {
			if app.reg.SupportsTarget(sk, tid) {
				supported = append(supported, tid)
			}
		}
		skillDTOs = append(skillDTOs, skillDTO{
			ID:               sk.ID,
			Name:             sk.Name,
			Category:         sk.Category,
			WhenToUse:        sk.WhenToUse,
			SupportedTargets: supported,
		})
	}

	targetDTOs := make([]targetDTO, 0, len(targetIDs))
	for _, id := range targetIDs {
		targetDTOs = append(targetDTOs, targetDTO{ID: id})
	}

	resp := registryResponse{
		Categories: app.reg.Categories,
		Targets:    targetDTOs,
		Skills:     skillDTOs,
	}
	if err := writeJSON(w, http.StatusOK, resp); err != nil {
		app.serverError(w, r, err)
	}
}

// buildRequest mirrors prompt.Inputs; kept as its own type (rather
// than decoding directly into prompt.Inputs) so the JSON contract can
// evolve independently of the domain type.
type buildRequest struct {
	Target       string   `json:"target"`
	Skills       []string `json:"skills"`
	Goal         string   `json:"goal"`
	Role         string   `json:"role"`
	Context      string   `json:"context"`
	Constraints  string   `json:"constraints"`
	OutputFormat string   `json:"outputFormat"`
}

// buildResponse is the JSON shape of POST /api/build for every
// outcome - a successful build, a malformed request, or a build-logic
// error (unknown target/skill) - so the page's live preview always
// gets back the same {prompt, error} shape regardless of status code,
// and never needs to branch on status to render something.
type buildResponse struct {
	Prompt string `json:"prompt"`
	Error  string `json:"error,omitempty"`
}

// handleBuild wraps prompt.Build - the same pure function the
// flag-only CLI path and the TUI's live preview already call - making
// that same call reachable over HTTP.
//
// A request-shape problem (malformed JSON, oversized body, unknown
// field) is reported as 400/413: the request itself was invalid. An
// unknown target/skill is a normal, expected outcome of live preview
// (the user hasn't finished picking valid values yet), so it's
// reported as 200 with the error in the body, not a 4xx.
func (app *application) handleBuild(w http.ResponseWriter, r *http.Request) {
	var req buildRequest
	if err := readJSON(w, r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	out, buildErr := prompt.Build(app.reg, prompt.Inputs{
		Target:       req.Target,
		Skills:       req.Skills,
		Goal:         req.Goal,
		Role:         req.Role,
		Context:      req.Context,
		Constraints:  req.Constraints,
		OutputFormat: req.OutputFormat,
	})

	resp := buildResponse{Prompt: out}
	if buildErr != nil {
		resp.Error = buildErr.Error()
	}

	if err := writeJSON(w, http.StatusOK, resp); err != nil {
		app.serverError(w, r, err)
	}
}
