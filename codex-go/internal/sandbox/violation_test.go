package sandbox

import (
	"strings"
	"testing"
	"time"
)

func TestDetectViolation_NoViolation(t *testing.T) {
	detector := NewViolationDetector("native")

	tests := []struct {
		name   string
		result *Result
	}{
		{
			name: "successful command",
			result: &Result{
				Stdout:   "hello world",
				Stderr:   "",
				ExitCode: 0,
			},
		},
		{
			name: "command not found",
			result: &Result{
				Stdout:   "",
				Stderr:   "bash: foobar: command not found",
				ExitCode: 127,
			},
		},
		{
			name: "shell builtin misuse",
			result: &Result{
				Stdout:   "",
				Stderr:   "bash: cd: too many arguments",
				ExitCode: 2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violation := detector.DetectViolation(tt.result)
			if violation != nil {
				t.Errorf("expected no violation, got: %+v", violation)
			}
		})
	}
}

func TestDetectViolation_SeatbeltViolations(t *testing.T) {
	detector := NewViolationDetector("native")

	tests := []struct {
		name           string
		result         *Result
		expectedType   ViolationType
		expectedOp     string
		shouldHavePath bool
	}{
		{
			name: "seatbelt write denial",
			result: &Result{
				Stdout:   "",
				Stderr:   "sandbox-exec: /etc/hosts: Operation not permitted",
				ExitCode: 1,
			},
			expectedType:   ViolationTypeFileSystem,
			expectedOp:     "seatbelt_denied",
			shouldHavePath: true,
		},
		{
			name: "seatbelt generic error",
			result: &Result{
				Stdout:   "",
				Stderr:   "Sandbox: failed to write file due to seatbelt policy",
				ExitCode: 1,
			},
			expectedType:   ViolationTypeFileSystem,
			expectedOp:     "write",
			shouldHavePath: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violation := detector.DetectViolation(tt.result)
			if violation == nil {
				t.Fatal("expected violation, got nil")
			}
			if violation.Type != tt.expectedType {
				t.Errorf("expected type %s, got %s", tt.expectedType, violation.Type)
			}
			if !strings.Contains(violation.Operation, tt.expectedOp) && violation.Operation != tt.expectedOp {
				t.Errorf("expected operation containing %s, got %s", tt.expectedOp, violation.Operation)
			}
			if tt.shouldHavePath && violation.Path == nil {
				t.Error("expected path to be set, but it was nil")
			}
		})
	}
}

func TestDetectViolation_LandlockViolations(t *testing.T) {
	detector := NewViolationDetector("native")

	tests := []struct {
		name           string
		result         *Result
		expectedType   ViolationType
		shouldHavePath bool
	}{
		{
			name: "landlock EACCES",
			result: &Result{
				Stdout:   "",
				Stderr:   "open /root/secret.txt: EACCES (Permission denied)",
				ExitCode: 1,
			},
			expectedType:   ViolationTypeFileSystem,
			shouldHavePath: true,
		},
		{
			name: "landlock EPERM",
			result: &Result{
				Stdout:   "",
				Stderr:   "write to /etc/passwd failed: EPERM",
				ExitCode: 1,
			},
			expectedType:   ViolationTypeFileSystem,
			shouldHavePath: true,
		},
		{
			name: "landlock with keyword",
			result: &Result{
				Stdout:   "",
				Stderr:   "landlock: operation not permitted on /var/sensitive",
				ExitCode: 1,
			},
			expectedType:   ViolationTypeFileSystem,
			shouldHavePath: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violation := detector.DetectViolation(tt.result)
			if violation == nil {
				t.Fatal("expected violation, got nil")
			}
			if violation.Type != tt.expectedType {
				t.Errorf("expected type %s, got %s", tt.expectedType, violation.Type)
			}
			if tt.shouldHavePath && violation.Path == nil {
				t.Error("expected path to be set, but it was nil")
			}
			if tt.shouldHavePath && violation.Path != nil {
				t.Logf("Detected path: %s", *violation.Path)
			}
		})
	}
}

func TestDetectViolation_SeccompViolations(t *testing.T) {
	detector := NewViolationDetector("native")

	tests := []struct {
		name         string
		result       *Result
		expectedType ViolationType
	}{
		{
			name: "seccomp SIGSYS signal",
			result: &Result{
				Stdout:   "",
				Stderr:   "",
				ExitCode: 159, // 128 + 31 (SIGSYS)
			},
			expectedType: ViolationTypeSyscall,
		},
		{
			name: "seccomp with error message",
			result: &Result{
				Stdout:   "",
				Stderr:   "seccomp: connect syscall blocked",
				ExitCode: 1,
			},
			expectedType: ViolationTypeSyscall,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violation := detector.DetectViolation(tt.result)
			if violation == nil {
				t.Fatal("expected violation, got nil")
			}
			if violation.Type != tt.expectedType {
				t.Errorf("expected type %s, got %s", tt.expectedType, violation.Type)
			}
		})
	}
}

