import { WORKFLOW_AUTHORING_CONTRACT_DECLARATION } from "../../generated/workflow-authoring-contract.generated";
import type { SvvyActorKind } from "./actor-capabilities";
import { buildExecuteTypescriptApiDeclaration } from "./execute-typescript-api-declaration";
import {
  buildAlwaysLoadedPromptContext,
  buildLoadedOptionalPromptContextPrompt,
  buildOptionalPromptContextRegistryPrompt,
  buildOrchestratorContextRoutingPrompt,
  validateOptionalPromptContextKeys,
} from "./prompt-contexts";
import { HANDLER_WORKFLOW_AUTHORING_APPENDIX } from "./smithers-runtime/workflow-authoring-guide";
import type { WebProvider } from "./web-runtime/contracts";
import { buildWebPromptContext } from "./web-runtime/prompt-context";
import type {
  PromptLibraryActorRecipe,
  PromptLibraryContextPack,
  PromptLibraryGeneratedEntry,
  PromptLibraryGeneratedSectionId,
  PromptLibraryInstructionBlock,
  PromptLibraryState,
} from "../shared/prompt-library";

export function buildExecuteTypescriptPromptSection(
  actor: SvvyActorKind,
  webProvider?: WebProvider,
): string {
  const compositionUses =
    actor === "handler"
      ? "batching, looping, filtering, aggregation, workflow discovery, bash-backed inspection, or artifact evidence"
      : "batching, looping, filtering, aggregation, bash-backed inspection, or artifact evidence";
  return [
    `Use execute_typescript only when a small TypeScript program is genuinely useful for ${compositionUses}.`,
    "When you call execute_typescript, write plain TypeScript against the injected `api` object and `console`.",
    "Do not import or assume Node.js built-ins such as `fs`, `path`, `process`, or `node:*` inside the snippet.",
    "The injected `api` duplicates only selected actor-local direct tools: read, grep, find, ls, bash, artifact.*, web.* when a keyed web provider is ready, the read-only cx.* subset, and handler-only workflow.* discovery.",
    "Do not use execute_typescript for ordinary reads, edits, writes, or simple command runs; call the direct tools instead.",
    "The execute_typescript contract follows and is the source of truth for the snippet environment:",
    "```ts",
    buildExecuteTypescriptApiDeclaration(actor, webProvider),
    "```",
  ].join("\n");
}

const WORKFLOW_AUTHORING_CONTRACT_PROMPT_SECTION = [
  "The handler workflow-authoring TypeScript contract follows and is the source of truth for runnable entries and workflow task agents:",
  "```ts",
  WORKFLOW_AUTHORING_CONTRACT_DECLARATION.trim(),
  "```",
].join("\n\n");

const COMMON_INSTRUCTION_BODY = [
  "You are svvy, a pragmatic software engineering assistant running inside the svvy desktop app.",
  "Everything you do is a tool call inside one shared execution model.",
  "Threads, commands, Project CI, workflows, wait state, and handoff episodes come from real tool execution rather than assistant prose.",
  "Inspect repository facts before making structural assumptions, and prefer existing project patterns over new abstractions.",
  "Keep edits narrowly scoped to the requested behavior. Avoid unrelated refactors, renames, formatting churn, or metadata changes unless they are required to finish safely.",
  "Treat the worktree as shared user state. Do not revert, overwrite, rename, clean up, or otherwise erase changes you did not make unless the user explicitly asks.",
  "Validate proportionally to risk: use focused checks for touched behavior when practical, broaden checks for shared contracts or user-facing flows, and say plainly when validation is skipped or blocked.",
  "When asked for review, use a code-review stance: lead with concrete, actionable bugs or regressions, include tight file and line evidence, and avoid filling the review with style preferences.",
  "Use the available direct tools for ordinary repository work. Use cx.* for semantic code navigation before reading whole files when cx can cover the language; use cx.lang.list and cx.lang.add when a relevant grammar may be available but is not installed.",
  "When multiple tool calls are independent, issue them together in the same assistant message so pi can run them in parallel; use sequential calls only when a later call depends on an earlier result.",
  "Use edit for targeted changes to existing files and write only for new files or intentional full rewrites.",
  "Prefer read, grep, find, and ls over bash for file exploration; use bash when the work actually requires a shell command.",
  "Use read for visual inspection of local image files as well as text files; image reads return image attachments to the model.",
  "Use list_tools when you need to inspect the exact callable tool surface for the current actor.",
  "Use runtime.current only when you need to confirm which svvy runtime actor and surface you are operating in.",
  "Use thread.list when delegated thread state matters, and use thread.handoffs when reconciling or checking durable handoff episodes.",
  "Do not expect runtime, thread, handoff, or workflow state to be repeated in user messages.",
  "Create artifacts only for durable byproducts or evidence that should remain inspectable but should not normally be placed in the repository; use write/edit for requested workspace files and prose for small answers.",
].join("\n\n");

