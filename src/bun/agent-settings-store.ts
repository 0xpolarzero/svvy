import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import {
  DEFAULT_AGENT_SETTINGS_STATE,
  DEFAULT_ORCHESTRATOR_PROFILE_ID,
  type AgentSettingsState,
  type AgentProfileId,
  type AgentProfileSettings,
  type AppAppearance,
  type AppPreferences,
  type PreferredExternalEditor,
  type WebProviderId,
  type AgentPromptSettings,
  type WorkflowAgentKey,
  type WorkflowAgentSettings,
  type WorkflowAgentToolName,
} from "../shared/agent-settings";
import {
  WORKFLOW_TASK_TOOL_REGISTRY,
  type WorkflowTaskAgentConfig,
} from "./smithers-runtime/workflow-authoring-contract";

export type AgentSettingsStore = {
  getState(): AgentSettingsState;
  setAgentProfile(profile: AgentProfileSettings): AgentSettingsState;
  deleteAgentProfile(id: AgentProfileId): AgentSettingsState;
  reorderOrchestratorProfiles(ids: AgentProfileId[]): AgentSettingsState;
  setWorkflowAgent(key: WorkflowAgentKey, settings: WorkflowAgentSettings): AgentSettingsState;
  setAppPreferences(preferences: AppPreferences): AgentSettingsState;
  ensureWorkflowAgentsComponent(): string;
};

export function createAgentSettingsStore(input: {
  cwd: string;
  agentDir: string;
}): AgentSettingsStore {
  const settingsPath = join(input.agentDir, "agent-settings.json");
  const workflowAgentsPath = join(input.cwd, ".svvy", "workflows", "components", "agents.ts");

  const readState = (): AgentSettingsState => {
    if (!existsSync(settingsPath)) {
      return structuredClone(DEFAULT_AGENT_SETTINGS_STATE);
    }
    const raw = JSON.parse(readFileSync(settingsPath, "utf8")) as Partial<AgentSettingsState>;
    return normalizeAgentSettingsState(raw);
  };

  const writeState = (state: AgentSettingsState): AgentSettingsState => {
    mkdirSync(dirname(settingsPath), { recursive: true });
    writeFileSync(settingsPath, `${JSON.stringify(state, null, 2)}\n`);
    return state;
  };

  const writeWorkflowAgents = (state: AgentSettingsState): string => {
    mkdirSync(dirname(workflowAgentsPath), { recursive: true });
    writeFileSync(workflowAgentsPath, renderWorkflowAgentsComponent(state.workflowAgents));
    return workflowAgentsPath;
  };

  return {
    getState: readState,
    setAgentProfile: (profile) => {
      const state = readState();
      const normalizedProfile = normalizeAgentProfile(profile);
      const existingOrchestratorIndex = state.agents.orchestrators.findIndex(
        (agent) => agent.id === normalizedProfile.id,
      );
      const orchestrators =
        normalizedProfile.kind === "orchestrator"
          ? existingOrchestratorIndex >= 0
            ? state.agents.orchestrators.map((agent, index) =>
                index === existingOrchestratorIndex
                  ? normalizeAgentProfile({
                      ...normalizedProfile,
                      builtin: agent.builtin,
                      locked: agent.locked,
                    })
                  : agent,
              )
            : [
                ...state.agents.orchestrators,
                normalizeAgentProfile({
                  ...normalizedProfile,
                  builtin: false,
                  locked: false,
                }),
              ]
          : state.agents.orchestrators;
      state.agents = normalizeAgentProfileState({
        ...state.agents,
        orchestrators,
        special:
          normalizedProfile.kind === "special"
            ? {
                ...state.agents.special,
                threadHandler:
                  normalizedProfile.id === state.agents.special.threadHandler.id
                    ? normalizeAgentProfile({
                        ...normalizedProfile,
                        kind: "special",
                        builtin: true,
                        locked: true,
                      })
                    : state.agents.special.threadHandler,
              }
            : state.agents.special,
      });
      return writeState(state);
    },
    deleteAgentProfile: (id) => {
      const state = readState();
      state.agents = normalizeAgentProfileState({
        ...state.agents,
        orchestrators: state.agents.orchestrators.filter(
          (agent) => agent.id !== id || agent.locked,
        ),
      });
      return writeState(state);
    },
    reorderOrchestratorProfiles: (ids) => {
      const state = readState();
      const byId = new Map(state.agents.orchestrators.map((agent) => [agent.id, agent]));
      const locked = state.agents.orchestrators.filter((agent) => agent.locked);
      const ordered = ids
        .map((id) => byId.get(id))
        .filter((agent): agent is AgentProfileSettings => agent !== undefined && !agent.locked);
      const missing = state.agents.orchestrators.filter(
        (agent) => !agent.locked && !ids.includes(agent.id),
      );
      state.agents = normalizeAgentProfileState({
        ...state.agents,
        orchestrators: [...locked, ...ordered, ...missing],
      });
      return writeState(state);
    },
    setWorkflowAgent: (key, settings) => {
      const state = readState();
      state.workflowAgents[key] = normalizeWorkflowAgentSettings(key, settings);
      writeState(state);
      writeWorkflowAgents(state);
      return state;
    },
    setAppPreferences: (preferences) => {
      const state = readState();
      state.appPreferences = normalizeAppPreferences(preferences);
      return writeState(state);
    },
    ensureWorkflowAgentsComponent: () => writeWorkflowAgents(readState()),
  };
}

