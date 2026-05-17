# Codebase Audit Issue Research

Date: 2026-05-17

This document consolidates the final audit messages from these conversations:

- `019e3098-209c-7072-b5c5-9c148f205941`
- `019e3098-3eed-70e2-ad4c-e79eb88eda74`
- `019e3098-34a6-7570-90ac-1bf590b6769c`
- `019e3098-2a4e-7712-b253-db862219e711`
- `019e3098-1291-7dc3-81e3-afda539ceb71`

The purpose is to turn each extractable issue into an evidence-backed implementation plan. Every issue below must be grounded in local code, product docs, and the relevant pi or Smithers reference material before implementation work starts.

## Method

For each issue, record:

- **Audit source**: Which audit conversations raised it.
- **Current contract**: The exact product or external-reference behavior the code should satisfy.
- **Confirmed code path**: Files, functions, and control flow that currently implement or violate the contract.
- **Precise issue**: The smallest accurate statement of what is wrong.
- **Impact**: User-visible, reliability, security, performance, or product-doc consequence.
- **Best fix**: The cleanest, most reliable, optimized fix after researching the actual code.
- **Verification**: Unit, e2e, manual, or static checks needed to prove the fix.
- **Documentation updates**: PRD, features, progress, or specs updates required if behavior changes.

## Issue Index

| ID | Impact | Issue | Audit sources | Research status |
|---|---:|---|---|---|
| AUD-001 | Later | Host execution for `execute_typescript` and bash is not sandboxed | `3eed`, `2a4e`, `34a6` | Deferred |
| AUD-002 | P1 | `execute_typescript` exposes workflow APIs across actor surfaces | `3eed`, `2a4e`, `34a6`, `1291` | Researched |
| AUD-003 | P1 | Local browser-tools bridge exposes sensitive runtime state | all five | Researched |
| AUD-004 | P1 | Artifact file attachment and preview can escape workspace/artifact roots | `3eed`, `209c`, `1291` | Researched |
| AUD-005 | P1 | Raw command, log, and artifact persistence can retain secrets without a unified redaction policy | `3eed`, `34a6`, `1291` | Researched |
| AUD-006 | P1 | HTML artifact previews use `srcdoc` without an iframe sandbox | `34a6` | Researched |
| AUD-007 | P1 | Duplicate same-cwd workspace tabs share durable session state while owning separate live runtimes | `209c`, `3eed`, `2a4e`, `34a6` | Researched |
| AUD-008 | P1 | Workspace-scoped RPC handlers still route through active runtime state | `209c`, `2a4e`, `1291` | Researched |
| AUD-009 | P1 | Nonterminal Smithers workflow runs are not reliably reattached after restart | `209c`, `3eed`, `2a4e` | Researched |
| AUD-010 | P1 | `smithers.run_workflow` can implicitly resume the wrong run when `runId` is omitted | `3eed`, `34a6`, `2a4e` | Researched |
| AUD-011 | P1 | Smithers cancellation may leave inactive waiting runs stuck | `1291` | Researched |
| AUD-012 | P1 | Project CI projection depends on in-memory terminal output and is not restart-safe | `1291` | Researched |
| AUD-013 | P1 | Queued message drain can double-dispatch or strand `dispatching` rows after restart | `209c`, `34a6`, `2a4e`, `1291` | Researched |
| AUD-014 | P2 | Queued-message reorder can spam durable writes during drag movement | `2a4e` | Researched |
| AUD-015 | P1 | Handler `thread.handoff` reconciliation can be dropped while the orchestrator is active | `34a6`, `2a4e` | Researched |
| AUD-016 | P1 | Initial handler auto-start handoff can fail to wake the orchestrator | `2a4e` | Researched |
| AUD-017 | P1 | Prompt freshness is detected but not enforced before the next pi turn | `209c` | Researched |
| AUD-018 | P1 | Session mode changes and new-session creation can use raw or double-wrapped system prompts | `1291`, `3eed` | Researched |
| AUD-019 | P1 | Orchestrator and handler surfaces can inherit ambient pi extension tools beyond the actor contract | `34a6` | Researched |
| AUD-020 | P1 | Workflow task-agent authoring contract, generated agent config, and runtime tool surface can diverge | `209c`, `2a4e`, `1291` | Researched |
| AUD-021 | P2 | Workflow task agents and `request_context` do not fully match prompt/context binding semantics | `2a4e` | Researched |
| AUD-022 | P1 | Dockview chat panels miss semantic transcript blocks and artifact/path callbacks | `34a6`, `2a4e`, `1291` | Researched |
| AUD-023 | P2 | Artifact/static inspector panes remain focus-global instead of surface-owned | `3eed`, `209c` | Researched |
| AUD-024 | P1 | Restart recovery can leave in-flight prompts, pending messages, running turns, or initial handler starts stale | `2a4e`, `34a6`, `209c` | Researched |
| AUD-025 | P1 | Streaming assistant deltas emit full surface snapshots and reparse too much Markdown | all five | Researched |
| AUD-026 | P1 | Workflow inspector live mode polls and rebuilds too much state | `34a6`, `2a4e`, `3eed`, `1291` | Researched |
| AUD-027 | P1 | Structured-state selectors materialize large snapshots with insufficient indexes/read models | `2a4e`, `34a6`, `209c`, `3eed` | Researched |
| AUD-028 | P2 | Native command-palette shortcut payload shape breaks app-menu accelerators | `34a6`, `209c` | Researched |
| AUD-029 | P2 | Command palette results and routes are incomplete for handler/task/Logs/Context/Project CI flows | `34a6`, `209c`, `1291` | Researched |
| AUD-030 | P1 | Destructive Delete Session conflicts with archive-as-hide product semantics | `1291`, `2a4e` | Researched |
| AUD-031 | P2 | E2E and proof tests encode retries, live-provider dependencies, brittle selectors, or missing restart paths | all five | Researched |
| AUD-032 | P2 | `docs/progress.md` and feature status tracking drift from shipped implementation | all five | Researched |
| AUD-033 | P2 | Smithers approval, task-attempt, and artifact shapes may drift from native Smithers contracts | `34a6`, `209c` | Researched |
| AUD-034 | P2 | Minor UI and observability edge cases need confirmation: layout slot hydration, inspector split targets, log related links, double-click rename, private URL guards, provider secret storage | `209c`, `2a4e`, `34a6` | Researched; split into AUD-034A-F |

## Cross-Check Addendum

After compaction, the source audit transcripts and research subagent final messages were re-extracted from local Codex JSONL logs and compared against this document. The researched AUD-001 through AUD-034 sections cover every issue that had a dedicated research subagent output. The original five audit finals also contained meta guidance, scoring, and several source-only highlights that were not individually researched in the second pass. Those are preserved here so they are not lost.

### Aggregate Audit Scores

The five source audits used slightly different scoring scales, but the combined signal was consistent:

- **Architecture consistency:** mostly 5/10 to 7/10. The pi-backed orchestrator/handler/Smithers shape is visible, but workspace identity, actor capability boundaries, prompt lifecycle, and Smithers resume semantics drift.
- **Reliability/resume:** mostly 4/10 to 5/10. Durable state exists, but queue claims, active prompt recovery, initial handler starts, Smithers reattach/cancel, Project CI projection, and restart paths are the main weakness.
- **Test quality:** mostly 5/10 to 6/10. Unit coverage is broad, but e2e/proof tests include retries, force clicks, live-provider gates, stale delete assertions, and missing restart/resume paths.
- **UX/product clarity:** mostly 5/10 to 7/10. Dockview, logs, command palette, and inspectors are substantial, but missing semantic transcript cards, incomplete palette routes, artifact ownership, and related-link gaps can confuse users.
- **Performance risk:** mostly 6/10 to 8/10 risk. The repeated concerns were full snapshot streaming, Markdown reparsing, workflow inspector polling/rebuilds, broad SQLite selectors, app-log aggregation, path-index churn, registry refresh churn, and sync scans.
- **Security/privacy risk:** mostly 7/10 to 8/10 risk, with one audit scoring 4/10 but still listing high-leverage risks. Repeated concerns were bridge exposure, unsandboxed `execute_typescript`, arbitrary artifact file reads, HTML preview sandboxing, provider secrets, private URL validation, and redaction gaps.

### Do Not Change Casually

The source audits repeatedly called out these intentional product directions:

- Keep pi as the interactive runtime/session substrate. Do not introduce a standalone shell, alternate TUI loop, or svvy-owned terminal runtime.
- Keep the orchestrator/handler/task-agent actor split. Fix leaks rather than flattening all actors into one global tool surface.
- Keep Smithers as the workflow execution/runtime surface. Do not replace Smithers-native control with a parallel `workflow.*` control abstraction.
- Keep repo-root `workflows/` as an authoring workspace, not the shipped runtime or registry.
- Keep structured session state as the product read model. The issue is scoping, recovery, and read-model efficiency, not the existence of structured state.
- Keep Dockview as the renderer workbench/layout engine. The fix direction is stronger surface ownership and restoration, not a return to bespoke pane state.
- Keep workflow-task agents using task-local direct tools with extension gating. The issue is aligning contracts and runtime behavior, not removing the separate task-agent layer.

### Product Decisions And Open Questions Preserved From Source Audits

Resolved product decisions:

- The browser-tools bridge is dev/e2e/manual-inspection only. Shipped production builds must not mount browser-tools bridge behavior, and the dev/e2e/manual-inspection lane does not require an additional auth token.

Open product questions:

- Should duplicate same-cwd workspace tabs remain a shipped requirement now, or should the UI temporarily prevent duplicates until persistence is truly runtime-scoped?
- Is `execute_typescript` meant to be a trusted full-process escape hatch, or a constrained API-only sandbox?
- Should `execute_typescript` be available to the orchestrator at all, or only to handler/task-agent contexts?
- Should workflow task-agent `toolSurface` be product-configurable, or should the contract define one fixed surface?
- Should workflow-authored task agents expose a small factory API, or should all task-agent creation go through generated workflow components?
- Should destructive session deletion exist at all, or should archive/unarchive be the only user-visible deletion model?
- Should Project CI palette commands target the focused handler thread when one is focused, or always route through orchestrator policy?
- Should provider secrets move to OS credential storage before broader distribution?
- Should direct coding tools be whole-filesystem by policy, or should svvy add workspace confinement above pi?
- Should `smithers.resolve_approval` mirror Smithers' richer native payload exactly, or preserve a simplified svvy affordance? This conflicts with the current Smithers-native direction and should be resolved explicitly if simplified shapes are kept.

### Source-Only Highlights Requiring Follow-Up

The items below were highlighted by source audit finals or source audit subagent rosters but did not receive dedicated second-pass research sections. They should not be treated as fully proven implementation tasks until confirmed against code.

