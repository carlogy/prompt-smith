.PHONY: fmt vet staticcheck build build-empty test verify tidy update-golden gosec govulncheck security install install-empty ui-css

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

# ui-css compiles the web UI's Tailwind input into the committed,
# embedded internal/server/assets/static/app.css - run this after
# editing internal/server/assets/tailwind/input.css or any template
# that changes which Tailwind classes are used, then commit the
# regenerated app.css alongside your change. Requires the Tailwind
# standalone CLI (https://tailwindcss.com/blog/standalone-cli) on
# PATH as `tailwindcss` - no Node, and not needed at runtime or in CI,
# since the built binary just embeds the already-committed output.
ui-css:
	tailwindcss \
		-i internal/server/assets/tailwind/input.css \
		-o internal/server/assets/static/app.css \
		--minify

verify: fmt vet staticcheck build test security
	@echo "verify: all checks passed"
