import { describe, expect, it, mock } from "bun:test";
import type { AgentMessage } from "@mariozechner/pi-agent-core";
import { getModel, type AssistantMessage, type Message } from "@mariozechner/pi-ai";
import type { ChatStorage, CustomProvider } from "./chat-storage";
import {
  parseComposerAttachmentTextSignature,
  type AppWorkspaceUiRestoreState,
  type AppLogEntry,
  type AppLogSummary,
  type AppLogUpdateMessage,
  type ConversationSurfaceSnapshot,
  type PromptTarget,
  type SendPromptRequest,
  type SurfaceSyncMessage,
  type WorkspaceCommandInspector,
  type WorkspaceHandlerThreadInspector,
  type WorkspaceHandlerThreadSummary,
  type WorkspaceProjectCiStatusPanel,
  type WorkspaceSessionSummary,
  type WorkspaceSyncMessage,
  type WorkspaceWorkflowTaskAttemptInspector,
  type WorkspaceTabInfo,
} from "../shared/workspace-contract";
import type { PromptHistoryEntry } from "./prompt-history";
import type { ChatRuntimeRpcClient } from "./chat-runtime";
import type { WorkspaceDockviewLayoutState, WorkspaceLayoutSlotId } from "./pane-layout";
import { getPromptLibraryContentKey, type PromptLibraryState } from "../shared/prompt-library";
import { buildWorkspaceSessionNavigation } from "./session-state";

mock.module("electrobun/view", () => {
  const MockElectroview = Object.assign(
    function MockElectroview() {
      return undefined;
    },
    {
      defineRPC() {
        return {
          request: {},
          addMessageListener() {},
          removeMessageListener() {},
        };
      },
    },
  );

  return {
    Electroview: MockElectroview,
  };
});

type ReasoningEffort = ConversationSurfaceSnapshot["reasoningEffort"];
const TEST_WORKSPACE_INFO: WorkspaceTabInfo = {
  workspaceTabId: "workspace-tab-1",
  workspaceId: "/tmp/svvy#runtime-1",
  cwd: "/tmp/svvy",
  workspaceLabel: "svvy",
  kind: "user",
  branch: "main",
  openedAt: "2026-04-10T10:00:00.000Z",
};

type PromptHandlerResult = {
  assistantText: string;
  extraMessages?: AgentMessage[];
  reason?: Extract<SurfaceSyncMessage["reason"], "prompt.settled" | "surface.updated">;
  emitSurfaceSyncBeforeStreamDone?: boolean;
};

type PromptHandler = (
  request: SendPromptRequest,
  harness: FakeRpcHarness,
) => Promise<PromptHandlerResult> | PromptHandlerResult;

type SurfaceRecord = {
  snapshot: ConversationSurfaceSnapshot;
  retainCount: number;
};

type WorkspaceUiRestoreState = AppWorkspaceUiRestoreState & {
  layouts: Record<WorkspaceLayoutSlotId, WorkspaceDockviewLayoutState | null>;
};

type FakeRpcHarness = {
  client: ChatRuntimeRpcClient;
  openedTargets: PromptTarget[];
  closeRequests: PromptTarget[];
  promptRequests: SendPromptRequest[];
  modelUpdates: Array<{ target: PromptTarget; model: string }>;
  thoughtLevelUpdates: Array<{ target: PromptTarget; level: ReasoningEffort }>;
  cancelRequests: PromptTarget[];
  requestCounts: {
    listSessions: number;
  };
  appLogSeenRequests: number[];
  branchListRequests: string[];
  branchSwitchRequests: Array<{ workspaceId: string; branch: string }>;
  emitAppLogUpdate: (payload: AppLogUpdateMessage) => void;
  commandInspectorRequests: Array<{ sessionId: string; commandId: string }>;
  handlerThreadListRequests: string[];
  handlerThreadInspectorRequests: Array<{ sessionId: string; threadId: string }>;
  workflowTaskAttemptInspectorRequests: Array<{
    sessionId: string;
    workflowTaskAttemptId: string;
  }>;
  projectCiStatusRequests: string[];
  setPromptHandler: (surfacePiSessionId: string, handler: PromptHandler) => void;
  updateSummary: (sessionId: string, updater: (summary: WorkspaceSessionSummary) => void) => void;
  emitWorkspaceSync: (reason?: WorkspaceSyncMessage["reason"], workspaceId?: string) => void;
  emitSurfaceSync: (
    payload: Omit<SurfaceSyncMessage, "workspaceId"> & { workspaceId?: string },
  ) => void;
  getRetainCount: (surfacePiSessionId: string) => number;
  getSurfaceSnapshot: (surfacePiSessionId: string) => ConversationSurfaceSnapshot;
  getWorkspaceUiRestore: (workspaceId: string) => WorkspaceUiRestoreState | null;
  setWorkspaceUiRestore: (workspaceId: string, state: WorkspaceUiRestoreState) => void;
};

function cloneTarget(target: PromptTarget): PromptTarget {
  return structuredClone(target);
}

const defaultPromptHandler: PromptHandler = async (request) => ({
  assistantText: `Reply for ${request.target.surfacePiSessionId}`,
});

function userMessage(text: string): AgentMessage {
  return {
    role: "user",
    timestamp: Date.now(),
    content: [{ type: "text", text }],
  };
}

function assistantMessage(
  text: string,
  options: {
    provider?: string;
    model?: string;
  } = {},
): AssistantMessage {
  return {
    role: "assistant",
    timestamp: Date.now(),
    api: `${options.provider ?? "openai"}-responses`,
    provider: options.provider ?? "openai",
    model: options.model ?? "gpt-4o",
    usage: {
      input: 0,
      output: 0,
      cacheRead: 0,
      cacheWrite: 0,
      totalTokens: 0,
      cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0, total: 0 },
    },
    stopReason: "stop",
    content: [{ type: "text", text }],
  };
}

function createDeferred<T = void>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((nextResolve, nextReject) => {
    resolve = nextResolve;
    reject = nextReject;
  });
  return { promise, resolve, reject };
}

async function waitFor(condition: () => boolean, timeoutMs = 2_000): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (condition()) {
      return;
    }
    await Bun.sleep(10);
  }

  throw new Error("Timed out waiting for chat runtime state.");
}

function createOrchestratorTarget(workspaceSessionId: string): PromptTarget {
  return {
    workspaceSessionId,
    surface: "orchestrator",
    surfacePiSessionId: workspaceSessionId,
  };
}

function createThreadTarget(
  workspaceSessionId: string,
  surfacePiSessionId: string,
  threadId: string,
): PromptTarget {
  return {
    workspaceSessionId,
    surface: "thread",
    surfacePiSessionId,
    threadId,
  };
}

function hasUserText(messages: AgentMessage[], text: string): boolean {
  return messages.some((message) => {
    if (message.role !== "user" || !Array.isArray(message.content)) {
      return false;
    }
    return message.content.some((content) => content.type === "text" && content.text === text);
  });
}

function createSummary(
  id: string,
  title: string,
  preview: string,
  reasoningEffort: ReasoningEffort = "medium",
  options: { parentSessionId?: string } = {},
): WorkspaceSessionSummary {
  return {
    id,
    title,
    preview,
    createdAt: "2026-04-10T10:00:00.000Z",
    updatedAt: "2026-04-10T10:05:00.000Z",
    messageCount: 2,
    status: "idle",
    isPinned: false,
    pinnedAt: null,
    isArchived: false,
    archivedAt: null,
    isUnread: false,
    unreadAt: null,
    unreadReason: null,
    lastReadAt: null,
    provider: "openai",
    modelId: "gpt-4o",
    thinkingLevel: reasoningEffort,
    ...(options.parentSessionId ? { parentSessionId: options.parentSessionId } : {}),
    wait: null,
    counts: {
      turns: 0,
      threads: 0,
      commands: 0,
      episodes: 0,
      ciRuns: 0,
      ciChecks: 0,
      workflows: 0,
      artifacts: 0,
      events: 0,
    },
    threadIdsByStatus: {
      runningHandler: [],
      runningWorkflow: [],
      waiting: [],
      troubleshooting: [],
    },
  };
}

function createSurfaceSnapshot(input: {
  target: PromptTarget;
  messages: AgentMessage[];
  pendingUserMessage?: AgentMessage | null;
  streamMessage?: AssistantMessage | null;
  streamSequence?: number;
  queuedMessages?: ConversationSurfaceSnapshot["queuedMessages"];
  provider?: string;
  model?: string;
  reasoningEffort?: ReasoningEffort;
  sessionMode?: ConversationSurfaceSnapshot["sessionMode"];
  sessionAgentKey?: ConversationSurfaceSnapshot["sessionAgentKey"];
  systemPrompt?: string;
  resolvedSystemPrompt?: string;
  externalContextSources?: ConversationSurfaceSnapshot["externalContextSources"];
  promptStatus?: ConversationSurfaceSnapshot["promptStatus"];
  activeTurnId?: string | null;
  activeTurnStartedAt?: string | null;
  turnTimings?: ConversationSurfaceSnapshot["turnTimings"];
}): ConversationSurfaceSnapshot {
  const systemPrompt = input.systemPrompt ?? "You are svvy.";
  return {
    target: structuredClone(input.target),
    messages: structuredClone(input.messages),
    pendingUserMessage: input.pendingUserMessage ? structuredClone(input.pendingUserMessage) : null,
    queuedMessages: structuredClone(input.queuedMessages ?? []),
    composerDraft: { text: "", attachments: [], updatedAt: null },
    streamMessage: input.streamMessage ? structuredClone(input.streamMessage) : null,
    streamSequence: input.streamSequence ?? 0,
    provider: input.provider ?? "openai",
    model: input.model ?? "gpt-4o",
    reasoningEffort: input.reasoningEffort ?? "medium",
    sessionMode: input.sessionMode ?? "orchestrator",
    sessionAgentKey: input.sessionAgentKey ?? "defaultSession",
    systemPrompt,
    resolvedSystemPrompt: input.resolvedSystemPrompt ?? systemPrompt,
    externalContextSources: structuredClone(input.externalContextSources ?? []),
    promptStatus: input.promptStatus ?? "idle",
    activeTurnId: input.activeTurnId ?? null,
    activeTurnStartedAt: input.activeTurnStartedAt ?? null,
    turnTimings: structuredClone(input.turnTimings ?? []),
  };
}

function createCommandInspector(
  commandId = "command-1",
  toolName = "execute_typescript",
): WorkspaceCommandInspector {
  return {
    commandId,
    threadId: "thread-1",
    workflowRunId: null,
    toolName,
    visibility: "summary",
    status: "succeeded",
    title: "Inspect docs",
    summary: "Read docs and created 1 artifact.",
    facts: {
      repoReads: 1,
      artifactsCreated: 1,
    },
    error: null,
    startedAt: "2026-04-10T10:00:00.000Z",
    updatedAt: "2026-04-10T10:05:00.000Z",
    finishedAt: "2026-04-10T10:05:00.000Z",
    artifacts: [],
    childCount: 1,
    summaryChildCount: 1,
    traceChildCount: 0,
    summaryChildren: [
      {
        commandId: "command-summary-1",
        toolName: "artifact_write_text",
        visibility: "summary",
        status: "succeeded",
        title: "Create summary.md",
        summary: "Created summary.md.",
        error: null,
        facts: {
          name: "summary.md",
        },
        startedAt: "2026-04-10T10:01:00.000Z",
        updatedAt: "2026-04-10T10:02:00.000Z",
        finishedAt: "2026-04-10T10:02:00.000Z",
        artifacts: [],
      },
    ],
    traceChildren: [],
  };
}

function createHandlerThreadSummary(threadId = "thread-1"): WorkspaceHandlerThreadSummary {
  return {
    threadId,
    surfacePiSessionId: `thread-session-${threadId}`,
    title: "Parser fix thread",
    objective: "Patch the parser bug and add regression coverage.",
    status: "completed",
    wait: null,
    startedAt: "2026-04-10T10:00:00.000Z",
    updatedAt: "2026-04-10T10:05:00.000Z",
    finishedAt: "2026-04-10T10:05:00.000Z",
    commandCount: 1,
    workflowRunCount: 1,
    episodeCount: 1,
    artifactCount: 1,
    ciRunCount: 1,
    loadedContextKeys: ["ci"],
    latestWorkflowRun: {
      workflowRunId: "workflow-1",
      workflowName: "project_ci",
      status: "completed",
      summary: "Project CI workflow completed.",
      updatedAt: "2026-04-10T10:04:30.000Z",
      artifacts: [],
    },
    latestCiRun: {
      ciRunId: "ci-run-1",
      workflowRunId: "workflow-1",
      workflowId: "project_ci",
      status: "passed",
      summary: "Project CI passed.",
      updatedAt: "2026-04-10T10:04:30.000Z",
    },
    latestEpisode: {
      episodeId: "episode-1",
      kind: "change",
      title: "Latest handoff",
      summary: "Patched the parser transitions and added regression coverage.",
      createdAt: "2026-04-10T10:04:00.000Z",
    },
  };
}

function createHandlerThreadInspector(threadId = "thread-1"): WorkspaceHandlerThreadInspector {
  return {
    ...createHandlerThreadSummary(threadId),
    commandRollups: [],
    workflowRuns: [
      {
        workflowRunId: "workflow-1",
        workflowName: "project_ci",
        status: "completed",
        summary: "Project CI workflow completed.",
        updatedAt: "2026-04-10T10:04:30.000Z",
        artifacts: [],
      },
    ],
    workflowTaskAttempts: [
      {
        workflowTaskAttemptId: "workflow-task-attempt-1",
        workflowRunId: "workflow-1",
        smithersRunId: "smithers-run-1",
        nodeId: "assistant",
        iteration: 0,
        attempt: 1,
        title: "assistant",
        kind: "agent",
        status: "completed",
        summary: "Transcript probe completed.",
        updatedAt: "2026-04-10T10:03:30.000Z",
        commandCount: 1,
        artifactCount: 0,
        transcriptMessageCount: 2,
        contextBudget: null,
      },
    ],
    episodes: [
      {
        episodeId: "episode-1",
        kind: "change",
        title: "Latest handoff",
        summary: "Patched the parser transitions and added regression coverage.",
        createdAt: "2026-04-10T10:04:00.000Z",
      },
    ],
    artifacts: [],
  };
}

function createProjectCiStatusPanel(): WorkspaceProjectCiStatusPanel {
  return {
    status: "passed",
    summary: "Project CI passed.",
    entries: [
      {
        workflowId: "project_ci",
        label: "Project CI",
        summary: "Runs Project CI checks.",
        sourceScope: "saved",
        entryPath: ".svvy/workflows/entries/ci/project-ci.tsx",
      },
    ],
    activeWorkflowRun: null,
    latestRun: {
      ciRunId: "ci-run-1",
      workflowRunId: "workflow-1",
      workflowId: "project_ci",
      status: "passed",
      summary: "Project CI passed.",
      updatedAt: "2026-04-10T10:04:30.000Z",
      threadId: "thread-1",
      threadTitle: "Parser fix thread",
      smithersRunId: "smithers-run-1",
      entryPath: ".svvy/workflows/entries/ci/project-ci.tsx",
      startedAt: "2026-04-10T10:03:30.000Z",
      finishedAt: "2026-04-10T10:04:30.000Z",
    },
    checks: [
      {
        checkResultId: "ci-check-1",
        checkId: "typecheck",
        label: "Typecheck",
        kind: "typecheck",
        status: "passed",
        required: true,
        command: ["bun", "run", "typecheck"],
        exitCode: 0,
        summary: "Typecheck passed.",
        artifactIds: [],
        artifacts: [],
        startedAt: "2026-04-10T10:03:30.000Z",
        finishedAt: "2026-04-10T10:04:30.000Z",
        updatedAt: "2026-04-10T10:04:30.000Z",
      },
    ],
    checkCounts: {
      passed: 1,
      failed: 0,
      cancelled: 0,
      skipped: 0,
      blocked: 0,
      total: 1,
    },
    updatedAt: "2026-04-10T10:04:30.000Z",
  };
}

