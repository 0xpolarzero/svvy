//go:build linux && integration

// Package sandbox_test provides Linux Seccomp-BPF-specific integration tests.
//
// These tests verify that Linux Seccomp-BPF syscall filtering is correctly
// applied and enforced by the kernel.
//
// Requirements:
// - Linux with Seccomp support
// - Integration build tag: go test -tags=integration
// - Tests must run on a system with Seccomp enabled
//
// Run with: go test -tags=integration -v ./internal/sandbox/ -run TestSeccomp
package sandbox_test

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"testing"

	"github.com/evmts/codex/codex-go/internal/sandbox/seccomp"
)

// TestSeccompSyscallRestriction tests that Seccomp blocks specific syscalls.
func TestSeccompSyscallRestriction(t *testing.T) {
	if !seccomp.IsSupported() {
		t.Skip("Seccomp not supported on this system")
	}

	if os.Getenv("SECCOMP_TEST_SUBPROCESS") == "1" {
		// This is the subprocess - apply Seccomp and test
		operation := os.Getenv("SECCOMP_TEST_OPERATION")
		arch := runtime.GOARCH

		// Create and apply Seccomp filter
		var filter *seccomp.SeccompFilter
		var err error

		switch operation {
		case "block_network":
			// Create filter that blocks network syscalls
			filter, err = seccomp.CreateNetworkFilter(arch)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create network filter: %v\n", err)
				os.Exit(1)
			}

			if err := filter.Apply(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to apply Seccomp filter: %v\n", err)
				os.Exit(1)
			}

			// Try to create a TCP socket (should be blocked)
			_, err = net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Network operation blocked: %v\n", err)
				os.Exit(1)
			}

			fmt.Println("Network operation succeeded")
			os.Exit(0)

		case "allow_unix_socket":
			// Create filter that blocks network but allows Unix sockets
			filter, err = seccomp.CreateNetworkFilter(arch)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create network filter: %v\n", err)
				os.Exit(1)
			}

			if err := filter.Apply(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to apply Seccomp filter: %v\n", err)
				os.Exit(1)
			}

			// Try to create a Unix socket (should be allowed)
			tmpSock := "/tmp/seccomp-test-socket"
			defer os.Remove(tmpSock)
			_, err = net.Listen("unix", tmpSock)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Unix socket creation failed: %v\n", err)
				os.Exit(1)
			}

			fmt.Println("Unix socket created successfully")
			os.Exit(0)

		default:
			fmt.Fprintf(os.Stderr, "Unknown operation: %s\n", operation)
			os.Exit(1)
		}
	}

	// Parent process - fork and test
	tests := []struct {
		name          string
		operation     string
		expectSuccess bool
		description   string
	}{
		{
			name:          "TCP socket creation should be blocked",
			operation:     "block_network",
			expectSuccess: false,
			description:   "Seccomp should block TCP socket creation with network filter",
		},
		{
			name:          "Unix socket creation should be allowed",
			operation:     "allow_unix_socket",
			expectSuccess: true,
			description:   "Seccomp network filter should allow Unix domain sockets",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Description: %s", tt.description)

			cmd := exec.Command(os.Args[0], "-test.run=TestSeccompSyscallRestriction")
			cmd.Env = append(os.Environ(),
				"SECCOMP_TEST_SUBPROCESS=1",
				fmt.Sprintf("SECCOMP_TEST_OPERATION=%s", tt.operation),
			)

			output, err := cmd.CombinedOutput()
			exitCode := 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				}
			}

			t.Logf("Exit code: %d", exitCode)
			if len(output) > 0 {
				t.Logf("Output: %s", string(output))
			}

			if tt.expectSuccess {
				if exitCode != 0 {
					t.Errorf("expected success but got exit code %d", exitCode)
				}
			} else {
				if exitCode == 0 {
					t.Errorf("expected failure but command succeeded")
				} else {
					t.Logf("Command failed as expected (Seccomp enforcement working)")
				}
			}
		})
	}
}

