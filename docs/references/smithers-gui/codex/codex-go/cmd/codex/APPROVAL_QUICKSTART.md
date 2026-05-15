# Approval System Quick Start Guide

## TL;DR

**The CLI now requires approval before executing commands. This is a SECURITY feature.**

```bash
# Default: Safe, prompts for approval
./codex -m "your message"

# Skip prompts for read operations only
./codex -m "your message" --approval-policy=semi-auto

# DANGEROUS: Skip all prompts (use only in sandboxed environments)
./codex -m "your message" --auto-approve
```

## Quick Examples

### Safe Exploration (Recommended)
```bash
# Analyze code - will prompt before any operations
./codex -m "analyze this codebase for security issues"
```

### Balanced Approach
```bash
# Auto-approve reads, prompt for writes
./codex -m "find all TODO comments and create a summary" --approval-policy=semi-auto
```

### Automated Scripts (Use Carefully)
```bash
# For CI/CD or automated workflows only
./codex -m "run tests and generate report" --auto-approve
```

## Approval Prompt

When prompted, you'll see:

```
================================================================================
⚠️  APPROVAL REQUIRED
================================================================================
Tool: shell
Command: rm old_file.txt
Working Directory: /home/user/project

Options:
  [y] Approve this operation
  [a] Approve this and all similar operations for this session
  [n] Deny this operation
  [q] Abort the entire task
--------------------------------------------------------------------------------

Your choice [y/a/n/q]:
```

## Quick Decision Guide

**Choose [y]** - Approve just this operation
- When: You trust this specific command
- Safe: Yes, only affects this one operation

**Choose [a]** - Approve this and similar operations
- When: AI needs to run same command multiple times
- Safe: Use carefully, caches approval for session

**Choose [n]** - Deny this operation
- When: Command looks suspicious or unnecessary
- Safe: Yes, AI will try alternative approach

**Choose [q]** - Abort everything
- When: AI is doing something completely unexpected
- Safe: Yes, stops the entire task immediately

## Troubleshooting

### "I don't want to approve every single operation"

Use semi-auto mode:
```bash
./codex -m "your message" --approval-policy=semi-auto
```

### "My script is hanging waiting for input"

Add `--auto-approve` flag:
```bash
./codex -m "automated task" --auto-approve
```
⚠️ Only use in trusted/sandboxed environments!

### "I want to see what the AI does without executing"

Use never mode:
```bash
./codex -m "your message" --approval-policy=never
```
AI will provide advice but won't execute anything.

## Security Tips

✅ **DO**:
- Use default (manual) for production
- Read each approval prompt carefully
- Deny suspicious operations
- Use semi-auto for development

❌ **DON'T**:
- Use --auto-approve on production systems
- Blindly approve [a] without understanding
- Ignore risk warnings in prompts
- Use auto-approve with untrusted prompts

## Need More Info?

See comprehensive documentation: `APPROVAL_SECURITY.md`

## Getting Help

```bash
# Show all available flags
./codex --help

# Test different approval policies safely
./codex -m "echo 'hello world'" --approval-policy=manual
./codex -m "echo 'hello world'" --approval-policy=semi-auto
./codex -m "echo 'hello world'" --auto-approve
```
