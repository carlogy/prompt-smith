# Prompt-engineering roadmap

Tracks the phased plan for prompt-smith's prompt-engineering work. Each
phase below is either **Completed** (with the commit pair that landed it)
or **Locked** (decisions are final; not yet implemented). Start here cold —
no need to re-derive anything.

## Completed

### Phase 0 — TUI theme + layout height decoupling
Commit `refactor(tui): centralize styles, decouple field height from count`.
- New `internal/tui/theme.go` centralizes all 8 lipgloss style vars
  previously split across `view.go` and `highlight.go`.
- Fixed: accent color was a literal `lipgloss.Color("14")` in
  `highlight.go` AND bound to `activeColor` in `view.go` — two sources of
  truth for one value.
- Fixed: no `lipgloss.AdaptiveColor` anywhere, so the accent was
  near-invisible on light terminals. `activeColor` is now
  `AdaptiveColor{Light: "6", Dark: "14"}`. Dark is unchanged on purpose —
  dark-terminal output stays byte-identical.
- `layout.go`: `numFields` (a count) no longer doubles as the field
  stack's height. New `fieldHeights []int` + `totalFieldsHeight()`
  replace it at both call sites. All heights are still 1 today, so output
  is unchanged. This is the seam Phase 1's multi-line Examples field
  will use.
- `layout_test.go` gained an equivalence test (old count-math still
  matches), a seam test (proves the variable-height path works), and a
  guard test (`len(fieldHeights) == numFields`) — the two are synced by
  convention/comment, not derivation, since `computeLayout` is pure and
  runs before any `model` exists.

### Phase 4 — three new skills
Commit `feat(skills): add research and safety skills, two new categories`.
- New categories `research` (after `planning`) and `safety` (after `git`).
  Canonical order is now: planning, research, coding, debugging, testing,
  review, git, safety, communication, learning.
- New skills: `quote-grounding` (research/10), `generalize-not-hardcode`
  (testing/30), `safe-actions` (safety/10), each with a body file under
  `internal/registry/data/bodies/`.
- Updated `internal/cli/list_test.go` (pins all 10 categories) and
  `internal/registry/integration_test.go` (skill count 11→14, new
  `TestLoad_RealRegistry_ResearchAndSafetyCategories`).

### Phase 1 — `<examples>`
Commit `feat(prompt): add repeatable few-shot examples input` (`648ecee`).
- New `Examples []string` field on `prompt.Inputs`.
- Section emitted **last**, after `<output_format>`.
- Rendered as `<examples>` wrapping N `<example>` children via a dedicated
  nested builder (`examplesSection`) — the existing `section()` helper
  didn't fit.
- `<example>` matches `prompthl.Classify`'s `^<[a-z_]+>$` regex, so nested
  highlighting is free — the "tag alone on its own line" invariant holds.
- CLI flag `-e/--example` uses **`StringArrayVarP`, not
  `StringSliceVarP`** — StringSlice CSV-splits, so an example containing a
  comma would silently fragment.
- TUI and web UI each use **one** multi-line field holding all examples,
  split on a line containing only `---` (matches the SKILL.md convention;
  survives examples with internal blank lines). `NormalizeExamples`,
  `SplitExamples`, `JoinExamples` implement and round-trip this.
- TUI field is a new `bubbles/textarea` (already in the pinned
  `bubbles v1.0.0` — no new module needed), using the `fieldHeights` seam
  added in Phase 0. `numFields` 5→6, `fieldHeights` `{1,1,1,1,1,4}`.
- Verified end-to-end: full local `make verify`, `go test -race`, a real-
  binary CLI smoke test (including the comma-preserving `-e` guarantee),
  and a manual web-UI smoke test of `/` and `/preview` (the e2e suite
  needs Docker and didn't cover this locally). CI (both `CI` and `E2E`
  workflows) passed on push.
- Two open cosmetic items surfaced during implementation, deferred to
  later phases:
  1. The vendored `bubbles/textarea`'s default `CursorLine` background
     tint was deliberately left in place rather than neutralized — a
     Phase 5 polish call.
  2. **Correction (found during Phase 10-12 work): this claim was
     false.** It said `internal/server/e2e_test.go` "has no 'type into
     a form field' flow at all." It does:
     `TestE2E_LivePreviewUpdatesAfterDebounce`
     (`e2e_test.go:272-304`) opens the "Optional fields" `<details>`,
     clicks `#examples`, and calls `chromedp.SendKeys("#examples",
     marker, chromedp.ByQuery)`, then polls for the debounced update.
     This is the same class of error as Phase 7's 0BSD premise
     correction — a claim stated once and never re-checked before
     being repeated — recorded here rather than silently fixed so the
     pattern is visible.

### Phase 2 — presets
Commit `feat(cli): add reusable prompt presets via -p/--preset` (`12eaae0`).
- New `internal/preset` package — NOT `internal/registry`, since this is
  `Inputs` data, not skill data.
- Directory resolution mirrors `internal/registry/userskills.go` exactly:
  `$PROMPTSMITH_PRESETS_DIR` → `$XDG_CONFIG_HOME/promptsmith/presets` →
  `~/.config/promptsmith/presets`. Same non-fatal-warning philosophy: an
  absent dir is silent (the common case), everything else warns on
  stderr.
- Flat `<name>.yaml` files — fields target, skills, role, context,
  constraints, output_format, examples, deliberately **not** goal: a
  preset describes how to ask, not what to ask, so the goal stays a
  fresh per-invocation argument. Writing `goal:` into a preset doesn't
  silently vanish — it gets its own dedicated warning distinct from the
  generic unknown-key one, since it's the one omission a user is likely
  to hit by mistake.
- Flag `-p/--preset`. Precedence (preset supplies defaults, an explicit
  flag always wins) gates on **`cmd.Flags().Changed(name)`**, never an
  empty-value check — `--target` defaults to `"generic"`, so an
  empty-check can't distinguish "the user never passed --target" from
  "the user explicitly passed --target generic"; both look identical,
  and a preset would wrongly clobber the second case.
- Unknown-key warnings deliberately do NOT use `yaml.KnownFields(true)`:
  that option fails the *entire* decode on the first unrecognized key,
  discarding an otherwise-usable preset over one typo. Instead, the same
  bytes are decoded twice — once leniently into the typed struct, once
  into `map[string]any` — and the second pass's keys are diffed against
  the known set to produce warnings without losing anything the first
  pass already parsed.
- Discovered during implementation, extending that same principle from
  wrong *keys* to wrong-typed *values*: yaml.v3 guarantees (yaml.go:63-66
  for `Unmarshal`, and the `TypeError` doc at yaml.go:311-314) that when
  one or more fields have the wrong YAML type, decoding continues to the
  end of the document and the destination struct is still partially
  populated — only a `*yaml.TypeError` comes back, not a fatal error. So
  a wrong-typed field (e.g. `examples: "just one"` where a list was
  expected) is treated as a warning for that field alone, not a reason
  to discard the rest of an otherwise-good file. A plain, non-TypeError
  decode error (a genuine YAML syntax error) remains fatal, since
  nothing usable survives that. This cost about four extra lines on top
  of the unknown-key handling already described above.
- Discovered during implementation: `applyPreset` has to run in
  `runGenerate` **before** `NormalizeSkills`, not after — `decideUseTUI`,
  `warnStraySkillArgs`, and `NormalizeSkills` all key off
  `len(opts.skills)`, and `decideUseTUI` treats zero skills as "launch
  the interactive picker". A preset's skills have to land in
  `opts.skills` before that count is taken, or `promptsmith -p mypreset
  "goal"` would silently fail to suppress the picker.
- Bug caught during implementation, before it shipped: applying a preset
  field unconditionally whenever its flag is unchanged blanks out flag
  *defaults* for any key the preset simply omits — a preset with no
  `target:` key would overwrite `--target`'s `"generic"` default with
  `""` and then fail downstream with `unknown target ""`. Fixed by
  having each field-apply func also skip when the *preset's own* value
  is empty/nil, on top of (not instead of) the `Changed()` gate — the
  two checks answer different questions (was the flag passed vs. did the
  preset actually set this key) and neither alone is sufficient. This is
  orthogonal to the `Changed()` gate and does not reintroduce the
  empty-check trap it was written to avoid, because `Changed()` remains
  the sole signal used for flag-vs-preset precedence; the empty check
  only ever short-circuits the preset side.
- Scope added beyond the locked spec above, by explicit decision: the
  `promptsmith presets` subcommand, plus a richer "unknown preset" error
  that lists the presets that DO exist instead of just naming the bad
  one. `presets` prints exactly one name per line on stdout (composes
  with `| fzf`, shell completion, etc.) and the resolved presets
  directory as a hint on stderr — kept separate so stdout stays a clean
  list.
