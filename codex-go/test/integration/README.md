# Integration Tests

This directory contains end-to-end integration tests that verify the interaction between multiple Codex components.

## Test Coverage

### 1. TestSimpleNonStreamingTurn
**Status**: Framework complete, skipped pending full Op handling in manager

Tests basic conversation flow:
- Create conversation manager with mock client
- Create session with turn context
- Submit user input operation
- Verify conversation state is updated correctly

**Key Components Tested**:
- `internal/conversation/manager`
- `internal/client` (mocked)
- `internal/protocol` (Op types)

### 2. TestStreamingWithToolCalls
**Status**: Framework complete, skipped pending event system

Tests streaming responses with tool calls:
- Stream text deltas from model
- Detect tool call request in stream
- Trigger approval workflow
- Execute tool
- Send tool result back to model
- Receive final response

**Key Components Tested**:
- `internal/client` (streaming)
- `internal/conversation/manager`
- `internal/tools/orchestrator`
- `internal/protocol` (Events)

### 3. TestSandboxEscalation
**Status**: Framework complete, skipped pending full sandbox implementations

Tests sandbox retry logic:
- Execute tool in native sandbox
- Detect permission denial
- Escalate to docker sandbox
- Verify successful execution

**Key Components Tested**:
- `internal/tools/orchestrator`
- `internal/sandbox/native`
- `internal/sandbox/docker` (pending)
- `internal/tools/runtime`

### 4. TestPatchToolEndToEnd
**Status**: Framework complete, skipped pending patch tool implementation

Tests patch tool workflow:
- Setup in-memory filesystem (afero)
- Request file modification
- Preview changes (dry-run)
- Apply patch
- Verify file contents

**Key Components Tested**:
- `internal/tools/patch` (pending)
- `internal/tools/orchestrator`
- File I/O with afero

### 5. TestFullSessionWithPersistence
**Status**: Framework complete, skipped pending persistence integration

Tests session lifecycle with history:
- Create session with persistence enabled
- Submit multiple turns
- Verify history is written to disk (JSONL)
- Close and reload session
- Verify state restoration

**Key Components Tested**:
- `internal/conversation/manager`
- `internal/history/persistence`
- `internal/history/compaction`

### 6. TestOrchestratorIntegration
**Status**: ✅ PASSING

Tests tool orchestration:
- Register mock tool
- Create orchestrator with auto-approval
- Execute tool
- Verify result

**Key Components Tested**:
- `internal/tools/orchestrator`
- `internal/tools/runtime`

## Running Tests

### Run all integration tests
```bash
go test ./test/integration -v
```

### Run specific test
```bash
go test ./test/integration -v -run TestOrchestratorIntegration
```

### Run with race detector
```bash
go test ./test/integration -v -race
```

### Run with coverage
```bash
go test ./test/integration -v -coverprofile=integration_coverage.out
go tool cover -html=integration_coverage.out
```

## Test Strategy

### Mocking Strategy

Integration tests use selective mocking:
- **Real components**: Manager, orchestrator, state machine, tools
- **Mocked components**: External services (OpenAI API, Docker, kubectl)
- **Test filesystems**: afero for isolated file operations

### Test Data

Test fixtures are stored in `test/integration/fixtures/`:
- Protocol messages
- Sample code files
- Configuration examples

### Golden Files

For complex outputs, golden files provide expected results:
- `fixtures/golden/` - Reference outputs for comparison
- Update with: `go test ./test/integration -update`

## Current Status

- **Total tests**: 6
- **Passing**: 1 (TestOrchestratorIntegration)
- **Skipped**: 5 (awaiting full implementation of dependencies)

As components are completed, tests will be un-skipped to provide continuous validation.

## Adding New Tests

When adding new integration tests:

1. **Name clearly**: Use descriptive test names that indicate what is being verified
2. **Document dependencies**: Note which components must be implemented
3. **Use sub-tests**: Group related assertions with `t.Run()`
4. **Mock strategically**: Only mock external dependencies
5. **Clean up**: Defer cleanup operations (filesystem, goroutines)
6. **Add to this README**: Document new test coverage

## CI/CD Integration

Integration tests run in CI on:
- Every pull request
- Merges to main branch
- Nightly builds

Test results are reported via GitHub Actions with coverage metrics.
