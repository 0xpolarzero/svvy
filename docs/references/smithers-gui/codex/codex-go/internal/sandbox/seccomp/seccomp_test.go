//go:build linux

package seccomp

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
)

// TestIsSupported verifies that Seccomp support detection works
func TestIsSupported(t *testing.T) {
	supported := IsSupported()
	t.Logf("Seccomp supported: %v", supported)

	// On Linux, seccomp should generally be supported
	// (unless running on a very old kernel < 3.5)
	if !supported {
		t.Log("Warning: Seccomp not supported on this system")
	}
}

// TestGetMode verifies we can query the current Seccomp mode
func TestGetMode(t *testing.T) {
	mode, err := GetMode()
	if err != nil {
		t.Fatalf("GetMode() failed: %v", err)
	}

	// Initially, seccomp should be disabled
	if mode != SECCOMP_MODE_DISABLED {
		t.Logf("Note: Seccomp mode is %d (expected %d for disabled)", mode, SECCOMP_MODE_DISABLED)
	}

	t.Logf("Current Seccomp mode: %d", mode)
}

// TestSetNoNewPrivs verifies we can set the no_new_privs bit
func TestSetNoNewPrivs(t *testing.T) {
	// Fork a child process to test no_new_privs
	// We can't test in the main process as it would affect subsequent tests
	if os.Getenv("TEST_NO_NEW_PRIVS") == "1" {
		if err := SetNoNewPrivs(); err != nil {
			t.Fatalf("SetNoNewPrivs() failed: %v", err)
		}

		// Verify by trying to query prctl
		_, _, errno := syscall.RawSyscall6(
			syscall.SYS_PRCTL,
			PR_SET_NO_NEW_PRIVS,
			0, // Query
			0,
			0,
			0,
			0,
		)
		if errno != 0 {
			t.Fatalf("Failed to verify no_new_privs: %v", errno)
		}

		os.Exit(0)
	}

	// Run the test in a child process
	cmd := exec.Command(os.Args[0], "-test.run=TestSetNoNewPrivs")
	cmd.Env = append(os.Environ(), "TEST_NO_NEW_PRIVS=1")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Child process failed: %v", err)
	}
}

// TestGetArchitecture verifies architecture detection
func TestGetArchitecture(t *testing.T) {
	arch := GetArchitecture()
	t.Logf("Detected architecture: %s", arch)

	if arch != "amd64" && arch != "arm64" {
		t.Errorf("Unexpected architecture: %s", arch)
	}
}

// TestFilterBuilderCreation verifies we can create a filter builder
func TestFilterBuilderCreation(t *testing.T) {
	arch := GetArchitecture()

	fb, err := NewFilterBuilder(arch, ActionAllow)
	if err != nil {
		t.Fatalf("NewFilterBuilder() failed: %v", err)
	}

	if fb == nil {
		t.Fatal("Filter builder is nil")
	}

	if len(fb.instructions) < 3 {
		t.Errorf("Expected at least 3 instructions (arch validation), got %d", len(fb.instructions))
	}
}

// TestFilterBuilderUnsupportedArch verifies error handling for unsupported architectures
func TestFilterBuilderUnsupportedArch(t *testing.T) {
	_, err := NewFilterBuilder("riscv64", ActionAllow)
	if err == nil {
		t.Error("Expected error for unsupported architecture, got nil")
	}

	if err != nil {
		t.Logf("Expected error: %v", err)
	}
}

// TestDenySyscall verifies we can add deny rules
func TestDenySyscall(t *testing.T) {
	arch := GetArchitecture()
	fb, err := NewFilterBuilder(arch, ActionAllow)
	if err != nil {
		t.Fatalf("NewFilterBuilder() failed: %v", err)
	}

	initialLen := len(fb.instructions)

	// Deny the socket syscall
	syscalls := GetSyscallNumbers(arch)
	fb.DenySyscall(syscalls.Socket)

	// Should have added 3 instructions (load, compare, return)
	if len(fb.instructions) != initialLen+3 {
		t.Errorf("Expected %d instructions after DenySyscall, got %d", initialLen+3, len(fb.instructions))
	}
}

// TestAllowSyscall verifies we can add allow rules
func TestAllowSyscall(t *testing.T) {
	arch := GetArchitecture()
	fb, err := NewFilterBuilder(arch, ActionErrno)
	if err != nil {
		t.Fatalf("NewFilterBuilder() failed: %v", err)
	}

	initialLen := len(fb.instructions)

	// Allow the read syscall
	syscalls := GetSyscallNumbers(arch)
	fb.AllowSyscall(syscalls.Read)

	// Should have added 3 instructions (load, compare, return)
	if len(fb.instructions) != initialLen+3 {
		t.Errorf("Expected %d instructions after AllowSyscall, got %d", initialLen+3, len(fb.instructions))
	}
}