- **Unread clearing and log read-state gaps:** Mentioned in renderer/UI audit summaries. AUD-032 confirms durable unread state exists and progress docs drift, while AUD-034E covers app-log related links. A follow-up should verify unread clearing, log read-state transitions, and whether any open/read events fail to mark the correct surface/log entries seen.
- **Provider preflight and renderer-side custom key handling:** Mentioned by UI/security audit summaries. AUD-034B covers provider credential storage and precedence, but a follow-up should inspect provider readiness/preflight UI and any renderer-side handling of custom provider keys.
- **Per-workspace `.env` gap:** Mentioned by a persistence/restart audit roster. This needs confirmation against provider/env resolution and workspace-runtime scoping before becoming an implementation issue.
- **Dropped compaction events:** Mentioned by a pi/reference audit roster. AUD-024 covers restart and stale running work generally, but compaction event persistence/replay needs a dedicated check against pi integration and structured state.
- **Missing `thread.start` runtime guard:** Mentioned by an actor-surface audit roster. AUD-019 covers ambient pi extension leakage and actor tool-surface isolation. A follow-up should verify that handlers cannot dynamically invoke `thread.start` through any runtime/code-mode path.
- **Repo-root workflow registry asset leakage:** Mentioned by a Smithers audit roster. AUD-020/AUD-033 cover task-agent and Smithers contract drift, but packaged workflow discovery should be checked specifically for source-checkout-relative registry assets.
- **Panel release gaps:** Mentioned by renderer audit summaries. AUD-023 covers focus-global artifact/static inspector state, but pane close/release reference counting should be checked separately for leaked or prematurely disposed live surfaces.
- **Fabricated workflow progress:** Mentioned by a renderer audit roster. AUD-022 and AUD-026 cover transcript and workflow inspector projection, but a follow-up should verify whether any UI progress display is inferred rather than derived from durable Smithers/structured state.
- **Additional performance hotspots:** Registry refresh churn, app-log aggregation, path-index churn, log burst cost, and sync scans were called out in audit summaries. AUD-025 through AUD-027 cover the largest performance issues, but these named paths need targeted profiling or static confirmation before implementation.
- **Workflow rename state:** Mentioned in progress drift summaries. AUD-032 covers progress/docs drift, but workflow rename shipped state should be verified before ticking or rewriting progress items.

### Source Provenance And Commands

The cross-check used these local transcript files as source material:

- `/Users/polarzero/.codex/sessions/2026/05/16/rollout-2026-05-16T13-42-16-019e3098-209c-7072-b5c5-9c148f205941.jsonl`
- `/Users/polarzero/.codex/sessions/2026/05/16/rollout-2026-05-16T13-42-24-019e3098-3eed-70e2-ad4c-e79eb88eda74.jsonl`
- `/Users/polarzero/.codex/sessions/2026/05/16/rollout-2026-05-16T13-42-21-019e3098-34a6-7570-90ac-1bf590b6769c.jsonl`
- `/Users/polarzero/.codex/sessions/2026/05/16/rollout-2026-05-16T13-42-19-019e3098-2a4e-7712-b253-db862219e711.jsonl`
- `/Users/polarzero/.codex/sessions/2026/05/16/rollout-2026-05-16T13-42-13-019e3098-1291-7dc3-81e3-afda539ceb71.jsonl`
- `/Users/polarzero/.codex/sessions/2026/05/17/rollout-2026-05-17T16-27-25-019e3655-b015-7592-8632-e8c8853c1d58.jsonl`
- `/Users/polarzero/.codex/sessions/2026/05/17/rollout-2026-05-17T16-27-30-019e3655-c2da-72d2-ab49-2a07bcc90c6d.jsonl`
- `/Users/polarzero/.codex/sessions/2026/05/17/rollout-2026-05-17T16-27-35-019e3655-d5c2-7800-b8a3-f98cc0ac3443.jsonl`
- `/Users/polarzero/.codex/sessions/2026/05/17/rollout-2026-05-17T16-27-40-019e3655-e7fd-7a22-87d0-a67e50f1c14a.jsonl`
- `/Users/polarzero/.codex/sessions/2026/05/17/rollout-2026-05-17T16-27-45-019e3655-fe0c-7d52-8699-907e801d5558.jsonl`
- `/Users/polarzero/.codex/sessions/2026/05/17/rollout-2026-05-17T16-27-50-019e3656-1137-76d0-bc84-864ef2ae0510.jsonl`
- `/Users/polarzero/.codex/sessions/2026/05/17/rollout-2026-05-17T16-27-55-019e3656-2349-7600-845c-66ee21ce18e1.jsonl`
- `/Users/polarzero/.codex/sessions/2026/05/17/rollout-2026-05-17T16-28-02-019e3656-3d7d-7f50-a48e-8927a4ca8f2a.jsonl`
- `/Users/polarzero/.codex/sessions/2026/05/17/rollout-2026-05-17T16-28-06-019e3656-4f23-7e80-be81-1da554cb96fb.jsonl`
- `/Users/polarzero/.codex/sessions/2026/05/17/rollout-2026-05-17T16-28-11-019e3656-611f-74a3-be24-eb09a1d6dea7.jsonl`
- `/Users/polarzero/.codex/sessions/2026/05/17/rollout-2026-05-17T16-28-15-019e3656-7211-7913-9f28-9e8dde2d1868.jsonl`
- `/Users/polarzero/.codex/sessions/2026/05/17/rollout-2026-05-17T16-28-20-019e3656-84d0-71e2-8e74-38ed0ea74629.jsonl`
- `/Users/polarzero/.codex/sessions/2026/05/17/rollout-2026-05-17T16-28-25-019e3656-9747-7362-8c80-86f003cd92cc.jsonl`

The original audits reported mixed command outcomes: several `bun run check` runs passed, one failed in `format:check` but then passed on rerun, and some failed during `test:unit` with a workflow-supervision proof flake. The document creation pass later ran `bun run check` successfully after adding this docs file.

## Researched Issues

### AUD-001 - Host execution for `execute_typescript` and bash is not sandboxed

**Disposition:** Deferred. This is a valid host-execution hardening concern, but it is not an immediate product fix. For now, `svvy` treats both `execute_typescript` and bash-style command execution as trusted local coding-agent capabilities rather than sandboxed untrusted-code surfaces.

**Impact:** Future security and auditability concern. `execute_typescript` presents a constrained TypeScript API, but runtime execution can still reach host globals and perform operations outside the declared `api` surface. Bash commands have the same broad host-execution character by design.

**Precise issue:** `src/bun/execute-typescript-tool.ts` typechecks and transpiles snippets, then runs the emitted JavaScript through `new Function(...)`. The generated declaration and prompt say that snippets should use the injected `api` object and should not rely on Node or Bun built-ins, but that is only a type/prompt contract. It is not a runtime boundary.

The audit probe confirmed that snippet code can reach host objects:

- `Function("return process")().cwd()` returns the repository cwd.
- `Function("return globalThis.Bun")().file("docs/prd.md").exists()` can access Bun file APIs.

Relevant code:

- `src/bun/execute-typescript-tool.ts`: host-side typecheck/transpile around the execution pipeline.
- `src/bun/execute-typescript-tool.ts`: runtime execution via `new Function(...)`.
- `src/bun/default-system-prompt.ts`: prompt-only instruction that code should use the provided API.

**Why this matters later:** TypeScript checking prevents some undeclared names at compile time, but JavaScript dynamic evaluation still reaches host capabilities. If `svvy` later needs to treat agent code or shell commands as untrusted, the current model is not enough: file, process, network, or other host operations can happen without going through a narrow recorded `api.*` call surface.

**Later fix:** Design one host-execution confinement model that covers both `execute_typescript` and bash.

The clean long-term fix is a real execution boundary:

1. Keep typechecking/transpilation in the trusted Bun process.
2. Execute compiled snippets in a constrained subprocess, worker isolate, or OS sandbox with a minimal global object.
3. Expose only the allowed `api.*` capabilities through an RPC bridge from the sandbox to the host.
4. Validate every RPC call against the current actor's capability profile before performing host work.
5. Record host-side effects from that RPC layer, not from snippet-side assumptions.

Do not rely on deleting globals from the current realm, prompt text, TypeScript declarations, lexical wrappers, or command allowlist prose. Those are not a durable security boundary against dynamic evaluation or shell execution.

**Future verification required:**

- Unit tests where snippets try `Function("return process")`, `globalThis.Bun`, `eval`, dynamic imports, and constructor escapes; all must fail without host side effects.
- Tests showing allowed `api.*` calls still work and are recorded.
- Regression tests for current typecheck behavior, artifact APIs, child-command APIs, and web APIs.
- A destructive-operation guard test proving denied snippets do not spawn child commands or write files.

**Documentation impact:** No immediate PRD/spec change is needed. If sandboxing becomes a product requirement later, update the relevant execution/tool specs to describe the shared host-execution boundary for both TypeScript snippets and bash.

**Confidence:** High. The runtime escape was directly confirmed.

### AUD-002 - Actor-scoped workflow API leaks through `execute_typescript`

**Impact:** Critical capability-boundary issue. Actors that should not receive workflow discovery or Smithers control can reach workflow APIs from code mode.

**Current contract:** `execute_typescript` is actor-local. The orchestrator receives code mode for bounded local composition, but it does not receive workflow discovery, Smithers runtime control, or any `workflow` or `smithers` namespace through code mode; it delegates workflow action by calling `thread.start`. Handler threads receive the workflow and Smithers composition surface defined by the product contract: direct `workflow.*` discovery tools, handler-only `api.workflow` helpers for typed composition over workflow assets and authoring models, and direct `smithers.*` tools for runtime supervision. Workflow task agents receive only task-local direct-tool APIs through code mode and do not receive workflow discovery, Smithers supervision, handler/orchestrator control, or `api.workflow`.

**Precise issue:** The top-level tool surface is actor-scoped, but the generated `execute_typescript` SDK and runtime API are not.

Relevant code:

- `src/bun/session-catalog.ts`: direct workflow tools are withheld from the orchestrator top-level surface and added for handler surfaces.
- `src/bun/default-system-prompt.ts`: builds one execute TypeScript SDK for all actors.
- `src/bun/execute-typescript-api-contract.ts`: always includes `api.workflow`.
- `src/bun/execute-typescript-tool.ts`: always registers workflow tools inside code mode.

The audit probe confirmed an orchestrator-context `runExecuteTypescript` call can invoke `api.workflow.list_assets`.

**Why this matters:** The product contract separates orchestrator, handler, and task-agent powers. Handler agents may discover workflow assets through handler-owned workflow APIs and supervise Smithers through direct `smithers.*` tools. Orchestrators and task agents should only see the surfaces explicitly granted to them. Code mode currently bypasses that separation and turns a handler-only workflow discovery namespace into a universal API.

**Best fix:** Make `execute_typescript` actor-aware at both declaration time and runtime.

1. Define an explicit capability profile for each actor type.
2. Pass that profile into `buildExecuteTypescriptApiDeclaration`.
3. Pass the same profile into `createExecuteTypescriptTool`, `runExecuteTypescript`, and `createExecuteTypescriptApi`.
4. Only generate `api.workflow.*` declarations for handler contexts.
5. Do not generate any `api.smithers` code-mode namespace; handlers supervise Smithers through direct `smithers.*` tools.
6. Reject runtime calls whose namespace is absent from the actor capability profile, even if malicious code constructs the call dynamically.

The declaration and runtime guard must use the same source of truth. Declaration-only filtering would still be bypassable through dynamic JavaScript.

