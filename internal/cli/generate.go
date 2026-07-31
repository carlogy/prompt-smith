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

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/atotto/clipboard"
	"github.com/spf13/cobra"

	"github.com/carlogy/prompt-smith/internal/preset"
	"github.com/carlogy/prompt-smith/internal/prompt"
	"github.com/carlogy/prompt-smith/internal/promptlint"
	"github.com/carlogy/prompt-smith/internal/registry"
	"github.com/carlogy/prompt-smith/internal/server"
	"github.com/carlogy/prompt-smith/internal/tui"
)

// copyToClipboard puts text on the system clipboard. A package variable
// so tests can substitute a spy instead of touching the real clipboard.
var copyToClipboard = clipboard.WriteAll

// runTUIFunc launches the interactive skill picker. A package variable
// so tests can substitute a spy instead of starting a real Bubble Tea
// program (which would block reading real stdin).
var runTUIFunc = tui.Run

// runServerFunc launches the local web UI (see --ui) and blocks until
// ctx is done. A package variable, same reasoning as runTUIFunc: tests
// substitute a spy so they never bind a real port or open a browser.
var runServerFunc = server.Serve

// errEmptyGoal is returned when no goal text was given.
var errEmptyGoal = errors.New(`promptsmith: a goal is required, e.g. promptsmith "fix the flaky test" or promptsmith -g "fix the flaky test"`)

// errGoalConflict is returned when a goal was given both via --goal and
// as positional args: see resolveGoal for why these stay mutually
// exclusive rather than one silently winning.
var errGoalConflict = errors.New("promptsmith: --goal and a positional goal are mutually exclusive; pass the goal one of the two ways")

// errUnknownTarget mirrors prompt.Build's own "unknown target" error
// (internal/prompt/build.go) byte-for-byte, deliberately. It exists so
// runGenerate can reject a bogus -t BEFORE launching either interactive
// route (see the --ui and useTUI branches below), rather than letting
// prompt.Build discover it once the user has already committed to an
// action inside the picker or the web UI. cli already imports prompt
// (see prompt.Build and prompt.Inputs used throughout this file), so
// importing isn't the obstacle here. The obstacle is that prompt has
// no standalone target-validation entry point - the only way to reach
// this error today is prompt.Build itself, which demands a full set of
// inputs and can fail for other reasons too, so it can't double as a
// cheap pre-flight check. Duplicating the format string is what keeps
// the early rejection byte-identical to what the non-interactive path
// already emits; keep this comment and prompt.Build's error in sync by
// hand if either changes. Exporting a validator from prompt would
// remove the duplication and is the natural cleanup if this string
// ever needs a third call site.
func errUnknownTarget(target string) error {
	return fmt.Errorf("prompt: unknown target %q", target)
}

// generateOptions holds the root command's flag values.
type generateOptions struct {
	target       string
	skills       []string
	goal         string
	context      string
	constraints  string
	role         string
	outputFormat string
	examples     []string
	toClipboard  bool
	out          string
	quick        bool
	tui          bool
	noHints      bool
	ui           bool
	port         int
	noBrowser    bool
	preset       string
	savePreset   string
	force        bool
}

// addGenerateFlags registers the generate flags on cmd and wires its RunE.
func addGenerateFlags(cmd *cobra.Command, reg *registry.Registry) {
	opts := &generateOptions{}

	cmd.Flags().StringVarP(&opts.target, "target", "t", "generic", "target harness: generic|opencode|claude-code|gemini-cli|codex")
	cmd.Flags().StringSliceVarP(&opts.skills, "skills", "s", nil, "skills to include (comma-separated or repeatable)")
	cmd.Flags().StringVarP(&opts.goal, "goal", "g", "", "the goal/task (alternative to passing it as a positional argument)")
	cmd.Flags().StringVarP(&opts.context, "context", "x", "", "background/context for the goal")
	cmd.Flags().StringVarP(&opts.constraints, "constraints", "c", "", "constraints the solution must respect")
	cmd.Flags().StringVarP(&opts.role, "role", "r", "", "role/persona to open the prompt with")
	cmd.Flags().StringVarP(&opts.outputFormat, "output-format", "f", "", "desired shape of the response")
	// StringArrayVarP, not StringSliceVarP: StringSlice CSV-splits its
	// value on every comma, and a worked example is extremely likely to
	// contain one ("input: a, b, c -> output: 3") - that would silently
	// fragment into multiple broken examples with no error. StringArray
	// appends each occurrence verbatim instead. This repo has already
	// shipped exactly that bug once with --skills (see NormalizeSkills's
	// doc comment); --example doesn't get to repeat it.
	cmd.Flags().StringArrayVarP(&opts.examples, "example", "e", nil, "a worked example of the desired output (repeatable)")
	cmd.Flags().StringVarP(&opts.preset, "preset", "p", "", "load prompt defaults from a saved preset (see `promptsmith presets`)")
	cmd.Flags().StringVar(&opts.savePreset, "save-preset", "", "save the resolved flags (after any -p/--preset merge) as a new preset under this name")
	// No shorthand for --force: -f is already taken by --output-format
	// above, and cobra panics at init if two flags on the same command
	// register the same shorthand. Don't "fix" this by giving --force
	// a shorthand later without checking that first.
	cmd.Flags().BoolVar(&opts.force, "force", false, "with --save-preset, overwrite an existing preset of the same name")
	cmd.Flags().BoolVarP(&opts.toClipboard, "copy", "y", false, "copy the prompt to the clipboard instead of stdout")
	cmd.Flags().StringVarP(&opts.out, "out", "o", "", "write the prompt to this file instead of stdout")
	cmd.Flags().BoolVarP(&opts.quick, "quick", "q", false, "never launch the interactive picker, even in a terminal")
	cmd.Flags().BoolVar(&opts.tui, "tui", false, "launch the interactive picker even if --skills was given")
	cmd.Flags().BoolVar(&opts.noHints, "no-hints", false, "suppress prompt-quality hints on stderr")
	cmd.Flags().BoolVar(&opts.ui, "ui", false, "launch the local web UI in your browser")
	cmd.Flags().IntVar(&opts.port, "port", 0, "port for --ui to bind (default: an OS-assigned free port)")
	cmd.Flags().BoolVar(&opts.noBrowser, "no-browser", false, "with --ui, don't automatically open a browser")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runGenerate(cmd, reg, opts, args)
	}
}