function createWorkflowTaskAttemptInspector(
  workflowTaskAttemptId = "workflow-task-attempt-1",
): WorkspaceWorkflowTaskAttemptInspector {
  return {
    workflowTaskAttemptId,
    workflowRunId: "workflow-1",
    smithersRunId: "smithers-run-1",
    nodeId: "assistant",
    iteration: 0,
    attempt: 1,
    title: "assistant",
    kind: "agent",
    status: "completed",
    summary: "Transcript probe completed.",
    updatedAt: "2026-04-10T10:03:30.000Z",
    commandCount: 1,
    artifactCount: 0,
    transcriptMessageCount: 2,
    contextBudget: null,
    surfacePiSessionId: "pi-task-agent-1",
    smithersState: "finished",
    prompt: "Summarize the transcript probe.",
    responseText: '{"reply":"Handled: Summarize the transcript probe."}',
    error: null,
    cached: false,
    jjPointer: null,
    jjCwd: null,
    heartbeatAt: null,
    agentId: "svvy-deterministic-transcript-agent",
    agentModel: "gpt-4o",
    agentEngine: "pi",
    agentResume: "/tmp/task-agent-session.json",
    meta: null,
    startedAt: "2026-04-10T10:03:00.000Z",
    finishedAt: "2026-04-10T10:03:30.000Z",
    transcript: [
      {
        messageId: "workflow-task-message-1",
        role: "user",
        source: "prompt",
        text: "Summarize the transcript probe.",
        createdAt: "2026-04-10T10:03:00.000Z",
      },
      {
        messageId: "workflow-task-message-2",
        role: "assistant",
        source: "responseText",
        text: '{"reply":"Handled: Summarize the transcript probe."}',
        createdAt: "2026-04-10T10:03:30.000Z",
      },
    ],
    commandRollups: [],
    artifacts: [],
  };
}

function createMemoryStorage(): ChatStorage {
  const providerKeys = new Map<string, string>();
  const customProviders = new Map<string, CustomProvider>();
  const promptHistory = new Map<string, PromptHistoryEntry[]>();

  return {
    providerKeys: {
      get: async (provider: string) => providerKeys.get(provider) ?? null,
      set: async (provider: string, key: string) => {
        providerKeys.set(provider, key);
      },
      delete: async (provider: string) => {
        providerKeys.delete(provider);
      },
      list: async () => Array.from(providerKeys.keys()),
      has: async (provider: string) => providerKeys.has(provider),
    },
    customProviders: {
      get: async (id: string) => customProviders.get(id) ?? null,
      set: async (provider: CustomProvider) => {
        customProviders.set(provider.id, provider);
      },
      delete: async (id: string) => {
        customProviders.delete(id);
      },
      getAll: async () => Array.from(customProviders.values()),
      has: async (id: string) => customProviders.has(id),
    },
    promptHistory: {
      list: async (workspaceId: string) => promptHistory.get(workspaceId) ?? [],
      append: async (entry: PromptHistoryEntry) => {
        const existing = promptHistory.get(entry.workspaceId) ?? [];
        const next = [...existing, entry];
        promptHistory.set(entry.workspaceId, next);
        return entry;
      },
    },
  } as ChatStorage;
}

function createWorkspaceRestoreState(
  layout: WorkspaceUiRestoreState["layouts"]["A"],
  activeLayoutId: "A" | "B" | "C" = "A",
): WorkspaceUiRestoreState {
  return {
    version: 5,
    layouts: {
      A: activeLayoutId === "A" ? layout : null,
      B: activeLayoutId === "B" ? layout : null,
      C: activeLayoutId === "C" ? layout : null,
    },
  };
}

