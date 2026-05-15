package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/evmts/codex/codex-go/internal/tools/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testRepo represents a temporary git repository for testing.
type testRepo struct {
	dir string
	t   *testing.T
}

// newTestRepo creates a new temporary git repository for testing.
func newTestRepo(t *testing.T) *testRepo {
	// Create temporary directory
	dir, err := os.MkdirTemp("", "git-test-*")
	require.NoError(t, err)

	repo := &testRepo{
		dir: dir,
		t:   t,
	}

	// Initialize git repository
	repo.git("init")
	repo.git("config", "user.name", "Test User")
	repo.git("config", "user.email", "test@example.com")

	return repo
}

// cleanup removes the temporary repository.
func (r *testRepo) cleanup() {
	os.RemoveAll(r.dir)
}

// git runs a git command in the test repository.
func (r *testRepo) git(args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Logf("git %v failed: %v\nOutput: %s", args, err, output)
	}
	return string(output)
}

// writeFile writes a file in the test repository.
func (r *testRepo) writeFile(path, content string) {
	fullPath := filepath.Join(r.dir, path)
	dir := filepath.Dir(fullPath)
	if dir != r.dir {
		err := os.MkdirAll(dir, 0755)
		require.NoError(r.t, err)
	}
	err := os.WriteFile(fullPath, []byte(content), 0644)
	require.NoError(r.t, err)
}

// commit creates a commit with the given message.
func (r *testRepo) commit(message string) {
	r.git("add", ".")
	r.git("commit", "-m", message)
}

// TestGitStatus tests the git status tool.
func TestGitStatus(t *testing.T) {
	repo := newTestRepo(t)
	defer repo.cleanup()

	// Create initial commit
	repo.writeFile("README.md", "# Test")
	repo.commit("Initial commit")

	tool := NewStatusTool()
	ctx := context.Background()

	t.Run("clean working tree", func(t *testing.T) {
		req := &runtime.ToolRequest{
			CallID:           "test-1",
			ToolName:         "git_status",
			Arguments:        "{}",
			WorkingDirectory: repo.dir,
		}

		resp, err := tool.Execute(ctx, req, &runtime.ExecutionContext{})
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, *resp.Success)
		assert.Contains(t, resp.Content, "clean")
	})

	t.Run("untracked files", func(t *testing.T) {
		repo.writeFile("new.txt", "new file")

		req := &runtime.ToolRequest{
			CallID:           "test-2",
			ToolName:         "git_status",
			Arguments:        "{}",
			WorkingDirectory: repo.dir,
		}

		resp, err := tool.Execute(ctx, req, &runtime.ExecutionContext{})
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, *resp.Success)
		assert.Contains(t, resp.Content, "Untracked files")
		assert.Contains(t, resp.Content, "new.txt")
	})

	t.Run("staged changes", func(t *testing.T) {
		repo.git("add", "new.txt")

		req := &runtime.ToolRequest{
			CallID:           "test-3",
			ToolName:         "git_status",
			Arguments:        "{}",
			WorkingDirectory: repo.dir,
		}

		resp, err := tool.Execute(ctx, req, &runtime.ExecutionContext{})
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, *resp.Success)
		assert.Contains(t, resp.Content, "Changes to be committed")
		assert.Contains(t, resp.Content, "new.txt")
	})

	t.Run("modified files", func(t *testing.T) {
		repo.commit("Add new.txt")
		repo.writeFile("new.txt", "modified content")

		req := &runtime.ToolRequest{
			CallID:           "test-4",
			ToolName:         "git_status",
			Arguments:        "{}",
			WorkingDirectory: repo.dir,
		}

		resp, err := tool.Execute(ctx, req, &runtime.ExecutionContext{})
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, *resp.Success)
		assert.Contains(t, resp.Content, "not staged")
		assert.Contains(t, resp.Content, "new.txt")
	})

	t.Run("not a git repository", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "not-git-*")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		req := &runtime.ToolRequest{
			CallID:           "test-5",
			ToolName:         "git_status",
			Arguments:        "{}",
			WorkingDirectory: tempDir,
		}

		resp, err := tool.Execute(ctx, req, &runtime.ExecutionContext{})
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.False(t, *resp.Success)
		assert.Contains(t, resp.Content, "Not a git repository")
	})
}

