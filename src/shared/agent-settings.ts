import type {
  ReasoningEffort,
  WorkflowTaskAgentConfig,
  WorkflowTaskToolName,
} from "../bun/smithers-runtime/workflow-authoring-contract";

export type { ReasoningEffort };
export type AgentProfileKind = "orchestrator" | "special";
export type AgentProfileSpecialKey = "threadHandler";
export type AgentProfileId = string;
export type WorkflowAgentKey = "explorer" | "implementer" | "reviewer";
export type AppAppearance = "system" | "light" | "dark";
export type PreferredExternalEditor = "system" | "code" | "cursor" | "zed" | "sublime" | "custom";
export type WebProviderId = "tinyfish" | "firecrawl";
export type WorkflowAgentToolName = WorkflowTaskToolName;

export interface AgentDefaults {
  provider: string;
  model: string;
  reasoningEffort: ReasoningEffort;
}

export interface AgentPromptSettings extends AgentDefaults {
  systemPrompt: string;
}

export interface AgentProfileSettings extends AgentDefaults {
  id: AgentProfileId;
  kind: AgentProfileKind;
  name: string;
  systemPrompt: string;
  extensions: string[];
  updateFromComposer: boolean;
  builtin: boolean;
  locked: boolean;
}

export interface AgentProfileState {
  orchestrators: AgentProfileSettings[];
  special: Record<AgentProfileSpecialKey, AgentProfileSettings>;
  titleNamer: AgentPromptSettings;
}

export interface WorkflowAgentSettings extends WorkflowTaskAgentConfig {
  id: WorkflowAgentKey;
  label: string;
}

export interface AgentSettingsState {
  version: 2;
  agents: AgentProfileState;
  workflowAgents: Record<WorkflowAgentKey, WorkflowAgentSettings>;
  appPreferences: AppPreferences;
}

export interface AppPreferences {
  appAppearance: AppAppearance;
  preferredExternalEditor: PreferredExternalEditor;
  customExternalEditorCommand: string;
  webProvider: WebProviderId | null;
}

export const DEFAULT_AGENT_SETTINGS = {
  provider: "zai",
  model: "glm-5-turbo",
  reasoningEffort: "medium",
} satisfies AgentDefaults;

export const DEFAULT_ORCHESTRATOR_PROFILE_ID = "default-orchestrator";
export const DEFAULT_THREAD_HANDLER_PROFILE_ID = "thread-handler";

export const DEFAULT_ORCHESTRATOR_SESSION_PROMPT =
  "You are svvy, the main orchestrator. Own strategy, route bounded delegated work through handler threads, and make final user-facing decisions.";

export const DEFAULT_THREAD_HANDLER_PROMPT =
  "You are a svvy delegated handler thread. Own the bounded objective, supervise workflow runs when useful, ask for clarification when blocked, and hand durable outcomes back to the orchestrator.";

export const DEFAULT_NAMER_SESSION_PROMPT = [
  "You generate concise session titles for svvy.",
  "Return exactly one title and nothing else.",
  "",
  "The prompt you receive is formatted as:",
  "First user message:",
  "<the user's first message>",
  "",
  "Rules:",
  "- Title only the message after the label.",
  "- Use 2 to 6 words and stay at or below 50 characters.",
  "- Describe the user's concrete intent with specific nouns and verbs from the message.",
  "- Distill the message; do not copy the whole first message as the title.",
  "- Preserve important product names, acronyms, and proper nouns.",
  "- For greetings or vague openers, name the interaction intent, for example Greeting and help request.",
  "- Never return generic titles such as New, New Session, Session, Chat, Conversation, Request, or Task.",
  "- Do not use quotes, colons, markdown, bullets, trailing punctuation, or explanations.",
  "- Use sentence case unless preserving acronyms or proper nouns.",
  "",
  "Examples:",
  "First user message: Hi there",
  "Greeting and help request",
  "",
  "First user message: Implement the Dockview workspace layout integration",
  "Dockview workspace layout",
  "",
  "First user message: The app is duplicating assistant messages after streaming finishes, fix it",
  "Assistant streaming duplicates",
].join("\n");