function createFakeRpc(input: {
  sessions: WorkspaceSessionSummary[];
  surfaces: ConversationSurfaceSnapshot[];
  commandInspector?: WorkspaceCommandInspector;
  handlerThreads?: WorkspaceHandlerThreadSummary[];
  handlerThreadInspector?: WorkspaceHandlerThreadInspector;
  workflowTaskAttemptInspector?: WorkspaceWorkflowTaskAttemptInspector;
  projectCiStatus?: WorkspaceProjectCiStatusPanel;
}): FakeRpcHarness {
  const workspaceSyncListeners = new Set<(payload: WorkspaceSyncMessage) => void>();
  const surfaceSyncListeners = new Set<(payload: SurfaceSyncMessage) => void>();
  const appLogUpdateListeners = new Set<(payload: AppLogUpdateMessage) => void>();
  const summaries = new Map(
    input.sessions.map((summary) => [summary.id, structuredClone(summary)]),
  );
  const surfaces = new Map<string, SurfaceRecord>(
    input.surfaces.map((snapshot) => [
      snapshot.target.surfacePiSessionId,
      { snapshot: structuredClone(snapshot), retainCount: 0 },
    ]),
  );
  const promptHandlers = new Map<string, PromptHandler>();
  const pendingPromptSurfaces = new Set<string>();
  const cancelledPromptSurfaces = new Set<string>();
  let archivedGroupCollapsed = true;
  const openedTargets: PromptTarget[] = [];
  const closeRequests: PromptTarget[] = [];
  const promptRequests: SendPromptRequest[] = [];
  const modelUpdates: Array<{ target: PromptTarget; model: string }> = [];
  const thoughtLevelUpdates: Array<{ target: PromptTarget; level: ReasoningEffort }> = [];
  const cancelRequests: PromptTarget[] = [];
  const commandInspectorRequests: Array<{ sessionId: string; commandId: string }> = [];
  const handlerThreadListRequests: string[] = [];
  const handlerThreadInspectorRequests: Array<{ sessionId: string; threadId: string }> = [];
  const workflowTaskAttemptInspectorRequests: Array<{
    sessionId: string;
    workflowTaskAttemptId: string;
  }> = [];
  const projectCiStatusRequests: string[] = [];
  const appLogSeenRequests: number[] = [];
  const branchListRequests: string[] = [];
  const branchSwitchRequests: Array<{ workspaceId: string; branch: string }> = [];
  const workspaceUiRestore = new Map<string, AppWorkspaceUiRestoreState>();
  let workspaceInfo = structuredClone(TEST_WORKSPACE_INFO);
  let appLogEntries: AppLogEntry[] = [];
  let appLogSeenSeq = 0;
  const requestCounts = {
    listSessions: 0,
  };
  let focusedSurfacePiSessionId: string | null = null;
  let queuedMessageSequence = 0;

  const listSessions = (): WorkspaceSessionSummary[] =>
    Array.from(summaries.values()).map((summary) => structuredClone(summary));

  const listNavigation = () =>
    buildWorkspaceSessionNavigation(listSessions(), archivedGroupCollapsed);

  const getSurfaceRecord = (surfacePiSessionId: string): SurfaceRecord => {
    const record = surfaces.get(surfacePiSessionId) ?? null;
    if (!record) {
      throw new Error(`Missing fake surface ${surfacePiSessionId}`);
    }
    return record;
  };

  const updateSummary = (
    sessionId: string,
    updater: (summary: WorkspaceSessionSummary) => void,
  ): void => {
    const summary = summaries.get(sessionId) ?? null;
    if (!summary) {
      throw new Error(`Missing fake workspace session ${sessionId}`);
    }
    updater(summary);
  };

  const emitWorkspaceSync = (
    reason: WorkspaceSyncMessage["reason"] = "workspace.updated",
    workspaceId = TEST_WORKSPACE_INFO.workspaceId,
  ): void => {
    const payload: WorkspaceSyncMessage = {
      workspaceId,
      reason,
      sessions: listSessions(),
      navigation: listNavigation(),
    };
    for (const listener of workspaceSyncListeners) {
      listener(structuredClone(payload));
    }
  };

  const emitSurfaceSync = (
    payload: Omit<SurfaceSyncMessage, "workspaceId"> & { workspaceId?: string },
  ): void => {
    const scopedPayload: SurfaceSyncMessage = {
      ...payload,
      workspaceId: payload.workspaceId ?? TEST_WORKSPACE_INFO.workspaceId,
    };
    if (scopedPayload.reason === "surface.closed") {
      surfaces.delete(scopedPayload.target.surfacePiSessionId);
    } else if (scopedPayload.snapshot) {
      const existing = surfaces.get(scopedPayload.target.surfacePiSessionId);
      if (existing) {
        existing.snapshot = structuredClone(scopedPayload.snapshot);
      } else {
        surfaces.set(scopedPayload.target.surfacePiSessionId, {
          snapshot: structuredClone(scopedPayload.snapshot),
          retainCount: 0,
        });
      }
    }

    for (const listener of surfaceSyncListeners) {
      listener(structuredClone(scopedPayload));
    }
  };

  const summarizeAppLogs = (): AppLogSummary => {
    const latestSeq = appLogEntries.at(-1)?.seq ?? 0;
    const totals = {
      total: appLogEntries.length,
      info: appLogEntries.filter((entry) => entry.level === "info").length,
      warning: appLogEntries.filter((entry) => entry.level === "warning").length,
      error: appLogEntries.filter((entry) => entry.level === "error").length,
    };
    const unreadEntries = appLogEntries.filter((entry) => entry.seq > appLogSeenSeq);
    return {
      latestSeq,
      seenSeq: appLogSeenSeq,
      unread: {
        total: unreadEntries.length,
        info: unreadEntries.filter((entry) => entry.level === "info").length,
        warning: unreadEntries.filter((entry) => entry.level === "warning").length,
        error: unreadEntries.filter((entry) => entry.level === "error").length,
      },
      totals,
    };
  };

  const emitAppLogUpdate = (payload: AppLogUpdateMessage): void => {
    const known = new Set(appLogEntries.map((entry) => entry.id));
    appLogEntries = [
      ...appLogEntries,
      ...payload.entries.filter((entry) => !known.has(entry.id)),
    ].toSorted((left, right) => left.seq - right.seq);
    for (const listener of appLogUpdateListeners) {
      listener(structuredClone(payload));
    }
  };

  const emitAssistantStream = (
    target: PromptTarget,
    text: string,
    provider: string,
    model: string,
  ): void => {
    const partial = assistantMessage("", { provider, model });
    partial.content = [{ type: "text", text }];
    const record = getSurfaceRecord(target.surfacePiSessionId);
    record.snapshot = {
      ...record.snapshot,
      target: cloneTarget(target),
      streamMessage: partial,
      promptStatus: "streaming",
    };
    emitSurfaceSync({
      reason: "surface.updated",
      target: cloneTarget(target),
      snapshot: structuredClone(record.snapshot),
    });
  };

  let promptLibraryState: PromptLibraryState = {
    version: 1,
    revision: 1,
    updatedAt: new Date(0).toISOString(),
    instructionBlocks: {},
    contextPacks: {},
    actorRecipes: {
      orchestrator: {
        actor: "orchestrator",
        instructionBlockIds: [],
        contextPackIds: [],
        generatedSectionIds: ["execute-typescript"],
      },
      handler: {
        actor: "handler",
        instructionBlockIds: [],
        contextPackIds: [],
        generatedSectionIds: ["execute-typescript"],
      },
      "workflow-task": {
        actor: "workflow-task",
        instructionBlockIds: [],
        contextPackIds: [],
        generatedSectionIds: ["execute-typescript"],
      },
    },
  };

  const harness: FakeRpcHarness = {
    client: {
      request: {
        getDefaults: async () => ({
          provider: "openai",
          model: "gpt-4o",
          reasoningEffort: "medium",
        }),
        getAgentSettings: async () => ({
          version: 1,
          sessionAgents: {
            defaultSession: {
              provider: "openai",
              model: "gpt-4o",
              reasoningEffort: "medium",
              systemPrompt: "Default",
            },
            dumbOrchestrator: {
              provider: "openai",
              model: "gpt-4o-mini",
              reasoningEffort: "low",
              systemPrompt: "Dumb",
            },
            namer: {
              provider: "openai-codex",
              model: "gpt-5.4-mini",
              reasoningEffort: "low",
              systemPrompt: "Name the session",
            },
          },
          workflowAgents: {
            explorer: {
              id: "explorer",
              label: "Explorer",
              provider: "openai",
              model: "gpt-4o",
              reasoningEffort: "medium",
              systemPrompt: "Explore",
              toolSurface: ["execute_typescript"],
            },
            implementer: {
              id: "implementer",
              label: "Implementer",
              provider: "openai",
              model: "gpt-4o",
              reasoningEffort: "medium",
              systemPrompt: "Implement",
              toolSurface: ["execute_typescript"],
            },
            reviewer: {
              id: "reviewer",
              label: "Reviewer",
              provider: "openai",
              model: "gpt-4o",
              reasoningEffort: "medium",
              systemPrompt: "Review",
              toolSurface: ["execute_typescript"],
            },
          },
          appPreferences: {
            appAppearance: "system",
            preferredExternalEditor: "system",
            customExternalEditorCommand: "",
            webProvider: null,
          },
        }),
        getPromptLibrary: async () => structuredClone(promptLibraryState),
        getPromptLibraryDefaults: async () => structuredClone(promptLibraryState),
        updatePromptLibrary: async ({ state }) => {
          promptLibraryState = structuredClone(state);
          return structuredClone(promptLibraryState);
        },
        resetPromptLibrary: async () => {
          promptLibraryState = {
            ...promptLibraryState,
            revision: promptLibraryState.revision + 1,
            updatedAt: new Date().toISOString(),
          };
          return structuredClone(promptLibraryState);
        },
        listPromptLibrarySnapshots: async () => [],
        createPromptLibrarySnapshot: async ({ name }) => ({
          id: "snapshot-1",
          name,
          createdAt: new Date().toISOString(),
          revision: promptLibraryState.revision,
          contentKey: getPromptLibraryContentKey(promptLibraryState),
        }),
        renamePromptLibrarySnapshot: async ({ snapshotId, name }) => ({
          id: snapshotId,
          name,
          createdAt: new Date().toISOString(),
          revision: promptLibraryState.revision,
          contentKey: getPromptLibraryContentKey(promptLibraryState),
        }),
        restorePromptLibrarySnapshot: async () => structuredClone(promptLibraryState),
        getOpenWorkspaces: async () => [structuredClone(TEST_WORKSPACE_INFO)],
        updateSessionAgentDefault: async ({ key, settings, workspaceId }) => {
          return {
            ...(await harness.client.request.getAgentSettings({ workspaceId })),
            sessionAgents: {
              ...(await harness.client.request.getAgentSettings({ workspaceId })).sessionAgents,
              [key]: settings,
            },
          };
        },
        updateWorkflowAgent: async ({ key, settings, workspaceId }) => {
          return {
            ...(await harness.client.request.getAgentSettings({ workspaceId })),
            workflowAgents: {
              ...(await harness.client.request.getAgentSettings({ workspaceId })).workflowAgents,
              [key]: settings,
            },
          };
        },
        updateAppPreferences: async ({ workspaceId, ...preferences }) => {
          return {
            ...(await harness.client.request.getAgentSettings({ workspaceId })),
            appPreferences: preferences,
          };
        },
        ensureWorkflowAgentsComponent: async () => ({
          path: "/tmp/svvy/.svvy/workflows/components/agents.ts",
        }),
        getProviderAuthState: async () => ({ connected: true, accountId: "openai-oauth" }),
        getWorkspaceInfo: async () => structuredClone(workspaceInfo),
        getWorkspaceUiRestore: async ({ workspaceId }) =>
          structuredClone(workspaceUiRestore.get(workspaceId) ?? null),
        setWorkspaceUiRestore: async ({ workspaceId, state }) => {
          workspaceUiRestore.set(workspaceId, structuredClone(state));
          return { ok: true };
        },
        listWorkspaceBranches: async ({ workspaceId }) => {
          branchListRequests.push(workspaceId);
          return {
            currentBranch: workspaceInfo.branch,
            branches: ["main", "feature/sidebar"].map((branch) => ({
              name: branch,
              current: branch === workspaceInfo.branch,
            })),
          };
        },
        switchWorkspaceBranch: async ({ workspaceId, branch }) => {
          branchSwitchRequests.push({ workspaceId, branch });
          if (branch === "missing") {
            return {
              ok: false,
              workspace: structuredClone(workspaceInfo),
              error: "Branch is not available in this workspace.",
            };
          }
          workspaceInfo = { ...workspaceInfo, branch };
          return { ok: true, workspace: structuredClone(workspaceInfo) };
        },
        getAppLogs: async (query) => {
          const scopedQuery = query ?? {
            workspaceId: TEST_WORKSPACE_INFO.workspaceId,
          };
          const entries = appLogEntries.filter((entry) => {
            if (scopedQuery.afterSeq !== undefined && entry.seq <= scopedQuery.afterSeq)
              return false;
            if (scopedQuery.levels?.length && !scopedQuery.levels.includes(entry.level))
              return false;
            if (scopedQuery.sources?.length && !scopedQuery.sources.includes(entry.source))
              return false;
            return true;
          });
          return { entries: structuredClone(entries), summary: summarizeAppLogs() };
        },
        getAppLogSummary: async () => summarizeAppLogs(),
        markAppLogsSeen: async ({ throughSeq }) => {
          appLogSeenRequests.push(throughSeq);
          appLogSeenSeq = Math.max(appLogSeenSeq, throughSeq);
          return summarizeAppLogs();
        },
        writeClipboardText: async () => ({ ok: true }),
        listWorkspacePaths: async () => [
          { kind: "file", workspaceRelativePath: "docs/progress.md" },
          { kind: "folder", workspaceRelativePath: "src/mainview/" },
        ],
        pickWorkspaceAttachments: async () => ({
          attachments: [
            {
              id: "file:docs/progress.md",
              kind: "file",
              name: "progress.md",
              path: "docs/progress.md",
              workspaceRelativePath: "docs/progress.md",
            },
          ],
          skippedPaths: [],
        }),
        importComposerAttachments: async () => ({ attachments: [], skippedPaths: [] }),
        openWorkspacePath: async ({ workspaceRelativePath }) => ({
          opened: workspaceRelativePath === "docs/progress.md",
          kind: workspaceRelativePath === "docs/progress.md" ? "file" : "missing",
        }),
        getSavedWorkflowLibrary: async () => ({
          rootPath: ".svvy/workflows",
          artifactRootPath: ".svvy/artifacts/workflows",
          items: [],
          counts: {
            definition: 0,
            prompt: 0,
            component: 0,
            entry: 0,
            "artifact-workflow": 0,
          },
          diagnostics: [],
          preferredExternalEditor: "system",
          customExternalEditorCommand: "",
          updatedAt: new Date(0).toISOString(),
        }),
        deleteSavedWorkflowLibraryItem: async () => ({
          rootPath: ".svvy/workflows",
          artifactRootPath: ".svvy/artifacts/workflows",
          items: [],
          counts: {
            definition: 0,
            prompt: 0,
            component: 0,
            entry: 0,
            "artifact-workflow": 0,
          },
          diagnostics: [],
          preferredExternalEditor: "system",
          customExternalEditorCommand: "",
          updatedAt: new Date(0).toISOString(),
        }),
        openWorkspaceSourceInEditor: async ({ path }) => ({
          opened: true,
          editor: "system",
          path,
        }),
        openPromptLibraryExternalSourceInEditor: async ({ path }) => ({
          opened: true,
          editor: "system",
          path,
        }),
        getPromptLibraryGeneratedEntries: async () => ({
          orchestrator: [],
          handler: [],
          "workflow-task": [],
        }),
        getPromptLibraryExternalSources: async () => [],
        listSessions: async () => {
          requestCounts.listSessions += 1;
          return { sessions: listSessions(), navigation: listNavigation() };
        },
        getCommandInspector: async ({ sessionId, commandId }) => {
          commandInspectorRequests.push({ sessionId, commandId });
          return structuredClone(input.commandInspector ?? createCommandInspector(commandId));
        },
        listHandlerThreads: async ({ sessionId }) => {
          handlerThreadListRequests.push(sessionId);
          return structuredClone(
            input.handlerThreads ?? [createHandlerThreadSummary(`thread-for-${sessionId}`)],
          );
        },
        getHandlerThreadInspector: async ({ sessionId, threadId }) => {
          handlerThreadInspectorRequests.push({ sessionId, threadId });
          return structuredClone(
            input.handlerThreadInspector ?? createHandlerThreadInspector(threadId),
          );
        },
        getWorkflowTaskAttemptInspector: async ({ sessionId, workflowTaskAttemptId }) => {
          workflowTaskAttemptInspectorRequests.push({
            sessionId,
            workflowTaskAttemptId,
          });
          return structuredClone(
            input.workflowTaskAttemptInspector ??
              createWorkflowTaskAttemptInspector(workflowTaskAttemptId),
          );
        },
        getWorkflowInspector: async ({ sessionId, workflowRunId }) => ({
          surfaceId: `workflow-inspector:${workflowRunId}`,
          workflowRunId,
          smithersRunId: "smithers-run-1",
          owningSessionId: sessionId,
          owningThreadId: "thread-1",
          selectedNodeKey: "root",
          expandedNodeKeys: ["root"],
          mode: { kind: "live" },
          runHeader: {
            svvyStatus: "running",
            smithersStatus: "running",
            runId: "smithers-run-1",
            workflowId: "workflow-1",
            workflowLabel: "Workflow",
            owningHandlerThreadTitle: "Thread",
            startedAt: null,
            finishedAt: null,
            updatedAt: null,
            heartbeatAt: null,
            lastEventAt: null,
            frameNo: null,
            frameCount: 0,
            lastSeq: null,
          },
          tree: {
            nodes: [],
            visibleNodeKeys: [],
            searchQuery: "",
            matchedNodeKeys: [],
          },
          frames: [],
          selectedNode: null,
          detailTabs: [],
          rawSnapshot: null,
        }),
        streamWorkflowInspector: async ({ sessionId, workflowRunId, fromSeq }) => ({
          workflowRunId,
          smithersRunId: "smithers-run-1",
          fromSeq: fromSeq ?? null,
          lastSeq: fromSeq ?? null,
          events: [],
          inspector: {
            surfaceId: `workflow-inspector:${workflowRunId}`,
            workflowRunId,
            smithersRunId: "smithers-run-1",
            owningSessionId: sessionId,
            owningThreadId: "thread-1",
            selectedNodeKey: "root",
            expandedNodeKeys: ["root"],
            mode: { kind: "live" },
            runHeader: {
              svvyStatus: "running",
              smithersStatus: "running",
              runId: "smithers-run-1",
              workflowId: "workflow-1",
              workflowLabel: "Workflow",
              owningHandlerThreadTitle: "Thread",
              startedAt: null,
              finishedAt: null,
              updatedAt: null,
              heartbeatAt: null,
              lastEventAt: null,
              frameNo: null,
              frameCount: 0,
              lastSeq: fromSeq ?? null,
            },
            tree: { nodes: [], visibleNodeKeys: [], searchQuery: "", matchedNodeKeys: [] },
            frames: [],
            selectedNode: null,
            detailTabs: [],
            rawSnapshot: null,
          },
        }),
        getProjectCiStatus: async ({ sessionId }) => {
          projectCiStatusRequests.push(sessionId);
          return structuredClone(input.projectCiStatus ?? createProjectCiStatusPanel());
        },
        getArtifactPreview: async ({ sessionId, artifactId }) => ({
          artifactId,
          sessionId,
          kind: "text",
          name: `${artifactId}.txt`,
          createdAt: "2026-04-10T10:04:30.000Z",
          missingFile: false,
          content: `artifact ${artifactId}`,
        }),
        createSession: async ({ title }) => {
          const sessionId = `session-${summaries.size + 1}`;
          const summary = createSummary(sessionId, title ?? "New Session", "");
          const snapshot = createSurfaceSnapshot({
            target: createOrchestratorTarget(sessionId),
            messages: [],
          });
          summaries.set(sessionId, summary);
          surfaces.set(sessionId, { snapshot, retainCount: 1 });
          return structuredClone(snapshot);
        },
        openSession: async ({ sessionId }) => {
          const record = getSurfaceRecord(sessionId);
          record.retainCount += 1;
          openedTargets.push(cloneTarget(record.snapshot.target));
          return structuredClone(record.snapshot);
        },
        recordSessionOpened: async ({ sessionId }) => {
          openedTargets.push(cloneTarget(createOrchestratorTarget(sessionId)));
          return { ok: true };
        },
        openSurface: async ({ target }) => {
          const record = getSurfaceRecord(target.surfacePiSessionId);
          record.retainCount += 1;
          record.snapshot = {
            ...record.snapshot,
            target: cloneTarget(target),
          };
          openedTargets.push(cloneTarget(target));
          return structuredClone(record.snapshot);
        },
        closeSurface: async ({ target }) => {
          closeRequests.push(cloneTarget(target));
          const record = getSurfaceRecord(target.surfacePiSessionId);
          record.retainCount = Math.max(0, record.retainCount - 1);
          if (record.retainCount === 0 && record.snapshot.promptStatus !== "streaming") {
            queueMicrotask(() => {
              emitSurfaceSync({
                reason: "surface.closed",
                target: cloneTarget(target),
              });
            });
          }
          return { ok: true };
        },
        renameSession: async ({ sessionId, title }) => {
          updateSummary(sessionId, (summary) => {
            summary.title = title;
          });
          return { ok: true };
        },
        setSessionMode: async ({ target, mode }) => {
          const record = getSurfaceRecord(target.surfacePiSessionId);
          const snapshot = createSurfaceSnapshot({
            ...record.snapshot,
            sessionMode: mode,
            sessionAgentKey: mode === "dumb" ? "dumbOrchestrator" : "defaultSession",
            systemPrompt: mode === "dumb" ? "Dumb" : "Default",
            resolvedSystemPrompt: mode === "dumb" ? "Dumb" : "Default",
          });
          record.snapshot = snapshot;
          return { ok: true, snapshot: structuredClone(snapshot) };
        },
        forkSession: async ({ sessionId, title }) => {
          const sourceSummary = summaries.get(sessionId) ?? null;
          const sourceSurface = getSurfaceRecord(sessionId).snapshot;
          if (!sourceSummary) {
            throw new Error(`Missing source session ${sessionId}`);
          }
          const nextSessionId = `session-${summaries.size + 1}`;
          const summary = createSummary(
            nextSessionId,
            title ?? `${sourceSummary.title} fork`,
            sourceSummary.preview,
            sourceSurface.reasoningEffort,
            { parentSessionId: sessionId },
          );
          const snapshot = createSurfaceSnapshot({
            target: createOrchestratorTarget(nextSessionId),
            messages: sourceSurface.messages,
            provider: sourceSurface.provider,
            model: sourceSurface.model,
            reasoningEffort: sourceSurface.reasoningEffort,
            systemPrompt: sourceSurface.systemPrompt,
            resolvedSystemPrompt: sourceSurface.resolvedSystemPrompt,
          });
          summaries.set(nextSessionId, summary);
          surfaces.set(nextSessionId, { snapshot, retainCount: 1 });
          return structuredClone(snapshot);
        },
        deleteSession: async ({ sessionId }) => {
          summaries.delete(sessionId);
          for (const [surfacePiSessionId, record] of surfaces.entries()) {
            if (record.snapshot.target.workspaceSessionId === sessionId) {
              surfaces.delete(surfacePiSessionId);
            }
          }
          return { ok: true };
        },
        pinSession: async ({ sessionId }) => {
          updateSummary(sessionId, (summary) => {
            summary.isPinned = true;
            summary.pinnedAt = "2026-04-10T10:10:00.000Z";
            summary.isArchived = false;
            summary.archivedAt = null;
          });
          return { ok: true };
        },
        unpinSession: async ({ sessionId }) => {
          updateSummary(sessionId, (summary) => {
            summary.isPinned = false;
            summary.pinnedAt = null;
          });
          return { ok: true };
        },
        archiveSession: async ({ sessionId }) => {
          updateSummary(sessionId, (summary) => {
            summary.isArchived = true;
            summary.archivedAt = "2026-04-10T10:10:00.000Z";
            summary.isPinned = false;
            summary.pinnedAt = null;
          });
          return { ok: true };
        },
        unarchiveSession: async ({ sessionId }) => {
          updateSummary(sessionId, (summary) => {
            summary.isArchived = false;
            summary.archivedAt = null;
          });
          return { ok: true };
        },
        markSessionUnread: async ({ sessionId }) => {
          updateSummary(sessionId, (summary) => {
            summary.isUnread = true;
            summary.unreadAt = "2026-04-10T10:10:00.000Z";
            summary.unreadReason = "manual";
          });
          return { ok: true };
        },
        markSessionRead: async ({ sessionId }) => {
          updateSummary(sessionId, (summary) => {
            summary.isUnread = false;
            summary.unreadAt = null;
            summary.unreadReason = null;
            summary.lastReadAt = "2026-04-10T10:11:00.000Z";
          });
          return { ok: true };
        },
        recordFocusedSession: async ({ sessionId, surfacePiSessionId }) => {
          focusedSurfacePiSessionId = sessionId ? (surfacePiSessionId ?? null) : null;
          if (sessionId) {
            updateSummary(sessionId, (summary) => {
              summary.isUnread = false;
              summary.unreadAt = null;
              summary.unreadReason = null;
              summary.lastReadAt = "2026-04-10T10:11:00.000Z";
            });
          }
          return { ok: true };
        },
        setArchivedGroupCollapsed: async ({ collapsed }) => {
          archivedGroupCollapsed = collapsed;
          return { ok: true };
        },
        setSessionNavigationSectionState: async ({ section, collapsed }) => {
          if (section === "archived" && typeof collapsed === "boolean") {
            archivedGroupCollapsed = collapsed;
          }
          return { ok: true };
        },
        sendPrompt: async (request) => {
          const record = getSurfaceRecord(request.target.surfacePiSessionId);
          const pendingUserMessage =
            (request.messages as AgentMessage[]).findLast((message) => message.role === "user") ??
            null;
          if (request.queueOnly || record.snapshot.promptStatus === "streaming") {
            record.snapshot = {
              ...record.snapshot,
              queuedMessages: [
                ...record.snapshot.queuedMessages,
                {
                  id: `queued-${++queuedMessageSequence}`,
                  kind: "user_message",
                  text:
                    pendingUserMessage && typeof pendingUserMessage.content !== "string"
                      ? pendingUserMessage.content
                          .map((block) => (block.type === "text" ? block.text : ""))
                          .join("")
                      : String(pendingUserMessage?.content ?? ""),
                  status: "queued",
                  createdAt: "2026-04-10T10:12:00.000Z",
                  updatedAt: "2026-04-10T10:12:00.000Z",
                },
              ],
            };
            queueMicrotask(() => {
              emitSurfaceSync({
                reason: "surface.updated",
                target: cloneTarget(request.target),
                snapshot: structuredClone(record.snapshot),
              });
            });
            return {
              target: cloneTarget(request.target),
              queued: true,
              snapshot: structuredClone(record.snapshot),
            };
          }

          promptRequests.push(structuredClone(request));
          pendingPromptSurfaces.add(request.target.surfacePiSessionId);
          record.snapshot = {
            ...record.snapshot,
            target: cloneTarget(request.target),
            pendingUserMessage: pendingUserMessage ? structuredClone(pendingUserMessage) : null,
            streamMessage: null,
            streamSequence: 0,
            promptStatus: "streaming",
            activeTurnId: `turn-${promptRequests.length}`,
            activeTurnStartedAt: "2026-04-10T10:12:00.000Z",
          };
          updateSummary(request.target.workspaceSessionId, (summary) => {
            summary.status = "running";
          });
          emitWorkspaceSync("workspace.updated");
          queueMicrotask(() => {
            emitSurfaceSync({
              reason: "background.started",
              target: cloneTarget(request.target),
              snapshot: structuredClone(record.snapshot),
            });
          });
          const promptHandler =
            promptHandlers.get(request.target.surfacePiSessionId) ?? defaultPromptHandler;
          const result = await promptHandler(structuredClone(request), harness);
          const cancelled = cancelledPromptSurfaces.has(request.target.surfacePiSessionId);
          pendingPromptSurfaces.delete(request.target.surfacePiSessionId);
          if (cancelled) {
            cancelledPromptSurfaces.delete(request.target.surfacePiSessionId);
            return { target: cloneTarget(request.target) };
          }

          const provider = request.provider ?? record.snapshot.provider;
          const model = request.model ?? record.snapshot.model;
          const nextMessages = [
            ...(request.messages as AgentMessage[]),
            ...(result.extraMessages ? structuredClone(result.extraMessages) : []),
            assistantMessage(result.assistantText, { provider, model }),
          ];

          record.snapshot = {
            ...record.snapshot,
            target: cloneTarget(request.target),
            messages: nextMessages,
            provider,
            model,
            reasoningEffort: request.reasoningEffort ?? record.snapshot.reasoningEffort,
            systemPrompt: request.systemPrompt ?? record.snapshot.systemPrompt,
            pendingUserMessage: null,
            streamMessage: null,
            promptStatus: "idle",
            activeTurnId: null,
            activeTurnStartedAt: null,
          };

          updateSummary(request.target.workspaceSessionId, (summary) => {
            summary.preview = result.assistantText;
            summary.messageCount = nextMessages.length;
            summary.status = "idle";
            if (focusedSurfacePiSessionId !== request.target.surfacePiSessionId) {
              summary.isUnread = true;
              summary.unreadAt = "2026-04-10T10:12:00.000Z";
              summary.unreadReason = "assistant-turn-finished";
            }
          });

          queueMicrotask(() => {
            const surfaceSyncPayload: SurfaceSyncMessage = {
              workspaceId: TEST_WORKSPACE_INFO.workspaceId,
              reason: result.reason ?? "prompt.settled",
              target: cloneTarget(request.target),
              snapshot: structuredClone(record.snapshot),
            };
            if (result.emitSurfaceSyncBeforeStreamDone) {
              emitSurfaceSync(surfaceSyncPayload);
            } else {
              emitAssistantStream(request.target, result.assistantText, provider, model);
              emitSurfaceSync(surfaceSyncPayload);
            }
            emitWorkspaceSync("workspace.updated");
            const [nextQueued, ...remainingQueued] = record.snapshot.queuedMessages;
            if (nextQueued) {
              record.snapshot = { ...record.snapshot, queuedMessages: remainingQueued };
              void harness.client.request.sendPrompt({
                ...request,
                messages: [
                  ...(record.snapshot.messages as AgentMessage[]),
                  userMessage(nextQueued.text),
                ] as Message[],
              });
            }
          });

          return { target: cloneTarget(request.target) };
        },
        editCommittedUserMessage: async ({ target, messageTimestamp, message, workspaceId }) => {
          const record = getSurfaceRecord(target.surfacePiSessionId);
          const messages = record.snapshot.messages as AgentMessage[];
          const editIndex = messages.findIndex(
            (candidate) =>
              candidate.role === "user" && String(candidate.timestamp) === String(messageTimestamp),
          );
          if (editIndex < 0) {
            throw new Error(
              "Unable to edit: user message was not found in the active conversation.",
            );
          }
          return await harness.client.request.sendPrompt({
            workspaceId,
            target,
            messages: [...messages.slice(0, editIndex), message] as Message[],
          });
        },
        updateComposerDraft: async ({ target, draft }) => {
          const record = getSurfaceRecord(target.surfacePiSessionId);
          record.snapshot = {
            ...record.snapshot,
            composerDraft: {
              text: draft.text,
              attachments: structuredClone(draft.attachments),
              updatedAt:
                draft.text.trim() || draft.attachments.length > 0
                  ? "2026-04-10T10:12:00.000Z"
                  : null,
            },
          };
          return {
            ok: true,
            target: cloneTarget(target),
            snapshot: structuredClone(record.snapshot),
          };
        },
        deleteQueuedSurfaceMessage: async ({ target, queuedMessageId }) => {
          const record = getSurfaceRecord(target.surfacePiSessionId);
          record.snapshot = {
            ...record.snapshot,
            queuedMessages: record.snapshot.queuedMessages.filter(
              (message) => message.id !== queuedMessageId,
            ),
          };
          queueMicrotask(() =>
            emitSurfaceSync({
              reason: "surface.updated",
              target: cloneTarget(target),
              snapshot: structuredClone(record.snapshot),
            }),
          );
          return {
            ok: true,
            target: cloneTarget(target),
            snapshot: structuredClone(record.snapshot),
          };
        },
        editQueuedSurfaceMessage: async ({ target, queuedMessageId }) => {
          const record = getSurfaceRecord(target.surfacePiSessionId);
          const queued = record.snapshot.queuedMessages.find(
            (message) => message.id === queuedMessageId,
          );
          record.snapshot = {
            ...record.snapshot,
            queuedMessages: record.snapshot.queuedMessages.filter(
              (message) => message.id !== queuedMessageId,
            ),
          };
          return {
            ok: true,
            text: queued?.text,
            snapshot: structuredClone(record.snapshot),
          };
        },
        reorderQueuedSurfaceMessage: async ({ target, queuedMessageId, beforeQueuedMessageId }) => {
          const record = getSurfaceRecord(target.surfacePiSessionId);
          const moving = record.snapshot.queuedMessages.find(
            (message) => message.id === queuedMessageId,
          );
          if (moving) {
            const remaining = record.snapshot.queuedMessages.filter(
              (message) => message.id !== queuedMessageId,
            );
            const beforeIndex = beforeQueuedMessageId
              ? remaining.findIndex((message) => message.id === beforeQueuedMessageId)
              : remaining.length;
            record.snapshot = {
              ...record.snapshot,
              queuedMessages: [
                ...remaining.slice(0, beforeIndex < 0 ? remaining.length : beforeIndex),
                moving,
                ...remaining.slice(beforeIndex < 0 ? remaining.length : beforeIndex),
              ],
            };
          }
          queueMicrotask(() =>
            emitSurfaceSync({
              reason: "surface.updated",
              target: cloneTarget(target),
              snapshot: structuredClone(record.snapshot),
            }),
          );
          return {
            ok: true,
            target: cloneTarget(target),
            snapshot: structuredClone(record.snapshot),
          };
        },
        steerQueuedSurfaceMessage: async ({ target, queuedMessageId }) => {
          const record = getSurfaceRecord(target.surfacePiSessionId);
          record.snapshot = {
            ...record.snapshot,
            queuedMessages: record.snapshot.queuedMessages.map((message) =>
              message.id === queuedMessageId
                ? { ...message, status: "steering", updatedAt: "2026-04-10T10:13:00.000Z" }
                : message,
            ),
          };
          queueMicrotask(() =>
            emitSurfaceSync({
              reason: "surface.updated",
              target: cloneTarget(target),
              snapshot: structuredClone(record.snapshot),
            }),
          );
          return {
            ok: true,
            target: cloneTarget(target),
            snapshot: structuredClone(record.snapshot),
          };
        },
        queuePromptRefresh: async ({ target }) => {
          const record = getSurfaceRecord(target.surfacePiSessionId);
          const existing = record.snapshot.queuedMessages.find(
            (message) => message.kind === "prompt_refresh",
          );
          if (!existing) {
            record.snapshot = {
              ...record.snapshot,
              queuedMessages: [
                {
                  id: `queued-${record.snapshot.queuedMessages.length + 1}`,
                  kind: "prompt_refresh",
                  text: "Update instructions",
                  summary: "Context revision 2",
                  status: "queued",
                  createdAt: "2026-04-10T10:12:00.000Z",
                  updatedAt: "2026-04-10T10:12:00.000Z",
                },
                ...record.snapshot.queuedMessages,
              ],
            };
          }
          queueMicrotask(() =>
            emitSurfaceSync({
              reason: "surface.updated",
              target: cloneTarget(target),
              snapshot: structuredClone(record.snapshot),
            }),
          );
          return {
            ok: true,
            target: cloneTarget(target),
            snapshot: structuredClone(record.snapshot),
          };
        },
        setSurfaceModel: async ({ target, provider, model }) => {
          modelUpdates.push({ target: cloneTarget(target), model });
          const record = getSurfaceRecord(target.surfacePiSessionId);
          record.snapshot = {
            ...record.snapshot,
            provider,
            model,
          };
          if (target.surface === "orchestrator") {
            updateSummary(target.workspaceSessionId, (summary) => {
              summary.provider = provider;
              summary.modelId = model;
            });
          }
          queueMicrotask(() => {
            emitSurfaceSync({
              reason: "surface.updated",
              target: cloneTarget(target),
              snapshot: structuredClone(record.snapshot),
            });
          });
          return { ok: true, target: cloneTarget(target) };
        },
        setSurfaceThoughtLevel: async ({ target, level }) => {
          thoughtLevelUpdates.push({ target: cloneTarget(target), level });
          const record = getSurfaceRecord(target.surfacePiSessionId);
          record.snapshot = {
            ...record.snapshot,
            reasoningEffort: level,
          };
          if (target.surface === "orchestrator") {
            updateSummary(target.workspaceSessionId, (summary) => {
              summary.thinkingLevel = level;
            });
          }
          queueMicrotask(() => {
            emitSurfaceSync({
              reason: "surface.updated",
              target: cloneTarget(target),
              snapshot: structuredClone(record.snapshot),
            });
          });
          return { ok: true, target: cloneTarget(target) };
        },
        cancelPrompt: async ({ target }) => {
          cancelRequests.push(cloneTarget(target));
          if (pendingPromptSurfaces.has(target.surfacePiSessionId)) {
            cancelledPromptSurfaces.add(target.surfacePiSessionId);
          }
          const record = getSurfaceRecord(target.surfacePiSessionId);
          record.snapshot = {
            ...record.snapshot,
            pendingUserMessage: null,
            streamMessage: null,
            promptStatus: "idle",
            activeTurnId: null,
            activeTurnStartedAt: null,
          };
          queueMicrotask(() => {
            emitSurfaceSync({
              reason: "prompt.settled",
              target: cloneTarget(target),
              snapshot: structuredClone(record.snapshot),
            });
          });
          return { ok: true };
        },
        listProviderAuths: async () => [
          { provider: "openai", hasKey: true, keyType: "oauth", supportsOAuth: true },
        ],
        setProviderApiKey: async () => ({ ok: true }),
        startOAuth: async () => ({ ok: true }),
        removeProviderAuth: async () => ({ ok: true }),
      },
      addMessageListener: (messageName: string, listener: unknown) => {
        if (messageName === "sendWorkspaceSync") {
          workspaceSyncListeners.add(listener as (payload: WorkspaceSyncMessage) => void);
          return;
        }
        if (messageName === "sendSurfaceSync") {
          surfaceSyncListeners.add(listener as (payload: SurfaceSyncMessage) => void);
          return;
        }
        if (messageName === "sendAppLogUpdate") {
          appLogUpdateListeners.add(listener as (payload: AppLogUpdateMessage) => void);
        }
      },
      removeMessageListener: (messageName: string, listener: unknown) => {
        if (messageName === "sendWorkspaceSync") {
          workspaceSyncListeners.delete(listener as (payload: WorkspaceSyncMessage) => void);
          return;
        }
        if (messageName === "sendSurfaceSync") {
          surfaceSyncListeners.delete(listener as (payload: SurfaceSyncMessage) => void);
          return;
        }
        if (messageName === "sendAppLogUpdate") {
          appLogUpdateListeners.delete(listener as (payload: AppLogUpdateMessage) => void);
        }
      },
    },
    openedTargets,
    closeRequests,
    promptRequests,
    modelUpdates,
    thoughtLevelUpdates,
    cancelRequests,
    requestCounts,
    commandInspectorRequests,
    handlerThreadListRequests,
    handlerThreadInspectorRequests,
    workflowTaskAttemptInspectorRequests,
    projectCiStatusRequests,
    appLogSeenRequests,
    branchListRequests,
    branchSwitchRequests,
    setPromptHandler: (surfacePiSessionId, handler) => {
      promptHandlers.set(surfacePiSessionId, handler);
    },
    updateSummary,
    emitWorkspaceSync,
    emitSurfaceSync,
    emitAppLogUpdate,
    getRetainCount: (surfacePiSessionId) => surfaces.get(surfacePiSessionId)?.retainCount ?? 0,
    getSurfaceSnapshot: (surfacePiSessionId) =>
      structuredClone(getSurfaceRecord(surfacePiSessionId).snapshot),
    getWorkspaceUiRestore: (workspaceId) =>
      structuredClone((workspaceUiRestore.get(workspaceId) as WorkspaceUiRestoreState) ?? null),
    setWorkspaceUiRestore: (workspaceId, state) => {
      workspaceUiRestore.set(workspaceId, structuredClone(state));
    },
  };

  return harness;
}