// presetFieldSpecs states, once, how each Preset field maps onto opts
// in BOTH directions, and onto the flag whose explicit use should take
// precedence over it. Table-driven so the mapping is stated in exactly
// one place: a reviewer can check all seven flag names in one pass
// instead of hunting through applyPreset's and the save path's bodies
// separately, and a mistyped flagName here is what
// TestApplyPreset_ExplicitFlagBeatsPreset's per-field cases would
// catch (a bad name makes cmd.Flags().Changed(name) always report
// false - see pflag's Changed - so the preset would keep clobbering an
// explicit flag for that field instead of yielding to it).
//
// apply is the preset->opts direction, used by applyPreset when
// loading -p/--preset. It also skips a preset field that's
// empty/nil: a preset.Preset has no way to distinguish "the YAML
// omitted this key" from "the key was explicitly set to its zero
// value" (see presetDoc), so treating an omitted field as a no-op is
// the only reading that doesn't clobber a flag's own default with an
// empty string - --target's default is "generic" (see
// addGenerateFlags), so a preset that only sets, say, role would
// otherwise blank out the target to "" and fail with "unknown target
// \"\"".
//
// collect is the opts->preset direction, used by
// collectPresetFromOpts when saving via --save-preset. It's
// deliberately value-based: it copies opts's current value across
// whenever it's non-empty, and is NEVER gated on
// cmd.Flags().Changed. Two consequences follow, both intended rather
// than oversights:
//
//   - --target defaults to "generic" and so is never empty, which
//     means the saved preset always carries a target: key, even when
//     the user never typed --target. A preset that omitted target
//     would leave a future load falling back to whatever the loader
//     happens to default to, silently, instead of recording what
//     generation actually used.
//   - Because the save (see runGenerate) runs AFTER applyPreset, a
//     command like `promptsmith -p base --save-preset derived`
//     correctly inherits every field base supplied into derived, not
//     only the ones typed directly on this invocation. Gating collect
//     on Changed would only copy across flags the user just typed on
//     THIS command line and would silently drop values a loaded
//     preset had already merged into opts.
//
// fromInputs is a third, later-added direction: prompt.Inputs->preset,
// used by collectPresetFromInputs when the TUI picker returns
// tui.ActionSavePreset. It exists as a third leg on this SAME table,
// not as a standalone function elsewhere, for the identical reason the
// other two directions are table entries rather than separate
// hand-written mappings: the seven-field mapping is stated exactly
// once, so a reviewer checks every direction in one pass instead of
// hunting a second (or third) table. It's needed at all because
// result.Inputs, not opts, is authoritative once the picker returns -
// the picker lets the user edit role/output_format/examples after they
// were seeded from opts, so opts is stale the moment the picker
// returns (this is the same reason runInteractive's lint pass reads
// result.Inputs rather than opts). Like collect, it's value-based
// (non-empty check, never gated on anything resembling Changed - a
// prompt.Inputs has no flag-parse state to gate on in the first
// place).
var presetFieldSpecs = []struct {
	flagName   string
	apply      func(opts *generateOptions, p *preset.Preset)
	collect    func(opts *generateOptions, p *preset.Preset)
	fromInputs func(in prompt.Inputs, p *preset.Preset)
}{
	{"target",
		func(opts *generateOptions, p *preset.Preset) {
			if p.Target != "" {
				opts.target = p.Target
			}
		},
		func(opts *generateOptions, p *preset.Preset) {
			if opts.target != "" {
				p.Target = opts.target
			}
		},
		func(in prompt.Inputs, p *preset.Preset) {
			if in.Target != "" {
				p.Target = in.Target
			}
		},
	},
	{"skills",
		func(opts *generateOptions, p *preset.Preset) {
			if len(p.Skills) > 0 {
				opts.skills = p.Skills
			}
		},
		func(opts *generateOptions, p *preset.Preset) {
			if len(opts.skills) > 0 {
				p.Skills = opts.skills
			}
		},
		func(in prompt.Inputs, p *preset.Preset) {
			if len(in.Skills) > 0 {
				p.Skills = in.Skills
			}
		},
	},
	{"role",
		func(opts *generateOptions, p *preset.Preset) {
			if p.Role != "" {
				opts.role = p.Role
			}
		},
		func(opts *generateOptions, p *preset.Preset) {
			if opts.role != "" {
				p.Role = opts.role
			}
		},
		func(in prompt.Inputs, p *preset.Preset) {
			if in.Role != "" {
				p.Role = in.Role
			}
		},
	},
	{"context",
		func(opts *generateOptions, p *preset.Preset) {
			if p.Context != "" {
				opts.context = p.Context
			}
		},
		func(opts *generateOptions, p *preset.Preset) {
			if opts.context != "" {
				p.Context = opts.context
			}
		},
		func(in prompt.Inputs, p *preset.Preset) {
			if in.Context != "" {
				p.Context = in.Context
			}
		},
	},
	{"constraints",
		func(opts *generateOptions, p *preset.Preset) {
			if p.Constraints != "" {
				opts.constraints = p.Constraints
			}
		},
		func(opts *generateOptions, p *preset.Preset) {
			if opts.constraints != "" {
				p.Constraints = opts.constraints
			}
		},
		func(in prompt.Inputs, p *preset.Preset) {
			if in.Constraints != "" {
				p.Constraints = in.Constraints
			}
		},
	},
	// "output-format", NOT the YAML key "output_format": Changed()
	// looks flags up by their cobra flag name (hyphenated), not by the
	// preset file's key.
	{"output-format",
		func(opts *generateOptions, p *preset.Preset) {
			if p.OutputFormat != "" {
				opts.outputFormat = p.OutputFormat
			}
		},
		func(opts *generateOptions, p *preset.Preset) {
			if opts.outputFormat != "" {
				p.OutputFormat = opts.outputFormat
			}
		},
		func(in prompt.Inputs, p *preset.Preset) {
			if in.OutputFormat != "" {
				p.OutputFormat = in.OutputFormat
			}
		},
	},
	// "example", singular: the flag is -e/--example even though both
	// the preset field and the opts field are plural (Examples/examples).
	{"example",
		func(opts *generateOptions, p *preset.Preset) {
			if len(p.Examples) > 0 {
				opts.examples = p.Examples
			}
		},
		func(opts *generateOptions, p *preset.Preset) {
			if len(opts.examples) > 0 {
				p.Examples = opts.examples
			}
		},
		func(in prompt.Inputs, p *preset.Preset) {
			if len(in.Examples) > 0 {
				p.Examples = in.Examples
			}
		},
	},
}

