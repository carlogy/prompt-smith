package server

import (
	"net/http"

	"github.com/carlogy/prompt-smith/internal/fielddesc"
)

// indexPageData is what index.html (see assets/templates/index.html) renders
// from.
type indexPageData struct {
	Categories   []categoryData
	Targets      []targetOptionData
	Initial      initialData
	AdvancedOpen bool // pre-expand the optional-fields <details> when any were seeded
	Hints        fieldHints
}

type categoryData struct {
	Name   string
	Skills []skillOptionData
}

type skillOptionData struct {
	ID        string
	Name      string
	WhenToUse string
	Checked   bool
	// SupportedTargets is every target id this skill renders on (see
	// Registry.SupportsTarget) - the same check `list -t` and the TUI
	// picker use to hide unsupported skills. index.html renders this
	// as a data attribute and greys/disables the row client-side when
	// the selected target isn't in the list, since target changes
	// happen entirely in the browser (no round trip to re-render this
	// page).
	SupportedTargets []string
}

type targetOptionData struct {
	ID       string
	Name     string
	Selected bool
}

// initialData is app.initial's picker-relevant fields, reshaped for
// the template - Skills becomes per-skillOptionData.Checked above
// rather than being rendered directly.
type initialData struct {
	Goal         string
	Role         string
	Context      string
	Constraints  string
	OutputFormat string
}

// fieldHints carries the canonical descriptive sentence (see
// internal/fielddesc) for each field, so index.html renders its hints
// from data instead of hardcoding them - the same sentences the TUI's
// footer descriptor shows (see tui/view.go's footerHelpFor).
type fieldHints struct {
	Target       string
	Goal         string
	Role         string
	Context      string
	Constraints  string
	OutputFormat string
}

// handleIndex serves the page: the same skill/category/target data
// handleRegistry exposes as JSON, rendered server-side instead, with
// app.initial (seeded from --ui's flags/args - see cli's runUI)
// pre-filling the form. Live preview is the form's own htmx wiring,
// posting to handlePreview on every change (see preview.go); this
// initial render doesn't need to call it.
func (app *application) handleIndex(w http.ResponseWriter, r *http.Request) {
	initialSkills := make(map[string]bool, len(app.initial.Skills))
	for _, id := range app.initial.Skills {
		initialSkills[id] = true
	}

	targetIDs := sortedTargetIDs(app.reg)

	byCategory := make(map[string][]skillOptionData)
	for _, sk := range sortedSkills(app.reg) {
		supported := make([]string, 0, len(targetIDs))
		for _, tid := range targetIDs {
			if app.reg.SupportsTarget(sk, tid) {
				supported = append(supported, tid)
			}
		}
		byCategory[sk.Category] = append(byCategory[sk.Category], skillOptionData{
			ID:               sk.ID,
			Name:             sk.Name,
			WhenToUse:        sk.WhenToUse,
			Checked:          initialSkills[sk.ID],
			SupportedTargets: supported,
		})
	}

	categories := make([]categoryData, 0, len(app.reg.Categories))
	for _, cat := range app.reg.Categories {
		if skills, ok := byCategory[cat]; ok {
			categories = append(categories, categoryData{Name: cat, Skills: skills})
		}
	}

	targets := make([]targetOptionData, 0, len(targetIDs))
	for _, id := range targetIDs {
		targets = append(targets, targetOptionData{
			ID:       id,
			Name:     app.reg.Targets[id].DisplayName(),
			Selected: id == app.initial.Target,
		})
	}

	data := indexPageData{
		Categories: categories,
		Targets:    targets,
		Initial: initialData{
			Goal:         app.initial.Goal,
			Role:         app.initial.Role,
			Context:      app.initial.Context,
			Constraints:  app.initial.Constraints,
			OutputFormat: app.initial.OutputFormat,
		},
		AdvancedOpen: app.initial.Role != "" || app.initial.Context != "" ||
			app.initial.Constraints != "" || app.initial.OutputFormat != "",
		Hints: fieldHints{
			Target:       fielddesc.Sentence(fielddesc.Target),
			Goal:         fielddesc.Sentence(fielddesc.Goal),
			Role:         fielddesc.Sentence(fielddesc.Role),
			Context:      fielddesc.Sentence(fielddesc.Context),
			Constraints:  fielddesc.Sentence(fielddesc.Constraints),
			OutputFormat: fielddesc.Sentence(fielddesc.OutputFormat),
		},
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := app.tmpl.ExecuteTemplate(w, "index.html", data); err != nil {
		app.serverError(w, r, err)
	}
}
