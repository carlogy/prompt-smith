#!/usr/bin/env bash
#
# Guards against a silent ldflags no-op: if internal/cli.version's -X
# override ever stopped matching the package path (e.g. after a
# refactor), or the ldflags in .goreleaser.yaml never applied,
# promptsmith would silently fall back to `go version -m`/runtime/
# debug's pseudo-version heuristics (reporting things like "(devel)",
# "unknown", or a +dirty-suffixed VCS version) instead of the intended
# version string - and, in a real release, that binary would already
# be uploaded as a release asset by the time anyone noticed.
#
# Shared by two callers so this ~35-line check exists exactly once:
#   - .github/workflows/release.yml   (tag push: expects the tag itself)
#   - .github/workflows/ci.yml        (every push/PR: expects the
#     "-snapshot-" marker GoReleaser's snapshot mode stamps in,
#     proving the -X landed at all before any tag exists)
# It's also runnable by hand after `make release-snapshot` (see the
# `release-assert` Makefile target) for local sanity-checking.
#
# Usage: scripts/assert-version.sh <expected-substring> [platform-token]
#   <expected-substring>  required. Substring the `version` output
#                          must contain, e.g. a tag like "v0.1.0" or
#                          the snapshot marker "-snapshot-".
#   [platform-token]      optional, e.g. "linux_amd64". Defaults to
#                          the host's own GOOS_GOARCH so this runs
#                          with no extra arguments on a developer's
#                          machine.
#
# Env:
#   DIST_DIR  optional, default "dist". Override to point at a
#             GoReleaser output directory other than the default.
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <expected-substring> [platform-token]" >&2
  exit 1
fi

expected="$1"
# Defaulting to the host's own GOOS_GOARCH is what lets a developer
# run this locally on darwin/arm64 with no arguments beyond the
# expected string - CI passes linux_amd64 explicitly since that's the
# platform CI runs the snapshot/release build on.
platform="${2:-$(go env GOOS)_$(go env GOARCH)}"
dist_dir="${DIST_DIR:-dist}"

# Unmatched globs expand to nothing (instead of the literal pattern
# string) so a missing directory is caught by the match-count check
# below rather than by a confusing "command not found" from trying to
# execute a glob string.
shopt -s nullglob

# assert_version checks one build variant's binary reports the
# expected version string and none of the known "ldflags didn't
# apply" markers.
#
# GoReleaser's unpacked build dirs are named
# <dist_dir>/<build-id>_<goos>_<goarch>[_<arch-level>] - the
# arch-level suffix (_v1 for amd64, _v8.0 for arm64) is a GoReleaser
# implementation detail that has changed between versions, so it's
# globbed here rather than hardcoded.
assert_version() {
  local label="$1"
  local pattern="$2"
  # shellcheck disable=SC2206 # intentional glob expansion, not word-splitting
  local matches=( $pattern )

  if [[ "${#matches[@]}" -ne 1 ]]; then
    echo "::error::${label}: expected exactly one match for pattern '${pattern}', found ${#matches[@]}: ${matches[*]:-<none>}"
    exit 1
  fi

  local binary="${matches[0]}"
  local actual
  local status=0
  # cobra's cmd.Printf falls back to stderr when SetOut isn't called
  # - capture both streams so this assertion doesn't silently pass an
  # empty stdout while the real output sits on fd 2. Capture the exit
  # status explicitly via `||` (rather than letting `set -e` abort the
  # script on a nonzero return from the command substitution) so a
  # binary that can't even run (wrong architecture, not executable,
  # corrupt) produces a clear ::error:: with the binary path, exit
  # code, and whatever output it managed to produce - not a bare exit
  # 126 with no context, which is exactly the kind of failure this
  # script exists to diagnose.
  actual="$("${binary}" version 2>&1)" || status=$?
  if [[ "${status}" -ne 0 ]]; then
    echo "::error::${label}: failed to execute '${binary}' (exit ${status}): ${actual:-<no output>}"
    exit 1
  fi

  echo "${label}: expected='${expected}' actual='${actual}'"

  if [[ -z "${actual}" ]]; then
    echo "::error::${label}: version output is empty (expected '${expected}')"
    exit 1
  fi

  for marker in '(devel)' 'unknown' '+dirty'; do
    if [[ "${actual}" == *"${marker}"* ]]; then
      echo "::error::${label}: version output contains forbidden marker '${marker}': actual='${actual}' expected='${expected}'"
      exit 1
    fi
  done

  if [[ "${actual}" != *"${expected}"* ]]; then
    echo "::error::${label}: version output does not contain the expected substring: actual='${actual}' expected='${expected}'"
    exit 1
  fi
}

assert_version "promptsmith (default)" "${dist_dir}/promptsmith_${platform}*/promptsmith"
assert_version "promptsmith-empty" "${dist_dir}/promptsmith-empty_${platform}*/promptsmith"
