.PHONY: fmt vet staticcheck build test verify tidy update-golden gosec govulncheck security install

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

verify: fmt vet staticcheck build test security
	@echo "verify: all checks passed"