// applyPreset loads the preset named by -p/--preset, if given, and uses
// it to fill in any generate flag the caller did not explicitly pass.
//
// Gated on cmd.Flags().Changed("preset"), not opts.preset != "": that
// way `--preset ""` still reaches preset.Load (and its name-validation
// error), rather than being silently treated as "no preset requested".
//
// Each field from presetFieldSpecs is applied only if
// !cmd.Flags().Changed(<its flag>) - never on an empty-value check.
// --target defaults to "generic" (see addGenerateFlags above), so an
// empty check can't distinguish "the user never passed --target" from
// "the user explicitly passed --target generic"; gating on Changed is
// the only way a preset doesn't wrongly override an explicit
// --target generic.
func applyPreset(cmd *cobra.Command, opts *generateOptions) error {
	if !cmd.Flags().Changed("preset") {
		return nil
	}

	p, warnings, err := preset.Load(opts.preset)
	for _, w := range warnings {
		fmt.Fprintln(cmd.ErrOrStderr(), "promptsmith: "+w)
	}
	if err != nil {
		if errors.Is(err, preset.ErrNotFound) {
			// ListDir's own warnings matter here too: this is how a
			// user who created "foo.yml" instead of "foo.yaml" finds
			// out why their preset isn't in the list.
			names, listWarnings, _ := preset.ListDir()
			for _, w := range listWarnings {
				fmt.Fprintln(cmd.ErrOrStderr(), "promptsmith: "+w)
			}
			return unknownPresetError(opts.preset, names)
		}
		return fmt.Errorf("promptsmith: %w", err)
	}

	for _, spec := range presetFieldSpecs {
		if !cmd.Flags().Changed(spec.flagName) {
			spec.apply(opts, p)
		}
	}
	return nil
}

// collectPresetFromOpts builds a *preset.Preset out of opts's current
// values by running every collect func in presetFieldSpecs, so the
// fields the saved preset carries are governed by the exact same
// seven-entry table applyPreset reads in the opposite direction - see
// presetFieldSpecs's doc comment for the collect direction's
// value-based semantics (non-empty check, no Changed gating).
func collectPresetFromOpts(opts *generateOptions) *preset.Preset {
	p := &preset.Preset{}
	for _, spec := range presetFieldSpecs {
		spec.collect(opts, p)
	}
	return p
}

