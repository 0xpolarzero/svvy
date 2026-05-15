.PHONY: all test build lint clean install-tools help
.DEFAULT_GOAL := help

all: lint test build

# Show help message with available targets
help:
	@echo "Codex Go - Available Make Targets"
	@echo ""
	@echo "Building:"
	@echo "  make build              - Build the TUI binary"
	@echo "  make all                - Run lint, test, and build"
	@echo ""
	@echo "Testing:"
	@echo "  make test               - Run tests with coverage"
	@echo "  make test-verbose       - Run tests with verbose output"
	@echo "  make test-unit          - Run only unit tests (fast)"
	@echo "  make test-coverage      - Generate coverage report"
	@echo "  make test-race          - Run tests with race detector"
	@echo "  make test-timeout       - Run tests with 30s timeout"
	@echo "  make bench              - Run benchmarks"
	@echo ""
	@echo "Golden Files:"
	@echo "  make golden-update      - Update golden test files"
	@echo "  make update-golden      - Alias for golden-update"
	@echo ""
	@echo "Mocks:"
	@echo "  make generate-mocks     - Generate mocks from interfaces"
	@echo "  make mocks              - Alias for generate-mocks"
	@echo "  make clean-mocks        - Remove generated mocks"
	@echo "  make regen-mocks        - Clean and regenerate all mocks"
	@echo ""
	@echo "Code Quality:"
	@echo "  make lint               - Run linters"
	@echo "  make fmt                - Format code"
	@echo ""
	@echo "Dependencies:"
	@echo "  make deps               - Download and tidy dependencies"
	@echo "  make install-tools      - Install development tools"
	@echo ""
	@echo "Cleanup:"
	@echo "  make clean              - Remove build artifacts"
	@echo "  make clean-mocks        - Remove generated mocks"
	@echo ""

# Run tests with coverage
test:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Run tests with verbose output
test-verbose:
	go test -v -race ./...

# Run only unit tests (fast)
test-unit:
	go test -v -short ./...

# Build the TUI binary
build:
	go build -o bin/codex ./cmd/codex

# Install development tools
install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install go.uber.org/mock/mockgen@latest
	go install golang.org/x/tools/cmd/goimports@latest

# Run linters
lint:
	golangci-lint run ./...

# Format code
fmt:
	gofmt -s -w .
	goimports -w .

# Clean build artifacts
clean:
	rm -rf bin/ coverage.out coverage.html
	go clean -cache -testcache

# Run golden tests and update snapshots
golden-update:
	go test -update ./...

# Update golden files (alias)
update-golden: golden-update

# Test with coverage and generate HTML report
test-coverage:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run tests with race detector
test-race:
	go test -v -race ./...

# Run benchmarks
bench:
	go test -bench=. -benchmem ./...

# Run tests with specific timeout
test-timeout:
	go test -v -timeout 30s ./...

# Download dependencies
deps:
	go mod download
	go mod tidy

# Generate mocks for all interfaces
generate-mocks:
	@echo "Generating mocks..."
	go generate ./...
	@echo "Mocks generated successfully"

# Alias for generate-mocks
mocks: generate-mocks

# Clean generated mocks
clean-mocks:
	@echo "Cleaning generated mocks..."
	find ./test/mocks -name "mock_*.go" -type f -delete
	@echo "Mocks cleaned"

# Regenerate all mocks (clean + generate)
regen-mocks: clean-mocks generate-mocks
