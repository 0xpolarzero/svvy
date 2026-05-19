import {
  OPTIONAL_PROMPT_CONTEXT_METADATA,
  isOptionalPromptContextKey,
  type OptionalPromptContextKey,
  type OptionalPromptContextMetadata,
  type PromptContextActor,
} from "../shared/prompt-context";
import type { WebProvider } from "./web-runtime/contracts";
import { buildWebPromptContext } from "./web-runtime/prompt-context";

export type { OptionalPromptContextKey, PromptContextActor } from "../shared/prompt-context";

export interface OptionalPromptContext extends OptionalPromptContextMetadata {
  prompt: string;
}

const CX_CONTEXT_PROMPT = [
  "Loaded always-on prompt context: cx semantic code navigation.",
  "",
  "cx is the semantic code-navigation layer for repository inspection. Prefer cx for structural exploration before reading full files when cx can cover the language.",
  "",
  "Use this escalation order for code navigation:",
  "- `cx_overview` for a directory or file table of contents.",
  "- `cx_symbols` to search project symbols by kind, name glob, or file.",
  "- `cx_definition` to inspect a symbol body without reading the full file.",
  "- `cx_references` to find callers and usage sites.",
  "- `cx_lang_list` when you need to check whether a grammar is available or installed.",
  "- `cx_lang_add` when a relevant grammar is available but missing and semantic navigation would materially help the task.",
  "- `read`, `grep`, `find`, or `ls` when semantic navigation is insufficient, when raw text is required, or when cx cannot cover the target language.",
  "",
  "cx command behavior:",
  "- `cx_overview` accepts a path and can include full per-file detail for directories.",
  "- `cx_symbols` supports `kind`, `name`, `file`, pagination, and JSON output through the native tool result.",
  "- `cx_definition` supports `name`, `kind`, `from`, pagination, and `maxLines` for large bodies.",
  "- `cx_references` supports `name`, `file`, `unique`, pagination, and JSON output through the native tool result.",
  "- `cx_lang_list`, `cx_lang_add`, `cx_lang_remove`, `cx_cache_path`, and `cx_cache_clean` manage grammars and cache state.",
  "",
  "Use top-level `cx_*` tools for ordinary semantic navigation. Inside `execute_typescript`, use only the read-only `api.cx_*` subset when TypeScript control flow is needed for batching or aggregation.",
].join("\n");

const SMITHERS_ORCHESTRATOR_CONTEXT_PROMPT = [
  "Loaded always-on prompt context: Smithers workflow routing.",
  "",
  "Handler threads supervise Smithers workflow runs. The orchestrator knows this capability exists, but it does not receive `smithers_*` tool declarations.",
  "",
  "When work requires workflow execution, workflow authoring, workflow inspection, or Project CI workflow operation, delegate a bounded objective to a handler thread with `thread_start`, or use `thread_resume` when a completed handler thread already has the right delegated context for follow-up work.",
].join("\n");

const SMITHERS_HANDLER_CONTEXT_PROMPT = [
  "Loaded always-on prompt context: Smithers workflow supervision.",
  "",
  "Handler threads supervise Smithers workflow runs through native `smithers_*` tools. Use direct tools for simple repository work, then saved runnable entries, then artifact workflow authoring when a workflow graph is the right unit of work.",
  "",
  "Use `smithers_list_workflows` to discover runnable saved and artifact entries. Use `smithers_run_workflow({ workflowId, input })` for a fresh launch. Use `smithers_run_workflow({ workflowId, input, runId })` only when you intend to resume that exact run. Omitting `runId` never silently resumes; if this handler already owns a nonterminal run with the same `workflowId`, the call is rejected. Different `workflowId` values can run concurrently under the same handler thread.",
  "",
  "Use Smithers inspection and control tools for supervision: `smithers_get_run`, `smithers_watch_run`, `smithers_explain_run`, `smithers_list_pending_approvals`, `smithers_resolve_approval`, `smithers_get_node_detail`, `smithers_list_artifacts`, `smithers_get_chat_transcript`, `smithers_get_run_events`, `smithers_runs_cancel`, `smithers_signals_send`, `smithers_frames_list`, `smithers_get_devtools_snapshot`, and `smithers_stream_devtools`.",
  "",
  "Workflow waits, approvals, retries, repairs, and resumptions stay inside the supervising handler thread. Call `thread_handoff` only after the current objective span is no longer running or waiting on an owned workflow run.",
].join("\n");

const SMITHERS_WORKFLOW_TASK_CONTEXT_PROMPT = [
  "Loaded always-on prompt context: Smithers task-agent boundary.",
  "",
  "Smithers owns this task attempt's lifecycle, retries, validation, approval gates, and workflow state.",
].join("\n");

