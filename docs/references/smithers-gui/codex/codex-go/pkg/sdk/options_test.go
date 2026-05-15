package sdk

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/evmts/codex/codex-go/internal/tools/runtime"
	"github.com/evmts/codex/codex-go/pkg/sdk/client"
)

func TestOptions_Validate(t *testing.T) {
	validClient := &client.Client{}

	tests := []struct {
		name    string
		opts    Options
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid options with client only",
			opts: Options{
				Client: validClient,
			},
			wantErr: false,
		},
		{
			name:    "missing client",
			opts:    Options{},
			wantErr: true,
			errMsg:  "client is required",
		},
		{
			name: "valid with tools",
			opts: Options{
				Client: validClient,
				Tools:  []runtime.ToolRuntime{&mockTool{}},
			},
			wantErr: false,
		},
		{
			name: "valid with tool registry",
			opts: Options{
				Client:       validClient,
				ToolRegistry: runtime.NewToolRegistry(),
			},
			wantErr: false,
		},
		{
			name: "both tools and tool registry",
			opts: Options{
				Client:       validClient,
				Tools:        []runtime.ToolRuntime{&mockTool{}},
				ToolRegistry: runtime.NewToolRegistry(),
			},
			wantErr: true,
			errMsg:  "cannot specify both ToolRegistry and Tools",
		},
		{
			name: "valid history path",
			opts: Options{
				Client:        validClient,
				EnableHistory: true,
				HistoryPath:   filepath.Join(os.TempDir(), "test-history.jsonl"),
			},
			wantErr: false,
		},
		{
			name: "relative history path",
			opts: Options{
				Client:        validClient,
				EnableHistory: true,
				HistoryPath:   "relative/path.jsonl",
			},
			wantErr: true,
			errMsg:  "must be an absolute path",
		},
		{
			name: "history path in system directory",
			opts: Options{
				Client:        validClient,
				EnableHistory: true,
				HistoryPath:   "/etc/codex-history.jsonl",
			},
			wantErr: true,
			errMsg:  "cannot be in system directory",
		},
		{
			name: "history path with non-existent parent",
			opts: Options{
				Client:        validClient,
				EnableHistory: true,
				HistoryPath:   "/nonexistent/directory/history.jsonl",
			},
			wantErr: true,
			errMsg:  "parent directory does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Options.Validate() expected error but got nil")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Options.Validate() error = %v, should contain %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Options.Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestSessionOptions_Validate(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name    string
		opts    SessionOptions
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty options (valid)",
			opts:    SessionOptions{},
			wantErr: false,
		},
		{
			name: "valid approval policy - auto",
			opts: SessionOptions{
				ApprovalPolicy: ApprovalPolicyAuto,
			},
			wantErr: false,
		},
		{
			name: "valid approval policy - manual with callback",
			opts: SessionOptions{
				ApprovalPolicy: ApprovalPolicyManual,
				OnToolApproval: func(string, string) bool { return true },
			},
			wantErr: false,
		},
		{
			name: "valid approval policy - never",
			opts: SessionOptions{
				ApprovalPolicy: ApprovalPolicyNever,
			},
			wantErr: false,
		},
		{
			name: "invalid approval policy",
			opts: SessionOptions{
				ApprovalPolicy: "invalid",
			},
			wantErr: true,
			errMsg:  "invalid ApprovalPolicy",
		},
		{
			name: "typo in approval policy",
			opts: SessionOptions{
				ApprovalPolicy: "alwyas", // typo
			},
			wantErr: true,
			errMsg:  "invalid ApprovalPolicy",
		},
		{
			name: "manual approval without callback",
			opts: SessionOptions{
				ApprovalPolicy: ApprovalPolicyManual,
				OnToolApproval: nil,
			},
			wantErr: true,
			errMsg:  "OnToolApproval callback is required",
		},
		{
			name: "valid sandbox policy - off",
			opts: SessionOptions{
				SandboxPolicy: SandboxPolicyOff,
			},
			wantErr: false,
		},
		{
			name: "valid sandbox policy - read-only",
			opts: SessionOptions{
				SandboxPolicy: SandboxPolicyReadOnly,
			},
			wantErr: false,
		},
		{
			name: "valid sandbox policy - workspace-write",
			opts: SessionOptions{
				SandboxPolicy: SandboxPolicyWorkspaceWrite,
			},
			wantErr: false,
		},
		{
			name: "valid sandbox policy - native",
			opts: SessionOptions{
				SandboxPolicy: SandboxPolicyNative,
			},
			wantErr: false,
		},
		{
			name: "valid sandbox policy - danger-full-access",
			opts: SessionOptions{
				SandboxPolicy: SandboxPolicyDangerFullAccess,
			},
			wantErr: false,
		},
		{
			name: "invalid sandbox policy",
			opts: SessionOptions{
				SandboxPolicy: "invalid",
			},
			wantErr: true,
			errMsg:  "invalid SandboxPolicy",
		},
		{
			name: "sandbox policy with underscores instead of hyphens",
			opts: SessionOptions{
				SandboxPolicy: "read_only", // wrong format
			},
			wantErr: true,
			errMsg:  "invalid SandboxPolicy",
		},
		{
			name: "valid working directory",
			opts: SessionOptions{
				WorkingDirectory: tempDir,
			},
			wantErr: false,
		},
		{
			name: "relative working directory",
			opts: SessionOptions{
				WorkingDirectory: "relative/path",
			},
			wantErr: true,
			errMsg:  "must be an absolute path",
		},
		{
			name: "non-existent working directory",
			opts: SessionOptions{
				WorkingDirectory: "/nonexistent/directory",
			},
			wantErr: true,
			errMsg:  "does not exist",
		},
		{
			name: "working directory is a file",
			opts: SessionOptions{
				WorkingDirectory: createTempFile(t, tempDir),
			},
			wantErr: true,
			errMsg:  "is not a directory",
		},
		{
			name: "working directory in system root",
			opts: SessionOptions{
				WorkingDirectory: "/",
			},
			wantErr: true,
			errMsg:  "cannot be in system directory",
		},
		{
			name: "working directory in /etc",
			opts: SessionOptions{
				WorkingDirectory: "/etc",
			},
			wantErr: true,
			errMsg:  "cannot be in system directory",
		},
		{
			name: "valid model",
			opts: SessionOptions{
				Model: "gpt-4",
			},
			wantErr: false,
		},
		{
			name: "valid model with version",
			opts: SessionOptions{
				Model: "claude-3-opus-20240229",
			},
			wantErr: false,
		},
		{
			name: "valid model with underscores",
			opts: SessionOptions{
				Model: "gpt_5_codex",
			},
			wantErr: false,
		},
		{
			name: "model with invalid characters",
			opts: SessionOptions{
				Model: "gpt-4!",
			},
			wantErr: true,
			errMsg:  "invalid character",
		},
		{
			name: "model with spaces",
			opts: SessionOptions{
				Model: "gpt 4",
			},
			wantErr: true,
			errMsg:  "invalid character",
		},
		{
			name: "extremely long model name",
			opts: SessionOptions{
				Model: strings.Repeat("a", 101),
			},
			wantErr: true,
			errMsg:  "too long",
		},
		{
			name: "valid system prompt",
			opts: SessionOptions{
				SystemPrompt: "You are a helpful assistant.",
			},
			wantErr: false,
		},
		{
			name: "extremely large system prompt",
			opts: SessionOptions{
				SystemPrompt: strings.Repeat("a", 100001),
			},
			wantErr: true,
			errMsg:  "SystemPrompt is too large",
		},
		{
			name: "valid conversation ID",
			opts: SessionOptions{
				ConversationID: "test-conversation-123",
			},
			wantErr: false,
		},
		{
			name: "combined valid options",
			opts: SessionOptions{
				SystemPrompt:     "Test prompt",
				ApprovalPolicy:   ApprovalPolicyAuto,
				SandboxPolicy:    SandboxPolicyNative,
				WorkingDirectory: tempDir,
				Model:            "gpt-4",
				ConversationID:   "test-123",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("SessionOptions.Validate() expected error but got nil")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("SessionOptions.Validate() error = %v, should contain %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("SessionOptions.Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestValidateApprovalPolicy(t *testing.T) {
	tests := []struct {
		name    string
		policy  string
		wantErr bool
	}{
		{"auto", ApprovalPolicyAuto, false},
		{"manual", ApprovalPolicyManual, false},
		{"never", ApprovalPolicyNever, false},
		{"invalid", "invalid", true},
		{"always (old name)", "always", true},
		{"empty", "", true},
		{"typo", "autoo", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateApprovalPolicy(tt.policy)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateApprovalPolicy(%q) error = %v, wantErr %v", tt.policy, err, tt.wantErr)
			}
		})
	}
}

func TestValidateSandboxPolicy(t *testing.T) {
	tests := []struct {
		name    string
		policy  string
		wantErr bool
	}{
		{"off", SandboxPolicyOff, false},
		{"read-only", SandboxPolicyReadOnly, false},
		{"workspace-write", SandboxPolicyWorkspaceWrite, false},
		{"native", SandboxPolicyNative, false},
		{"danger-full-access", SandboxPolicyDangerFullAccess, false},
		{"invalid", "invalid", true},
		{"read_only (underscore)", "read_only", true},
		{"workspace_write (underscore)", "workspace_write", true},
		{"full_access (old name)", "full_access", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSandboxPolicy(tt.policy)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSandboxPolicy(%q) error = %v, wantErr %v", tt.policy, err, tt.wantErr)
			}
		})
	}
}

