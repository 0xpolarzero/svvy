export type FeatureStatus = "shipped" | "in-progress";

export interface ProductFeature {
  id: string;
  name: string;
  status: FeatureStatus;
  summary: string;
  sourceSpecs: string[];
}

export const PRODUCT_FEATURES: ProductFeature[] = [
  {
    id: "desktop-shell",
    name: "Electrobun Desktop Shell",
    status: "shipped",
    summary: "Runs svvy as a native desktop coding app with a Bun-side pi host and renderer shell.",
    sourceSpecs: ["docs/prd.md"],
  },
  {
    id: "provider-auth",
    name: "Provider Auth And Settings",
    status: "shipped",
    summary:
      "Manages provider keys, OAuth-backed access, and app-level preferences such as the user's preferred external editor through TanStack Form-backed desktop settings surfaces with local validation, dirty state, reset/cancel behavior, and backend-authoritative persistence.",
    sourceSpecs: ["docs/prd.md"],
  },
  {
    id: "true-system-prompt-channel",
    name: "True System Prompt Channel",
    status: "in-progress",
    summary:
      "Loads svvy's orchestrator and handler-thread instructions through pi's real `systemPrompt` channel, sends new user input as real pi user messages without flattened transcript reconstruction or hidden durable state prose, slices generated capability declarations by actor so each surface sees only its own callable API, and renders the active system prompt as expandable surface metadata instead of inline transcript text.",
    sourceSpecs: ["docs/prd.md"],
  },
  {
    id: "artifacts-panel",
    name: "Artifacts Projection",
    status: "shipped",
    summary: "Presents generated artifacts in a docked preview panel.",
    sourceSpecs: ["docs/prd.md", "docs/specs/structured-session-state.spec.md"],
  },
  {
    id: "durable-artifact-storage",
    name: "Durable Artifact Storage",
    status: "in-progress",
    summary:
      "Moves artifacts into a dedicated workspace artifact directory with SQLite metadata and path indexing, including submitted execute_typescript snippets for every attempt and workflow-related logs and exports.",
    sourceSpecs: ["docs/prd.md", "docs/specs/structured-session-state.spec.md"],
  },
  {
    id: "execute-typescript-surface",
    name: "Direct Tools And Execute Typescript",
    status: "in-progress",
    summary:
      "Provides native direct tools as the default coding-agent work surface, with svvy recording around semantic cx navigation, text and image reads, searches, edits, writes, bash commands, artifacts, workflow discovery, provider-backed web search and fetch when a keyed provider is ready, and actor-local tool-surface inspection through `list_tools`; explicitly teaches agents to issue independent tool calls together so pi can run them in parallel; keeps execute_typescript as a typed composition tool with a generated JSDoc-rich declaration for read/search/bash/artifact/workflow, optional provider-shaped web helpers when ready, and read-only api.cx calls embedded in the system prompt, preflight typecheck or compile diagnostics, file-backed snippet artifacts for every attempt, and parent-first rollups over nested child command facts.",
    sourceSpecs: [
      "docs/prd.md",
      "docs/specs/execute-typescript.spec.md",
      "docs/specs/web-tools.spec.md",
    ],
  },
  {
    id: "web-tool-surface",
    name: "Provider-Backed Web Tools",
    status: "shipped",
    summary:
      "Lets users choose TinyFish or Firecrawl as the active keyed web provider in settings, store provider API keys there, expose provider-shaped `web.search` and deterministic artifact-backed `web.fetch` tools plus generated `api.web` composition helpers only when the selected provider is ready, use TinyFish's official TypeScript SDK for TinyFish Search and Fetch contracts and runtime calls, regenerate prompt context and tool declarations cleanly on provider changes, and keep the default product state web-disabled with no no-key Local fallback.",
    sourceSpecs: ["docs/specs/web-tools.spec.md"],
  },
  {
    id: "handler-thread-surfaces",
    name: "Delegated Handler Thread Surfaces",
    status: "in-progress",
    summary:
      "Lets the orchestrator open pi-backed delegated handler threads as fully interactive conversation surfaces that supervise one delegated objective, immediately start the handler's first turn from the raw objective, optionally preload prompt context keys such as `ci`, derive handler-thread titles through the configured `namer` from the supplied objective instead of exposing a separate title field to the orchestrator, stay multi-turn and directly messageable before and after handoff, expose runtime and thread state through `runtime.current`, `thread.current`, `thread.list`, and `thread.handoffs`, distinguish idle, handler-active, workflow-active, waiting, troubleshooting, and completed thread states, reject `thread.handoff` while the thread still owns a running or waiting workflow run for the current span, route workflow attention back to the owning handler surface rather than the focused Dockview panel, can be inspected on demand without becoming the default reconciliation path, and return control to the orchestrator only through explicit `thread.handoff` calls that append ordered handoff episodes over the thread's lifetime and immediately trigger a fresh orchestrator reconciliation turn.",
    sourceSpecs: ["docs/prd.md", "docs/specs/structured-session-state.spec.md"],
  },
  {
    id: "prompt-contexts",
    name: "Prompt Context Packs",
    status: "in-progress",
    summary:
      "Keeps product prompt knowledge modular with always-loaded cx semantic-navigation context, actor-specific always-loaded Smithers context, settings-derived always-loaded web provider context, and optional handler-only `ci` context loaded through `thread.start({ context })` or `request_context({ keys })`, with loaded optional keys persisted on the handler thread for resume and no prompt-context loading through the `execute_typescript` `api.*` SDK.",
    sourceSpecs: [
      "docs/prd.md",
      "docs/specs/prompt-contexts.spec.md",
      "docs/specs/web-tools.spec.md",
      "docs/specs/project-ci.spec.md",
      "docs/specs/structured-session-state.spec.md",
    ],
  },
  {
    id: "smithers-tool-surface",
    name: "Smithers-Native Tool Surface",
    status: "in-progress",
    summary:
      "Exposes Smithers-native semantic workflow control and inspection tools through the Bun bridge for handler-thread surfaces, with normal startup discovering only configured saved entries under `.svvy/workflows/entries/` and artifact entries under `.svvy/artifacts/workflows/`, a stable `smithers.run_workflow({ workflowId, input, runId? })` launch-or-resume tool validated against each entry's real TypeScript or Zod launch schema, `smithers.list_workflows({ workflowId?, productKind? })` returning full runnable-entry contract metadata including `workflowId`, `label`, `summary`, `sourceScope`, `entryPath`, grouped asset refs, derived `assetPaths`, `launchInputSchema`, and optional product metadata such as Project CI `productKind` and result schema, `smithers.list_runs` returning workspace-global compact run summaries enriched with svvy `sessionId` and `threadId` ownership when known, and the rest of the handler-thread surface preserving official Smithers names such as `get_run`, `watch_run`, `explain_run`, `list_pending_approvals`, `resolve_approval`, `get_node_detail`, `list_artifacts`, `get_chat_transcript`, `get_run_events`, `runs.cancel`, `signals.send`, `frames.list`, `getDevToolsSnapshot`, and `streamDevTools` instead of inventing a parallel svvy `workflow.*` abstraction, while preserving transport and invocation metadata in command facts, returning an empty workflow list when no real entries are configured, and avoiding any dependency on the repo authoring workspace under `workflows/`.",
    sourceSpecs: ["docs/prd.md", "docs/specs/workflow-supervision.spec.md"],
  },
  {
    id: "workflow-task-agents",
    name: "Workflow Task Agents",
    status: "in-progress",
    summary:
      "Defines lower-level Smithers workflow task agents as a separate actor class beneath handler threads, using a PI-backed svvy task configuration with a task-local direct-tool callable surface including cx semantic navigation, `list_tools`, plus `execute_typescript`, no ambient pi built-ins or extension-tool leakage, task-root or worktree execution aligned to the active Smithers attempt, first-class durable workflow-task-attempt records keyed by Smithers attempt identity before task-local tool calls run, message-native retry and hijack continuation, live task-agent activity streaming, and projected nested transcript, command, artifact, and usage traces, while keeping approval and hijack as Smithers runtime controls rather than ordinary task-agent tools.",
    sourceSpecs: [
      "docs/prd.md",
      "docs/specs/workflow-supervision.spec.md",
      "docs/specs/execute-typescript.spec.md",
    ],
  },
  {
    id: "context-budget-observability",
    name: "Context Budget Observability",
    status: "shipped",
    summary:
      "Shows active context usage as a percentage of the current model's maximum for orchestrator surfaces, handler-thread surfaces, and workflow task-agent attempts, with neutral below 40%, orange from 40%, and red from 60% so context pressure is visible without treating any single percentage as a universal model failure point.",
    sourceSpecs: ["docs/prd.md", "docs/specs/context-budget-observability.spec.md"],
  },
  {
    id: "workflow-library",
    name: "Workflow Authoring And Artifact Workflows",
    status: "in-progress",
    summary:
      "Centers workflow execution around authored artifact workflows stored under `.svvy/artifacts/workflows/`, with every handler thread receiving generated workflow-authoring TypeScript contracts plus always-loaded Smithers prompt context, checking saved entries and reusable assets before authoring, authoring through reusable definitions, prompts, and components when needed, and launching concrete saved or artifact entries through the Smithers-native runtime surface.",
    sourceSpecs: ["docs/prd.md", "docs/specs/workflow-library.spec.md"],
  },
  {
    id: "saved-workflow-library",
    name: "Workspace Saved Workflow Library",
    status: "shipped",
    summary:
      "Stores reusable workflow source assets under `.svvy/workflows/definitions`, `prompts`, and `components`, stores launchable saved entries under `.svvy/workflows/entries`, exposes minimal asset index metadata from required JSDoc and MDX frontmatter, keeps conventional workflow agents such as `explorer`, `implementer`, and `reviewer` as ordinary component exports in `.svvy/workflows/components/agents.ts`, supports optional product metadata and result schemas on entries such as Project CI, lets handlers list asset paths through `workflow.list_assets` and read saved asset source through direct file tools, validates writes under `.svvy/workflows/...` automatically through structured tool output, and presents a read-only library surface with source previews, diagnostics, deletion controls, and open-in-editor handoff to the user's configured external editor.",
    sourceSpecs: ["docs/prd.md", "docs/specs/workflow-library.spec.md"],
  },
  {
    id: "prompt-history",
    name: "Workspace Prompt History",
    status: "shipped",
    summary:
      "Stores non-empty submitted prompts per workspace, including failed and provider-blocked attempts, and exposes shell-like recall in the composer.",
    sourceSpecs: ["docs/specs/prompt-history.spec.md"],
  },
  {
    id: "composer-mention-links",
    name: "Composer Mention Links",
    status: "shipped",
    summary:
      "Lets the composer autocomplete indexed workspace files and folders after `@`, attach files or folders through the native paperclip picker, insert removable attachment chips, serialize selected context as ordinary `@path` user text, and render sent transcript mentions as actionable workspace links that reveal files, open folders, and visibly mark missing paths without prompt injection, eager file reads, folder expansion, or a special context-target model.",
    sourceSpecs: ["docs/specs/composer-mention-links.spec.md", "docs/progress.md"],
  },
  {
    id: "assistant-markdown-rendering",
    name: "Assistant Markdown Rendering",
    status: "shipped",
    summary:
      "Renders streamed assistant transcript Markdown inside a TanStack Virtual transcript surface with compact prose spacing, reliable list markers, GitHub-style tables and task lists, syntax-highlighted fenced code blocks with copy actions, inline and display math through KaTeX, Mermaid diagrams rendered as SVG with source copy fallback, escaped raw HTML so assistant output cannot inject executable markup, variable-height row measurement, pane-local scroll restoration, and bottom-following only while the user is pinned there.",
    sourceSpecs: ["docs/prd.md"],
  },
  {
    id: "workspace-navigation-core-projection",
    name: "Workspace Navigation And Core Projection",
    status: "shipped",
    summary:
      "Keeps each workspace tab navigable with pinned active sessions, durable session-level unread dots that appear when assistant turns finish outside the focused pane surface and clear on session-pane focus, a single collapsed-by-default Archived group for non-destructive session hiding, layered sidebar rows where orchestrator session state, handler-thread state, and workflow-run state stay local to their owning rows, session row context menus for mark unread, pin, rename, and archive actions, compact running indicators, tone-aware open-pane highlighting, context-budget rails for open orchestrator and handler rows, compact thread and workflow-run artifact blocks backed by durable artifact records, compact latest Project CI projection near the focused surface or relevant handler thread, and restart restoration for stable Dockview panel bindings, focus, panel-local scroll, display preferences, and inspector selections while deliberately excluding transient UI, composer drafts, transcript selections, and stale live stream state.",
    sourceSpecs: [
      "docs/prd.md",
      "docs/specs/pane-layout.spec.md",
      "docs/specs/workspace-navigation-core-projection.spec.md",
      "docs/specs/multi-session-support.spec.md",
      "docs/specs/structured-session-state.spec.md",
      "docs/specs/project-ci.spec.md",
    ],
  },
  {
    id: "command-palette",
    name: "Command Palette And Quick Open",
    status: "in-progress",
    summary:
      "Defines a VS Code-like shared palette where `Cmd+Shift+P` opens the same input as `Cmd+P` with `>` prefilled, those launcher chords remain available while text inputs are focused and switch the focused palette between command and quick-open modes when it is already open, the leading `>` live-switches quick-open search into command/action mode, command mode discovers and executes product actions through existing session, surface, orchestrator, handler-thread, workflow task-agent projection, Project CI, Smithers-native, Dockview panel, settings, and agent-setting routing, a product shortcut registry backed by TanStack Hotkeys owns scoped renderer dispatch, input policy, and shared shortcut display, sidebar shell actions reveal compact shortcut hints instantly on hover or focus, icon-only or ambiguous action controls show faster delayed explanatory tooltips with consistent keycap chips, open-session results show visually distinct kind badges across orchestrator, handler-thread, and task-agent categories, `Cmd+P` remains a file quick-open placeholder until file surfaces exist, `cmdk-sv` is the intended Svelte UI primitive, and unmatched non-empty command-mode text creates a normal new session initial prompt without the `>` prefix or a parallel runtime, shell, terminal loop, or workflow abstraction.",
    sourceSpecs: ["docs/prd.md", "docs/specs/command-palette.spec.md"],
  },
  {
    id: "session-agent-settings",
    name: "Session Agent And Workflow Agent Settings",
    status: "shipped",
    summary:
      "Persists app-wide default-session, dumb-orchestrator, and namer session-agent defaults with provider, model, reasoning, and system prompt; records session mode and prompt selection on created sessions; uses the same configured `namer` for top-level session titles and handler-thread titles derived from delegated objectives; allows handler threads to carry per-thread model, reasoning, and prompt overrides through `thread.start`; exposes focused-surface agent summaries in pane chrome and TanStack Form-backed direct-saving settings inspection/editing for session agents and conventional workflow agents through connected-provider model dropdowns plus selected-model reasoning dropdowns, with validation, dirty state, reset, pending, and async error state; and synchronizes `explorer`, `implementer`, and `reviewer` workflow-agent settings to `.svvy/workflows/components/agents.ts` as a normal saved component asset.",
    sourceSpecs: [
      "docs/prd.md",
      "docs/specs/structured-session-state.spec.md",
      "docs/specs/workflow-library.spec.md",
      "docs/specs/command-palette.spec.md",
    ],
  },
  {
    id: "multi-session-support",
    name: "Multi-Session Workspace Navigation",
    status: "shipped",
    summary:
      "Supports creating, listing, switching, renaming, forking, pinning, and archiving multiple pi-backed session containers from one workspace window, with archive serving as the user-facing hide/delete equivalent while preserving session history, top-level session auto-titling owned by a durable one-shot namer flow that starts concurrently with the first orchestrator turn, the namer settings prompt as the sole naming instruction, manual rename blocked while title generation is pending or running, titles frozen after manual rename or the first successful generated title, and delegated handler titles owned by the same namer flow over the handler objective rather than by an orchestrator-supplied title.",
    sourceSpecs: ["docs/specs/multi-session-support.spec.md"],
  },
  {
    id: "multi-surface-runtime",
    name: "Multi-Surface Live Runtime",
    status: "in-progress",
    summary:
      "Separates integrated app-chrome workspace tabs, durable workspace state, live surface runtimes, and Dockview-backed layout state, using one backend workspace runtime per open tab and explicit runtime `workspaceId` routing for workspace-scoped requests and sync events; allows opening the same cwd in multiple independent workspace tabs; keeps workspace tabs left-aligned at the start of the main chrome, horizontally scrollable when crowded, draggable for user reordering, durably restored in user-defined order, and paired with compact icon controls plus colored running, unread, waiting, and error count badges shown only above zero with hover context; uses Dockview core for panels, groups, tabs, tab groups, splitters, drag/drop overlays, edge groups, floating groups, popouts, and serialized layout restore inside fixed workspace layout slots A, B, and C pinned at the far right while svvy stores panel-to-surface bindings and panel-local metadata; keeps empty layout slots muted but selectable, manages live pi surfaces in a shared registry keyed by `surfacePiSessionId`, gives each surface its own prompt lock, model or reasoning lifecycle, pending user message, and surface-owned live assistant stream state, supports explicit open and close semantics, sidebar panel-location indicators, compact thread and workflow-run projections, and lets zero, one, or multiple panels attach to the same streaming surface without duplicating or cancelling the underlying runtime while keeping panel-local scroll independent.",
    sourceSpecs: [
      "docs/prd.md",
      "docs/specs/multi-session-support.spec.md",
      "docs/specs/structured-session-state.spec.md",
      "docs/specs/pane-layout.spec.md",
      "docs/specs/workspace-navigation-core-projection.spec.md",
    ],
  },
  {
    id: "structured-session-state",
    name: "Structured Session State Overlay",
    status: "in-progress",
    summary:
      "Adds a workspace-scoped svvy-owned state layer above pi and Smithers with durable session, turn, thread, workflow-run, workflow-task-attempt, command, episode, artifact, Project CI run/check result, wait, and lifecycle event records, explicit surface-target identity (`workspaceSessionId`, `surfacePiSessionId`, `threadId`), workflow-task-attempt binding bootstrapped by exact persisted Smithers resume-handle lookup instead of heuristic attempt-table scans or transcript-derived repair, workspace-level metadata projection that survives reload, and live-surface transcript updates kept separate from durable workspace read models.",
    sourceSpecs: ["docs/specs/structured-session-state.spec.md"],
  },
  {
    id: "turn-command-state",
    name: "Turn And Command State",
    status: "in-progress",
    summary:
      "Tracks every turn on the orchestrator surface and handler thread surfaces, including each turn's top-level turn decision, plus every tool call including execute_typescript snippets and nested api.* child command facts, as durable state with lifecycle status, ownership, linkage, attempts, and trace-versus-surface visibility.",
    sourceSpecs: ["docs/specs/structured-session-state.spec.md"],
  },
  {
    id: "session-threads",
    name: "Structured Handler Threads",
    status: "in-progress",
    summary:
      "Tracks delegated handler threads as durable interactive surfaces keyed separately from workspace session containers and pi surface ids, with objective, handler-attention status, wait state, worktree context, and linkage to multiple workflow runs and multiple handoff episodes over the thread's lifetime without flattening workflow outcome into thread terminal state.",
    sourceSpecs: ["docs/specs/structured-session-state.spec.md"],
  },
  {
    id: "durable-episodes",
    name: "Durable Episodes",
    status: "in-progress",
    summary:
      "Stores reusable semantic outputs as first-class episode records, with handler threads able to emit multiple ordered handoff episodes over their lifetime through explicit `thread.handoff` calls as the semantic half of returning control to the orchestrator, including orchestrator-local episodes when substantive local work completes, while ordinary tool runs keep their own command summaries and artifacts.",
    sourceSpecs: ["docs/specs/structured-session-state.spec.md"],
  },
  {
    id: "project-ci-lane",
    name: "Project CI Lane",
    status: "in-progress",
    summary:
      'Provides Project CI status and result projection over normal saved Smithers entries under `.svvy/workflows/.../ci/`, records CI run and CI check result state only from entries declaring `productKind = "project-ci"` whose terminal output validates against the declared result schema, exposes latest CI status in specialized UI, and delivers CI authoring guidance only through the optional `ci` prompt context loaded by `thread.start({ context: ["ci"] })` or handler-side `request_context({ keys: ["ci"] })`, without a setup launcher, CI-specific orchestrator, or shipped placeholder CI entry.',
    sourceSpecs: [
      "docs/specs/project-ci.spec.md",
      "docs/specs/prompt-contexts.spec.md",
      "docs/specs/structured-session-state.spec.md",
      "docs/specs/workspace-navigation-core-projection.spec.md",
      "docs/specs/workflow-library.spec.md",
      "docs/specs/workflow-supervision.spec.md",
    ],
  },
  {
    id: "workflow-run-records",
    name: "Delegated Workflow Run Records",
    status: "in-progress",
    summary:
      "Stores one svvy-side record for each Smithers workflow run under a handler thread, including run identity, workflow source, runnable entry path plus saved-entry linkage when relevant, normalized status, raw Smithers status, wait kind, reconnect cursor, pending-versus-delivered handler-attention cursors, heartbeat freshness, lineage, summary, timestamps, and related artifacts and command history, with lifecycle projection owned by explicit bridge-event writes, Smithers tool-boundary writes, and reconnect or bootstrap control-plane writes, plus idempotent terminal replay handling so duplicate terminal snapshots do not reopen a completed thread or redeliver handler attention.",
    sourceSpecs: ["docs/specs/structured-session-state.spec.md"],
  },
  {
    id: "session-wait-state",
    name: "Session And Thread Wait State",
    status: "in-progress",
    summary:
      "Represents handler-owned and workflow-owned blocking conditions explicitly through surface-local wait state and whole-session frontier wait state, preserving whether a wait came from user input, approval, signal, timer, or other external dependency without inventing wait episodes or relying on transcript inference.",
    sourceSpecs: ["docs/specs/structured-session-state.spec.md"],
  },
  {
    id: "session-summary-read-models",
    name: "Metadata-First Session Read Models",
    status: "in-progress",
    summary:
      "Derives orchestrator-local idle, running, waiting, and error session status, pinned and archived navigation fields, row-local handler-thread and workflow-run sidebar projections, pending attention, and compact summary data from structured state for workspace navigation and restart recovery without rolling child handler or workflow lifecycle state into the parent session row, transcript replay, transcript-file heuristics, or any global active-surface overlay.",
    sourceSpecs: [
      "docs/specs/structured-session-state.spec.md",
      "docs/specs/workspace-navigation-core-projection.spec.md",
    ],
  },
  {
    id: "workflow-inspector",
    name: "Workflow Inspector Surface",
    status: "shipped",
    summary:
      "Provides a durable tree-first Dockview panel surface for Smithers runs, modeled after React DevTools and the Smithers GUI live-run tree, with searchable expandable rows, selected and expanded node state, normalized svvy status beside raw Smithers status, launch arguments and props, Smithers DevTools snapshot and event-cursor streaming, historical frame inspection, selected-node status, output, partial output, artifact, workflow-agent, task-attempt, command, worktree, timing, wait-reason, output/diff/log/transcript/command/event/raw detail, Project CI check rows only for declared CI entries, and related handler-thread, task-agent, command, CI check, and artifact Dockview targets without forcing the orchestrator to absorb raw workflow history.",
    sourceSpecs: [
      "docs/prd.md",
      "docs/specs/workflow-supervision.spec.md",
      "docs/specs/workflow-inspector.spec.md",
    ],
  },
  {
    id: "app-logs-surface",
    name: "App Logs Surface",
    status: "shipped",
    summary:
      "Provides workspace-scoped product observability through structured, redacted, persisted app logs with monotonic sequence numbers, unread state, live renderer updates, a sidebar Logs entry with per-level unread counts, and a dense Dockview logs pane with TanStack Virtual long-scroll rendering, older-page loading, level/source/search filtering, mark-read behavior, explicit Live/Frozen tail control, expandable details, normalized errors, stack traces, and related session, surface, thread, workflow, task, command, and artifact ids without making logs canonical product state.",
    sourceSpecs: ["docs/specs/app-logs.spec.md"],
  },
];
