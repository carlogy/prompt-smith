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
// and onto the flag whose explicit use should take precedence over it.
// Table-driven so the mapping is stated in exactly one place: a
// reviewer can check all seven flag names in one pass instead of
// hunting through applyPreset's body, and a mistyped flagName here is
// what TestApplyPreset_ExplicitFlagBeatsPreset's per-field cases would
// catch (a bad name makes cmd.Flags().Changed(name) always report
// false - see pflag's Changed - so the preset would keep clobbering an
// explicit flag for that field instead of yielding to it).
//
// Each apply func also skips a preset field that's empty/nil: a
// preset.Preset has no way to distinguish "the YAML omitted this key"
// from "the key was explicitly set to its zero value" (see
// presetDoc), so treating an omitted field as a no-op is the only
// reading that doesn't clobber a flag's own default with an empty
// string - --target's default is "generic" (see addGenerateFlags), so
// a preset that only sets, say, role would otherwise blank out the
// target to "" and fail with "unknown target \"\"".
var presetFieldSpecs = []struct {
	flagName string
	apply    func(opts *generateOptions, p *preset.Preset)
}{
	{"target", func(opts *generateOptions, p *preset.Preset) {
		if p.Target != "" {
			opts.target = p.Target
		}
	}},
	{"skills", func(opts *generateOptions, p *preset.Preset) {
		if len(p.Skills) > 0 {
			opts.skills = p.Skills
		}
	}},
	{"role", func(opts *generateOptions, p *preset.Preset) {
		if p.Role != "" {
			opts.role = p.Role
		}
	}},
	{"context", func(opts *generateOptions, p *preset.Preset) {
		if p.Context != "" {
			opts.context = p.Context
		}
	}},
	{"constraints", func(opts *generateOptions, p *preset.Preset) {
		if p.Constraints != "" {
			opts.constraints = p.Constraints
		}
	}},
	// "output-format", NOT the YAML key "output_format": Changed()
	// looks flags up by their cobra flag name (hyphenated), not by the
	// preset file's key.
	{"output-format", func(opts *generateOptions, p *preset.Preset) {
		if p.OutputFormat != "" {
			opts.outputFormat = p.OutputFormat
		}
	}},
	// "example", singular: the flag is -e/--example even though both
	// the preset field and the opts field are plural (Examples/examples).
	{"example", func(opts *generateOptions, p *preset.Preset) {
		if len(p.Examples) > 0 {
			opts.examples = p.Examples
		}
	}},
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

func runGenerate(cmd *cobra.Command, reg *registry.Registry, opts *generateOptions, args []string) error {
	if err := validateUIFlags(cmd, opts); err != nil {
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
	if opts.ui {
		return runUI(cmd, reg, opts, goal)
	}

	useTUI, err := decideUseTUI(isInteractive(), opts.quick, opts.tui, len(opts.skills), goal == "")
	if err != nil {
		return err
	}

	if useTUI {
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
	result, err := runTUIFunc(reg, prompt.Inputs{
		Target:       opts.target,
		Skills:       opts.skills,
		Goal:         goal,
		Context:      opts.context,
		Constraints:  opts.constraints,
		Role:         opts.role,
		OutputFormat: opts.outputFormat,
		Examples:     opts.examples,
	})
	if err != nil {
		return err
	}

	if result.Action == tui.ActionCancel {
		fmt.Fprintln(cmd.ErrOrStderr(), "promptsmith: canceled")
		return nil
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
