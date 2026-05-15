package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSanitizeGitArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid status command",
			args:    []string{"status", "--short"},
			wantErr: false,
		},
		{
			name:    "valid diff command",
			args:    []string{"diff", "--cached"},
			wantErr: false,
		},
		{
			name:    "valid log command",
			args:    []string{"log", "--oneline", "-n", "10"},
			wantErr: false,
		},
		{
			name:    "dangerous command - push",
			args:    []string{"push", "--force"},
			wantErr: true,
			errMsg:  "not allowed",
		},
		{
			name:    "dangerous command - pull",
			args:    []string{"pull", "origin", "main"},
			wantErr: true,
			errMsg:  "not allowed",
		},
		{
			name:    "dangerous command - fetch",
			args:    []string{"fetch", "origin"},
			wantErr: true,
			errMsg:  "not allowed",
		},
		{
			name:    "dangerous command - clone",
			args:    []string{"clone", "https://github.com/example/repo.git"},
			wantErr: true,
			errMsg:  "not allowed",
		},
		{
			name:    "command injection attempt with semicolon",
			args:    []string{"status", "; rm -rf /"},
			wantErr: false, // Warning only, not blocked
		},
		{
			name:    "command injection attempt with pipe",
			args:    []string{"status", "| cat /etc/passwd"},
			wantErr: false, // Warning only, not blocked
		},
		{
			name:    "dangerous option - upload-pack",
			args:    []string{"status", "--upload-pack=evil"},
			wantErr: true,
			errMsg:  "dangerous",
		},
		{
			name:    "dangerous option - receive-pack",
			args:    []string{"status", "--receive-pack=evil"},
			wantErr: true,
			errMsg:  "dangerous",
		},
		{
			name:    "dangerous option - exec",
			args:    []string{"status", "--exec=evil"},
			wantErr: true,
			errMsg:  "dangerous",
		},
		{
			name:    "null byte in argument",
			args:    []string{"status", "\x00--all"},
			wantErr: true,
			errMsg:  "null byte",
		},
		{
			name:    "null byte in middle",
			args:    []string{"status", "file\x00name.txt"},
			wantErr: true,
			errMsg:  "null byte",
		},
		{
			name:    "too many args",
			args:    make([]string, MaxGitArgsCount+1),
			wantErr: true,
			errMsg:  "too many",
		},
		{
			name:    "config override attempt with -c",
			args:    []string{"status", "-c", "protocol.ext.allow=always"},
			wantErr: true,
			errMsg:  "dangerous",
		},
		{
			name:    "config override attempt with --config",
			args:    []string{"status", "--config=protocol.ext.allow=always"},
			wantErr: true,
			errMsg:  "dangerous",
		},
		{
			name:    "empty args",
			args:    []string{},
			wantErr: true,
			errMsg:  "cannot be empty",
		},
		{
			name:    "arg exceeds max length",
			args:    []string{"status", strings.Repeat("a", MaxGitArgLength+1)},
			wantErr: true,
			errMsg:  "exceeds maximum length",
		},
		{
			name:    "valid commit command",
			args:    []string{"commit", "-m", "Test commit"},
			wantErr: false,
		},
		{
			name:    "valid config read",
			args:    []string{"config", "user.name"},
			wantErr: false,
		},
		{
			name:    "dangerous option standalone -c",
			args:    []string{"log", "-c"},
			wantErr: true,
			errMsg:  "dangerous",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize first element for too many args test
			if len(tt.args) > 0 && tt.args[0] == "" {
				tt.args[0] = "status"
			}

			err := sanitizeGitArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("sanitizeGitArgs() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errMsg != "" && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("expected error containing '%s', got '%v'", tt.errMsg, err)
			}
		})
	}
}

func TestValidateWorkingDirectory(t *testing.T) {
	// Create a temp directory as a git repo
	tmpDir := t.TempDir()
	exec.Command("git", "init", tmpDir).Run()
	exec.Command("git", "config", "user.name", "Test User").Run()
	exec.Command("git", "config", "user.email", "test@example.com").Run()

	executor := newGitExecutor()

	tests := []struct {
		name    string
		workDir string
		setup   func() string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid git repo",
			workDir: tmpDir,
			wantErr: false,
		},
		{
			name:    "non-existent directory",
			workDir: "/nonexistent/path/that/does/not/exist",
			wantErr: true,
			errMsg:  "does not exist",
		},
		{
			name: "file not directory",
			setup: func() string {
				filePath := filepath.Join(tmpDir, "file.txt")
				os.WriteFile(filePath, []byte("test"), 0644)
				return filePath
			},
			wantErr: true,
			errMsg:  "not a directory",
		},
		{
			name:    "empty working directory",
			workDir: "",
			wantErr: true,
			errMsg:  "cannot be empty",
		},
		{
			name: "non-git directory",
			setup: func() string {
				nonGitDir := t.TempDir()
				return nonGitDir
			},
			wantErr: true,
			errMsg:  "not a git repository",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workDir := tt.workDir
			if tt.setup != nil {
				workDir = tt.setup()
			}

			err := executor.validateWorkingDirectory(workDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateWorkingDirectory() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errMsg != "" && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("expected error containing '%s', got '%v'", tt.errMsg, err)
			}
		})
	}
}