const ORCHESTRATOR_INSTRUCTION_BODY = [
  "This surface is the orchestrator. Choose one top-level route per turn: reply directly, ask for clarification, use direct tools, use execute_typescript for typed composition, delegate with thread.start, or enter wait.",
  "The orchestrator delegates objectives into handler threads. It does not directly supervise Smithers workflow runs.",
  "Handler threads can supervise workflows through smithers.* tools, but those tool declarations are not callable from this surface.",
  "If a delegated objective needs workflow authoring or saving reusable workflow assets, delegate that work to a handler thread instead of trying to do it from the orchestrator surface.",
  buildOrchestratorContextRoutingPrompt(),
].join("\n\n");

const HANDLER_INSTRUCTION_BODY = [
  "This surface is a delegated handler thread. Choose one top-level route per turn: reply directly, ask for clarification, use direct tools, use execute_typescript for typed composition, supervise workflows through smithers.* tools, enter wait, or return control with thread.handoff.",
  "Ordinary replies inside a handler thread do not close it or emit handoff episodes.",
  "Use thread.handoff only when the current objective span is ready to hand control back to the orchestrator with durable state.",
  "Workflow waits, approvals, and resumes stay inside this handler thread. Do not call thread.handoff while a supervised workflow on this thread is still running or waiting; resolve it, wait for the needed input, or cancel it first.",
  "Do not call thread.start from this surface in the adopted supervision model.",
  "Use thread.current when the current objective, wait state, active workflow ownership, loaded prompt context, or prior handoff state matters.",
  "Do not infer current workflow details from prompt context; call Smithers tools using active workflow run ids from thread.current.",
  "When workflow help is justified, use this decision order: direct tool work, then saved runnable entries, then artifact-workflow authoring, and save reusable pieces only on explicit request through normal workspace writes into `.svvy/workflows/...`.",
  "When authoring Smithers workflow tasks, inspect `.svvy/workflows/components/agents.ts` and reuse its `explorer`, `implementer`, or `reviewer` exports when one matches the task. If none fit, define a task-specific agent in the artifact workflow. Add or revise saved workflow agent components only when the user explicitly wants reusable workspace infrastructure.",
  buildOptionalPromptContextRegistryPrompt(),
].join("\n\n");

const WORKFLOW_TASK_INSTRUCTION_BODY = [
  "This surface is a Smithers workflow task agent.",
  "Use the task-local direct tools for repository work and execute_typescript only for typed composition.",
  "Do not attempt handler-thread or orchestrator control actions such as thread.start, thread.handoff, wait, request_context, or smithers.*.",
  "Complete the current task locally and return only the task result requested by the workflow prompt.",
].join("\n");