**Verification required:**

- Prompt/declaration tests proving `api.workflow` is absent for orchestrators and workflow task agents.
- Prompt/declaration tests proving `api.workflow` is present for handlers.
- Prompt/declaration tests proving no actor receives `api.smithers`.
- Runtime tests proving dynamic access to blocked namespaces fails.
- Runtime tests proving handler workflow discovery calls still work through direct tools and handler-only `api.workflow`.

**Documentation impact:** Update the execute TypeScript spec if it currently describes one universal SDK. The actor-scoped contract should be explicit.

**Confidence:** High. The top-level actor scoping and code-mode leak were both identified in source, and the orchestrator code-mode call was confirmed.

### AUD-003 - Browser-tools bridge must stay out of production

**Impact:** Critical local attack-surface and data-exposure issue if browser-tools behavior is present in production.

**Decision:** The `electrobun-browser-tools` bridge is only for development, OrbStack e2e, and manual inspection. The production app path must contain no browser-tools bridge behavior. Because the bridge is non-production-only, svvy does not require an additional dev auth token or bearer-token model for that lane.

**Precise issue:** Current code mounts the `electrobun-browser-tools` bridge on a predictable local port derived from process id. The bridge exposes read and write inspection/control methods intended for automation and manual inspection. That is acceptable only in explicitly enabled dev/e2e/manual-inspection contexts, not in shipped production behavior. The app also records or prints the resolved bridge URL, which should remain limited to the non-production inspection lane and must not be treated as normal app-log observability.

Relevant code:

- `src/bun/tool-bridge.ts`: builds bridge state including cwd, open workspaces, provider auth summaries, session summaries, surfaces, and system prompts.
- `src/bun/tool-bridge.ts`: mounts the bridge on `59000 + (process.pid % 1000)`.
- `node_modules/electrobun-browser-tools/dist/chunk-VKPXYR7I.js`: handles bridge GET/POST requests without bearer-token validation.
- `src/bun/index.ts`: records bridge events/logs including the resolved bridge URL.

**Why this matters:** If browser-tools behavior is present in production, any local process that discovers the port can inspect and drive the app. Depending on browser loopback/private-network behavior, browser-origin access may also be relevant. The bridge state is not limited to harmless liveness information; it includes sensitive operational metadata and rendered UI state. Provider keys were not confirmed to be in the bridge state, but password input values can be read by DOM tools while a user is editing them.

**Best fix:**

1. Exclude browser-tools mounting, request handlers, state capture, and URL logging from shipped production builds.
2. Keep browser-tools available only through explicit dev/e2e/manual-inspection launch paths.
3. Do not add a bearer-token or dev-auth requirement for the non-production inspection lane.
4. Keep browser-tools URL output and state capture scoped to non-production inspection workflows, not product app logs.
5. Reduce non-production bridge state to the minimum needed for inspection. Replace raw prompt text and sensitive metadata with hashes, labels, or redacted summaries where inspection does not need the raw value.
6. Mask password and token field values in DOM/state bridge output.

**Verification required:**

- Production/default launch does not mount browser-tools routes, request handlers, or state-capture behavior.
- Dev/e2e/manual-inspection launch paths can still use `electrobun-browser-tools` without additional auth.
- Product app logs do not include browser-tools URL output and do not treat browser-tools telemetry as normal production observability.
- Non-production bridge snapshots do not include raw system prompts or password/token input values unless a specific inspection task intentionally requires that raw field and documents the exposure.

**Documentation impact:** Keep the app-log spec clear that browser-tools telemetry is outside production observability. Keep `AGENTS.md` guidance for manual inspection aligned with the non-production-only bridge lane and no dev auth/token requirement.

**Confidence:** High for unauthenticated local bridge and URL logging in the current inspection path. Browser-origin reachability matters only for the non-production lane once production excludes browser-tools behavior.

### AUD-004 - Artifact paths are not contained to the workspace artifact area

**Impact:** Critical local file disclosure risk through artifact preview and artifact reads.

**Precise issue:** Artifact creation and preview paths can persist arbitrary absolute or process-relative paths. Inline artifact content is written under an artifact root, but attached path-based artifacts are stored and later read without canonical containment checks.

Relevant code:

- `src/bun/svvy-direct-tools.ts`: `artifact.create` stores `input.path` directly.
- `src/bun/structured-session-state.ts`: accepts and resolves artifact paths without enforcing an artifact root for path-only artifacts.
- `src/bun/session-catalog.ts`: reads artifact preview content from `artifact.path`.
- `src/bun/structured-session-selectors.ts`: checks existence at `artifact.path`.

**Why this matters:** A tool call can point an artifact at an arbitrary file on disk. Later preview/read paths can expose that file as if it were a generated artifact. Relative paths are also dangerous because they resolve against process cwd rather than a declared artifact root.

**Best fix:**

1. Centralize artifact path resolution in one module.
2. Define the only allowed persisted artifact path roots, preferably `.svvy/artifacts/<session-id>/...` or the existing canonical artifact store.
3. Reject absolute paths, `..` traversal, and process-relative paths for persisted artifact records.
4. Resolve and `realpath` both the candidate path and allowed root, then enforce root containment after symlink resolution.
5. For `attach_file` or path-based artifact evidence, copy the file into the managed artifact root and persist the copied path.
6. Store the original source path only as redacted provenance metadata, not as the preview/read target.
7. Make preview/read APIs refuse any stored artifact path that is outside the allowed root, even for legacy records.

**Verification required:**

- Absolute outside path is rejected.
- `../` traversal is rejected.
- Symlink escape from the artifact root is rejected.
- Valid attached files are copied into the managed artifact root and previewed from there.
- A malicious pre-existing database record pointing outside the root is refused by preview/read code.

**Documentation impact:** Update artifact/workspace-navigation specs to define managed artifact storage and source-file provenance.

**Confidence:** High.

### AUD-005 - Redaction is app-log-specific and does not cover command, tool, artifact, and web records

**Impact:** High confidentiality issue. Secrets can persist outside the app-log redaction pipeline.

**Precise issue:** The app has an app-log redactor, but command summaries, command facts, tool execution records, `execute_typescript` snippets/logs/errors, artifact content, and web fetch outputs are not consistently passed through a shared sensitive-data policy before persistence or display.

Relevant code:

- `src/bun/app-log-store.ts`: app-log redaction policy.
- `src/bun/structured-session-state.ts`: persists command summaries, facts, errors, and related records.
- `src/bun/tool-execution-command-tracker.ts`: stores raw generic tool args/results.
- `src/bun/execute-typescript-tool.ts`: records snippets, logs, errors, and facts.
- `src/bun/svvy-direct-tools.ts`: writes artifact text/json content.
- `src/bun/web-runtime/providers/shared.ts`: writes web bodies/metadata without the app-log redactor.

**Why this matters:** Users can paste API keys, bearer tokens, provider responses, auth headers, or private data into commands and tool calls. Those values can land in SQLite, artifact files, inspectors, previews, exports, or bridge state even if app logs are redacted.

**Best fix:**

1. Extract a shared sensitive-data redaction module used by every persistence/display path that records tool input, tool output, command facts, command summaries, errors, web metadata, and automatic execution artifacts.
2. Apply redaction before writing to SQLite and before emitting renderer-facing snapshots.
3. Keep an explicit distinction between user-authored artifact content and automatically captured execution evidence. User-authored artifacts may need exact bytes; automatic evidence should default to redacted previews.
4. Redact common secret shapes, provider keys, auth headers, bearer tokens, cookies, private URLs with embedded credentials, and environment-variable style secrets.
5. Make path redaction conservative: preserve useful file names/cwd context where safe, but redact secret-like path segments and credentials.

**Verification required:**

- Inject fake secrets into bash commands, tool args, tool output, command facts, `execute_typescript` console logs, thrown errors, artifact writes, and web metadata.
- Assert persisted snapshots and renderer projections redact those secrets.
- Assert intentional user-authored artifact content behavior is documented and tested.
- Assert redaction does not corrupt non-secret structured fields required by inspectors.

**Documentation impact:** Update structured-state, workspace-navigation, and app-log specs to say where redaction applies and where exact user-authored artifact bytes are intentionally preserved.

**Confidence:** Medium-high. Coverage gaps are clear in code; exact artifact raw-content policy needs product decision.

### AUD-006 - Visible HTML artifact previews are unsandboxed

**Impact:** High renderer security issue.

**Precise issue:** The artifact contract says HTML artifacts run in a sandboxed preview. The hidden iframe used for log capture has `sandbox="allow-scripts"`, but the visible artifact preview iframe uses `srcdoc` without a sandbox attribute.

Relevant code:

- `src/bun/artifacts.ts`: contract text describes sandboxed previews and uses a hidden sandboxed iframe for log capture.
- `src/renderer/components/ArtifactsPanel.svelte`: visible preview iframe uses `srcdoc` without sandbox.
- `e2e/transcript-artifacts.test.ts`: checks that an iframe exists, but not that it is sandboxed.

**Why this matters:** An unsandboxed `srcdoc` iframe normally shares origin with the parent document. Malicious generated HTML can access parent DOM, inspect app state, or modify UI unless browser/electrobun embedding behavior prevents it. The product contract should not depend on that accidental behavior.

**Best fix:**

Use a sandboxed visible preview iframe:

```svelte
<iframe
  sandbox="allow-scripts"
  referrerpolicy="no-referrer"
  srcdoc={artifact.html}
/>
```

Do not add `allow-same-origin`, `allow-top-navigation`, `allow-popups`, or `allow-forms` unless a specific reviewed product requirement needs them.

**Verification required:**

- Component/e2e test asserting the visible iframe has the expected sandbox attribute.
- Malicious HTML fixture that tries to read or mutate `parent.document`; parent state must remain unchanged.
- Regression test that ordinary HTML artifact scripts still run inside the preview.

**Documentation impact:** Update artifact preview security notes if not already present in specs.

**Confidence:** High.

### AUD-007 - Duplicate same-cwd workspace tabs share durable state

**Impact:** High state-isolation issue. The PRD says each workspace tab has its own runtime and session catalog, but duplicate tabs for the same cwd share durable session state.

**Precise issue:** Runtime identity is per tab, but the durable session/catalog path is cwd-derived. Opening the same cwd in two tabs creates two live runtime identities and log DBs, but both use the same session directory, structured SQLite database, pi sessions, thread surfaces, queues, and catalog sessions.

Relevant code:

- `src/bun/workspace-runtime-registry.ts`: creates a unique runtime id per opened workspace tab and per-runtime app logs.
- `src/bun/workspace-runtime-registry.ts`: passes `sessionDir = getSvvySessionDir(cwd, agentDir)` into the session catalog.
- `src/bun/session-catalog.ts`: uses that session dir for structured DB and thread sessions.
- `src/bun/structured-session-state.ts`: opens a cwd-derived structured DB and reuses workspace rows.
- `src/bun/workspace-runtime-registry.test.ts`: currently expects sharing for duplicate cwd tabs.

