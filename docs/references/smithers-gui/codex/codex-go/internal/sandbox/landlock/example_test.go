//go:build linux

package landlock_test

import (
	"fmt"
	"log"
	"os"

	"github.com/evmts/codex/codex-go/internal/sandbox/landlock"
)

// Example_basic demonstrates basic Landlock usage.
func Example_basic() {
	// Check if Landlock is supported
	if !landlock.IsSupported() {
		fmt.Println("Landlock not supported on this kernel")
		return
	}

	// Create a ruleset
	ruleset := landlock.NewRuleset()

	// Allow read-only access to system directories
	ruleset.AddReadOnlyPath("/usr")
	ruleset.AddReadOnlyPath("/lib")

	// Allow read-write access to temp directory
	ruleset.AddReadWritePath("/tmp")

	// Note: We don't actually apply this in the example
	// to avoid restricting the test process
	fmt.Println("Ruleset configured successfully")
	// Output: Ruleset configured successfully
}

// Example_defaultPolicy demonstrates the default policy.
func Example_defaultPolicy() {
	// Check support
	if !landlock.IsSupported() {
		fmt.Println("Landlock not supported")
		return
	}

	// Apply default policy with writable roots
	writableRoots := []string{"/tmp", os.TempDir()}

	// In real code, you would call:
	// err := landlock.ApplyDefault(writableRoots)
	// if err != nil {
	//     log.Fatal(err)
	// }

	fmt.Printf("Would apply default policy with %d writable roots\n", len(writableRoots))
	// Output: Would apply default policy with 2 writable roots
}

// Example_gracefulDegradation shows how to handle older kernels.
func Example_gracefulDegradation() {
	// Use TryApply for optional sandboxing
	ruleset := landlock.NewRuleset()
	ruleset.AddReadOnlyPath("/")
	ruleset.AddReadWritePath("/tmp")

	// TryApply returns nil if Landlock is not supported
	// Only returns error if Landlock is supported but application failed
	err := ruleset.TryApply()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Sandboxing applied (if supported)")
	// Output: Sandboxing applied (if supported)
}

// Example_policyBuilder demonstrates the fluent API.
func Example_policyBuilder() {
	// Build a policy using the fluent API
	policy := landlock.NewPolicy().
		AddReadOnly("/usr", "/lib").
		AddReadWrite("/tmp").
		WithBestEffort(true).
		Build()

	fmt.Printf("Policy has %d read-only and %d read-write paths\n",
		len(policy.ReadOnlyPaths), len(policy.ReadWritePaths))
	// Output: Policy has 2 read-only and 1 read-write paths
}

// Example_systemInfo demonstrates getting system information.
func Example_systemInfo() {
	info, err := landlock.GetInfo()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Supported: %v, ABI Version: %d\n",
		info.Supported, info.ABIVersion)

	// Note: Output will vary based on kernel version
}

// Example_customRules shows fine-grained access control.
func Example_customRules() {
	ruleset := landlock.NewRuleset()

	// Custom access: only read files and list directories in /opt
	ruleset.AddRule("/opt",
		landlock.AccessFSReadFile|landlock.AccessFSReadDir)

	// Full read-write access to workspace
	ruleset.AddReadWritePath("/home/user/workspace")

	// Explicit deny (optional, default is deny)
	ruleset.AddDenyPath("/secret")

	fmt.Println("Custom ruleset configured")
	// Output: Custom ruleset configured
}

// Example_runSandboxed shows how to run code in a sandbox.
func Example_runSandboxed() {
	// Define writable directories
	writableRoots := []string{"/tmp"}

	// This would apply sandboxing and run the function
	// Note: We don't actually run this to avoid restricting the process
	_ = writableRoots

	fmt.Println("Would run sandboxed code")
	// Output: Would run sandboxed code
}

// Example_checkSupport demonstrates checking Landlock availability.
func Example_checkSupport() {
	// Simple check
	if landlock.IsSupported() {
		fmt.Println("Landlock is supported")
	} else {
		fmt.Println("Landlock is not supported")
	}

	// Get ABI version
	version := landlock.GetABIVersion()
	fmt.Printf("ABI Version: %d\n", version)

	// Get kernel version
	kernelVersion, _ := landlock.GetKernelVersion()
	fmt.Printf("Kernel: %s\n", kernelVersion)
}

// Example_readOnlyFilesystem shows how to create a read-only filesystem.
func Example_readOnlyFilesystem() {
	if !landlock.IsSupported() {
		fmt.Println("Landlock not supported")
		return
	}

	// Apply read-only policy (would need to be tested on Linux)
	// err := landlock.ApplyReadOnly()
	// if err != nil {
	//     log.Fatal(err)
	// }

	fmt.Println("Would apply read-only filesystem")
	// Output: Would apply read-only filesystem
}