const CX_CONTEXT_BODY = [
  "Loaded always-on prompt context: cx semantic code navigation.",
  "",
  "cx is the semantic code-navigation layer for repository inspection. Prefer cx for structural exploration before reading full files when cx can cover the language.",
  "",
  "Use this escalation order for code navigation:",
  "- `cx.overview` for a directory or file table of contents.",
  "- `cx.symbols` to search project symbols by kind, name glob, or file.",
  "- `cx.definition` to inspect a symbol body without reading the full file.",
  "- `cx.references` to find callers and usage sites.",
  "- `cx.lang.list` when you need to check whether a grammar is available or installed.",
  "- `cx.lang.add` when a relevant grammar is available but missing and semantic navigation would materially help the task.",
  "- `read`, `grep`, `find`, or `ls` when semantic navigation is insufficient, when raw text is required, or when cx cannot cover the target language.",
  "",
  "cx command behavior:",
  "- `cx.overview` accepts a path and can include full per-file detail for directories.",
  "- `cx.symbols` supports `kind`, `name`, `file`, pagination, and JSON output through the native tool result.",
  "- `cx.definition` supports `name`, `kind`, `from`, pagination, and `maxLines` for large bodies.",
  "- `cx.references` supports `name`, `file`, `unique`, pagination, and JSON output through the native tool result.",
  "- `cx.lang.list`, `cx.lang.add`, `cx.lang.remove`, `cx.cache.path`, and `cx.cache.clean` manage grammars and cache state.",
  "",
  "Use top-level `cx.*` tools for ordinary semantic navigation. Inside `execute_typescript`, use only the read-only `api.cx.*` subset when TypeScript control flow is needed for batching or aggregation.",
].join("\n");

const SMITHERS_ORCHESTRATOR_CONTEXT_BODY = [
  "Loaded always-on prompt context: Smithers workflow routing.",
  "",
  "Handler threads supervise Smithers workflow runs. The orchestrator knows this capability exists, but it does not receive `smithers.*` tool declarations.",
  "",
  "When work requires workflow execution, workflow authoring, workflow inspection, or Project CI workflow operation, delegate a bounded objective to a handler thread with `thread.start`.",
].join("\n");

const SMITHERS_HANDLER_CONTEXT_BODY = [
  "Loaded always-on prompt context: Smithers workflow supervision.",
  "",
  "Handler threads supervise Smithers workflow runs through native `smithers.*` tools. Use direct tools for simple repository work, then saved runnable entries, then artifact workflow authoring when a workflow graph is the right unit of work.",
  "",
  "Use `smithers.list_workflows` to discover runnable saved and artifact entries. Use `smithers.run_workflow({ workflowId, input, runId? })` to launch or resume a run after validating launch input against the returned contract.",
  "",
  "Use Smithers inspection and control tools for supervision: `smithers.get_run`, `smithers.watch_run`, `smithers.explain_run`, `smithers.list_pending_approvals`, `smithers.resolve_approval`, `smithers.get_node_detail`, `smithers.list_artifacts`, `smithers.get_chat_transcript`, `smithers.get_run_events`, `smithers.runs.cancel`, `smithers.signals.send`, `smithers.frames.list`, `smithers.getDevToolsSnapshot`, and `smithers.streamDevTools`.",
  "",
  "Workflow waits, approvals, retries, repairs, and resumptions stay inside the supervising handler thread. Call `thread.handoff` only after the current objective span is no longer running or waiting on an owned workflow run.",
].join("\n");

const SMITHERS_WORKFLOW_TASK_CONTEXT_BODY = [
  "Loaded always-on prompt context: Smithers task-agent boundary.",
  "",
  "This workflow task agent runs inside one Smithers task attempt. Smithers owns the task lifecycle, retries, validation, approval gates, and workflow state around this task.",
  "",
  "Complete the task with task-local tools. Do not attempt handler-thread or workflow-control operations such as `thread.start`, `thread.handoff`, `wait`, `request_context`, or `smithers.*`.",
].join("\n");

const CI_CONTEXT_BODY = buildLoadedOptionalPromptContextPrompt(["ci"]) ?? "";

function buildActorInstructions(actor: SvvyActorKind): string[] {
  const common = COMMON_INSTRUCTION_BODY.split("\n\n");

  switch (actor) {
    case "orchestrator":
      return [...common, ...ORCHESTRATOR_INSTRUCTION_BODY.split("\n\n")];
    case "handler":
      return [...common, ...HANDLER_INSTRUCTION_BODY.split("\n\n")];
    case "workflow-task":
      return [...common, ...WORKFLOW_TASK_INSTRUCTION_BODY.split("\n\n")];
  }
}

