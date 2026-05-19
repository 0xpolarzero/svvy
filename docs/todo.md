# Deferred Follow-Ups

This file tracks audit-review decisions that we intentionally parked for later so they are not lost while the current audit discussion continues.

It is not a general backlog and it does not include source-only audit addendum notes that we have not discussed yet.

## Later After The Audit

### AUD-001 - Host Execution Sandboxing

Status: deferred.

Decision: no sandboxing for now. Treat `execute_typescript` and bash as trusted local coding-agent capabilities during the current product pass.

Later follow-up: design one shared host-execution confinement model that covers both `execute_typescript` and bash if/when `svvy` needs to treat agent code or shell commands as untrusted.

Reference: `docs/codebase-audit-issue-research.md`, AUD-001.

### AUD-019 - Ambient Agent Resources

Status: adopted for later implementation.

Decision: do not remove all native host resources outright. Behavior-changing ambient resources should be disabled by default and enabled only through explicit settings.

Later follow-up: implement the provider-neutral Ambient Agent Resources model from `docs/specs/ambient-agent-resources-baseline.spec.md`, including categories such as extensions/packages, skills, prompt templates, commands, hooks, UI resources, provider/model adapters, credentials, and execution-policy resources.

Reference: `docs/codebase-audit-issue-research.md`, AUD-019.

### Smithers-Native Follow-Ups From AUD-011

Status: parked for the relevant later Smithers issues, not treated as AUD-011 blockers.

Decision context: these were discovered while fixing/reviewing AUD-011. We chose not to expand AUD-011 into a broader Smithers-native refactor and kept moving through the audit.

Later follow-ups:

- `smithers.resolve_approval` should be checked against native Smithers approval semantics and payloads. Target issue: AUD-033 unless it proves more severe.
- `smithers.watch_run` should be checked for custom polling drift. Target issues: AUD-026 and AUD-033.
- `smithers.get_node_detail`, `smithers.list_artifacts`, transcript, event, and related inspection tools should preserve native Smithers read-model fields unless product docs explicitly choose a smaller projection. Target issue: AUD-033.
- Workflow task-attempt records should bind to exact Smithers run, node, iteration, and attempt identity rather than recency-style lookup. Target issues: AUD-020 and AUD-033.
- `smithers.streamDevTools` should be checked against Smithers' intended streaming behavior. Target issues: AUD-026 and AUD-033.

Reference: `docs/codebase-audit-issue-research.md`, "Smithers-Native Follow-Ups From AUD-011 Exploration".

## Not Decided Follow-Ups

The "Source-Only Highlights Requiring Follow-Up" section in `docs/codebase-audit-issue-research.md` is not copied here because we have not reviewed those items one by one. They are preserved in the audit document as untriaged source-audit input, not as accepted follow-up work.
