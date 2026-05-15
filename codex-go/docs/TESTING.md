# Testing Strategy

This document describes the testing approach, patterns, and best practices for Codex Go.

## Overview

Codex Go follows **Test-Driven Development (TDD)** with comprehensive test coverage across all layers. Our testing strategy emphasizes:

- Write tests before implementation
- High coverage (target: >80%)
- Fast test execution
- Minimal test flakiness
- Clear test organization

## Testing Layers

### 1. Unit Tests

Test individual functions and methods in isolation.

**Characteristics:**
- Fast (<1ms per test)
- No external dependencies
- Mock all I/O operations
- Focus on logic correctness

**Example:**
```go
func TestTokenCounter_Count(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected int
    }{
        {
            name:     "empty string",
            input:    "",
            expected: 0,
        },
        {
            name:     "simple text",
            input:    "hello world",
            expected: 2,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            counter := NewTokenCounter()
            result := counter.Count(tt.input)
            require.Equal(t, tt.expected, result)
        })
    }
}
```

### 2. Integration Tests

Test interaction between multiple components.

**Characteristics:**
- Slower than unit tests
- May use test doubles for external services
- Test component integration
- Verify data flow

**Example:**
```go
func TestSession_WithMockClient(t *testing.T) {
    // Setup mocks
    mockClient := &MockClient{}
    mockTools := NewMockToolRegistry()

    // Create session with mocks
    session := NewSession(mockClient, mockTools, nil)

    // Configure mock expectations
    mockClient.On("Complete", mock.Anything, mock.Anything).
        Return(&Response{Content: "Hello"}, nil)

    // Execute
    err := session.Send(context.Background(), "Hi")
    require.NoError(t, err)

    // Verify
    mockClient.AssertExpectations(t)
}
```

### 3. Golden File Tests

Compare complex outputs against saved snapshots.

**Use Cases:**
- JSON structures
- ANSI terminal rendering
- Generated code/diffs
- Complex data transformations

**Example:**
```go
func TestMessageRenderer_RenderJSON(t *testing.T) {
    renderer := NewMessageRenderer()
    message := &protocol.Message{
        Role:    "assistant",
        Content: "Hello, world!",
    }

    output := renderer.RenderJSON(message)
    test.GoldenJSON(t, "message-render", output)
}
```

**Updating Golden Files:**
```bash
# Update all golden files
make golden-update

# Update specific test
go test ./internal/renderer -update

# Review changes before committing
git diff test/testdata/golden/
```

### 4. Table-Driven Tests

Test multiple scenarios with parameterized inputs.

**Benefits:**
- Reduce code duplication
- Easy to add new test cases
- Clear test coverage
- Consistent test structure

**Example:**
```go
func TestParser_Parse(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected *Result
        wantErr  bool
        errMsg   string
    }{
        {
            name:     "valid JSON",
            input:    `{"key": "value"}`,
            expected: &Result{Key: "value"},
            wantErr:  false,
        },
        {
            name:     "invalid JSON",
            input:    `{invalid}`,
            expected: nil,
            wantErr:  true,
            errMsg:   "invalid character",
        },
        {
            name:     "empty input",
            input:    "",
            expected: nil,
            wantErr:  true,
            errMsg:   "empty input",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := Parse(tt.input)

            if tt.wantErr {
                require.Error(t, err)
                if tt.errMsg != "" {
                    require.Contains(t, err.Error(), tt.errMsg)
                }
                return
            }

            require.NoError(t, err)
            require.Equal(t, tt.expected, result)
        })
    }
}
```

## Test Organization

### Directory Structure

```
codex-go/
├── internal/
│   └── mypackage/
│       ├── mypackage.go           # Implementation
│       ├── mypackage_test.go      # Unit tests
│       └── testdata/              # Test data
│           ├── fixtures/          # Input data
│           └── golden/            # Expected outputs
└── test/
    ├── testhelpers.go             # Shared test utilities
    ├── testdata/                  # Shared test data
    │   ├── fixtures/              # Shared fixtures
    │   └── golden/                # Shared golden files
    └── integration/               # Integration tests
        └── session_test.go
```

### File Naming

- Test files: `*_test.go`
- Golden files: `{test-name}.json` or `{test-name}.txt`
- Fixture files: `{fixture-name}.{ext}`
- Mock files: `mock_*.go` or `*_mock.go`

## Test Utilities

### Test Helpers (`test/testhelpers.go`)

