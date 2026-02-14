.PHONY: test test-js test-update-golden test-coverage coverage coverage-html clean help build-dev release-check lint security housekeeping tools format install-web build-web test-web clean-web

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
	@echo "  test               - Run all tests (Go + frontend)"
	@echo "  test-js            - Run JavaScript unit tests (legacy)"
	@echo "  test-web           - Run frontend tests (Vitest)"
	@echo "  test-update-golden - Update golden test fixtures"
	@echo "  test-coverage      - Run tests with coverage percentage"
	@echo "  coverage           - Generate coverage profile (coverage.out)"
	@echo "  coverage-html      - Generate HTML coverage report (coverage.html)"
	@echo ""
	@echo "Building:"
	@echo "  install-web        - Install frontend dependencies"
	@echo "  build-web          - Build frontend assets"
	@echo "  build-dev          - Build for current platform with CGO (includes frontend)"
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
security: tools
	$(TOOLS_BIN)/govulncheck ./...

# Normalize module dependencies
housekeeping:
	go mod tidy

# Run all tests
test:
	go test ./...
	$(MAKE) test-web

# Legacy JS tests (will be removed after migration)
test-js:
	node --test cmd/watch/viewer_state.test.mjs cmd/watch/viewer_protocol.test.mjs

# Frontend tests using Vitest
test-web:
	cd cmd/watch/web && npm test

# Update golden test fixtures (only packages using goldie)
test-update-golden:
	go test ./tests/litmus/... ./tests/integration/graph/... ./tests/languagespecs/java/tests/... ./cmd/graph/formatters/dot/... ./cmd/graph/formatters/mermaid/... -args -update

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

# Install frontend dependencies
install-web:
	cd cmd/watch/web && npm install

# Build frontend assets
build-web:
	cd cmd/watch/web && npm run build

# Build for current platform only (RECOMMENDED for local testing)
# No cross-compilation, no GoReleaser, no Zig required
build-dev: build-web
	@echo "Building for current platform with version: $(VERSION), commit: $(COMMIT)"
	CGO_ENABLED=1 go build -ldflags "-s -w -X github.com/LegacyCodeHQ/clarity/cmd.version=$(VERSION) -X github.com/LegacyCodeHQ/clarity/cmd.buildDate=$(BUILD_DATE) -X github.com/LegacyCodeHQ/clarity/cmd.commit=$(COMMIT) -X github.com/LegacyCodeHQ/clarity/cmd.enableDevCommands=true" -o clarity ./main.go
	@echo ""
	@echo "Build successful! Run './clarity --version' to test"

# Validate the GoReleaser configuration
release-check:
	goreleaser check

# Clean frontend build artifacts
clean-web:
	rm -rf cmd/watch/web/node_modules cmd/watch/dist

# Clean coverage files and binary
clean: clean-web
	rm -f coverage.out coverage.html coverage.tmp *.coverprofile *.cover clarity
	rm -rf dist/