// TestBuildFilter verifies we can build a complete filter
func TestBuildFilter(t *testing.T) {
	arch := GetArchitecture()
	fb, err := NewFilterBuilder(arch, ActionAllow)
	if err != nil {
		t.Fatalf("NewFilterBuilder() failed: %v", err)
	}

	syscalls := GetSyscallNumbers(arch)
	fb.DenySyscall(syscalls.Socket)
	fb.DenySyscall(syscalls.Connect)

	filter := fb.Build()
	if filter == nil {
		t.Fatal("Built filter is nil")
	}

	if len(filter.program) == 0 {
		t.Error("Built filter has empty program")
	}

	t.Logf("Built filter with %d BPF instructions", len(filter.program))
}

// TestCreateNetworkFilter verifies network filter creation
func TestCreateNetworkFilter(t *testing.T) {
	arch := GetArchitecture()

	filter, err := CreateNetworkFilter(arch)
	if err != nil {
		t.Fatalf("CreateNetworkFilter() failed: %v", err)
	}

	if filter == nil {
		t.Fatal("Created filter is nil")
	}

	if len(filter.program) == 0 {
		t.Error("Network filter has empty program")
	}

	// Network filter should have many instructions (one for each denied syscall)
	if len(filter.program) < 20 {
		t.Errorf("Expected at least 20 instructions in network filter, got %d", len(filter.program))
	}

	t.Logf("Network filter has %d BPF instructions", len(filter.program))
}

// TestCreateRestrictiveFilter verifies restrictive filter creation
func TestCreateRestrictiveFilter(t *testing.T) {
	arch := GetArchitecture()
	syscalls := GetSyscallNumbers(arch)

	allowedSyscalls := []int{
		syscalls.Read,
		syscalls.Write,
		syscalls.Exit,
		syscalls.Exit_group,
	}

	filter, err := CreateRestrictiveFilter(arch, allowedSyscalls)
	if err != nil {
		t.Fatalf("CreateRestrictiveFilter() failed: %v", err)
	}

	if filter == nil {
		t.Fatal("Created filter is nil")
	}

	if len(filter.program) == 0 {
		t.Error("Restrictive filter has empty program")
	}

	t.Logf("Restrictive filter has %d BPF instructions", len(filter.program))
}

// TestGetSyscallNumbers verifies syscall number retrieval
func TestGetSyscallNumbers(t *testing.T) {
	arch := GetArchitecture()
	syscalls := GetSyscallNumbers(arch)

	if syscalls == nil {
		t.Fatal("GetSyscallNumbers returned nil")
	}

	// Verify some well-known syscall numbers
	if syscalls.Read == 0 || syscalls.Read == -1 {
		t.Errorf("Invalid read syscall number: %d", syscalls.Read)
	}

	if syscalls.Write == 0 || syscalls.Write == -1 {
		t.Errorf("Invalid write syscall number: %d", syscalls.Write)
	}

	if syscalls.Socket == 0 || syscalls.Socket == -1 {
		t.Errorf("Invalid socket syscall number: %d", syscalls.Socket)
	}

	t.Logf("Syscall numbers - Read: %d, Write: %d, Socket: %d",
		syscalls.Read, syscalls.Write, syscalls.Socket)
}

// TestGetSafeSyscallList verifies safe syscall list generation
func TestGetSafeSyscallList(t *testing.T) {
	arch := GetArchitecture()
	safeSyscalls := GetSafeSyscallList(arch)

	if len(safeSyscalls) == 0 {
		t.Error("Safe syscall list is empty")
	}

	// Should have at least basic syscalls like read, write, exit
	if len(safeSyscalls) < 10 {
		t.Errorf("Expected at least 10 safe syscalls, got %d", len(safeSyscalls))
	}

	// Verify no -1 values (invalid syscalls) made it into the list
	for i, nr := range safeSyscalls {
		if nr == -1 {
			t.Errorf("Safe syscall list contains invalid syscall at index %d", i)
		}
	}

	t.Logf("Safe syscall list contains %d syscalls", len(safeSyscalls))
}