Common utilities for all tests:

#### Golden File Testing
```go
// Compare JSON output
test.GoldenJSON(t, "test-name", actualData)

// Compare text output
test.GoldenText(t, "test-name", actualText)
```

#### Fixture Loading
```go
// Load raw fixture
data := test.LoadFixture(t, "example.json")

// Load and unmarshal JSON fixture
var config Config
test.LoadFixtureJSON(t, "config.json", &config)
```

#### Temporary Files
```go
// Create temp directory (auto-cleanup)
tmpDir := test.TempDir(t)

// Write test file
path := test.WriteFile(t, tmpDir, "file.txt", []byte("content"))
```

### Custom Assertions

Use `testify/require` for clear assertions:

```go
// Equality
require.Equal(t, expected, actual)
require.NotEqual(t, expected, actual)

// Errors
require.NoError(t, err)
require.Error(t, err)
require.ErrorIs(t, err, ErrNotFound)
require.ErrorContains(t, err, "expected message")

// Nil checks
require.Nil(t, value)
require.NotNil(t, value)

// Collections
require.Len(t, slice, 3)
require.Contains(t, slice, item)
require.ElementsMatch(t, expected, actual)

// Strings
require.Contains(t, str, substring)
require.Regexp(t, pattern, str)
```

## Mocking Strategies

### 1. Interface-Based Mocks

Define interfaces for dependencies:

```go
// Define interface
type APIClient interface {
    Complete(ctx context.Context, req Request) (*Response, error)
}

// Create mock
type MockAPIClient struct {
    mock.Mock
}

func (m *MockAPIClient) Complete(ctx context.Context, req Request) (*Response, error) {
    args := m.Called(ctx, req)
    return args.Get(0).(*Response), args.Error(1)
}

// Use in test
func TestWithMock(t *testing.T) {
    mockClient := new(MockAPIClient)
    mockClient.On("Complete", mock.Anything, mock.Anything).
        Return(&Response{Text: "hello"}, nil)

    // Use mockClient in code under test

    mockClient.AssertExpectations(t)
}
```

### 2. Struct Embedding for Partial Mocks

Override only specific methods:

```go
type TestClient struct {
    *RealClient
    CompleteFunc func(ctx context.Context, req Request) (*Response, error)
}

func (c *TestClient) Complete(ctx context.Context, req Request) (*Response, error) {
    if c.CompleteFunc != nil {
        return c.CompleteFunc(ctx, req)
    }
    return c.RealClient.Complete(ctx, req)
}
```

### 3. Test Doubles

Create simplified implementations for testing:

```go
// Simple in-memory implementation
type InMemoryRepository struct {
    data map[string]*Conversation
    mu   sync.RWMutex
}

func (r *InMemoryRepository) Save(ctx context.Context, conv *Conversation) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.data[conv.ID] = conv
    return nil
}
```

## Testing Patterns

### Testing Async Code

```go
func TestStreamingResponse(t *testing.T) {
    client := NewClient()

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    events, err := client.Stream(ctx, request)
    require.NoError(t, err)

    var received []Event
    for event := range events {
        received = append(received, event)
    }

    require.Len(t, received, expectedCount)
}
```

### Testing Errors

```go
func TestErrorHandling(t *testing.T) {
    tests := []struct {
        name      string
        input     string
        wantErr   error
        checkErr  func(t *testing.T, err error)
    }{
        {
            name:    "not found error",
            input:   "missing",
            wantErr: ErrNotFound,
        },
        {
            name:  "validation error",
            input: "invalid",
            checkErr: func(t *testing.T, err error) {
                var validErr *ValidationError
                require.ErrorAs(t, err, &validErr)
                require.Equal(t, "input", validErr.Field)
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := DoSomething(tt.input)

            if tt.wantErr != nil {
                require.ErrorIs(t, err, tt.wantErr)
            }

            if tt.checkErr != nil {
                tt.checkErr(t, err)
            }
        })
    }
}
```

### Testing Time-Dependent Code

```go
// Use time abstraction
type Clock interface {
    Now() time.Time
}

type RealClock struct{}
func (RealClock) Now() time.Time { return time.Now() }

type TestClock struct {
    current time.Time
}
func (c *TestClock) Now() time.Time { return c.current }

// In test
func TestTimeDependent(t *testing.T) {
    clock := &TestClock{current: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}
    service := NewService(clock)

    // Test with fixed time
    result := service.DoSomething()
    require.Equal(t, expected, result)
}
```