func TestDetectViolation_FilesystemViolations(t *testing.T) {
	detector := NewViolationDetector("native")

	tests := []struct {
		name           string
		result         *Result
		expectedType   ViolationType
		shouldHavePath bool
	}{
		{
			name: "read-only filesystem",
			result: &Result{
				Stdout:   "",
				Stderr:   "cannot create file: Read-only file system",
				ExitCode: 1,
			},
			expectedType:   ViolationTypeFileSystem,
			shouldHavePath: false,
		},
		{
			name: "failed to write file",
			result: &Result{
				Stdout:   "",
				Stderr:   "failed to write file /tmp/test.txt",
				ExitCode: 1,
			},
			expectedType:   ViolationTypeFileSystem,
			shouldHavePath: true,
		},
		{
			name: "operation not permitted on path",
			result: &Result{
				Stdout:   "",
				Stderr:   "operation not permitted: /etc/shadow",
				ExitCode: 1,
			},
			expectedType:   ViolationTypeFileSystem,
			shouldHavePath: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violation := detector.DetectViolation(tt.result)
			if violation == nil {
				t.Fatal("expected violation, got nil")
			}
			if violation.Type != tt.expectedType {
				t.Errorf("expected type %s, got %s", tt.expectedType, violation.Type)
			}
			if tt.shouldHavePath && violation.Path == nil {
				t.Error("expected path to be set, but it was nil")
			}
		})
	}
}

func TestExtractPath(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name:     "permission denied with path",
			output:   "permission denied: /etc/passwd",
			expected: "/etc/passwd",
		},
		{
			name:     "path before error",
			output:   "/var/log/test.log: permission denied",
			expected: "/var/log/test.log",
		},
		{
			name:     "open system call",
			output:   "open /home/user/file.txt: permission denied",
			expected: "/home/user/file.txt",
		},
		{
			name:     "no path present",
			output:   "generic error occurred",
			expected: "",
		},
		{
			name:     "complex error with path",
			output:   "Error: cannot access /usr/local/bin/myapp: Operation not permitted",
			expected: "/usr/local/bin/myapp",
		},
		{
			name:     "path with spaces in directory name",
			output:   "open /home/user/my documents/file.txt failed",
			expected: "/home/user/my",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractPath(tt.output)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestExtractSyscall(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name:     "syscall with colon",
			output:   "syscall: connect",
			expected: "connect",
		},
		{
			name:     "blocked syscall",
			output:   "blocked syscall: socket",
			expected: "socket",
		},
		{
			name:     "seccomp format",
			output:   "seccomp: sendto not allowed",
			expected: "sendto",
		},
		{
			name:     "no syscall present",
			output:   "generic error",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractSyscall(tt.output)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestExtractErrorLine(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name: "multi-line with error",
			output: `some output
error: permission denied
more output`,
			expected: "error: permission denied",
		},
		{
			name: "first line is error",
			output: `permission denied: /etc/hosts
some other line`,
			expected: "permission denied: /etc/hosts",
		},
		{
			name:     "single line",
			output:   "sandbox violation occurred",
			expected: "sandbox violation occurred",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractErrorLine(tt.output)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestFormatViolation(t *testing.T) {
	path := "/etc/passwd"
	syscall := "connect"

	violation := &Violation{
		Type:         ViolationTypeFileSystem,
		Operation:    "write",
		Path:         &path,
		Syscall:      &syscall,
		ErrorMessage: "permission denied",
		ExitCode:     1,
		Timestamp:    time.Now(),
	}

	formatted := violation.FormatViolation()

	// Check that all key information is present
	if !strings.Contains(formatted, "filesystem") {
		t.Error("expected formatted output to contain violation type")
	}
	if !strings.Contains(formatted, "write") {
		t.Error("expected formatted output to contain operation")
	}
	if !strings.Contains(formatted, "/etc/passwd") {
		t.Error("expected formatted output to contain path")
	}
	if !strings.Contains(formatted, "connect") {
		t.Error("expected formatted output to contain syscall")
	}
	if !strings.Contains(formatted, "permission denied") {
		t.Error("expected formatted output to contain error message")
	}
}

func TestHasKeywordInOutput(t *testing.T) {
	detector := NewViolationDetector("native")

	tests := []struct {
		name     string
		result   *Result
		expected bool
	}{
		{
			name: "has sandbox keyword in stderr",
			result: &Result{
				Stderr: "sandbox violation detected",
			},
			expected: true,
		},
		{
			name: "has permission denied in stdout",
			result: &Result{
				Stdout: "Permission Denied",
			},
			expected: true,
		},
		{
			name: "has seccomp keyword",
			result: &Result{
				Stderr: "seccomp filter blocked syscall",
			},
			expected: true,
		},
		{
			name: "no keywords",
			result: &Result{
				Stderr: "some other error",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.hasKeywordInOutput(tt.result)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// Benchmark tests
func BenchmarkDetectViolation(b *testing.B) {
	detector := NewViolationDetector("native")
	result := &Result{
		Stdout:   "",
		Stderr:   "permission denied: /etc/passwd",
		ExitCode: 1,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detector.DetectViolation(result)
	}
}

func BenchmarkExtractPath(b *testing.B) {
	output := "open /home/user/very/long/path/to/some/file.txt: permission denied"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractPath(output)
	}
}
