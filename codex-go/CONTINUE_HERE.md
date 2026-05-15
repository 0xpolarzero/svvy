# 🚀 Codex Go Rewrite - Start Here Tomorrow

**Location:** `/Users/williamcory/codex/codex-go/`  
**Status:** Phase 1 Complete ✅ (7/8 agents successful, 55% test coverage)  
**Next Action:** Deploy 8 parallel agents for Phase 2-3

---

## Quick Context

**Mission:** Full Go rewrite of Codex (replacing Rust) using parallel TDD agents

**Why:** Single ecosystem, simpler maintenance, faster long-term velocity

**Phase 1 Done:**
- ✅ protocol/ - Op/Event types (34.9% coverage)
- ✅ config/ - TOML + env vars (74.7% coverage)  
- ✅ errors/ - Error types (56.0% coverage)
- ✅ client/ - Interfaces defined
- ✅ tokencount/ - Token counting (100% passing)
- ✅ tools/runtime/ - Interfaces defined
- ✅ test/ - Full infrastructure
- ✅ CI/CD, Makefile, linters

**Read First:** `PROGRESS_SUMMARY.md` in this directory

---

## Next: Deploy 8 Parallel Agents for Phase 2-3

###IMPORTANT: Launch all 8 in ONE message using Task tool 8 times

### Agent 1: internal/client/openai/
**Implement:** OpenAI API client with SSE streaming  
**Files:** openai.go, stream.go, retry.go, ratelimit.go, tests  
**Interface:** Already in `internal/client/client.go`  
**Key:** SSE parsing, retry (429/500/503), rate limits, token usage  
**Tests:** Use test.NewHTTPMockServer()  
**Ref:** codex-rs/codex-core/src/client.rs

### Agent 2: internal/conversation/manager/
**Implement:** Session & conversation management  
**Files:** manager.go, session.go, turn.go, state.go, tests  
**Key:** Session lifecycle, turn submission, approval workflow  
**Tests:** State transitions, mock client  
**Ref:** codex-rs/codex-core/src/codex.rs

### Agent 3: internal/conversation/state/
**Implement:** Conversation state tracking  
**Files:** state.go, context.go, message.go, tests  
**Key:** Turn context, policies, tool call tracking  
**Tests:** State updates, context management  
**Ref:** codex-rs/codex-core/src/codex.rs (state fields)

### Agent 4: internal/history/persistence/
**Implement:** History I/O (JSONL format)  
**Files:** persistence.go, rollout.go, format.go, tests  
**Key:** ~/.codex/sessions/{id}/history.jsonl, rollouts, resume  
**Tests:** Use test.NewMemFS() for mocking  
**Ref:** codex-rs/codex-core/src/history.rs

### Agent 5: internal/history/compaction/
**Implement:** Message compaction & summarization  
**Files:** compaction.go, truncation.go, summarize.go, tests  
**Key:** Token limits, sliding window, preserve system messages  
**Tests:** Truncation strategies, token integration  
**Ref:** codex-rs/codex-core/src/conversation/compaction.rs

### Agent 6: internal/tools/orchestrator/
**Implement:** Tool execution coordinator  
**Files:** orchestrator.go, approval.go, sandbox_selector.go, tests  
**Key:** Route tools, approval flow, sandbox escalation, parallel exec  
**Tests:** Approval caching, mock tools  
**Ref:** codex-rs/codex-core/src/tools/runtime.rs

### Agent 7: internal/tools/shell/
**Implement:** Shell command execution  
**Files:** shell.go, exec.go, output.go, tests  
**Key:** exec.Command, streaming output, exit codes, timeouts  
**Implements:** tools.ToolRuntime interface  
**Tests:** Use test.CommandMocker()  
**Ref:** codex-rs/codex-core/src/tools/shell.rs

### Agent 8: internal/tools/file/
**Implement:** File operations (read/write/list/grep)  
**Files:** read.go, write.go, list.go, grep.go, tests  
**Key:** File I/O, directory listing, grep, sandboxing  
**Implements:** tools.ToolRuntime (one per operation)  
**Tests:** Use afero.Fs mocking  
**Ref:** codex-rs/codex-core/src/tools/file.rs

---

## Task Tool Template

For each agent, use this structure:

```
subagent_type: "general-purpose"
description: "Implement [package name]"
prompt: "You are Agent [N] implementing [package].

**Your Task:** Implement internal/[path]/ with TDD.

**Context:**
- Location: /Users/williamcory/codex/codex-go/
- Interfaces: [list any existing interfaces to implement]
- Test helpers: test/testhelpers.go
- Rust reference: /Users/williamcory/codex/codex-rs/[path]

**Deliverables:**
1. [file1.go] - [description]
2. [file2.go] - [description]
3. [tests.go] - Comprehensive tests

**Requirements:**
- [Key requirement 1]
- [Key requirement 2]
- Use TDD: write tests first
- [Testing approach]

Return: Summary of implementation, test coverage, design decisions."
```

---

## Testing Commands

```bash
cd /Users/williamcory/codex/codex-go

# Run all tests
make test

# Run specific package
go test ./internal/[package]/... -v -cover

# Update golden files
make golden-update

# Generate mocks
make generate-mocks

# Lint
make lint

# Help
make help
```

---

## Key Files to Reference

- **Progress:** `PROGRESS_SUMMARY.md` (detailed status)
- **Test helpers:** `test/testhelpers.go` (525 lines of utilities)
- **Protocol:** `internal/protocol/README.md` (full docs)
- **Client interface:** `internal/client/client.go` (already defined)
- **Tool runtime:** `internal/tools/runtime/` (interfaces defined)
- **Rust source:** `/Users/williamcory/codex/codex-rs/` (behavior reference)

---

## Success Criteria for Phase 2-3

**Target:**
- All 8 packages implemented
- 50%+ test coverage per package
- All tests passing
- Rust behavior compatibility
- Clean lint (make lint passes)

**Timeline:** Week 2-3 (parallel execution ~30-45 min)

---

## After Phase 2-3

Next phases:
- Phase 4: Remaining tools (patch, sandbox)
- Phase 5: MCP integration  
- Phase 6: SDK public API
- Phase 7-8: Bubble Tea TUI
- Phase 9: Integration tests, performance

**6-week total timeline remains on track**

---

*Generated: 2025-10-25*  
*Phase: 1→2 transition*  
*Ready for parallel deployment*