**Why this matters:** Duplicate tabs can leak or race session state across tabs. One tab's sessions, queues, prompt locks, managed surfaces, workflow runs, or thread state can appear in another tab that is supposed to be isolated. Live app logs and UI restore are per runtime, which makes the split especially confusing: some state is isolated and some is shared.

**Best fix:**

Make the durable session/state root runtime-scoped when the product opens duplicate workspace tabs:

1. Use `workspaceId` as part of the session root, such as `agentDir/workspace-runtimes/<workspaceId>/sessions`.
2. Pass that runtime-scoped session root into the catalog.
3. Ensure the structured workspace row id matches the runtime workspace id.
4. Invert the existing sharing test so duplicate same-cwd tabs get distinct DB paths and distinct session catalogs.

If shared same-cwd durable state is actually desired, the PRD and features need to be rewritten. That would be a product-contract change, not an implementation fix.

**Verification required:**

- Open the same cwd twice and assert different structured DB paths.
- Create session/thread/queue state in tab A and assert it is invisible in tab B.
- Assert app logs, UI restore, and session catalog all agree on the same runtime identity.
- Remove or rewrite tests that codify duplicate-tab sharing.

**Documentation impact:** None if fixed to match the current PRD. If sharing is retained, update `docs/prd.md` and `docs/features.ts`.

**Confidence:** High.

### AUD-008 - Workspace-scoped RPC paths still use the active runtime

**Impact:** High multi-workspace correctness issue.

**Precise issue:** Some backend handlers receive workspace-scoped requests but resolve the active runtime instead of the request's workspace id. Prompt-library calls are the clearest confirmed case. Some settings/workflow-agent paths are also active-runtime based and need classification into app-global versus workspace-affecting behavior.

Relevant code:

- `src/bun/workspace-contract.ts`: defines workspace-scoped prompt-library methods.
- `src/renderer/runtime/chat-runtime.ts`: sends scoped prompt-library requests.
- `src/bun/index.ts`: prompt-library handlers use `getActiveRuntime()`.
- `src/bun/index.ts`: several settings/workflow-agent handlers also use active runtime.
- `src/renderer/components/Settings.svelte`: calls settings methods without a workspace id.

**Why this matters:** In a multi-workspace app, a non-active workspace request can mutate or read the active workspace. Prompt libraries, generated/external sources, workflow-agent generated files, and settings that write workspace files can land in the wrong cwd.

**Best fix:**

1. Route every `WorkspaceScopedRequest` through `getWorkspaceRuntime(input)` or equivalent.
2. Split settings APIs into app-global settings and workspace-affecting settings.
3. Require workspace id for any setting that writes workspace files, syncs workflow-agent components, or reads workspace-local prompt/library state.
4. Update renderer settings surfaces to carry the target workspace id explicitly.

**Verification required:**

- Open workspace A and B, make B active, send prompt-library update scoped to A, and assert only A changes.
- Repeat for prompt-library reads, snapshots, and generated/external sources.
- For workflow-agent component sync, assert the generated file is written to the requested workspace, not the active workspace.
- Renderer tests proving scoped calls retain workspace id.

**Documentation impact:** Update settings/workspace specs if the app-global versus workspace-local settings split is formalized.

**Confidence:** High for prompt-library misuse. Medium-high for settings until each handler is classified.

### AUD-009 - Nonterminal Smithers runs are not reattached after restart

**Impact:** High workflow reliability issue.

**Precise issue:** The product contract calls for recovery of supervised workflow runs after restart: find nonterminal runs, bootstrap state, reconnect monitors from event cursors, and re-emit durable attention. The current restore path hydrates state and events, but does not reattach a live monitor or explicitly resume Smithers execution for nonterminal runs.

Relevant code:

- `src/bun/session-catalog.ts`: creates the manager, resumes title generation, and restores workflow state when surfaces open or prompts prepare.
- `src/bun/workflow-supervision/manager.ts`: restore path hydrates ownership and flushes events, but does not recreate monitor execution or call Smithers resume.
- `src/bun/workflow-supervision/manager.ts`: `runWorkflowInBackground` is launch-oriented.
- Smithers full documentation: resuming requires `runWorkflow(..., { runId, resume: true })`.

**Why this matters:** After restart, a run can appear waiting/running in durable state while no process is actually monitoring or resuming it. Future Smithers events, approval attention, and task projections may never arrive unless another path incidentally repairs the state.

**Best fix:**

1. On workspace startup, enumerate every nonterminal structured workflow run.
2. For each run, load durable Smithers state and the last consumed event cursor.
3. Recreate monitor state and subscribe from that cursor.
4. For resumable active runs, call Smithers resume with the explicit `runId`.
5. Re-emit undelivered durable attention without creating duplicate runs.
6. Keep lazy surface-open restoration as a fallback, not the only recovery path.

**Verification required:**

- Start a run, leave it running/waiting, recreate the manager/app, and assert a new monitor is attached.
- Restart while waiting for approval or signal and assert attention reappears without duplicate run creation.
- Restart after terminal state and assert no resume happens.
- Assert event cursor recovery does not replay duplicate projections.

**Documentation impact:** None if implementing the current contract. If recovery stays lazy/on-open only, specs and progress must say that explicitly.

**Confidence:** High for missing reattach/resume. Medium for startup-wide scope until full app boot path is rechecked.

### AUD-010 - Omitted Smithers `runId` can implicitly resume the wrong run

**Impact:** High workflow correctness issue.

**Precise issue:** The Smithers tool contract says a handler can resume a run by explicitly supplying `runId`. Current manager logic can substitute a resumable same-thread, same-workflow run when `runId` is omitted.

Relevant code:

- `src/bun/smithers-tools.ts`: `runId` is optional in `smithers.run_workflow`.
- `src/bun/workflow-supervision/manager.ts`: launch logic substitutes a resumable run if one exists.
- `src/bun/workflow-supervision/manager.ts`: selector chooses the newest same-thread same-workflow waiting/continued run.

**Why this matters:** A handler call that appears to start a new workflow can instead resume stale state from an earlier run. This is especially dangerous after changed input, repair attempts, or unrelated workflow invocations with the same workflow id.

**Best fix:**

Remove implicit resume.

Recommended behavior:

- If `runId` is omitted, start a new run.
- If an active same-thread workflow would make a new run ambiguous, reject with a clear error that tells the handler to pass `runId`, cancel the old run, or request an explicit replacement mode.
- If `runId` is supplied, resume exactly that run after validating workflow identity, ownership, and input compatibility.

**Verification required:**

- Omitted `runId` creates a new run or rejects due to active conflict; it never silently resumes.
- Explicit `runId` resumes the intended run.
- Mismatched workflow/input/source is rejected.
- Existing recovery/resume tests updated to pass explicit run ids.

**Documentation impact:** Smithers tool docs/spec should state the explicit resume rule.

**Confidence:** High.

### AUD-011 - Cancelling inactive waiting Smithers runs can leave them stuck

**Impact:** High workflow lifecycle issue.

**Precise issue:** Cancelling a waiting run requests cancellation and aborts any in-memory monitor, but an inactive paused run may have no live engine loop to observe the cancellation request and terminalize the run.

Relevant code:

- `src/bun/smithers-tools.ts`: `runs.cancel` calls the workflow manager.
- `src/bun/workflow-supervision/manager.ts`: cancellation requests cancellation, aborts in-memory monitor, and flushes events.
- Smithers reference behavior: paused waiting-approval or waiting-timer states can be terminalized directly with `RunCancelled`; running states use cancellation observed by the engine loop.

**Why this matters:** A waiting approval/timer/event can remain nonterminal after cancellation. That blocks handler progress, `thread.handoff`, and wait-state clearing.

**Best fix:**

Mirror Smithers cancellation semantics:

1. Inspect the current run status.
2. For inactive paused states that Smithers supports for direct cancellation, terminalize immediately and write a `RunCancelled` event.
3. Update attempt/timer/approval state consistently.
4. Flush/project terminal state into structured session records.
5. Keep live running runs on the request-and-abort path.
6. Decide and document whether `waiting-event` is directly cancellable or remains engine-mediated.

**Verification required:**

- Cancel waiting-approval with no live monitor; Smithers row becomes cancelled, structured workflow becomes cancelled, wait attention clears.
- Cancel waiting-timer similarly.
- Cover waiting-event according to the resolved policy.
- Running cancellation still uses the live cancellation path.

**Documentation impact:** Update Smithers tool/spec docs if waiting-event support is explicitly included or excluded.

**Confidence:** Medium-high. Waiting approval/timer risk is clear; waiting-event policy needs confirmation.

### AUD-012 - Project CI projection depends on in-memory terminal output

**Impact:** High durability issue for Project CI status.

**Precise issue:** Project CI records are created only when terminal workflow output is available in an in-memory map. If the app restarts after the Smithers run finishes but before projection records the CI result, the terminal output is lost and the manager cannot validate or record the CI result later.

Relevant code:

- `src/bun/workflow-supervision/manager.ts`: terminal output for CI projection is read from `terminalOutputByRunId`.
- `src/bun/workflow-supervision/manager.ts`: projection refuses to record CI if the in-memory output is missing or invalid.
- `src/bun/structured-session-state.ts`: persists CI run/check records after projection.

**Why this matters:** A CI workflow can finish successfully in Smithers, but the svvy UI can permanently miss the result after a restart. CI state should be derived from durable workflow output, not process memory.

**Best fix:**

1. Persist terminal workflow output needed for CI projection, or re-read it from Smithers durable state during terminal projection.
2. Validate the durable output against `resultSchema` before recording CI.
3. Keep CI upserts idempotent so recovery can safely run after restart.
4. Preserve current invalid-output rejection behavior.

**Verification required:**

- Launch declared Project CI workflow, finish it, recreate manager before CI projection, and assert CI run/check records are recovered from durable output.
- Invalid or lookalike outputs still do not create CI records.
- Re-running recovery does not duplicate checks.

**Documentation impact:** None if implementing existing durability expectations.

**Confidence:** High.

### AUD-013 - Queued-message claiming is not atomic and dispatching rows are hidden

**Impact:** High conversation reliability issue.

**Precise issue:** Surface queued messages are drained through a read-then-mark sequence instead of a single transactional claim. The backend peeks the next pending surface message and then marks it dispatching. A concurrent drain can select the same row before either caller marks it. Separately, dispatching messages are excluded from the queue projection, so the UI can lose the visible "sending" row while a prompt is in flight.

Relevant code:

- `src/bun/session-catalog.ts`: prompt sends queue messages for busy surfaces and drains them later.
- `src/bun/structured-session-state.ts`: `peekPendingSurfaceMessage` and `markSurfaceMessageDispatching` are separate operations.
- `src/bun/structured-session-state.ts`: startup resets `steering`/`dispatching` queue states back to `queued`.
- `src/bun/structured-session-state.ts`: queue list/projection excludes dispatching rows.

**Why this matters:** A race can double-dispatch one queued user message. Even without the race, the user can lose visible feedback that a queued message has started dispatching.

**Best fix:**

1. Replace the peek-then-mark pair with one transactional `claimNextQueuedSurfaceMessage(surfacePiSessionId)` operation.
2. Use a conditional update from `queued` to `dispatching` ordered by queue position and return the claimed row.
3. Include `dispatching` rows in the renderer projection as locked/in-flight queue items.
4. Keep the existing startup reset from `dispatching` to `queued`, but record a recovery event or audit fact so the restart behavior is explainable.

