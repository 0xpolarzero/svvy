# Sandbox Integration Tests

This directory contains comprehensive integration tests for sandbox enforcement in codex-go. These tests verify that sandbox restrictions are **actually enforced** by the operating system, not just that the API calls succeed.

## Overview

The integration tests are organized into several files:

- **integration_test.go**: Core integration tests (platform-independent)
- **integration_seatbelt_test.go**: macOS Seatbelt-specific tests
- **integration_landlock_test.go**: Linux Landlock-specific tests
- **integration_seccomp_test.go**: Linux Seccomp-BPF-specific tests

## Requirements

### General Requirements

- Go 1.18 or later
- Integration build tag enabled: `-tags=integration`
- Network access (for network restriction tests)
- Sufficient disk space for temporary test files

### Platform-Specific Requirements

#### macOS (Seatbelt)

- macOS 10.10 or later
- System Integrity Protection (SIP) may need to be configured for some tests
- `/usr/bin/sandbox-exec` must be available

#### Linux (Landlock)

- Linux kernel 5.13 or later
- Landlock LSM enabled in kernel
- Check support: `cat /sys/kernel/security/lsm | grep landlock`

#### Linux (Seccomp)

- Linux kernel 3.5 or later (for Seccomp-BPF)
- Seccomp support enabled in kernel
- Check support: `grep SECCOMP /boot/config-$(uname -r)`

## Running the Tests

### Run All Integration Tests

```bash
# All integration tests
go test -tags=integration -v ./internal/sandbox/

# With race detection
go test -tags=integration -race -v ./internal/sandbox/

# With coverage
go test -tags=integration -cover -v ./internal/sandbox/
```

### Run Platform-Specific Tests

```bash
# macOS Seatbelt tests only
go test -tags=integration -v ./internal/sandbox/ -run TestSeatbelt

# Linux Landlock tests only
go test -tags=integration -v ./internal/sandbox/ -run TestLandlock

# Linux Seccomp tests only
go test -tags=integration -v ./internal/sandbox/ -run TestSeccomp
```

### Run Specific Test Categories

```bash
# Filesystem restriction tests
go test -tags=integration -v ./internal/sandbox/ -run Filesystem

# Network restriction tests
go test -tags=integration -v ./internal/sandbox/ -run Network

# Violation detection tests
go test -tags=integration -v ./internal/sandbox/ -run Violation
```

### Skip Integration Tests

Integration tests are automatically skipped when:
- The `-short` flag is provided: `go test -short`
- The `integration` build tag is not set
- Platform-specific requirements are not met (e.g., Landlock not available)

```bash
# Skip integration tests (run unit tests only)
go test -short -v ./internal/sandbox/
```

## Test Coverage

### Core Integration Tests (integration_test.go)

These tests verify basic sandbox enforcement across all platforms:

#### Filesystem Restrictions

1. **TestFilesystemReadOnlyEnforcement**
   - Verifies read-only paths cannot be written
   - Verifies read-only files cannot be modified
   - Verifies read-only files cannot be deleted
   - Verifies read operations succeed on read-only paths

2. **TestFilesystemReadWriteEnforcement**
   - Verifies read-write paths can be written
   - Verifies files in read-write directories can be modified
   - Verifies files can be created and deleted in read-write directories

3. **TestFilesystemPathTraversalPrevention**
   - Verifies directory traversal with `..` is blocked
   - Verifies symlink escape attempts are blocked

#### Network Restrictions

4. **TestNetworkRestrictionEnforcement**
   - Verifies HTTP requests are blocked when network is disabled
   - Verifies HTTP requests succeed when network is enabled

5. **TestNetworkPortRestrictions**
   - Verifies localhost connections work when network enabled
   - Verifies localhost connections fail when network disabled

6. **TestRawSocketCreation**
   - Verifies TCP socket creation is blocked when network disabled
   - Verifies TCP socket creation succeeds when network enabled

#### Violation Detection