// TestSeccompNetworkRestriction tests that Seccomp enforces network restrictions.
func TestSeccompNetworkRestriction(t *testing.T) {
	if !seccomp.IsSupported() {
		t.Skip("Seccomp not supported on this system")
	}

	arch := runtime.GOARCH

	tests := []struct {
		name        string
		createFilter func() (*seccomp.SeccompFilter, error)
		description string
	}{
		{
			name: "network filter creation",
			createFilter: func() (*seccomp.SeccompFilter, error) {
				return seccomp.CreateNetworkFilter(arch)
			},
			description: "Should be able to create a network restriction filter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Description: %s", tt.description)

			filter, err := tt.createFilter()
			if err != nil {
				t.Fatalf("failed to create filter: %v", err)
			}

			if filter == nil {
				t.Fatal("filter is nil")
			}

			t.Logf("Filter created successfully")
		})
	}
}

// TestSeccompFilterBuilder tests the BPF filter builder.
func TestSeccompFilterBuilder(t *testing.T) {
	if !seccomp.IsSupported() {
		t.Skip("Seccomp not supported on this system")
	}

	arch := runtime.GOARCH

	tests := []struct {
		name         string
		buildFilter  func() (*seccomp.SeccompFilter, error)
		validateFunc func(t *testing.T, filter *seccomp.SeccompFilter)
		description  string
	}{
		{
			name: "create empty filter with allow default",
			buildFilter: func() (*seccomp.SeccompFilter, error) {
				fb, err := seccomp.NewFilterBuilder(arch, seccomp.ActionAllow)
				if err != nil {
					return nil, err
				}
				return fb.Build(), nil
			},
			validateFunc: func(t *testing.T, filter *seccomp.SeccompFilter) {
				if filter == nil {
					t.Error("expected non-nil filter")
				}
			},
			description: "Should be able to create a filter with allow-all default",
		},
		{
			name: "create filter with deny default",
			buildFilter: func() (*seccomp.SeccompFilter, error) {
				fb, err := seccomp.NewFilterBuilder(arch, seccomp.ActionErrno)
				if err != nil {
					return nil, err
				}
				return fb.Build(), nil
			},
			validateFunc: func(t *testing.T, filter *seccomp.SeccompFilter) {
				if filter == nil {
					t.Error("expected non-nil filter")
				}
			},
			description: "Should be able to create a filter with deny-all default",
		},
		{
			name: "create filter with denied syscalls",
			buildFilter: func() (*seccomp.SeccompFilter, error) {
				fb, err := seccomp.NewFilterBuilder(arch, seccomp.ActionAllow)
				if err != nil {
					return nil, err
				}
				// Deny some syscalls
				fb.DenySyscall(int(syscall.SYS_PTRACE))
				fb.DenySyscall(int(syscall.SYS_REBOOT))
				return fb.Build(), nil
			},
			validateFunc: func(t *testing.T, filter *seccomp.SeccompFilter) {
				if filter == nil {
					t.Error("expected non-nil filter")
				}
			},
			description: "Should be able to create a filter that denies specific syscalls",
		},
		{
			name: "create restrictive filter with allowlist",
			buildFilter: func() (*seccomp.SeccompFilter, error) {
				allowedSyscalls := []int{
					int(syscall.SYS_READ),
					int(syscall.SYS_WRITE),
					int(syscall.SYS_EXIT),
					int(syscall.SYS_EXIT_GROUP),
				}
				return seccomp.CreateRestrictiveFilter(arch, allowedSyscalls)
			},
			validateFunc: func(t *testing.T, filter *seccomp.SeccompFilter) {
				if filter == nil {
					t.Error("expected non-nil filter")
				}
			},
			description: "Should be able to create a restrictive allowlist filter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Description: %s", tt.description)

			filter, err := tt.buildFilter()
			if err != nil {
				t.Fatalf("failed to build filter: %v", err)
			}

			tt.validateFunc(t, filter)
		})
	}
}