### Testing HTTP Clients

```go
func TestAPIClient(t *testing.T) {
    // Create test server
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        require.Equal(t, "/api/complete", r.URL.Path)
        require.Equal(t, "application/json", r.Header.Get("Content-Type"))

        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(Response{Text: "hello"})
    }))
    defer server.Close()

    // Create client pointing to test server
    client := NewClient(server.URL)

    // Test
    resp, err := client.Complete(context.Background(), Request{})
    require.NoError(t, err)
    require.Equal(t, "hello", resp.Text)
}
```

## Coverage

### Measuring Coverage

```bash
# Run tests with coverage
make test

# View coverage report in browser
go tool cover -html=coverage.out

# Check coverage percentage
go tool cover -func=coverage.out | grep total
```

### Coverage Goals

- Overall: >80%
- Core packages (protocol, client, session): >90%
- TUI components: >70% (harder to test UI)
- Test utilities: 100%

### Ignoring Code from Coverage

```go
// For unreachable code or testing utilities
func HelperFunc() {
    // This function is used only in tests
    // coverage:ignore
    panic("not implemented")
}
```

## Performance Testing

### Benchmark Tests

```go
func BenchmarkTokenCount(b *testing.B) {
    input := "some text to tokenize"
    counter := NewTokenCounter()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        counter.Count(input)
    }
}
```

Run benchmarks:
```bash
go test -bench=. -benchmem ./...
```

### Profiling

```bash
# CPU profiling
go test -cpuprofile=cpu.prof -bench=. ./internal/tokencount

# Memory profiling
go test -memprofile=mem.prof -bench=. ./internal/tokencount

# View profile
go tool pprof cpu.prof
```

## Testing Best Practices

### Do's

1. Write tests first (TDD)
2. Test behavior, not implementation
3. Use descriptive test names
4. One assertion per test (when possible)
5. Use table-driven tests for similar scenarios
6. Mock external dependencies
7. Clean up resources (use `t.Cleanup()`)
8. Test error cases
9. Keep tests fast
10. Use golden files for complex outputs

### Don'ts

1. Don't test private functions directly
2. Don't use real external services
3. Don't share state between tests
4. Don't use sleep for timing
5. Don't ignore test failures
6. Don't copy-paste test code
7. Don't test third-party libraries
8. Don't make tests dependent on execution order
9. Don't skip tests without good reason
10. Don't commit failing tests

## Continuous Integration

Tests run automatically on:
- Every pull request
- Every push to main/develop
- Multiple OS (Linux, macOS, Windows)
- Multiple Go versions (1.23, 1.24)

See [.github/workflows/ci.yml](../.github/workflows/ci.yml) for details.

## Debugging Tests

### Run Specific Tests

```bash
# Run one test
go test -v -run TestSpecificTest ./internal/mypackage

# Run tests matching pattern
go test -v -run TestParser.* ./internal/mypackage

# Run tests in one package
go test -v ./internal/mypackage
```

### Verbose Output

```bash
# Show all test output
go test -v ./...

# Show only failures
go test ./...
```

### Test Timeout

```bash
# Set custom timeout
go test -timeout 30s ./...
```

### Race Detection

```bash
# Run with race detector
go test -race ./...
```

## Common Testing Patterns

### Test Setup/Teardown

```go
func TestWithSetup(t *testing.T) {
    // Setup
    resource := setupResource(t)
    t.Cleanup(func() {
        // Teardown
        resource.Close()
    })

    // Test
    result := resource.DoSomething()
    require.NoError(t, result)
}
```

### Subtests

```go
func TestMultipleScenarios(t *testing.T) {
    t.Run("scenario1", func(t *testing.T) {
        // Test scenario 1
    })

    t.Run("scenario2", func(t *testing.T) {
        // Test scenario 2
    })
}
```

### Parallel Tests

```go
func TestParallel(t *testing.T) {
    tests := []struct {
        name string
        // ...
    }{
        // test cases
    }

    for _, tt := range tests {
        tt := tt // capture range variable
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel() // run tests in parallel
            // test body
        })
    }
}
```

## Resources

- [Testing package documentation](https://pkg.go.dev/testing)
- [Testify documentation](https://github.com/stretchr/testify)
- [Table-driven tests in Go](https://dave.cheney.net/2019/05/07/prefer-table-driven-tests)
- [Go testing best practices](https://talks.golang.org/2014/testing.slide)