// collectPresetFromInputs builds a *preset.Preset out of in's values by
// running every fromInputs func in presetFieldSpecs - the third leg of
// the same seven-entry table collectPresetFromOpts reads for
// --save-preset. Used by runInteractive when the picker returns
// tui.ActionSavePreset: result.Inputs, not opts, is authoritative once
// the picker returns (see presetFieldSpecs's fromInputs doc comment),
// so the save-as-preset path has to build its Preset from Inputs rather
// than reusing collectPresetFromOpts against stale opts.
func collectPresetFromInputs(in prompt.Inputs) *preset.Preset {
	p := &preset.Preset{}
	for _, spec := range presetFieldSpecs {
		spec.fromInputs(in, p)
	}
	return p
}

// saveGeneratedPreset implements --save-preset: it builds a
// *preset.Preset from opts (see collectPresetFromOpts) and writes it
// via preset.Save, then confirms the full written path on stderr.
// Stderr, not stdout, mirrors copyAndConfirm's reasoning for --copy:
// with a goal present, stdout carries the generated prompt and has to
// stay clean enough to pipe.
func saveGeneratedPreset(cmd *cobra.Command, opts *generateOptions) error {
	p := collectPresetFromOpts(opts)
	path, err := preset.Save(opts.savePreset, p, opts.force)
	if err != nil {
		return fmt.Errorf("promptsmith: %w", err)
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "promptsmith: saved preset %q to %s\n", opts.savePreset, path)
	return nil
}

// savePresetFromInputs implements tui.ActionSavePreset: it builds a
// *preset.Preset from result.Inputs (see collectPresetFromInputs) and
// writes it via preset.Save, confirming the full written path on
// stderr - the same shape as saveGeneratedPreset above, just sourced
// from the picker's authoritative Inputs instead of opts. See
// runInteractive's call site for why this runs before prompt.Build.
//
// result.OverwritePreset is passed straight through as force, never
// hardcoded true: it's set only when the user explicitly confirmed
// overwriting an existing preset inside the picker (see tui.Result's
// doc comment). Hardcoding true here would let a preset created
// between the picker's own existence check and this save get silently
// clobbered - exactly the unrecoverable loss preset.Save's
// refuse-without-force default exists to prevent.
//
// Deliberately does NOT also print the assembled prompt to stdout the
// way tui.ActionStdout does. The two additive flag-only paths this
// mirrors most closely - --save-preset alongside a goal (see
// runGenerate's savePresetRequested handling) - are additive because
// --save-preset is a flag layered onto an otherwise-unchanged generate
// invocation and has to coexist with whatever delivery that invocation
// already requested. The TUI has no equivalent "rest of the
// invocation" to coexist with: deliver's own doc comment already notes
// the picker "offers exactly one action per confirm", unlike the
// flag-only path's additive --copy/--out. The user picked "save this
// as a reusable preset" AS their one action, not "save it, and also
// show me the prompt" - so this returns as soon as the save succeeds.
func savePresetFromInputs(cmd *cobra.Command, result tui.Result) error {
	p := collectPresetFromInputs(result.Inputs)
	path, err := preset.Save(result.PresetName, p, result.OverwritePreset)
	if err != nil {
		return fmt.Errorf("promptsmith: %w", err)
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "promptsmith: saved preset %q to %s\n", result.PresetName, path)
	return nil
}