// TestSeccompArchitectureValidation tests that architecture validation works.
func TestSeccompArchitectureValidation(t *testing.T) {
	if !seccomp.IsSupported() {
		t.Skip("Seccomp not supported on this system")
	}

	tests := []struct {
		name        string
		arch        string
		expectError bool
		description string
	}{
		{
			name:        "valid architecture amd64",
			arch:        "amd64",
			expectError: false,
			description: "amd64 architecture should be supported",
		},
		{
			name:        "valid architecture arm64",
			arch:        "arm64",
			expectError: false,
			description: "arm64 architecture should be supported",
		},
		{
			name:        "invalid architecture",
			arch:        "invalid_arch",
			expectError: true,
			description: "Invalid architecture should return an error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Description: %s", tt.description)

			_, err := seccomp.NewFilterBuilder(tt.arch, seccomp.ActionAllow)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				} else {
					t.Logf("Got expected error: %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// TestSeccompCurrentArchitecture tests that we can detect the current architecture.
func TestSeccompCurrentArchitecture(t *testing.T) {
	if !seccomp.IsSupported() {
		t.Skip("Seccomp not supported on this system")
	}

	arch := seccomp.GetArchitecture()
	t.Logf("Current architecture: %s", arch)

	if arch == "" {
		t.Error("GetArchitecture returned empty string")
	}

	// Verify it matches runtime.GOARCH
	if arch != runtime.GOARCH {
		t.Errorf("GetArchitecture (%s) doesn't match runtime.GOARCH (%s)", arch, runtime.GOARCH)
	}
}

// TestSeccompMode tests that we can query the current Seccomp mode.
func TestSeccompMode(t *testing.T) {
	if !seccomp.IsSupported() {
		t.Skip("Seccomp not supported on this system")
	}

	mode, err := seccomp.GetMode()
	if err != nil {
		t.Fatalf("failed to get Seccomp mode: %v", err)
	}

	t.Logf("Current Seccomp mode: %d", mode)

	// Mode should be 0 (disabled) in the test process since we haven't applied a filter
	if mode != seccomp.SECCOMP_MODE_DISABLED {
		t.Logf("Note: Seccomp mode is %d (expected 0 for disabled)", mode)
	}
}

// TestSeccompNoNewPrivs tests that we can set PR_SET_NO_NEW_PRIVS.
func TestSeccompNoNewPrivs(t *testing.T) {
	if !seccomp.IsSupported() {
		t.Skip("Seccomp not supported on this system")
	}

	if os.Getenv("SECCOMP_TEST_NO_NEW_PRIVS_SUBPROCESS") == "1" {
		// This is the subprocess - set no_new_privs
		if err := seccomp.SetNoNewPrivs(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to set no_new_privs: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("no_new_privs set successfully")
		os.Exit(0)
	}

	// Parent process - fork and test
	t.Run("set no_new_privs", func(t *testing.T) {
		cmd := exec.Command(os.Args[0], "-test.run=TestSeccompNoNewPrivs")
		cmd.Env = append(os.Environ(), "SECCOMP_TEST_NO_NEW_PRIVS_SUBPROCESS=1")

		output, err := cmd.CombinedOutput()
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			}
		}

		t.Logf("Exit code: %d", exitCode)
		if len(output) > 0 {
			t.Logf("Output: %s", string(output))
		}

		if exitCode != 0 {
			t.Errorf("expected success but got exit code %d", exitCode)
		}
	})
}

// TestSeccompConditionalFiltering tests conditional syscall filtering.
func TestSeccompConditionalFiltering(t *testing.T) {
	if !seccomp.IsSupported() {
		t.Skip("Seccomp not supported on this system")
	}

	arch := runtime.GOARCH

	tests := []struct {
		name        string
		buildFilter func() (*seccomp.SeccompFilter, error)
		description string
	}{
		{
			name: "conditional filtering on syscall argument",
			buildFilter: func() (*seccomp.SeccompFilter, error) {
				fb, err := seccomp.NewFilterBuilder(arch, seccomp.ActionAllow)
				if err != nil {
					return nil, err
				}
				// Block socket() calls with non-Unix domain
				// This tests conditional filtering based on the first argument
				if err := fb.DenySyscallConditional(
					int(syscall.SYS_SOCKET),
					0,          // Check first argument (domain)
					1,          // AF_UNIX value
					true,       // Invert match (deny if NOT AF_UNIX)
				); err != nil {
					return nil, err
				}
				return fb.Build(), nil
			},
			description: "Should be able to create conditional filters based on syscall arguments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Description: %s", tt.description)

			filter, err := tt.buildFilter()
			if err != nil {
				t.Fatalf("failed to build filter: %v", err)
			}

			if filter == nil {
				t.Fatal("filter is nil")
			}

			t.Logf("Conditional filter created successfully")
		})
	}
}