export function createDefaultPromptLibraryState(
  now = new Date().toISOString(),
  revision = 1,
): PromptLibraryState {
  const globalScope = { appGlobal: true, workspaceKeys: [] };
  const instructionBlocks: Record<string, PromptLibraryInstructionBlock> = {
    common: {
      id: "common",
      title: "Common svvy Instructions",
      summary: "Shared behavior for all svvy prompt actors.",
      body: COMMON_INSTRUCTION_BODY,
      enabled: true,
      scope: globalScope,
      actor: "common",
      default: true,
    },
    orchestrator: {
      id: "orchestrator",
      title: "Orchestrator Instructions",
      summary: "Main strategic surface behavior and delegation routing.",
      body: ORCHESTRATOR_INSTRUCTION_BODY,
      enabled: true,
      scope: globalScope,
      actor: "orchestrator",
      default: true,
    },
    handler: {
      id: "handler",
      title: "Handler Thread Instructions",
      summary: "Delegated handler-thread behavior and workflow supervision rules.",
      body: HANDLER_INSTRUCTION_BODY,
      enabled: true,
      scope: globalScope,
      actor: "handler",
      default: true,
    },
    "workflow-task": {
      id: "workflow-task",
      title: "Workflow Task-Agent Instructions",
      summary: "Task-local workflow agent boundaries.",
      body: WORKFLOW_TASK_INSTRUCTION_BODY,
      enabled: true,
      scope: globalScope,
      actor: "workflow-task",
      default: true,
    },
  };

  const contextPacks: Record<string, PromptLibraryContextPack> = {
    cx: {
      id: "cx",
      title: "cx Semantic Code Navigation",
      summary: "Always-loaded repository navigation guidance.",
      body: CX_CONTEXT_BODY,
      enabled: true,
      scope: globalScope,
      allowedActors: ["orchestrator", "handler", "workflow-task"],
      default: true,
    },
    "smithers-orchestrator": {
      id: "smithers-orchestrator",
      title: "Smithers Workflow Routing",
      summary: "Orchestrator awareness of handler-supervised workflow execution.",
      body: SMITHERS_ORCHESTRATOR_CONTEXT_BODY,
      enabled: true,
      scope: globalScope,
      allowedActors: ["orchestrator"],
      default: true,
    },
    "smithers-handler": {
      id: "smithers-handler",
      title: "Smithers Workflow Supervision",
      summary: "Handler-thread Smithers workflow supervision guidance.",
      body: SMITHERS_HANDLER_CONTEXT_BODY,
      enabled: true,
      scope: globalScope,
      allowedActors: ["handler"],
      default: true,
    },
    "smithers-workflow-task": {
      id: "smithers-workflow-task",
      title: "Smithers Task-Agent Boundary",
      summary: "Workflow task-agent runtime boundary guidance.",
      body: SMITHERS_WORKFLOW_TASK_CONTEXT_BODY,
      enabled: true,
      scope: globalScope,
      allowedActors: ["workflow-task"],
      default: true,
    },
    ci: {
      id: "ci",
      title: "Project CI",
      summary: "Optional Project CI authoring guidance.",
      body: CI_CONTEXT_BODY,
      enabled: true,
      scope: globalScope,
      allowedActors: ["handler"],
      default: true,
      optionalContextKey: "ci",
    },
  };

  return {
    version: 1,
    revision,
    updatedAt: now,
    instructionBlocks,
    contextPacks,
    actorRecipes: {
      orchestrator: {
        actor: "orchestrator",
        instructionBlockIds: ["common", "orchestrator"],
        contextPackIds: ["cx", "smithers-orchestrator"],
        generatedSectionIds: ["web-context", "execute-typescript"],
      },
      handler: {
        actor: "handler",
        instructionBlockIds: ["common", "handler"],
        contextPackIds: ["cx", "smithers-handler"],
        generatedSectionIds: [
          "web-context",
          "workflow-authoring-contract",
          "handler-workflow-authoring-appendix",
          "loaded-optional-context",
          "execute-typescript",
        ],
      },
      "workflow-task": {
        actor: "workflow-task",
        instructionBlockIds: ["common", "workflow-task"],
        contextPackIds: ["cx", "smithers-workflow-task"],
        generatedSectionIds: ["web-context", "execute-typescript"],
      },
    },
  };
}

