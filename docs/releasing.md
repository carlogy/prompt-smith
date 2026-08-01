# Releasing

Maintainer runbook for cutting a promptsmith release. If you're looking
for install instructions instead, see the [README](../README.md#install).

## Versioning policy

promptsmith follows [SemVer](https://semver.org). The first release is
`v0.1.0`. Under SemVer, `0.y.z` makes no API-stability promise - a minor
bump (`0.1.0` -> `0.2.0`) is allowed to change or remove behavior, not
just add to it. Don't rely on `0.y.z` compatibility across minor
versions.

Tags MUST be `vMAJOR.MINOR.PATCH` (optionally with a `-<prerelease>`
suffix, see below). The `v` prefix is required by Go's module tooling,
not a style choice - `0.1.0` (no `v`) and `v.0.1.0` (stray dot after the
`v`) are both invalid; only `v0.1.0` resolves for `go install
module@version`.

At `v2.0.0` and beyond, Go requires the module path itself to carry a
`/v2` suffix (`go.mod`'s `module` line and every import path). That's a
known future task when/if a breaking `v2` happens - nothing to do about
it now, and there's no need to prepare for it ahead of time.

## Cut a release (happy path)

1. Make sure `main` is green (CI passing) and your local tree is clean
   (`git status`).
2. Create an **annotated** tag:

   ```sh
   git tag -a v0.1.0 -m "v0.1.0"
   ```

3. Push it:

   ```sh
   git push origin v0.1.0
   ```

4. [`.github/workflows/release.yml`](../.github/workflows/release.yml)
   picks up the pushed tag and does the rest: builds all 12 archives,
   generates `checksums.txt`, publishes the GitHub release with a
   changelog, and attests provenance for every artifact. Watch the
   [Actions run](https://github.com/carlogy/prompt-smith/actions/workflows/release.yml)
   until it's green.

**Why annotated, not lightweight:** `git describe` (which GoReleaser and
plenty of other tooling use to find "the current version") only
considers annotated tags by default; a lightweight tag can be silently
invisible to that kind of lookup. The annotated tag's message is also
available to release tooling as metadata, even though this project's
actual release notes are generated from commits (see below), not from
the tag message.

Tags are currently unsigned. Signing is optional and not required for
now - if you do want to sign later, SSH signing works fine even when
pushing over HTTPS with a personal access token, since tag signing and
the push transport are unrelated: `git tag -s`/`-a` operates locally
before anything goes over the wire.

## Pre-release rehearsal

To dry-run the full pipeline against a real tag (without it looking like
a normal release), use a prerelease suffix:

```sh
git tag -a v0.1.0-rc.1 -m "v0.1.0-rc.1"
git push origin v0.1.0-rc.1
```

Use the **dotted** `-rc.N` form (`rc.1`, `rc.2`, ..., `rc.10`), not
`rc1`/`rc10`. SemVer compares dot-separated identifiers within a
prerelease segment numerically when they're all digits, but compares
non-numeric-looking segments lexically - `rc1` and `rc10` are strings,
so `rc10` sorts *before* `rc2` lexically. `rc.1` and `rc.10` are separate
numeric identifiers and compare correctly.

GoReleaser's `prerelease: auto` (see
[`.goreleaser.yaml`](../.goreleaser.yaml)) detects the `-rc.1` suffix and
marks the resulting GitHub release as a pre-release automatically - no
extra flag needed.

## Local validation before tagging

Neither of these is needed for normal development - only when changing
the release pipeline itself. Both require
[goreleaser](https://goreleaser.com) on `PATH`:

```sh
go install github.com/goreleaser/goreleaser/v2@latest
```

| Command | What it does |
|---|---|
| `make release-check` | `goreleaser check` - validates `.goreleaser.yaml`'s schema without building anything. |
| `make release-snapshot` | `goreleaser release --snapshot --clean` - builds both variants for all 6 platforms into `dist/` (gitignored). Publishes nothing: no tag, no GitHub release, no token required. |

To spot-check a snapshot build's version stamping:

```sh
make release-snapshot
./dist/promptsmith_darwin_amd64_v1/promptsmith version 2>&1
```

The directory name under `dist/` includes an arch-level suffix
(`_v1` for amd64, `_v8.0` for arm64) that's a GoReleaser implementation
detail and has changed between versions - `ls dist/` if the exact name
above doesn't match. With no tags in the repo yet, a snapshot build's
version looks like `v0.0.0-snapshot-<shortcommit>`; once tags exist, it's
`v<next-version>-snapshot-<shortcommit>` derived from the most recent
tag - either way, `2>&1` is needed because `version` writes to stderr.

## Rollback / kill-switch

If a release goes out broken before anyone has consumed it (e.g. you
tagged and pushed, CI is still running or just finished, and you've
already spotted a problem):

```sh
# delete the tag locally and on the remote
git tag -d v0.1.0
git push origin :refs/tags/v0.1.0

# delete the GitHub release (and its assets)
gh release delete v0.1.0 --cleanup-tag
```

`--cleanup-tag` also removes the tag from GitHub if the local/remote tag
deletion above already happened or you skip it - either order works,
just make sure both the tag and the release are gone.

**Never move or re-point a tag that has already been published and
plausibly fetched by anyone.** The moment a single `go install
module@v0.1.0` or `go get` happens anywhere, Go's module proxy and the
public checksum database (`sum.golang.org`) record the content hash for
that exact version, permanently. If you then delete and re-push `v0.1.0`
pointing at different content, every future consumer's Go toolchain
computes a different hash than the one already recorded and fails with
a checksum mismatch - a security feature (it's designed to catch exactly
this: a version's content silently changing underneath you) that will
look like promptsmith is broken or compromised to anyone who hits it.

If a published release genuinely needs fixing: cut a new patch version
(`v0.1.1`) instead, and optionally add a
[`retract`](https://go.dev/ref/mod#go-mod-file-retract) directive to
`go.mod` for the bad version so `go` tooling warns anyone still depending
on it.

## Release notes / changelog

There is deliberately no hand-maintained `CHANGELOG.md`. Release notes
are generated by GoReleaser from the commit log between the previous tag
and the new one (`changelog.use: git` in
[`.goreleaser.yaml`](../.goreleaser.yaml)), grouped by
[Conventional Commits](https://www.conventionalcommits.org/) prefix:

| Group | Matches |
|---|---|
| Features | `feat:`, `feat(scope):`, `feat!:` |
| Bug fixes | `fix:`, `fix(scope):`, `fix!:` |
| Others | everything else, except... |
| (excluded) | commits starting `docs:` or `test:` are dropped entirely |

Because of this, commit message hygiene on `main` directly determines
release-note quality - a vague commit message shows up verbatim in the
GitHub release. Write commit subjects as if they'll be read by a user
skimming release notes, because they will be.

## What CI enforces

The tag-triggered workflow doesn't just build and upload - after
building, it runs each variant's binary and asserts that `promptsmith
version` reports the pushed tag verbatim, with no `(devel)`, `unknown`,
or `+dirty` markers anywhere in the output. This exists because the
version string is stamped via `-ldflags -X` (see
[`internal/cli/version.go`](../internal/cli/version.go)); if that ever
silently stopped applying (a refactor renames the package, a GoReleaser
config change drops the ldflag, the build tree is unexpectedly dirty),
the release would otherwise ship a binary that reports the wrong version
with nothing catching it until a user noticed. This check fails the
release job instead.

## Troubleshooting

**`promptsmith version` on a published release prints `(devel)`,
`unknown`, or something with `+dirty`.** This should never happen - CI's
version-stamping assertion (above) is specifically designed to catch it
before the release publishes. If you see it anyway, the release run is
suspect: check whether the "Assert version stamping" step in the
[release workflow run](https://github.com/carlogy/prompt-smith/actions/workflows/release.yml)
actually passed, and treat the release as broken (see Rollback above) if
it didn't.

**Pushed a tag but no workflow ran.** The release workflow only triggers
on tags matching the glob `v*.*.*` (see the `on.push.tags` filter in
[`.github/workflows/release.yml`](../.github/workflows/release.yml)).
Check the exact tag name - `0.1.0` (missing `v`), `v0.1` (missing patch
segment), or a typo won't match. Delete the bad tag and re-tag correctly.

**Expected 6 files, got 12 (or vice versa).** Every release publishes
both variants - the default (`promptsmith_...`, all 17 skills) and the
empty one (`promptsmith-empty_...`, no skills) - across 6 platform
combinations (linux/darwin/windows x amd64/arm64), plus `checksums.txt`:
`6 x 2 = 12` archives + 1 checksums file. If someone reports "only 6
files" they likely filtered/searched for one variant's prefix and missed
the other - see the [Install](../README.md#install) table in the README
for which pattern is which.
