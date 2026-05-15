# Tool Approval Security Documentation

## Overview

The Codex CLI has been updated to include a comprehensive approval workflow system that protects users from potentially dangerous AI-initiated operations. This document explains the security model, usage, and best practices.

## Security Context

**CRITICAL**: Previous versions of this application auto-approved ALL tool executions without user consent. This represented a CVSS 9.1 severity vulnerability that could allow:
- Execution of arbitrary shell commands
- Modification or deletion of files
- Network requests to external resources
- System configuration changes
- Data exfiltration

The approval system implemented here addresses this vulnerability by requiring explicit user consent for operations.

## Approval Policies

The CLI now supports four approval policies, selectable via command-line flags:

### 1. Manual (Default - Recommended)

**Flag**: `--approval-policy=manual` or no flag (default)

**Behavior**: Prompts for approval before EVERY tool execution.

**Security Level**: HIGHEST

**Use Cases**:
- Production environments
- Working with sensitive data
- Untrusted or new AI prompts
- Learning and understanding what the AI does

**Example**:
```bash
./codex -m "list files in /etc" --approval-policy=manual
```

### 2. Semi-Auto

**Flag**: `--approval-policy=semi-auto`

**Behavior**:
- Auto-approves known-safe read-only operations (ls, cat, grep, etc.)
- Prompts for approval on write operations, deletions, network access
- Prompts for approval on retry attempts after sandbox failures

**Security Level**: HIGH

**Use Cases**:
- Development environments with limited write operations
- When you trust read operations but want control over modifications
- Balancing security with convenience

**Example**:
```bash
./codex -m "analyze the codebase and suggest improvements" --approval-policy=semi-auto
```

### 3. Auto (DANGEROUS)

**Flag**: `--approval-policy=auto` or `--auto-approve`

**Behavior**: Auto-approves ALL operations without prompting.

**Security Level**: NONE

**Warning**: Using this mode is equivalent to giving the AI unrestricted access to your system.

**Use Cases** (ONLY):
- Sandboxed/isolated environments (Docker, VMs)
- Automated testing/CI environments
- When you FULLY TRUST both the AI model and all prompts
- Educational demonstrations in safe environments

**Example**:
```bash
./codex -m "set up a new project" --auto-approve
# A security warning will be displayed and you must press Enter to continue
```

### 4. Never

**Flag**: `--approval-policy=never`

**Behavior**: Denies ALL tool executions (read-only AI conversations only).

**Security Level**: MAXIMUM (but limited functionality)

**Use Cases**:
- When you only want AI advice/suggestions without any execution
- Untrusted environments where no system access should be granted
- Policy enforcement in restricted environments

**Example**:
```bash
./codex -m "explain how to set up authentication" --approval-policy=never
```

## Interactive Approval Prompts

When using `manual` or `semi-auto` policies, you'll see detailed approval prompts:

```
================================================================================
⚠️  APPROVAL REQUIRED
================================================================================
Tool: shell
Command: rm -rf /tmp/old_files
Working Directory: /home/user/project
Justification: Cleaning up temporary files from previous build

Risk Level: HIGH
Risk Reasons:
  • Recursive deletion operation
  • Multiple files may be affected
Mitigation: Operation limited to /tmp directory

--------------------------------------------------------------------------------
Options:
  [y] Approve this operation
  [a] Approve this and all similar operations for this session
  [n] Deny this operation
  [q] Abort the entire task
--------------------------------------------------------------------------------

Your choice [y/a/n/q]:
```

### Approval Options Explained

- **[y] Yes**: Approve this specific operation only. The next operation will require approval again.
- **[a] Always**: Approve this operation AND cache the approval for similar operations in this session. Useful when the AI needs to run the same command multiple times.
- **[n] No**: Deny this operation. The AI will be informed and may try alternative approaches.
- **[q] Quit**: Abort the entire task immediately. Use this if the AI is attempting something unexpected.

## Risk Assessment

The approval system includes intelligent risk assessment:

### Risk Levels

- **LOW**: Read-only operations with minimal impact (ls, cat, pwd)
- **MEDIUM**: Write operations within workspace (echo, touch, mkdir)
- **HIGH**: System modifications, network access, operations outside workspace
- **CRITICAL**: Destructive operations (rm -rf, chmod, system configuration changes)

### Risk Factors Considered

The system analyzes:
- Command type and known dangerous patterns
- Target paths (system directories, home directory, workspace)
- Destructive flags (-r, -f, --force, --delete)
- Network operations
- Privilege escalation attempts
- Shell operators (pipes, redirects, command chaining)

## Security Best Practices

### 1. Default to Manual Approval

Unless you have a specific reason to use a more permissive policy, always use `manual` (the default).

### 2. Review Each Approval Request Carefully

When prompted for approval:
- Read the command carefully
- Verify the working directory
- Check the risk assessment
- Consider if the operation aligns with your request
- When in doubt, choose [n] to deny

### 3. Be Cautious with "Always" Approval

The [a] option caches approval for similar operations. While convenient, this can be dangerous if:
- The AI changes its approach mid-task
- The command pattern matches something you didn't intend
- You forget you've cached approval

### 4. Never Use Auto-Approve in Production