func runGenerate(cmd *cobra.Command, reg *registry.Registry, opts *generateOptions, args []string) error {
	if err := validateUIFlags(cmd, opts); err != nil {
		return err
	}
	if err := validateForceFlag(cmd, opts); err != nil {
		return err
	}

	// Resolved immediately after validateUIFlags and, critically,
	// BEFORE NormalizeSkills below: decideUseTUI, warnStraySkillArgs,
	// and NormalizeSkills all key off len(opts.skills), and
	// decideUseTUI in particular treats zero skills as "launch the
	// interactive picker" (see interactive.go). A preset's skills need
	// to already be in opts.skills before that count is taken, so that
	// `promptsmith -p mypreset "goal"` counts exactly as if those
	// skills had been typed via --skills and goes straight to output -
	// applied any later, a preset would silently fail to suppress the
	// picker (or the goal-only stderr note below).
	if err := applyPreset(cmd, opts); err != nil {
		return err
	}

	// Normalized here, at the CLI boundary, rather than deferred to
	// prompt.Build: decideUseTUI and the no-skills note below both
	// branch on len(opts.skills), and an unnormalized ["architect", ""]
	// (from an unquoted "-s architect, ") would skew both counts even
	// though only one skill actually resolves.
	opts.skills = prompt.NormalizeSkills(opts.skills)

	// Normalized here too, for the same reason: prompt.Build would
	// normalize on its own, but normalizing at the boundary means the
	// TUI picker and the web UI form are seeded with already-clean
	// values (see runInteractive/runUI below), and Phase 3's lint rule
	// that counts examples needs that count to be accurate rather than
	// inflated by stray whitespace-only entries.
	opts.examples = prompt.NormalizeExamples(opts.examples)

	// Runs before resolveGoal deliberately: if someone writes
	// `-g "goal" -s a, b`, the actionable skill-list hint should print
	// before the goal-conflict error below aborts the command, not be
	// swallowed by it.
	warnStraySkillArgs(cmd.ErrOrStderr(), reg, opts.skills, args)

	goal, err := resolveGoal(opts.goal, args)
	if err != nil {
		return err
	}

	// Gated on cmd.Flags().Changed("save-preset"), not
	// opts.savePreset != "" - the same reasoning as applyPreset's own
	// gate on Changed("preset"): --save-preset "" must still reach
	// preset.Save's name-validation error, rather than being silently
	// treated as "no save requested".
	//
	// Placed HERE - after resolveGoal, but before the --ui branch
	// below - is load-bearing, not incidental: resolveGoal is where a
	// malformed invocation (--goal plus a positional goal, i.e.
	// errGoalConflict) errors out, and that has to happen BEFORE
	// anything touches the filesystem - never write a file for a
	// command that's about to fail. Running after applyPreset
	// (further up) is equally deliberate: it's what lets
	// `-p base --save-preset derived` inherit base's already-merged
	// values into the saved preset - see presetFieldSpecs's
	// collect-direction doc comment.
	savePresetRequested := cmd.Flags().Changed("save-preset")
	if savePresetRequested {
		if err := saveGeneratedPreset(cmd, opts); err != nil {
			return err
		}
	}

	// An unknown target must be rejected HERE, before either interactive
	// route below ever opens - not deferred to prompt.Build. The
	// non-interactive path already gets this for free (Build runs at
	// the bottom of this function and errors out), but --ui and --tui
	// return early and never reach it. Left unchecked, a bogus -t would
	// launch the web UI or the picker anyway: registry.SupportsTarget
	// (see internal/registry/registry.go) returns false for ANY
	// unknown target, so every skill row in the picker would just
	// render disabled with no explanation, and the web UI's Initial
	// would seed a form nothing can ever submit successfully. See
	// errUnknownTarget below for why the message is duplicated from
	// prompt.Build rather than imported.
	if opts.ui {
		if !reg.HasTarget(opts.target) {
			return errUnknownTarget(opts.target)
		}
		return runUI(cmd, reg, opts, goal)
	}

	// --save-preset with no goal is already a complete, successful
	// command (the preset was saved above) and must NOT fall through
	// into decideUseTUI's goalEmpty branch below - opening the
	// interactive skill picker because someone asked to save a preset
	// would be a surprising, unrelated side effect. The !opts.tui half
	// of the condition matters: this guard only suppresses the
	// IMPLICIT empty-goal reason for launching the picker. An explicit
	// --tui is the user directly asking for it, and swallowing that
	// would be its own surprise - so `--save-preset name --tui` (still
	// no goal) falls through to decideUseTUI below, which opens the
	// picker anyway via forceTUI. Net effect: saving is additive with
	// every other mode (goal: saves and generates; --ui: saves and
	// serves; --tui: saves and opens the picker) rather than
	// introducing any new mutual exclusion.
	if savePresetRequested && goal == "" && !opts.tui {
		return nil
	}

	useTUI, err := decideUseTUI(isInteractive(), opts.quick, opts.tui, len(opts.skills), goal == "")
	if err != nil {
		return err
	}

	if useTUI {
		// Same guard as the --ui branch above, and for the identical
		// reason: decideUseTUI only looks at flags/counts, never the
		// registry, so it has no way to catch a bogus -t on its own.
		if !reg.HasTarget(opts.target) {
			return errUnknownTarget(opts.target)
		}
		// goal may be empty here (bare `promptsmith`): the picker
		// collects it inline, focused on the goal field by default.
		return runInteractive(cmd, reg, opts, goal)
	}

	if goal == "" {
		return errEmptyGoal
	}

	// Hoisted into a local rather than built twice (once for
	// prompt.Build, once for promptlint.Check below): those two calls
	// have to see the identical Inputs value, or the lint findings
	// could describe a prompt that isn't the one actually delivered.
	in := prompt.Inputs{
		Target:       opts.target,
		Skills:       opts.skills,
		Goal:         goal,
		Context:      opts.context,
		Constraints:  opts.constraints,
		Role:         opts.role,
		OutputFormat: opts.outputFormat,
		Examples:     opts.examples,
	}

	out, err := prompt.Build(reg, in)
	if err != nil {
		return err
	}

	// Note the goal-only fallback only once generation has actually
	// succeeded - an invalid target/skill should just error, not also
	// claim a goal-only prompt was generated. The parenthetical points
	// at both remaining ways to get skills into the prompt: --skills
	// itself, or the interactive picker (now shipped, not a future
	// promise) for anyone running in a terminal without --quick.
	if len(opts.skills) == 0 {
		fmt.Fprintln(cmd.ErrOrStderr(), "promptsmith: no --skills given; generating a goal-only prompt (pass --skills, or run in a terminal without --quick for the interactive picker)")
	}

	warnLintFindings(cmd.ErrOrStderr(), reg, in, opts.noHints)

	return deliver(cmd, opts, out)
}