// TestGitDiff tests the git diff tool.
func TestGitDiff(t *testing.T) {
	repo := newTestRepo(t)
	defer repo.cleanup()

	// Create initial commit
	repo.writeFile("file.txt", "original content\n")
	repo.commit("Initial commit")

	tool := NewDiffTool()
	ctx := context.Background()

	t.Run("no changes", func(t *testing.T) {
		req := &runtime.ToolRequest{
			CallID:           "test-1",
			ToolName:         "git_diff",
			Arguments:        "{}",
			WorkingDirectory: repo.dir,
		}

		resp, err := tool.Execute(ctx, req, &runtime.ExecutionContext{})
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, *resp.Success)
		assert.Contains(t, resp.Content, "No unstaged changes")
	})

	t.Run("unstaged changes", func(t *testing.T) {
		repo.writeFile("file.txt", "modified content\n")

		req := &runtime.ToolRequest{
			CallID:           "test-2",
			ToolName:         "git_diff",
			Arguments:        "{}",
			WorkingDirectory: repo.dir,
		}

		resp, err := tool.Execute(ctx, req, &runtime.ExecutionContext{})
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, *resp.Success)
		assert.Contains(t, resp.Content, "file.txt")
		assert.Contains(t, resp.Content, "-original content")
		assert.Contains(t, resp.Content, "+modified content")
	})

	t.Run("staged changes", func(t *testing.T) {
		repo.git("add", "file.txt")

		req := &runtime.ToolRequest{
			CallID:           "test-3",
			ToolName:         "git_diff",
			Arguments:        `{"staged": true}`,
			WorkingDirectory: repo.dir,
		}

		resp, err := tool.Execute(ctx, req, &runtime.ExecutionContext{})
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, *resp.Success)
		assert.Contains(t, resp.Content, "file.txt")
		assert.Contains(t, resp.Content, "-original content")
		assert.Contains(t, resp.Content, "+modified content")
	})

	t.Run("diff with path filter", func(t *testing.T) {
		repo.writeFile("other.txt", "other file\n")
		repo.git("add", ".")

		req := &runtime.ToolRequest{
			CallID:           "test-4",
			ToolName:         "git_diff",
			Arguments:        `{"staged": true, "path": "file.txt"}`,
			WorkingDirectory: repo.dir,
		}

		resp, err := tool.Execute(ctx, req, &runtime.ExecutionContext{})
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, *resp.Success)
		assert.Contains(t, resp.Content, "file.txt")
		assert.NotContains(t, resp.Content, "other.txt")
	})
}

// TestGitLog tests the git log tool.
func TestGitLog(t *testing.T) {
	repo := newTestRepo(t)
	defer repo.cleanup()

	// Create multiple commits
	repo.writeFile("file1.txt", "content 1")
	repo.commit("First commit")

	time.Sleep(100 * time.Millisecond)

	repo.writeFile("file2.txt", "content 2")
	repo.commit("Second commit")

	time.Sleep(100 * time.Millisecond)

	repo.writeFile("file3.txt", "content 3")
	repo.commit("Third commit")

	tool := NewLogTool()
	ctx := context.Background()

	t.Run("default log", func(t *testing.T) {
		req := &runtime.ToolRequest{
			CallID:           "test-1",
			ToolName:         "git_log",
			Arguments:        "{}",
			WorkingDirectory: repo.dir,
		}

		resp, err := tool.Execute(ctx, req, &runtime.ExecutionContext{})
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, *resp.Success)
		assert.Contains(t, resp.Content, "First commit")
		assert.Contains(t, resp.Content, "Second commit")
		assert.Contains(t, resp.Content, "Third commit")
	})

	t.Run("limited commits", func(t *testing.T) {
		req := &runtime.ToolRequest{
			CallID:           "test-2",
			ToolName:         "git_log",
			Arguments:        `{"max_count": 1}`,
			WorkingDirectory: repo.dir,
		}

		resp, err := tool.Execute(ctx, req, &runtime.ExecutionContext{})
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, *resp.Success)
		assert.Contains(t, resp.Content, "Third commit")
		assert.NotContains(t, resp.Content, "First commit")
	})

	t.Run("oneline format", func(t *testing.T) {
		req := &runtime.ToolRequest{
			CallID:           "test-3",
			ToolName:         "git_log",
			Arguments:        `{"oneline": true}`,
			WorkingDirectory: repo.dir,
		}

		resp, err := tool.Execute(ctx, req, &runtime.ExecutionContext{})
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, *resp.Success)
		// Oneline format should have commit hash and message on same line
		lines := strings.Split(resp.Content, "\n")
		assert.GreaterOrEqual(t, len(lines), 3)
	})

	t.Run("path filter", func(t *testing.T) {
		req := &runtime.ToolRequest{
			CallID:           "test-4",
			ToolName:         "git_log",
			Arguments:        `{"path": "file1.txt"}`,
			WorkingDirectory: repo.dir,
		}

		resp, err := tool.Execute(ctx, req, &runtime.ExecutionContext{})
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, *resp.Success)
		assert.Contains(t, resp.Content, "First commit")
		assert.NotContains(t, resp.Content, "Second commit")
		assert.NotContains(t, resp.Content, "Third commit")
	})
}

