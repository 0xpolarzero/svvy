// Package shell provides the shell command execution tool runtime.
//
// The shell tool enables execution of shell commands with support for:
//   - Command approval workflows
//   - Sandbox isolation via orchestrator (seatbelt/landlock/seccomp)
//   - Output streaming
//   - Escalated permissions for trusted operations
//   - Automatic retry without sandbox on permission denial
//   - Timeout enforcement from tool arguments
//   - Working directory validation with path traversal prevention
//   - Environment variable validation and filtering
//   - Command sanitization to prevent injection attacks
//
// Security Model:
//
// The shell tool implements multiple layers of security:
//
// 1. Sandbox Orchestration:
//   - Sandbox policy is determined by the ExecutionContext.SandboxAttempt
//   - The orchestrator (not the tool) controls sandbox application
//   - No bypass paths - all commands go through the sandbox layer
//   - Supports read-only, workspace-write, and full-access modes
//
// 2. Working Directory Validation:
//   - Validates directory existence before execution
//   - Prevents path traversal attacks (../ sequences)
//   - Enforces workspace boundaries when sandboxed
//   - Rejects paths pointing to files instead of directories
//
// 3. Environment Variable Security:
//   - Filters out credential-containing variables (API keys, tokens, secrets)
//   - Blocks dangerous variables that could bypass sandbox (LD_PRELOAD, etc.)
//   - Validates PATH for relative paths and world-writable directories
//   - Prevents library/module injection attacks
//
// 4. Command Sanitization:
//   - Removes null bytes and control characters
//   - Preserves newlines and tabs for multi-line commands
//   - Prevents command injection via control character sequences
//
// 5. Timeout Enforcement:
//   - Honors timeout from shellArgs.Timeout (milliseconds)
//   - Falls back to ToolRequest.Timeout if not specified
//   - Properly cancels long-running commands
//   - Returns timeout errors with proper error kind
//
// Usage Example:
//
//	tool := shell.NewShellTool()
//	req := &runtime.ToolRequest{
//	    CallID: "cmd-123",
//	    ToolName: "shell",
//	    Arguments: `{"command": "ls -la", "timeout": 5000}`,
//	    WorkingDirectory: "/workspace",
//	}
//	execCtx := &runtime.ExecutionContext{
//	    SandboxAttempt: &runtime.SandboxAttempt{
//	        Policy: runtime.SandboxWorkspaceWrite,
//	        WorkingDirectory: "/workspace",
//	    },
//	}
//	resp, err := tool.Execute(ctx, req, execCtx)
//
// All security validations are applied automatically. The tool will reject:
//   - Commands with dangerous environment variables
//   - Working directories outside workspace bounds (when sandboxed)
//   - Paths with traversal attempts
//   - Commands that timeout
//   - Invalid or malicious command strings
package shell