func TestExecuteGit_Timeout(t *testing.T) {
	tmpDir := t.TempDir()
	exec.Command("git", "init", tmpDir).Run()
	exec.Command("git", "config", "user.name", "Test User").Run()
	exec.Command("git", "config", "user.email", "test@example.com").Run()

	// Create initial commit
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tmpDir
	cmd.Run()

	executor := newGitExecutor()
	ctx := context.Background()

	t.Run("command completes within timeout", func(t *testing.T) {
		_, _, err := executor.executeGitWithTimeout(ctx, tmpDir, DefaultGitTimeout, "status")
		if err != nil {
			t.Errorf("executeGitWithTimeout() unexpected error: %v", err)
		}
	})

	t.Run("uses default timeout when zero", func(t *testing.T) {
		// This should use DefaultGitTimeout internally
		_, _, err := executor.executeGitWithTimeout(ctx, tmpDir, 0, "status")
		if err != nil {
			t.Errorf("executeGitWithTimeout() with zero timeout unexpected error: %v", err)
		}
	})
}

func TestExecuteGit_ValidationIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	exec.Command("git", "init", tmpDir).Run()
	exec.Command("git", "config", "user.name", "Test User").Run()
	exec.Command("git", "config", "user.email", "test@example.com").Run()

	// Create initial commit
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tmpDir
	cmd.Run()

	executor := newGitExecutor()
	ctx := context.Background()

	t.Run("valid command executes successfully", func(t *testing.T) {
		stdout, stderr, err := executor.executeGit(ctx, tmpDir, "status", "--short")
		if err != nil {
			t.Errorf("executeGit() unexpected error: %v, stderr: %s", err, stderr)
		}
		_ = stdout
	})

	t.Run("invalid working directory fails", func(t *testing.T) {
		_, _, err := executor.executeGit(ctx, "/nonexistent", "status")
		if err == nil {
			t.Error("executeGit() expected error for nonexistent directory")
		}
		if !strings.Contains(err.Error(), "invalid working directory") {
			t.Errorf("expected 'invalid working directory' error, got: %v", err)
		}
	})

	t.Run("dangerous command fails", func(t *testing.T) {
		_, _, err := executor.executeGit(ctx, tmpDir, "push", "--force")
		if err == nil {
			t.Error("executeGit() expected error for dangerous command")
		}
		if !strings.Contains(err.Error(), "not allowed") {
			t.Errorf("expected 'not allowed' error, got: %v", err)
		}
	})

	t.Run("dangerous option fails", func(t *testing.T) {
		_, _, err := executor.executeGit(ctx, tmpDir, "status", "--upload-pack=evil")
		if err == nil {
			t.Error("executeGit() expected error for dangerous option")
		}
		if !strings.Contains(err.Error(), "dangerous") {
			t.Errorf("expected 'dangerous' error, got: %v", err)
		}
	})
}

func TestValidateGitDiffArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid relative path",
			args:    []string{"src/main.go"},
			wantErr: false,
		},
		{
			name:    "valid option",
			args:    []string{"--cached"},
			wantErr: false,
		},
		{
			name:    "absolute path blocked",
			args:    []string{"/etc/passwd"},
			wantErr: true,
			errMsg:  "absolute paths not allowed",
		},
		{
			name:    "path traversal blocked",
			args:    []string{"../../etc/passwd"},
			wantErr: true,
			errMsg:  "path traversal not allowed",
		},
		{
			name:    "path traversal in middle blocked",
			args:    []string{"src/../../../etc/passwd"},
			wantErr: true,
			errMsg:  "path traversal not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGitDiffArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateGitDiffArgs() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errMsg != "" && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("expected error containing '%s', got '%v'", tt.errMsg, err)
			}
		})
	}
}

func TestValidateGitLogArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "with limit using -n",
			args:    []string{"-n", "10"},
			wantErr: false,
		},
		{
			name:    "with limit using --max-count",
			args:    []string{"--max-count=10"},
			wantErr: false,
		},
		{
			name:    "without explicit limit",
			args:    []string{"--oneline"},
			wantErr: false, // Currently doesn't error, just warns
		},
		{
			name:    "empty args",
			args:    []string{},
			wantErr: false, // Currently doesn't error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGitLogArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateGitLogArgs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsGitRepo_UsesUnsafeExecution(t *testing.T) {
	// This test verifies that isGitRepo uses executeGitUnsafe
	// to avoid circular dependency with validateWorkingDirectory

	tmpDir := t.TempDir()
	exec.Command("git", "init", tmpDir).Run()

	executor := newGitExecutor()
	ctx := context.Background()

	t.Run("valid git repo", func(t *testing.T) {
		isRepo := executor.isGitRepo(ctx, tmpDir)
		if !isRepo {
			t.Error("isGitRepo() expected true for valid git repo")
		}
	})

	t.Run("non-git directory", func(t *testing.T) {
		nonGitDir := t.TempDir()
		isRepo := executor.isGitRepo(ctx, nonGitDir)
		if isRepo {
			t.Error("isGitRepo() expected false for non-git directory")
		}
	})

	t.Run("nonexistent directory", func(t *testing.T) {
		isRepo := executor.isGitRepo(ctx, "/nonexistent/path")
		if isRepo {
			t.Error("isGitRepo() expected false for nonexistent directory")
		}
	})
}

func TestSecurityConstants(t *testing.T) {
	t.Run("default timeout is reasonable", func(t *testing.T) {
		if DefaultGitTimeout <= 0 {
			t.Error("DefaultGitTimeout should be positive")
		}
		if DefaultGitTimeout > 5*60*time.Second { // 5 minutes
			t.Error("DefaultGitTimeout seems too long")
		}
	})

	t.Run("max args count is reasonable", func(t *testing.T) {
		if MaxGitArgsCount <= 0 {
			t.Error("MaxGitArgsCount should be positive")
		}
		if MaxGitArgsCount < 10 {
			t.Error("MaxGitArgsCount seems too restrictive")
		}
	})

	t.Run("max arg length is reasonable", func(t *testing.T) {
		if MaxGitArgLength <= 0 {
			t.Error("MaxGitArgLength should be positive")
		}
		if MaxGitArgLength < 100 {
			t.Error("MaxGitArgLength seems too restrictive")
		}
	})
}

func TestAllowedCommands(t *testing.T) {
	readOnlyCommands := []string{"status", "diff", "log", "show", "branch", "rev-parse"}
	writeCommands := []string{"commit", "config", "init", "add"}

	t.Run("read-only commands are allowed", func(t *testing.T) {
		for _, cmd := range readOnlyCommands {
			if !allowedGitCommands[cmd] {
				t.Errorf("read-only command %s should be allowed", cmd)
			}
		}
	})

	t.Run("write commands are allowed", func(t *testing.T) {
		for _, cmd := range writeCommands {
			if !allowedGitCommands[cmd] {
				t.Errorf("write command %s should be allowed", cmd)
			}
		}
	})

	t.Run("dangerous commands are not allowed", func(t *testing.T) {
		dangerousCommands := []string{"push", "pull", "fetch", "clone", "remote"}
		for _, cmd := range dangerousCommands {
			if allowedGitCommands[cmd] {
				t.Errorf("dangerous command %s should not be allowed", cmd)
			}
		}
	})
}

func TestDangerousOptions(t *testing.T) {
	expectedDangerous := []string{"--upload-pack", "--receive-pack", "--exec", "-c", "--config"}

	t.Run("all dangerous options are in list", func(t *testing.T) {
		for _, opt := range expectedDangerous {
			found := false
			for _, dangerous := range dangerousGitOptions {
				if dangerous == opt {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected dangerous option %s not found in dangerousGitOptions", opt)
			}
		}
	})

	t.Run("dangerous options list is not empty", func(t *testing.T) {
		if len(dangerousGitOptions) == 0 {
			t.Error("dangerousGitOptions should not be empty")
		}
	})
}

// Benchmark tests for performance validation
func BenchmarkSanitizeGitArgs(b *testing.B) {
	args := []string{"status", "--short", "--branch"}
	for i := 0; i < b.N; i++ {
		sanitizeGitArgs(args)
	}
}

func BenchmarkValidateWorkingDirectory(b *testing.B) {
	tmpDir := b.TempDir()
	exec.Command("git", "init", tmpDir).Run()

	executor := newGitExecutor()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		executor.validateWorkingDirectory(tmpDir)
	}
}