function getFallbackRecipe(actor: SvvyActorKind): PromptLibraryActorRecipe {
  return createDefaultPromptLibraryState().actorRecipes[actor];
}

function getEnabledInstructionBlock(
  state: PromptLibraryState,
  id: string,
  workspaceKey?: string,
): PromptLibraryInstructionBlock | null {
  const block = state.instructionBlocks[id];
  return block?.enabled && isPromptBlockActive(block.scope, workspaceKey) ? block : null;
}

function getEnabledContextPack(
  state: PromptLibraryState,
  actor: SvvyActorKind,
  id: string,
  workspaceKey?: string,
): PromptLibraryContextPack | null {
  const pack = state.contextPacks[id];
  if (
    !pack?.enabled ||
    !pack.allowedActors.includes(actor) ||
    !isPromptBlockActive(pack.scope, workspaceKey)
  ) {
    return null;
  }
  return pack;
}

function isPromptBlockActive(
  scope: { appGlobal: boolean; workspaceKeys: readonly string[] },
  workspaceKey?: string,
): boolean {
  return scope.appGlobal || (!!workspaceKey && scope.workspaceKeys.includes(workspaceKey));
}

function buildLoadedOptionalPromptContextFromLibrary(
  state: PromptLibraryState,
  keys: readonly string[],
): string | undefined {
  const validKeys = validateOptionalPromptContextKeys(keys);
  if (validKeys.length === 0) {
    return undefined;
  }

  const sections = validKeys
    .map((key) =>
      Object.values(state.contextPacks).find(
        (pack) => pack.enabled && pack.optionalContextKey === key,
      ),
    )
    .filter((pack): pack is PromptLibraryContextPack => Boolean(pack))
    .map((pack) => pack.body.trim())
    .filter(Boolean);
  return sections.length > 0 ? sections.join("\n\n") : undefined;
}

export function buildSystemPromptFromLibrary(
  actor: SvvyActorKind,
  state: PromptLibraryState,
  options: {
    loadedContextKeys?: readonly string[];
    webProvider?: WebProvider;
    workspaceKey?: string;
  } = {},
): string {
  const recipe = state.actorRecipes[actor] ?? getFallbackRecipe(actor);
  const sections: string[] = [];
  for (const id of recipe.instructionBlockIds) {
    const block = getEnabledInstructionBlock(state, id, options.workspaceKey);
    if (block?.body.trim()) {
      sections.push(block.body.trim());
    }
  }
  for (const id of recipe.contextPackIds) {
    const pack = getEnabledContextPack(state, actor, id, options.workspaceKey);
    if (pack?.body.trim()) {
      sections.push(pack.body.trim());
    }
  }
  for (const generatedId of recipe.generatedSectionIds) {
    if (generatedId === "web-context") {
      sections.push(buildWebPromptContext(actor, options.webProvider));
    } else if (generatedId === "workflow-authoring-contract" && actor === "handler") {
      sections.push(WORKFLOW_AUTHORING_CONTRACT_PROMPT_SECTION);
    } else if (generatedId === "handler-workflow-authoring-appendix" && actor === "handler") {
      sections.push(HANDLER_WORKFLOW_AUTHORING_APPENDIX);
    } else if (generatedId === "loaded-optional-context" && actor === "handler") {
      const loadedContextPrompt = buildLoadedOptionalPromptContextFromLibrary(
        state,
        options.loadedContextKeys ?? [],
      );
      if (loadedContextPrompt) {
        sections.push(loadedContextPrompt);
      }
    } else if (generatedId === "execute-typescript") {
      sections.push(buildExecuteTypescriptPromptSection(actor, options.webProvider));
    }
  }
  return sections.join("\n\n");
}

