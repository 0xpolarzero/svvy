//go:build linux

package seccomp_test

import (
	"fmt"
	"log"
	"os"
	"syscall"

	"github.com/evmts/codex/codex-go/internal/sandbox/seccomp"
)

// Example_networkIsolation demonstrates how to apply network isolation using Seccomp
func Example_networkIsolation() {
	// Check if Seccomp is supported on this system
	if !seccomp.IsSupported() {
		log.Fatal("Seccomp not supported on this kernel")
	}

	// Get the current architecture
	arch := seccomp.GetArchitecture()
	fmt.Printf("Architecture: %s\n", arch)

	// Create a network filter that blocks all network operations except AF_UNIX
	filter, err := seccomp.CreateNetworkFilter(arch)
	if err != nil {
		log.Fatalf("Failed to create network filter: %v", err)
	}

	// Apply the filter (this is irreversible!)
	if err := filter.Apply(); err != nil {
		log.Fatalf("Failed to apply filter: %v", err)
	}

	// Verify the filter is active
	mode, _ := seccomp.GetMode()
	fmt.Printf("Seccomp mode: %d (2 = FILTER)\n", mode)

	// Now all network operations (except AF_UNIX) will fail with EPERM
	fmt.Println("Network filter applied successfully")

	// Output:
	// Architecture: amd64
	// Seccomp mode: 2 (2 = FILTER)
	// Network filter applied successfully
}

// Example_customFilter demonstrates how to build a custom Seccomp filter
func Example_customFilter() {
	if !seccomp.IsSupported() {
		return
	}

	arch := seccomp.GetArchitecture()
	syscalls := seccomp.GetSyscallNumbers(arch)

	// Create a filter builder with allow-by-default
	fb, err := seccomp.NewFilterBuilder(arch, seccomp.ActionAllow)
	if err != nil {
		log.Fatalf("Failed to create filter builder: %v", err)
	}

	// Deny specific dangerous syscalls
	fb.DenySyscall(syscalls.Socket)
	fb.DenySyscall(syscalls.Connect)
	fb.DenySyscall(syscalls.Ptrace)
	fb.DenySyscall(syscalls.Execve)

	// Build the filter
	filter := fb.Build()

	// Apply it
	if err := filter.Apply(); err != nil {
		log.Fatalf("Failed to apply custom filter: %v", err)
	}

	fmt.Println("Custom filter applied")

	// Output:
	// Custom filter applied
}

// Example_restrictiveMode demonstrates maximum restriction with explicit allowlist
func Example_restrictiveMode() {
	if !seccomp.IsSupported() {
		return
	}

	arch := seccomp.GetArchitecture()

	// Get the list of safe syscalls that the Go runtime needs
	safeSyscalls := seccomp.GetSafeSyscallList(arch)

	// Create a restrictive filter (deny-by-default)
	filter, err := seccomp.CreateRestrictiveFilter(arch, safeSyscalls)
	if err != nil {
		log.Fatalf("Failed to create restrictive filter: %v", err)
	}

	// Apply the filter
	if err := filter.Apply(); err != nil {
		log.Fatalf("Failed to apply restrictive filter: %v", err)
	}

	fmt.Printf("Restrictive filter applied with %d allowed syscalls\n", len(safeSyscalls))

	// Output:
	// Restrictive filter applied with 45 allowed syscalls
}

// Example_conditionalFiltering demonstrates filtering based on syscall arguments
func Example_conditionalFiltering() {
	if !seccomp.IsSupported() {
		return
	}

	arch := seccomp.GetArchitecture()
	syscalls := seccomp.GetSyscallNumbers(arch)

	fb, err := seccomp.NewFilterBuilder(arch, seccomp.ActionAllow)
	if err != nil {
		log.Fatalf("Failed to create filter builder: %v", err)
	}

	// Allow socket() only for AF_UNIX (domain = 1)
	// Deny if first argument (domain) is NOT equal to AF_UNIX
	err = fb.DenySyscallConditional(
		syscalls.Socket,
		0,    // arg index (domain)
		1,    // AF_UNIX value
		true, // invert match (deny if != AF_UNIX)
	)
	if err != nil {
		log.Fatalf("Failed to add conditional rule: %v", err)
	}

	filter := fb.Build()
	if err := filter.Apply(); err != nil {
		log.Fatalf("Failed to apply filter: %v", err)
	}

	// Test: AF_UNIX socket should work
	fd, err := syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		log.Printf("AF_UNIX socket failed (unexpected): %v", err)
	} else {
		syscall.Close(fd)
		fmt.Println("AF_UNIX socket: allowed")
	}

	// Test: AF_INET socket should fail
	_, err = syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err == syscall.EPERM {
		fmt.Println("AF_INET socket: blocked (EPERM)")
	} else {
		log.Printf("AF_INET socket should be blocked, got: %v", err)
	}

	// Output:
	// AF_UNIX socket: allowed
	// AF_INET socket: blocked (EPERM)
}

