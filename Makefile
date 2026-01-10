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

# Run tests with coverage percentage
test-coverage:
	go test -cover ./...

# Generate coverage profile
coverage:
	go test -coverprofile=coverage.out ./...

# Generate HTML coverage report (requires coverage.out)
coverage-html: coverage
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Clean coverage files
clean:
	rm -f coverage.out coverage.html *.coverprofile *.cover