func TestValidateModel(t *testing.T) {
	tests := []struct {
		name    string
		model   string
		wantErr bool
	}{
		{"gpt-4", "gpt-4", false},
		{"gpt-5-codex", "gpt-5-codex", false},
		{"claude-3-opus-20240229", "claude-3-opus-20240229", false},
		{"with underscores", "model_name_123", false},
		{"with dots", "model.v1.0", false},
		{"uppercase", "GPT-4", false},
		{"mixed case", "Claude-3-Opus", false},
		{"empty", "", true},
		{"with spaces", "gpt 4", true},
		{"with special chars", "gpt-4!", true},
		{"with slash", "gpt/4", true},
		{"too long", strings.Repeat("a", 101), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateModel(tt.model)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateModel(%q) error = %v, wantErr %v", tt.model, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePath(t *testing.T) {
	tempDir := t.TempDir()
	validPath := filepath.Join(tempDir, "test.jsonl")

	tests := []struct {
		name     string
		path     string
		pathType string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "valid path",
			path:     validPath,
			pathType: "test path",
			wantErr:  false,
		},
		{
			name:     "empty path",
			path:     "",
			pathType: "test path",
			wantErr:  true,
			errMsg:   "cannot be empty",
		},
		{
			name:     "relative path",
			path:     "relative/path",
			pathType: "test path",
			wantErr:  true,
			errMsg:   "must be an absolute path",
		},
		{
			name:     "system directory /etc",
			path:     "/etc/test.jsonl",
			pathType: "test path",
			wantErr:  true,
			errMsg:   "cannot be in system directory",
		},
		{
			name:     "non-existent parent",
			path:     "/nonexistent/dir/file.jsonl",
			pathType: "test path",
			wantErr:  true,
			errMsg:   "parent directory does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePath(tt.path, tt.pathType)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validatePath() expected error but got nil")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("validatePath() error = %v, should contain %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("validatePath() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestValidateNotSystemDirectory(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name     string
		path     string
		pathType string
		wantErr  bool
	}{
		{"safe temp directory", tempDir, "test path", false},
		{"home directory", os.Getenv("HOME"), "test path", false},
		{"root directory", "/", "test path", true},
		{"etc directory", "/etc", "test path", true},
		{"etc subdirectory", "/etc/nginx", "test path", true},
		{"bin directory", "/bin", "test path", true},
		{"usr directory", "/usr", "test path", true},
		{"usr subdirectory", "/usr/local/bin", "test path", true},
		{"var directory", "/var", "test path", true},
		{"proc directory", "/proc", "test path", true},
		{"sys directory", "/sys", "test path", true},
		{"dev directory", "/dev", "test path", true},
		{"boot directory", "/boot", "test path", true},
		{"lib directory", "/lib", "test path", true},
		{"lib64 directory", "/lib64", "test path", true},
		{"sbin directory", "/sbin", "test path", true},
		{"root home", "/root", "test path", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNotSystemDirectory(tt.path, tt.pathType)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateNotSystemDirectory(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

// Helper functions

type mockTool struct{}

func (m *mockTool) Name() string { return "mock" }
func (m *mockTool) Description() string { return "Mock tool" }
func (m *mockTool) InputSchema() map[string]interface{} { return nil }
func (m *mockTool) Execute(input map[string]interface{}) (string, error) { return "", nil }
func (m *mockTool) NeedsInitialApproval(req *runtime.ToolRequest, approvalPolicy runtime.ApprovalPolicy, sandboxPolicy runtime.SandboxPolicy) bool { return false }
func (m *mockTool) NeedsRetryApproval(approvalPolicy runtime.ApprovalPolicy) bool { return false }
func (m *mockTool) SandboxPreference() runtime.SandboxPreference { return runtime.SandboxAuto }

func createTempFile(t *testing.T, dir string) string {
	t.Helper()
	f, err := os.CreateTemp(dir, "test-file-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer f.Close()
	return f.Name()
}