**Verification required:**

- Two concurrent claim attempts against one queued row return one claim and one empty result.
- Pre-accept failure restores the row to queued.
- Restart with a dispatching row restores it to queued.
- Renderer shows dispatching queue rows as locked/in-flight.

**Documentation impact:** Update queue behavior specs if dispatching/recovery states are documented.

**Confidence:** High for non-atomic claim and hidden dispatching state. Medium for observed double-dispatch frequency; the race is present, but was not reproduced under load.

### AUD-014 - Queue reorder persists continuously during drag hover

**Impact:** Medium performance and durability issue.

**Precise issue:** The queued-message strip persists reorder changes while drag movement is still in progress, not only when the user drops the item.

Relevant code:

- `src/renderer/components/QueuedMessagesStrip.svelte`: calls `onReorder` during drag movement.
- `src/bun/session-catalog.ts`: exposes reorder RPC.
- `src/bun/structured-session-state.ts`: rewrites positions and emits queue events.

**Why this matters:** A single drag can write many intermediate orders to SQLite and emit many durable events. It also makes partially explored drag positions durable before the user commits the drop.

**Best fix:**

1. Keep drag-hover order changes local in the component.
2. Call the backend reorder RPC exactly once on pointer-up/drop.
3. Add backend no-op detection so identical order submissions do not write rows or emit events.
4. Update only changed rows instead of rewriting the full queue where practical.

**Verification required:**

- Component test: drag across several positions, release once, observe one reorder RPC.
- Store test: identical order produces no event/write.
- Restart test: only the final dropped order persists.

**Documentation impact:** None.

**Confidence:** High.

### AUD-015 - Handler `thread.handoff` reconciliation can be dropped when orchestrator is active

**Impact:** High orchestration correctness issue.

**Precise issue:** A successful handler `thread.handoff` records the episode and decision, then attempts to wake the orchestrator for reconciliation. If the orchestrator is already active, the wake path returns and no durable pending reconciliation is stored.

Relevant code:

- `src/bun/thread-handoff-tool.ts`: completes thread handoff, episode, and decision state.
- `src/bun/session-catalog.ts`: `resumeOrchestratorAfterHandlerHandoff` returns when the orchestrator is active.

**Why this matters:** The product contract says a successful handoff should immediately open an orchestrator reconciliation turn. If the orchestrator is busy at the wrong moment, the handler's result can remain durable but unprocessed until a user manually prompts the orchestrator.

**Best fix:**

Introduce a durable handoff-reconciliation delivery queue keyed by episode/thread/command.

1. On successful handoff, persist a pending reconciliation record.
2. Attempt to wake the orchestrator immediately.
3. If the orchestrator is busy, leave the record pending.
4. After the orchestrator settles, on surface open, and after restart, drain pending reconciliation records exactly once.
5. Mark reconciliation delivered only after the synthetic orchestrator turn is accepted.

Do not overload the normal user queued-message mechanism; reconciliation is system work tied to a handoff episode and needs exact-once semantics.

**Verification required:**

- Handler hands off while orchestrator is idle: one reconciliation turn starts.
- Handler hands off while orchestrator is active: no concurrent turn starts, but exactly one reconciliation turn starts after settle.
- Restart with a pending reconciliation record delivers it once.
- Duplicate handoff events do not duplicate reconciliation.

**Documentation impact:** Update thread/handoff specs to describe pending reconciliation delivery if a spec exists.

**Confidence:** High.

### AUD-016 - Initial auto-start handler handoff does not wake the orchestrator

**Impact:** High orchestration correctness issue.

**Precise issue:** Handler threads can auto-start with an initial prompt. If that initial handler turn calls `thread.handoff`, the regular post-run wake path is not invoked, so the orchestrator may never get the reconciliation turn.

Relevant code:

- `src/bun/session-catalog.ts`: `createHandlerThread` schedules `startInitialHandlerThreadPrompt`.
- `src/bun/session-catalog.ts`: `startInitialHandlerThreadPrompt` runs the handler prompt but does not call the orchestrator handoff resume path after completion.
- Existing tests cover a normal handler reply path but not initial auto-start handoff reconciliation.

**Why this matters:** Short delegated tasks are likely to complete during their first turn. Those are exactly the cases where a missing wake leaves the orchestrator idle with completed handler work.

**Best fix:**

Use the same durable reconciliation mechanism from AUD-015 for all handler prompt completions.

Minimal safe change:

1. Ensure `startInitialHandlerThreadPrompt` runs the same finalizer used by normal handler turns.
2. That finalizer should inspect newly completed handoffs and enqueue/deliver reconciliation.

Better change:

- Make `thread.handoff` itself responsible for durable reconciliation delivery, independent of which handler-turn entrypoint produced it.

**Verification required:**

- Auto-start handler immediately calls `thread.handoff`; orchestrator receives one synthetic reconciliation turn.
- Same scenario while orchestrator is busy; reconciliation is pending and delivered after settle.
- Restart after auto-start handoff but before delivery; reconciliation still delivers once.

**Documentation impact:** None if implementing the existing handoff contract.

**Confidence:** High.

### AUD-017 - Prompt freshness is detected but not enforceably applied

**Impact:** High prompt correctness issue.

**Precise issue:** Surfaces can detect stale prompt bindings and show a warning, but there is no complete path to apply an updated prompt binding before the next user turn. Existing managed-session recreation checks actor/provider/model/recreate flags, but not prompt drift. The UI banner is informational and lacks working `Update for next turn` behavior.

Relevant code:

- `src/bun/session-catalog.ts`: computes prompt stale state.
- `src/renderer/components/DockviewPanelHost.svelte`: shows a stale prompt banner.
- `src/bun/session-catalog.ts`: recomputes resolved prompt during dispatch preparation.
- `src/bun/session-catalog.ts`: managed session recreation does not use prompt drift as a recreation/update trigger.
- No complete update API was found for applying the fresh binding to an existing surface.

**Why this matters:** Users can see that a session prompt is stale, choose or expect freshness, and still send the next turn with the old prompt. This breaks prompt-library, runtime standards, generated tool/schema blocks, and actor instruction updates.

**Best fix:**

1. Persist prompt binding metadata per surface: source ids, revision ids, standards hashes, resolved prompt hash, and resolved text snapshot.
2. Add an explicit surface prompt-binding update API.
3. If a surface is idle, recreate or update the managed pi session before the next user turn.
4. If a surface is active, queue the update for the next turn and display that state.
5. Emit surface snapshots when prompt-library/runtime-standard changes affect stale status.
6. Make dispatch refuse to silently use an old prompt after the user has requested an update.

**Verification required:**

- Existing surface with stale prompt keeps old prompt when user chooses keep.
- User chooses update; next prompt sent to pi includes the new resolved prompt.
- Runtime standards drift triggers stale status.
- Queued message drain, handler handoff resume, workflow attention resume, and normal prompt send all honor pending prompt updates.
- Renderer actions exist and update state.

**Documentation impact:** Update prompt-binding and surface lifecycle specs. If the user-visible feature shape changes, update `docs/features.ts`.

**Confidence:** High.

### AUD-018 - New-session and mode-switch prompt composition can double-wrap or bypass generated prompts

**Impact:** High agent behavior issue.

**Precise issue:** Session defaults can already contain a composed system prompt, then the catalog composes again using those defaults. Non-default session-agent text can be appended under `## Session Agent` even when that text is already a full composed prompt. Mode switching has inconsistent call paths that can pass raw prompt text directly.

Relevant code:

- `src/bun/index.ts`: session defaults are built with a composed prompt.
- `src/bun/session-catalog.ts`: create-session path calls prompt composition again with `defaults.systemPrompt`.
- `src/bun/session-catalog.ts`: non-default suffix handling appends session-agent content under `## Session Agent`.
- `src/bun/session-catalog.ts`: mode switching can pass defaults prompt text directly into managed-surface recreation.

**Why this matters:** Agents can receive duplicated generated sections, duplicated tool declarations, duplicated web context, or a full prompt nested as a suffix. Conversely, raw mode input can bypass expected generated prompt sections. Both cases make behavior hard to reason about and can alter tool use.

**Best fix:**

1. Separate raw session-agent settings from resolved system prompts at the type level.
2. `SessionDefaults` should carry `sessionAgentKey` and raw `SessionAgentSettings`, not a pre-composed system prompt.
3. Centralize final prompt composition in the session catalog exactly once.
4. Use distinct names/types such as `sessionAgentSuffix` and `resolvedSystemPrompt` so a full prompt cannot accidentally be treated as a suffix.
5. Make mode switching route through the same composition function as new-session creation.

**Verification required:**

- New session prompt contains each generated section once.
- Custom "dumb" or suffix-style session agent appears once under the intended heading.
- No nested full prompt appears under `## Session Agent`.
- Mode switching preserves generated sections and applies only the intended changed agent settings.

**Documentation impact:** Update prompt-composition specs if they currently describe the old flow.

**Confidence:** High for double-wrap risk. Medium for raw mode-switch bypass until all mode-switch callers are exhaustively tested.

### AUD-019 - Ambient pi extension tools can leak into orchestrator and handler sessions

**Impact:** High capability-isolation and determinism issue.

**Precise issue:** Workflow task agents disable pi extension discovery, but orchestrator and handler resource loaders do not pass `noExtensions: true`. The custom tool list suppresses built-ins, but ambient workspace/user pi extensions can still be discovered by pi.

Relevant code:

- `src/bun/session-catalog.ts`: orchestrator and handler sessions use `DefaultResourceLoader` with prompt overrides and custom tools, but without `noExtensions: true`.
- `src/bun/workflow-task-agent.ts`: task agents already pass `noExtensions: true` and have regression coverage.
- pi reference behavior: default resource loading can discover `.pi/extensions`.

**Why this matters:** The actor contract says svvy actors receive the generated callable API, not arbitrary ambient pi extensions. Extension leakage weakens capability isolation and makes behavior dependent on local user/workspace files.

**Best fix:**

1. Pass `noExtensions: true` for orchestrator and handler resource loaders.
2. Keep explicit svvy tool registration as the only source of actor tools.
3. Add active-tool assertions for orchestrator, handler, and task-agent sessions.

**Verification required:**

- Create a fixture `.pi/extensions/leak.ts` tool.
- Create/open orchestrator and handler sessions; assert leaked extension is absent.
- Assert active tool names exactly match the generated/allowed svvy tool set.
- Preserve existing task-agent no-extension regression test.

**Documentation impact:** None if implementing current actor-surface contract.

**Confidence:** High from source and pi behavior. Dynamic reproduction was not run in the audit.

### AUD-020 - Workflow task-agent config, generated declarations, saved settings, and runtime behavior diverge

**Impact:** High workflow authoring correctness issue.

**Precise issue:** The generated workflow contract, saved settings renderer, normalization, and runtime tool-surface enforcement disagree.

Confirmed drift:

- Generated `WorkflowTaskAgentConfig` uses `thinkingLevel`.
- Rendered workflow-agent settings use `reasoningEffort`.
- Generated/default contracts include `cx.*`, but normalization filters `cx.*` out.
- Runtime task-agent creation hard-codes the full task tool set and does not enforce `config.toolSurface`.

Relevant code:

- `src/bun/workflow-authoring-contract.ts`: generated config declaration.
- `src/bun/workflow-task-agent.ts`: runtime task-agent tool registration.
- `src/bun/session-agent-settings.ts`: rendered workflow-agent components/settings.
- Normalization code for workflow task-agent tool surfaces.

**Why this matters:** Authors and generated code can believe a task agent has one configuration while runtime gives it another. Narrow tool surfaces are not enforced. `cx.*` tools can disappear despite being part of the declared/default surface. The `thinkingLevel` versus `reasoningEffort` mismatch can break assignability or silently drop intended model settings.

**Best fix:**

1. Define one canonical workflow task-agent config schema.
2. Generate TypeScript declarations, settings UI defaults, normalization, persistence, and runtime behavior from that schema.
3. Choose either `thinkingLevel` or `reasoningEffort`, then migrate all code to that one name without compatibility aliases unless explicitly requested.
4. Preserve `cx.*` if it remains part of the contract, or remove it from the contract and docs if not.
5. Make runtime tool registration filter an ordered registry by `config.toolSurface`.

**Verification required:**

- Rendered workflow-agent settings are assignable to the generated `WorkflowTaskAgentConfig`.
- `cx.*` survives normalization if still documented.
- A narrow `toolSurface` produces exactly that task-agent tool set plus any required framework tool such as `list_tools`.
- A broad/default tool surface matches the generated declaration.

**Documentation impact:** Update `docs/features.ts` and workflow authoring specs if task-agent config names or tool surfaces are materially changed.

**Confidence:** High.

### AUD-021 - Task-agent prompts and `request_context` prompt binding are not durable enough

**Impact:** High prompt and provenance correctness issue.

**Precise issue:** Custom workflow task-agent prompts can replace the svvy workflow-task base prompt instead of layering on top of it. Task attempts store prompt text/model/resume metadata but not enough binding metadata to prove which prompt-library revision, standards hash, or resolved prompt hash was used. Handler `request_context` state is durable, but active pi sessions are not guaranteed to be recreated or updated so the newly requested context is present on the next turn.

Relevant code:

- `src/bun/workflow-task-agent.ts`: custom config system prompt can replace the generated base task prompt.
- `src/bun/structured-session-state.ts`: task attempt prompt metadata lacks revision/hash/source binding fields.
- `src/bun/request-context-tool.ts`: records requested context.
- `src/bun/session-catalog.ts`: rebuilds resolved prompts, but retained managed sessions are recreated for actor/provider/model/recreate flags, not loaded context changes.

**Why this matters:** Workflow task agents can lose svvy's task contract and API instructions. Task-attempt records are not audit-grade enough to reconstruct the exact prompt provenance. Handler-requested context, including Project CI context, may be recorded in state but absent from the live pi prompt used for the next turn.

**Best fix:**

1. Treat workflow task-agent custom prompts as overlays appended to the svvy workflow-task base prompt unless an explicit product decision says full replacement is allowed.
2. Persist prompt binding metadata on task attempts: prompt source ids, revision ids, standards hashes, resolved prompt hash, and any generated artifact/schema ids.
3. When `request_context` changes loaded context, mark the managed handler surface as needing prompt update before the next turn.
4. Recreate or update the live pi session before dispatching the next handler prompt when loaded context changed.
5. Avoid recency-based task-attempt lookup where exact run/node/iteration/attempt identity is available.

**Verification required:**

- Custom task prompt still includes the base workflow-task instructions and API contract.
- Task attempts persist prompt binding metadata at creation.
- `request_context(["ci"])` causes the next handler prompt sent to pi to include CI context.
- Duplicate or repeated task-agent resume handles do not bind to the wrong attempt.

**Documentation impact:** Update workflow task-agent prompt semantics and task-attempt provenance specs.

**Confidence:** High for custom-prompt replacement and request-context risk. Medium-high for metadata gap. Medium for duplicate resume ambiguity until exact live cases are tested.

### AUD-022 - Dockview transcript panels omit semantic blocks and action callbacks

**Impact:** Medium-high UI functionality issue.

**Precise issue:** `ChatTranscript` supports semantic transcript blocks and callbacks, but the Dockview panel host mounts it with only basic props. The static Dockview chat path therefore loses wait/failure/command/handoff/thread/workflow cards and their actions.

Relevant code:

- `src/renderer/components/ChatTranscript.svelte`: supports `semanticBlocks` and action callbacks.
- `src/renderer/transcript-projection.ts`: builds semantic blocks.
- `src/renderer/components/DockviewPanelHost.svelte`: mounts `ChatTranscript` without those semantic blocks/callbacks and passes an empty mention path set.

**Why this matters:** The Dockview UI is supposed to be the main semantic workspace. If it renders only raw transcript text, users lose structured workflow status, command cards, handoff cards, failure actions, and navigation affordances.

**Best fix:**

1. Move semantic block construction and callback wiring into the Dockview chat panel path.
2. Pass `semanticBlocks`, real workspace mention paths, and action callbacks into `ChatTranscript`.
3. Ensure callbacks open static inspector panes using the target surface model rather than global focus state.

**Verification required:**

- Component test rendering command, artifact, wait, failure, thread, and workflow events in a Dockview chat panel.
- Each semantic block action calls the expected runtime/open-surface callback.
- Mention paths are not always empty in Dockview panels.

**Documentation impact:** None unless the semantic transcript feature inventory needs wording updates.

**Confidence:** High.

### AUD-023 - Artifact/static inspector state still depends on global focus

**Impact:** Medium-high multi-pane UX correctness issue.

**Precise issue:** The codebase has a newer Dockview surface target model for static artifact and inspector panes, but legacy focus-global artifact/inspector state remains. Some artifact and inspector openings depend on `currentSession`, `currentPane`, or the focused panel rather than a durable pane target.

Relevant code:

- `src/renderer/components/ChatWorkspace.svelte`: maintains focus-global artifact state and a global `ArtifactsPanel`.
- `src/renderer/components/DockviewPanelHost.svelte`: supports static artifact/inspector targets.
- `src/bun/workspace-contract.ts`: defines `WorkspacePaneSurfaceTarget`-style static targets.

**Why this matters:** Multiple panes cannot independently inspect different artifacts or related records if the preview model is partly global. Opening an artifact from one pane can affect another pane or the global drawer. It also makes deep links and restored static panes less reliable.

**Best fix:**

1. Retire the focus-global artifact drawer/modal inspector paths for Dockview-owned surfaces.
2. Open artifacts, commands, threads, workflow runs, task attempts, and CI checks as explicit `WorkspacePaneSurfaceTarget` records through `runtime.openSurface`.
3. Store local preview state in pane-local state, not global workspace focus state.
4. Keep any global drawer only as a deliberate app-global shortcut with clear source routing.

**Verification required:**

- Open two panes, inspect two different artifacts, and assert each pane keeps its own artifact.
- Repeat for command/thread/task/workflow/CI inspectors.
- Restored layout reopens static panes without relying on focus.

**Documentation impact:** Update workspace navigation spec if the static inspector target model is made the only supported path.

**Confidence:** High that both models coexist. Exact reachability of every legacy path needs route-by-route cleanup.

### AUD-024 - Restart recovery lacks one durable scheduler for active prompts, initial starts, queues, and workflow monitors

**Impact:** Medium-high cross-cutting reliability issue.

**Precise issue:** Several restart-sensitive flows have local repair logic, but there is no single durable scheduler that owns recovery of in-flight or pending work across prompts, queues, initial handler starts, handoff reconciliation, and workflow supervision.

Concrete confirmed pieces:

- AUD-009: nonterminal Smithers runs are hydrated but not reattached/resumed.
- AUD-013: dispatching queue rows are restored to queued on startup, but claiming is non-atomic and dispatching rows are hidden.
- AUD-015: handoff reconciliation can be dropped when the orchestrator is busy.
- AUD-016: initial auto-start handler handoff lacks the normal wake path.

**Why this matters:** Each individual bug has a local fix, but the shared product requirement is stronger: pending work should survive process boundaries and resume exactly once. Without one recovery owner, fixes can remain fragmented and new flows can repeat the same pattern.

**Best fix:**

Create a workspace-runtime recovery coordinator that runs at startup and after runtime rehydration.

Responsibilities:

1. Reattach or settle nonterminal workflow runs.
2. Restore dispatching/steering queued messages to queued and expose their status.
3. Drain durable handoff reconciliation records.
4. Resume pending initial handler starts exactly once.
5. Emit explicit recovery events/facts so the UI and logs can explain what happened.
6. Use transactional claims for every resumed work item.

**Verification required:**

- App restart with one pending item from each category; coordinator resumes or restores each exactly once.
- Concurrent startup/open-surface paths do not double-run recovery.
- Recovery order is deterministic where dependencies exist, especially workflow attention before handler/orchestrator prompt resume.

**Documentation impact:** Add or update a recovery section in structured-session/workspace-runtime docs.

**Confidence:** Medium-high. This section consolidates confirmed restart gaps rather than introducing a separate newly probed call site.

### AUD-025 - Streaming emits full snapshots and reparses full Markdown on every delta

**Impact:** Medium-high scalability and responsiveness issue.

**Precise issue:** Backend stream events emit full surface snapshots on every content delta. The renderer receives those snapshots, resets/replaces large message structures, and sends the accumulated assistant content back through full Markdown parsing/highlighting paths each time.

Relevant code:

- `src/bun/session-catalog.ts`: stream events update `activeStreamMessage` and call surface sync for each delta.
- `src/bun/session-catalog.ts`: surface sync builds full `ConversationSurfaceSnapshot` payloads.
- `src/renderer/runtime/chat-runtime.ts`: sync handling resets/replaces agent messages and stream state.
- `src/renderer/components/ChatTranscript.svelte` and `src/renderer/components/AssistantMarkdown.svelte`: render accumulated content through Markdown parsing/code-block extraction.

**Why this matters:** Cost grows with transcript size and response length. Every token or chunk can carry full conversation state across the bridge and trigger repeated parse/highlight work for content that did not change.

**Best fix:**

1. Introduce a stream patch protocol with monotonic stream sequence numbers.
2. Use full snapshots only for open/rebaseline/settled states.
3. Emit compact append/replace/finalize patches for live stream deltas.
4. Apply stream patches in the renderer without resetting the whole agent/session object.
5. Cache Markdown parse results by stable block/message id.
6. During streaming, parse only changed segments where possible and defer expensive syntax highlighting/Mermaid work until fences are complete or the stream is final.

**Verification required:**

- Backend test: stream deltas emit compact patches instead of full message snapshots.
- Renderer test: applying a patch mutates only the active stream message.
- Markdown test: unchanged blocks are not reparsed across stream deltas.
- Manual long-response profile showing lower bridge payload size and render work.

**Documentation impact:** Define snapshot versus patch/rebaseline behavior in the surface-sync spec. Update `docs/features.ts` only if the user-facing streaming model changes.

