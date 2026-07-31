.PHONY: fmt vet staticcheck build build-empty test test-e2e verify tidy update-golden gosec govulncheck security install install-empty ui-css ui-css-check print-tailwindcss-version release-check release-snapshot release-assert

# fmt fails (non-zero exit) if any file needs gofmt, printing which ones.
fmt:
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needs to be run on:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

vet:
	go vet ./...

staticcheck:
	staticcheck ./...

build:
	go build ./...

# build-empty compiles the "empty" variant (see
# internal/registry/embed_empty.go): the canonical categories/targets
# scaffold with no bundled skills, for users who only want their own via
# PROMPTSMITH_SKILLS_DIR.
build-empty:
	go build -tags empty ./...

test:
	go test ./...

# test-e2e runs internal/server's chromedp-driven browser tests (see
# internal/server/e2e_test.go) inside Dockerfile.e2e - a pinned,
# isolated image bundling an exact Chromium build with the Go
# toolchain, so these tests see the same browser everywhere (a
# contributor's laptop, CI) rather than whatever happens to be
# installed on the host. Excluded from the default `test` target and
# the -race CI matrix since they need Docker and are slower/less
# deterministic than the rest of the suite. Mounts the host's Go
# module cache read-write so repeat runs don't re-download modules.
# See .github/workflows/e2e.yml for the CI job that runs these.
test-e2e:
	docker build -f Dockerfile.e2e -t promptsmith-e2e .
	docker run --rm \
		-v $(CURDIR):/workspace \
		-v $(or $(shell go env GOMODCACHE 2>/dev/null),/tmp/promptsmith-e2e-modcache):/root/go/pkg/mod \
		promptsmith-e2e \
		go test -tags e2e -run TestE2E ./internal/server/...

tidy:
	go mod tidy

# update-golden regenerates golden test fixtures after an intentional
# behavior change (see internal/prompt).
update-golden:
	go test ./... -update

# gosec is a security *scanner* (unsafe patterns, weak perms, etc.) -
# distinct from staticcheck, which is a correctness/style linter and
# doesn't check for this class of issue.
gosec:
	gosec -quiet ./...

# govulncheck checks every dependency (direct and transitive) against the
# Go vulnerability database for known CVEs reachable from this code.
govulncheck:
	govulncheck ./...

security: gosec govulncheck
	@echo "security: no issues found"

install:
	go install ./cmd/promptsmith

# install-empty installs the "empty" variant (see build-empty) in place
# of the default one - both install to the same $GOBIN/promptsmith path,
# so this is a swap, not a side-by-side install.
install-empty:
	go install -tags empty ./cmd/promptsmith

# TAILWINDCSS_VERSION pins the standalone CLI version the committed
# app.css must be reproducible with (see its own first line,
# "tailwindcss v4.3.3") - a different CLI version can produce
# different output bytes for the same input, so this is the single
# source of truth ui-css-check's CI job reads (via
# print-tailwindcss-version below) rather than duplicating the
# version number in .github/workflows/ci.yml. Bump it here if you
# deliberately upgrade the CLI.
TAILWINDCSS_VERSION ?= 4.3.3

# ui-css compiles the web UI's Tailwind input into the committed,
# embedded internal/server/assets/static/app.css - run this after
# editing internal/server/assets/tailwind/input.css or any template
# that changes which Tailwind classes are used, then commit the
# regenerated app.css alongside your change. Requires the Tailwind
# standalone CLI (https://tailwindcss.com/blog/standalone-cli), at
# TAILWINDCSS_VERSION above, on PATH as `tailwindcss` - no Node. Not
# needed at runtime (the built binary just embeds the
# already-committed output), but CI *does* run it now, to check
# app.css is fresh - see ui-css-check below.
#
# app.css itself is first-party build output, not vendored - it does
# NOT belong in THIRD-PARTY-NOTICES.md. The two files this repo
# actually vendors from upstream (htmx.min.js, idiomorph-ext.min.js,
# both in internal/server/assets/static/) do; see that file at the
# repo root, and update it in the same commit as any re-vendor of
# either one.
ui-css:
	tailwindcss \
		-i internal/server/assets/tailwind/input.css \
		-o internal/server/assets/static/app.css \
		--minify

# ui-css-check regenerates app.css and fails (via `git diff
# --exit-code`) if the result differs from what's committed - this is
# what actually enforces app.css freshness; ui-css alone is silent
# about drift. Catches a template edit that changes the compiled
# output without a corresponding `make ui-css` + commit. See
# .github/workflows/ci.yml's ui-css-check job, which installs the
# pinned TAILWINDCSS_VERSION CLI before running this.
ui-css-check: ui-css
	git diff --exit-code -- internal/server/assets/static/app.css

# print-tailwindcss-version prints TAILWINDCSS_VERSION so CI can read
# the pinned version from this one place instead of duplicating it in
# YAML.
print-tailwindcss-version:
	@echo $(TAILWINDCSS_VERSION)

verify: fmt vet staticcheck build test security
	@echo "verify: all checks passed"

# release-check and release-snapshot require goreleaser
# (https://goreleaser.com, see .goreleaser.yaml) on PATH:
#   go install github.com/goreleaser/goreleaser/v2@latest
# Neither is needed for normal development - only when working on the
# release pipeline itself.

# release-check validates .goreleaser.yaml's schema without building
# anything.
release-check:
	goreleaser check

# release-snapshot builds both variants for every target OS/arch
# locally (into dist/, gitignored) without publishing or requiring a
# git tag - use this to sanity-check a .goreleaser.yaml change before
# it runs for real on a tagged release.
release-snapshot:
	goreleaser release --snapshot --clean

# release-assert checks that the binaries in dist/ report a
# well-formed version (see scripts/assert-version.sh) instead of
# silently falling back to "(devel)"/"unknown"/"+dirty" - run this
# after `make release-snapshot` to sanity-check a ldflags/version
# change locally before it runs for real in CI or on a tagged release.
release-assert:
	scripts/assert-version.sh -snapshot-