// TestGitCommit tests the git commit tool.
func TestGitCommit(t *testing.T) {
	repo := newTestRepo(t)
	defer repo.cleanup()

	// Create initial commit
	repo.writeFile("README.md", "# Test")
	repo.commit("Initial commit")

	tool := NewCommitTool()
	ctx := context.Background()

	t.Run("commit staged changes", func(t *testing.T) {
		repo.writeFile("new.txt", "new file")
		repo.git("add", "new.txt")

		req := &runtime.ToolRequest{
			CallID:           "test-1",
			ToolName:         "git_commit",
			Arguments:        `{"message": "Add new.txt"}`,
			WorkingDirectory: repo.dir,
		}

		resp, err := tool.Execute(ctx, req, &runtime.ExecutionContext{})
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, *resp.Success)
		assert.Contains(t, resp.Content, "new.txt")

		// Verify commit was created
		output := repo.git("log", "--oneline", "-1")
		assert.Contains(t, output, "Add new.txt")
	})

	t.Run("commit with -a flag", func(t *testing.T) {
		repo.writeFile("README.md", "# Modified")

		req := &runtime.ToolRequest{
			CallID:           "test-2",
			ToolName:         "git_commit",
			Arguments:        `{"message": "Update README", "all": true}`,
			WorkingDirectory: repo.dir,
		}

		resp, err := tool.Execute(ctx, req, &runtime.ExecutionContext{})
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, *resp.Success)

		// Verify commit was created
		output := repo.git("log", "--oneline", "-1")
		assert.Contains(t, output, "Update README")
	})

	t.Run("nothing to commit", func(t *testing.T) {
		req := &runtime.ToolRequest{
			CallID:           "test-3",
			ToolName:         "git_commit",
			Arguments:        `{"message": "Empty commit"}`,
			WorkingDirectory: repo.dir,
		}

		resp, err := tool.Execute(ctx, req, &runtime.ExecutionContext{})
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.False(t, *resp.Success)
		assert.Contains(t, resp.Content, "Nothing to commit")
	})

	t.Run("empty message", func(t *testing.T) {
		req := &runtime.ToolRequest{
			CallID:           "test-4",
			ToolName:         "git_commit",
			Arguments:        `{"message": ""}`,
			WorkingDirectory: repo.dir,
		}

		_, err := tool.Execute(ctx, req, &runtime.ExecutionContext{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "message cannot be empty")
	})

	t.Run("commit specific files", func(t *testing.T) {
		repo.writeFile("file1.txt", "content 1")
		repo.writeFile("file2.txt", "content 2")

		req := &runtime.ToolRequest{
			CallID:           "test-5",
			ToolName:         "git_commit",
			Arguments:        `{"message": "Add file1", "files": ["file1.txt"]}`,
			WorkingDirectory: repo.dir,
		}

		resp, err := tool.Execute(ctx, req, &runtime.ExecutionContext{})
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.True(t, *resp.Success)

		// Verify only file1.txt was committed
		output := repo.git("status", "--short")
		assert.NotContains(t, output, "file1.txt")
		assert.Contains(t, output, "file2.txt")
	})
}

// TestToolApprovalConfiguration tests approval configuration for git tools.
func TestToolApprovalConfiguration(t *testing.T) {
	t.Run("status needs no approval", func(t *testing.T) {
		tool := NewStatusTool()
		req := &runtime.ToolRequest{
			CallID:   "test",
			ToolName: "git_status",
		}

		needsApproval := tool.NeedsInitialApproval(req, runtime.ApprovalOnRequest, runtime.SandboxDangerFullAccess)
		assert.False(t, needsApproval)
	})

	t.Run("diff needs no approval", func(t *testing.T) {
		tool := NewDiffTool()
		req := &runtime.ToolRequest{
			CallID:   "test",
			ToolName: "git_diff",
		}

		needsApproval := tool.NeedsInitialApproval(req, runtime.ApprovalOnRequest, runtime.SandboxDangerFullAccess)
		assert.False(t, needsApproval)
	})

	t.Run("log needs no approval", func(t *testing.T) {
		tool := NewLogTool()
		req := &runtime.ToolRequest{
			CallID:   "test",
			ToolName: "git_log",
		}

		needsApproval := tool.NeedsInitialApproval(req, runtime.ApprovalOnRequest, runtime.SandboxDangerFullAccess)
		assert.False(t, needsApproval)
	})

	t.Run("commit needs approval", func(t *testing.T) {
		tool := NewCommitTool()
		req := &runtime.ToolRequest{
			CallID:   "test",
			ToolName: "git_commit",
		}

		needsApproval := tool.NeedsInitialApproval(req, runtime.ApprovalOnRequest, runtime.SandboxDangerFullAccess)
		assert.True(t, needsApproval)
	})

	t.Run("commit with never policy needs no approval", func(t *testing.T) {
		tool := NewCommitTool()
		req := &runtime.ToolRequest{
			CallID:   "test",
			ToolName: "git_commit",
		}

		needsApproval := tool.NeedsInitialApproval(req, runtime.ApprovalNever, runtime.SandboxDangerFullAccess)
		assert.False(t, needsApproval)
	})
}

