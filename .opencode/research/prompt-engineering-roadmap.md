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
  2. `internal/server/e2e_test.go` has no "type into a form field" flow
     at all, so no field-level e2e selector was added for `#examples`.
     Worth revisiting if e2e coverage of form input is ever wanted.

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
  because the alternative adds a bare positional `bool` to four call
  sites, three of which are tests that don't care. Documented on both
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
  `presetFieldSpecs` entry has both direction funcs non-nil.

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

Deferred to a follow-up (moved to Deferred follow-ups below): a
save-as-preset key in the TUI.

## Locked, not yet implemented

### Phase 7 — shipped-asset hygiene
Two units, both about what ships inside `internal/server/assets` and how
it got there.

**Unit A — third-party license notices.** This half is a compliance
obligation, not a nice-to-have.
- Two vendored third-party assets, both under
  `internal/server/assets/static/`, both checked into git with **no**
  build-time fetch step anywhere in the Makefile: `htmx.min.js` (v2.0.10,
  51,238 bytes) and `idiomorph-ext.min.js` (idiomorph v0.7.4, the `-ext`
  build, from jsDelivr, 10,153 bytes).
- Embedded via `internal/server/templates.go:44-45`; idiomorph is
  referenced at `internal/server/assets/templates/index.html:8`.
- Both are BSD-2-Clause. **Neither license text exists anywhere in the
  repo** — the only `LICENSE` is the project's own AGPL text.
  BSD-2-Clause requires the copyright notice and license text to
  accompany redistribution, so an AGPL project shipping these without
  attribution has a real (if minor) compliance gap. This is the reason
  the phase exists.
- `idiomorph-ext.min.js` contains no version string at all; its only
  provenance record today is commit `3fc9109`'s message. `htmx.min.js`
  is marginally better off — its version is discoverable only by
  grepping the minified string `version:"2.0.10"`.
- `app.css` (19,880 bytes) is **first-party** Tailwind output, NOT
  vendored, and already has its own convention (the explanatory comment
  at `internal/server/assets/tailwind/input.css:1-26` plus `make ui-css`
  at `Makefile:88-91`). Say so explicitly so nobody "fixes" it too.

Locked decisions:
- A single repo-root notices file (e.g. `THIRD-PARTY-NOTICES.md`)
  listing, per asset: name, exact version, upstream source URL, license
  identifier, and the date vendored — followed by the full BSD-2-Clause
  text for each. Rationale: one file a re-vendoring step must update,
  and the conventional place a license auditor looks. Explicitly NOT
  per-file sidecar `.LICENSE` files and NOT a comment inside the
  minified JS — a comment can't survive re-minification or a re-vendor
  reliably, which is exactly how the current gap happened.
- The convention gets documented where the next person will actually
  collide with it — adjacent to the asset tooling / `ui-css` target —
  so re-vendoring an asset updates the notice rather than silently
  drifting.

**Unit B — scope Tailwind's content detection.**
- The recorded cause was wrong. It is **not** the
  `@source "../templates/**/*.html"` directive at
  `internal/server/assets/tailwind/input.css:28` — that glob correctly
  points only at templates. The actual cause is **Tailwind v4's
  automatic full-project content detection**, which scans the rest of
  the repo including `.go` files, where ordinary Go identifiers collide
  with utility names.
- Measured empirically, not estimated: a full-project build produces
  19,948 bytes vs. 16,793 for an isolated templates-only build —
  **3,155 bytes of dead CSS**, and **20** spurious utilities, not the
  single `.truncate` originally reported: `absolute blur collapse
  filter fixed grow hidden inline invert invisible lowercase ordinal
  outline relative resize shrink table transition truncate visible`.
  `truncateToWidth` is at `internal/tui/view.go:243`.
- No Go file in this repo contains Tailwind class names in a string
  literal, so there is no legitimate reason to scan `.go` sources at
  all.
- This is also what produced the spurious `app.css` diff that Phase 6's
  integration step had to investigate and revert; fixing it should make
  `make ui-css` a genuine no-op.

Locked decisions:
- Disable Tailwind's automatic content detection explicitly (v4's
  `source(none)` on the `@import "tailwindcss"`, or the equivalent) so
  only the template glob is scanned. Rationale: the templates are the
  complete and only source of class names, so automatic detection can
  only ever add false positives here.
- Acceptance criteria, all mechanically checkable: regenerated
  `app.css` no longer contains `.truncate{`, its size drops to roughly
  16.8KB, `make ui-css` is idempotent (hash unchanged across two runs),
  and the web UI still renders correctly.