const CI_CONTEXT_PROMPT = [
  "Loaded optional prompt context: Project CI.",
  "",
  "Project CI is svvy's dedicated product lane for repeatable repository confidence checks. It uses normal handler-thread execution with this prompt context loaded.",
  "",
  "When configuring or modifying Project CI:",
  "- Inspect real repository facts first, including package scripts, lockfiles, task runners, existing CI files, Makefiles, README guidance, and test configuration.",
  "- Ask the user when the durable confidence policy is ambiguous.",
  "- Write reusable Project CI workflow assets only under `.svvy/workflows/{definitions,prompts,components,entries}/ci/`.",
  "- The conventional saved entry path is `.svvy/workflows/entries/ci/project-ci.tsx` and the conventional workflow id is `project_ci`.",
  '- CI entries are normal Smithers runnable saved entries. They must export `productKind = "project-ci" as const` and a `resultSchema`.',
  "- The entry's terminal output must validate against `resultSchema` and must contain a Project CI result with `status`, `summary`, and stable `checks`.",
  "- Use stable `checkId` values such as `typecheck`, `unit_tests`, `eslint`, `build`, `integration`, `docs`, or repository-specific ids.",
  "- Use open check `kind` strings; recommended kinds are `typecheck`, `test`, `lint`, `build`, `integration`, `docs`, and `manual`.",
  "- After writing saved workflow files, rely on the returned saved-workflow validation feedback and keep editing until the final saved workflow state validates cleanly.",
  '- Confirm the entry appears through `smithers_list_workflows({ productKind: "project-ci" })`, then run it with `smithers_run_workflow`.',
  "",
  "Project CI recording rules are strict:",
  '- Project CI records are created only from entries declaring `productKind = "project-ci"`.',
  "- The terminal output must directly validate against the entry's declared `resultSchema`.",
  "- Never infer CI state from logs, command names, labels, filenames, node output, final prose, or arbitrary workflow output.",
  "- Invalid or missing result output is a CI troubleshooting state.",
].join("\n");

export const OPTIONAL_PROMPT_CONTEXTS: Record<OptionalPromptContextKey, OptionalPromptContext> = {
  ci: {
    ...OPTIONAL_PROMPT_CONTEXT_METADATA.ci,
    prompt: CI_CONTEXT_PROMPT,
  },
};

export function validateOptionalPromptContextKeys(
  keys: readonly string[],
): OptionalPromptContextKey[] {
  const validKeys: OptionalPromptContextKey[] = [];
  const seen = new Set<OptionalPromptContextKey>();
  for (const key of keys) {
    if (!isOptionalPromptContextKey(key)) {
      throw new Error(`Unknown prompt context key: ${key}`);
    }
    if (!seen.has(key)) {
      seen.add(key);
      validKeys.push(key);
    }
  }
  return validKeys;
}

export function getOptionalPromptContext(key: OptionalPromptContextKey): OptionalPromptContext {
  return OPTIONAL_PROMPT_CONTEXTS[key];
}

export function buildAlwaysLoadedPromptContext(
  actor: PromptContextActor,
  options: { webProvider?: WebProvider } = {},
): string {
  const sections = [CX_CONTEXT_PROMPT];
  if (actor === "orchestrator") {
    sections.push(SMITHERS_ORCHESTRATOR_CONTEXT_PROMPT);
  } else if (actor === "handler") {
    sections.push(SMITHERS_HANDLER_CONTEXT_PROMPT);
  } else {
    sections.push(SMITHERS_WORKFLOW_TASK_CONTEXT_PROMPT);
  }
  sections.push(buildWebPromptContext(actor, options.webProvider));
  return sections.join("\n\n");
}

export function buildOptionalPromptContextRegistryPrompt(): string {
  return [
    "Available optional prompt context keys:",
    '- `ci`: Project CI authoring guidance. If Project CI only needs to be run, discover configured CI entries with `smithers_list_workflows({ productKind: "project-ci" })` and run one through `smithers_run_workflow`. If Project CI needs to be configured or modified, call `request_context({ keys: ["ci"] })` before authoring CI assets.',
  ].join("\n");
}

export function buildOrchestratorContextRoutingPrompt(): string {
  return [
    "Optional prompt context routing:",
    '- `ci` is available for Project CI authoring. When a delegated objective clearly needs Project CI configuration or modification from the first handler turn, pass `context: ["ci"]` to `thread_start`.',
  ].join("\n");
}

export function buildLoadedOptionalPromptContextPrompt(
  keys: readonly string[],
): string | undefined {
  const validKeys = validateOptionalPromptContextKeys(keys);
  if (validKeys.length === 0) {
    return undefined;
  }

  return validKeys.map((key) => getOptionalPromptContext(key).prompt).join("\n\n");
}