// TestToolSandboxConfiguration tests sandbox configuration for git tools.
func TestToolSandboxConfiguration(t *testing.T) {
	t.Run("status allows sandbox", func(t *testing.T) {
		tool := NewStatusTool()
		assert.Equal(t, runtime.SandboxAuto, tool.SandboxPreference())
		assert.False(t, tool.EscalateOnFailure())
	})

	t.Run("diff allows sandbox", func(t *testing.T) {
		tool := NewDiffTool()
		assert.Equal(t, runtime.SandboxAuto, tool.SandboxPreference())
		assert.False(t, tool.EscalateOnFailure())
	})

	t.Run("log allows sandbox", func(t *testing.T) {
		tool := NewLogTool()
		assert.Equal(t, runtime.SandboxAuto, tool.SandboxPreference())
		assert.False(t, tool.EscalateOnFailure())
	})

	t.Run("commit forbids sandbox", func(t *testing.T) {
		tool := NewCommitTool()
		assert.Equal(t, runtime.SandboxForbid, tool.SandboxPreference())
		assert.True(t, tool.EscalateOnFailure())

		req := &runtime.ToolRequest{
			CallID: "test",
		}
		assert.True(t, tool.WantsEscalatedFirstAttempt(req))
	})
}

// TestHelperFunctions tests helper functions.
func TestHelperFunctions(t *testing.T) {
	t.Run("formatGitOutput", func(t *testing.T) {
		tests := []struct {
			name     string
			input    string
			expected string
		}{
			{
				name:     "empty string",
				input:    "",
				expected: "",
			},
			{
				name:     "single line",
				input:    "test",
				expected: "test",
			},
			{
				name:     "trim leading newlines",
				input:    "\n\ntest\n",
				expected: "test",
			},
			{
				name:     "trim trailing newlines",
				input:    "test\n\n\n",
				expected: "test",
			},
			{
				name:     "preserve internal newlines",
				input:    "line1\nline2\nline3",
				expected: "line1\nline2\nline3",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := formatGitOutput(tt.input)
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("parseFileStatus", func(t *testing.T) {
		tests := []struct {
			name        string
			input       string
			expectedXY  string
			expectedPath string
			expectedOld string
		}{
			{
				name:         "modified file",
				input:        " M file.txt",
				expectedXY:   " M",
				expectedPath: "file.txt",
				expectedOld:  "",
			},
			{
				name:         "added file",
				input:        "A  new.txt",
				expectedXY:   "A ",
				expectedPath: "new.txt",
				expectedOld:  "",
			},
			{
				name:         "renamed file",
				input:        "R  old.txt -> new.txt",
				expectedXY:   "R ",
				expectedPath: "new.txt",
				expectedOld:  "old.txt",
			},
			{
				name:         "untracked file",
				input:        "?? untracked.txt",
				expectedXY:   "??",
				expectedPath: "untracked.txt",
				expectedOld:  "",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				xy, path, oldPath := parseFileStatus(tt.input)
				assert.Equal(t, tt.expectedXY, xy)
				assert.Equal(t, tt.expectedPath, path)
				assert.Equal(t, tt.expectedOld, oldPath)
			})
		}
	})

	t.Run("statusCodeDescription", func(t *testing.T) {
		tests := []struct {
			code     string
			expected string
		}{
			{code: "M ", expected: "modified"},
			{code: " M", expected: "modified"},
			{code: "MM", expected: "modified, modified in working tree"},
			{code: "A ", expected: "added"},
			{code: "D ", expected: "deleted"},
			{code: "R ", expected: "renamed"},
			{code: "??", expected: "untracked"},
			{code: "  ", expected: "unchanged"},
		}

		for _, tt := range tests {
			t.Run(tt.code, func(t *testing.T) {
				result := statusCodeDescription(tt.code)
				assert.Equal(t, tt.expected, result)
			})
		}
	})
}