7. **TestViolationDetectionAccuracy**
   - Tests detection of permission denied errors
   - Tests detection of operation not permitted errors
   - Tests detection of read-only filesystem errors
   - Tests detection of seccomp violations
   - Verifies normal failures are not misclassified

#### Performance

8. **BenchmarkSandboxedCommandExecution**
   - Benchmarks overhead of sandboxed execution

### Seatbelt Tests (integration_seatbelt_test.go)

macOS-specific tests for Seatbelt sandboxing:

1. **TestSeatbeltFilesystemRestriction**
   - Read-only profile blocks writes
   - Read-only profile allows reads
   - Workspace-write allows writes in workspace
   - Workspace-write blocks writes outside workspace

2. **TestSeatbeltNetworkRestriction**
   - Read-only profile blocks network
   - Workspace-write without network blocks network
   - Workspace-write with network allows network
   - Danger-full-access allows network

3. **TestSeatbeltProfileGeneration**
   - Read-only profile structure validation
   - Workspace-write profile structure validation
   - Danger-full-access profile structure validation

4. **TestSeatbeltWritableRootsDetection**
   - Verifies .git directories are properly handled

5. **TestSeatbeltPathNormalization**
   - Tests path normalization in profiles

### Landlock Tests (integration_landlock_test.go)

Linux-specific tests for Landlock LSM:

1. **TestLandlockFilesystemRestriction**
   - Write to read-only directory blocked
   - Read from read-only directory allowed
   - Write to read-write directory allowed
   - Access outside allowed paths blocked
   - Delete in read-only directory blocked

2. **TestLandlockEnforcementInSubprocess**
   - Tests actual enforcement by forking child processes
   - Write to allowed directory succeeds
   - Write to blocked directory fails
   - Read from allowed directory succeeds

3. **TestLandlockABIVersion**
   - Detects Landlock ABI version

4. **TestLandlockRulesetCreation**
   - Empty ruleset creation
   - Ruleset with read-only paths
   - Ruleset with read-write paths
   - Ruleset with multiple paths

5. **TestLandlockPathValidation**
   - Absolute path handling
   - Nonexistent path handling
   - Relative path conversion

6. **TestLandlockPolicyBuilder**
   - Policy builder API tests
   - Best-effort mode tests

7. **TestLandlockErrorMessages**
   - Empty policy error handling
   - Nonexistent path error handling

### Seccomp Tests (integration_seccomp_test.go)

Linux-specific tests for Seccomp-BPF:

1. **TestSeccompSyscallRestriction**
   - TCP socket creation blocked with network filter
   - Unix socket creation allowed with network filter

2. **TestSeccompNetworkRestriction**
   - Network filter creation

3. **TestSeccompFilterBuilder**
   - Empty filter with allow default
   - Filter with deny default
   - Filter with denied syscalls
   - Restrictive filter with allowlist

4. **TestSeccompArchitectureValidation**
   - amd64 architecture support
   - arm64 architecture support
   - Invalid architecture error handling

5. **TestSeccompCurrentArchitecture**
   - Current architecture detection

6. **TestSeccompMode**
   - Query current Seccomp mode

7. **TestSeccompNoNewPrivs**
   - Setting PR_SET_NO_NEW_PRIVS

8. **TestSeccompConditionalFiltering**
   - Conditional filtering based on syscall arguments

9. **TestSeccompFilterApplication**
   - Filter application in subprocess

10. **TestSeccompErrorHandling**
    - Empty filter error handling
    - Invalid argument index error handling

## Test Execution Flow

### Subprocess Testing Pattern

Many tests use a subprocess pattern to actually apply sandbox restrictions:

```go
if os.Getenv("TEST_SUBPROCESS") == "1" {
    // This code runs in the subprocess
    // Apply sandbox restrictions
    // Perform test operation
    // Exit with status code
}

// This code runs in the parent process
// Fork subprocess with environment variable set
// Check exit code and output
```

This pattern is necessary because:
1. Sandbox restrictions are irreversible once applied
2. Restrictions affect the entire process
3. We need to isolate test cases from each other

### Test Environment Setup

Each test creates a controlled environment:

```go
type TestEnvironment struct {
    TempDir      string  // Temporary directory for test files
    ReadOnlyDir  string  // Directory that should be read-only
    ReadWriteDir string  // Directory that should be read-write
    TestFile     string  // File in read-write directory
    ReadOnlyFile string  // File in read-only directory
}
```

## Expected Behavior

### Success Criteria

Tests verify actual enforcement by checking:
- **Exit codes**: Failed operations should return non-zero
- **Error messages**: Should contain sandbox-specific errors
- **Violation detection**: Should identify sandbox violations
- **File system state**: Files should/shouldn't exist based on restrictions

### Acceptable Failures

Some tests may fail in specific environments:
- **Network tests**: May fail if network is actually unavailable
- **Permission tests**: May behave differently as root
- **Platform tests**: Only run on appropriate platforms

Tests use warning logs instead of hard failures where appropriate:

```go
if result.ExitCode != 0 {
    t.Logf("Warning: Expected success but got exit code %d", result.ExitCode)
}
```

## Debugging Test Failures

### Enable Verbose Output

```bash
go test -tags=integration -v ./internal/sandbox/ -run TestName
```

### Check System Requirements

```bash
# Linux: Check Landlock support
cat /sys/kernel/security/lsm | grep landlock
uname -r  # Kernel version should be >= 5.13

# Linux: Check Seccomp support
grep SECCOMP /boot/config-$(uname -r)

# macOS: Check sandbox-exec
which sandbox-exec
sandbox-exec -h
```

### Run Single Test

```bash
# Run a specific test
go test -tags=integration -v ./internal/sandbox/ -run TestFilesystemReadOnlyEnforcement

# Run with race detector
go test -tags=integration -race -v ./internal/sandbox/ -run TestName
```

### Common Issues

1. **"Landlock not supported"**
   - Kernel too old (< 5.13)
   - Landlock not enabled in kernel config
   - Solution: Upgrade kernel or enable Landlock

2. **"Seccomp not supported"**
   - Kernel too old (< 3.5)
   - Seccomp not enabled in kernel config
   - Solution: Upgrade kernel or enable Seccomp

3. **Network tests timing out**
   - Network actually unavailable
   - Firewall blocking connections
   - Solution: Check network connectivity

4. **Permission denied errors**
   - Running tests as wrong user
   - SELinux/AppArmor interference
   - Solution: Check permissions and security policies

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Integration Tests

on: [push, pull_request]

jobs:
  test:
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest]
    runs-on: ${{ matrix.os }}

    steps:
      - uses: actions/checkout@v3

      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Run integration tests
        run: go test -tags=integration -v ./internal/sandbox/

      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          files: ./coverage.out
```

### Docker Testing

```bash
# Test in Docker container
docker run --rm -v $(pwd):/workspace -w /workspace golang:1.21 \
  go test -tags=integration -v ./internal/sandbox/

# Test with specific kernel (for Landlock)
docker run --rm -v $(pwd):/workspace -w /workspace \
  --security-opt seccomp=unconfined \
  golang:1.21 \
  go test -tags=integration -v ./internal/sandbox/ -run TestLandlock
```

## Contributing

When adding new integration tests:

1. **Use the subprocess pattern** for tests that apply sandbox restrictions
2. **Add descriptive comments** explaining what each test verifies
3. **Use appropriate build tags** (e.g., `//go:build darwin && integration`)
4. **Include skip conditions** for unavailable features
5. **Log detailed information** for debugging
6. **Test both success and failure** cases
7. **Document requirements** in test comments
8. **Update this README** with new test descriptions

## References

- [Linux Landlock Documentation](https://docs.kernel.org/userspace-api/landlock.html)
- [Linux Seccomp Documentation](https://www.kernel.org/doc/html/latest/userspace-api/seccomp_filter.html)
- [macOS Sandbox Guide](https://developer.apple.com/library/archive/documentation/Security/Conceptual/AppSandboxDesignGuide/)
- [Go Testing Package](https://pkg.go.dev/testing)

## License

These tests are part of the codex-go project and follow the same license.