- Risk to state plainly: over-tightening could drop a utility the
  templates genuinely use. The e2e suite plus a hand check of the
  rendered page is the guard, and any *reduction* in classes the
  templates need would show up as visibly broken layout rather than a
  test failure.

### Phase 8 — narrow-width fields pane
The sequel to Phase 5's `2ae848a` ("fix(tui): truncate skill labels so
the list renders at narrow widths"), which fixed the structurally
identical defect one pane over.

Facts (verified this pass):
- `viewFields` at `internal/tui/view.go:424-438` builds five single-line
  rows as `"%-*s: %s"`, padding each label to `fieldLabelWidth =
  len("Constraints")` = 11 (`view.go:134`), joins them plus
  `viewExamplesField()` (`view.go:453-461`) with newlines, then applies
  **one** `lipgloss.NewStyle().Width(width - scrollbarWidth).Render(block)`
  to the composed multi-line block (`view.go:437`).
- Root cause: there is **no per-row truncation anywhere** — unlike
  `viewSkillList`, the composed block is handed to a single `Width()`
  call at the end, which wraps the composed row text. `Constraints` is
  the longest label, so it's the first to break mid-word
  (`Constrai`/`nts:`).
- Second symptom, same root cause: the block has no height cap, so a
  row that wraps to 2+ physical lines makes the block exceed
  `totalFieldsHeight()`'s budget (`internal/tui/layout.go:68-101`), and
  `computeLayout` never re-checks actual rendered height against that
  budget — the extra lines push past the pane and clip the examples
  placeholder.
- Test gap: the only narrow-width-relevant `viewFields` test,
  `TestView_FieldRowsDoNotWrapWithLongValues`
  (`internal/tui/fields_view_test.go:106-144`), runs at **width 90** —
  comfortably wide. Nothing exercises `viewFields` below 80 columns. By
  contrast `internal/tui/truncate_test.go` covers `viewSkillList` at
  widths 11 and 24.

Locked decisions:
- Reuse the existing `truncateToWidth` helper from `2ae848a`, applied
  to the **label only**, before the `%-*s` padding. Rationale: field
  *value* wrapping is intentional and the `textinput` owns it (see the
  doc comment at `view.go:414-423`) — truncating the whole composed
  row, the way `viewSkillList` does, would break a behavior that's
  deliberate here.
- Treat the bottom-clipping as the same root cause, but **prove it with
  a test** at a width in the 40-60 range rather than assuming. If the
  label fix alone doesn't restore the line count, an explicit height
  clamp is a second, separately-justified change — not a silent
  add-on.
- Do NOT touch `internal/fielddesc`. The label strings are
  `viewFields`' own, from `fieldSpecs()` (`view.go:404-412`);
  `fielddesc` holds one descriptive *sentence* per field
  (`fielddesc.go:46`) consumed by the footer and web hints. They're
  different strings, so no cross-surface consistency work is entangled
  here — which is what the separation in `fielddesc`'s package comment
  was for.
- Mirror `truncate_test.go`'s established harness for this defect
  class: call the render function directly at a hand-picked narrow
  width, assert exact line count via `strings.Split`, assert
  `lipgloss.Height(line) == 1` per line, and add one end-to-end
  `Update(tea.WindowSizeMsg{...})` + `View()` panic guard at a
  realistic narrow terminal size.

### Phase 9 — what the TUI silently swallows
Three independent units. The theme across all three: input problems the
picker currently absorbs instead of reporting.

**Unit A — validate the target before launching the picker (and keep the
banner).** This unit corrects a factual error in the previous roadmap.
The old deferred item claimed the TUI's build-error banner was
unreachable because "the picker structurally prevents all three" of
`prompt.Build`'s error cases, and asked whether the banner earns its
keep. Research found that only two of three are prevented:
- Unknown skill id: genuinely prevented — `buildItems`
  (`internal/tui/model.go:223-243`) only ever iterates `reg.Skills`, so
  an unknown id in `initial.Skills` never becomes a selectable item.
- Skill unsupported on target: genuinely prevented — `buildItems` sets
  `disabled := !reg.SupportsTarget(sk, target)` and forces `selected:
  !disabled && selectedSet[sk.ID]` for every row, agnostic of whether
  the ids came from a preset, `--skills`, or the picker, and it re-runs
  on every target switch (`updateTargetField`, `model.go:518-547`).