export function buildPromptLibraryGeneratedEntries(
  actor: SvvyActorKind,
  state: PromptLibraryState,
  options: {
    loadedContextKeys?: readonly string[];
    webProvider?: WebProvider;
  } = {},
): PromptLibraryGeneratedEntry[] {
  const recipe = state.actorRecipes[actor] ?? getFallbackRecipe(actor);
  return recipe.generatedSectionIds
    .map((id) => buildPromptLibraryGeneratedEntry(actor, state, id, options))
    .filter((entry): entry is PromptLibraryGeneratedEntry => Boolean(entry));
}

function buildPromptLibraryGeneratedEntry(
  actor: SvvyActorKind,
  state: PromptLibraryState,
  id: PromptLibraryGeneratedSectionId,
  options: {
    loadedContextKeys?: readonly string[];
    webProvider?: WebProvider;
  },
): PromptLibraryGeneratedEntry | null {
  if (id === "web-context") {
    return {
      id,
      title: "Web Context",
      source: "src/bun/web-runtime/prompt-context.ts",
      sourcePath: "src/bun/web-runtime/prompt-context.ts",
      content: buildWebPromptContext(actor, options.webProvider),
    };
  }
  if (id === "workflow-authoring-contract" && actor === "handler") {
    return {
      id,
      title: "Workflow Authoring Contract",
      source: "generated/workflow-authoring-contract.generated.ts",
      sourcePath: "generated/workflow-authoring-contract.generated.ts",
      content: WORKFLOW_AUTHORING_CONTRACT_PROMPT_SECTION,
    };
  }
  if (id === "handler-workflow-authoring-appendix" && actor === "handler") {
    return {
      id,
      title: "Handler Workflow Authoring Appendix",
      source: "src/bun/smithers-runtime/workflow-authoring-guide.ts",
      sourcePath: "src/bun/smithers-runtime/workflow-authoring-guide.ts",
      content: HANDLER_WORKFLOW_AUTHORING_APPENDIX,
    };
  }
  if (id === "loaded-optional-context" && actor === "handler") {
    const content = buildLoadedOptionalPromptContextFromLibrary(
      state,
      options.loadedContextKeys ?? [],
    );
    if (!content) {
      return null;
    }
    return {
      id,
      title: "Loaded Optional Context",
      source: "src/bun/default-system-prompt.ts",
      sourcePath: "src/bun/default-system-prompt.ts",
      content,
    };
  }
  if (id === "execute-typescript") {
    return {
      id,
      title: "Execute Typescript",
      source: "generated/execute-typescript-api.generated.ts",
      sourcePath: "generated/execute-typescript-api.generated.ts",
      content: buildExecuteTypescriptPromptSection(actor, options.webProvider),
    };
  }
  return null;
}

export function buildSystemPrompt(
  actor: SvvyActorKind,
  options: {
    loadedContextKeys?: readonly string[];
    promptLibraryState?: PromptLibraryState;
    workspaceKey?: string;
    webProvider?: WebProvider;
  } = {},
): string {
  if (options.promptLibraryState) {
    return buildSystemPromptFromLibrary(actor, options.promptLibraryState, options);
  }
  const sections = [...buildActorInstructions(actor)];
  sections.push(buildAlwaysLoadedPromptContext(actor, { webProvider: options.webProvider }));
  if (actor === "handler") {
    sections.push(WORKFLOW_AUTHORING_CONTRACT_PROMPT_SECTION);
    sections.push(HANDLER_WORKFLOW_AUTHORING_APPENDIX);
    const loadedContextPrompt = buildLoadedOptionalPromptContextPrompt(
      options.loadedContextKeys ?? [],
    );
    if (loadedContextPrompt) {
      sections.push(loadedContextPrompt);
    }
  }
  sections.push(buildExecuteTypescriptPromptSection(actor, options.webProvider));
  return sections.join("\n\n");
}

export const DEFAULT_SYSTEM_PROMPT = buildSystemPrompt("orchestrator");
export const HANDLER_SYSTEM_PROMPT = buildSystemPrompt("handler");
export const WORKFLOW_TASK_SYSTEM_PROMPT = buildSystemPrompt("workflow-task");