export function normalizeAgentSettingsState(
  input: Partial<AgentSettingsState>,
): AgentSettingsState {
  const defaults = structuredClone(DEFAULT_AGENT_SETTINGS_STATE);
  const workflowAgents = (input.workflowAgents ?? {}) as Partial<
    AgentSettingsState["workflowAgents"]
  >;

  return {
    version: 2,
    agents: normalizeAgentProfileState(
      (input.agents ?? {}) as Partial<AgentSettingsState["agents"]>,
    ),
    workflowAgents: {
      explorer: normalizeWorkflowAgentSettings("explorer", {
        ...defaults.workflowAgents.explorer,
        ...workflowAgents.explorer,
      }),
      implementer: normalizeWorkflowAgentSettings("implementer", {
        ...defaults.workflowAgents.implementer,
        ...workflowAgents.implementer,
      }),
      reviewer: normalizeWorkflowAgentSettings("reviewer", {
        ...defaults.workflowAgents.reviewer,
        ...workflowAgents.reviewer,
      }),
    },
    appPreferences: normalizeAppPreferences({
      ...defaults.appPreferences,
      ...input.appPreferences,
    }),
  };
}

function normalizeAgentProfileState(
  input: Partial<AgentSettingsState["agents"]>,
): AgentSettingsState["agents"] {
  const defaults = structuredClone(DEFAULT_AGENT_SETTINGS_STATE.agents);
  const orchestratorsInput = Array.isArray(input.orchestrators) ? input.orchestrators : [];
  const orchestrators = orchestratorsInput
    .map((profile) => normalizeAgentProfile(profile))
    .filter((profile) => profile.kind === "orchestrator")
    .map((profile) =>
      profile.id === DEFAULT_ORCHESTRATOR_PROFILE_ID
        ? { ...profile, builtin: true, locked: true }
        : profile,
    );
  if (!orchestrators.some((profile) => profile.id === DEFAULT_ORCHESTRATOR_PROFILE_ID)) {
    const defaultOrchestrator = defaults.orchestrators[0];
    if (defaultOrchestrator) {
      orchestrators.unshift(defaultOrchestrator);
    }
  }
  const byId = new Map<string, AgentProfileSettings>();
  for (const profile of orchestrators) {
    byId.set(profile.id, profile);
  }

  const threadHandler = normalizeAgentProfile({
    ...defaults.special.threadHandler,
    ...input.special?.threadHandler,
    id: defaults.special.threadHandler.id,
    kind: "special",
    locked: true,
    builtin: true,
  });
  const titleNamer = normalizeAgentPromptSettings({
    ...defaults.titleNamer,
    ...input.titleNamer,
  });
  return {
    orchestrators: [...byId.values()].toSorted((left, right) => {
      if (left.id === DEFAULT_ORCHESTRATOR_PROFILE_ID) return -1;
      if (right.id === DEFAULT_ORCHESTRATOR_PROFILE_ID) return 1;
      return 0;
    }),
    special: { threadHandler },
    titleNamer,
  };
}