// resolveGoal picks the goal from --goal or the positional args, which
// are mutually exclusive: silently merging them (or letting one win)
// hides the common mistake of an unquoted multi-word value leaking
// stray words into the goal.
func resolveGoal(flagGoal string, args []string) (string, error) {
	positional := strings.TrimSpace(strings.Join(args, " "))
	flagGoal = strings.TrimSpace(flagGoal)
	if flagGoal != "" && positional != "" {
		return "", errGoalConflict
	}
	if flagGoal != "" {
		return flagGoal, nil
	}
	return positional, nil
}

// warnStraySkillArgs flags the classic `-s a, b, c` mistake: the shell
// splits the spaced list, --skills only receives "a", and "b"/"c" land
// in the positional args where they're silently absorbed into the goal.
// Reported rather than rejected: a legitimate goal can name a skill in
// passing, so this stays a hint on stderr.
func warnStraySkillArgs(w io.Writer, reg *registry.Registry, skills, args []string) {
	if len(skills) == 0 || len(args) == 0 {
		return
	}

	selected := make(map[string]bool, len(skills))
	for _, id := range skills {
		selected[id] = true
	}

	var strays []string
	seen := make(map[string]bool)
	for _, arg := range args {
		candidate := strings.TrimSpace(arg)
		candidate = strings.TrimSuffix(candidate, ",")
		candidate = strings.TrimSpace(candidate)
		if candidate == "" || selected[candidate] {
			continue
		}
		if _, ok := reg.SkillByID(candidate); !ok {
			continue
		}
		if seen[candidate] {
			continue
		}
		seen[candidate] = true
		strays = append(strays, candidate)
	}

	if len(strays) == 0 {
		return
	}
	fmt.Fprintf(w, "promptsmith: warning: parsed as goal text, not skills: %s; --skills takes a comma-separated list with no spaces (e.g. -s a,b,c)\n", strings.Join(strays, ", "))
}

// warnLintFindings renders promptlint.Check's advisory findings for in
// as hints on w, unless noHints suppresses all of it. Mirrors
// warnStraySkillArgs's shape - a raw io.Writer rather than a
// *cobra.Command - for the same reason: both call sites (runGenerate,
// runInteractive) already hold a plain io.Writer (cmd.ErrOrStderr())
// and passing the whole *cobra.Command through would be a wider
// dependency than either function needs.
//
// Three of promptlint's seven RuleIDs - RuleNoRole,
// RuleNoOutputFormat, and RuleNoExamples - are "pure absence"
// findings: each one only ever says "you left X out". They're
// collapsed into a single line below instead of one line apiece,
// because without the collapse the single simplest command a
// first-time user runs - a bare `promptsmith "some goal"` - trips all
// three simultaneously, and would print three lines of unsolicited
// advice stacked on top of the existing "no --skills given" note
// (runGenerate's own hint, not a lint finding) that already fires on
// that same command: four lines of hints on someone's very first
// invocation. These three rules all answer one question - "what did
// you leave out?" - and read naturally as one joined sentence,
// whereas every other rule (negative constraints, short goal,
// oversized prompt) is a distinct judgment that needs its own
// explanation and earns its own line.
//
// The web UI deliberately does NOT collapse these three: it has room
// to list each finding on its own. This per-surface divergence has
// direct precedent in this repo: internal/fielddesc's package comment
// keeps the CLI's flag help and the TUI's placeholders as their own,
// terser, local strings "since those have different space budgets and
// voices" - the same reasoning applies here, one level up, between
// the CLI's compact stderr hints and the web UI's roomier findings
// list.
func warnLintFindings(w io.Writer, reg *registry.Registry, in prompt.Inputs, noHints bool) {
	if noHints {
		return
	}
	findings := promptlint.Check(reg, in)

	// First pass: collect the missing field names in the order
	// Check itself returns them (role, then output_format, then
	// examples - see promptlint.Check's documented ordering
	// contract), so the collapsed sentence below always lists them in
	// that fixed order no matter which subset fired.
	var missingFields []string
	for _, f := range findings {
		switch f.Rule {
		case promptlint.RuleNoRole:
			missingFields = append(missingFields, "role")
		case promptlint.RuleNoOutputFormat:
			missingFields = append(missingFields, "output_format")
		case promptlint.RuleNoExamples:
			missingFields = append(missingFields, "examples")
		}
	}

	// Second pass: render in Check's own order, emitting each
	// non-collapsible finding as its own line as it's reached, and
	// emitting the one collapsed line at the position of the FIRST
	// collapsible finding encountered. This single pass is only
	// correct because RuleNoRole, RuleNoOutputFormat, and
	// RuleNoExamples are contiguous in Check's documented return order
	// (negative-constraints, no-role, no-output_format, examples,
	// short-goal, oversized-prompt) - if Check is ever reordered so
	// the three are no longer adjacent, this loop would need to become
	// two passes instead of relying on "first encountered" as a proxy
	// for "the group's position".
	collapsedPrinted := false
	for _, f := range findings {
		switch f.Rule {
		case promptlint.RuleNoRole, promptlint.RuleNoOutputFormat, promptlint.RuleNoExamples:
			if !collapsedPrinted {
				fmt.Fprintln(w, collapsedAbsenceHint(missingFields))
				collapsedPrinted = true
			}
		default:
			fmt.Fprintf(w, "promptsmith: hint: %s\n", lowercaseFirstRune(f.Message))
		}
	}
}

