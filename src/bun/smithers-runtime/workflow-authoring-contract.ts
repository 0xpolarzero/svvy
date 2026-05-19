/**
 * Source of truth for handler-authored workflow modules and workflow task
 * agents.
 *
 * The build regenerates a prompt declaration from this module. Keep runtime
 * loading, validation, and handler guidance aligned with these exported types.
 */

import type { ThinkingLevel } from "@mariozechner/pi-agent-core";
import type { AgentLike, SmithersWorkflow } from "smithers-orchestrator";
import type { z } from "zod";

export type ReasoningEffort = ThinkingLevel;

export const WORKFLOW_TASK_TOOL_REGISTRY = [
  "cx_overview",
  "cx_symbols",
  "cx_definition",
  "cx_references",
  "cx_lang_list",
  "cx_lang_add",
  "cx_lang_remove",
  "cx_cache_path",
  "cx_cache_clean",
  "read",
  "grep",
  "find",
  "ls",
  "edit",
  "write",
  "bash",
  "artifact_write_text",
  "artifact_write_json",
  "artifact_attach_file",
  "web_search",
  "web_fetch",
  "execute_typescript",
] as const;

export type WorkflowTaskToolName = (typeof WORKFLOW_TASK_TOOL_REGISTRY)[number];

/**
 * Where a runnable workflow entry and its declared assets live.
 */
export type RunnableWorkflowSourceScope = "saved" | "artifact";

/**
 * Product lane marker for entries that project specialized product state.
 */
export type RunnableWorkflowProductKind = "project-ci";

/**
 * Runtime value returned by a runnable entry factory.
 */
export interface RunnableWorkflowRuntimeEntry {
  workflowId: string;
  workflowSource: RunnableWorkflowSourceScope;
  productKind?: RunnableWorkflowProductKind;
  launchSchema: z.ZodTypeAny;
  resultSchema?: z.ZodTypeAny;
  workflow: SmithersWorkflow<any>;
}

/**
 * Input passed by svvy when loading a runnable entry against the workspace
 * Smithers runtime database.
 */
export interface RunnableWorkflowCreateInput {
  dbPath: string;
}

/**
 * Exports required from every handler-authored runnable workflow entry file.
 */
export interface RunnableWorkflowEntryModule {
  workflowId: string;
  label: string;
  summary: string;
  productKind?: RunnableWorkflowProductKind;
  launchSchema: z.ZodTypeAny;
  resultSchema?: z.ZodTypeAny;
  definitionPaths: string[];
  promptPaths: string[];
  componentPaths: string[];
  createRunnableEntry(input: RunnableWorkflowCreateInput): RunnableWorkflowRuntimeEntry;
}

/**
 * Registry entry compiled from a runnable workflow entry module.
 */
export interface RunnableWorkflowRegistryEntry extends RunnableWorkflowEntryModule {
  sourceScope: RunnableWorkflowSourceScope;
  entryPath: string;
  assetPaths: string[];
}

/**
 * svvy workflow task-agent configuration. Workflow task agents are AgentLike
 * values configured through this shape and receive only the task-local tool
 * surface.
 */
export interface WorkflowTaskAgentConfig {
  provider: string;
  model: string;
  reasoningEffort: ReasoningEffort;
  systemPrompt: string;
  toolSurface: readonly WorkflowTaskToolName[];
}

/**
 * AgentLike is the only agent contract handlers should use inside authored
 * Smithers tasks.
 */
export type SvvyWorkflowTaskAgent = AgentLike;