async function createRuntime(
  harness: FakeRpcHarness,
  storage = createMemoryStorage(),
  workspaceInfo = TEST_WORKSPACE_INFO,
  options: { seedInitialLayout?: boolean } = {},
) {
  if (
    options.seedInitialLayout !== false &&
    workspaceInfo.kind === "user" &&
    !harness.getWorkspaceUiRestore(workspaceInfo.workspaceId)
  ) {
    const catalog = await harness.client.request.listSessions({
      workspaceId: workspaceInfo.workspaceId,
    });
    const initialSession = catalog.sessions[0];
    if (initialSession) {
      try {
        harness.getSurfaceSnapshot(initialSession.id);
        harness.setWorkspaceUiRestore(
          workspaceInfo.workspaceId,
          createWorkspaceRestoreState({
            dockview: null,
            compactSurfaces: [],
            panels: [
              {
                panelId: "primary",
                binding: createOrchestratorTarget(initialSession.id),
                localState: {
                  scroll: null,
                  timelineDensity: "comfortable",
                },
              },
            ],
            focusedPanelId: "primary",
            updatedAt: "2026-04-27T00:00:00.000Z",
          }),
        );
      } catch {
        // Tests without a surface snapshot exercise empty-layout startup.
      }
    }
  }
  const { createChatRuntime } = await import("./chat-runtime");
  return await createChatRuntime({ workspaceInfo }, harness.client as never, storage);
}