// lowercaseFirstRune returns s with only its first rune lowercased;
// every other byte, including any digit or quoted identifier further
// in, is left untouched.
//
// This exists on the CLI side specifically, not as a change to
// promptlint.Finding.Message itself: Message is documented as a
// complete, capitalized, standalone sentence, which is exactly right
// when the web UI renders it as its own <li> (see previewData's
// Findings field in internal/server/preview.go) - a capitalized
// sentence there needs no adjustment. It's only the CLI's stderr
// rendering that inlines the message after a lowercase "promptsmith:
// hint: " prefix, where every other stderr line in this repo continues
// in lowercase (see errEmptyGoal, warnStraySkillArgs, the
// no-skills note above, copyAndConfirm's confirmation, and Go's own
// convention for error/log strings). One canonical Message, adjusted
// only by the one surface whose voice needs it - the same
// per-surface-voice principle recorded in internal/fielddesc's package
// comment for the CLI/TUI split.
//
// Implemented with utf8.DecodeRuneInString rather than lowercasing
// s[0] as a byte: byte-indexing would corrupt a multibyte leading
// rune. No current Finding.Message starts with one, but this function
// has no way to assume that stays true.
func lowercaseFirstRune(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	return string(unicode.ToLower(r)) + s[size:]
}

// collapsedAbsenceHint renders the single line naming which of role,
// output_format, and examples were missing, in that fixed order.
// Joined with a natural-language "or" (and an Oxford comma once
// there are three), with singular/plural agreement on "it"/"them",
// and pointing at the docs the same way presets.go's
// noPresetsGuidance does for the "Presets" section.
func collapsedAbsenceHint(fields []string) string {
	var joined string
	switch len(fields) {
	case 1:
		joined = fields[0]
	case 2:
		joined = fields[0] + " or " + fields[1]
	default:
		joined = strings.Join(fields[:len(fields)-1], ", ") + ", or " + fields[len(fields)-1]
	}
	pronoun := "it"
	if len(fields) > 1 {
		pronoun = "them"
	}
	return fmt.Sprintf(`promptsmith: hint: no %s given; adding %s measurably improves output (see the "Hints" section of README.md)`, joined, pronoun)
}

// validateUIFlags enforces --ui's flag relationships: --port and
// --no-browser only make sense alongside --ui, and --ui itself is
// mutually exclusive with the other ways of choosing what happens to
// the generated prompt (--tui: a different interactive mode; --quick:
// explicitly asks to skip any interactive mode; --copy/--out: the web
// UI decides delivery itself - browser copy/download - so a
// server-side delivery flag has nothing to act on).
func validateUIFlags(cmd *cobra.Command, opts *generateOptions) error {
	if !opts.ui {
		if cmd.Flags().Changed("port") {
			return errors.New("promptsmith: --port requires --ui")
		}
		if cmd.Flags().Changed("no-browser") {
			return errors.New("promptsmith: --no-browser requires --ui")
		}
		return nil
	}

	switch {
	case opts.tui:
		return errors.New("promptsmith: --ui and --tui are mutually exclusive")
	case opts.quick:
		return errors.New("promptsmith: --ui and --quick are mutually exclusive")
	case opts.toClipboard:
		return errors.New("promptsmith: --ui and --copy are mutually exclusive")
	case opts.out != "":
		return errors.New("promptsmith: --ui and --out are mutually exclusive")
	}
	return nil
}

// validateForceFlag enforces --force's one flag relationship:
// overwriting an existing preset is meaningless without a preset name
// to overwrite, so --force alone is a user error, mirroring
// validateUIFlags's --port/--no-browser precedent above. Kept as its
// own small function rather than folded into validateUIFlags: that
// function is specifically about --ui's own flag relationships, and
// growing it to cover an unrelated flag pairing would leave it
// misnamed.
func validateForceFlag(cmd *cobra.Command, opts *generateOptions) error {
	if opts.force && !cmd.Flags().Changed("save-preset") {
		return errors.New("promptsmith: --force requires --save-preset")
	}
	return nil
}

// runUI launches the local web UI and blocks until it's interrupted
// (Ctrl-C) or otherwise stops. Unlike the TUI, --ui doesn't require an
// interactive terminal: "open a browser" doesn't depend on the calling
// process's own stdio, so it works just as well from a script.
//
// signal.NotifyContext lives here, not in the server package: Serve
// takes a plain context.Context so it can be shut down deterministically
// in a test (a context.WithCancel, not a real OS signal, which would
// affect the whole test process).
func runUI(cmd *cobra.Command, reg *registry.Registry, opts *generateOptions, goal string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	return runServerFunc(ctx, reg, server.Options{
		Port:      opts.port,
		NoBrowser: opts.noBrowser,
		NoHints:   opts.noHints,
		Stdout:    cmd.OutOrStdout(),
		// Seeds the page's form, exactly like --tui pre-populates the
		// picker from the same flags (see runInteractive).
		Initial: prompt.Inputs{
			Target:       opts.target,
			Skills:       opts.skills,
			Goal:         goal,
			Context:      opts.context,
			Constraints:  opts.constraints,
			Role:         opts.role,
			OutputFormat: opts.outputFormat,
			Examples:     opts.examples,
		},
	})
}

