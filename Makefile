.PHONY: test test-coverage coverage coverage-html clean help

# Default target
help:
	@echo "Available targets:"
	@echo "  test          - Run all tests"
	@echo "  test-coverage - Run tests with coverage percentage"
	@echo "  coverage      - Generate coverage profile (coverage.out)"
	@echo "  coverage-html - Generate HTML coverage report (coverage.html)"
	@echo "  clean         - Remove coverage files"

# Run all tests
test:
	go test ./...

# Run tests with coverage percentage (exclude cmd package as it has no tests)
test-coverage:
	@go list ./... | grep -v '/cmd$$' | xargs go test -cover

# Alternative: test all packages including cmd (may fail on Go 1.25+)
test-coverage-all:
	go test -cover ./...

# Generate coverage profile (exclude cmd package as it has no tests)
coverage:
	@echo "mode: atomic" > coverage.out
	@go list ./... | grep -v '/cmd$$' | while read pkg; do \
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

# Clean coverage files
clean:
	rm -f coverage.out coverage.html coverage.tmp *.coverprofile *.cover
