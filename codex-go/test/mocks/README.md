# Generated Mocks Directory

This directory contains auto-generated mock implementations created by [go.uber.org/mock](https://github.com/uber-go/mock).

## DO NOT EDIT

All files in this directory are generated automatically. Any manual changes will be overwritten when mocks are regenerated.

## Generating Mocks

Mocks are generated from `go:generate` directives in source files. To regenerate all mocks:

```bash
make generate-mocks
# or
go generate ./...
```

## Clean and Regenerate

To clean old mocks and regenerate:

```bash
make regen-mocks
```

## Adding New Mocks

To generate a mock for a new interface:

1. Add a `go:generate` directive above the interface in your source file:

```go
//go:generate mockgen -destination=../test/mocks/mock_myinterface.go -package=mocks . MyInterface

type MyInterface interface {
    DoSomething(ctx context.Context, arg string) error
}
```

2. Run `make generate-mocks`

## Mock Naming Convention

Generated mock files follow the pattern: `mock_<interface_name>.go`

Examples:
- `mock_repository.go`
- `mock_service.go`
- `mock_client.go`