- A deliberate divergence from `internal/registry/userskills.go`'s
  directory-scanning behavior: that code ignores stray files in the
  skills directory silently, because a skill is a whole subdirectory and
  one extra file at the root is meaningless. Presets warn instead on any
  non-`.yaml` file (e.g. a typo'd `stale.yml`), because for a preset the
  *file itself* is the whole unit of meaning, so a stray wrong-extension
  file sitting there is almost certainly a mistake worth surfacing.
- Preset names get the same path-traversal guard skill ids don't need:
  rejecting any name containing a path separator, or equal to `.`/`..`,
  before anything touches the filesystem — proven by a `panicFS` test
  (an `fs.FS` whose `Open` fails the test immediately if ever called) so
  a bad name provably never reaches disk, not just "returns an error
  eventually."
- Verified end-to-end: full local `make verify` (fmt, vet, staticcheck,
  build, test, security — all clean) and `go test ./... -race -count=1`
  (all packages green); the Docker-gated e2e suite was not run locally,
  as expected. A real, ldflags-version-stamped binary (matching
  `.goreleaser.yaml`'s `-X …cli.version=v{{.Version}}` stamp) was smoke-
  tested by hand against a throwaway `PROMPTSMITH_PRESETS_DIR` for all
  nine scenarios the spec called out: empty-dir guidance (stdout empty,
  stderr guidance, exit 0); a populated dir (`presets` prints exactly the
  name on stdout, directory on stderr); a preset's role/constraints/
  output_format/both examples all present in the rendered prompt with no
  "no --skills given" note; `-r OVERRIDE` beating the preset's role;
  `--target generic` against a preset with `target: opencode` producing
  the visibly-different generic (inlined skill body, no `<tools>` block)
  rendering rather than the opencode one; a preset with an unknown key, a
  `goal:` key, and a wrong-typed `examples` string producing exactly
  three stderr warnings while every other field still applied and exit
  stayed 0; an unknown preset name failing non-zero and listing the
  presets that do exist; a `../../../etc/passwd` preset name rejected
  with a validation error before any file access; and a stray `stale.yml`
  warned about and excluded from the `presets` listing. All nine matched
  expectations exactly. Pushed to `main`; both the `CI` and `E2E` GitHub
  Actions workflows completed with conclusion `success`.
- See the note under Phase 5's polish items below re: `promptsmith -p
  <preset>` with no goal still hitting `errEmptyGoal` instead of opening
  the picker — pre-existing `decideUseTUI` behavior, not something
  presets introduced, but surfaced while testing presets.

### Phase 3 — lint
Commit `feat(promptlint): add advisory prompt-quality hints` (`ef3920f`).

**Package and API**
- New `internal/promptlint` package (naming consistent with `prompthl`).
  Pure `Check(reg *registry.Registry, in prompt.Inputs) []Finding`,
  findings returned in a fixed documented rule order so all rendering
  surfaces are deterministic and golden-testable.
- `Finding` is `{Rule RuleID; Message string}` with **no severity
  field** — all findings are one tier (advisory), and a field with
  exactly one possible value would invite three surfaces to branch on
  something that never varies. Adding it later is a one-line change
  with every call site in-repo.
- `RuleID` is a **string**, not an `int` enum like `prompthl.Kind`.
  Deliberate divergence: `Kind` is a closed presentational
  classification never shown to a user, whereas `RuleID` is
  user-facing identity — it renders straight into the web UI's
  `data-rule` attribute, reads legibly in test failures, and would let
  a future `--no-hints=<rule>` parse directly.
- **Seven `RuleID`s for six rules.** `RuleNoExamples` and
  `RuleFewExamples` are mutually-exclusive outputs of one rule func
  sharing one doc citation. They are separate IDs because the CLI
  collapses pure-absence findings into one line and must distinguish
  "zero examples" (collapsible) from "1-2 examples" (its own sentence)
  by ID alone — the alternative, recounting examples in the CLI, would
  duplicate rule logic in a second place where it could drift.

**Import direction (a question that came up and is now settled)**
- No import cycle exists: `prompt` imports only `registry`, one-way,
  so `promptlint → prompt → registry` is acyclic and nothing below
  imports `promptlint`.
- `promptlint` deliberately DOES import `prompt` and calls
  `prompt.Build` inside `Check` for rule 6's character count, rather
  than estimating the length from `Inputs` plus skill bodies resolved
  via `reg`. An estimate re-derives Build's section layout in a second
  place and drifts silently the moment a section is added — Phase 1's
  `<examples>` section would have broken exactly such an estimate with
  no test failure. Cost is pure string concatenation over at most
  ~12 KB, behind the web UI's existing 300ms debounce.
- `reg` is used by rule 6 alone, which is what keeps rules 1-5 pure
  functions of `Inputs` and testable with no registry at all.

**Rule 1 — the one with real false-positive risk**
- Fires only when `Constraints` splits into **two or more clauses AND
  every clause is clause-initially negative**. Clauses split on
  newline / `.` / `;`; a leading bullet marker (`-`, `*`, `•`, `1.`)
  then a leading conjunction (`and`/`but`) is stripped before testing;
  negation markers are a small closed list matched case-folded at the
  START of a clause only.
- Two traps this exists to avoid, both regression-tested and
  smoke-tested against a real binary: a keyword-anywhere matcher flags
  `add no new dependencies` (positively framed, merely contains "no"
  — and it's this repo's own `all_optional_fields_present.golden`
  constraints value); and firing on a single negative clause flags
  `Don't break the build.`, a legitimate constraint. A constraints
  linter that fires on "don't break the build" is worse than no
  linter.
- Deliberate under-detection: `don't do x and don't do y` on one
  unpunctuated line reads as a single clause and stays silent.
  Quiet-by-default is the intended bias; splitting on `and`/`or` would
  start guessing at boundaries English doesn't reliably mark.
- Implementation detail worth keeping: `splitClauses` does NOT treat a
  `.` immediately preceded by a digit as a boundary, so a `1.`/`2.`
  ordinal bullet survives intact for the bullet-stripper instead of
  being cut in half at its own period. A plain `[\n.;]` split silently
  broke the ordinal-bullet case; this was found and fixed during
  implementation.

**Thresholds (were unspecified in the locked spec; now decided, with
grounding)**
- Fewer than 3 examples — already stated in `internal/fielddesc`'s
  Examples sentence and the README `--example` row, kept consistent.
- Goal shorter than **15 characters** (`minGoalChars`). Grounded in
  this repo's own example goals: `"fix the bug"` (11) fires, `"fix the
  flaky checkout test"` (27) does not. Because 15 flags "fix the bug",
  every goal example under 15 chars in `README.md`,
  `internal/cli/root.go`'s cobra `Example:` block, and
  `internal/cli/presets.go` was lengthened as part of this phase —
  otherwise the tool's own `--help` and docs would demo a goal it then
  complains about.
- Assembled prompt over **8000 characters** on inline targets only
  (`maxInlinePromptChars`). Grounded: the 14 bundled skill bodies total
  ~11,081 chars (~792 average), so 8000 trips only at roughly 10 of 14
  inlined — an indiscriminate selection — while staying silent on a
  focused 3-5 skill prompt (~4,000) and still catching large user
  context/examples. Measured on the real binary: all 14 skills on
  `generic` assembles to 11,171 chars and fires; the same skill list on
  `opencode` (reference mode) does not.
- All three are **character counts. No token estimation anywhere** —
  consistent with token estimation being explicitly out of scope.

**Surfacing**
- CLI: stderr only, stdout stays clean for piping, never affects the
  exit code, `--no-hints` suppresses. Verified on a real binary that
  hints never reach stdout and that exit stays 0 with hints firing.
- The CLI **collapses the three pure-absence findings** (no role, no
  output_format, no examples) into a single line, because a bare
  `promptsmith "goal"` trips all three at once and one-line-per-finding
  would print four lines of unsolicited advice (counting the
  pre-existing `no --skills given` note) on the simplest command a
  first-time user runs. Rules 1/5/6 and the 1-2-examples variant each
  keep their own line, being distinct judgments needing their own
  explanation. The web UI does NOT collapse — it has room. This
  per-surface divergence follows the principle `internal/fielddesc`'s
  package comment already records: the CLI's help and the TUI's
  placeholders stay separate terser strings because those surfaces
  have different space budgets and voices.
- `Finding.Message` stays a capitalized standalone sentence (correct
  for the web UI's list items); the CLI lowercases only the first rune
  when inlining it after the `promptsmith: hint: ` prefix, because
  every other stderr line in this repo is lowercase after that prefix
  and Go's own convention agrees. Done rune-correctly via
  `utf8.DecodeRuneInString`, leaving quoted target ids and digit runs
  verbatim.
- Web UI: rendered **inside the existing `#preview` partial** as an
  `#preview-hints` block with `data-rule` per item, nested in
  `preview.html`'s `{{else if .Lines}}` branch only — so a build error
  keeps the pane (the error branch owns it, and stacking suggestions
  under a hard error is noise) and the "Enter a goal…" placeholder is
  unaffected. No htmx multi-target work was needed, as the locked spec
  predicted.
- The findings block deliberately carries **no `role="alert"` and no
  `role="status"`**. The form re-posts on
  `hx-trigger="input changed delay:300ms"`, so a live region would
  re-announce every suggestion to a screen-reader user on essentially
  every keystroke. The error `<p>`'s `role="alert"` is correct for an
  actual error and was left untouched. A test pins the absence of both
  roles so a future change has to consciously break it.
- `--no-hints` reaches **every** surface, not just stderr:
  `server.Options.NoHints` → `application.noHints` → `handlePreview`
  skips `promptlint.Check`. Otherwise the flag would mean one thing on
  the command line and nothing under `--ui`. Small scope addition
  beyond the locked spec, by explicit decision — same category as
  Phase 2's `presets` subcommand.
- Deliberate deviation worth recording: `noHints` is assigned by
  `Serve` immediately after `newApplication` rather than threaded
  through `newApplication`'s parameter list. `initial` is the
  precedent for an Options field a handler needs going through the
  constructor, so this is inconsistent with it — accepted anyway
  because the alternative adds a bare positional `bool` to five call
  sites, four of which are tests that don't care.
  **Correction (found during Phase 10-12 work): this had drifted from
  five to a stale "four."** The actual call sites are `server.go`,
  `testhelpers_test.go`, two in `page_test.go`, and `api_test.go` —
  verified directly, not by memory. Immaterial to anything that
  shipped; recorded only so the count is accurate. Documented on both
  sides (`application.noHints`'s comment and `Serve`'s), and the zero
  value (false = show hints) is the safe default.

**TUI: deferred, and why**
- The roadmap's locked plan said "TUI: hints area". **Verified during
  this phase that no hints area exists.** The footer is a single line
  fully occupied by per-focus-zone keybind help (`footerHelpFor`), and
  errors are inlined into the preview viewport's own content.
- Building one now was declined: findings are 0-6 items, so the region
  is variable-height, but `computeLayout` is deliberately pure and
  documented as running before any `model` exists. A variable region
  forces a `computeLayout` signature change, which rewrites
  `layout_test.go`'s 6-case pinned golden table plus its
  equivalence/seam/guard tests — and Phase 5's error-banner item would
  then rework the same region a second time.
- Interim coverage: hints print on stderr after the picker exits
  (`runInteractive`, after `prompt.Build` succeeds and after the
  `ActionCancel` early return). Lints `result.Inputs`, NOT `opts` — the
  picker lets the user edit role/output_format/examples after they
  were seeded from `opts`, so `opts` is stale the moment the picker
  returns.

**Scope added beyond the locked spec, by explicit decision**
- `--no-hints` reaching the web preview (above).
- Lengthening every under-15-char goal example in README, `root.go`,
  and `presets.go`.
- `internal/server/assets/static/app.css` regenerated via
  `make ui-css`. Note that this also picked up **pre-existing
  staleness**: `index.html` had been using `hover:underline`, `mt-10`,
  and `focus:outline` without them being compiled in, so those styles
  were silently not rendering. Confirmed pre-existing by a controlled
  A/B (reverting `preview.html` to HEAD and re-running `make ui-css`
  produced the same extra utilities). `make ui-css` was confirmed
  idempotent afterwards.

**Verification**
- Full local `make verify` (fmt, vet, staticcheck, build, test, gosec,
  govulncheck — all clean), `go test ./... -race -count=1` green,
  `make build-empty` compiles under the `empty` tag, `make ui-css`
  idempotent. Docker-gated e2e not run locally, as expected.
- A real ldflags-version-stamped binary (matching
  `.goreleaser.yaml`'s `-s -w -X …cli.version=` stamp) smoke-tested by
  hand against throwaway `PROMPTSMITH_PRESETS_DIR`/
  `PROMPTSMITH_SKILLS_DIR` dirs across nine scenarios: bare goal-only
  producing exactly one collapsed hint plus the pre-existing no-skills
  note; `--no-hints` suppressing every hint while keeping that note; a
  short goal rendering lowercase after the prefix; **a well-formed
  prompt (skills + role + output_format + 3 examples + long goal)
  producing completely empty stderr**; an all-negative constraints
  block firing; **both `"don't break the build"` and `"Don't change
  assertions; add no new dependencies."` NOT firing**; all 14 skills on
  `generic` firing at 11,171 chars while the same list on `opencode`
  stayed silent; stdout/stderr separation with exit 0 despite hints;
  and a live `--ui` server confirming `id="preview-hints"`/
  `data-rule="no-role"` present normally and absent under
  `--no-hints`. All nine matched expectations.
- Pushed to `main`; both the `CI` and `E2E` GitHub Actions workflows
  completed with conclusion `success` (runs 30584308342 and
  30584308173).

### Phase 5 — idiomorph + polish
Commits `53114b7` `3fc9109` `492edaa` `c8ce7e4` `43e4e97` `cc8a2c3`
`2ae848a`.
- `fix(cli): open the picker when a goal is missing in a terminal` —
  closes the `-p <preset>` gap Phase 2 flagged: `decideUseTUI` now also
  opens the picker when the goal is empty in a terminal, instead of
  hitting `errEmptyGoal` unconditionally. Piped/non-TTY invocations still
  fail fast on an empty goal — verified on a real binary (`--quick` and
  whitespace-only `-g` both still exit non-zero with the empty-goal
  error).
- `feat(server): morph the live preview with idiomorph` — vendored
  `idiomorph-ext.min.js` (v0.7.4) to `internal/server/assets/static/`,
  added `hx-ext="morph"` on `<body>`, changed the form to
  `hx-swap="morph:innerHTML"`. Fixes the live preview losing scroll
  position and text selection on every 300ms re-render. `make ui-css`
  reconfirmed idempotent afterward (Tailwind scans templates, and the
  new attributes don't introduce new utility classes).
- `refactor(tui): adopt bubbles/help and bubbles/key` — replaced the
  hand-rolled `footerHelpFor` strings with a `keyMap` (`help.KeyMap`)
  whose `ShortHelp()` switches on focused zone; named-key dispatch goes
  through `key.Matches`. Added a `?` full-screen help overlay. No key's
  behavior changed, only the matching mechanism.
- `fix(tui): page the skill list with PgUp/PgDn when it has focus` —
  PgUp/PgDn previously scrolled the preview regardless of focus; now
  pages the skill list when it has focus.
- `feat(tui): show prompt errors and hints in the preview pane` — gave
  the TUI the real advisory region Phase 3 deferred to stderr, replacing
  the inlined `"error: " + err.Error()` text with a styled banner and
  surfacing `promptlint` findings in the same viewport.
- `feat(tui): show unsupported skills greyed out instead of hiding
  them` — matches the web UI's existing behavior (screen-reader text)
  instead of the TUI silently hiding skills unsupported on the current
  target.
- `fix(tui): truncate skill labels so the list renders at narrow
  widths` — `viewSkillList` was letting `lipgloss.Style.Width` wrap long
  labels mid-word, breaking the one-item-one-display-row invariant
  `visibleWindow`'s scroll math depends on. Added `truncateToWidth`
  (measures with `lipgloss.Width`, cuts on rune boundaries, ellipsizes)
  and truncates each row's plain text before styling — never the cursor
  prefix or the `[-]`/`[x]`/`[ ]` marker. Rendering-only fix;
  `computeLayout`, `visibleWindow`, hit-testing, and the scrollbar are
  untouched. Also documented the picker's recommended terminal size in
  the README.
- Verification: full local `make verify` (fmt, vet, staticcheck, build,
  test, gosec, govulncheck — all clean), `go test ./... -race` green,
  `make build-empty` compiles under the `empty` tag (the greyed-skills
  work touches `buildItems`, so an empty registry still has to render),
  `make ui-css` confirmed idempotent (hash unchanged before/after).
  Docker-gated e2e not run locally (no daemon available), covered by
  CI's `E2E` workflow instead. A real binary was smoke-tested end to
  end: `--help`/`--version`/`list`, a piped non-interactive generate, the
  `--quick`-with-no-goal and whitespace-only-goal empty-goal-error
  guarantees, opencode-vs-generic reference-mode rendering, the `empty`
  build-tag variant generating a goal-only prompt with no skills, and
  the web UI's `/` (200, `hx-ext="morph"` + `idiomorph-ext.min.js` +
  `morph:innerHTML` all present), `/static/idiomorph-ext.min.js` (200,
  JS content type, body contains `Idiomorph`), and `POST /preview` (200,
  rendered prompt HTML with highlighting and hint suggestions) all
  checked by hand. Full validation record:
  `.opencode/validation/phase5/results.md`.
- Two rounds of manual review during this phase: round 1 found the
  narrow-width skill-list wrapping bug (fixed by `2ae848a` above); round
  2 confirmed that fix and found a residual, structurally identical
  wrapping defect in the fields pane (`viewFields`/`Constraints:`),
  deferred below rather than fixed now to keep this phase's blast radius
  to the skill list it was already touching.

### Phase 6 — `--save-preset`
Commit `feat(cli): add --save-preset to author presets from flags`.
Closes the authoring gap Phase 2 left open: the `presets` subcommand
lists presets and `-p/--preset` consumes them, but the only way to
create one was still to hand-write YAML. `internal/preset` had no write
path at all (`Dir`, `validateName`, `LoadFS`/`Load`, `List`/`ListDir`
were all read-only); this phase adds the first one, `Save`.

**What shipped, against the locked spec**
- `--save-preset <name>` flag on the root generate command, not a
  `presets save` subcommand — every existing generate flag doubles as
  the authoring surface.
- Never writes a `goal` key; omits unset fields entirely rather than
  writing empty strings; refuses to overwrite without `--force`
  (naming the full path); `--save-preset` with no goal saves and exits
  0 without opening the picker, and is additive with a goal present;
  writes `.yaml`, never `.yml`; creates the presets directory
  (`0o700`) if absent, file `0o600`; name validation reuses
  `preset.validateName`. All eight locked decisions landed as
  specified.

**Decisions made during implementation, beyond the locked eight**
- `presetDoc` (the on-disk shape `LoadFS` already used) is reused as
  the write shape too, with `,omitempty` added to all seven yaml tags,
  rather than introducing a second write-only struct. `omitempty` is
  marshal-only and decode-inert in yaml.v3, so `LoadFS` is completely
  unchanged by this. It also turns two of the locked guarantees
  *structural* rather than merely checked: a zero-value field can't be
  marshaled as a blank (no per-field "is this empty, skip it" checks
  needed on the write side), and since `presetDoc` has no `Goal`
  field, a `goal` key is unwritable by construction, not by a check
  that could be forgotten.
- The refuse-to-overwrite path uses `os.OpenFile` with
  `O_CREATE|O_EXCL` rather than stat-then-write: atomic, with no
  TOCTOU window in which a concurrently created file could be
  clobbered anyway — the exact race a stat-then-write pair would
  reopen despite `--force` existing to prevent silent clobbers.
- Caught in review of the first cut: the `--force` path's
  `os.WriteFile` does not change the mode of an already-existing
  file, so `--force` over a pre-existing `0o644` preset (hand-authored,
  or saved before this fix) would have left it world-readable —
  defeating the entire reason preset files are `0o600` in the first
  place, since a preset's role/context/constraints text can carry
  proprietary detail. Fixed with an explicit `os.OpenFile` +
  `f.Chmod(0o600)` on the force branch, pinned by
  `TestSave_ForceFixesLooseModeOnPreexistingFile`.
- `preset.Save` returns the full path it wrote, so the CLI's stderr
  confirmation can name it without re-deriving the `dir/name+".yaml"`
  join a second time — that join, and the `.yaml` extension in
  particular, stays this package's invariant to own alone.
- `presetFieldSpecs` (Phase 2's single table stating each preset
  field's flag name and preset<-opts apply func) gained a second
  per-entry func for the opts->preset direction. The seven-field
  mapping is still stated exactly once; a reviewer checks both
  directions in one pass instead of hunting a second table.
  `collectPresetFromOpts` iterates the same table `applyPreset` reads,
  just calling the other func.
- The save direction (`collect`) is value-based and deliberately NOT
  gated on `cmd.Flags().Changed(name)`, unlike the load direction. The
  save runs after `applyPreset`, so `promptsmith -p base --save-preset
  derived` inherits every field `base` supplied into `derived`, not
  only the ones typed on this exact command line — a `Changed`-gated
  save would silently drop anything a loaded preset had merged in. One
  chosen consequence: `--target` defaults to `"generic"` and so is
  never empty, meaning a saved preset always carries a `target:` key,
  even when the user never typed `--target`.
- `saveGeneratedPreset` (called when `--save-preset` is present) runs
  after `resolveGoal`, specifically so a malformed invocation (`--goal
  x` plus a positional goal, `errGoalConflict`) fails *before*
  anything touches the filesystem — no file gets written for a command
  that's about to error out. Pinned by
  `TestGenerate_SavePresetSkippedOnGoalConflict`.
- The no-goal guarantee is an early `return nil` gated on
  `savePresetRequested && goal == "" && !opts.tui` — `decideUseTUI` and
  `interactive.go` were not touched at all. The `!opts.tui` half is
  the distinction that matters: the locked decision suppresses only
  the *implicit* empty-goal trigger for the picker (Phase 5's
  `goalEmpty` branch), but an explicit `--tui` is the user directly
  asking for the picker, so `--save-preset name --tui` still opens it.
  Both halves are pinned by tests
  (`TestGenerate_SavePresetWithNoGoalSavesAndExitsCleanly`,
  `TestGenerate_SavePresetWithNoGoalAndExplicitTUIStillOpensPicker`),
  and `interactive_test.go`'s pre-existing "a preset with no goal
  launches the TUI" test served as the regression canary that this
  change didn't reach that code path at all.
- Saving is additive with every mode, not just a bare goal: with
  `--ui` it saves *and* serves; with `--tui` it saves *and* opens the
  picker. No new mutual exclusions were introduced.
- `--force` is long-only: `-f` is already `--output-format` (a
  duplicate cobra shorthand on one command panics at init), and
  `--force` without `--save-preset` is an error — enforced by a new
  `validateForceFlag` rather than folded into `validateUIFlags`, which
  is specifically about `--ui`'s own flag relationships.
- Tests added: `TestSave_RoundTripsWithLoadFS_ZeroWarnings` plus 8 more
  in `internal/preset/save_test.go` (9 total, covering full round-trip,
  file/dir modes, directory creation, omitted-fields-stay-omitted, the
  no-force-refuses and force-overwrites paths including the mode-fix
  regression above, and invalid-name rejection); 13 in
  `internal/cli/generate_save_preset_test.go` (additive-with-goal,
  no-goal-saves-and-exits, no-goal-plus-explicit---tui,
  round-trip-through-the-real-loader-with-zero-warnings,
  omits-unset-fields, refuse/force-overwrite, `--force` without
  `--save-preset`, empty name, path-traversal name, inherits a loaded
  preset's values, skipped on a goal conflict) including a mechanical
  `TestPresetFieldSpecs_EveryEntryHasBothFuncs` guard that every
  `presetFieldSpecs` entry has both direction funcs non-nil (retired in
  Phase 13 as a strict subset of Phase 10's three-func successor —
  see that phase's note).

**Fixed during this integration pass (not part of the three builders'
work)**
- `gosec` flagged two new G304 ("potential file inclusion via
  variable") findings on `save.go`'s two `os.OpenFile` calls — a
  regression against `main`'s clean `gosec -quiet ./...` baseline,
  confirmed by re-running gosec against `main` with this phase's
  changes stashed. Both are false positives: `path` is
  `filepath.Join(dir, name+".yaml")` where `dir` comes from `Dir()`
  (never user text) and `name` has already passed `validateName`,
  which rejects any path separator and `.`/`..` before this point is
  reached — `path` cannot resolve outside `dir`. Resolved with
  `#nosec G304` comments carrying that justification inline, matching
  this repo's existing convention for a confirmed gosec false positive
  (see `internal/server/browser.go`'s `#nosec G204` on `openBrowser`).
- CI's `windows-latest` test job failed on the first push (commit
  `0a6a5df`): `TestSave_FileAndDirModes` and
  `TestSave_ForceFixesLooseModeOnPreexistingFile` assert exact
  `0600`/`0700` permission bits, which Windows doesn't support (any
  writable file/dir reports `0666`/`0777` regardless of the mode
  passed to `OpenFile`/`MkdirAll`). Not caught locally since local
  verification only ran on macOS. Fixed by guarding the assertions
  with `runtime.GOOS != "windows"`, matching the existing convention
  already used for the identical scenario in
  `internal/cli/generate_test.go`'s `-o`/file-mode tests. Landed as a
  second commit, `fix(preset): skip unix-only file-mode assertions on
  windows` (`2407e65`), rather than amending `0a6a5df`, to avoid a
  force-push.

**Verification**
- `gofmt -l .` clean; `go vet ./...` clean; `go build ./...` clean;
  `staticcheck ./...` clean; `go test ./... -race -count=1` green
  across every package (`cli`, `fielddesc`, `naming`, `preset`,
  `prompt`, `prompthl`, `promptlint`, `registry`, `server`, `tui`);
  `make build-empty` compiles under the `empty` tag; `govulncheck
  ./...` reports no vulnerabilities. `gosec -quiet ./...` was clean
  after the two `#nosec G304` annotations above (dirty before them —
  see that item); confirmed the baseline (`main` with this phase's
  changes stashed via `git stash -u`) was itself clean, so the two
  findings were this phase's regression, not pre-existing noise.
- `make ui-css` was run to confirm idempotency and instead surfaced
  pre-existing staleness unrelated to this phase: Phase 5's
  `truncateToWidth` identifier (`internal/tui/view.go`, landed in
  `2ae848a`) contains the substring "truncate", which Tailwind's
  content scanner picks up as a candidate utility class name and
  compiles in a `.truncate{...}` rule that's never actually used by
  any template. Confirmed pre-existing, not introduced by this phase,
  by re-running `make ui-css` against plain `main` (this phase's
  changes stashed) and observing the identical single-rule diff. This
  phase touched no CSS or templates, so the regenerated
  `internal/server/assets/static/app.css` was reverted
  (`git checkout --`) rather than committed — matching the task's
  instruction that this phase's touch of `ui-css` must be a no-op.
  Worth a follow-up decision on whether to special-case "truncate" or
  otherwise tighten Tailwind's content globs; not done here to keep
  this phase's blast radius to `--save-preset`.
- Docker-gated e2e (`make test-e2e`) was not run locally: `docker info`
  confirmed no daemon reachable (`failed to connect to the docker API
  at unix:///Users/.../docker.sock`). Covered by CI's `E2E` workflow
  instead.
- A real ldflags-version-stamped binary (`-s -w -X
  …cli.version=v0.0.0-smoketest`, matching `.goreleaser.yaml`'s stamp)
  was smoke-tested by hand against a throwaway
  `PROMPTSMITH_PRESETS_DIR` ($(mktemp -d)) for all seven scenarios the
  spec called out: `-r reviewer -c "no breaking changes" --save-preset
  code-review` with no goal exits 0, no picker, confirms the full path
  on stderr with stdout empty, and the file exists as `0o600` in a
  `0o700` directory; the file's raw bytes contain only `target:`,
  `role:`, and `constraints:` — no `goal:` key, no empty-valued keys;
  re-running the identical command without `--force` refuses, naming
  the full path and mentioning `--force`, exit 1; re-running with
  `--force` succeeds and the file stays `0o600`; `promptsmith presets`
  lists `code-review` (and `code-review-2` from an earlier step),
  proving the saver and the pre-existing lister agree on the filename
  convention; `promptsmith -p code-review "fix the flaky checkout
  test"` generates a prompt with the role and constraints applied,
  with `preset.Load`'s own warnings (unknown-key/goal-key/type-error)
  at zero - confirmed both by inspecting the CLI's stderr (no such
  warning text) and by a second run against a preset saved with every
  field set (`-s diagnose -f "markdown checklist" -e "example one"`),
  which reproduced the exact zero-preset-warnings guarantee the new
  `TestGenerate_SavePresetRoundTripsThroughRealLoaderWithZeroWarnings`
  test pins. Note: stderr in both runs still carried the pre-existing,
  unrelated "no --skills given" note and/or a `promptlint` hint about
  example count - both fire off the actual field values regardless of
  whether they came from a preset or from flags directly, and are
  orthogonal to whether the preset itself loaded warning-free; `promptsmith
  --force "some goal"` errors with `promptsmith: --force requires
  --save-preset`, exit 1. All seven matched expectations.
- Pushed to `main` as two commits (see the windows-mode-assertion item
  above): `feat(cli): add --save-preset to author presets from flags`
  (`0a6a5df`) and `fix(preset): skip unix-only file-mode assertions on
  windows` (`2407e65`). Both the `CI` and `E2E` GitHub Actions
  workflows completed with conclusion `success` on the final commit
  (`2407e65`); `CI` failed on `0a6a5df` alone (windows-latest only,
  fixed by the follow-up commit), `E2E` was green on both.

Deferred to a follow-up at the time: a save-as-preset key in the TUI.
Landed as Phase 10 below.

### Phase 7 — shipped-asset hygiene
Commits `bea8609` (`docs: add third-party license notices for vendored
web assets`) and `5fc2b7a` (`fix(server): scope tailwind content
detection to the templates`). Two independent units sharing one phase
because both touch what ships inside `internal/server/assets`.

**The locked premise was wrong, and the correction matters more than
anything else in this phase.** The locked spec asserted both vendored
assets are BSD-2-Clause and that "BSD-2-Clause requires the copyright
notice and license text to accompany redistribution... This is the
reason the phase exists." That's false. htmx v2.0.10 and idiomorph
v0.7.4 are both **0BSD (Zero-Clause BSD)** — verified directly against
both upstream `LICENSE` files at the pinned tags
(https://raw.githubusercontent.com/bigskysoftware/htmx/v2.0.10/LICENSE
and the idiomorph v0.7.4 equivalent) and against both packages' npm
`license` metadata. 0BSD is the BSD-2-Clause text with both conditions
removed, including notice retention — its entire operative text is
"Permission to use, copy, modify, and/or distribute this software for
any purpose with or without fee is hereby granted." There is no
attribution or notice-retention obligation. **There was never a
compliance gap.** Record this explicitly as a correction to the
roadmap's own earlier claim, so a future reader doesn't re-derive the
wrong premise — this is exactly the kind of error this file exists to
prevent recurring.

Unit A shipped anyway, re-justified on honest grounds: provenance and
supply-chain hygiene, not compliance. This repo is the only record of
what these vendored binaries are (no build-time fetch step anywhere);
exact versions plus sha256s make a future upgrade or CVE response
auditable; `idiomorph-ext.min.js` embeds no version string at all, so
without the notices file its version is unrecoverable from the
artifact alone; and a repo-root notices file is the conventional place
an auditor or downstream packager looks.

**What shipped in Unit A** (`bea8609`):
- `THIRD-PARTY-NOTICES.md` at the repo root: per asset — name, exact
  version, upstream source URL, SPDX id (`0BSD`), date vendored (from
  git history), sha256 of the vendored bytes, and the verbatim
  upstream license text.
- Both vendored files were verified byte-for-byte identical to their
  published upstream releases by fetching upstream and comparing
  sha256: `htmx.min.js`
  `71ea67185bfa8c98c39d31717c6fce5d852370fcdfd129db4543774d3145c0de`,
  `idiomorph-ext.min.js`
  `a6437e55b1b6a07bc421f0d230266a39399b6826c6ed19e0ed9c63b707444a5f`.
  Vendored dates: htmx 2026-07-17 (commit `f06751b`), idiomorph
  2026-07-30 (commit `3fc9109`).
- `internal/server/templates.go`'s `staticFiles` doc comment was stale
  — named only htmx, omitted idiomorph entirely, and carried a stale
  "from a later commit" phrasing. Now names both assets and points at
  the notices file.
- Convention noted next to the `ui-css` Makefile target, plus the
  exact Tailwind CLI version used (v4.3.3), since the target pins
  none — a latent reproducibility gap worth knowing about but not
  fixed here.
- README's License section now points at the notices file.
- **A `files:` block was added to both goreleaser archive entries** —
  neither had one, so only goreleaser's default patterns applied and
  the notices file would never have reached a binary recipient. The
  gotcha discovered here, worth keeping: specifying `files:`
  **replaces** goreleaser's defaults rather than adding to them, and a
  literal non-glob entry like `CHANGELOG` is treated as
  required-to-exist and hard-fails the build (`globbing failed for
  pattern CHANGELOG`) in a repo with no CHANGELOG — the wildcard form
  `CHANGELOG*` is required instead. Verified by an actual `make
  release-snapshot` plus `tar -tzf`, confirming `LICENSE`, `README.md`,
  and `THIRD-PARTY-NOTICES.md` all present in both variants' archives.

**What shipped in Unit B** (`5fc2b7a`):
- `@import "tailwindcss" source(none);` at `input.css:27`, keeping the
  explicit `@source "../templates/**/*.html"` glob. The locked spec's
  cause was correct on the second attempt: it was Tailwind v4's
  automatic full-project content detection scanning `.go` files, not
  the `@source` glob.
- Measured outcome, all confirmed empirically rather than estimated:
  committed `app.css` went **19,880 → 16,793 bytes**; 150 → 131
  distinct rule selectors; exactly **19** bare utility rules removed
  (`.absolute .blur .collapse .filter .fixed .grow .hidden .inline
  .invert .invisible .lowercase .ordinal .outline .relative .resize
  .shrink .table .transition .visible`), gained set empty. A fresh
  full-project build for comparison was 19,949 bytes / 151 selectors,
  the 151st being `.truncate`.
- Verification technique worth keeping, reusable for any future CSS
  change and stronger than eyeballing the page: extract and diff the
  selector sets with `rg -o --pcre2 '\.[a-zA-Z0-9\:._-]+(?=\{)' | sort
  -u` and `comm`, asserting the removed set exactly and the gained set
  empty.
- Risk retired: the locked spec worried that over-tightening could
  drop a utility the templates genuinely use. Measurably it did not —
  the bare `.outline` and `.absolute` rules are dead, but the variant
  rules the templates actually use (`.focus\:absolute:focus`,
  `.focus\:outline:focus`, `.focus\:outline-2:focus`,
  `.focus\:outline-cornflower-500:focus`, `.shrink-0`,
  `.transition-opacity`) are different rules and all survived,
  verified individually.
- `make ui-css` is now a genuine no-op. This was deliberately
  re-verified **after** Phase 8 landed, not just after Phase 7,
  because Phase 8 edits `internal/tui/view.go` — the very file whose
  `truncateToWidth` identifier leaked the bogus `.truncate` rule in
  the first place. Identical shasum across two runs with Phase 8's
  code present, `git status --porcelain` clean. That's what proves the
  fix generalizes instead of merely special-casing one identifier —
  closing the follow-up question Phase 6's integration pass left open.

### Phase 8 — narrow-width fields pane
Commit `94e04ae` (`fix(tui): truncate field labels so the fields pane
renders at narrow widths`). The sequel to Phase 5's `2ae848a`, fixing
the structurally identical defect one pane over.

- The locked design held: `truncateToWidth` applied to the **label
  only**, never the value — value wrapping is deliberate and the
  `textinput` owns it.
- A new `effectiveFieldLabelWidth(availableWidth)` helper in `view.go`
  computes a label-column budget that **shrinks** with the pane. The
  critical subtlety: padding a truncated label back out to the fixed
  `fieldLabelWidth` const via `%-*s` would silently undo the
  truncation — the padded target width itself has to shrink, not just
  the string. It reserves `minContentWidth` for the value so the value
  area is never squeezed to zero.
- `viewExamplesField` now takes the label width from its caller
  instead of recomputing from the const, and
  `internal/tui/model.go`'s `fieldWidth` calculation in the
  `WindowSizeMsg` handler derives the same budget through the same
  helper — so the textinput's width and the label rendered beside it
  can never drift out of sync.
- **The height clamp was not needed and was deliberately not added.**
  Evidence: after the label fix, `viewFields` produces exactly
  `totalFieldsHeight()` = 9 lines at terminal widths 80, 60, 55, 50,
  45, 40, and 30. Only at terminal width 20 (a degenerate
  `leftContentWidth` of 2, where the marker and ": " overhead alone
  exceed the budget) does it still wrap, covered by a no-panic guard
  test and matching the same tolerance `viewSkillList`'s precedent
  tests allow at extreme widths. The roadmap's own instruction to
  prove the clamp necessity with a test rather than assume it paid off
  here — the assumption would have been wrong.
- The red phase was observed and recorded before fixing: at terminal
  width 50 (`leftContentWidth` 12), `viewFields` produced 15 lines
  instead of 9, with `Constraints` breaking the row.
- Test gap closed: nothing exercised `viewFields` below 80 columns
  before (the only narrow-relevant test ran at width 90 and tests long
  values, not narrow terminals). Three tests added to
  `fields_view_test.go`: a narrow-pane label-fit test, an
  extreme-narrow no-panic test, and a `WindowSizeMsg` + `View()`
  end-to-end panic guard. `TestView_FieldRowsDoNotWrapWithLongValues`
  was left unmodified.
- `internal/fielddesc` untouched, as locked.

### Phase 9 — what the TUI silently swallows
Three independent units, each landed as its own commit. The theme:
input problems the picker used to absorb instead of reporting.

**Unit A — validate the target before launching the picker (and keep
the banner).** Commit `f07c56c` (`fix(cli): reject an unknown target
before launching the picker`).
- Added `Registry.HasTarget(targetID string) bool` to
  `internal/registry/registry.go` — a genuine gap, since
  `SupportsTarget` folds "unknown target" into a plain `false` and
  every other call site inlined the map lookup.
- `runGenerate` now rejects an unknown `-t` before both interactive
  branches (`--ui` and `useTUI`).
- The error string is byte-identical to what the non-interactive path
  already emitted (`prompt: unknown target %q`), via a small
  `errUnknownTarget` helper in `internal/cli/generate.go`, so no
  user-visible message changed on any path. The deliberate
  duplication's real reason: `internal/cli` already imports
  `internal/prompt`, so importing isn't the obstacle — the obstacle is
  that `internal/prompt` exposes no standalone target-validation entry
  point, only `prompt.Build`, which needs a complete input set and can
  fail for other reasons, so it can't serve as a cheap pre-flight
  check. Exporting a validator from `internal/prompt` is the natural
  cleanup if that string ever needs a third call site.
- Tests assert the sharp thing, not just that an error occurred: the
  `runTUIFunc`/`runServerFunc` spies must never be invoked. Three
  routes covered — `-t bogus --tui`, `-t bogus` with an empty goal in
  an interactive terminal, and `-t bogus --ui`.
- The TUI build-error banner was kept and its white-box test
  (`internal/tui/preview_hints_test.go`) left untouched. The old
  roadmap question "does the banner earn its keep" is definitively
  answered yes; validation was placed in the CLI precisely so the
  banner stays reachable.

**Unit B — make registry warnings survivable.** Commit `a988f0a`
(`fix(cli): print registry warnings after the picker exits`).
- `internal/cli/root.go`'s `Execute()` was split into a thin
  `os.Exit` wrapper plus a testable `run(stdout, stderr io.Writer,
  args []string) int`. Necessary, not gratuitous: `Execute()` was
  called by zero tests and called `os.Exit` inline, so the new
  ordering would otherwise have been unverifiable.
- Warnings now print **after** the command tree finishes and
  **before** the terminal error, so a command failure stays closest to
  the user's cursor. The trap that forced the int-return: the original
  code's `os.Exit(1)` immediately after the command's error check
  would have swallowed every warning printed above it.
- Covered by two new tests driving `run` directly with a real
  malformed skill in `PROMPTSMITH_SKILLS_DIR`, asserting exit code and
  the byte-offset ordering of warning vs. error in captured stderr.
- Honest limitation to record: the ordering is verified against a
  `bytes.Buffer`, not a real TTY alt-screen — actual scrollback loss
  was not simulated. Also `registry.Load`'s own error branch has no
  warnings to swallow (every error return there carries a nil
  warnings slice), so it was left as-is.
- The `--ui` gap this unit's locked spec left "explicitly unproven" was
  confirmed in this pass (warnings never reached the web path at all)
  and then closed by Phase 12 below.

**Unit C — the filename prompt's help text is wrong.** Commit
`7d3b040` (`fix(tui): correct the filename prompt's save-path help
text`).
- The prompt claimed the parent directory must already exist and that
  `~` is not expanded. Both false, re-verified in code: `writeFile`
  (`internal/cli/generate.go`) calls `expandPath` (which resolves both
  `~` and `~user`) then `os.MkdirAll` before writing, and it's the
  single function backing both the TUI save path and the `--out`
  flag. New wording mirrors README's existing phrasing for `--out`.
- Sharp detail worth recording: `TestView_FilenamePromptDocumentsSavePathBehavior`
  (in `model_test.go`, not `view_test.go` as the older note said) only
  asserted the substrings "current directory" and "absolute path" —
  it never asserted the false clause, so it passed both before and
  after and was not a safety net. Assertions pinning the corrected
  claims had to be added, or the fix would have shipped untested.

**Verification (Phases 7–9, merged)**
- `gofmt -l .`, `go vet ./...`, `go build ./...`, `staticcheck ./...`,
  `go test ./... -race -count=1`, `make build-empty`, `gosec -quiet
  ./...`, `govulncheck ./...`, and `goreleaser check` all clean on the
  merged result, against a pre-recorded baseline captured on untouched
  `main` **before** any work started — so any finding would have been
  attributable rather than dismissed as pre-existing noise, a lesson
  carried over from Phase 6.
- `make test-e2e` was **not** run: no Docker daemon reachable locally;
  deferred to CI's E2E workflow.
- Process: the three phases were implemented in parallel in three
  separate `git worktree`s on three branches, each verified in
  isolation, then rebased and fast-forwarded into `main` in the fixed
  order **7 → 9 → 8**, with `make verify` re-run after each merge,
  producing six linear commits with no merge commits.
- The file sets were fully disjoint by design, which is what made the
  parallelism safe: Phases 8 and 9C both touch `internal/tui/view.go`,
  so they were deliberately assigned to the **same** track rather than
  parallelized; Phase 9A and 9B both touch `internal/cli`, so they
  shared a track too.
- **Correction (found during Phase 10-12 work): this note was
  stale.** It originally said the six commits "are on local `main` but
  not yet pushed; CI/E2E outcomes are therefore not yet known and are
  not claimed here." They were pushed. `0243973` (the roadmap-update
  commit that landed alongside them) ran all six checks that existed
  at the time — `test` on ubuntu/macos/windows, `verify`, `e2e`,
  `release-config` — all with conclusion `success`. That commit
  predates Phase 11's new `ui-css-check` job, so it could not have run
  it.

### Phase 10 — TUI save-as-preset
Commit `7b35f47` (`feat(tui): save the assembled prompt as a preset`).
The deliberate follow-up Phase 6 deferred: Phase 6 shipped
`--save-preset` CLI-only, and the picker still had no way to save what
the user had just assembled.

**Resolution of the locked flow, which was architecturally
impossible as written.** The locked spec said the flow "must be:
submit name → `Save(name, p, false)` → on the already-exists error,
enter a confirm state" — a filesystem call made from inside the TUI.
That contradicts `tui.Result`'s own doc comment, which says plainly
that Run never performs the action itself; the caller does. The
resolution: `internal/cli` passes `existingPresets []string` (from the
existing read-only `preset.ListDir()` — the same path the `presets`
subcommand already uses) into `tui.Run`, and the TUI compares
name-to-name against that list, never touching the filesystem itself.
Several reasons converged on this, not just the one contradiction: it
preserves the "Run never performs the action" invariant; it leaves
`internal/tui` with **zero** dependency on `internal/preset`, verified
with `go list -deps`, not just a grep for the import; it avoids
re-deriving the `dir/name+".yaml"` join and the `.yaml` extension,
which Phase 6 established as `internal/preset`'s own invariant to own
alone; and it means no exported `ErrExists` sentinel was needed on
`preset.Save` at all.

Recorded as a deliberate non-change: `preset.Save`'s already-exists
error is still a bare `fmt.Errorf` string, not a sentinel and not a
wrapped `os.ErrExist`, unlike `preset.ErrNotFound`, which *is* an
exported sentinel. Left alone because nothing in this design needs to
detect it programmatically. One accepted wart follows from that: in a
TOCTOU race — a preset created between the picker's existence check
and the actual save — the user sees `Save`'s CLI-flavored "use
`--force` to overwrite" wording surfacing from a TUI-initiated action,
which reads oddly but is harmless. If `ListDir` fails, the CLI warns
non-fatally on stderr and passes `nil`; safe because `Save`'s
non-force path still uses `O_CREATE|O_EXCL` and refuses to clobber
regardless of what the existence list said.

**The prompt-mode seam.** New `internal/tui/promptmode.go` introduces
`promptMode` (`promptModeNone` / `promptModeWriteFilename` /
`promptModeSavePreset`), replacing the `enteringFilename` bool. It
landed as its own no-behavior-change unit, with the existing
`w`-flow tests green before any new key existed, exactly as the
locked spec required. It routes **two** interception points, not one:
the `tea.KeyMsg` dispatch and a `tea.MouseMsg` drop that had its own
separate `enteringFilename` check. Worth recording: the
overwrite-confirm sub-state is a plain `savePresetConfirm bool`, **not**
a fourth enum member — the reason is documented directly in
`promptmode.go`, where the sub-state is a modal-within-a-modal that
still intercepts input at the same two `Update` points the enum
exists to unify, so it needs no case of its own there. Also worth
recording: during that seam-only unit, `staticcheck` flagged the
not-yet-used `promptModeSavePreset` constant (U1000), forcing an empty
`case` purely to keep it referenced — replaced with real handling in
the next unit.

`existingPresets` is assigned inside `Run`, **not** `newModel`,
deliberately, to avoid touching the roughly 18 test files that call
`newModel` directly. A test constructing a model directly has to set
the field itself.

**`Result`** gained `ActionSavePreset`, `PresetName`, and
`OverwritePreset`. `OverwritePreset` is passed straight through as
`Save`'s `force` argument; it is never hardcoded true, because the TUI
only sets it when the user has explicitly confirmed the overwrite.

**The confirm UI — this closes the roadmap's one explicitly-unresolved
design question.** Resolved as **y/n**, not "press Enter again." `y`
overwrites. `n` **and** `esc` both return to the **name prompt with
the typed name preserved**, not to the picker — because `esc` from the
name prompt already means "back to the picker," so collapsing the two
would remove the user's ability to simply pick a different name
without abandoning the save entirely. A destructive action must not
share a keystroke with a benign one.

**The name input is seeded empty**, with a placeholder (`e.g.
terse-code-reviewer`), deliberately NOT from
`naming.SuggestFilename(m.goal, ...)` the way the `w` flow is. A
preset describes *how* to ask, not *what* to ask — it has no `Goal`
field at all — so a goal-derived name would actively contradict the
concept.

**Empty-name Enter is a no-op**, and beyond non-emptiness the TUI does
**not** validate the name any further: `internal/preset`'s own
validation (rejecting path separators and `.`/`..`) is unexported and
stays that package's invariant to own, so duplicating it here would
create a second source of truth. A bad name therefore fails on the CLI
side after the picker quits — consistent with how the existing `w`
flow already behaves for an unwritable path.

**The asymmetry was preserved, not harmonized**, exactly as the locked
spec insisted: `w` overwrites files silently by design ("same as a
shell redirect would," per `writeFile`'s doc comment), while presets
refuse without explicit confirmation because hand-authoring is the
only way presets exist and a clobbered copy is unrecoverable.

`s` is dispatched via the raw `tea.KeyRunes` string switch alongside
`c`/`w`/`?`, with a **display-only** `Save key.Binding` added to
`keyMap` purely so the footer has a label — exactly how `k.Copy` and
`k.Write` already work, for the reason already documented at
`keys.go:22-32`.

**The footer, which was the hard constraint.** The `focusSkills` row
went from 77 columns to **exactly 80 of 80, with zero spare**. Trades
made to get there: Tab's description dropped (`"next"` → `""`), Copy
and Write folded into a single `c/w copy/write` entry (mirroring the
existing two-keys-one-slot `↑/↓` pattern), and `s save` added.
`move`/`select`/`ok`/`copy`/`write`/`cancel`/`enter` were all preserved
verbatim because tests assert those exact literals.
`TestFooter_StaysOneRowAtNarrowWidth` and
`TestView_FooterAlwaysPresentRegardlessOfContent` pass **unmodified**.
The fit was verified by simulating `bubbles/help@v1.0.0`'s actual
`ShortHelpView` loop with real `lipgloss.Width` and the real `" • "`
separator — exact, not estimated — carrying the same caveat the
pre-existing budget already had: it doesn't account for terminals
whose glyph-width tables disagree with `go-runewidth`. **Any future
addition to that row now requires an explicit trade, not an append.**

**CLI side.** `ActionSavePreset` is handled **before** `prompt.Build`
and returns without ever calling it: a preset records how to ask, not
what to ask, so it needs nothing `Build` produces, and a generation
failure must never block a save. This mirrors `--save-preset`'s
existing placement in `runGenerate`. It also does **not** additionally
print the assembled prompt — the picker offers exactly one action per
confirm, per `deliver`'s doc comment, unlike the flag-only path's
additive `--copy`/`--out`; `--save-preset` is additive because it
layers onto an otherwise-unrunning invocation, whereas
`ActionSavePreset` *is* the chosen delivery.

`presetFieldSpecs` gained a **third** leg, `fromInputs func(in
prompt.Inputs, p *preset.Preset)`, plus `collectPresetFromInputs` —
keeping Phase 6's principle that the seven-field mapping is stated
exactly once. It sources `result.Inputs`, **not** `opts`, because the
picker lets the user edit fields after they were seeded from `opts`,
so `opts` is stale the moment the picker returns — the same reason
Phase 3's lint pass lints `result.Inputs`. All seven preset fields
were confirmed sourceable from `prompt.Inputs`.

Guard test: new `TestPresetFieldSpecs_EveryEntryHasAllThreeFuncs` in
`generate_preset_test.go`. Recorded as a deliberate wart at the time:
the older two-func `TestPresetFieldSpecs_EveryEntryHasBothFuncs` still
lived in `generate_save_preset_test.go`, kept byte-untouched apart
from a mechanical closure-signature edit, because it was Phase 6's
regression canary. The old guard was a redundant strict subset of the
new one and was flagged as a candidate for retirement.
**Retired in Phase 13** — see that phase's note.

Tests added: 8 direct-`Update` tests, 1 view test, 1 `teatest`
end-to-end test, and 1 footer-content test in `internal/tui`; 6
CLI-level `TestGenerate_TUI_SavePresetAction_*` tests covering
round-trip through the real loader with zero warnings, using
`result.Inputs` rather than `opts`, omitting `goal` and unset fields,
refusing without overwrite while preserving the original contents,
overwriting successfully at `0o600` (guarded with `runtime.GOOS !=
"windows"`, per the convention Phase 6's Windows CI failure
established), and an invalid name surfacing the validation error
without writing anything.

### Phase 11 — enforce `app.css` freshness in CI
Commit `35fc67a` (`ci: fail when app.css is stale relative to the
templates`). Closes the "`app.css` freshness is unenforced" item
carried in Deferred follow-ups since Phase 7.

- `TAILWINDCSS_VERSION ?= 4.3.3` in the Makefile is now the single
  source of truth for the pinned CLI version; a new
  `print-tailwindcss-version` target exists purely so CI reads the
  version back rather than duplicating it in YAML. `ui-css-check` runs
  `ui-css` and then `git diff --exit-code` on `app.css`.
- The check is its **own** `ui-css-check` job in `ci.yml`,
  ubuntu-latest only — deliberately not folded into `verify` (whose
  steps are all Go tooling and would then install Tailwind on every
  run) and not added to the `test` matrix (macOS/Windows would need
  platform-specific Tailwind binaries for zero extra signal).
- CI `curl`s
  `https://github.com/tailwindlabs/tailwindcss/releases/download/v<VERSION>/tailwindcss-linux-x64`
  and verifies a pinned sha256 (`dc61b3ac…313a`) taken from that
  release's own `sha256sums.txt`. **The gap here is real and worth
  stating honestly:** that checksum is platform-specific (linux-x64)
  and lives in the YAML, not the Makefile, so the single-source-of-truth
  claim above does not fully extend to it — a version bump must update
  **both** `TAILWINDCSS_VERSION` and the checksum. Flagged inline in
  the step's own comment, and carried forward to Deferred follow-ups.
- `ui-css`'s header comment was updated: its "not needed in CI" claim
  had become false, and the version is no longer recorded only in
  prose.
- GREEN proof: `app.css`'s sha256 (`ab8260d6…ce2e`) was identical
  before and after regeneration, and the downloaded linux-x64 v4.3.3
  binary's sha256 matched the official `sha256sums.txt`, so CI's
  binary is provably the same version that produced the committed
  file. RED proof: adding `rotate-45` to `index.html`'s `<body class>`
  made `make ui-css-check` regenerate, diff, and fail with a non-zero
  exit.
- **Phase 7 Unit B was this check's precondition.** Without its
  `source(none)` plus explicit `@source` fix, this check would have
  had false positives from the `.truncate` leak that fix retired.
  Worth stating plainly, since it retroactively justifies why this
  item sat deferred rather than simply forgotten.
- No README/CONTRIBUTING note was added: there's no existing precedent
  for documenting the `ui-css` workflow there, and the Makefile's own
  comments already carry the explanation.

### Phase 12 — surface registry warnings under `--ui`
Commit `1eeb2dd` (`feat(server): surface registry warnings in the web
ui`). Closes the "`--ui` registry warnings" item Phase 9 Unit B left
deferred and Phase 9's closing note confirmed as a real, unmitigated
gap.

- **Sharpen the problem statement, because the old framing understated
  it.** `run()` prints warnings after the command tree returns — but
  under `--ui`, `server.Serve` blocks until the context is cancelled,
  so on the web path those warnings only ever reached the terminal at
  *shutdown*, long after the user had already been working in a
  browser against a silently incomplete registry. So "it already
  prints to the terminal" was never a real mitigation on this path,
  quite apart from a detached server having no usable stderr at all.
- **The plumbing hop worth recording as a deliberate deviation.**
  Warnings ride cobra's `Command.Context()`. `run()` does
  `root.SetContext(withWarnings(ctx, warnings))` before `Execute()`,
  and `runUI` reads them back via `warningsFromContext(cmd.Context())`,
  through a new unexported `warningsContextKey{}` plus
  `withWarnings`/`warningsFromContext`. Chosen over widening
  `newRootCmd`/`addGenerateFlags`, which roughly 90 existing test call
  sites invoke directly. Be honest here: using context as a data
  channel for a non-request-scoped value is generally discouraged, and
  the reason it was accepted anyway is the call-site count, not
  elegance — recorded explicitly so a future reader doesn't mistake
  this for a pattern to copy elsewhere.
- `Options.Warnings []string` → `app.warnings`, assigned in `Serve`
  after `newApplication` and documented on both sides, following the
  `noHints` precedent exactly (see the correction above); `newApplication`'s
  own signature stayed untouched.
- `logger.Warn("registry warning", "warning", w)` fires once per
  warning at `Serve` startup — not per request.
- A `#registry-notice` block in `index.html`, placed between
  `</header>` and `<main>` so it precedes the form in document order,
  gated on `{{if .RegistryWarnings}}` so no empty shell renders when
  there's nothing to say. Rendered in `index.html` only, **never**
  `preview.html` — registry warnings are load-time and static, while
  `preview.html` is rebuilt on every keystroke and is the wrong
  lifetime for them.
- **No `role="alert"` and no `role="status"`, but for a different
  reason than Phase 3's** — worth recording both so they don't get
  conflated later. Phase 3's hints omit them because the form re-posts
  every 300ms and a live region would re-announce on every keystroke.
  This region omits them because it's present at *initial page load*,
  so it's read in normal document order; ARIA live regions exist for
  dynamically inserted content, and one here would be semantically
  wrong regardless of re-announcement concerns. Pinned by
  `TestHandleIndex_RegistryNoticeHasNoLiveRegionRole`.
- The markup uses only utility classes already present in the
  committed `app.css` — structural ones lifted verbatim from
  `#preview-hints`, warning colors from `preview.html`'s error `<p>` —
  each verified before use, so `make ui-css-check` stayed a genuine
  no-op and this phase stayed independent of both the Tailwind
  toolchain and of Phase 11.
- `registry.Load`'s doc comment was updated; its "(Execute prints them
  to stderr)" parenthetical had become incomplete now that a second
  path exists.
- **Two gotchas worth keeping.** `html/template` silently strips
  literal HTML comments at render time, and auto-escapes `"` to
  `&#34;` — a test asserting a warning string containing quotes failed
  until it was changed to a quote-free string matching the real
  `skip %s: no SKILL.md found` shape. And, for anyone trying to
  reproduce a registry warning by hand: a bare top-level directory
  with no `SKILL.md` in `PROMPTSMITH_SKILLS_DIR` is silently treated as
  an empty *category* and emits **no** warning — the `no SKILL.md
  found` warning only fires for a two-level `category/skill/` layout.

**Verification (Phases 10-12, merged)**
- `gofmt -l .`, `go vet ./...`, `go build ./...`, `staticcheck ./...`,
  `go test ./... -race -count=1`, `make build-empty` plus `go vet
  -tags empty` / `staticcheck -tags empty`, `gosec -quiet ./...`,
  `govulncheck ./...`, `make ui-css-check`, `goreleaser check`, and
  `make verify` — all clean, with **zero gosec/govulncheck delta**
  against a baseline captured on untouched `main` before any work
  began, the same attributability discipline carried from Phases 6 and
  9. The baseline lives at `.opencode/validation/phase10-12/baseline.md`,
  which is gitignored and therefore **local-only** — it is not, and
  will never be, part of this repo's history.
- `make test-e2e` was not run locally — `docker info` failed, as in
  every prior phase; covered by CI's `E2E` workflow instead.
- A real ldflags-version-stamped binary was smoke-tested by hand
  across 12 scenarios against throwaway
  `PROMPTSMITH_PRESETS_DIR`/`PROMPTSMITH_SKILLS_DIR` dirs: `--help`/
  `--version`/`list`; the full `--save-preset` no-goal/refuse/`--force`
  cycle with a `0o600` file in a `0o700` dir and no `goal:` key;
  `presets` listing; `-p` round-trip with zero preset-loader warnings
  (distinguished from the orthogonal pre-existing "no --skills given"
  note and promptlint hint); `--force` without `--save-preset`
  erroring; both empty-goal guarantees; `-t bogus` rejected before any
  launch; opencode-vs-generic rendering still visibly different; the
  malformed-skill `--ui` case showing `id="registry-notice"` and the
  warning text, with the `logger.Warn` line appearing **at startup
  rather than shutdown**; a clean skills dir producing no notice
  region at all; and `--no-hints` still suppressing `#preview-hints`.
  All 12 matched.
- The three commits were each independently verified to build and
  pass tests in an isolated `git worktree`, not merely diffed. The
  `internal/cli/generate.go`/`generate_test.go` split across commits 2
  and 3 required hunk-level staging.
- Pushed to `main`. All **seven** checks green on `1eeb2dd`: `test` on
  ubuntu/macos/**windows**, `verify`, **`ui-css-check` (its first-ever
  real-runner execution)**, `e2e`, and `release-config`.
- **Two honest limitations.** `gh` was authenticated only to an
  enterprise host, so raw job logs weren't fetchable (403); the
  `windows-latest` confirmation therefore rests on the check-run's
  `success` conclusion plus source inspection of the `runtime.GOOS !=
  "windows"` guard, not a log grep. And **Phase 10's interactive
  save-preset flow has no real-terminal confirmation** — it is
  covered only by the `teatest` harness and direct-`Update` tests, so
  a hands-on check in a real terminal remains open.

### Phase 13 — footer priority reorder, Tailwind checksum co-location, one retired test
Three independent, file-disjoint units, one integration pass. Commits
`ci: keep the tailwind version and checksum in one place`, `fix(tui):
order footer hints by priority and restore tab's label`, `test(cli):
retire the subsumed presetFieldSpecs guard`.

**The footer — including its decision history, since the reasoning is
the durable part.** The `focusSkills` row had been hand-tuned by Phase
10 to exactly 80 of 80 columns, which cost Tab its description
(`withLabel(k.Tab, "tab", "")`) and produced a visible stray double
space where the empty desc met the separator bullet — a real, shipped
cosmetic bug, not a style nit.

- **The mechanism, confirmed by reading `bubbles/help@v1.0.0`'s own
  source, not inferred:** `help.KeyMap`'s doc comment states help
  renders "in the order in which the help items are returned",
  and `ShortHelpView`'s loop (`shouldAddItem`) appends `"…"` and
  `break`s the moment the next entry would overflow `m.help.Width` —
  it never wraps to a second line. **Order is the priority API** —
  this is the intended mechanism the library ships, not a workaround
  for a limitation.
- **First reversal:** the first cut put `esc cancel` **first**, so it
  would survive down to width 40 on priority alone. Rejected: an exit
  key leading a footer is unconventional placement, regardless of
  whether the width math works out.
- **Second reversal, final:** `esc cancel` moved to **last**, by
  convention. That makes it the *first* truncation casualty, which
  means the row now has to fit within 80 columns for cancel to
  survive — a width-budget guarantee, not a priority-order one. The
  room for that got bought by tightening `help.Model.ShortSeparator`
  from `bubbles/help`'s default `" • "` (3 cols) to two plain spaces
  (2 cols) in `newModel` — 1 column back per gap, across the six gaps
  in this row. Measured (not estimated) via a direct call to
  `viewFooter()` at width 200 (uncapped) with `stripANSI`: `focusSkills`
  is **78** of 80 columns, `focusPreview` is **76**. `FullSeparator`
  (the `?` overlay's column gap, default `"    "`, 4 cols) was
  deliberately left untouched — the overlay has no width pressure and
  a tighter gap there would only hurt legibility for no benefit.
- **The 80-column budget is back as a real constraint.** With ~2
  columns of slack on `focusSkills` and ~4 on `focusPreview`, adding a
  key now needs a genuine trade again — state this plainly so nobody
  reads "order is the priority API" as "appends are free." They are
  not; truncation only decides *which* entry goes missing, not whether
  the row keeps growing forever.
- Rejected alternatives, and why: dropping `tab next` (loses the very
  thing this fix restores); abbreviating `enter` to `ent` (breaks a
  pinned test literal); Tab with an empty description (that *was* the
  stray-double-space bug this phase fixes).
- `s save` is now advertised in `focusPreview`, not just
  `focusSkills`, closing an inconsistency rather than adding a feature:
  `s`/`c`/`w`/`?` dispatch identically from both zones (both fall
  through to `updatePicker`; `model.go` documents that every other
  zone returns first), so advertising `s` in only one of the two was
  an oversight. Its `c`/`w` pair folds into one `"c/w copy/write"`
  entry, the same way `↑`/`↓` already fold into one arrow glyph — that
  fold is what buys the room.
- **Correction to this roadmap's own earlier claim.** The Deferred
  follow-ups list (removed below, now closed) said `focusPreview` "has
  no spare columns either." Measured directly against the pre-Phase-13
  row (`git worktree` at the parent commit): it rendered at 74 of 80
  columns — **roughly 6 columns of slack**, not zero — and no test
  pinned `copy`/`write`/`save` in that row at all, so nothing was
  actually blocking the addition. The claim was wrong; recorded here
  so it isn't repeated.
- New test `TestShortHelp_PriorityOrderSurvivesTruncation` pins:
  `cancel`/`s save`/`tab next` all present at widths 80/100/120, `esc
  cancel` is the final `ShortHelp()` entry, and height stays exactly 1
  row at width 40.

**The Charm v2 investigation — recorded because it will be asked
again.** All three Charm v2 module lines are now GA:
`bubbletea/v2 v2.0.8`, `bubbles/v2 v2.1.1`, `lipgloss/v2 v2.0.5` (as of
2026-07-31). The pinned v1 `bubbles` line is already at its newest —
**v1.0.0 is the latest v1 tag** — so there is no in-line v1 upgrade
available; the only forward path is v2.
- **`bubbles/v2@v2.1.1`'s `help.ShortHelpView` is the same algorithm as
  v1.0.0's** — same loop, same `shouldAddItem` ellipsis-then-`break`.
  No priority system, no multi-line short help, no responsive dropping
  of individual entries. The only changes are cosmetic: `Width`
  becoming `SetWidth()`/`Width()` methods instead of a public field,
  and `DefaultStyles(isDark bool)` using `lipgloss.LightDark`. **A v2
  upgrade offers nothing for the footer problem this phase just
  solved.** Recorded explicitly so this isn't re-derived next time
  someone proposes "just upgrade to v2" as the fix for a footer
  space problem.
- v2.1.1's source imports `charm.land/bubbles/v2`, suggesting a
  module-path change away from `github.com/charmbracelet/...` —
  **unverified from this repo alone**, flagged as such.
- Full migration-surface measurement written to
  `.opencode/research/charm-v2-migration-surface.md` (registered under
  Retained research files below). Confined entirely to `internal/tui`
  — 2,680 production + 4,341 test lines across 23 test files; nothing
  else in the repo imports any `charmbracelet` module, confirmed by
  grep, not assumption. Highlights: 136 literal `tea.KeyMsg{}`
  constructions in tests would need renaming to `tea.KeyPressMsg{}`.
  v1's single `tea.MouseMsg` (with an `Action` enum) **splits into four
  distinct v2 message types** (`MouseClickMsg`/`MouseReleaseMsg`/
  `MouseMotionMsg`/`MouseWheelMsg`) — 8 sites needing genuine
  restructuring, not renaming. Four sites do `msg.Type == tea.KeyRunes`
  plus `string(msg.Runes)` for bare-rune dispatch (`c`/`w`/`s`/`y`/`n`/
  `?`), which `keys.go`'s own comments say exist *because* a generic
  rune catch-all can't be expressed as one `key.Binding` — v2 replaces
  `Type`/`Runes` with a `Key{Text, Mod, Code}` shape, so these are
  load-bearing logic to re-derive, not search-and-replace targets.
  `tea.NewProgram`'s `WithAltScreen()`/`WithMouseCellMotion()` v2
  equivalents are unconfirmed from the public API surface inspected.
  The single biggest unknown: this repo depends on
  `charmbracelet/x/exp/teatest` at a commit pseudo-version with no
  confirmed v2-compatible successor — a bad outcome there would strand
  the end-to-end TUI tests, including Phase 10's save-preset one.
- Migration was investigated and **declined** for now — the surface is
  large, several sites are non-mechanical, and the one component this
  phase actually needed (`help.ShortHelpView`'s truncation behavior)
  is identical in both versions anyway.

**Tailwind checksum co-location.** `TAILWINDCSS_SHA256_LINUX_X64` moved
from `ci.yml` into the Makefile, next to `TAILWINDCSS_VERSION`, with a
new `print-tailwindcss-sha256-linux-x64` target CI reads alongside the
existing `print-tailwindcss-version` one — so a version bump is now a
single-file edit instead of the two-file edit Phase 11 flagged.
Deliberately **not** switched to fetching the release's own
`sha256sums.txt` at CI time: a manifest fetched from the same server
serving the binary protects against corruption in transit but not
against a tampered or re-tagged release, which is exactly what a
pinned known-good hash guards against — the pin is the entire point,
not a workaround for not having automated it yet. Documented directly
in the Makefile comment so a future reader doesn't "simplify" this
into a fetch-and-verify. What still has to happen by hand on a version
bump: fetching the new linux-x64 hash from the release's own
`sha256sums.txt` and pasting it in. Hash value is byte-identical to
before; only its location changed.

**Guard test retired.** `TestPresetFieldSpecs_EveryEntryHasBothFuncs`
(Phase 6, `generate_save_preset_test.go`) checked that every
`presetFieldSpecs` entry had non-nil `apply`/`collect` funcs. Phase
10's `TestPresetFieldSpecs_EveryEntryHasAllThreeFuncs` checks all
three funcs, strictly including those same two, so the older test
added no coverage the newer one didn't already give — it was kept
around only as Phase 6's regression canary (see that phase's note and
Phase 10's). Deleted, with its unique reasoning merged into the
surviving test's doc comment rather than lost: a nil `apply` or
`collect` would panic at runtime the first time that entry's direction
is exercised (`applyPreset` for apply, `collectPresetFromOpts` for
collect), not fail at compile time, since table literals don't enforce
"every field must be set." Also corrected a now-false sentence in that
comment, which had described the newer test as living in a separate
file specifically to leave the older one untouched — no longer true
now that the older one is gone.

**Verification.** `gofmt -l .`, `go vet ./...`, `go build ./...`,
`staticcheck ./...`, `go test ./... -race -count=1`, `make
build-empty` plus `go vet -tags empty`/`staticcheck -tags empty`,
`gosec -quiet ./...`, `govulncheck ./...`, `make ui-css-check`,
`goreleaser check`, `make verify`, and `actionlint
.github/workflows/ci.yml` all clean, with **zero gosec/govulncheck
delta** against the Phase 10-12 baseline
(`.opencode/validation/phase10-12/baseline.md`, local-only) — both
were already clean there, and remained clean here. Both
`make -s print-tailwindcss-version` (`4.3.3`) and `make -s
print-tailwindcss-sha256-linux-x64` (the 64-hex-char pinned hash)
print exactly the expected value with no stray output, confirming
CI's shell-substitution reads will work. `make test-e2e` was not run
locally — `docker info` failed, as in every prior phase; covered by
CI's `E2E` workflow instead. A real binary was smoke-tested
proportionately to this phase's small blast radius (no full sweep):
`--help`/`--version`/`list`, a piped non-interactive generate, and
`make ui-css-check` confirmed to leave `app.css` untouched
(`git status --porcelain` on that path empty both before and after).
The three named tests
(`TestShortHelp_PriorityOrderSurvivesTruncation`,
`TestFooter_StaysOneRowAtNarrowWidth`,
`TestView_FooterAlwaysPresentRegardlessOfContent`) pass by name; the
interactive footer itself was **not** verified in a real terminal —
that remains a manual step (folded into the existing deferred item
below, which now covers both the save-preset flow and this reorder).
- Each of the three commits was independently verified to build and
  pass its package's tests in an isolated `git worktree`, not merely
  diffed — the file sets were fully disjoint (`Makefile` +
  `.github/workflows/ci.yml`; `internal/tui/keys.go` +
  `keys_test.go` + `model.go`; `internal/cli/generate_save_preset_test.go`
  + `generate_preset_test.go`), confirmed via `git status` before
  staging, so no hunk-level staging was needed for any of the three.
- Pushed to `main`. All CI and E2E jobs completed with conclusion
  `success`, including `ui-css-check` specifically (the job most at
  risk from the checksum move) on its first run against the new
  Makefile-sourced value.
- `gh` was authenticated only to an enterprise host, so job-log fetches
  again required unauthenticated `api.github.com` calls for check-run
  metadata, matching Phase 10-12's same limitation.

## Explicitly out of scope
- Long-context section reordering. If revisited: opt-in `--layout` flag,
  default unchanged.
- Token estimation — declined; this is why Phase 3's oversized-prompt
  rule uses character count instead.
- Prompt scoring/evaluation — needs an LLM, breaks the
  no-LLM-at-generation-time invariant.
- Prefilled assistant turns — Anthropic removed support; 4.6+ returns 400.
- A chain-of-thought toggle — adaptive thinking is default-on, manual CoT
  is now only a thinking-off fallback, and Opus 4.5 is sensitive to the
  word "think" while Opus 5 over-verifies.

## Deferred follow-ups
- **Display-line-aware skill-list scrolling — latent, explicitly
  do-not-schedule.** Verified *unreachable* today, because the `item`
  struct (`model.go:46-52`) has exactly five fields (`isHeader`,
  `category`, `skill`, `selected`, `disabled`) with no description or
  multi-line text, and every rendered row — skill rows and category
  headers alike — passes through `truncateToWidth`, so nothing can
  occupy more than one display line. Disabled skills render as a
  single row with a `[-]` marker, not an extra line. The coupling that
  makes the fix expensive is real and was verified: `visibleWindow`
  (`internal/tui/visible_window.go:28`) feeds `viewSkillList`
  (`view.go:290`) and `handleLeftClick` (`model.go:691`), which passes
  its offset to `itemAtPoint` (`internal/tui/hittest.go:39-57`) where
  `globalIndex := offset + listRow` is a direct one-row-per-item
  assumption; `skillsPageSize` (`model.go:815-822`) / `pageSkills`
  (`model.go:836`) derive paging from the same row budget; and
  `scrollbar` (`internal/tui/scrollbar.go:41`) takes item counts as its
  total/visible. A display-line-aware rewrite touches 5-6 production
  functions plus three test files — L-sized work for a defect that
  cannot currently be triggered. Decision: leave `2ae848a`'s regression
  tests as the tripwire and revisit only when something actually makes
  it reachable. Triggers to watch for: a per-skill description or
  summary line, wrapping instead of truncating long labels, a
  multi-line "why is this disabled" annotation, group headers gaining
  subtitles, or replacing `truncateToWidth` for i18n/CJK width reasons.
- **htmx 4 migration — still blocked, now with a dated version check.**
  As of 2026-07-31, npm dist-tags for `htmx.org` report `latest:
  2.0.10` and `next: 4.0.0-beta6` — so 4.0 has not shipped stable, and
  the vendored `htmx.min.js` is exactly the current stable release,
  meaning there is no upgrade debt right now. The migration surface is
  confirmed small: three lifecycle event names
  (`htmx:beforeRequest`, `htmx:afterRequest`, `htmx:afterSettle`) in
  one inline `<script>` block in
  `internal/server/assets/templates/index.html` (around lines
  232-246), where they drive `aria-busy` and the `role="status"`
  announcement, plus swapping the vendored asset. Revisit when 4.0.0
  goes stable.
- **Real-terminal confirmation of the TUI's interactive flows.** Phase
  10's `s` flow — name entry, the y/n overwrite confirm, and the
  actual `preset.Save` call — is exercised only by the `teatest`
  harness and direct `model.Update` calls; nothing has driven it
  through a real terminal by hand. Phase 13's reordered footer row
  (both zones, and the `esc cancel`-last / tightened-separator
  mechanics behind it) is in the same position — covered by
  `TestShortHelp_PriorityOrderSurvivesTruncation`,
  `TestFooter_StaysOneRowAtNarrowWidth`, and
  `TestView_FooterAlwaysPresentRegardlessOfContent`, but not by eyes on
  a real terminal. Low risk in both cases (save-preset is a straight
  copy of the `w` flow's already-proven pattern; the footer reorder is
  covered by three tests including one that pins exact substring
  presence and position at multiple widths) but still an open item.
- **`preset.Save` has no `ErrExists` sentinel.** See Phase 10's note
  above — a one-line addition if a caller ever needs to detect the
  already-exists case programmatically rather than by string.
- **`focusExamples`'s footer row is tight at wide terminals too, for a
  different reason than the skills/preview rows.** Found while
  measuring Phase 13's column budgets, out of scope for that phase:
  this zone's footer descriptor sentence (`footerDescriptorFor`) is
  long enough that `help.Width` is already at or near zero by 120
  columns, squeezing out its own keybind hints before truncation ever
  gets a chance to prioritize between them. The descriptor-to-keybind
  gap here is `viewFooter`'s own fixed `"  "` literal, not
  `ShortSeparator` — so Phase 13's separator tightening does not touch
  this zone's problem at all, and any fix belongs to a different code
  path than the one this phase changed.

## Dependency notes
Pinned `bubbletea v1.3.10` / `bubbles v1.0.0` / `lipgloss v1.1.0` are
**v1** APIs. The Charm GitHub READMEs document v2 (e.g.
`tea.KeyPressMsg` vs. this repo's `tea.KeyMsg`) — their examples cannot
be copied verbatim. As of Phase 13 (2026-07-31), all three v2 lines
are GA (`bubbletea/v2 v2.0.8`, `bubbles/v2 v2.1.1`, `lipgloss/v2
v2.0.5`); the pinned v1 `bubbles` line is already at its newest tag
(v1.0.0), so there is no in-line v1 upgrade available. Migration to v2
was investigated in Phase 13 and explicitly declined for now — see
that phase's note and `.opencode/research/charm-v2-migration-surface.md`
for the full surface (confined to `internal/tui`; several sites are
non-mechanical, notably the `tea.MouseMsg`→four-message-type split and
the unconfirmed v2 successor to the pinned `charmbracelet/x/exp/teatest`
harness).

## Retained research files
- `.opencode/research/ui-web-server.md` — retained deliberately as an
  input to Phase 5.
- `.opencode/research/constraints-field-touchpoints.md` — deleted; it
  existed to support Phase 1, which has now landed.
- `.opencode/research/cli-lint-hints-wiring.md` — deleted; it existed
  to support Phase 3, which has now landed.
- `.opencode/research/charm-v2-migration-surface.md` — retained
  deliberately: the Charm v2 GA lines will surface again ("just
  upgrade" is a predictable suggestion), and this file is the
  measured answer for why that's not a quick win, so the analysis
  doesn't need re-deriving from scratch next time it comes up.