function normalizeAgentProfile(input: AgentProfileSettings): AgentProfileSettings {
  return {
    id: requireNonEmpty(input.id, "id"),
    kind: input.kind === "special" ? "special" : "orchestrator",
    name: requireNonEmpty(input.name, "name"),
    provider: requireNonEmpty(input.provider, "provider"),
    model: requireNonEmpty(input.model, "model"),
    reasoningEffort: input.reasoningEffort,
    systemPrompt: requireNonEmpty(input.systemPrompt, "systemPrompt"),
    extensions: Array.isArray(input.extensions) ? input.extensions.filter(Boolean) : [],
    updateFromComposer: Boolean(input.updateFromComposer),
    builtin: Boolean(input.builtin),
    locked: Boolean(input.locked),
  };
}

export function renderWorkflowAgentsComponent(
  agents: Record<WorkflowAgentKey, WorkflowAgentSettings>,
): string {
  const lines = [
    "/**",
    " * @svvyAssetKind component",
    " * @svvyId workflow_agents",
    " * @svvyTitle Workflow Agents",
    " * @svvySummary Conventional explorer, implementer, and reviewer agents for Smithers workflows.",
    " */",
    "",
    `export type WorkflowTaskToolName = ${WORKFLOW_TASK_TOOL_REGISTRY.map((tool) => JSON.stringify(tool)).join(" | ")};`,
    "",
    "export interface WorkflowTaskAgentConfig {",
    "  provider: string;",
    "  model: string;",
    "  reasoningEffort: string;",
    "  systemPrompt: string;",
    "  toolSurface: readonly WorkflowTaskToolName[];",
    "}",
    "",
    "export type WorkflowAgentComponent = WorkflowTaskAgentConfig & {",
    "  id: string;",
    "  label: string;",
    "};",
    "",
  ];
  for (const key of ["explorer", "implementer", "reviewer"] as const) {
    const agent = agents[key];
    lines.push(
      `export const ${key}: WorkflowAgentComponent = ${JSON.stringify(agent, null, 2)};`,
      "",
    );
  }
  return `${lines.join("\n")}\n`;
}

function normalizeAgentPromptSettings(input: AgentPromptSettings): AgentPromptSettings {
  return {
    provider: requireNonEmpty(input.provider, "provider"),
    model: requireNonEmpty(input.model, "model"),
    reasoningEffort: input.reasoningEffort,
    systemPrompt: requireNonEmpty(input.systemPrompt, "systemPrompt"),
  };
}

function normalizeWorkflowAgentSettings(
  key: WorkflowAgentKey,
  input: WorkflowAgentSettings,
): WorkflowAgentSettings {
  return assertWorkflowAgentSettingsAssignableToTaskConfig({
    id: key,
    label: requireNonEmpty(input.label, "label"),
    ...normalizeAgentPromptSettings(input),
    toolSurface: normalizeWorkflowAgentToolSurface(input.toolSurface),
  });
}

function normalizeWorkflowAgentToolSurface(
  input: readonly WorkflowAgentToolName[],
): readonly WorkflowAgentToolName[] {
  const allowed = new Set<WorkflowAgentToolName>(WORKFLOW_TASK_TOOL_REGISTRY);
  const normalized = input.filter((tool): tool is WorkflowAgentToolName => allowed.has(tool));
  return normalized.length > 0
    ? normalized
    : DEFAULT_AGENT_SETTINGS_STATE.workflowAgents.implementer.toolSurface;
}

function assertWorkflowAgentSettingsAssignableToTaskConfig<T extends WorkflowTaskAgentConfig>(
  settings: T,
): T {
  return settings;
}

function requireNonEmpty(value: string, label: string): string {
  const trimmed = value.trim();
  if (!trimmed) {
    throw new Error(`Expected non-empty ${label}.`);
  }
  return trimmed;
}

function normalizeAppPreferences(input: AppPreferences): AppPreferences {
  return {
    appAppearance: normalizeAppAppearance(input.appAppearance),
    preferredExternalEditor: normalizePreferredExternalEditor(input.preferredExternalEditor),
    customExternalEditorCommand: input.customExternalEditorCommand.trim(),
    webProvider: normalizeWebProvider(input.webProvider),
  };
}

function normalizeAppAppearance(input: string | null | undefined): AppAppearance {
  return input === "light" || input === "dark" || input === "system" ? input : "system";
}

function normalizePreferredExternalEditor(input: string): PreferredExternalEditor {
  if (
    input === "system" ||
    input === "code" ||
    input === "cursor" ||
    input === "zed" ||
    input === "sublime" ||
    input === "custom"
  ) {
    return input;
  }

  return "system";
}

function normalizeWebProvider(input: string | null | undefined): WebProviderId | null {
  return input === "tinyfish" || input === "firecrawl" ? input : null;
}