- **Unknown target: NOT prevented.** `newModel` takes `initial.Target`
  verbatim with no check against `reg.Targets` (`model.go:179`), and
  the CLI never calls `prompt.Build` before launching the picker —
  `runGenerate` goes `decideUseTUI` → `runInteractive` directly. So
  `promptsmith -t bogus --tui`, or `-t bogus` with no goal in an
  interactive terminal (since `decideUseTUI` also fires on
  `goalEmpty`), opens the picker on a bogus target. Nothing forces the
  user through the target field, so `Result.Inputs.Target` stays bogus
  and the failure only surfaces from `runInteractive`'s own
  `prompt.Build` call after the user has already committed to an
  action. The non-interactive path errors correctly today
  (`internal/cli/generate_test.go:332-335`); no test covers the
  interactive path.

Locked decisions:
- **Keep the build-error banner.** It is load-bearing for a real
  user-error path, not defensive-only. This supersedes the old "decide
  whether it earns its keep" question, which is now answered: it earns
  it. The banner lives in `recomputePreview` (`model.go:773-796`, error
  branch at `778-780`) with `errorBannerStyle` (`internal/tui/theme.go:109-113`)
  having exactly that one call site.
- **Reject an invalid target before the picker launches** (project
  owner's decision), erroring exactly the way the non-interactive path
  already does for `-t does-not-exist`. Rationale: it's consistent
  across both paths, and the TUI never holds an invalid target at all.
  The alternatives considered and rejected were having the picker snap
  to the registry default with a banner explaining the substitution
  (silently changes what the user asked for) and documenting the
  status quo while only closing the test gap.
- Implementation caution to record: validation has to land before
  `runInteractive` without double-erroring on the non-interactive path
  (which already fails via `prompt.Build`) and without breaking
  `--ui`, which also carries a target.
- Add the missing integration tests: `-t bogus --tui`, **and** the
  goal-empty equivalent (`-t bogus` with no goal in an interactive
  terminal), since those are two distinct routes into the same hole.
  Note that the existing banner tests
  (`internal/tui/preview_hints_test.go:29-48`, which drives `newModel`
  white-box with `Target: "does-not-exist"`) stay valid and should not
  be deleted.

**Unit B — make registry warnings survivable.**
`loadUserSkills` warnings are generated at
`internal/registry/embed.go:62`, returned from `registry.Load()`
(`embed.go:34-63`), and printed at `internal/cli/root.go:40-42` inside
`Execute()` — immediately before `newRootCmd(reg).Execute()` at
`root.go:44`. The picker's alt-screen (`internal/tui/tui.go:41`,
`tea.WithAltScreen()`) opens long after, and most terminals don't
restore scrollback across it, so the warnings are gone for good. A
malformed user skill in `PROMPTSMITH_SKILLS_DIR` is invisible from the
user's point of view.

