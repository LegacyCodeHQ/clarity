.PHONY: test test-coverage coverage coverage-html clean help build-dev release-check lint

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
	@echo "  test             - Run all tests"
	@echo "  test-coverage    - Run tests with coverage percentage"
	@echo "  coverage         - Generate coverage profile (coverage.out)"
	@echo "  coverage-html    - Generate HTML coverage report (coverage.html)"
	@echo "  lint             - Run golangci-lint"
	@echo ""
	@echo "Building:"
	@echo "  build-dev      - Build for current platform with CGO"
	@echo ""
	@echo "Releasing:"
	@echo "  release-check    - Validate GoReleaser configuration"
	@echo "  (For actual releases: push a git tag to trigger GitHub Actions)"
	@echo ""
	@echo "Cleanup:"
	@echo "  clean            - Remove coverage files and binary"

# Run all tests
test:
	go test ./...

# Run linter
lint:
	golangci-lint run ./...

# Run tests with coverage percentage (exclude cmd packages as they have no tests)
test-coverage:
	@go list ./... | grep -Ev '/cmd($$|/)' | xargs go test -cover

# Alternative: test all packages including cmd (may fail on Go 1.25+)
test-coverage-all:
	go test -cover ./...

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

# Clean coverage files and binary
clean:
	rm -f coverage.out coverage.html coverage.tmp *.coverprofile *.cover sanity
	rm -rf dist/

# Validate the GoReleaser configuration
release-check:
	goreleaser check

# Build for current platform only (RECOMMENDED for local testing)
# No cross-compilation, no GoReleaser, no Zig required
build-dev:
	@echo "Building for current platform with version: $(VERSION), commit: $(COMMIT)"
	CGO_ENABLED=1 go build -ldflags "-s -w -X github.com/LegacyCodeHQ/sanity/cmd.version=$(VERSION) -X github.com/LegacyCodeHQ/sanity/cmd.buildDate=$(BUILD_DATE) -X github.com/LegacyCodeHQ/sanity/cmd.commit=$(COMMIT)" -o sanity ./main.go
	@echo ""
	@echo "Build successful! Run './sanity --version' to test"