**Confidence:** High for current full-snapshot/reparse path. Actual measured cost still needs profiling.

### AUD-026 - Workflow inspector polls and rebuilds full snapshots instead of applying deltas

**Impact:** Medium-high performance issue for large workflow runs.

**Precise issue:** The workflow inspector uses a short polling loop and replaces the whole inspector snapshot. Backend streaming calls still rebuild the full inspector from Smithers DevTools state, frames, events, details, and projections. Renderer expansion/search/selection changes can trigger more full reloads.

Relevant code:

- `src/renderer/components/WorkflowInspectorPane.svelte`: schedules polling and replaces the inspector state.
- `src/bun/session-catalog.ts`: inspector stream calls into the workflow manager and returns full inspector snapshots.
- `src/bun/workflow-supervision/manager.ts`: fetches full DevTools state and full inspector data.
- `src/bun/workflow-inspector.ts`: rebuilds tree/projection data by scanning full snapshots.

**Why this matters:** A tree-first inspector should scale with changed workflow events. Full rebuilds and polling amplify SQLite/Smithers reads, bridge payloads, and renderer work as run history grows.

**Best fix:**

1. Build an inspector live-tree store.
2. Load one initial baseline.
3. Apply Smithers event deltas to the tree and detail indexes.
4. Rebaseline only on snapshot gaps, time-travel changes, or explicit refresh.
5. Keep selection, expansion, and search local unless focused detail data is missing.
6. Pre-index nodes, commands, task attempts, artifacts, CI checks, and event ids for lookup.

**Verification required:**

- Projection tests for add/remove/update/replace deltas.
- Backend stream test proving normal event updates do not call full `getDevToolsSnapshot` every poll.
- Renderer test proving local expansion/search does not refetch the full inspector.
- Manual large-run inspector profile.

**Documentation impact:** Update the workflow-inspector spec to define baseline-plus-delta wire behavior.

**Confidence:** High.

### AUD-027 - Structured selectors and read APIs materialize broad snapshots with limited indexes

**Impact:** Medium-high scalability issue.

**Precise issue:** Hot read paths materialize broad `StructuredSessionSnapshot` objects and then filter/sort in memory. The SQLite schema has relatively few indexes for the number of session/thread/workflow/task/surface/artifact lookup patterns used by sidebars, inspectors, runtime tools, and selectors.

Relevant code:

- `src/bun/structured-session-state.ts`: broad `StructuredSessionSnapshot`, `getSessionState`, and `listSessionStates` materialize many arrays.
- `src/bun/session-catalog.ts`: list/read flows load full snapshots and projections.
- `src/bun/structured-session-selectors.ts`: repeatedly filters and sorts arrays for derived views.
- Existing explicit indexing is limited relative to access patterns.

**Why this matters:** Small workspaces can tolerate snapshot-first reads. Large transcripts, many workflow runs, artifacts, commands, and panes will make sidebar refreshes, inspector opens, attention queries, and runtime tool calls slower and more memory-heavy.

**Best fix:**

1. Add narrow store-level read models for hot views: session list, attention list, queue list, thread list, workflow run list, task-attempt list, artifact list, and command summary list.
2. Keep full `getSessionState` for debug/export paths, not routine UI refresh.
3. Add indexes for session id, thread id, workflow id, task attempt id, surface id, Smithers run id, parent command id, status, updated timestamp, artifact ownership, and queue order.
4. Build selector maps once per snapshot where full snapshots are still needed.
5. Add scale tests with thousands of rows to lock in query count and runtime.

**Verification required:**

- Migration tests for new indexes.
- Equivalence tests comparing new read models to old selector output.
- Scale tests for `listSessions`, inspector open, attention list, and artifact list.
- Ensure snapshots remain available for export/debug.

**Documentation impact:** Update structured-state docs with the read-model/index expectations.

**Confidence:** High.

### AUD-028 - Native app-menu command actions are ignored by the renderer

**Impact:** Medium user-facing command issue.

**Precise issue:** Native menu actions are emitted as an action string by the runtime subscription, but the renderer callback destructures the callback argument as an object. That makes `action` undefined and app-menu commands no-op.

Relevant code:

- `src/shared/shortcut-registry.ts`: shared action registry.
- `src/bun/index.ts`: native app menu sends `{ action }` over the bridge.
- `src/renderer/runtime/chat-runtime.ts`: unwraps the RPC and invokes listeners with the action string.
- `src/renderer/components/ChatWorkspace.svelte`: registers `({ action }) => handleAppMenuAction(action)`.

**Why this matters:** Renderer hotkeys and sidebar actions can still work, but native menu clicks and menu accelerators do not dispatch the intended command from this path.

**Best fix:**

Change the renderer subscription callback to accept the string directly:

```ts
runtime.subscribeAppMenuAction((action) => {
  handleAppMenuAction(action);
});
```

Then keep the bridge/runtime callback type aligned so this mismatch cannot recur.

**Verification required:**

- Unit or renderer test emitting a native menu action and asserting `handleAppMenuAction` receives the expected action.
- Optional e2e/native-menu harness test for a real menu accelerator.

**Documentation impact:** None.

**Confidence:** High.

### AUD-029 - Command palette coverage is incomplete and not fully target-driven

**Impact:** Medium workspace navigation issue.

**Precise issue:** The command palette is partially hard-coded and misses first-class surfaces/actions that exist elsewhere in the app. Handler and task-agent results are limited by focused-session state, and several target types do not use the static pane target model.

Relevant code:

- `src/renderer/command-palette.ts`: registry includes session actions, settings, workflows, panes, and some Project CI prompt actions.
- `src/renderer/command-palette.ts`: handler threads come only from focused session input; task agents use a session/modal route.
- `src/renderer/components/ChatWorkspace.svelte`: already has routes for Logs, Workflows, Context, workflow/task views.
- `src/bun/workspace-contract.ts`: defines static targets for task attempts and Project CI checks.

Confirmed gaps:

- No direct Open Logs command.
- No direct Open Context command.
- Workflow inspector and Project CI inspection actions are incomplete.
- Handler/thread results are limited to focused orchestrator context.
- Task-agent command route is unsettled relative to static target panes.

**Why this matters:** The palette should be the high-speed navigation surface for sessions, logs, workflows, context, task agents, Project CI, and panes. Partial coverage forces users back into sidebar-specific flows and makes restored/static panes less discoverable.

**Best fix:**

1. Add direct Logs and Context commands.
2. Use `WorkspacePaneSurfaceTarget` as the command result target for static surfaces.
3. Build handler, thread, workflow, and task results from durable navigation/read-model rows instead of only focused-session props.
4. Add latest/active Project CI actions and direct check inspection actions.
5. Settle task-agent routing through static pane targets where applicable.

**Verification required:**

- Registry tests for Logs, Context, Workflows, Workflow Inspector, Project CI latest/active/check, handlers, threads, and task attempts.
- E2E/palette tests that searching and selecting each command opens the expected surface.
- Multi-pane tests proving commands target the intended pane/context.

**Documentation impact:** Update command-palette/navigation feature docs if command coverage is described exhaustively.

**Confidence:** High.

### AUD-030 - Destructive session delete remains in backend/API despite archive-first product contract

**Impact:** Medium-high data-loss risk.

**Precise issue:** The product contract describes archive/unarchive as the user-facing non-destructive lifecycle. The UI appears to use Archive/Unarchive, but backend/runtime contracts still expose destructive `deleteSession` paths and stale tests assert delete dialogs/events.

Relevant code:

- `src/bun/workspace-contract.ts`: exposes `deleteSession`.
- `src/bun/session-catalog.ts`: delete unlinks orchestrator and handler pi files.
- `src/bun/index.ts`: bridge event path emits deletion.
- `src/renderer/runtime/chat-runtime.ts`: runtime delete method.
- `src/renderer/components/SessionSidebar.svelte` and `SessionListItem.svelte`: current UI labels are archive/unarchive.
- E2E/bridge tests still cover delete behavior.

**Why this matters:** Any caller that reaches the delete RPC can cause transcript/session loss, even though the product direction is reversible archiving. Tests that preserve destructive behavior make future cleanup harder.

**Best fix:**

1. Remove public `deleteSession` RPC/runtime methods from the normal product API.
2. Convert stale tests to archive/unarchive.
3. Keep archive state durable and reversible.
4. If permanent purge is required later, introduce a separate explicit purge contract with confirmation, scope, cleanup rules, and documentation.

**Verification required:**

- Archive hides the session but preserves files/facts/state.
- Unarchive restores the session.
- No normal renderer path calls destructive delete.
- Old delete tests are removed or rewritten.

**Documentation impact:** Remove stale delete language from specs, including app-log/spec references if present. No PRD/features change is needed if archive remains the intended product behavior.

**Confidence:** High.

### AUD-031 - E2E/proof tests use retries, broad waits, force clicks, and live-provider gates

**Impact:** Medium reliability issue that can hide product bugs or fail in normal local runs.

**Precise issue:** The e2e harness and workflow proof tests include retry loops, broad polling waits, force clicks, brittle selectors, and a real provider-key requirement in default test paths. Repo instructions explicitly say not to hack around failing e2e tests with retries, broad waits, selector churn, best-effort fallbacks, or test-only behavior.

Relevant code:

- `e2e/harness.ts`: launch metadata retries and long workspace chrome polling.
- `e2e/workflow-supervision.test.ts`: live `ZAI_API_KEY` gating and force/retry interactions.
- `e2e/workflow-supervision-proof.test.ts`: long custom polling/stub paths.
- Unit/integration coverage exists for parts of workflow restart behavior, but a full app-level restart e2e was not confirmed.

**Why this matters:** Default e2e should be deterministic and runnable without private provider keys. Broad waits and force clicks can make broken readiness or accessibility states look passing. Live-provider dependency makes local preflight less reliable.

**Best fix:**

1. Split deterministic stubbed e2e from opt-in live-provider smoke tests.
2. Make the default `bun run test:e2e` lane require no provider keys.
3. Replace broad waits with app bridge events/state synchronization.
4. Replace force clicks and brittle nth selectors with stable roles/test ids tied to actual UI contracts.
5. Add a real workflow-supervision restart e2e if restart recovery is a shipped contract.
6. Keep the OrbStack lane as the only default e2e execution path, per repo instructions.

**Verification required:**

- `bun run test:e2e` runs without provider keys.
- Opt-in live-provider smoke test is skipped unless explicitly enabled.
- Restart workflow e2e drives app, relaunches, and verifies recovery using bridge/state evidence.
- No new broad waits/retries/force clicks are introduced.

**Documentation impact:** Update e2e setup docs if the OrbStack/default versus live-provider split is not already clear.

**Confidence:** High for retries/live gates/force waits. Medium for exact restart e2e absence until the full test matrix is rechecked.

### AUD-032 - Progress and feature documentation drift from implemented reality

**Impact:** Medium planning and product-source-of-truth issue.

**Precise issue:** `docs/progress.md` has unchecked or stale items for work that appears shipped or partly shipped, and at least one checked item still has `Commit(s): pending`. The repo instructions say progress is forward-looking, completed items should be ticked with landing commits, and stale migration wording should be rewritten to the resolved design.

