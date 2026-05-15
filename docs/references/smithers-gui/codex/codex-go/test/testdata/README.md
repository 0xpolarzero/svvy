# Test Data Directory

This directory contains test data fixtures and golden files for the Codex Go test suite.

## Directory Structure

```
testdata/
├── fixtures/       # Reusable test data (input files)
├── golden/         # Golden file outputs (expected results)
├── protocol/       # Protocol test fixtures (Claude Desktop Protocol)
└── README.md       # This file
```

## Usage

### Fixtures

The `fixtures/` directory contains reusable test data that can be loaded in tests using the `LoadFixture` helper:

```go
data := test.LoadFixture(t, "example.json")
```

Or for JSON fixtures:

```go
var obj MyStruct
test.LoadFixtureJSON(t, "example.json", &obj)
```

### Golden Files

The `golden/` directory contains expected output files. Use the `GoldenJSON` or `GoldenText` helpers:

```go
// Compare JSON output
test.GoldenJSON(t, "example", myResult)

// Compare text output
test.GoldenText(t, "example", myOutput)

// Update golden files with -update flag:
// go test -update
```

### Protocol Fixtures

The `protocol/` directory contains test fixtures for the Claude Desktop Protocol, including:
- Op (Operation) definitions
- Event payloads
- Conversation histories
- Raw protocol messages

## Adding New Test Data

1. Place input test data in `fixtures/`
2. Use descriptive filenames: `<feature>_<scenario>.<ext>`
3. Keep fixtures minimal but representative
4. Document complex fixtures with comments in adjacent .md files

## Best Practices

- Keep fixtures small and focused
- Use realistic but anonymized data
- Version control all fixture files
- Update golden files when intentionally changing behavior
- Use fixtures to test edge cases and error conditions
