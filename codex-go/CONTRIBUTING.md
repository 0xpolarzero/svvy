# Contributing to Codex Go

Thank you for your interest in contributing to the Codex Go rewrite! This document provides guidelines and instructions for contributors.

## Table of Contents

- [Development Philosophy](#development-philosophy)
- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Testing Guidelines](#testing-guidelines)
- [Code Review Process](#code-review-process)
- [Commit Guidelines](#commit-guidelines)

## Development Philosophy

This project follows **Test-Driven Development (TDD)** principles:

1. Write tests first before implementing functionality
2. Write minimal code to make tests pass
3. Refactor while keeping tests green
4. Use golden files for complex outputs (JSON, ANSI)
5. Mock external dependencies (HTTP, filesystem, exec)

### Why TDD?

- **Design first**: Writing tests forces us to think about API design
- **Confidence**: Tests provide safety net for refactoring
- **Documentation**: Tests serve as executable documentation
- **Quality**: Catches bugs early in development cycle

## Getting Started

### Prerequisites

- Go 1.23 or later
- Make
- Git

### Initial Setup

1. Clone the repository:
   ```bash
   cd /path/to/codex/codex-go
   ```

2. Install development tools:
   ```bash
   make install-tools
   ```

3. Download dependencies:
   ```bash
   make deps
   ```

4. Verify setup:
   ```bash
   make test
   make lint
   make build
   ```

## Development Workflow

### 1. Create a Feature Branch

```bash
git checkout -b feature/your-feature-name
```

### 2. Follow the TDD Cycle

For each new feature or bug fix:

#### Red Phase - Write Failing Test

```bash
# Create or edit test file
vim internal/mypackage/mypackage_test.go
```

Example test structure:
```go
func TestMyFeature(t *testing.T) {
    // Arrange
    input := "test input"
    expected := "expected output"

    // Act
    result := MyFeature(input)

    // Assert
    require.Equal(t, expected, result)
}
```

Run tests to verify they fail:
```bash
make test
```

#### Green Phase - Implement Minimal Code

```bash
# Implement feature
vim internal/mypackage/mypackage.go
```

Run tests to verify they pass:
```bash
make test
```

#### Refactor Phase - Improve Code

- Clean up implementation
- Remove duplication
- Improve naming
- Keep tests passing throughout

```bash
make test  # Run after each change
```

### 3. Working with Golden Files

Golden files capture expected output for complex data structures.

#### Creating Golden Tests

```go
import "github.com/evmts/codex/codex-go/test"

func TestComplexOutput(t *testing.T) {
    output := GenerateComplexOutput()
    test.GoldenJSON(t, "complex-output", output)
}
```

#### Updating Golden Files

When output legitimately changes:

```bash
# Update all golden files
make golden-update

# Or update specific test
go test ./internal/mypackage -update
```

Review the diff carefully before committing:
```bash
git diff test/testdata/golden/
```

### 4. Running Tests

```bash
# Run all tests with coverage
make test

# Run tests in watch mode (requires entr)
find . -name "*.go" | entr -c make test

# Run specific package tests
go test -v ./internal/mypackage

# Run specific test
go test -v ./internal/mypackage -run TestMyFeature

# Run only fast unit tests
make test-unit

# Run with verbose output
make test-verbose
```

### 5. Linting and Formatting

Before committing:

```bash
# Format code
make fmt

# Run linters
make lint
```

Fix any issues reported by the linter.

### 6. Building

```bash
# Build binary
make build

# Test binary
./bin/codex
```

## Testing Guidelines

### Test Organization

```
internal/mypackage/
├── mypackage.go           # Implementation
├── mypackage_test.go      # Unit tests
└── testdata/              # Test fixtures
    ├── fixtures/          # Input test data
    └── golden/            # Expected output
```

### Test Naming Conventions

```go
// Good test names
func TestUserRepository_Create_ValidUser_ReturnsNoError(t *testing.T)
func TestMessageParser_Parse_InvalidJSON_ReturnsError(t *testing.T)

// Pattern: Test{Type}_{Method}_{Scenario}_{ExpectedResult}
```

### Using Test Helpers

```go
import "github.com/evmts/codex/codex-go/test"

// Temporary directories
tmpDir := test.TempDir(t)

// Load test fixtures
data := test.LoadFixture(t, "example.json")

// Golden file comparison
test.GoldenJSON(t, "output", result)
test.GoldenText(t, "rendered", output)
```

### Mocking External Dependencies

Use interfaces for dependencies and mock them in tests:

```go
// Define interface
type HTTPClient interface {
    Do(req *http.Request) (*http.Response, error)
}

// Mock implementation
type MockHTTPClient struct {
    mock.Mock
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
    args := m.Called(req)
    return args.Get(0).(*http.Response), args.Error(1)
}
```

### Table-Driven Tests

For testing multiple scenarios:

```go
func TestParser(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected Result
        wantErr  bool
    }{
        {
            name:     "valid input",
            input:    "valid",
            expected: Result{Value: "valid"},
            wantErr:  false,
        },
        {
            name:     "invalid input",
            input:    "",
            expected: Result{},
            wantErr:  true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := Parse(tt.input)

            if tt.wantErr {
                require.Error(t, err)
                return
            }

            require.NoError(t, err)
            require.Equal(t, tt.expected, result)
        })
    }
}
```

## Code Review Process

### Submitting a Pull Request

1. Ensure all tests pass:
   ```bash
   make test
   ```

2. Ensure linting passes:
   ```bash
   make lint
   ```

3. Ensure code is formatted:
   ```bash
   make fmt
   ```

4. Commit changes with descriptive messages:
   ```bash
   git add .
   git commit -m "Add feature: description"
   ```

5. Push to your branch:
   ```bash
   git push origin feature/your-feature-name
   ```

6. Open a Pull Request on GitHub with:
   - Clear description of changes
   - Link to related issues
   - Screenshots/examples if applicable
   - Test coverage information

### Review Checklist

Reviewers will check:

- [ ] Tests are included and passing
- [ ] Code follows Go best practices
- [ ] Public APIs have documentation comments
- [ ] Error handling is appropriate
- [ ] No unnecessary complexity
- [ ] Golden files are reviewed if updated
- [ ] CI pipeline passes

### Addressing Review Feedback

1. Make requested changes
2. Run tests and linting
3. Push updates to the same branch
4. Respond to comments explaining changes

## Commit Guidelines

### Commit Message Format

```
<type>: <subject>

<body>

<footer>
```

### Types

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `test`: Adding or updating tests
- `refactor`: Code refactoring
- `perf`: Performance improvements
- `chore`: Maintenance tasks

### Examples

```
feat: add streaming message parser

Implements parser for SSE streaming events from API.
Includes support for partial JSON parsing and reconnection.

Fixes #123
```

```
test: add golden tests for ANSI renderer

Adds comprehensive golden file tests for terminal rendering
to prevent visual regressions.
```

## Getting Help

- Check existing issues and PRs
- Review documentation in `docs/`
- Ask questions in pull request discussions
- Join project discussions

## Code of Conduct

Be respectful, professional, and constructive in all interactions.

Thank you for contributing to Codex Go!
