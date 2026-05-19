import { DEFAULT_AGENT_SETTINGS } from "../../shared/agent-settings";
import { WORKFLOW_TASK_SYSTEM_PROMPT } from "../default-system-prompt";
import {
  WORKFLOW_TASK_TOOL_REGISTRY,
  type WorkflowTaskAgentConfig,
} from "./workflow-authoring-contract";
export type { WorkflowTaskAgentConfig } from "./workflow-authoring-contract";

export function createDefaultWorkflowTaskAgentConfig(
  input: Partial<Pick<WorkflowTaskAgentConfig, "provider" | "model" | "reasoningEffort">> = {},
): WorkflowTaskAgentConfig {
  return {
    provider: input.provider ?? DEFAULT_AGENT_SETTINGS.provider,
    model: input.model ?? DEFAULT_AGENT_SETTINGS.model,
    reasoningEffort: input.reasoningEffort ?? DEFAULT_AGENT_SETTINGS.reasoningEffort,
    systemPrompt: WORKFLOW_TASK_SYSTEM_PROMPT,
    toolSurface: WORKFLOW_TASK_TOOL_REGISTRY.filter(
      (toolName) => toolName !== "web_search" && toolName !== "web_fetch",
    ),
  };
}