describe("createChatRuntime", () => {
  it("hydrates the primary pane from an orchestrator surface and keeps the resolved prompt separate", async () => {
    const rawPrompt = "You are svvy.";
    const resolvedPrompt =
      "You are svvy.\n\n# Project Context\n\nCurrent date: 2026-04-21\nCurrent working directory: /tmp/svvy";
    const harness = createFakeRpc({
      sessions: [createSummary("session-1", "Prompt Channel", "done")],
      surfaces: [
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-1"),
          messages: [userMessage("inspect"), assistantMessage("done")],
          systemPrompt: rawPrompt,
          resolvedSystemPrompt: resolvedPrompt,
          externalContextSources: [
            {
              id: "0:/tmp/svvy/AGENTS.md",
              kind: "AGENTS.md",
              title: "AGENTS.md",
              path: "/tmp/svvy/AGENTS.md",
              content: "# Standards",
              contentHash: "abc123",
              order: 0,
            },
          ],
        }),
      ],
    });

    const runtime = await createRuntime(harness);
    const controller = runtime.getPaneController(runtime.primaryPaneId);

    expect(runtime.sessions).toHaveLength(1);
    expect(runtime.paneLayout.focusedPanelId).toBe(runtime.primaryPaneId);
    expect(runtime.getPane(runtime.primaryPaneId)?.target).toEqual(
      createOrchestratorTarget("session-1"),
    );
    expect(controller).toBeTruthy();
    expect(controller?.agent.state.systemPrompt).toBe(rawPrompt);
    expect(controller?.resolvedSystemPrompt).toContain("# Project Context");
    expect(controller?.externalContextSources).toEqual([
      expect.objectContaining({ path: "/tmp/svvy/AGENTS.md", contentHash: "abc123" }),
    ]);
    expect(controller?.agent.state.messages.at(-1)).toMatchObject({
      role: "assistant",
    });

    runtime.dispose();
  });

  it("keeps workspace pane state and live surface state separate when multiple surfaces are open", async () => {
    const threadTarget = createThreadTarget("session-1", "thread-session-1", "thread-123");
    const harness = createFakeRpc({
      sessions: [
        createSummary("session-1", "Orchestrator", "main reply"),
        createSummary("session-2", "Second", "second reply", "high"),
      ],
      surfaces: [
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-1"),
          messages: [userMessage("main"), assistantMessage("main reply")],
        }),
        createSurfaceSnapshot({
          target: threadTarget,
          messages: [userMessage("worker context"), assistantMessage("worker ready")],
        }),
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-2"),
          messages: [userMessage("second"), assistantMessage("second reply")],
          reasoningEffort: "high",
        }),
      ],
    });

    const runtime = await createRuntime(harness);
    await runtime.openSurface(threadTarget, "secondary");

    const primaryController = runtime.getPaneController(runtime.primaryPaneId);
    const secondaryController = runtime.getPaneController("secondary");

    expect(runtime.paneLayout.focusedPanelId).toBe("secondary");
    expect(runtime.getPane(runtime.primaryPaneId)?.target).toEqual(
      createOrchestratorTarget("session-1"),
    );
    expect(runtime.getPane("secondary")?.target).toEqual(threadTarget);
    expect(primaryController?.target).toEqual(createOrchestratorTarget("session-1"));
    expect(secondaryController?.target).toEqual(threadTarget);
    expect(primaryController).not.toBe(secondaryController);
    expect(harness.openedTargets.at(-1)).toEqual(threadTarget);

    runtime.dispose();
  });

  it("shares one live surface controller across panes and only releases it after the last pane closes", async () => {
    const threadTarget = createThreadTarget("session-1", "thread-session-1", "thread-123");
    const harness = createFakeRpc({
      sessions: [createSummary("session-1", "Orchestrator", "main reply")],
      surfaces: [
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-1"),
          messages: [userMessage("main"), assistantMessage("main reply")],
        }),
        createSurfaceSnapshot({
          target: threadTarget,
          messages: [userMessage("worker"), assistantMessage("worker ready")],
        }),
      ],
    });

    const runtime = await createRuntime(harness);
    await runtime.openSurface(threadTarget, "secondary");
    await runtime.openSurface(threadTarget, "tertiary");

    const secondaryController = runtime.getPaneController("secondary");
    const tertiaryController = runtime.getPaneController("tertiary");

    expect(secondaryController).toBeTruthy();
    expect(secondaryController).toBe(tertiaryController);
    expect(harness.getRetainCount(threadTarget.surfacePiSessionId)).toBe(1);

    await runtime.closePaneSurface("secondary");

    expect(runtime.getPane("secondary")).toBeUndefined();
    expect(runtime.getPaneController("tertiary")).toBe(tertiaryController);
    expect(runtime.getSurfaceController(threadTarget.surfacePiSessionId)).toBe(tertiaryController);
    expect(harness.closeRequests).toHaveLength(0);
    expect(harness.getRetainCount(threadTarget.surfacePiSessionId)).toBe(1);

    await runtime.closePaneSurface("tertiary");
    await waitFor(() => runtime.getSurfaceController(threadTarget.surfacePiSessionId) === null);

    expect(runtime.getPane("tertiary")).toBeUndefined();
    expect(harness.closeRequests).toHaveLength(1);
    expect(harness.getRetainCount(threadTarget.surfacePiSessionId)).toBe(0);

    runtime.dispose();
  });

  it("releases a closed pane without disposing a streaming surface and reopens from a fresh snapshot", async () => {
    const threadTarget = createThreadTarget("session-1", "thread-session-1", "thread-123");
    const freshStreamMessage = assistantMessage("fresh worker state");
    const harness = createFakeRpc({
      sessions: [createSummary("session-1", "Orchestrator", "main reply")],
      surfaces: [
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-1"),
          messages: [userMessage("main"), assistantMessage("main reply")],
        }),
        createSurfaceSnapshot({
          target: threadTarget,
          messages: [userMessage("worker")],
          streamMessage: assistantMessage("still working"),
          promptStatus: "streaming",
        }),
      ],
    });

    const runtime = await createRuntime(harness);
    await runtime.openSurface(threadTarget, "secondary");

    await runtime.closePaneSurface("secondary");

    expect(runtime.getPane("secondary")).toBeUndefined();
    expect(harness.closeRequests).toHaveLength(1);
    expect(harness.getRetainCount(threadTarget.surfacePiSessionId)).toBe(0);
    expect(runtime.getSurfaceController(threadTarget.surfacePiSessionId)?.promptStatus).toBe(
      "streaming",
    );

    harness.emitSurfaceSync({
      reason: "surface.updated",
      target: threadTarget,
      snapshot: createSurfaceSnapshot({
        target: threadTarget,
        messages: [userMessage("worker")],
        streamMessage: freshStreamMessage,
        promptStatus: "streaming",
      }),
    });
    await runtime.openSurface(threadTarget, "secondary");
    expect(harness.getRetainCount(threadTarget.surfacePiSessionId)).toBe(1);
    const reopenedStream = runtime.getPaneController("secondary")?.agent.state.streamMessage;
    expect(reopenedStream?.role === "assistant" ? reopenedStream.content[0] : null).toMatchObject({
      type: "text",
      text: "fresh worker state",
    });

    runtime.dispose();
  });

  it("removes the final pane instead of leaving an empty pane behind", async () => {
    const harness = createFakeRpc({
      sessions: [createSummary("session-1", "Orchestrator", "main reply")],
      surfaces: [
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-1"),
          messages: [userMessage("main"), assistantMessage("main reply")],
        }),
      ],
    });

    const runtime = await createRuntime(harness);

    await runtime.closePane(runtime.primaryPaneId);
    await waitFor(() => runtime.getSurfaceController("session-1") === null);

    expect(runtime.paneLayout.panels).toHaveLength(0);
    expect(runtime.paneLayout.focusedPanelId).toBeNull();
    expect(runtime.paneLayout.dockview).toBeNull();
    expect(harness.closeRequests).toHaveLength(1);

    await runtime.createSession({}, { kind: "new-panel", direction: "right" });

    expect(runtime.paneLayout.panels).toHaveLength(1);
    expect(runtime.paneLayout.panels[0]?.binding).toEqual(createOrchestratorTarget("session-2"));
    expect(runtime.paneLayout.panels.some((panel) => panel.binding === null)).toBe(false);

    runtime.dispose();
  });

  it("ignores stale pane close events for panels already removed from runtime state", async () => {
    const harness = createFakeRpc({
      sessions: [
        createSummary("session-1", "Orchestrator", "main reply"),
        createSummary("session-2", "Second", "second reply"),
      ],
      surfaces: [
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-1"),
          messages: [userMessage("main"), assistantMessage("main reply")],
        }),
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-2"),
          messages: [userMessage("second"), assistantMessage("second reply")],
        }),
      ],
    });

    const runtime = await createRuntime(harness);
    await runtime.openSession("session-2", "secondary");

    expect(runtime.paneLayout.panels.map((panel) => panel.panelId)).toEqual([
      "primary",
      "secondary",
    ]);

    await runtime.closePane("primary");
    await runtime.closePane("primary");

    expect(runtime.getPane("primary")).toBeUndefined();
    expect(runtime.getPane("secondary")?.target).toEqual(createOrchestratorTarget("session-2"));
    expect(runtime.paneLayout.panels.map((panel) => panel.panelId)).toEqual(["secondary"]);
    expect(runtime.paneLayout.panels.some((panel) => panel.binding === null)).toBe(false);

    runtime.dispose();
  });

  it("deletes the final session without creating a replacement session", async () => {
    const harness = createFakeRpc({
      sessions: [createSummary("session-1", "Only Session", "main reply")],
      surfaces: [
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-1"),
          messages: [userMessage("main"), assistantMessage("main reply")],
        }),
      ],
    });

    const runtime = await createRuntime(harness);

    await runtime.deleteSession("session-1", runtime.primaryPaneId);

    expect(runtime.sessions).toEqual([]);
    expect(runtime.paneLayout.panels).toHaveLength(0);
    expect(runtime.paneLayout.focusedPanelId).toBeNull();
    expect(harness.client.request.listSessions).toBeDefined();

    runtime.dispose();
  });

  it("keeps prompt dispatch independent across concurrent surfaces", async () => {
    const threadTarget = createThreadTarget("session-1", "thread-session-1", "thread-123");
    const orchestratorGate = createDeferred<void>();
    const handlerGate = createDeferred<void>();
    const harness = createFakeRpc({
      sessions: [createSummary("session-1", "Orchestrator", "ready")],
      surfaces: [
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-1"),
          messages: [assistantMessage("ready")],
        }),
        createSurfaceSnapshot({
          target: threadTarget,
          messages: [assistantMessage("worker ready")],
        }),
      ],
    });

    harness.setPromptHandler("session-1", async () => {
      await orchestratorGate.promise;
      return { assistantText: "Orchestrator settled." };
    });
    harness.setPromptHandler(threadTarget.surfacePiSessionId, async () => {
      await handlerGate.promise;
      return { assistantText: "Handler settled." };
    });

    const runtime = await createRuntime(harness);
    await runtime.openSurface(threadTarget, "secondary");
    const orchestratorController = runtime.getPaneController(runtime.primaryPaneId);
    const handlerController = runtime.getPaneController("secondary");
    if (!orchestratorController || !handlerController) {
      throw new Error("Expected both surface controllers.");
    }

    const orchestratorPrompt = orchestratorController.sendPrompt({
      text: "Continue orchestrating",
      attachments: [],
    });
    const handlerPrompt = handlerController.sendPrompt({
      text: "Continue handling",
      attachments: [],
    });

    await waitFor(
      () =>
        orchestratorController.promptStatus === "streaming" &&
        handlerController.promptStatus === "streaming",
    );

    handlerGate.resolve();
    await handlerPrompt;
    await handlerController.agent.waitForIdle();

    expect(handlerController.promptStatus).toBe("idle");
    expect(orchestratorController.promptStatus).toBe("streaming");

    orchestratorGate.resolve();
    await orchestratorPrompt;
    await orchestratorController.agent.waitForIdle();

    expect(harness.promptRequests.map((request) => request.target.surfacePiSessionId)).toEqual([
      "session-1",
      threadTarget.surfacePiSessionId,
    ]);
    expect(
      handlerController.agent.state.messages.some(
        (message) =>
          message.role === "assistant" &&
          message.content[0]?.type === "text" &&
          message.content[0].text === "Handler settled.",
      ),
    ).toBe(true);
    expect(
      orchestratorController.agent.state.messages.some(
        (message) =>
          message.role === "assistant" &&
          message.content[0]?.type === "text" &&
          message.content[0].text === "Orchestrator settled.",
      ),
    ).toBe(true);

    runtime.dispose();
  });

  it("queues prompts sent to a streaming surface and dispatches them after the active turn settles", async () => {
    const session = createSummary("session-1", "Parser", "Initial");
    const target = createOrchestratorTarget(session.id);
    const harness = createFakeRpc({
      sessions: [session],
      surfaces: [
        createSurfaceSnapshot({
          target,
          messages: [userMessage("Initial"), assistantMessage("Ready")],
        }),
      ],
    });
    const runtime = await createRuntime(harness);
    const controller = runtime.getPaneController("primary");
    expect(controller).not.toBeNull();
    if (!controller) return;

    const activeTurn = createDeferred<PromptHandlerResult>();
    harness.setPromptHandler(target.surfacePiSessionId, () => activeTurn.promise);

    const firstPrompt = controller.sendPrompt({ text: "Run the first turn", attachments: [] });
    await waitFor(() => controller.promptStatus === "streaming");

    await controller.sendPrompt({ text: "Follow up while streaming", attachments: [] });

    expect(harness.promptRequests).toHaveLength(1);
    expect(controller.queuedPrompts.map((prompt) => prompt.text)).toEqual([
      "Follow up while streaming",
    ]);

    activeTurn.resolve({ assistantText: "First turn done" });
    await firstPrompt;
    await waitFor(() => harness.promptRequests.length >= 2);

    const queuedRequest = harness.promptRequests[1];
    expect(queuedRequest).toBeDefined();
    if (!queuedRequest) return;
    const queuedUserMessage = (queuedRequest.messages as AgentMessage[]).findLast(
      (message) => message.role === "user",
    );
    expect(queuedUserMessage?.content).toEqual([
      { type: "text", text: "Follow up while streaming" },
    ]);
    expect(controller.queuedPrompts).toEqual([]);

    runtime.dispose();
  });

  it("sends composer image attachments as tagged attachment metadata plus image content", async () => {
    const session = createSummary("session-1", "Vision", "Initial");
    const target = createOrchestratorTarget(session.id);
    const harness = createFakeRpc({
      sessions: [session],
      surfaces: [
        createSurfaceSnapshot({
          target,
          messages: [userMessage("Initial"), assistantMessage("Ready")],
        }),
      ],
    });
    const runtime = await createRuntime(harness);
    const controller = runtime.getPaneController("primary");
    expect(controller).not.toBeNull();
    if (!controller) return;

    await controller.sendPrompt({
      text: "What changed?",
      attachments: [
        {
          id: "file:docs/progress.md",
          kind: "file",
          name: "progress.md",
          path: "docs/progress.md",
          workspaceRelativePath: "docs/progress.md",
          mimeType: "text/markdown",
          sizeBytes: 1200,
        },
        {
          id: "attachment:.svvy/attachments/user-input/proof.png",
          kind: "image",
          name: "proof.png",
          path: ".svvy/attachments/user-input/proof.png",
          workspaceRelativePath: ".svvy/attachments/user-input/proof.png",
          mimeType: "image/png",
          dataBase64: "aW1hZ2U=",
        },
      ],
    });

    const request = harness.promptRequests[0];
    const user = request?.messages.findLast((message) => message.role === "user");
    const attachmentMetadata =
      Array.isArray(user?.content) && user.content[1]?.type === "text"
        ? parseComposerAttachmentTextSignature(user.content[1].textSignature)
        : [];
    expect(user?.content).toEqual([
      { type: "text", text: "What changed?" },
      {
        type: "text",
        text: "Attached files are available at these workspace-relative paths:\n- file path: docs/progress.md (name: progress.md)\n- image path: .svvy/attachments/user-input/proof.png (name: proof.png)",
        textSignature: expect.any(String),
      },
      { type: "image", data: "aW1hZ2U=", mimeType: "image/png" },
    ]);
    expect(attachmentMetadata).toEqual([
      {
        id: "file:docs/progress.md",
        kind: "file",
        name: "progress.md",
        path: "docs/progress.md",
        workspaceRelativePath: "docs/progress.md",
        mimeType: "text/markdown",
        sizeBytes: 1200,
      },
      {
        id: "attachment:.svvy/attachments/user-input/proof.png",
        kind: "image",
        name: "proof.png",
        path: ".svvy/attachments/user-input/proof.png",
        workspaceRelativePath: ".svvy/attachments/user-input/proof.png",
        mimeType: "image/png",
        sizeBytes: undefined,
      },
    ]);

    runtime.dispose();
  });

  it("keeps an optimistic sent message visible when an older idle snapshot arrives", async () => {
    const session = createSummary("session-1", "Draft Race", "Initial");
    const target = createOrchestratorTarget(session.id);
    const initialMessages = [userMessage("Initial"), assistantMessage("Ready")];
    const harness = createFakeRpc({
      sessions: [session],
      surfaces: [
        createSurfaceSnapshot({
          target,
          messages: initialMessages,
        }),
      ],
    });
    const runtime = await createRuntime(harness);
    const controller = runtime.getPaneController("primary");
    expect(controller).not.toBeNull();
    if (!controller) return;

    const activeTurn = createDeferred<PromptHandlerResult>();
    harness.setPromptHandler(target.surfacePiSessionId, () => activeTurn.promise);

    const sendPromise = controller.sendPrompt({ text: "Keep this visible", attachments: [] });
    await waitFor(() => controller.promptStatus === "streaming");
    expect(hasUserText(controller.agent.state.messages, "Keep this visible")).toBe(true);

    harness.emitSurfaceSync({
      reason: "surface.updated",
      target,
      snapshot: createSurfaceSnapshot({
        target,
        messages: initialMessages,
        promptStatus: "idle",
      }),
    });

    expect(controller.promptStatus).toBe("streaming");
    expect(hasUserText(controller.agent.state.messages, "Keep this visible")).toBe(true);

    activeTurn.resolve({ assistantText: "Done" });
    await sendPromise;
    runtime.dispose();
  });

  it("edits, deletes, promotes, and reorders queued prompts without touching the active prompt", async () => {
    const session = createSummary("session-1", "Parser", "Initial");
    const target = createOrchestratorTarget(session.id);
    const harness = createFakeRpc({
      sessions: [session],
      surfaces: [
        createSurfaceSnapshot({
          target,
          messages: [userMessage("Initial"), assistantMessage("Ready")],
        }),
      ],
    });
    const runtime = await createRuntime(harness);
    const controller = runtime.getPaneController("primary");
    expect(controller).not.toBeNull();
    if (!controller) return;

    const activeTurn = createDeferred<PromptHandlerResult>();
    harness.setPromptHandler(target.surfacePiSessionId, () => activeTurn.promise);

    const activePrompt = controller.sendPrompt({ text: "Active turn", attachments: [] });
    await waitFor(() => controller.promptStatus === "streaming");
    await controller.sendPrompt({ text: "First queued", attachments: [] });
    await controller.sendPrompt({ text: "Second queued", attachments: [] });
    await controller.sendPrompt({ text: "Third queued", attachments: [] });

    const [first, second, third] = controller.queuedPrompts;
    expect(first).toBeDefined();
    expect(second).toBeDefined();
    expect(third).toBeDefined();
    if (!first || !second || !third) return;
    const editedText = await controller.editQueuedPrompt(second.id);
    expect(editedText).toBe("Second queued");
    await controller.sendPrompt({ text: "Second queued, revised", attachments: [] });
    expect(await controller.reorderQueuedPrompt(third.id, first.id)).toBe(true);
    expect(await controller.deleteQueuedPrompt(first.id)).toBe(true);
    expect(await controller.steerQueuedPrompt(third.id)).toBe(true);

    expect(harness.promptRequests).toHaveLength(1);
    expect(controller.queuedPrompts.map((prompt) => [prompt.text, prompt.status])).toEqual([
      ["Third queued", "steering"],
      ["Second queued, revised", "queued"],
    ]);

    activeTurn.resolve({ assistantText: "Active turn done" });
    await activePrompt;

    runtime.dispose();
  });

  it("edits a committed user message by continuing from that message point", async () => {
    const session = createSummary("session-1", "Parser", "Initial");
    const target = createOrchestratorTarget(session.id);
    const firstUser = userMessage("Original request");
    firstUser.timestamp = 101;
    const firstAssistant = assistantMessage("Original reply");
    firstAssistant.timestamp = 102;
    const secondUser = userMessage("Follow-up");
    secondUser.timestamp = 103;
    const secondAssistant = assistantMessage("Follow-up reply");
    secondAssistant.timestamp = 104;
    const harness = createFakeRpc({
      sessions: [session],
      surfaces: [
        createSurfaceSnapshot({
          target,
          messages: [firstUser, firstAssistant, secondUser, secondAssistant],
        }),
      ],
    });
    const runtime = await createRuntime(harness);
    const controller = runtime.getPaneController("primary");
    expect(controller).not.toBeNull();
    if (!controller) return;

    await controller.editCommittedUserMessage(101, {
      text: "Revised request",
      attachments: [],
    });

    await waitFor(() => controller.agent.state.messages.length === 2);
    const committedUserMessages = controller.agent.state.messages.filter(
      (message) => message.role === "user",
    );
    expect(
      committedUserMessages.map((message) =>
        typeof message.content === "string"
          ? message.content
          : message.content.map((block) => (block.type === "text" ? block.text : "")).join(""),
      ),
    ).toEqual(["Revised request"]);

    const [promptRequest] = harness.promptRequests;
    expect(promptRequest?.messages).toHaveLength(1);

    runtime.dispose();
  });

  it("queues and cancels prompt refresh control items through the surface controller", async () => {
    const session = createSummary("session-1", "Parser", "Initial");
    const target = createOrchestratorTarget(session.id);
    const harness = createFakeRpc({
      sessions: [session],
      surfaces: [
        createSurfaceSnapshot({
          target,
          messages: [userMessage("Initial"), assistantMessage("Ready")],
        }),
      ],
    });
    const runtime = await createRuntime(harness);
    const controller = runtime.getPaneController("primary");
    expect(controller).not.toBeNull();
    if (!controller) return;

    expect(await controller.queuePromptRefresh()).toBe(true);
    expect(controller.queuedPrompts.map((prompt) => [prompt.kind, prompt.text])).toEqual([
      ["prompt_refresh", "Update instructions"],
    ]);
    const refresh = controller.queuedPrompts[0];
    expect(refresh).toBeDefined();
    if (!refresh) return;

    expect(await controller.deleteQueuedPrompt(refresh.id)).toBe(true);
    expect(controller.queuedPrompts).toEqual([]);

    runtime.dispose();
  });

  it("does not duplicate the assistant reply when a settled surface snapshot arrives before stream completion", async () => {
    const harness = createFakeRpc({
      sessions: [createSummary("session-1", "Orchestrator", "ready")],
      surfaces: [
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-1"),
          messages: [],
        }),
      ],
    });
    harness.setPromptHandler("session-1", async () => ({
      assistantText: "Single settled reply.",
      emitSurfaceSyncBeforeStreamDone: true,
    }));

    const runtime = await createRuntime(harness);
    const controller = runtime.getPaneController(runtime.primaryPaneId);
    if (!controller) {
      throw new Error("Expected an orchestrator controller.");
    }

    await controller.sendPrompt({ text: "Greet me", attachments: [] });
    await waitFor(() =>
      controller.agent.state.messages.some(
        (message) =>
          message.role === "assistant" &&
          message.content[0]?.type === "text" &&
          message.content[0].text === "Single settled reply.",
      ),
    );

    const replies = controller.agent.state.messages.filter(
      (message) =>
        message.role === "assistant" &&
        message.content[0]?.type === "text" &&
        message.content[0].text === "Single settled reply.",
    );
    expect(replies).toHaveLength(1);

    runtime.dispose();
  });

  it("keeps sidebar state live for a background prompt after its pane closes", async () => {
    const promptGate = createDeferred<void>();
    const harness = createFakeRpc({
      sessions: [createSummary("session-1", "Background", "ready")],
      surfaces: [
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-1"),
          messages: [],
        }),
      ],
    });
    harness.setPromptHandler("session-1", async () => {
      await promptGate.promise;
      return { assistantText: "Finished in the background." };
    });

    const runtime = await createRuntime(harness);
    const controller = runtime.getPaneController(runtime.primaryPaneId);
    if (!controller) {
      throw new Error("Expected an orchestrator controller.");
    }

    const prompt = controller.sendPrompt({ text: "Run in the background", attachments: [] });
    await waitFor(
      () => runtime.sessions.find((session) => session.id === "session-1")?.status === "running",
    );

    await runtime.closePaneSurface(runtime.primaryPaneId);
    expect(runtime.sessions.find((session) => session.id === "session-1")?.status).toBe("running");

    promptGate.resolve();
    await prompt;
    await waitFor(
      () => runtime.sessions.find((session) => session.id === "session-1")?.isUnread === true,
    );
    expect(runtime.sessions.find((session) => session.id === "session-1")?.preview).toBe(
      "Finished in the background.",
    );

    runtime.dispose();
  });

  it("marks an open but unfocused pane unread when its assistant turn finishes", async () => {
    const harness = createFakeRpc({
      sessions: [
        createSummary("session-1", "Focused", "ready"),
        createSummary("session-2", "Unfocused", "ready"),
      ],
      surfaces: [
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-1"),
          messages: [],
        }),
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-2"),
          messages: [],
        }),
      ],
    });

    const runtime = await createRuntime(harness);
    await runtime.openSession("session-2", {
      kind: "split",
      panelId: runtime.primaryPaneId,
      direction: "right",
    });
    const session2PaneId = runtime.paneLayout.focusedPanelId;
    if (!session2PaneId) {
      throw new Error("Expected session 2 pane to be focused after opening.");
    }
    runtime.focusPane(runtime.primaryPaneId);

    const controller = runtime.getSurfaceController("session-2");
    if (!controller) {
      throw new Error("Expected session 2 controller.");
    }

    await controller.sendPrompt({ text: "Finish while unfocused", attachments: [] });
    await waitFor(
      () => runtime.sessions.find((session) => session.id === "session-2")?.isUnread === true,
    );
    expect(runtime.sessions.find((session) => session.id === "session-2")?.unreadReason).toBe(
      "assistant-turn-finished",
    );

    runtime.dispose();
  });

  it("renders pending user and surface-owned stream state from snapshots", async () => {
    const threadTarget = createThreadTarget("session-1", "thread-session-1", "thread-123");
    const pendingUser = userMessage("Inspect the repo");
    const liveAssistant = assistantMessage("Scanning files now...");
    const harness = createFakeRpc({
      sessions: [createSummary("session-1", "Streaming Handler", "worker running")],
      surfaces: [
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-1"),
          messages: [],
        }),
        createSurfaceSnapshot({
          target: threadTarget,
          messages: [],
          pendingUserMessage: pendingUser,
          streamMessage: liveAssistant,
          promptStatus: "streaming",
          activeTurnId: "turn-live-1",
          activeTurnStartedAt: "2026-04-10T10:12:00.000Z",
          turnTimings: [],
        }),
      ],
    });

    const runtime = await createRuntime(harness);
    await runtime.openSurface(threadTarget, "secondary");
    const controller = runtime.getPaneController("secondary");
    if (!controller) {
      throw new Error("Expected a restored controller.");
    }

    expect(controller.promptStatus).toBe("streaming");
    expect(controller.activeTurnId).toBe("turn-live-1");
    expect(controller.activeTurnStartedAt).toBe("2026-04-10T10:12:00.000Z");
    expect(controller.turnTimings).toEqual([]);
    expect(
      controller.agent.state.messages.some(
        (message) =>
          message.role === "user" &&
          "content" in message &&
          Array.isArray(message.content) &&
          message.content[0]?.type === "text" &&
          message.content[0].text === "Inspect the repo",
      ),
    ).toBe(true);
    const streamMessage = controller.agent.state.streamMessage;
    expect(streamMessage?.role).toBe("assistant");
    expect(streamMessage?.role === "assistant" ? streamMessage.content[0] : null).toMatchObject({
      type: "text",
      text: "Scanning files now...",
    });

    harness.emitSurfaceSync({
      reason: "prompt.settled",
      target: threadTarget,
      snapshot: createSurfaceSnapshot({
        target: threadTarget,
        messages: [pendingUser, liveAssistant],
        promptStatus: "idle",
        turnTimings: [
          {
            turnId: "turn-live-1",
            assistantMessageTimestamp: liveAssistant.timestamp,
            startedAt: "2026-04-10T10:12:00.000Z",
            finishedAt: "2026-04-10T10:12:42.000Z",
          },
        ],
      }),
    });

    expect(controller.agent.state.streamMessage).toBeNull();
    expect(controller.activeTurnId).toBeNull();
    expect(controller.activeTurnStartedAt).toBeNull();
    expect(controller.turnTimings).toEqual([
      {
        turnId: "turn-live-1",
        assistantMessageTimestamp: liveAssistant.timestamp,
        startedAt: "2026-04-10T10:12:00.000Z",
        finishedAt: "2026-04-10T10:12:42.000Z",
      },
    ]);
    expect(
      controller.agent.state.messages.filter(
        (message) =>
          message.role === "assistant" &&
          message.content[0]?.type === "text" &&
          message.content[0].text === "Scanning files now...",
      ),
    ).toHaveLength(1);

    runtime.dispose();
  });

  it("applies ordered stream patches without replacing the surface snapshot", async () => {
    const threadTarget = createThreadTarget("session-1", "thread-session-1", "thread-123");
    const harness = createFakeRpc({
      sessions: [createSummary("session-1", "Streaming Handler", "worker running")],
      surfaces: [
        createSurfaceSnapshot({
          target: threadTarget,
          messages: [userMessage("Inspect the repo")],
        }),
      ],
    });

    const runtime = await createRuntime(harness);
    await runtime.openSurface(threadTarget, "secondary");
    const controller = runtime.getPaneController("secondary");
    if (!controller) {
      throw new Error("Expected a restored controller.");
    }

    const streamMessage = assistantMessage("");
    streamMessage.content = [];
    harness.emitSurfaceSync({
      reason: "stream.patch",
      target: threadTarget,
      streamPatch: { type: "start", sequence: 1, message: streamMessage },
    });
    harness.emitSurfaceSync({
      reason: "stream.patch",
      target: threadTarget,
      streamPatch: { type: "text_start", sequence: 2, contentIndex: 0 },
    });
    harness.emitSurfaceSync({
      reason: "stream.patch",
      target: threadTarget,
      streamPatch: {
        type: "text_delta",
        sequence: 3,
        contentIndex: 0,
        delta: "Scanning",
      },
    });
    const firstDeltaStreamMessage = controller.agent.state.streamMessage;
    harness.emitSurfaceSync({
      reason: "stream.patch",
      target: threadTarget,
      streamPatch: {
        type: "text_delta",
        sequence: 4,
        contentIndex: 0,
        delta: " files",
      },
    });

    expect(controller.agent.state.messages).toHaveLength(1);
    const patchedStreamMessage = controller.agent.state.streamMessage;
    expect(patchedStreamMessage).not.toBe(firstDeltaStreamMessage);
    expect(
      patchedStreamMessage?.role === "assistant" ? patchedStreamMessage.content[0] : null,
    ).toMatchObject({
      type: "text",
      text: "Scanning files",
    });
    expect(controller.promptStatus).toBe("streaming");

    const midStreamSnapshot = createSurfaceSnapshot({
      target: threadTarget,
      messages: [userMessage("Inspect the repo")],
      streamMessage: controller.agent.state.streamMessage as AssistantMessage,
      streamSequence: 4,
      promptStatus: "streaming",
    });
    harness.emitSurfaceSync({
      reason: "surface.updated",
      target: threadTarget,
      snapshot: midStreamSnapshot,
    });
    harness.emitSurfaceSync({
      reason: "stream.patch",
      target: threadTarget,
      streamPatch: {
        type: "text_delta",
        sequence: 5,
        contentIndex: 0,
        delta: " now",
      },
    });

    const continuedStreamMessage = controller.agent.state.streamMessage;
    expect(continuedStreamMessage).not.toBe(patchedStreamMessage);
    expect(
      continuedStreamMessage?.role === "assistant" ? continuedStreamMessage.content[0] : null,
    ).toMatchObject({
      type: "text",
      text: "Scanning files now",
    });

    harness.emitSurfaceSync({
      reason: "stream.patch",
      target: threadTarget,
      streamPatch: { type: "clear", sequence: 6, reason: "done" },
    });

    expect(controller.agent.state.streamMessage).toBeNull();
    expect(controller.promptStatus).toBe("idle");

    runtime.dispose();
  });

  it("accepts a fresh stream sequence when dispatching after a previous stream snapshot", async () => {
    const threadTarget = createThreadTarget("session-1", "thread-session-1", "thread-123");
    const finishPrompt = createDeferred<void>();
    const harness = createFakeRpc({
      sessions: [createSummary("session-1", "Streaming Handler", "worker ready")],
      surfaces: [
        createSurfaceSnapshot({
          target: threadTarget,
          messages: [assistantMessage("Previous turn")],
          streamSequence: 9,
        }),
      ],
    });

    harness.setPromptHandler(threadTarget.surfacePiSessionId, async () => {
      const streamMessage = assistantMessage("");
      streamMessage.content = [];
      harness.emitSurfaceSync({
        reason: "stream.patch",
        target: threadTarget,
        streamPatch: { type: "start", sequence: 1, message: streamMessage },
      });
      harness.emitSurfaceSync({
        reason: "stream.patch",
        target: threadTarget,
        streamPatch: { type: "text_start", sequence: 2, contentIndex: 0 },
      });
      harness.emitSurfaceSync({
        reason: "stream.patch",
        target: threadTarget,
        streamPatch: {
          type: "text_delta",
          sequence: 3,
          contentIndex: 0,
          delta: "Visible fresh stream",
        },
      });
      await finishPrompt.promise;
      return { assistantText: "Final fresh stream" };
    });

    const runtime = await createRuntime(harness);
    await runtime.openSurface(threadTarget, "secondary");
    const controller = runtime.getPaneController("secondary");
    if (!controller) {
      throw new Error("Expected a restored controller.");
    }

    const prompt = controller.sendPrompt({ text: "Run a fresh turn", attachments: [] });
    try {
      await waitFor(() => {
        const streamMessage = controller.agent.state.streamMessage;
        const firstBlock = streamMessage?.role === "assistant" ? streamMessage.content[0] : null;
        return firstBlock?.type === "text" && firstBlock.text === "Visible fresh stream";
      });
    } finally {
      finishPrompt.resolve();
    }
    await prompt;

    runtime.dispose();
  });

  it("accepts the stream start after a streaming snapshot without a stream message", async () => {
    const threadTarget = createThreadTarget("session-1", "thread-session-1", "thread-123");
    const harness = createFakeRpc({
      sessions: [createSummary("session-1", "Streaming Handler", "worker ready")],
      surfaces: [
        createSurfaceSnapshot({
          target: threadTarget,
          messages: [assistantMessage("Previous turn")],
        }),
      ],
    });

    const runtime = await createRuntime(harness);
    await runtime.openSurface(threadTarget, "secondary");
    const controller = runtime.getPaneController("secondary");
    if (!controller) {
      throw new Error("Expected a restored controller.");
    }

    harness.emitSurfaceSync({
      reason: "background.started",
      target: threadTarget,
      snapshot: createSurfaceSnapshot({
        target: threadTarget,
        messages: [assistantMessage("Previous turn"), userMessage("Run another turn")],
        promptStatus: "streaming",
        streamMessage: null,
        streamSequence: 1,
      }),
    });

    const streamMessage = assistantMessage("");
    streamMessage.content = [];
    harness.emitSurfaceSync({
      reason: "stream.patch",
      target: threadTarget,
      streamPatch: { type: "start", sequence: 1, message: streamMessage },
    });
    harness.emitSurfaceSync({
      reason: "stream.patch",
      target: threadTarget,
      streamPatch: { type: "text_start", sequence: 2, contentIndex: 0 },
    });
    harness.emitSurfaceSync({
      reason: "stream.patch",
      target: threadTarget,
      streamPatch: {
        type: "text_delta",
        sequence: 3,
        contentIndex: 0,
        delta: "Visible stream after snapshot",
      },
    });

    const visibleStream = controller.agent.state.streamMessage;
    expect(visibleStream?.role === "assistant" ? visibleStream.content[0] : null).toMatchObject({
      type: "text",
      text: "Visible stream after snapshot",
    });

    runtime.dispose();
  });

  it("keeps model, reasoning, and cancel mutations scoped to the targeted surface", async () => {
    const threadTarget = createThreadTarget("session-1", "thread-session-1", "thread-123");
    const handlerGate = createDeferred<void>();
    const harness = createFakeRpc({
      sessions: [createSummary("session-1", "Orchestrator", "ready")],
      surfaces: [
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-1"),
          messages: [assistantMessage("ready")],
        }),
        createSurfaceSnapshot({
          target: threadTarget,
          messages: [assistantMessage("worker ready")],
        }),
      ],
    });

    harness.setPromptHandler(threadTarget.surfacePiSessionId, async () => {
      await handlerGate.promise;
      return { assistantText: "This should stay cancelled." };
    });

    const runtime = await createRuntime(harness);
    await runtime.openSurface(threadTarget, "secondary");

    const orchestratorController = runtime.getPaneController(runtime.primaryPaneId);
    const handlerController = runtime.getPaneController("secondary");
    if (!orchestratorController || !handlerController) {
      throw new Error("Expected both surface controllers.");
    }

    handlerController.agent.setModel(getModel("openai", "gpt-4.1"));
    handlerController.agent.setThinkingLevel("high");

    await waitFor(
      () => harness.modelUpdates.length === 1 && harness.thoughtLevelUpdates.length === 1,
    );

    expect(harness.modelUpdates[0]).toEqual({
      target: threadTarget,
      model: "gpt-4.1",
    });
    expect(harness.thoughtLevelUpdates[0]).toEqual({
      target: threadTarget,
      level: "high",
    });
    expect(orchestratorController.agent.state.model.id).toBe("gpt-4o");
    expect(orchestratorController.agent.state.thinkingLevel).toBe("medium");

    const handlerPrompt = handlerController.sendPrompt({
      text: "Continue handling",
      attachments: [],
    });
    await waitFor(() => handlerController.promptStatus === "streaming");
    await handlerController.abort();
    await waitFor(() => harness.cancelRequests.length === 1);
    handlerGate.resolve();
    await handlerPrompt;
    await waitFor(() => handlerController.promptStatus === "idle");

    expect(harness.cancelRequests[0]).toEqual(threadTarget);
    expect(orchestratorController.promptStatus).toBe("idle");

    runtime.dispose();
  });

  it("applies workspace summary updates without depending on a global active surface", async () => {
    const threadTarget = createThreadTarget("session-1", "thread-session-1", "thread-123");
    const harness = createFakeRpc({
      sessions: [
        createSummary("session-1", "Orchestrator", "main reply"),
        createSummary("session-2", "Background", "stale summary"),
      ],
      surfaces: [
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-1"),
          messages: [userMessage("main"), assistantMessage("main reply")],
        }),
        createSurfaceSnapshot({
          target: threadTarget,
          messages: [userMessage("worker"), assistantMessage("worker ready")],
        }),
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-2"),
          messages: [userMessage("background"), assistantMessage("done")],
        }),
      ],
    });

    const runtime = await createRuntime(harness);
    await runtime.openSurface(threadTarget, "secondary");

    harness.updateSummary("session-2", (summary) => {
      summary.preview = "Background workflow updated.";
      summary.status = "running";
    });
    harness.emitWorkspaceSync("workspace.updated");

    await waitFor(
      () =>
        runtime.sessions.find((session) => session.id === "session-2")?.preview ===
        "Background workflow updated.",
    );

    expect(runtime.getPane("secondary")?.target).toEqual(threadTarget);
    expect(runtime.paneLayout.focusedPanelId).toBe("secondary");
    expect(harness.requestCounts.listSessions).toBe(2);

    runtime.dispose();
  });

  it("ignores workspace and surface sync messages for other workspace ids", async () => {
    const harness = createFakeRpc({
      sessions: [createSummary("session-1", "Orchestrator", "main reply")],
      surfaces: [
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-1"),
          messages: [assistantMessage("main reply")],
        }),
      ],
    });

    const runtime = await createRuntime(harness);
    const controller = runtime.getPaneController(runtime.primaryPaneId);
    if (!controller) {
      throw new Error("Expected an orchestrator controller.");
    }

    harness.updateSummary("session-1", (summary) => {
      summary.preview = "foreign workspace update";
    });
    harness.emitWorkspaceSync("workspace.updated", "/tmp/other");
    harness.emitSurfaceSync({
      workspaceId: "/tmp/other",
      reason: "surface.updated",
      target: createOrchestratorTarget("session-1"),
      snapshot: createSurfaceSnapshot({
        target: createOrchestratorTarget("session-1"),
        messages: [assistantMessage("foreign surface update")],
      }),
    });

    expect(runtime.sessions.find((session) => session.id === "session-1")?.preview).toBe(
      "main reply",
    );
    const lastMessage = controller.agent.state.messages.at(-1);
    expect(
      lastMessage?.role === "assistant" && "content" in lastMessage ? lastMessage.content[0] : null,
    ).toMatchObject({ text: "main reply" });

    harness.emitWorkspaceSync("workspace.updated");
    await waitFor(
      () =>
        runtime.sessions.find((session) => session.id === "session-1")?.preview ===
        "foreign workspace update",
    );

    runtime.dispose();
  });

  it("uses the focused pane session by default for inspectors", async () => {
    const commandInspector = createCommandInspector("command-77");
    const handlerThreads = [createHandlerThreadSummary("thread-77")];
    const handlerThreadInspector = createHandlerThreadInspector("thread-77");
    const workflowTaskAttemptInspector = createWorkflowTaskAttemptInspector(
      "workflow-task-attempt-77",
    );
    const harness = createFakeRpc({
      sessions: [
        createSummary("session-1", "First", "first reply"),
        createSummary("session-2", "Second", "second reply"),
      ],
      surfaces: [
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-1"),
          messages: [userMessage("first"), assistantMessage("first reply")],
        }),
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-2"),
          messages: [userMessage("second"), assistantMessage("second reply")],
        }),
      ],
      commandInspector,
      handlerThreads,
      handlerThreadInspector,
      workflowTaskAttemptInspector,
    });

    const runtime = await createRuntime(harness);
    await runtime.openSession("session-2", "secondary");

    const detail = await runtime.getCommandInspector("command-77");
    const threads = await runtime.listHandlerThreads();
    const threadDetail = await runtime.getHandlerThreadInspector("thread-77");
    const workflowTaskAttemptDetail = await runtime.getWorkflowTaskAttemptInspector(
      "workflow-task-attempt-77",
    );
    const projectCiStatus = await runtime.getProjectCiStatus();

    expect(detail).toEqual(commandInspector);
    expect(threads).toEqual(handlerThreads);
    expect(threadDetail).toEqual(handlerThreadInspector);
    expect(workflowTaskAttemptDetail).toEqual(workflowTaskAttemptInspector);
    expect(projectCiStatus).toEqual(createProjectCiStatusPanel());
    expect(harness.commandInspectorRequests).toEqual([
      { sessionId: "session-2", commandId: "command-77" },
    ]);
    expect(harness.handlerThreadListRequests).toEqual(["session-2"]);
    expect(harness.handlerThreadInspectorRequests).toEqual([
      { sessionId: "session-2", threadId: "thread-77" },
    ]);
    expect(harness.workflowTaskAttemptInspectorRequests).toEqual([
      { sessionId: "session-2", workflowTaskAttemptId: "workflow-task-attempt-77" },
    ]);
    expect(harness.projectCiStatusRequests).toEqual(["session-2"]);

    runtime.dispose();
  });

  it("opens workspace paths through the runtime without adding agent context metadata", async () => {
    const harness = createFakeRpc({
      sessions: [createSummary("session-1", "First", "first reply")],
      surfaces: [
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-1"),
          messages: [assistantMessage("first reply")],
        }),
      ],
    });
    const runtime = await createRuntime(harness);

    await expect(runtime.openWorkspacePath("docs/progress.md")).resolves.toBe(true);
    await expect(runtime.openWorkspacePath("missing/file.ts")).resolves.toBe(false);

    runtime.dispose();
  });

  it("lists and switches workspace branches through workspace-scoped runtime RPC", async () => {
    const harness = createFakeRpc({
      sessions: [createSummary("session-1", "First", "first reply")],
      surfaces: [
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-1"),
          messages: [assistantMessage("first reply")],
        }),
      ],
    });
    const runtime = await createRuntime(harness);

    await expect(runtime.listWorkspaceBranches()).resolves.toEqual([
      { name: "main", current: true },
      { name: "feature/sidebar", current: false },
    ]);
    await runtime.switchWorkspaceBranch("feature/sidebar");

    expect(runtime.branch).toBe("feature/sidebar");
    expect(harness.branchListRequests).toEqual([TEST_WORKSPACE_INFO.workspaceId]);
    expect(harness.branchSwitchRequests).toEqual([
      { workspaceId: TEST_WORKSPACE_INFO.workspaceId, branch: "feature/sidebar" },
    ]);
    await expect(runtime.switchWorkspaceBranch("missing")).rejects.toThrow(
      "Branch is not available in this workspace.",
    );

    runtime.dispose();
  });

  it("applies pinned, archived, and archived-group navigation mutations from the backend read model", async () => {
    const harness = createFakeRpc({
      sessions: [
        createSummary("session-1", "First", "first reply"),
        createSummary("session-2", "Second", "second reply"),
      ],
      surfaces: [
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-1"),
          messages: [assistantMessage("first reply")],
        }),
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-2"),
          messages: [assistantMessage("second reply")],
        }),
      ],
    });

    const runtime = await createRuntime(harness);
    await runtime.pinSession("session-2");

    expect(runtime.sessionNavigation.pinnedSessions.map((session) => session.id)).toEqual([
      "session-2",
    ]);
    expect(runtime.sessionNavigation.activeSessions.map((session) => session.id)).toEqual([
      "session-1",
    ]);

    await runtime.archiveSession("session-2");
    await runtime.setArchivedGroupCollapsed(false);

    expect(runtime.sessionNavigation.pinnedSessions).toEqual([]);
    expect(runtime.sessionNavigation.archived.collapsed).toBe(false);
    expect(runtime.sessionNavigation.archived.sessions.map((session) => session.id)).toEqual([
      "session-2",
    ]);

    await runtime.unarchiveSession("session-2");
    expect(runtime.sessionNavigation.activeSessions.map((session) => session.id)).toContain(
      "session-2",
    );
    expect(runtime.sessions.find((session) => session.id === "session-2")?.isPinned).toBe(false);

    await runtime.markSessionUnread("session-2");
    expect(runtime.sessions.find((session) => session.id === "session-2")?.isUnread).toBe(true);
    expect(runtime.sessions.find((session) => session.id === "session-2")?.unreadReason).toBe(
      "manual",
    );

    await runtime.markSessionRead("session-2");
    expect(runtime.sessions.find((session) => session.id === "session-2")?.isUnread).toBe(false);
    expect(runtime.sessions.find((session) => session.id === "session-2")?.unreadReason).toBeNull();

    runtime.dispose();
  });

  it("restores pane bindings and focused pane after restart", async () => {
    const storage = createMemoryStorage();
    const threadTarget = createThreadTarget("session-1", "thread-session-1", "thread-123");
    const firstHarness = createFakeRpc({
      sessions: [createSummary("session-1", "Orchestrator", "main reply")],
      surfaces: [
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-1"),
          messages: [assistantMessage("main reply")],
        }),
        createSurfaceSnapshot({
          target: threadTarget,
          messages: [assistantMessage("worker ready")],
        }),
      ],
    });
    const firstRuntime = await createRuntime(firstHarness, storage);
    await firstRuntime.openSurface(threadTarget, "secondary");
    await Bun.sleep(0);
    firstRuntime.dispose();

    const restoreState = firstHarness.getWorkspaceUiRestore(TEST_WORKSPACE_INFO.workspaceId);
    expect(restoreState?.layouts.A?.focusedPanelId).toBe("secondary");
    expect(restoreState?.layouts.A?.panels).toContainEqual(
      expect.objectContaining({
        panelId: "secondary",
        binding: threadTarget,
      }),
    );

    const secondHarness = createFakeRpc({
      sessions: [createSummary("session-1", "Orchestrator", "main reply")],
      surfaces: [
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-1"),
          messages: [assistantMessage("main reply")],
        }),
        createSurfaceSnapshot({
          target: threadTarget,
          messages: [assistantMessage("worker ready")],
        }),
      ],
    });
    secondHarness.setWorkspaceUiRestore(TEST_WORKSPACE_INFO.workspaceId, restoreState!);
    const secondRuntime = await createRuntime(secondHarness, storage);

    expect(secondRuntime.paneLayout.focusedPanelId).toBe("secondary");
    expect(secondRuntime.getPane("secondary")?.target).toEqual(threadTarget);

    secondRuntime.dispose();
  });

  it("restores multiple pane-bound surfaces with one controller per interactive surface", async () => {
    const storage = createMemoryStorage();
    const orchestratorTarget = createOrchestratorTarget("session-1");
    const threadTarget = createThreadTarget("session-1", "thread-session-1", "thread-123");
    const workflowInspectorTarget = {
      workspaceSessionId: "session-1",
      surface: "workflow-inspector" as const,
      workflowRunId: "workflow-1",
    };
    const harness = createFakeRpc({
      sessions: [createSummary("session-1", "Orchestrator", "main reply")],
      surfaces: [
        createSurfaceSnapshot({
          target: orchestratorTarget,
          messages: [assistantMessage("main reply")],
        }),
        createSurfaceSnapshot({
          target: threadTarget,
          messages: [assistantMessage("worker ready")],
        }),
      ],
    });
    harness.setWorkspaceUiRestore(
      TEST_WORKSPACE_INFO.workspaceId,
      createWorkspaceRestoreState({
        dockview: null,
        compactSurfaces: [],
        panels: [
          {
            panelId: "primary",
            binding: orchestratorTarget,
            localState: {
              scroll: { transcriptAnchorId: "assistant-1", offsetPx: 12 },
              timelineDensity: "comfortable",
            },
          },
          {
            panelId: "thread-left",
            binding: threadTarget,
            localState: {
              scroll: null,
              timelineDensity: "compact",
            },
          },
          {
            panelId: "thread-right",
            binding: threadTarget,
            localState: {
              scroll: null,
              timelineDensity: "comfortable",
            },
          },
          {
            panelId: "inspector",
            binding: workflowInspectorTarget,
            localState: {
              scroll: null,
              timelineDensity: "comfortable",
            },
          },
        ],
        focusedPanelId: "thread-right",
        updatedAt: "2026-04-27T00:00:00.000Z",
      }),
    );

    const runtime = await createRuntime(harness, storage);

    expect(runtime.paneLayout.focusedPanelId).toBe("thread-right");
    expect(runtime.getPane("primary")?.target).toEqual(orchestratorTarget);
    expect(runtime.getPane("thread-left")?.target).toEqual(threadTarget);
    expect(runtime.getPane("thread-right")?.target).toEqual(threadTarget);
    expect(runtime.getPane("inspector")?.target).toEqual(workflowInspectorTarget);

    const threadController = runtime.getSurfaceController(threadTarget.surfacePiSessionId);
    expect(threadController?.ownerPaneIds.toSorted()).toEqual(["thread-left", "thread-right"]);
    expect(runtime.getPaneController("thread-left")).toBe(threadController);
    expect(runtime.getPaneController("thread-right")).toBe(threadController);
    expect(runtime.getPaneController("inspector")).toBeNull();
    expect(harness.getRetainCount(threadTarget.surfacePiSessionId)).toBe(1);
    expect(
      harness.openedTargets.filter(
        (target) => target.surfacePiSessionId === threadTarget.surfacePiSessionId,
      ),
    ).toHaveLength(1);

    runtime.dispose();
  });

  it("restores prompt lock state from opened surface snapshots", async () => {
    const storage = createMemoryStorage();
    const threadTarget = createThreadTarget("session-1", "thread-session-1", "thread-123");
    const harness = createFakeRpc({
      sessions: [createSummary("session-1", "Waiting Session", "worker waiting")],
      surfaces: [
        createSurfaceSnapshot({
          target: threadTarget,
          messages: [assistantMessage("still running")],
          promptStatus: "streaming",
        }),
      ],
    });
    harness.setWorkspaceUiRestore(
      TEST_WORKSPACE_INFO.workspaceId,
      createWorkspaceRestoreState({
        dockview: null,
        compactSurfaces: [],
        panels: [
          {
            panelId: "primary",
            binding: threadTarget,
            localState: {
              scroll: null,
              timelineDensity: "comfortable",
            },
          },
        ],
        focusedPanelId: "primary",
        updatedAt: "2026-04-27T00:00:00.000Z",
      }),
    );

    const runtime = await createRuntime(harness, storage);

    expect(runtime.getPaneController("primary")?.promptStatus).toBe("streaming");
    expect(runtime.getSurfaceController(threadTarget.surfacePiSessionId)?.promptStatus).toBe(
      "streaming",
    );

    runtime.dispose();
  });

  it("drops restored empty panes and focuses a restorable bound pane", async () => {
    const storage = createMemoryStorage();
    const harness = createFakeRpc({
      sessions: [createSummary("session-1", "Orchestrator", "main reply")],
      surfaces: [
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-1"),
          messages: [assistantMessage("main reply")],
        }),
      ],
    });
    harness.setWorkspaceUiRestore(
      TEST_WORKSPACE_INFO.workspaceId,
      createWorkspaceRestoreState({
        dockview: null,
        compactSurfaces: [],
        panels: [
          {
            panelId: "primary",
            binding: null,
            localState: {
              scroll: null,
              timelineDensity: "comfortable",
            },
          },
          {
            panelId: "secondary",
            binding: createOrchestratorTarget("session-1"),
            localState: {
              scroll: null,
              timelineDensity: "comfortable",
            },
          },
        ],
        focusedPanelId: "primary",
        updatedAt: "2026-04-27T00:00:00.000Z",
      }),
    );

    const runtime = await createRuntime(harness, storage);

    expect(runtime.paneLayout.panels).toHaveLength(1);
    expect(runtime.paneLayout.focusedPanelId).toBe("secondary");
    expect(runtime.getPane("primary")).toBeUndefined();
    expect(runtime.getPane("secondary")?.target).toEqual(createOrchestratorTarget("session-1"));

    runtime.dispose();
  });

  it("switches between fixed A/B/C layout slots and keeps empty slots selectable", async () => {
    const storage = createMemoryStorage();
    const harness = createFakeRpc({
      sessions: [createSummary("session-1", "Orchestrator", "main reply")],
      surfaces: [
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-1"),
          messages: [assistantMessage("main reply")],
        }),
      ],
    });

    const runtime = await createRuntime(harness, storage);

    expect(runtime.activeLayoutId).toBe("A");
    expect(runtime.layoutSlots).toEqual([
      expect.objectContaining({ id: "A", active: true, initialized: true }),
      expect.objectContaining({ id: "B", active: false, initialized: false }),
      expect.objectContaining({ id: "C", active: false, initialized: false }),
    ]);

    await runtime.switchWorkspaceLayout("B");
    expect(runtime.activeLayoutId).toBe("B");
    expect(runtime.paneLayout.panels).toHaveLength(0);
    expect(runtime.getPane("primary")).toBeUndefined();
    expect(runtime.layoutSlots).toEqual([
      expect.objectContaining({ id: "A", active: false, initialized: true }),
      expect.objectContaining({ id: "B", active: true, initialized: false }),
      expect.objectContaining({ id: "C", active: false, initialized: false }),
    ]);

    await runtime.openSession("session-1", runtime.primaryPaneId);
    expect(runtime.getPane("primary")?.target).toEqual(createOrchestratorTarget("session-1"));
    expect(runtime.layoutSlots.find((slot) => slot.id === "B")?.initialized).toBe(true);

    await runtime.switchWorkspaceLayout("A");
    expect(runtime.activeLayoutId).toBe("A");
    expect(runtime.getPane("primary")?.target).toEqual(createOrchestratorTarget("session-1"));

    const restoreState = harness.getWorkspaceUiRestore(TEST_WORKSPACE_INFO.workspaceId);
    expect(restoreState?.layouts.A?.panels[0]?.binding).toEqual(
      createOrchestratorTarget("session-1"),
    );
    expect(restoreState?.layouts.B?.panels[0]?.binding).toEqual(
      createOrchestratorTarget("session-1"),
    );
    expect(restoreState?.layouts.C).toBeNull();

    runtime.dispose();
  });

  it("uses the tab-selected active layout against durable workspace layout slots", async () => {
    const storage = createMemoryStorage();
    const harness = createFakeRpc({
      sessions: [createSummary("session-1", "Orchestrator", "main reply")],
      surfaces: [
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-1"),
          messages: [assistantMessage("main reply")],
        }),
      ],
    });
    harness.setWorkspaceUiRestore(
      TEST_WORKSPACE_INFO.workspaceId,
      createWorkspaceRestoreState(
        {
          dockview: null,
          compactSurfaces: [],
          panels: [
            {
              panelId: "secondary",
              binding: createOrchestratorTarget("session-1"),
              localState: {
                scroll: null,
                timelineDensity: "comfortable",
              },
            },
          ],
          focusedPanelId: "secondary",
          updatedAt: "2026-04-27T00:00:00.000Z",
        },
        "B",
      ),
    );

    const runtime = await createRuntime(harness, storage, {
      ...TEST_WORKSPACE_INFO,
      activeLayoutId: "B",
    });

    expect(runtime.activeLayoutId).toBe("B");
    expect(runtime.getPane("secondary")?.target).toEqual(createOrchestratorTarget("session-1"));

    runtime.dispose();
  });

  it("hydrates prompt pane controllers when switching to a saved inactive layout slot", async () => {
    const storage = createMemoryStorage();
    const harness = createFakeRpc({
      sessions: [
        createSummary("session-1", "Orchestrator", "main reply"),
        createSummary("session-2", "Second", "second reply"),
      ],
      surfaces: [
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-1"),
          messages: [assistantMessage("main reply")],
        }),
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-2"),
          messages: [assistantMessage("second reply")],
        }),
      ],
    });
    harness.setWorkspaceUiRestore(TEST_WORKSPACE_INFO.workspaceId, {
      version: 5,
      layouts: {
        A: {
          dockview: null,
          compactSurfaces: [],
          panels: [
            {
              panelId: "slot-a",
              binding: createOrchestratorTarget("session-1"),
              localState: {
                scroll: null,
                timelineDensity: "comfortable",
              },
            },
          ],
          focusedPanelId: "slot-a",
          updatedAt: "2026-04-27T00:00:00.000Z",
        },
        B: {
          dockview: null,
          compactSurfaces: [],
          panels: [
            {
              panelId: "slot-b",
              binding: createOrchestratorTarget("session-2"),
              localState: {
                scroll: null,
                timelineDensity: "comfortable",
              },
            },
          ],
          focusedPanelId: "slot-b",
          updatedAt: "2026-04-27T00:00:00.000Z",
        },
        C: null,
      },
    });

    const runtime = await createRuntime(harness, storage);

    expect(runtime.activeLayoutId).toBe("A");
    expect(runtime.getPaneController("slot-a")).toBeTruthy();
    expect(runtime.getPaneController("slot-b")).toBeNull();

    await runtime.switchWorkspaceLayout("B");

    expect(runtime.activeLayoutId).toBe("B");
    expect(runtime.getPane("slot-b")?.target).toEqual(createOrchestratorTarget("session-2"));
    expect(runtime.getPaneController("slot-b")).toBeTruthy();
    expect(harness.openedTargets).toContainEqual(createOrchestratorTarget("session-2"));

    runtime.dispose();
  });

  it("syncs shared workspace layout slot changes into another open tab on the same slot", async () => {
    const storage = createMemoryStorage();
    const harness = createFakeRpc({
      sessions: [createSummary("session-1", "Orchestrator", "main reply")],
      surfaces: [
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-1"),
          messages: [assistantMessage("main reply")],
        }),
      ],
    });
    const { createChatRuntime } = await import("./chat-runtime");
    let secondRuntime: Awaited<ReturnType<typeof createChatRuntime>> | null = null;
    let syncPromise: Promise<void> = Promise.resolve();
    const firstRuntime = await createChatRuntime(
      {
        workspaceInfo: TEST_WORKSPACE_INFO,
        onWorkspaceLayoutPersist: (state) => {
          syncPromise = secondRuntime?.syncWorkspaceLayoutState(state) ?? Promise.resolve();
        },
      },
      harness.client as never,
      storage,
    );
    secondRuntime = await createChatRuntime(
      { workspaceInfo: TEST_WORKSPACE_INFO },
      harness.client as never,
      storage,
    );

    await firstRuntime.openSurface({ surface: "app-logs" }, "primary");
    await syncPromise;

    expect(secondRuntime.getPane("primary")?.target).toEqual({ surface: "app-logs" });

    firstRuntime.dispose();
    secondRuntime.dispose();
  });

  it("preserves a saved empty layout without reopening the last session", async () => {
    const storage = createMemoryStorage();
    const harness = createFakeRpc({
      sessions: [createSummary("session-1", "Orchestrator", "main reply")],
      surfaces: [
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-1"),
          messages: [assistantMessage("main reply")],
        }),
      ],
    });
    harness.setWorkspaceUiRestore(
      TEST_WORKSPACE_INFO.workspaceId,
      createWorkspaceRestoreState({
        dockview: null,
        compactSurfaces: [],
        panels: [],
        focusedPanelId: null,
        updatedAt: "2026-04-27T00:00:00.000Z",
      }),
    );

    const runtime = await createRuntime(harness, storage);

    expect(runtime.sessions).toHaveLength(1);
    expect(runtime.paneLayout.panels).toHaveLength(0);
    expect(runtime.paneLayout.focusedPanelId).toBeNull();

    runtime.dispose();
  });

  it("starts an uninitialized user workspace layout empty instead of reopening the first session", async () => {
    const storage = createMemoryStorage();
    const harness = createFakeRpc({
      sessions: [createSummary("session-1", "Orchestrator", "main reply")],
      surfaces: [
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-1"),
          messages: [assistantMessage("main reply")],
        }),
      ],
    });

    const runtime = await createRuntime(harness, storage, TEST_WORKSPACE_INFO, {
      seedInitialLayout: false,
    });

    expect(runtime.sessions).toHaveLength(1);
    expect(runtime.paneLayout.panels).toHaveLength(0);
    expect(runtime.paneLayout.focusedPanelId).toBeNull();

    runtime.dispose();
  });

  it("starts default workspace tabs with only the Open Workspace pane", async () => {
    const storage = createMemoryStorage();
    const defaultWorkspaceInfo: WorkspaceTabInfo = {
      ...TEST_WORKSPACE_INFO,
      workspaceTabId: "default-tab-1",
      workspaceId: "workspace:default",
      cwd: "/tmp/svvy/default-workspace",
      workspaceLabel: "Default Workspace",
      kind: "default",
      branch: undefined,
    };
    const harness = createFakeRpc({ sessions: [], surfaces: [] });
    harness.setWorkspaceUiRestore(
      defaultWorkspaceInfo.workspaceId,
      createWorkspaceRestoreState({
        dockview: null,
        compactSurfaces: [],
        panels: [
          {
            panelId: "primary",
            binding: { surface: "app-logs" },
            localState: {
              scroll: null,
              timelineDensity: "comfortable",
            },
          },
        ],
        focusedPanelId: "primary",
        updatedAt: "2026-04-27T00:00:00.000Z",
      }),
    );

    const runtime = await createRuntime(harness, storage, defaultWorkspaceInfo);

    expect(runtime.sessions).toHaveLength(0);
    expect(runtime.getPane("primary")?.target).toEqual({ surface: "open-workspace" });
    expect(runtime.layoutSlotsEnabled).toBe(false);
    await runtime.switchWorkspaceLayout("B");
    expect(runtime.activeLayoutId).toBe("A");
    expect(runtime.getPane("primary")?.target).toEqual({ surface: "open-workspace" });

    runtime.dispose();
  });

  it("replaces the default Open Workspace pane when creating a session", async () => {
    const storage = createMemoryStorage();
    const defaultWorkspaceInfo: WorkspaceTabInfo = {
      ...TEST_WORKSPACE_INFO,
      workspaceTabId: "default-tab-1",
      workspaceId: "workspace:default",
      cwd: "/tmp/svvy/default-workspace",
      workspaceLabel: "Default Workspace",
      kind: "default",
      branch: undefined,
    };
    const harness = createFakeRpc({ sessions: [], surfaces: [] });
    harness.setWorkspaceUiRestore(
      defaultWorkspaceInfo.workspaceId,
      createWorkspaceRestoreState({
        dockview: null,
        compactSurfaces: [],
        panels: [
          {
            panelId: "primary",
            binding: { surface: "open-workspace" },
            localState: {
              scroll: null,
              timelineDensity: "comfortable",
            },
          },
        ],
        focusedPanelId: "primary",
        updatedAt: "2026-04-27T00:00:00.000Z",
      }),
    );

    const runtime = await createRuntime(harness, storage, defaultWorkspaceInfo);

    await runtime.createSession();

    expect(runtime.sessions).toHaveLength(1);
    expect(runtime.getPane("primary")?.target).toEqual(createOrchestratorTarget("session-1"));

    runtime.dispose();
  });

  it("replaces the default Open Workspace pane when sidebar session creation requests a new panel", async () => {
    const storage = createMemoryStorage();
    const defaultWorkspaceInfo: WorkspaceTabInfo = {
      ...TEST_WORKSPACE_INFO,
      workspaceTabId: "default-tab-1",
      workspaceId: "workspace:default",
      cwd: "/tmp/svvy/default-workspace",
      workspaceLabel: "Default Workspace",
      kind: "default",
      branch: undefined,
    };
    const harness = createFakeRpc({ sessions: [], surfaces: [] });
    harness.setWorkspaceUiRestore(
      defaultWorkspaceInfo.workspaceId,
      createWorkspaceRestoreState({
        dockview: null,
        compactSurfaces: [],
        panels: [
          {
            panelId: "primary",
            binding: { surface: "open-workspace" },
            localState: {
              scroll: null,
              timelineDensity: "comfortable",
            },
          },
        ],
        focusedPanelId: "primary",
        updatedAt: "2026-04-27T00:00:00.000Z",
      }),
    );

    const runtime = await createRuntime(harness, storage, defaultWorkspaceInfo);

    await runtime.createSession({}, { kind: "new-panel", direction: "right" });

    expect(runtime.paneLayout.panels).toHaveLength(1);
    expect(runtime.getPane("primary")?.target).toEqual(createOrchestratorTarget("session-1"));

    runtime.dispose();
  });

  it("removes restored prompt panes that fail to reopen instead of leaving unavailable surfaces", async () => {
    const storage = createMemoryStorage();
    const harness = createFakeRpc({
      sessions: [createSummary("missing-session", "Missing", "")],
      surfaces: [],
    });
    harness.setWorkspaceUiRestore(
      TEST_WORKSPACE_INFO.workspaceId,
      createWorkspaceRestoreState({
        dockview: null,
        compactSurfaces: [],
        panels: [
          {
            panelId: "primary",
            binding: createOrchestratorTarget("missing-session"),
            localState: {
              scroll: null,
              timelineDensity: "comfortable",
            },
          },
        ],
        focusedPanelId: "primary",
        updatedAt: "2026-04-27T00:00:00.000Z",
      }),
    );

    const runtime = await createRuntime(harness, storage);

    expect(runtime.sessions).toHaveLength(1);
    expect(runtime.paneLayout.panels).toHaveLength(0);
    expect(runtime.getPane("primary")).toBeUndefined();

    runtime.dispose();
  });

  it("changes an empty focused session mode without creating a new session or pane", async () => {
    const harness = createFakeRpc({
      sessions: [createSummary("session-1", "New Session", "")],
      surfaces: [
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-1"),
          messages: [],
        }),
      ],
    });

    const runtime = await createRuntime(harness);
    const initialSessionCount = runtime.sessions.length;
    const initialPaneCount = runtime.paneLayout.panels.length;

    await runtime.setSessionMode(runtime.primaryPaneId, "dumb");

    const focusedPane = runtime.getPane(runtime.primaryPaneId);
    const controller = runtime.getPaneController(runtime.primaryPaneId);
    expect(runtime.sessions).toHaveLength(initialSessionCount);
    expect(runtime.paneLayout.panels).toHaveLength(initialPaneCount);
    expect(
      focusedPane?.target && "workspaceSessionId" in focusedPane.target
        ? focusedPane.target.workspaceSessionId
        : undefined,
    ).toBe("session-1");
    expect(controller?.sessionMode).toBe("dumb");
    expect(controller?.sessionAgentKey).toBe("dumbOrchestrator");
    expect(harness.getSurfaceSnapshot("session-1").sessionMode).toBe("dumb");

    runtime.dispose();
  });

  it("tracks app log summaries, live updates, static logs panes, and mark-seen requests", async () => {
    const harness = createFakeRpc({
      sessions: [createSummary("session-1", "Orchestrator", "main reply")],
      surfaces: [
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-1"),
          messages: [assistantMessage("main reply")],
        }),
      ],
    });
    const runtime = await createRuntime(harness);

    const entry: AppLogEntry = {
      id: "app-log-1",
      seq: 1,
      createdAt: "2026-05-13T10:00:00.000Z",
      level: "error",
      source: "prompt",
      message: "Prompt failed",
      workspaceSessionId: "session-1",
      surfacePiSessionId: "session-1",
    };
    harness.emitAppLogUpdate({
      workspaceId: TEST_WORKSPACE_INFO.workspaceId,
      entries: [entry],
      summary: {
        latestSeq: 1,
        seenSeq: 0,
        unread: { total: 1, info: 0, warning: 0, error: 1 },
        totals: { total: 1, info: 0, warning: 0, error: 1 },
      },
    });

    expect(runtime.appLogSummary.unread.error).toBe(1);
    await runtime.openSurface({ surface: "app-logs" }, "primary");
    expect(runtime.getPane("primary")?.target).toEqual({ surface: "app-logs" });
    expect(await runtime.getAppLogs()).toMatchObject({ entries: [entry] });

    await runtime.markAppLogsSeen(1);
    expect(harness.appLogSeenRequests).toEqual([1]);
    expect(runtime.appLogSummary.unread.total).toBe(0);

    runtime.dispose();
  });

  it("ignores app log updates for other workspace runtimes", async () => {
    const harness = createFakeRpc({
      sessions: [createSummary("session-1", "Orchestrator", "main reply")],
      surfaces: [
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-1"),
          messages: [assistantMessage("main reply")],
        }),
      ],
    });
    const runtime = await createRuntime(harness);

    harness.emitAppLogUpdate({
      workspaceId: "workspace:other",
      entries: [
        {
          id: "other-log-1",
          seq: 1,
          createdAt: "2026-05-13T10:00:00.000Z",
          level: "error",
          source: "prompt",
          message: "Other workspace failed",
        },
      ],
      summary: {
        latestSeq: 1,
        seenSeq: 0,
        unread: { total: 1, info: 0, warning: 0, error: 1 },
        totals: { total: 1, info: 0, warning: 0, error: 1 },
      },
    });

    expect(runtime.appLogSummary.unread.total).toBe(0);

    runtime.dispose();
  });

  it("opens the context library as a static renderer pane without backend surface activation", async () => {
    const harness = createFakeRpc({
      sessions: [createSummary("session-1", "Prompt Work", "done")],
      surfaces: [
        createSurfaceSnapshot({
          target: createOrchestratorTarget("session-1"),
          messages: [assistantMessage("done")],
        }),
      ],
    });
    const runtime = await createRuntime(harness);
    const openedBefore = harness.openedTargets.length;

    await runtime.openSurface({ surface: "prompt-library" }, "primary");

    expect(runtime.getPane("primary")?.target).toEqual({ surface: "prompt-library" });
    expect(harness.openedTargets).toHaveLength(openedBefore);

    runtime.dispose();
  });
});