Confirmed implemented or partly implemented areas:

- Durable unread state in structured session state and runtime tests.
- Dockview mounted/persisted panes and pane-layout e2e coverage.
- TanStack Form use in agent/provider settings forms.
- App Logs virtualized UI.
- Queued messages backend/UI.

Relevant files:

- `docs/progress.md`
- `docs/features.ts`
- `src/bun/structured-session-state.ts`
- `src/renderer/runtime/chat-runtime.ts`
- `src/renderer/components/DockviewWorkspace.svelte`
- `src/renderer/components/AppLogsPane.svelte`
- `src/renderer/components/QueuedMessagesStrip.svelte`

**Why this matters:** The docs are the source of truth for product scope and roadmap. Drift can cause duplicated work, stale priorities, and incorrect assumptions by future agents.

**Best fix:**

1. Audit each stale progress item against source and tests.
2. Rewrite broad items into shipped capability plus explicit remaining gaps.
3. Tick completed items and add the landing commit hash where known.
4. Leave partly complete items unchecked but rewrite them to the actual remaining work.
5. Keep `docs/features.ts` exhaustive and steady-state, not a changelog.

**Verification required:**

- Cross-check each changed progress item against code/tests.
- Run docs format/check as part of `bun run check` where applicable.
- Avoid claiming completion without a source/test reference or commit hash.

**Documentation impact:** This is itself a docs update. It should be done after or alongside code fixes that change scope.

**Confidence:** High that drift exists. Exact completion status for broad items requires line-by-line doc/source reconciliation.

### AUD-033 - Smithers approval, task, artifact, and detail tools drift from native Smithers shapes

**Impact:** High agent-tool correctness and upgradeability issue.

**Precise issue:** The shipped Smithers-facing tools do not mirror the native Smithers tool schemas documented in the full Smithers reference. Some svvy schemas require fields Smithers treats as optional filters, rename fields, omit structured fields, or return non-native shapes. Task-attempt binding also uses recency lookup by resume handle rather than exact current run/node/attempt identity.

Smithers reference facts from `https://smithers.sh/llms-full.txt`:

- `list_pending_approvals` includes fields such as `requestedAtMs`, `decidedAtMs`, `request`, and `decision`.
- `resolve_approval` uses `action: "approve" | "deny"` plus optional filters such as `runId`, `workflowName`, `nodeId`, `iteration`, `note`, `decidedBy`, and structured `decision`; it has ambiguity guards.
- `get_node_detail` returns `{ detail: ... }`.
- `list_artifacts` takes `{ runId, nodeId?, includeRaw? }` and returns an artifact list.

Current drift:

- `resolve_approval` requires `runId` and `nodeId`, uses `decision` instead of native `action`, omits `workflowName`, `decidedBy`, and structured decision options.
- Approval resolution edits local DB state directly and hard-codes `decidedBy`.
- Pending approval output omits native fields and converts time shape.
- `list_artifacts` returns a svvy-specific `{ runId, outputs, frames }` shape with a non-native limit concept.
- `get_node_detail` does not wrap output as `{ detail }`.
- Task-agent execute TypeScript and structured lookup bind attempts by `agentResume` plus recency.

Relevant code:

- `src/bun/smithers-tools.ts`
- `src/bun/workflow-supervision/manager.ts`
- `src/bun/workflow-task-agent.ts`
- `src/bun/structured-session-state.ts`
- `docs/references/smithers/src/engine/approvals.ts`

**Why this matters:** The PRD says agent-facing workflow control should expose Smithers-native semantic tools and mirror Smithers naming closely. Drift makes generated/documented calls fail or lose fields, blocks native ambiguity handling, loses structured approval decisions, and can bind task results to the wrong attempt when resume handles repeat.

**Best fix:**

1. Generate or directly mirror native Smithers schemas for Smithers tools.
2. Change `resolve_approval` to accept native `action`, filters, `decidedBy`, `note`, and structured `decision`.
3. Preserve native ambiguity behavior when filters do not identify exactly one approval.
4. Use Smithers native approval helpers or an exact local transaction that preserves the same event/decision semantics.
5. Return native shapes for `list_pending_approvals`, `get_node_detail`, and `list_artifacts`.
6. Move svvy-specific frame/output views under separate clearly named tools such as `frames.list` if needed.
7. Bind task-attempt operations by exact current Smithers context: `runId`, `nodeId`, `iteration`, attempt id, and resume handle where relevant. Do not use newest-by-recency fallback when exact identity exists.

**Verification required:**

- Schema tests against the Smithers reference for approval/list/detail/artifact tools.
- `resolve_approval` tests for approve, deny, note, decidedBy, structured decision, ambiguous filters, and missing approval.
- Duplicate `agentResume` task-attempt test proving the exact current attempt is selected.
- Regression tests updating existing drift-lock tests to the native shapes.

**Documentation impact:** No PRD change if aligning with the current PRD. Update generated tool declarations and any svvy Smithers specs. If svvy intentionally keeps custom shapes, the PRD/features must be rewritten, but that conflicts with current instructions.

**Confidence:** High.

### AUD-034 - Additional extracted issues

This issue groups smaller findings that were still independently actionable. They should be tracked as separate fix tasks even though they were grouped by the audit agents.

#### AUD-034A - Private URL guard is syntactic and incomplete

**Impact:** Medium security issue for web fetch providers.

**Precise issue:** `web.fetch` is supposed to reject local files, private app URLs, localhost, private network, and non-web targets. Current validation is mostly syntactic and test coverage only clearly covers `127.0.0.1`.

Relevant code:

- `src/bun/web-runtime/providers/shared.ts`: URL guard helper.
- TinyFish/Firecrawl provider paths that call the shared guard.

Confirmed gaps to cover:

- Other `127.0.0.0/8` addresses.
- IPv6 loopback `[::1]`.
- IPv6 link-local/private/ULA ranges.
- `0.0.0.0`.
- `169.254.0.0/16`.
- Other reserved/private ranges.
- Provider final URLs after redirects or provider-side fetch resolution.

**Best fix:** Build one robust public-web URL classifier that resolves hostnames/IP literals, rejects private/reserved/local ranges for IPv4 and IPv6, enforces `http`/`https`, and validates both requested URL and final returned URLs/metadata from providers.

**Verification required:** Table-driven tests for every reserved/private class, encoded/IPv6 forms, DNS names resolving to private IPs where resolution is available, redirect/final URL rejection, and valid public URLs.

**Confidence:** High for incomplete guard coverage. DNS/final-URL behavior needs provider-specific tests.

#### AUD-034B - Provider credential storage and precedence do not match copy

**Impact:** Medium confidentiality and settings correctness issue.

**Precise issue:** Credentials are stored as plaintext JSON under the user config directory with file permissions, and stored credentials are read before environment variables. UI copy says environment variables override saved values.

Relevant code:

- `src/bun/auth-store.ts`: config-file credential storage and lookup order.
- `src/renderer/components/Settings.svelte`: password inputs and copy describing env override.

**Why this matters:** Plaintext at-rest provider keys are weaker than OS credential storage. Priority mismatch means users can set an environment override and still use an old saved credential.

**Best fix:**

1. Move provider secrets to OS credential storage, such as macOS Keychain, with a migration-free policy unless explicitly requested.
2. Make environment variables override saved credentials, or update UI copy if saved credentials intentionally win.
3. Keep only non-secret metadata in config files.
4. Ensure bridge/DOM inspection masks password values.

**Verification required:**

- Env var wins over saved value if that remains the copy.
- Removing a saved key removes it from credential storage.
- Config files contain no raw provider key.
- Password inputs are masked in bridge/DOM state.

**Confidence:** Medium-high. Plaintext storage and precedence mismatch are clear; final storage mechanism is a product/security choice.

#### AUD-034C - Saved layout slot switching normalizes but may not hydrate inactive slots

**Impact:** Medium UI persistence issue.

**Precise issue:** Startup applies the active saved layout slot. Switching workspace layout slots normalizes/emits state, but the Dockview component applies `fromJSON` only during creation. Inactive slots may not hydrate correctly when switched later.

Relevant code:

- `src/renderer/runtime/chat-runtime.ts`: startup layout restore and `switchWorkspaceLayout`.
- `src/renderer/components/DockviewWorkspace.svelte`: applies layout JSON on creation.

**Best fix:** Add a shared layout hydration routine that applies the selected slot JSON whenever the active slot changes, guarded by an `applyingLayout` flag so emitted layout-change events do not immediately overwrite the restored slot.

**Verification required:**

- Save two layout slots, switch away, restart, switch to inactive slot, and assert exact pane restoration.
- Ensure applying a slot does not emit a save that overwrites the slot with the pre-apply layout.

**Confidence:** Medium-high from code shape; needs an app-level slot-switch test.

#### AUD-034D - Workflow inspector split uses `paneId`/`panelId` inconsistently

**Impact:** Medium pane-placement issue.

**Precise issue:** The Dockview host passes `panelId`, while `WorkflowInspectorPane` declares and emits `paneId`. Runtime pane-layout APIs expect `panelId`. Split/open actions can therefore target an undefined or wrong pane id.

Relevant code:

- `src/renderer/components/DockviewPanelHost.svelte`
- `src/renderer/components/WorkflowInspectorPane.svelte`
- `src/renderer/pane-layout.ts`

**Best fix:** Standardize on `panelId` end-to-end for Dockview panel placement. Rename component props and emitted payload fields to match the runtime contract.

**Verification required:**

- Workflow inspector split action opens beside the intended panel.
- Type-level coverage prevents `paneId` from being passed where `panelId` is required.

**Confidence:** High.

#### AUD-034E - App-log related links are incomplete

**Impact:** Low-medium observability/navigation issue.

**Precise issue:** App-log related metadata supports some ids and the pane can open some related resources, but surface/thread links are display-only and artifact ids are absent from the related-link model.

Relevant code:

- `src/bun/workspace-contract.ts`: app-log related id model.
- `src/renderer/components/AppLogsPane.svelte`: renders and opens related links.

**Best fix:** Expand related metadata/actions to cover artifact, thread, surface, session, workflow, task, and command ids consistently. Route each through static pane targets where applicable, and make related-link search index those ids.

**Verification required:**

- App-log rows with session/workflow/task/command/thread/surface/artifact metadata render links.
- Each link opens the expected target.
- Search can find logs by related artifact/thread/surface ids.

**Confidence:** High for incomplete related-link coverage.

#### AUD-034F - Double-click rename appears implemented but lacks confirming coverage

**Impact:** Low-medium regression risk.

**Precise issue:** An audit item mentioned double-click rename. Source inspection shows double-click rename behavior appears implemented in the session list item, so this is not a confirmed product bug. The extracted issue is missing or insufficient coverage.

Relevant code:

- `src/renderer/components/SessionListItem.svelte`

**Best fix:** Add a focused component/e2e test that double-clicking a session title enters rename mode, commit updates the session name, and cancel leaves it unchanged.

**Verification required:** The new test should fail if double-click rename is removed or stops dispatching the rename action.

**Confidence:** Medium. The behavior appears present; the actionable gap is coverage.
