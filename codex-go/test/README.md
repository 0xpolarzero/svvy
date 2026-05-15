# Codex Go Test Infrastructure

Comprehensive test utilities and mock generation setup for the Codex Go rewrite.

## Overview

This directory contains:
- **testhelpers.go** - Reusable test helper functions
- **mocks.go** - Mock generation instructions and utilities
- **testdata/** - Test fixtures and golden files
- **mocks/** - Auto-generated mock implementations
- **testhelpers_example_test.go** - Usage examples

## Quick Start

### Running Tests

```bash
# Run all tests
make test

# Run tests with verbose output
make test-verbose

# Run only fast unit tests
make test-unit

# Run with race detector
make test-race

# Generate coverage report
make test-coverage
```

### Generating Mocks

```bash
# Generate all mocks
make generate-mocks

# Clean and regenerate mocks
make regen-mocks

# Clean only
make clean-mocks
```

### Golden Files

```bash
# Update all golden files
make golden-update

# Or use go test directly
go test -update ./...
```

## Test Helpers

### HTTP Mock Server

Create mock HTTP servers for testing API clients:

```go
// Basic mock server
mockServer := test.NewHTTPMockServer(t, func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("response"))
})

// JSON mock server
response := map[string]string{"status": "ok"}
mockServer := test.NewJSONMockServer(t, http.StatusOK, response)

// Verify requests
mockServer.AssertRequestCount(t, 1)
mockServer.AssertRequestMethod(t, 0, "GET")
mockServer.AssertRequestPath(t, 0, "/api/v1/endpoint")
body := mockServer.GetRequestBody(t, 0)
```

### Filesystem Mocks

Use in-memory or temporary filesystems for testing:

```go
// In-memory filesystem (fast, no cleanup needed)
fs := test.NewMemFS(t)
test.WriteFileFS(t, fs, "/path/to/file.txt", []byte("content"))
test.AssertFileExistsFS(t, fs, "/path/to/file.txt")
data := test.ReadFileFS(t, fs, "/path/to/file.txt")

// Real OS filesystem (with automatic cleanup)
fs, tmpDir := test.NewOsFS(t)
test.WriteFileFS(t, fs, "file.txt", []byte("content"))
```

### Context Helpers

Create contexts with automatic cleanup:

```go
// Context with timeout
ctx := test.ContextWithTimeout(t, 5*time.Second)

// Context with deadline
ctx := test.ContextWithDeadline(t, time.Now().Add(5*time.Second))

// Cancellable context
ctx, cancel := test.ContextWithCancel(t)
defer cancel()

// Short context (100ms) for fast tests
ctx := test.ShortContext(t)

// Long context (5s) for integration tests
ctx := test.LongContext(t)
```

### Async Operation Assertions

Wait for async operations to complete:

```go
// Wait for condition to be true
test.Eventually(t, func() bool {
    return someCondition == true
}, 1*time.Second, "condition should become true")

// Wait for channel to receive value
result := test.WaitForChannel(t, ch, 1*time.Second)

// Wait for specific value
value := test.AssertChannelReceives(t, ch, 1*time.Second, "should receive value")

// Assert channel is empty
test.AssertChannelEmpty(t, ch, "channel should be empty")
```

### Fixture Loading

Load test data from fixtures:

```go
// Load raw fixture
data := test.LoadFixture(t, "example.json")

// Load and unmarshal JSON fixture
var config Config
test.LoadFixtureJSON(t, "config_minimal.json", &config)
```

Available fixtures:
- `config_minimal.json` - Minimal configuration
- `config_full.json` - Full configuration with all options
- `op_initialize.json` - Initialize operation
- `op_send_message.json` - Send message operation
- `event_message_received.json` - Message received event
- `event_stream_start.json` - Stream start event
- `event_stream_chunk.json` - Stream chunk event
- `conversation_simple.json` - Simple conversation history
- `conversation_with_tools.json` - Conversation with tool calls
- `error_invalid_json.json` - Invalid JSON error
- `error_not_found.json` - Not found error

### Golden File Testing

Compare outputs with golden files:

```go
// JSON golden files
result := map[string]interface{}{"status": "success"}
test.GoldenJSON(t, "test_name", result)

// Text golden files
output := "Hello, World!"
test.GoldenText(t, "test_name", output)

// Update golden files when output changes
// go test -update ./...
```

### Command Mocking

Mock command executions:

```go
mocker := test.NewCommandMocker()

// Mock successful command
mocker.MockSuccess("git", "commit abc123")

// Mock failed command
mocker.MockError("docker", "error: daemon not running", 1)

// Get mock
cmd, ok := mocker.Get("git")
if ok {
    fmt.Println(cmd.Stdout) // "commit abc123"
    fmt.Println(cmd.ExitCode) // 0
}
```

### Additional Helpers

```go
// Create temporary directory (auto cleanup)
dir := test.TempDir(t)

// Write file to temp directory
path := test.WriteFile(t, dir, "test.txt", []byte("content"))

// Enhanced assertions
test.AssertNoError(t, err, "operation name")
test.AssertError(t, err, "operation name")
test.AssertErrorContains(t, err, "substring", "operation name")

// JSON helpers
data := test.MustMarshalJSON(t, obj)
test.MustUnmarshalJSON(t, data, &dest)

// Test control
test.RunParallel(t) // Mark test as parallel
test.SkipInShort(t, "reason") // Skip when -short flag
test.SkipInCI(t, "reason") // Skip in CI environment
```

## Mock Generation

### Overview

Mocks are generated using [go.uber.org/mock](https://github.com/uber-go/mock) (formerly gomock).

### Setting Up Mocks

1. **Define an interface** in your production code:

```go
package mypackage

//go:generate mockgen -destination=../test/mocks/mock_repository.go -package=mocks . Repository

type Repository interface {
    Get(ctx context.Context, id string) (*Entity, error)
    Save(ctx context.Context, entity *Entity) error
    Delete(ctx context.Context, id string) error
}
```

2. **Generate mocks**:

```bash
make generate-mocks
```

3. **Use mocks in tests**:

```go
package mypackage_test

import (
    "testing"
    "github.com/evmts/codex/codex-go/test/mocks"
    "go.uber.org/mock/gomock"
)

func TestMyFunction(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()

    mockRepo := mocks.NewMockRepository(ctrl)
    mockRepo.EXPECT().
        Get(gomock.Any(), "123").
        Return(&Entity{ID: "123"}, nil)

    // Use mockRepo in your test
    result, err := MyFunction(mockRepo)
    require.NoError(t, err)
    assert.Equal(t, "123", result.ID)
}
```

### Mock Patterns

**Basic expectation:**
```go
mock.EXPECT().Method(arg1, arg2).Return(result, nil)
```

**Any arguments:**
```go
mock.EXPECT().Method(gomock.Any(), gomock.Any()).Return(result, nil)
```

**Multiple calls:**
```go
mock.EXPECT().Method(arg).Return(result).Times(3)
```

**Ordered calls:**
```go
gomock.InOrder(
    mock.EXPECT().Method1().Return(result1),
    mock.EXPECT().Method2().Return(result2),
)
```

**Call count constraints:**
```go
mock.EXPECT().Method(arg).Return(result).MinTimes(1)
mock.EXPECT().Method(arg).Return(result).MaxTimes(5)
```

**Custom matchers:**
```go
mock.EXPECT().Method(gomock.Eq(expected)).Return(result)
mock.EXPECT().Method(gomock.Not(gomock.Nil())).Return(result)
mock.EXPECT().Method(test.StringContains("substring")).Return(result)
```

**Custom actions:**
```go
mock.EXPECT().Method(gomock.Any()).Do(func(arg string) {
    // Custom validation or side effects
}).Return(result)
```

## Directory Structure

```
test/
├── README.md                           # This file
├── testhelpers.go                      # Test helper functions
├── testhelpers_example_test.go         # Usage examples
├── mocks.go                            # Mock utilities and docs
├── mocks/                              # Generated mocks
│   ├── README.md                       # Mocks directory documentation
│   └── mock_*.go                       # Generated mock files (auto-generated)
└── testdata/                           # Test data
    ├── README.md                       # Test data documentation
    ├── fixtures/                       # Reusable test input data
    │   ├── config_minimal.json
    │   ├── config_full.json
    │   ├── op_initialize.json
    │   ├── op_send_message.json
    │   ├── event_*.json
    │   ├── conversation_*.json
    │   └── error_*.json
    ├── golden/                         # Expected output files
    │   └── *.json / *.txt              # Golden files (test-generated)
    └── protocol/                       # Protocol test fixtures
        ├── raw_initialize.json
        ├── raw_notification.json
        ├── raw_request.json
        └── raw_response.json
```

## Best Practices

### Test Organization

1. **Use table-driven tests** for multiple scenarios
2. **Run tests in parallel** when possible with `test.RunParallel(t)`
3. **Use subtests** for better organization with `t.Run()`
4. **Keep tests focused** - one test, one concept

### Mocking

1. **Mock at boundaries** - HTTP clients, file system, external services
2. **Use interfaces** - design for testability
3. **Avoid over-mocking** - test real code when possible
4. **Keep mocks simple** - complex mocks indicate design issues

### Fixtures

1. **Keep fixtures minimal** - only include necessary data
2. **Use realistic data** - but anonymize sensitive information
3. **Version control fixtures** - they're part of your tests
4. **Document complex fixtures** - add .md files when needed

### Golden Files

1. **Review changes carefully** - golden file updates affect test behavior
2. **Commit golden files** - they define expected behavior
3. **Use for complex outputs** - JSON, HTML, formatted text
4. **Update intentionally** - don't blindly accept changes

## Dependencies

Required test dependencies (already in go.mod):

- `github.com/stretchr/testify` - Assertions and test utilities
- `go.uber.org/mock` - Mock generation (mockgen)
- `github.com/spf13/afero` - Filesystem abstraction for testing

## Installation

Install development tools:

```bash
make install-tools
```

This installs:
- `golangci-lint` - Linter
- `mockgen` - Mock generator
- `goimports` - Import formatter

## Examples

See `testhelpers_example_test.go` for comprehensive examples of all test helpers.

Run examples:

```bash
go test -v ./test/... -run=Test
```

## Contributing

When adding new test helpers:

1. Add the helper to `testhelpers.go`
2. Add usage example to `testhelpers_example_test.go`
3. Document in this README
4. Ensure helpers use `t.Helper()` for better error messages

## Troubleshooting

**Mocks not generating?**
- Check `go:generate` directive syntax
- Run `make generate-mocks` explicitly
- Verify mockgen is installed: `which mockgen`

**Tests failing in CI but passing locally?**
- Check for timing issues - use generous timeouts
- Verify test isolation - avoid shared state
- Use `test.SkipInCI(t, "reason")` for environment-specific tests

**Golden files showing differences?**
- Review changes: `git diff test/testdata/golden/`
- Update if intentional: `make golden-update`
- Commit updated golden files

## Support

For issues or questions:
1. Check existing tests in `testhelpers_example_test.go`
2. Review this documentation
3. Check the original Rust implementation for reference
4. Consult the team