Locked decisions:
- Print the warnings **after** the picker exits rather than before it
  opens (project owner's decision): the smallest change that makes
  them survivable, and it needs no new model state or view plumbing.
  Alternatives considered and rejected for now: rendering them inside
  the picker as a general notification surface (the existing
  build-error banner is condition-specific, not a reusable toast, so
  this is M-sized new state), and gating the pre-TUI print on whether
  the TUI will launch (awkward against cobra's parse-then-run model).
- Record as a **remaining follow-up, explicitly unproven**: the `--ui`
  path appears to have the same gap and is arguably worse (a
  detached/backgrounded server has no stderr at all, and no surface in
  the browser shows warnings either). This was inferred from the
  absence of a warnings field in the registry response DTO and no
  warning-handling call site being found — it must be confirmed in
  `internal/server/app.go`'s `newApplication` before anyone acts on it.

**Unit C — the filename prompt's help text is wrong.**
`viewFilenamePrompt` (`internal/tui/view.go:463-471`) tells the user, at
lines 465-469, that the parent directory must already exist and that
`~` is not expanded. Both claims are false: `writeFile` in
`internal/cli/generate.go` calls `expandPath` (which handles `~`) and
`os.MkdirAll` before writing. Locked decision: correct the text to
match what actually happens. `TestView_FilenamePromptDocumentsSavePathBehavior`
(`internal/tui/view_test.go:554`) asserts the current wording and will
need updating alongside it — note that the test is what makes this
safe to change.

### Phase 10 — TUI save-as-preset
The deliberate follow-up Phase 6 deferred: Phase 6 shipped
`--save-preset` CLI-only, and the picker still has no way to save what
the user just assembled.

Facts to record — the blocking one first:
- **The existing filename-prompt mechanism is hardcoded to
  write-to-file, not generic.** There is a single `m.enteringFilename`
  bool plus one `m.filenameInput` (`internal/tui/model.go:92-93`);
  `Update` intercepts every `tea.KeyMsg` with an `if
  m.enteringFilename` check (`model.go:347-349`), not a switch over
  modes; and `updateFilenameInput`'s Enter branch hardcodes `Action:
  ActionWrite` (`model.go:643-647`). So a second prompt cannot simply
  reuse it.
- `w`/`c`/`?` are matched as raw `tea.KeyRunes` string comparisons
  inside `updatePicker` (`model.go:609-633`), **not** `key.Binding`s —
  the reason is documented at `internal/tui/keys.go:19-30` (bubbletea
  can't `key.Matches` a generic rune catch-all as one binding).
- The `w` flow for reference: on keypress it sets `enteringFilename`,
  builds a fresh `textinput.Model` seeded from
  `naming.SuggestFilename(m.goal, time.Now())`, and focuses it; Enter
  yields `Result{Action: ActionWrite, WritePath: ...}` + `tea.Quit`;
  Esc returns to the picker without cancelling the session. `Result`
  itself is at `internal/tui/result.go:22-37`
  (`ActionCancel/ActionStdout/ActionCopy/ActionWrite`, fields
  `Inputs`/`Action`/`WritePath`).
- `internal/preset/save.go`'s `Save(name string, p *preset.Preset,
  force bool) (string, error)` is the write path Phase 6 added; its
  non-force branch uses `O_CREATE|O_EXCL` and returns an
  already-exists error.

Locked decisions:
- **Replace the `enteringFilename` bool with a small prompt-mode enum**
  (none / write / save-preset) as the *first* unit, landed with the
  existing `w`-flow tests green before any new key is added.
  Rationale: a second bool would collide at the single `Update`
  interception point, and the enum keeps that routing to one branch
  instead of a growing chain of ifs.
- Add `ActionSavePreset` to `Result`, plus a `PresetName` field
  mirroring `WritePath`.
- **Overwrite confirmation is required, not polish.** `preset.Save`
  refuses to overwrite without `force`, and the TUI has no flag to
  pass, so the flow must be: submit name → `Save(name, p, false)` →
  on the already-exists error, enter a confirm state → on confirm,
  `Save(name, p, true)`. Record that **no confirm-dialog pattern
  exists anywhere in this TUI today** — there is no yes/no modal to
  mirror, only "Enter confirms" help text — so this is net-new UI and
  the main reason the phase is M-sized rather than a mechanical copy
  of the `w` path.
- Record a deliberate asymmetry that must NOT be "harmonized": the
  existing `w` write-to-file path overwrites silently by design
  (`writeFile`'s doc comment: "same as a shell redirect would"), while
  presets refuse without `--force` because, per `save.go`'s doc
  comment, hand-authoring is the only way presets exist and there's no
  recovering the clobbered copy. Two different defaults, both
  intentional.
- **Key is `s`** — confirmed free; only `c`, `w`, `?` and the named
  bindings (arrows, Tab/ShiftTab, Space, Enter, Esc, PgUp/PgDown,
  CtrlC) are taken. But the footer is a hard constraint:
  `ShortHelp`/`FullHelp` live at `internal/tui/keys.go:112-183`, and
  the doc comment at `keys.go:126-134` records that the `focusSkills`
  row is **already at its 80-column budget**, guarded by
  `TestFooter_StaysOneRowAtNarrowWidth` and
  `TestView_FooterAlwaysPresentRegardlessOfContent`. A new hint
  therefore requires shortening existing labels, not appending.
- Test patterns to mirror: `internal/tui/model_test.go:321-364` drives
  the `w` flow with direct `model.Update(tea.KeyMsg{...})` sequences
  (no `teatest` harness); view assertions live in
  `internal/tui/view_test.go:429`; and the CLI side needs a case
  exercising `Action: tui.ActionSavePreset` through the `runTUIFunc`
  spy, the pattern already used at
  `internal/cli/generate_test.go:366/399/478/525/579`.
- **One open design question, explicitly unresolved** and to be
  settled when this phase is specced: whether the overwrite
  confirmation is a y/n modal or a "press Enter again to confirm"
  step. Both are new UI; neither has precedent in this codebase.

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

## Dependency notes
Pinned `bubbletea v1.3.10` / `bubbles v1.0.0` / `lipgloss v1.1.0` are
**v1** APIs. The Charm GitHub READMEs document v2 (e.g.
`tea.KeyPressMsg` vs. this repo's `tea.KeyMsg`) — their examples cannot
be copied verbatim.

## Retained research files
- `.opencode/research/ui-web-server.md` — retained deliberately as an
  input to Phase 5.
- `.opencode/research/constraints-field-touchpoints.md` — deleted; it
  existed to support Phase 1, which has now landed.
- `.opencode/research/cli-lint-hints-wiring.md` — deleted; it existed
  to support Phase 3, which has now landed.