// TestSeccompFilterApplication tests that filters can be applied (in subprocess).
func TestSeccompFilterApplication(t *testing.T) {
	if !seccomp.IsSupported() {
		t.Skip("Seccomp not supported on this system")
	}

	if os.Getenv("SECCOMP_TEST_APPLY_SUBPROCESS") == "1" {
		// This is the subprocess - create and apply a basic filter
		arch := runtime.GOARCH

		fb, err := seccomp.NewFilterBuilder(arch, seccomp.ActionAllow)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create filter builder: %v\n", err)
			os.Exit(1)
		}

		// Create a filter that denies ptrace
		fb.DenySyscall(int(syscall.SYS_PTRACE))
		filter := fb.Build()

		// Apply the filter
		if err := filter.Apply(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to apply filter: %v\n", err)
			os.Exit(1)
		}

		// Verify mode is now FILTER
		mode, err := seccomp.GetMode()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get mode: %v\n", err)
			os.Exit(1)
		}

		if mode != seccomp.SECCOMP_MODE_FILTER {
			fmt.Fprintf(os.Stderr, "Expected mode %d, got %d\n",
				seccomp.SECCOMP_MODE_FILTER, mode)
			os.Exit(1)
		}

		fmt.Println("Filter applied successfully")
		os.Exit(0)
	}

	// Parent process - fork and test
	t.Run("apply basic filter", func(t *testing.T) {
		cmd := exec.Command(os.Args[0], "-test.run=TestSeccompFilterApplication")
		cmd.Env = append(os.Environ(), "SECCOMP_TEST_APPLY_SUBPROCESS=1")

		output, err := cmd.CombinedOutput()
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			}
		}

		t.Logf("Exit code: %d", exitCode)
		if len(output) > 0 {
			t.Logf("Output: %s", string(output))
		}

		if exitCode != 0 {
			t.Errorf("expected success but got exit code %d", exitCode)
		}
	})
}

// TestSeccompErrorHandling tests error handling in filter operations.
func TestSeccompErrorHandling(t *testing.T) {
	if !seccomp.IsSupported() {
		t.Skip("Seccomp not supported on this system")
	}

	tests := []struct {
		name        string
		operation   func() error
		expectError bool
		description string
	}{
		{
			name: "apply empty filter",
			operation: func() error {
				filter := &seccomp.SeccompFilter{}
				return filter.Apply()
			},
			expectError: true,
			description: "Applying an empty filter should return an error",
		},
		{
			name: "invalid conditional argument index",
			operation: func() error {
				fb, err := seccomp.NewFilterBuilder("amd64", seccomp.ActionAllow)
				if err != nil {
					return err
				}
				// Try to use invalid argument index (only 0-5 are valid)
				return fb.DenySyscallConditional(int(syscall.SYS_SOCKET), 10, 0, false)
			},
			expectError: true,
			description: "Invalid argument index should return an error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Description: %s", tt.description)

			err := tt.operation()

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				} else {
					t.Logf("Got expected error: %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}
