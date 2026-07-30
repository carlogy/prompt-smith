# promptsmith

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL_v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)
[![Release](https://img.shields.io/github/v/release/carlogy/prompt-smith)](https://github.com/carlogy/prompt-smith/releases/latest)

Generate portable, skill-aware prompts for any LLM or agent harness.

`promptsmith` assembles a deterministic, copy-paste prompt from a goal, a
set of methodology "skills", and a target harness (`generic`, `opencode`,
`claude-code`, `gemini-cli`, `codex`). No LLM runs at generation time: the
prompt is assembled from a registry of skills and per-target rendering
rules, so the same inputs always produce the same output.

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
- **`opencode`** / **`claude-code`** / **`gemini-cli`** / **`codex`** -
  renders a short "load this skill" reference instead, assuming the agent
  already has the skill available. `opencode`, `claude-code`, and
  `gemini-cli` also get a `<tools>` block mapping generic tool names to
  that harness's real tool names; `codex` is shell-centric (no discrete
  named tools) and omits that block.

## Install

Requires Go 1.26+ (only matters if you build from source or use `go install`).

### Download a release (recommended)

Every [release](https://github.com/carlogy/prompt-smith/releases) publishes
prebuilt archives for linux/darwin/windows x amd64/arm64. There are two
variants - pick the one that matches what you want:

| Archive pattern | Contains |
|---|---|
| `promptsmith_<version>_<os>_<arch>.tar.gz` (`.zip` on windows) | binary + all 11 built-in skills - the "just start generating prompts" build |
| `promptsmith-empty_<version>_<os>_<arch>.tar.gz` (`.zip` on windows) | binary only, zero bundled skills - for supplying your own via `PROMPTSMITH_SKILLS_DIR` (see [Custom skills](#custom-skills)) |

Download the one for your OS/arch, extract it, and put `promptsmith` on
your `PATH`, e.g.:

```sh
tar xzf promptsmith_0.1.0_darwin_arm64.tar.gz
sudo mv promptsmith /usr/local/bin/
```

See [Verifying a download](#verifying-a-download) below before trusting a
downloaded binary.

### go install

```sh
go install github.com/carlogy/prompt-smith/cmd/promptsmith@latest  # latest release
go install github.com/carlogy/prompt-smith/cmd/promptsmith@v0.1.0  # pinned
```

This always builds the default variant (all 11 skills embedded) - build
tags aren't part of a module version, so there's no `@version` form that
selects the empty variant. To get the empty variant via `go install`
instead, add the build tag to the invocation itself:

```sh
go install -tags empty github.com/carlogy/prompt-smith/cmd/promptsmith@latest
```

Either form installs to `$(go env GOPATH)/bin` (make sure that's on your
`PATH`).

### From source

```sh
git clone https://github.com/carlogy/prompt-smith.git
cd prompt-smith
make install          # go install ./cmd/promptsmith
make install-empty    # or: the empty variant instead
```

### Verifying a download

Every release also publishes a `checksums.txt` (sha256, one line per
archive) and a [SLSA build provenance
attestation](https://github.com/carlogy/prompt-smith/actions/workflows/release.yml)
for every file, including `checksums.txt` itself - so verifying the
checksums file also establishes that it (and everything it checksums) came
from this repo's release workflow, not a tampered mirror.

Check the sha256 sum against `checksums.txt` (run this from the directory
you downloaded into; both tools only check the line matching a file that's
actually present, so extra lines in `checksums.txt` for platforms you
didn't download are reported as merely "missing", not a failure):

```sh
# macOS
shasum -a 256 -c checksums.txt --ignore-missing

# Linux
sha256sum -c checksums.txt --ignore-missing
```

Then verify the provenance attestation with a recent [GitHub
CLI](https://cli.github.com/) (`gh`):

```sh
gh attestation verify promptsmith_0.1.0_darwin_arm64.tar.gz --repo carlogy/prompt-smith
```

This proves the file was built by `carlogy/prompt-smith`'s own release
workflow (not hand-uploaded or substituted), by checking a Sigstore-signed
attestation against GitHub's transparency log - it does not by itself
prove the release's *source code* is trustworthy, only that the binary
matches what the workflow produced from whatever was tagged.

## Quick start

```sh
# Minimal: a goal and a target (target defaults to "generic").
promptsmith "fix the flaky checkout test"

# With skills (comma-separated or repeated -s), role/context/constraints:
promptsmith -t claude-code -s diagnose,verify \
  --role "You are a senior Go engineer." \
  --context "checkout_test.go:42 is flaky." \
  -c "don't change the test's timeout value" \
  "fix the flaky checkout test"

# Copy to the clipboard, or write to a file, instead of stdout:
promptsmith -s diagnose -y "fix the bug"
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
| `promptsmith [flags] <goal>` | Generate a prompt (see flags below); `<goal>` may be given positionally or via `-g`/`--goal` - the two are mutually exclusive. |
| `promptsmith list [-t target]` | List available skills by category, optionally filtered to those supported on a target. |
| `promptsmith validate` | Check the loaded registry's structural integrity (duplicate ids, dangling categories/refs). |
| `promptsmith version` | Print the build version. |

### Generate flags

| Flag | Alias | Description |
|---|---|---|
| `--target` | `-t` | Target harness: `generic`\|`opencode`\|`claude-code`\|`gemini-cli`\|`codex` (default `generic`). |
| `--skills` | `-s` | Skills to include. Comma-separated with **no spaces**, or repeat the flag. |
| `--goal` | `-g` | The goal/task. An alternative to passing the goal as a positional argument; the two are mutually exclusive. |
| `--context` | `-x` | Background/context for the goal. |
| `--constraints` | `-c` | Constraints the solution must respect. |
| `--role` | `-r` | Role/persona to open the prompt with. |
| `--output-format` | `-f` | Desired shape of the response. |
| `--example` | `-e` | A worked example of the desired output. Repeatable - use the flag once per example; **3-5 is recommended**. |
| `--copy` | `-y` | Copy the prompt to the clipboard instead of stdout. |
| `--out` | `-o` | Write the prompt to this file instead of stdout (accepts `~`/`~user`; missing parent directories are created). |
| `--quick` | `-q` | Never launch the interactive picker, even in a terminal. |
| `--tui` | | Launch the interactive picker even if `--skills` was given. |
| `--ui` | | Launch the local web UI in your browser. |
| `--port` | | Port for `--ui` to bind (default: an OS-assigned free port). |
| `--no-browser` | | With `--ui`, don't automatically open a browser. |

#### Quoting and list syntax

Quote every multi-word flag value. An unquoted value is split by your
shell into separate words, and promptsmith treats each extra word as its
own positional argument, silently folding it into the goal - `-x fix the
login bug` (no quotes) does *not* set `--context` to `"fix the login
bug"`; it sets it to `fix`, and `the`, `login`, and `bug` end up appended
to the goal instead.

`--skills` takes a comma-separated list with **no spaces after the
commas**, or you can repeat the flag - both forms are equivalent:

```sh
# Comma-separated, no spaces after the commas:
promptsmith -s diagnose,verify,tdd "fix the flaky checkout test"

# Or repeat the flag:
promptsmith -s diagnose -s verify -s tdd "fix the flaky checkout test"
```

A stray space or trailing comma (`-s "diagnose, verify"`, `-s diagnose,`)
is tolerated - surrounding whitespace is trimmed and empty entries are
dropped. What's *not* tolerated is a plain space-separated list without
commas: `-s diagnose verify tdd` only sets `diagnose` as a skill, and
`verify` and `tdd` leak into the goal as ordinary words. If a leaked word
happens to match a known skill id, promptsmith warns on stderr so the
mistake doesn't pass silently.

`--example`/`-e` is the opposite of `--skills`: it's **never**
comma-separated, only repeatable - `-e "one" -e "two"` gives two
examples, full stop. This is deliberate, not an oversight: worked
examples routinely contain commas of their own, so comma-splitting them
the way `--skills` does would mangle real input.

```sh
promptsmith -t claude-code -s tdd \
  -e "Input: 3 + 4 * 2 -> Output: 11 (respects operator precedence)" \
  -e "Input: (1 + 2) * 3 -> Output: 9 (parentheses override precedence)" \
  "refactor the expression parser to use a proper precedence climber"
```

`--goal`/`-g` and a positional goal are mutually exclusive - passing
both is a hard error, not a silent merge, so you always know which one
won.

```sh
promptsmith -t claude-code -s diagnose,verify \
  -g "fix the flaky checkout test" \
  -x "checkout_test.go:42 fails about 1 in 20 runs." \
  -c "don't change the test's timeout value" \
  -r "You are a senior Go engineer." \
  -y
```

`--copy` and `--out` are additive - both can apply to the same
invocation. Without either, the prompt goes to stdout.

### Interactive picker

Running `promptsmith` from a terminal with no `--skills` (and no
`--quick`) launches a picker: browse skills by category with a live
preview of the assembled prompt, edit the
goal/role/context/constraints/examples inline, then choose to print,
copy, or write the result. `--tui` forces the picker even when
`--skills` was given; `-q`/`--quick` always skips it.

Both the terminal picker and the web UI (`--ui`) hold examples in a
single multi-line field rather than one field per example. Separate
multiple examples with a line containing only `---` - the same
delimiter `SKILL.md` frontmatter uses elsewhere in this project. Unlike
a blank line, `---` survives examples that themselves contain blank
lines, which multi-line input/output example pairs often do.

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
[opencode](https://opencode.ai/docs),
[Gemini CLI](https://geminicli.com/docs/cli/skills/), and
[Codex CLI](https://developers.openai.com/codex/build-skills) skills already use,
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

The empty variant is promptsmith with no bundled skills at all (just the
same categories and target definitions, with no skills) - for anyone who
only wants their own skills via `PROMPTSMITH_SKILLS_DIR` and would rather
not carry the built-in set. Get it as a prebuilt `promptsmith-empty_...`
archive from a [release](https://github.com/carlogy/prompt-smith/releases),
via `go install -tags empty ...` (see [Install](#install)), or build it
yourself with `make build-empty` / `make install-empty`. `install-empty`
installs to the same `$GOBIN/promptsmith` path as the default build, so
installing one replaces the other.

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
Linux, a `go build` + `go test -race` portability check across Linux,
macOS, and Windows, and a `release-config` job that validates
`.goreleaser.yaml` (`goreleaser check` plus a snapshot build) so a broken
release config is caught before it's needed. Pushing a tag matching
`v*.*.*` separately runs
[`.github/workflows/release.yml`](.github/workflows/release.yml), which
builds and publishes the actual release - see
[docs/releasing.md](docs/releasing.md) for the maintainer runbook.

## License

promptsmith is licensed under the GNU Affero General Public License
v3.0 or later (AGPL-3.0-or-later); see [LICENSE](LICENSE) for the full
text. It's copyleft: any distributed or network-hosted derivative must
make its complete corresponding source available under the same terms.
Per AGPL §13, if you run a modified promptsmith as a network service
(it has a web server), you must offer your users its source.
