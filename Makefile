.PHONY: fmt vet staticcheck build test verify tidy update-golden

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
# behavior change (see internal/assemble).
update-golden:
	go test ./... -update

verify: fmt vet staticcheck build test
	@echo "verify: all checks passed"