// runInteractive launches the picker (seeded with whatever was already
// supplied via flags/args) and applies whatever the user chose to do
// with the result, through the same delivery helpers the flag-only path
// uses.
func runInteractive(cmd *cobra.Command, reg *registry.Registry, opts *generateOptions, goal string) error {
	// existingPresets lets the picker warn before silently overwriting
	// a preset on save-as (a later phase; see tui.Result.OverwritePreset).
	// A listing failure is deliberately non-fatal: degrading to "acts
	// as if nothing exists" is safe because preset.Save's non-force
	// path already refuses to clobber (O_CREATE|O_EXCL) regardless of
	// what this list says, so the worst outcome of swallowing the
	// error here is a missed *warning*, never actual data loss.
	existingPresets, listWarnings, err := preset.ListDir()
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "promptsmith: %v\n", err)
		existingPresets = nil
	}
	for _, w := range listWarnings {
		fmt.Fprintln(cmd.ErrOrStderr(), "promptsmith: "+w)
	}

	result, err := runTUIFunc(reg, prompt.Inputs{
		Target:       opts.target,
		Skills:       opts.skills,
		Goal:         goal,
		Context:      opts.context,
		Constraints:  opts.constraints,
		Role:         opts.role,
		OutputFormat: opts.outputFormat,
		Examples:     opts.examples,
	}, existingPresets)
	if err != nil {
		return err
	}

	if result.Action == tui.ActionCancel {
		fmt.Fprintln(cmd.ErrOrStderr(), "promptsmith: canceled")
		return nil
	}

	// Handled BEFORE prompt.Build, and returns without ever calling
	// it: a preset records HOW to ask (target, skills, role, context,
	// constraints, output_format, examples - see the preset package
	// doc comment), never WHAT to ask, so saving one has nothing to
	// gain from Build succeeding and nothing to lose from Build
	// failing. This mirrors the existing --save-preset flag path's own
	// placement in runGenerate, ahead of both the unknown-target check
	// and prompt.Build - "a prompt-assembly failure never blocks a
	// preset save" is already this repo's rule, not a new one
	// introduced here.
	if result.Action == tui.ActionSavePreset {
		return savePresetFromInputs(cmd, result)
	}

	out, err := prompt.Build(reg, result.Inputs)
	if err != nil {
		return err
	}

	// No stderr hint emission here: the TUI now renders promptlint's
	// findings itself, inside the preview pane (internal/tui's
	// recomputePreview), so a second report on stderr after the
	// session ends would double up on the same findings the user
	// already saw live while editing.

	switch result.Action {
	case tui.ActionCopy:
		return copyAndConfirm(cmd, out)
	case tui.ActionWrite:
		return writeFile(result.WritePath, out)
	default: // tui.ActionStdout
		fmt.Fprintln(cmd.OutOrStdout(), out)
		return nil
	}
}

// deliver routes the assembled prompt to every requested destination
// (file, clipboard), additively; if none were requested, it prints to
// stdout. This is the flag-only path's delivery model: unlike the TUI
// (which offers exactly one action per confirm), --copy and --out can
// both apply in the same invocation.
func deliver(cmd *cobra.Command, opts *generateOptions, out string) error {
	delivered := false

	if opts.out != "" {
		if err := writeFile(opts.out, out); err != nil {
			return err
		}
		delivered = true
	}

	if opts.toClipboard {
		if err := copyAndConfirm(cmd, out); err != nil {
			return err
		}
		delivered = true
	}

	if !delivered {
		fmt.Fprintln(cmd.OutOrStdout(), out)
	}
	return nil
}

// writeFile persists out to path with owner-only permissions: a
// generated prompt can embed --context/--constraints or a goal
// containing sensitive detail (paths, internal notes), so it's kept
// unreadable to other users (gosec G306). Shared by the flag-only and
// TUI delivery paths so the guarantee is identical either way.
//
// path may use "~" shorthand (expanded via expandPath) and may name
// directories that don't exist yet (created via MkdirAll, also
// owner-only). An existing file at path is overwritten silently, same
// as a shell redirect would.
func writeFile(path, out string) error {
	expanded, err := expandPath(path)
	if err != nil {
		return err
	}

	if dir := filepath.Dir(expanded); dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("promptsmith: create directory %s: %w", dir, err)
		}
	}

	if err := os.WriteFile(expanded, []byte(out+"\n"), 0o600); err != nil {
		return fmt.Errorf("promptsmith: write %s: %w", expanded, err)
	}
	return nil
}

// copyAndConfirm copies out to the clipboard and confirms on stderr,
// keeping stdout clean for scripting/piping. Shared by the flag-only and
// TUI delivery paths.
func copyAndConfirm(cmd *cobra.Command, out string) error {
	if err := copyToClipboard(out); err != nil {
		return fmt.Errorf("promptsmith: copy to clipboard: %w", err)
	}
	fmt.Fprintln(cmd.ErrOrStderr(), "promptsmith: copied to clipboard")
	return nil
}
