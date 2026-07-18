# promptsmith

Generate portable, skill-aware prompts for any LLM or agent harness.

`promptsmith` assembles a deterministic, copy-paste prompt from a goal, a
set of methodology "skills", and a target harness (`generic`, `opencode`,
`claude-code`, `gemini-cli`). No LLM runs at generation time: the prompt is
assembled from a registry of skills and per-target rendering rules, so the
same inputs always produce the same output.

```
$ promptsmith -t opencode -s diagnose,verify "fix the flaky checkout test"
<task>
fix the flaky checkout test
</task>

<approach>
Load the `diagnose` skill: Hard bugs, failing tests, or performance regressions that need a disciplined debugging loop rather than guesswork.

Load the `verify` skill: Before marking any task done, and after every meaningful change, not only at the very end.
</approach>

<tools>
find: glob
read: read
search: grep
</tools>
```

## Why

Methodology write-ups (how to debug, how to review a diff, how to write
a commit message, ...) tend to live as agent-specific skill files -
useful inside one harness, but not portable to a plain LLM chat window
or a different tool. promptsmith keeps one registry of methodologies and
renders each one appropriately per target:

- **`generic`** - inlines the full methodology text directly into the
  prompt, for pasting into any plain LLM.
- **`opencode`** / **`claude-code`** / **`gemini-cli`** - renders a short
  "load this skill" reference instead (plus a `<tools>` block mapping
  generic tool names to that harness's real tool names), assuming the
  agent already has the skill available.

## Install

Requires Go 1.26+.

```sh
git clone <repo-url>
cd prompt-smith
make install          # go install ./cmd/promptsmith
```

This installs `promptsmith` to `$(go env GOPATH)/bin` (make sure that's
on your `PATH`).

## Quick start

```sh
# Minimal: a goal and a target (target defaults to "generic").
promptsmith "fix the flaky checkout test"

# With skills (comma-separated or repeated -s), role/context/constraints:
promptsmith -t claude-code -s diagnose,verify \
  --role "You are a senior Go engineer." \
  --context "checkout_test.go:42 is flaky." \
  "fix the flaky checkout test"

# Copy to the clipboard, or write to a file, instead of stdout:
promptsmith -s diagnose -c "fix the bug"
promptsmith -s diagnose -o ~/prompts/fix-the-bug.txt "fix the bug"

# No -s, no --quick, run from a terminal: launches an interactive
# skill picker with a live preview instead of requiring flags.
promptsmith
```

A goal is required outside the picker; running `promptsmith` with no
goal and no TTY (e.g. piped) errors with a reminder of the expected
form.

## Commands

The root command generates a prompt; everything else is a subcommand.

| Command | Purpose |
|---|---|
| `promptsmith [flags] <goal>` | Generate a prompt (see flags below). |
| `promptsmith list [-t target]` | List available skills by category, optionally filtered to those supported on a target. |
| `promptsmith validate` | Check the loaded registry's structural integrity (duplicate ids, dangling categories/refs). |
| `promptsmith version` | Print the build version. |

### Generate flags

| Flag | Alias | Description |
|---|---|---|
| `--target` | `-t` | Target harness: `generic`\|`opencode`\|`claude-code`\|`gemini-cli` (default `generic`). |
| `--skills` | `-s` | Skills to include (comma-separated or repeatable). |
| `--context` | `-x` | Background/context for the goal. |
| `--constraints` | `-C` | Constraints the solution must respect. |
| `--role` | `-r` | Role/persona to open the prompt with. |
| `--output-format` | `-f` | Desired shape of the response. |
| `--copy` | `-c` | Copy the prompt to the clipboard instead of stdout. |
| `--out` | `-o` | Write the prompt to this file instead of stdout (accepts `~`/`~user`; missing parent directories are created). |
| `--quick` | `-q` | Never launch the interactive picker, even in a terminal. |
| `--tui` | | Launch the interactive picker even if `--skills` was given. |

`--copy` and `--out` are additive - both can apply to the same
invocation. Without either, the prompt goes to stdout.

### Interactive picker

Running `promptsmith` from a terminal with no `--skills` (and no
`--quick`) launches a picker: browse skills by category with a live
preview of the assembled prompt, edit the goal/role/context/constraints
inline, then choose to print, copy, or write the result. `--tui` forces
the picker even when `--skills` was given; `-q`/`--quick` always skips
it.

## Custom skills

Beyond the built-in registry, promptsmith merges in skills from a
user-writable directory at load time - no rebuild required. It looks in,
in order:

1. `$PROMPTSMITH_SKILLS_DIR`, if set.
2. `$XDG_CONFIG_HOME/promptsmith/skills`, falling back to
   `~/.config/promptsmith/skills`.

It's not an error for this directory not to exist - that's the common
case.

Each skill is a plain `SKILL.md` file - the same format
[Claude](https://docs.claude.com/en/docs/claude-code/skills),
[opencode](https://opencode.ai/docs), and
[Gemini CLI](https://geminicli.com/docs/cli/skills/) skills already use,
so an existing skill set drops in unmodified:

```
---
name: my-team-standup
description: Writing a concise async standup update for the team channel.
---

State what shipped, what's next, and any blockers - three lines max, no filler.
```

Lay skills out as `<category>/<skill-id>/SKILL.md` to place them in a
specific category, or loose as `<skill-id>/SKILL.md` (no category
subdirectory) to fall into a catch-all `custom` category - e.g.:

```
~/.config/promptsmith/skills/
├── debugging/
│   └── my-checklist/SKILL.md   # category: debugging
└── my-team-standup/SKILL.md    # category: custom
```

A user skill whose `name` matches an existing skill id (built-in or
another user skill) overrides it outright; anything else is added.
Malformed or duplicate skills are skipped with a warning printed to
stderr rather than failing the whole load - one bad file can't take down
the CLI.

### Empty variant

`make build-empty` / `make install-empty` build promptsmith with no
bundled skills at all (just the same categories and target definitions,
with no skills) - for anyone who only wants their own skills via
`PROMPTSMITH_SKILLS_DIR` and would rather not carry the built-in set.
Both install to the same `$GOBIN/promptsmith` path as the default build,
so installing one replaces the other.

## Development

```sh
make verify   # fmt, vet, staticcheck, build, test, gosec, govulncheck
make test     # go test ./...
make install  # go install ./cmd/promptsmith
```

`make verify` additionally needs `staticcheck`, `gosec`, and
`govulncheck` on `PATH`:

```sh
go install honnef.co/go/tools/cmd/staticcheck@latest
go install github.com/securego/gosec/v2/cmd/gosec@latest
go install golang.org/x/vuln/cmd/govulncheck@latest
```

See the `Makefile` for the full list of targets.

### CI

Every push and pull request runs [`.github/workflows/ci.yml`](.github/workflows/ci.yml):
`make verify` plus a static check of the `-tags empty` build variant on
Linux, and a `go build` + `go test -race` portability check across
Linux, macOS, and Windows.