export const DEFAULT_AGENT_PROFILES = {
  orchestrators: [
    {
      id: DEFAULT_ORCHESTRATOR_PROFILE_ID,
      kind: "orchestrator",
      name: "Default orchestrator",
      ...DEFAULT_AGENT_SETTINGS,
      systemPrompt: DEFAULT_ORCHESTRATOR_SESSION_PROMPT,
      extensions: [],
      updateFromComposer: false,
      builtin: true,
      locked: true,
    },
  ],
  special: {
    threadHandler: {
      id: DEFAULT_THREAD_HANDLER_PROFILE_ID,
      kind: "special",
      name: "Thread handler",
      ...DEFAULT_AGENT_SETTINGS,
      systemPrompt: DEFAULT_THREAD_HANDLER_PROMPT,
      extensions: [],
      updateFromComposer: false,
      builtin: true,
      locked: true,
    },
  },
  titleNamer: {
    ...DEFAULT_AGENT_SETTINGS,
    provider: "openai-codex",
    model: "gpt-5.4-mini",
    reasoningEffort: "low",
    systemPrompt: DEFAULT_NAMER_SESSION_PROMPT,
  },
} satisfies AgentProfileState;

export const DEFAULT_WORKFLOW_AGENT_SETTINGS = {
  explorer: {
    id: "explorer",
    label: "Explorer",
    ...DEFAULT_AGENT_SETTINGS,
    systemPrompt:
      "Inspect the repository and return concise findings, evidence, and unresolved questions. Do not edit files.",
    toolSurface: [
      "cx_overview",
      "cx_symbols",
      "cx_definition",
      "cx_references",
      "cx_lang_list",
      "cx_cache_path",
      "read",
      "grep",
      "find",
      "ls",
      "bash",
      "web_search",
      "web_fetch",
      "execute_typescript",
    ],
  },
  implementer: {
    id: "implementer",
    label: "Implementer",
    ...DEFAULT_AGENT_SETTINGS,
    systemPrompt:
      "Implement the assigned scoped change, keep edits focused, and return changed files plus verification.",
    toolSurface: [
      "read",
      "grep",
      "find",
      "ls",
      "edit",
      "write",
      "bash",
      "cx_overview",
      "cx_symbols",
      "cx_definition",
      "cx_references",
      "cx_lang_list",
      "cx_lang_add",
      "cx_lang_remove",
      "cx_cache_path",
      "cx_cache_clean",
      "artifact_write_text",
      "artifact_write_json",
      "artifact_attach_file",
      "web_search",
      "web_fetch",
      "execute_typescript",
    ],
  },
  reviewer: {
    id: "reviewer",
    label: "Reviewer",
    ...DEFAULT_AGENT_SETTINGS,
    systemPrompt:
      "Review the assigned result for correctness, regressions, edge cases, and missing tests. Lead with findings.",
    toolSurface: [
      "cx_overview",
      "cx_symbols",
      "cx_definition",
      "cx_references",
      "cx_lang_list",
      "cx_cache_path",
      "read",
      "grep",
      "find",
      "ls",
      "bash",
      "web_search",
      "web_fetch",
      "execute_typescript",
    ],
  },
} satisfies Record<WorkflowAgentKey, WorkflowAgentSettings>;

export const DEFAULT_AGENT_SETTINGS_STATE = {
  version: 2,
  agents: DEFAULT_AGENT_PROFILES,
  workflowAgents: DEFAULT_WORKFLOW_AGENT_SETTINGS,
  appPreferences: {
    appAppearance: "system",
    preferredExternalEditor: "system",
    customExternalEditorCommand: "",
    webProvider: null,
  },
} satisfies AgentSettingsState;