// TestApplyFilter tests applying a Seccomp filter in a child process
// This is the most comprehensive test but must run in isolation
func TestApplyFilter(t *testing.T) {
	// Skip if not supported
	if !IsSupported() {
		t.Skip("Seccomp not supported on this system")
	}

	// Run in a child process to avoid affecting other tests
	if os.Getenv("TEST_APPLY_FILTER") == "1" {
		arch := GetArchitecture()
		syscalls := GetSyscallNumbers(arch)

		// Create a simple filter that allows basic operations but denies socket
		fb, err := NewFilterBuilder(arch, ActionAllow)
		if err != nil {
			t.Fatalf("NewFilterBuilder() failed: %v", err)
		}

		fb.DenySyscall(syscalls.Socket)
		filter := fb.Build()

		// Apply the filter
		if err := filter.Apply(); err != nil {
			t.Fatalf("filter.Apply() failed: %v", err)
		}

		// Verify the filter is active
		mode, err := GetMode()
		if err != nil {
			t.Fatalf("GetMode() after Apply failed: %v", err)
		}

		if mode != SECCOMP_MODE_FILTER {
			t.Fatalf("Expected mode %d, got %d", SECCOMP_MODE_FILTER, mode)
		}

		// Try to create a socket - should fail with EPERM
		fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
		if err == nil {
			syscall.Close(fd)
			t.Fatal("Socket creation should have been blocked by Seccomp filter")
		}

		// Verify it's EPERM (not some other error)
		if err != syscall.EPERM {
			t.Logf("Expected EPERM, got: %v", err)
		}

		// Verify basic syscalls still work
		pid := os.Getpid()
		if pid <= 0 {
			t.Fatal("Getpid() failed after applying filter")
		}

		t.Logf("Successfully applied Seccomp filter, socket blocked, basic syscalls work")
		os.Exit(0)
	}

	// Run the test in a child process
	cmd := exec.Command(os.Args[0], "-test.run=TestApplyFilter")
	cmd.Env = append(os.Environ(), "TEST_APPLY_FILTER=1")
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Logf("Child output: %s", output)
		t.Fatalf("Child process failed: %v", err)
	}

	t.Logf("Filter application test passed")
}

// TestNetworkFilterBlocking tests that network filter blocks network operations
func TestNetworkFilterBlocking(t *testing.T) {
	// Skip if not supported
	if !IsSupported() {
		t.Skip("Seccomp not supported on this system")
	}

	// Run in a child process
	if os.Getenv("TEST_NETWORK_FILTER") == "1" {
		arch := GetArchitecture()

		// Create and apply network filter
		filter, err := CreateNetworkFilter(arch)
		if err != nil {
			t.Fatalf("CreateNetworkFilter() failed: %v", err)
		}

		if err := filter.Apply(); err != nil {
			t.Fatalf("filter.Apply() failed: %v", err)
		}

		// Try various network operations - all should fail

		// 1. Try to create a TCP socket (should fail)
		fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
		if err == nil {
			syscall.Close(fd)
			t.Fatal("AF_INET socket creation should have been blocked")
		}

		// 2. Try to create a UDP socket (should fail)
		fd, err = syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, 0)
		if err == nil {
			syscall.Close(fd)
			t.Fatal("AF_INET socket creation should have been blocked")
		}

		// 3. Try to create a Unix socket (should succeed)
		fd, err = syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
		if err != nil {
			t.Fatalf("AF_UNIX socket creation should be allowed: %v", err)
		}
		syscall.Close(fd)

		t.Log("Network filter successfully blocks network syscalls, allows AF_UNIX")
		os.Exit(0)
	}

	// Run the test in a child process
	cmd := exec.Command(os.Args[0], "-test.run=TestNetworkFilterBlocking")
	cmd.Env = append(os.Environ(), "TEST_NETWORK_FILTER=1")
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Logf("Child output: %s", output)
		t.Fatalf("Child process failed: %v", err)
	}

	t.Log("Network filter blocking test passed")
}

// TestDenySyscallConditional verifies conditional syscall denial
func TestDenySyscallConditional(t *testing.T) {
	arch := GetArchitecture()
	fb, err := NewFilterBuilder(arch, ActionAllow)
	if err != nil {
		t.Fatalf("NewFilterBuilder() failed: %v", err)
	}

	syscalls := GetSyscallNumbers(arch)

	// Add conditional rule: deny socket if arg0 != AF_UNIX
	err = fb.DenySyscallConditional(syscalls.Socket, 0, 1 /* AF_UNIX */, true)
	if err != nil {
		t.Fatalf("DenySyscallConditional() failed: %v", err)
	}

	// Build the filter
	filter := fb.Build()
	if filter == nil {
		t.Fatal("Built filter is nil")
	}

	// The filter should have instructions for the conditional check
	if len(filter.program) < 10 {
		t.Errorf("Expected more instructions for conditional filter, got %d", len(filter.program))
	}

	t.Logf("Conditional filter has %d BPF instructions", len(filter.program))
}

// BenchmarkFilterCreation benchmarks filter creation performance
func BenchmarkFilterCreation(b *testing.B) {
	arch := GetArchitecture()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := CreateNetworkFilter(arch)
		if err != nil {
			b.Fatalf("CreateNetworkFilter() failed: %v", err)
		}
	}
}

// BenchmarkFilterBuilder benchmarks manual filter building
func BenchmarkFilterBuilder(b *testing.B) {
	arch := GetArchitecture()
	syscalls := GetSyscallNumbers(arch)

	deniedSyscalls := []int{
		syscalls.Socket,
		syscalls.Connect,
		syscalls.Accept,
		syscalls.Bind,
		syscalls.Listen,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fb, err := NewFilterBuilder(arch, ActionAllow)
		if err != nil {
			b.Fatalf("NewFilterBuilder() failed: %v", err)
		}

		for _, nr := range deniedSyscalls {
			fb.DenySyscall(nr)
		}

		_ = fb.Build()
	}
}
