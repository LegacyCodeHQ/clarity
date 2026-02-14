.PHONY: help tools format lint vulncheck security housekeeping test test-go test-js test-update-golden test-coverage coverage coverage-html check build-dev release-check clean

# Version information (can be overridden via command line)
# Try to get version from git tag, otherwise use "dev"
GIT_TAG := $(shell git describe --tags --exact-match 2>/dev/null || git describe --tags 2>/dev/null || echo "")
VERSION ?= $(if $(GIT_TAG),$(GIT_TAG),dev)
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
# Check for uncommitted changes (staged, unstaged, or untracked) and add -dirty suffix if present
DIRTY_CHECK := $(shell test -z "$$(git status --porcelain 2>/dev/null)" || echo "-dirty")
COMMIT := $(COMMIT)$(DIRTY_CHECK)

# Default target
help:
	@echo "Available targets:"
	@echo ""
	@echo "Testing:"
	@echo "  tools              - Install/update local tooling (bin/)"
	@echo "  format             - Run gofmt -s on all Go files"
	@echo "  lint               - Run golangci-lint"
	@echo "  vulncheck          - Run govulncheck"
	@echo "  housekeeping       - Run go mod tidy"
	@echo "  test               - Run all Go and JavaScript tests"
	@echo "  test-go            - Run Go unit and integration tests"
	@echo "  test-js            - Run watch UI JavaScript tests"
	@echo "  test-update-golden - Update golden test fixtures"
	@echo "  test-coverage      - Run tests with coverage percentage"
	@echo "  coverage           - Generate coverage profile (coverage.out)"
	@echo "  coverage-html      - Generate HTML coverage report (coverage.html)"
	@echo "  check              - Run lint, tests, and build (CI parity)"
	@echo ""
	@echo "Building:"
	@echo "  build-dev          - Build for current platform with CGO"
	@echo ""
	@echo "Releasing:"
	@echo "  release-check      - Validate GoReleaser configuration"
	@echo "    (For actual releases: push a git tag to trigger GitHub Actions)"
	@echo ""
	@echo "Cleanup:"
	@echo "  clean              - Remove coverage files and binary"

# Tooling (installed locally into ./.gotools)
TOOLS_BIN := $(CURDIR)/.gotools
GOLANGCI_LINT_VERSION ?= latest
GO_CONSISTENT_VERSION ?= latest
GOVULNCHECK_VERSION ?= latest

tools:
	@mkdir -p $(TOOLS_BIN)
	@[ -x $(TOOLS_BIN)/golangci-lint ] || GOBIN=$(TOOLS_BIN) go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	@[ -x $(TOOLS_BIN)/go-consistent ] || GOBIN=$(TOOLS_BIN) go install github.com/quasilyte/go-consistent@$(GO_CONSISTENT_VERSION)
	@[ -x $(TOOLS_BIN)/govulncheck ] || GOBIN=$(TOOLS_BIN) go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)

# Format Go source files
format:
	gofmt -s -w $$(find . -name '*.go' -not -path './.git/*')

# Run linters
lint: tools
	$(TOOLS_BIN)/golangci-lint run ./...
	$(TOOLS_BIN)/go-consistent ./...

# Run govulncheck
vulncheck: tools
	$(TOOLS_BIN)/govulncheck ./...

# Backward-compatible alias
security: vulncheck

# Normalize module dependencies
housekeeping:
	go mod tidy

# Run all tests
test:
	$(MAKE) test-go
	$(MAKE) test-js

test-go:
	go test ./...

test-js:
	node --test cmd/watch/viewer_state.test.mjs cmd/watch/viewer_protocol.test.mjs

# Update golden test fixtures (only packages using goldie)
test-update-golden:
	go test ./tests/litmus/... ./tests/integration/graph/... ./tests/languagespecs/java/tests/... ./cmd/languages/... ./cmd/show/formatters/... ./cmd/watch/... ./cmd/why/... -args -update

# Full product quality gate (used locally and in CI)
check:
	$(MAKE) lint
	$(MAKE) test
	$(MAKE) build-dev

# Run tests with coverage percentage (excludes cmd packages which have no tests)
test-coverage:
	@go list ./... | grep -Ev '/cmd($$|/)' | xargs go test -cover

# Generate coverage profile (exclude cmd packages as they have no tests)
coverage:
	@echo "mode: atomic" > coverage.out
	@go list ./... | grep -Ev '/cmd($$|/)' | while read pkg; do \
		go test -coverprofile=coverage.tmp -covermode=atomic $$pkg || true; \
		if [ -f coverage.tmp ]; then \
			tail -n +2 coverage.tmp >> coverage.out; \
			rm coverage.tmp; \
		fi; \
	done

# Generate HTML coverage report (requires coverage.out)
coverage-html: coverage
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Build for current platform only (RECOMMENDED for local testing)
# No cross-compilation, no GoReleaser, no Zig required
build-dev:
	@echo "Building for current platform with version: $(VERSION), commit: $(COMMIT)"
	CGO_ENABLED=1 go build -ldflags "-s -w -X github.com/LegacyCodeHQ/clarity/cmd.version=$(VERSION) -X github.com/LegacyCodeHQ/clarity/cmd.buildDate=$(BUILD_DATE) -X github.com/LegacyCodeHQ/clarity/cmd.commit=$(COMMIT) -X github.com/LegacyCodeHQ/clarity/cmd.enableDevCommands=true" -o clarity ./main.go
	@echo ""
	@echo "Build successful! Run './clarity --version' to test"

# Validate the GoReleaser configuration
release-check:
	goreleaser check

# Clean coverage files and binary
clean:
	rm -f coverage.out coverage.html coverage.tmp *.coverprofile *.cover clarity
	rm -rf dist/
