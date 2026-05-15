import type { ThinkingLevel } from "@mariozechner/pi-agent-core";

export type ReasoningEffort = ThinkingLevel;
export type SessionMode = "orchestrator" | "dumb";
export type SessionAgentKey = "defaultSession" | "dumbOrchestrator" | "namer";
export type WorkflowAgentKey = "explorer" | "implementer" | "reviewer";
export type AppAppearance = "system" | "light" | "dark";
export type PreferredExternalEditor = "system" | "code" | "cursor" | "zed" | "sublime" | "custom";
export type WebProviderId = "tinyfish" | "firecrawl";
export type WorkflowAgentToolName =
  | "read"
  | "grep"
  | "find"
  | "ls"
  | "edit"
  | "write"
  | "bash"
  | "cx.overview"
  | "cx.symbols"
  | "cx.definition"
  | "cx.references"
  | "cx.lang.list"
  | "cx.lang.add"
  | "cx.lang.remove"
  | "cx.cache.path"
  | "cx.cache.clean"
  | "artifact.write_text"
  | "artifact.write_json"
  | "artifact.attach_file"
  | "web.search"
  | "web.fetch"
  | "execute_typescript";

export interface AgentDefaults {
  provider: string;
  model: string;
  reasoningEffort: ReasoningEffort;
}

export interface SessionAgentSettings extends AgentDefaults {
  systemPrompt: string;
}

export interface SessionAgentDefaults {
  defaultSession: SessionAgentSettings;
  dumbOrchestrator: SessionAgentSettings;
  namer: SessionAgentSettings;
}

export interface WorkflowAgentSettings extends SessionAgentSettings {
  id: WorkflowAgentKey;
  label: string;
  toolSurface: readonly WorkflowAgentToolName[];
}

export interface AgentSettingsState {
  version: 1;
  sessionAgents: SessionAgentDefaults;
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

export const DEFAULT_ORCHESTRATOR_SESSION_PROMPT =
  "You are svvy, the main orchestrator. Own strategy, route bounded delegated work through handler threads, and make final user-facing decisions.";

export const DEFAULT_DUMB_ORCHESTRATOR_PROMPT =
  "You are svvy dumb orchestrator. Answer or act directly for short, focused work without starting handler threads unless delegation is explicitly necessary.";

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

export const DEFAULT_SESSION_AGENT_SETTINGS = {
  defaultSession: {
    ...DEFAULT_AGENT_SETTINGS,
    systemPrompt: DEFAULT_ORCHESTRATOR_SESSION_PROMPT,
  },
  dumbOrchestrator: {
    ...DEFAULT_AGENT_SETTINGS,
    systemPrompt: DEFAULT_DUMB_ORCHESTRATOR_PROMPT,
  },
  namer: {
    ...DEFAULT_AGENT_SETTINGS,
    provider: "openai-codex",
    model: "gpt-5.4-mini",
    reasoningEffort: "low",
    systemPrompt: DEFAULT_NAMER_SESSION_PROMPT,
  },
} satisfies SessionAgentDefaults;

export const DEFAULT_WORKFLOW_AGENT_SETTINGS = {
  explorer: {
    id: "explorer",
    label: "Explorer",
    ...DEFAULT_AGENT_SETTINGS,
    systemPrompt:
      "Inspect the repository and return concise findings, evidence, and unresolved questions. Do not edit files.",
    toolSurface: [
      "cx.overview",
      "cx.symbols",
      "cx.definition",
      "cx.references",
      "cx.lang.list",
      "cx.cache.path",
      "read",
      "grep",
      "find",
      "ls",
      "bash",
      "web.search",
      "web.fetch",
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
      "cx.overview",
      "cx.symbols",
      "cx.definition",
      "cx.references",
      "cx.lang.list",
      "cx.lang.add",
      "cx.lang.remove",
      "cx.cache.path",
      "cx.cache.clean",
      "artifact.write_text",
      "artifact.write_json",
      "artifact.attach_file",
      "web.search",
      "web.fetch",
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
      "cx.overview",
      "cx.symbols",
      "cx.definition",
      "cx.references",
      "cx.lang.list",
      "cx.cache.path",
      "read",
      "grep",
      "find",
      "ls",
      "bash",
      "web.search",
      "web.fetch",
      "execute_typescript",
    ],
  },
} satisfies Record<WorkflowAgentKey, WorkflowAgentSettings>;

export const DEFAULT_AGENT_SETTINGS_STATE = {
  version: 1,
  sessionAgents: DEFAULT_SESSION_AGENT_SETTINGS,
  workflowAgents: DEFAULT_WORKFLOW_AGENT_SETTINGS,
  appPreferences: {
    appAppearance: "system",
    preferredExternalEditor: "system",
    customExternalEditorCommand: "",
    webProvider: null,
  },
} satisfies AgentSettingsState;