The `--auto-approve` flag should NEVER be used:
- On production systems
- With sensitive data
- With untrusted prompts
- On your main development machine

### 5. Use Sandboxing When Available

The application supports sandbox policies. Always prefer sandboxed execution:
```bash
# Future: sandbox support will be enhanced
export SANDBOX_POLICY=workspace-write
./codex -m "build the project"
```

### 6. Monitor Auto-Approved Operations

When using `auto` policy, the system logs every auto-approved operation to stderr:
```
[AUTO-APPROVED] shell: ls -la
[AUTO-APPROVED] read_file
[AUTO-APPROVED] shell: npm install
```

Review these logs to understand what the AI is doing.

### 7. Session Isolation

Each session maintains its own approval cache. Use separate sessions for different tasks:
```bash
# Session for safe exploration
./codex -m "analyze code" -s explore --approval-policy=semi-auto

# Session for modifications (more careful)
./codex -m "fix bugs" -s bugfix --approval-policy=manual
```

## Audit Trail

All approval decisions are logged:
- Approval requests with full details
- User decisions (approved, denied, aborted)
- Risk assessments
- Commands executed

This audit trail is essential for:
- Understanding what the AI attempted
- Security reviews
- Compliance requirements
- Debugging unexpected behavior

## Migration from Auto-Approve

If you were using a version with auto-approve by default:

### What Changed

- **Old behavior**: ALL operations auto-approved silently
- **New behavior**: Manual approval required by default

### How to Migrate

1. **No changes needed for security**: The new default is secure
2. **If you want old behavior** (NOT recommended):
   ```bash
   ./codex -m "your message" --auto-approve
   ```
3. **If you want balanced approach**: Use semi-auto
   ```bash
   ./codex -m "your message" --approval-policy=semi-auto
   ```

### Breaking Changes

- Scripts that relied on auto-approval will now prompt for user input
- Automated workflows need explicit `--auto-approve` flag
- CI/CD pipelines should use `--approval-policy=auto` with appropriate warnings

## Implementation Details

### Approval Handler

The CLI uses a custom approval handler that:
1. Receives approval requests from the tool orchestrator
2. Displays formatted prompts on stderr (preserving stdout for AI output)
3. Reads user responses from stdin
4. Handles context cancellation and timeouts
5. Caches approved operations per session

### Integration with Orchestrator

The approval system integrates with the tool orchestrator's approval workflow:
- Initial approval before first execution
- Retry approval after sandbox failures
- Cached approval lookup for similar operations
- Risk assessment for informed decisions

### Code Location

Key files:
- `/cmd/codex/main.go`: CLI approval handler implementation
- `/internal/tools/orchestrator/approval.go`: Core approval logic
- `/internal/conversation/manager/approval_handler.go`: Session-based approval
- `/internal/tools/runtime/types.go`: Approval types and interfaces

## Testing

### Manual Testing

Test each approval policy:

```bash
# Test manual approval
./codex -m "create a test file" --approval-policy=manual
# Should prompt for approval

# Test semi-auto
./codex -m "list files and create test.txt" --approval-policy=semi-auto
# Should auto-approve ls, prompt for file creation

# Test auto (with warning)
./codex -m "create files" --auto-approve
# Should show warning, then auto-approve all

# Test never
./codex -m "create files" --approval-policy=never
# Should deny all operations
```

### Automated Testing

See `/cmd/codex/main_test.go` for unit tests of approval policy logic.

## FAQ

### Q: Why do I need to approve read operations?

A: Even read operations can be sensitive:
- Reading `/etc/shadow` or other system files
- Accessing private keys or credentials
- Reading large files that could consume resources
- Information disclosure for malicious prompts

### Q: Can I configure default policy via environment variable?

A: Not currently. This is intentional - approval policy should be an explicit choice per invocation. Future versions may add support for configuration files.

### Q: What happens if I press Ctrl+C during approval?

A: The operation is denied and the context is cancelled, which stops the AI task.

### Q: How do cached approvals work?

A: When you choose [a], the approval is cached based on:
- Tool name
- Command pattern
- Working directory

Similar operations in the same session will be auto-approved without prompting.

### Q: Can the AI bypass the approval system?

A: No. All tool executions go through the orchestrator, which enforces the approval policy. The AI cannot directly execute commands.

## Reporting Security Issues

If you discover a security vulnerability in the approval system:

1. **DO NOT** create a public issue
2. Contact the maintainers privately
3. Provide detailed reproduction steps
4. Allow time for a fix before disclosure

## Version History

- **v1.0.0** (2025-10-26): Initial implementation with manual, semi-auto, auto, and never policies
- **Previous versions**: Auto-approved all operations (INSECURE)

## Future Enhancements

Planned improvements:
- Configuration file support for default policies
- More granular approval rules (per-tool, per-path)
- Integration with system keychains for approval persistence
- Better sandbox integration
- Approval policy enforcement at system level
- Audit log export for compliance

## References

- CVSS 9.1 vulnerability details: See `/cmd/codex/main.go.md`
- Tool orchestrator documentation: See `/internal/tools/orchestrator/README.md`
- Runtime approval types: See `/internal/tools/runtime/types.go`