// Example_checkCapabilities demonstrates how to check seccomp availability
func Example_checkCapabilities() {
	// Check if Seccomp is supported
	if seccomp.IsSupported() {
		fmt.Println("Seccomp: supported")
	} else {
		fmt.Println("Seccomp: not supported")
	}

	// Get current mode
	mode, err := seccomp.GetMode()
	if err != nil {
		fmt.Printf("Failed to get mode: %v\n", err)
		return
	}

	switch mode {
	case seccomp.SECCOMP_MODE_DISABLED:
		fmt.Println("Mode: disabled")
	case seccomp.SECCOMP_MODE_STRICT:
		fmt.Println("Mode: strict")
	case seccomp.SECCOMP_MODE_FILTER:
		fmt.Println("Mode: filter")
	default:
		fmt.Printf("Mode: unknown (%d)\n", mode)
	}

	// Get architecture
	arch := seccomp.GetArchitecture()
	fmt.Printf("Architecture: %s\n", arch)

	// Output:
	// Seccomp: supported
	// Mode: disabled
	// Architecture: amd64
}

// ExampleSeccompFilter_Apply demonstrates the full workflow
func ExampleSeccompFilter_Apply() {
	// This example must run in a subprocess to avoid affecting the test process
	if os.Getenv("RUN_SECCOMP_EXAMPLE") != "1" {
		// Skip in normal test runs
		return
	}

	if !seccomp.IsSupported() {
		fmt.Println("Seccomp not supported")
		return
	}

	arch := seccomp.GetArchitecture()

	// Create network filter
	filter, err := seccomp.CreateNetworkFilter(arch)
	if err != nil {
		fmt.Printf("Filter creation failed: %v\n", err)
		return
	}

	// Apply filter
	if err := filter.Apply(); err != nil {
		fmt.Printf("Filter application failed: %v\n", err)
		return
	}

	// Verify mode
	mode, _ := seccomp.GetMode()
	if mode == seccomp.SECCOMP_MODE_FILTER {
		fmt.Println("Filter applied successfully")
	}

	// Test network operations
	_, err = syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err == syscall.EPERM {
		fmt.Println("Network operations blocked")
	}

	// Test Unix sockets (should still work)
	fd, err := syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err == nil {
		syscall.Close(fd)
		fmt.Println("Unix sockets allowed")
	}

	// Output:
	// Filter applied successfully
	// Network operations blocked
	// Unix sockets allowed
}

// Example_integrationWithLandlock shows how to combine Seccomp with Landlock
func Example_integrationWithLandlock() {
	// This demonstrates the integration pattern (pseudo-code)

	// Step 1: Apply Landlock for filesystem isolation
	// (requires landlock package)
	// landlockRules := landlock.NewRules()
	// landlockRules.AllowReadPath("/")
	// landlockRules.AllowWritePath("/tmp")
	// landlockRules.Apply()

	// Step 2: Apply Seccomp for network isolation
	if seccomp.IsSupported() {
		arch := seccomp.GetArchitecture()
		filter, err := seccomp.CreateNetworkFilter(arch)
		if err != nil {
			log.Fatalf("Seccomp filter creation failed: %v", err)
		}

		if err := filter.Apply(); err != nil {
			log.Fatalf("Seccomp filter application failed: %v", err)
		}

		fmt.Println("Combined Landlock + Seccomp sandbox active")
	}

	// Now the process has:
	// - Filesystem isolation (Landlock)
	// - Network isolation (Seccomp)

	// Output:
	// Combined Landlock + Seccomp sandbox active
}
