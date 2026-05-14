import { Agent, type AgentMessage, type StreamFn } from "@mariozechner/pi-agent-core";
import {
  createAssistantMessageEventStream,
  getModel,
  type AssistantMessage,
  type AssistantMessageEvent,
  type Message,
} from "@mariozechner/pi-ai";
import type {
  AppLogQuery,
  AppLogReadModel,
  AppLogSummary,
  AppLogUpdateMessage,
  ConversationSurfaceSnapshot,
  CreateSessionRequest,
  PromptTarget,
  SendPromptRequest,
  SurfaceSyncMessage,
  WorkspaceCommandInspector,
  WorkspacePathIndexEntry,
  WorkspaceHandlerThreadInspector,
  WorkspaceHandlerThreadSummary,
  WorkspaceArtifactPreview,
  WorkspaceProjectCiStatusPanel,
  WorkspaceSavedWorkflowLibraryReadModel,
  WorkspaceSessionNavigationReadModel,
  WorkspaceSessionSummary,
  WorkspaceSyncMessage,
  WorkspaceWorkflowTaskAttemptInspector,
  WorkspaceWorkflowInspectorMode,
  WorkspaceWorkflowInspectorLiveUpdate,
  WorkspaceWorkflowInspectorReadModel,
  WorkspacePaneSurfaceTarget,
} from "../shared/workspace-contract";
import {
  createChatStorage,
  type ChatStorage,
  type WorkspaceInspectorSelection,
  type WorkspaceUiRestoreState,
} from "./chat-storage";
import { DEFAULT_AGENT_SETTINGS, type ReasoningEffort } from "../shared/agent-settings";
import type { SessionAgentKey, SessionMode } from "../shared/agent-settings";
import type { AppMenuAction } from "../shared/shortcut-registry";
import {
  addDockviewPanel,
  bindPane,
  closePane,
  createEmptyPaneLayout,
  createPanelId,
  focusPane,
  normalizePaneLayout,
  PRIMARY_CHAT_PANE_ID,
  setDockviewSerializedLayout,
  setPaneInspectorSelection as setLayoutPaneInspectorSelection,
  setPaneScroll as setLayoutPaneScroll,
  splitPane,
  type PaneOpenTarget,
  type DockviewSplitDirection,
  type WorkspaceDockviewLayoutState,
} from "./pane-layout";
import { rpc } from "./rpc";
import { buildWorkspaceSessionNavigation } from "./session-state";

export { PRIMARY_CHAT_PANE_ID } from "./pane-layout";

type UsageStats = {
  input: number;
  output: number;
  cacheRead: number;
  cacheWrite: number;
  totalTokens: number;
  cost: {
    input: number;
    output: number;
    cacheRead: number;
    cacheWrite: number;
    total: number;
  };
};

const ZERO_USAGE: UsageStats = {
  input: 0,
  output: 0,
  cacheRead: 0,
  cacheWrite: 0,
  totalTokens: 0,
  cost: {
    input: 0,
    output: 0,
    cacheRead: 0,
    cacheWrite: 0,
    total: 0,
  },
};

type ChatRuntimeListener = () => void;
type PromptStatus = ConversationSurfaceSnapshot["promptStatus"];

export interface ChatPaneState {
  id: string;
  target: WorkspacePaneSurfaceTarget | null;
  inspectorSelection: WorkspaceInspectorSelection | null;
  scroll: ChatPaneLayoutState["panels"][number]["localState"]["scroll"];
  timelineDensity: "compact" | "comfortable";
}

export type ChatPaneLayoutState = WorkspaceDockviewLayoutState;

export interface ChatSurfaceController {
  agent: Agent;
  target: PromptTarget;
  resolvedSystemPrompt: string;
  sessionMode: SessionMode;
  sessionAgentKey: SessionAgentKey;
  promptStatus: PromptStatus;
  ownerPaneIds: string[];
  abort: () => Promise<void>;
  subscribe: (listener: ChatRuntimeListener) => () => void;
}

interface ChatSurfaceControllerInternal extends ChatSurfaceController {
  attachPane: (panelId: string) => void;
  detachPane: (panelId: string) => void;
  applySnapshot: (snapshot: ConversationSurfaceSnapshot) => void;
  dispose: () => void;
}

export interface ChatRuntimeRpcClient {
  request: {
    getDefaults: typeof rpc.request.getDefaults;
    getAgentSettings: typeof rpc.request.getAgentSettings;
    updateSessionAgentDefault: typeof rpc.request.updateSessionAgentDefault;
    updateWorkflowAgent: typeof rpc.request.updateWorkflowAgent;
    updateAppPreferences: typeof rpc.request.updateAppPreferences;
    ensureWorkflowAgentsComponent: typeof rpc.request.ensureWorkflowAgentsComponent;
    getProviderAuthState: typeof rpc.request.getProviderAuthState;
    getWorkspaceInfo: typeof rpc.request.getWorkspaceInfo;
    getAppLogs: typeof rpc.request.getAppLogs;
    getAppLogSummary: typeof rpc.request.getAppLogSummary;
    markAppLogsSeen: typeof rpc.request.markAppLogsSeen;
    writeClipboardText: typeof rpc.request.writeClipboardText;
    listWorkspacePaths: typeof rpc.request.listWorkspacePaths;
    pickWorkspaceAttachments: typeof rpc.request.pickWorkspaceAttachments;
    openWorkspacePath: typeof rpc.request.openWorkspacePath;
    getSavedWorkflowLibrary: typeof rpc.request.getSavedWorkflowLibrary;
    deleteSavedWorkflowLibraryItem: typeof rpc.request.deleteSavedWorkflowLibraryItem;
    openWorkflowSourceInEditor: typeof rpc.request.openWorkflowSourceInEditor;
    listSessions: typeof rpc.request.listSessions;
    getCommandInspector: typeof rpc.request.getCommandInspector;
    listHandlerThreads: typeof rpc.request.listHandlerThreads;
    getHandlerThreadInspector: typeof rpc.request.getHandlerThreadInspector;
    getWorkflowTaskAttemptInspector: typeof rpc.request.getWorkflowTaskAttemptInspector;
    getWorkflowInspector: typeof rpc.request.getWorkflowInspector;
    streamWorkflowInspector: typeof rpc.request.streamWorkflowInspector;
    getProjectCiStatus: typeof rpc.request.getProjectCiStatus;
    getArtifactPreview: typeof rpc.request.getArtifactPreview;
    createSession: typeof rpc.request.createSession;
    openSession: typeof rpc.request.openSession;
    recordSessionOpened: typeof rpc.request.recordSessionOpened;
    openSurface: typeof rpc.request.openSurface;
    closeSurface: typeof rpc.request.closeSurface;
    renameSession: typeof rpc.request.renameSession;
    setSessionMode: typeof rpc.request.setSessionMode;
    forkSession: typeof rpc.request.forkSession;
    deleteSession: typeof rpc.request.deleteSession;
    pinSession: typeof rpc.request.pinSession;
    unpinSession: typeof rpc.request.unpinSession;
    archiveSession: typeof rpc.request.archiveSession;
    unarchiveSession: typeof rpc.request.unarchiveSession;
    setArchivedGroupCollapsed: typeof rpc.request.setArchivedGroupCollapsed;
    sendPrompt: typeof rpc.request.sendPrompt;
    setSurfaceModel: typeof rpc.request.setSurfaceModel;
    setSurfaceThoughtLevel: typeof rpc.request.setSurfaceThoughtLevel;
    cancelPrompt: typeof rpc.request.cancelPrompt;
    listProviderAuths: typeof rpc.request.listProviderAuths;
    setProviderApiKey: typeof rpc.request.setProviderApiKey;
    startOAuth: typeof rpc.request.startOAuth;
    removeProviderAuth: typeof rpc.request.removeProviderAuth;
  };
  addMessageListener: typeof rpc.addMessageListener;
  removeMessageListener: typeof rpc.removeMessageListener;
}

const DEFAULT_RPC_CLIENT: ChatRuntimeRpcClient = rpc;

export interface ChatRuntimeOptions {
  onMissingProviderAccess?: (provider: string) => void;
}

export interface ChatRuntime {
  storage: ChatStorage;
  workspaceId: string;
  workspaceLabel: string;
  branch?: string;
  appLogSummary: AppLogSummary;
  sessions: WorkspaceSessionSummary[];
  sessionNavigation: WorkspaceSessionNavigationReadModel;
  paneLayout: ChatPaneLayoutState;
  primaryPaneId: string;
  dispose: () => void;
  subscribe: (listener: ChatRuntimeListener) => () => void;
  subscribeAppLogUpdate: (listener: (payload: AppLogUpdateMessage) => void) => () => void;
  subscribeAppMenuAction: (listener: (action: AppMenuAction) => void) => () => void;
  listSessions: () => Promise<WorkspaceSessionSummary[]>;
  getPane: (panelId: string) => ChatPaneState | undefined;
  getPaneController: (panelId: string) => ChatSurfaceController | null;
  getSurfaceController: (surfacePiSessionId: string) => ChatSurfaceController | null;
  focusPane: (panelId: string) => void;
  splitPane: (
    panelId: string,
    direction: DockviewSplitDirection,
    options?: { duplicateBinding?: boolean; size?: number },
  ) => Promise<string | null>;
  closePane: (panelId: string) => Promise<void>;
  setDockviewLayout: (
    dockview: WorkspaceDockviewLayoutState["dockview"],
    focusedPanelId?: string | null,
  ) => void;
  getCommandInspector: (
    commandId: string,
    sessionId?: string,
  ) => Promise<WorkspaceCommandInspector>;
  listHandlerThreads: (sessionId?: string) => Promise<WorkspaceHandlerThreadSummary[]>;
  getHandlerThreadInspector: (
    threadId: string,
    sessionId?: string,
  ) => Promise<WorkspaceHandlerThreadInspector>;
  getWorkflowTaskAttemptInspector: (
    workflowTaskAttemptId: string,
    sessionId?: string,
  ) => Promise<WorkspaceWorkflowTaskAttemptInspector>;
  getWorkflowInspector: (
    workflowRunId: string,
    options?: {
      sessionId?: string;
      selectedNodeKey?: string | null;
      expandedNodeKeys?: string[];
      userCollapsedNodeKeys?: string[];
      searchQuery?: string;
      mode?: WorkspaceWorkflowInspectorMode;
    },
  ) => Promise<WorkspaceWorkflowInspectorReadModel>;
  streamWorkflowInspector: (
    workflowRunId: string,
    options?: {
      sessionId?: string;
      selectedNodeKey?: string | null;
      expandedNodeKeys?: string[];
      userCollapsedNodeKeys?: string[];
      searchQuery?: string;
      mode?: WorkspaceWorkflowInspectorMode;
      fromSeq?: number | null;
    },
  ) => Promise<WorkspaceWorkflowInspectorLiveUpdate>;
  getProjectCiStatus: (sessionId?: string) => Promise<WorkspaceProjectCiStatusPanel>;
  getArtifactPreview: (artifactId: string, sessionId?: string) => Promise<WorkspaceArtifactPreview>;
  getAppLogs: (query?: AppLogQuery) => Promise<AppLogReadModel>;
  getAppLogSummary: () => Promise<AppLogSummary>;
  markAppLogsSeen: (throughSeq: number) => Promise<AppLogSummary>;
  writeClipboardText: (text: string) => Promise<void>;
  createSession: (
    request?: CreateSessionRequest,
    openTarget?: PaneOpenTarget | string,
  ) => Promise<void>;
  openSession: (sessionId: string, openTarget?: PaneOpenTarget | string) => Promise<void>;
  openSurface: (
    target: WorkspacePaneSurfaceTarget,
    openTarget?: PaneOpenTarget | string,
  ) => Promise<void>;
  closePaneSurface: (panelId: string) => Promise<void>;
  renameSession: (sessionId: string, title: string) => Promise<void>;
  setSessionMode: (panelId: string, mode: SessionMode) => Promise<void>;
  forkSession: (
    sessionId: string,
    title?: string,
    openTarget?: PaneOpenTarget | string,
    options?: { messageTimestamp?: string | number },
  ) => Promise<void>;
  deleteSession: (sessionId: string, panelId?: string) => Promise<void>;
  pinSession: (sessionId: string) => Promise<void>;
  unpinSession: (sessionId: string) => Promise<void>;
  archiveSession: (sessionId: string) => Promise<void>;
  unarchiveSession: (sessionId: string) => Promise<void>;
  setArchivedGroupCollapsed: (collapsed: boolean) => Promise<void>;
  setPaneInspectorSelection: (
    panelId: string,
    selection: WorkspaceInspectorSelection | null,
  ) => void;
  setPaneScroll: (
    panelId: string,
    scroll: ChatPaneLayoutState["panels"][number]["localState"]["scroll"],
  ) => void;
  sendPromptToTarget: (target: PromptTarget, input: string) => Promise<void>;
  syncProviderAuth: (providerId: string) => Promise<boolean>;
  requireProviderAccess: (providerId: string) => Promise<boolean>;
  listConfiguredProviders: () => Promise<string[]>;
  listWorkspacePaths: (options?: { refresh?: boolean }) => Promise<WorkspacePathIndexEntry[]>;
  pickWorkspaceAttachments: () => Promise<WorkspacePathIndexEntry[]>;
  openWorkspacePath: (workspaceRelativePath: string) => Promise<boolean>;
  getSavedWorkflowLibrary: () => Promise<WorkspaceSavedWorkflowLibraryReadModel>;
  deleteSavedWorkflowLibraryItem: (path: string) => Promise<WorkspaceSavedWorkflowLibraryReadModel>;
  openWorkflowSourceInEditor: (path: string) => Promise<boolean>;
}

function createRpcStreamId(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  return `${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
}

function createFailureMessage(
  error: unknown,
  provider: string,
  model: string,
  stopReason: "aborted" | "error" = "error",
): AssistantMessage {
  const message = error instanceof Error ? error.message : "Unable to generate a response.";
  return {
    role: "assistant",
    content: [{ type: "text", text: message }],
    api: `${provider}-responses`,
    provider,
    model,
    timestamp: Date.now(),
    usage: ZERO_USAGE,
    stopReason,
    errorMessage: message,
  };
}

function initializeStorage(): ChatStorage {
  return createChatStorage();
}

function normalizePromptTarget(target: PromptTarget): PromptTarget {
  return {
    workspaceSessionId: target.workspaceSessionId,
    surface: target.surface,
    surfacePiSessionId: target.surfacePiSessionId,
    ...(target.threadId ? { threadId: target.threadId } : {}),
  };
}

function isPromptTarget(target: WorkspacePaneSurfaceTarget | null): target is PromptTarget {
  return target?.surface === "orchestrator" || target?.surface === "thread";
}

function convertToLlm(messages: AgentMessage[]): Message[] {
  return messages.filter((message): message is Message => {
    return message.role === "user" || message.role === "assistant" || message.role === "toolResult";
  });
}

function applySurfaceSnapshotToAgent(agent: Agent, payload: ConversationSurfaceSnapshot): void {
  const currentTools = [...agent.state.tools];
  agent.reset();
  agent.sessionId = payload.target.surfacePiSessionId;
  agent.setSystemPrompt(payload.systemPrompt);
  agent.setModel(
    getModel(
      payload.provider as Parameters<typeof getModel>[0],
      payload.model as Parameters<typeof getModel>[1],
    ),
  );
  agent.setThinkingLevel(payload.reasoningEffort);
  agent.replaceMessages(payload.messages);
  agent.setTools(currentTools);
}

function createInitialAgent(snapshot: ConversationSurfaceSnapshot, streamFn: StreamFn): Agent {
  const agent = new Agent({
    initialState: {
      systemPrompt: snapshot.systemPrompt,
      model: getModel(
        snapshot.provider as Parameters<typeof getModel>[0],
        snapshot.model as Parameters<typeof getModel>[1],
      ),
      thinkingLevel: snapshot.reasoningEffort,
      messages: structuredClone(snapshot.messages),
      tools: [],
    },
    convertToLlm,
    streamFn,
  });
  agent.sessionId = snapshot.target.surfacePiSessionId;
  return agent;
}

class SurfaceControllerImpl implements ChatSurfaceControllerInternal {
  agent: Agent;
  target: PromptTarget;
  resolvedSystemPrompt: string;
  sessionMode: SessionMode;
  sessionAgentKey: SessionAgentKey;
  promptStatus: PromptStatus;

  private listeners = new Set<ChatRuntimeListener>();
  private panelIds = new Set<string>();
  private disposed = false;
  private promptDispatchInFlight = false;
  private applyingSnapshot = false;
  private suppressSurfaceMutationSync = false;
  private pendingSnapshot: ConversationSurfaceSnapshot | null = null;

  constructor(
    snapshot: ConversationSurfaceSnapshot,
    private readonly rpcClient: ChatRuntimeRpcClient,
  ) {
    this.target = normalizePromptTarget(snapshot.target);
    this.resolvedSystemPrompt = snapshot.resolvedSystemPrompt;
    this.sessionMode = snapshot.sessionMode;
    this.sessionAgentKey = snapshot.sessionAgentKey;
    this.promptStatus = snapshot.promptStatus;
    this.agent = createInitialAgent(snapshot, this.createStreamFn());

    const originalSetModel = this.agent.setModel.bind(this.agent);
    this.agent.setModel = (nextModel) => {
      originalSetModel(nextModel);
      if (!this.suppressSurfaceMutationSync) {
        void this.syncSurfaceModel(nextModel.provider, nextModel.id);
      }
    };

    const originalSetThinkingLevel = this.agent.setThinkingLevel.bind(this.agent);
    this.agent.setThinkingLevel = (level) => {
      originalSetThinkingLevel(level);
      if (!this.suppressSurfaceMutationSync) {
        void this.syncSurfaceThoughtLevel(level);
      }
    };

    this.agent.subscribe(() => {
      if (this.disposed || this.applyingSnapshot) {
        return;
      }

      if (!this.promptDispatchInFlight) {
        this.promptStatus = this.agent.state.isStreaming ? "streaming" : "idle";
      }

      if (!this.agent.state.isStreaming && this.pendingSnapshot) {
        const pendingSnapshot = this.pendingSnapshot;
        this.pendingSnapshot = null;
        this.applySnapshot(pendingSnapshot);
        return;
      }

      this.emit();
    });
  }

  get ownerPaneIds(): string[] {
    return Array.from(this.panelIds);
  }

  subscribe(listener: ChatRuntimeListener): () => void {
    this.listeners.add(listener);
    listener();
    return () => {
      this.listeners.delete(listener);
    };
  }

  attachPane(panelId: string): void {
    this.panelIds.add(panelId);
    this.emit();
  }

  detachPane(panelId: string): void {
    this.panelIds.delete(panelId);
    this.emit();
  }

  applySnapshot(snapshot: ConversationSurfaceSnapshot): void {
    if (this.disposed) {
      return;
    }

    if (this.promptDispatchInFlight) {
      this.pendingSnapshot = structuredClone(snapshot);
      this.resolvedSystemPrompt = snapshot.resolvedSystemPrompt;
      this.target = normalizePromptTarget(snapshot.target);
      this.sessionMode = snapshot.sessionMode;
      this.sessionAgentKey = snapshot.sessionAgentKey;
      this.promptStatus = snapshot.promptStatus;
      this.emit();
      return;
    }

    this.pendingSnapshot = null;
    this.target = normalizePromptTarget(snapshot.target);
    this.resolvedSystemPrompt = snapshot.resolvedSystemPrompt;
    this.sessionMode = snapshot.sessionMode;
    this.sessionAgentKey = snapshot.sessionAgentKey;
    this.promptStatus = snapshot.promptStatus;

    this.suppressSurfaceMutationSync = true;
    this.applyingSnapshot = true;
    try {
      applySurfaceSnapshotToAgent(this.agent, snapshot);
    } finally {
      this.applyingSnapshot = false;
      this.suppressSurfaceMutationSync = false;
    }
    this.emit();
  }

  async abort(): Promise<void> {
    try {
      await this.rpcClient.request.cancelPrompt({ target: this.target });
    } catch (error) {
      console.error("Failed to cancel prompt:", error);
    } finally {
      this.agent.abort();
    }
  }

  dispose(): void {
    this.disposed = true;
    this.listeners.clear();
  }

  private emit(): void {
    if (this.disposed) {
      return;
    }

    for (const listener of this.listeners) {
      listener();
    }
  }

  private createStreamFn(): StreamFn {
    return async (model, context, streamOptions) => {
      const stream = createAssistantMessageEventStream();
      const reasoningEffort =
        (streamOptions?.reasoning as ReasoningEffort | undefined) ??
        DEFAULT_AGENT_SETTINGS.reasoningEffort;
      const request: SendPromptRequest = {
        streamId: createRpcStreamId(),
        messages: context.messages as Message[],
        provider: model.provider,
        model: model.id,
        reasoningEffort,
        target: this.target,
        systemPrompt: context.systemPrompt,
      };
      const provider = request.provider ?? DEFAULT_AGENT_SETTINGS.provider;
      const modelId = request.model ?? DEFAULT_AGENT_SETTINGS.model;
      if (this.promptDispatchInFlight) {
        queueMicrotask(() => {
          const failure = createFailureMessage(
            new Error("Prompt dispatch already in flight."),
            provider,
            modelId,
            "error",
          );
          this.agent.state.error = failure.errorMessage;
          stream.push({
            type: "error",
            reason: "error",
            error: failure,
          });
        });
        return stream;
      }

      const activeStreamId = request.streamId;
      let completed = false;

      const cleanup = () => {
        this.rpcClient.removeMessageListener("sendStreamEvent", streamListener);
        if (streamOptions?.signal) {
          streamOptions.signal.removeEventListener("abort", abort);
        }
      };

      const finish = (): void => {
        this.promptDispatchInFlight = false;
        this.promptStatus = this.agent.state.isStreaming ? "streaming" : "idle";
        if (!this.agent.state.isStreaming && this.pendingSnapshot) {
          const pendingSnapshot = this.pendingSnapshot;
          this.pendingSnapshot = null;
          this.applySnapshot(pendingSnapshot);
          return;
        }
        this.emit();
      };

      const finishWithError = (stopReason: "aborted" | "error", error: unknown): void => {
        if (completed) {
          return;
        }
        completed = true;
        cleanup();
        const failure = createFailureMessage(error, provider, modelId, stopReason);
        this.agent.state.error = failure.errorMessage;
        this.promptDispatchInFlight = false;
        this.promptStatus = "idle";
        stream.push({
          type: "error",
          reason: stopReason,
          error: failure,
        });
        this.emit();
      };

      const handleStreamPayload = (payload: { streamId: string; event: AssistantMessageEvent }) => {
        if (completed || payload.streamId !== activeStreamId) {
          return;
        }

        stream.push(payload.event);
        if (payload.event.type === "done" || payload.event.type === "error") {
          completed = true;
          cleanup();
          finish();
        }
      };

      const streamListener = (payload: { streamId: string; event: AssistantMessageEvent }) => {
        handleStreamPayload(payload);
      };

      const abort = (): void => {
        if (completed) {
          return;
        }
        void this.rpcClient.request.cancelPrompt({ target: this.target });
        finishWithError("aborted", new Error("Request aborted by user"));
      };

      this.promptDispatchInFlight = true;
      this.promptStatus = "streaming";
      this.emit();

      this.rpcClient.addMessageListener("sendStreamEvent", streamListener);
      if (streamOptions?.signal) {
        streamOptions.signal.addEventListener("abort", abort, { once: true });
        if (streamOptions.signal.aborted) {
          abort();
        }
      }

      void (async () => {
        try {
          const response = await this.rpcClient.request.sendPrompt(request);
          this.target = normalizePromptTarget(response.target);
          this.agent.sessionId = response.target.surfacePiSessionId;
          this.emit();

          if (streamOptions?.signal?.aborted) {
            abort();
          }
        } catch (error) {
          finishWithError("error", error);
        }
      })();

      return stream;
    };
  }

  private async syncSurfaceModel(providerId: string, modelId: string): Promise<void> {
    try {
      const response = await this.rpcClient.request.setSurfaceModel({
        target: this.target,
        provider: providerId,
        model: modelId,
      });
      if (response.ok) {
        this.target = normalizePromptTarget(response.target);
        this.agent.sessionId = response.target.surfacePiSessionId;
        this.emit();
      }
    } catch (error) {
      console.error("Failed to sync session model:", error);
    }
  }

  private async syncSurfaceThoughtLevel(level: ReasoningEffort): Promise<void> {
    try {
      const response = await this.rpcClient.request.setSurfaceThoughtLevel({
        target: this.target,
        level,
      });
      if (response.ok) {
        this.target = normalizePromptTarget(response.target);
        this.agent.sessionId = response.target.surfacePiSessionId;
        this.emit();
      }
    } catch (error) {
      console.error("Failed to sync session thought level:", error);
    }
  }
}

export async function createChatRuntime(
  options: ChatRuntimeOptions = {},
  rpcClient: ChatRuntimeRpcClient = DEFAULT_RPC_CLIENT,
  storageOverride?: ChatStorage,
): Promise<ChatRuntime> {
  const storage = storageOverride ?? initializeStorage();
  const listeners = new Set<ChatRuntimeListener>();
  const appLogUpdateListeners = new Set<(payload: AppLogUpdateMessage) => void>();
  const surfaceControllers = new Map<string, ChatSurfaceControllerInternal>();
  let sessions: WorkspaceSessionSummary[] = [];
  let sessionNavigation: WorkspaceSessionNavigationReadModel = buildWorkspaceSessionNavigation([]);
  let appLogSummary: AppLogSummary = {
    latestSeq: 0,
    seenSeq: 0,
    unread: { total: 0, info: 0, warning: 0, error: 0 },
    totals: { total: 0, info: 0, warning: 0, error: 0 },
  };
  let paneLayout = createEmptyPaneLayout();
  let disposed = false;

  const emit = () => {
    if (disposed) {
      return;
    }

    for (const listener of listeners) {
      listener();
    }
  };

  const persistWorkspaceUiRestore = (): void => {
    if (disposed) {
      return;
    }

    const state: WorkspaceUiRestoreState = {
      version: 3,
      dockview: paneLayout.dockview,
      panels: paneLayout.panels,
      compactSurfaces: paneLayout.compactSurfaces,
      focusedPanelId: paneLayout.focusedPanelId,
      updatedAt: new Date().toISOString(),
    };

    void storage.workspaceUiRestore
      .set(workspaceInfo.workspaceId, state)
      .catch((error) => console.error("Failed to persist workspace UI restore state:", error));
  };

  const syncPaneTargetForSurface = (target: PromptTarget): void => {
    const normalizedTarget = normalizePromptTarget(target);
    paneLayout = {
      ...paneLayout,
      panels: paneLayout.panels.map((pane) =>
        isPromptTarget(pane.binding) &&
        pane.binding.surfacePiSessionId === normalizedTarget.surfacePiSessionId
          ? { ...pane, binding: normalizedTarget }
          : pane,
      ),
      updatedAt: new Date().toISOString(),
    };
  };

  const upsertSurfaceController = (
    snapshot: ConversationSurfaceSnapshot,
  ): ChatSurfaceControllerInternal => {
    const surfacePiSessionId = snapshot.target.surfacePiSessionId;
    const existing = surfaceControllers.get(surfacePiSessionId);
    if (existing) {
      existing.applySnapshot(snapshot);
      syncPaneTargetForSurface(snapshot.target);
      return existing;
    }

    const controller = new SurfaceControllerImpl(snapshot, rpcClient);
    surfaceControllers.set(surfacePiSessionId, controller);
    return controller;
  };

  const clearPaneBinding = (panelId: string): void => {
    const target = paneLayout.panels.find((pane) => pane.panelId === panelId)?.binding ?? null;
    if (!target) {
      return;
    }

    paneLayout = bindPane(paneLayout, panelId, null);
    if (isPromptTarget(target)) {
      surfaceControllers.get(target.surfacePiSessionId)?.detachPane(panelId);
    }
    persistWorkspaceUiRestore();
  };

  const releasePaneSurface = async (
    panelId: string,
    target: WorkspacePaneSurfaceTarget | null,
  ): Promise<void> => {
    if (!isPromptTarget(target)) {
      return;
    }

    const controller = surfaceControllers.get(target.surfacePiSessionId);
    controller?.detachPane(panelId);
    if (controller && controller.ownerPaneIds.length > 0) {
      return;
    }

    try {
      await rpcClient.request.closeSurface({ target });
    } catch (error) {
      console.error("Failed to close surface:", error);
    }
  };

  const bindPaneToSnapshot = async (
    panelId: string,
    snapshot: ConversationSurfaceSnapshot,
    bindOptions: { focus?: boolean; persist?: boolean } = {},
  ): Promise<void> => {
    const focus = bindOptions.focus ?? true;
    const persist = bindOptions.persist ?? true;
    const previousFocusedPaneId = paneLayout.focusedPanelId;
    const previousTarget =
      paneLayout.panels.find((pane) => pane.panelId === panelId)?.binding ?? null;
    const nextTarget = normalizePromptTarget(snapshot.target);
    if (
      isPromptTarget(previousTarget) &&
      previousTarget.surfacePiSessionId === nextTarget.surfacePiSessionId
    ) {
      paneLayout = bindPane(paneLayout, panelId, nextTarget);
      if (!focus) {
        paneLayout = { ...paneLayout, focusedPanelId: previousFocusedPaneId };
      }
      upsertSurfaceController({ ...snapshot, target: nextTarget }).attachPane(panelId);
      emit();
      if (persist) {
        persistWorkspaceUiRestore();
      }
      return;
    }

    const controller = upsertSurfaceController({ ...snapshot, target: nextTarget });
    paneLayout = bindPane(paneLayout, panelId, nextTarget);
    if (!focus) {
      paneLayout = { ...paneLayout, focusedPanelId: previousFocusedPaneId };
    }
    controller.attachPane(panelId);
    emit();

    if (isPromptTarget(previousTarget)) {
      surfaceControllers.get(previousTarget.surfacePiSessionId)?.detachPane(panelId);
    }
    if (persist) {
      persistWorkspaceUiRestore();
    }
  };

  const bindPaneToExistingController = (
    panelId: string,
    controller: ChatSurfaceControllerInternal,
  ): void => {
    const previousTarget =
      paneLayout.panels.find((pane) => pane.panelId === panelId)?.binding ?? null;
    const nextTarget = normalizePromptTarget(controller.target);
    paneLayout = bindPane(paneLayout, panelId, nextTarget);
    controller.attachPane(panelId);
    emit();

    if (
      isPromptTarget(previousTarget) &&
      previousTarget.surfacePiSessionId !== nextTarget.surfacePiSessionId
    ) {
      surfaceControllers.get(previousTarget.surfacePiSessionId)?.detachPane(panelId);
    }
    persistWorkspaceUiRestore();
  };

  const refreshSessions = async (): Promise<WorkspaceSessionSummary[]> => {
    const response = await rpcClient.request.listSessions();
    sessions = response.sessions;
    sessionNavigation = response.navigation;
    emit();
    return sessions;
  };

  const getSelectedSessionId = (sessionId?: string): string | undefined => {
    if (sessionId) {
      return sessionId;
    }

    const focusedTarget =
      paneLayout.panels.find((pane) => pane.panelId === paneLayout.focusedPanelId)?.binding ?? null;
    return focusedTarget?.workspaceSessionId;
  };

  const syncProviderAuth = async (providerId: string): Promise<boolean> => {
    const auth = await rpcClient.request.getProviderAuthState({ providerId });
    if (auth.connected) {
      await storage.providerKeys.set(providerId, auth.accountId || "oauth");
      return true;
    }

    await storage.providerKeys.delete(providerId);
    return false;
  };

  const requireProviderAccess = async (providerId: string): Promise<boolean> => {
    const hasAccess = await syncProviderAuth(providerId);
    if (!hasAccess) {
      options.onMissingProviderAccess?.(providerId);
    }
    return hasAccess;
  };

  const listConfiguredProviders = async (): Promise<string[]> => {
    const auths = await rpcClient.request.listProviderAuths();
    return auths.filter((authInfo) => authInfo.hasKey).map((authInfo) => authInfo.provider);
  };

  const getCommandInspector = async (
    commandId: string,
    sessionId = getSelectedSessionId(),
  ): Promise<WorkspaceCommandInspector> => {
    if (!sessionId) {
      throw new Error("Expected a workspace session before inspecting a command.");
    }

    const inspector = await rpcClient.request.getCommandInspector({
      sessionId,
      commandId,
    });
    if (!inspector) {
      throw new Error(`Structured command not found: ${commandId}`);
    }

    return inspector;
  };

  const listHandlerThreads = async (
    sessionId = getSelectedSessionId(),
  ): Promise<WorkspaceHandlerThreadSummary[]> => {
    if (!sessionId) {
      throw new Error("Expected a workspace session before listing handler threads.");
    }

    return await rpcClient.request.listHandlerThreads({ sessionId });
  };

  const getHandlerThreadInspector = async (
    threadId: string,
    sessionId = getSelectedSessionId(),
  ): Promise<WorkspaceHandlerThreadInspector> => {
    if (!sessionId) {
      throw new Error("Expected a workspace session before inspecting a handler thread.");
    }

    const inspector = await rpcClient.request.getHandlerThreadInspector({
      sessionId,
      threadId,
    });
    if (!inspector) {
      throw new Error(`Delegated handler thread not found: ${threadId}`);
    }

    return inspector;
  };

  const getWorkflowTaskAttemptInspector = async (
    workflowTaskAttemptId: string,
    sessionId = getSelectedSessionId(),
  ): Promise<WorkspaceWorkflowTaskAttemptInspector> => {
    if (!sessionId) {
      throw new Error("Expected a workspace session before inspecting a workflow task attempt.");
    }

    const inspector = await rpcClient.request.getWorkflowTaskAttemptInspector({
      sessionId,
      workflowTaskAttemptId,
    });
    if (!inspector) {
      throw new Error(`Workflow task attempt not found: ${workflowTaskAttemptId}`);
    }

    return inspector;
  };

  const getWorkflowInspector = async (
    workflowRunId: string,
    request: {
      sessionId?: string;
      selectedNodeKey?: string | null;
      expandedNodeKeys?: string[];
      userCollapsedNodeKeys?: string[];
      searchQuery?: string;
      mode?: WorkspaceWorkflowInspectorMode;
    } = {},
  ): Promise<WorkspaceWorkflowInspectorReadModel> => {
    const sessionId = request.sessionId ?? getSelectedSessionId();
    if (!sessionId) {
      throw new Error("Expected a workspace session before inspecting a workflow run.");
    }

    const inspector = await rpcClient.request.getWorkflowInspector({
      sessionId,
      workflowRunId,
      selectedNodeKey: request.selectedNodeKey,
      expandedNodeKeys: request.expandedNodeKeys,
      userCollapsedNodeKeys: request.userCollapsedNodeKeys,
      searchQuery: request.searchQuery,
      mode: request.mode,
    });
    if (!inspector) {
      throw new Error(`Workflow inspector not found: ${workflowRunId}`);
    }

    return inspector;
  };

  const streamWorkflowInspector = async (
    workflowRunId: string,
    request: {
      sessionId?: string;
      selectedNodeKey?: string | null;
      expandedNodeKeys?: string[];
      userCollapsedNodeKeys?: string[];
      searchQuery?: string;
      mode?: WorkspaceWorkflowInspectorMode;
      fromSeq?: number | null;
    } = {},
  ): Promise<WorkspaceWorkflowInspectorLiveUpdate> => {
    const sessionId = request.sessionId ?? getSelectedSessionId();
    if (!sessionId) {
      throw new Error("Expected a workspace session before streaming a workflow inspector.");
    }
    const update = await rpcClient.request.streamWorkflowInspector({
      sessionId,
      workflowRunId,
      selectedNodeKey: request.selectedNodeKey,
      expandedNodeKeys: request.expandedNodeKeys,
      userCollapsedNodeKeys: request.userCollapsedNodeKeys,
      searchQuery: request.searchQuery,
      mode: request.mode,
      fromSeq: request.fromSeq,
    });
    if (!update) {
      throw new Error(`Workflow inspector stream not found: ${workflowRunId}`);
    }
    return update;
  };

  const getProjectCiStatus = async (
    sessionId = getSelectedSessionId(),
  ): Promise<WorkspaceProjectCiStatusPanel> => {
    if (!sessionId) {
      throw new Error("Expected a workspace session before loading Project CI status.");
    }

    return await rpcClient.request.getProjectCiStatus({ sessionId });
  };

  const getArtifactPreview = async (
    artifactId: string,
    sessionId = getSelectedSessionId(),
  ): Promise<WorkspaceArtifactPreview> => {
    if (!sessionId) {
      throw new Error("Expected a workspace session before opening an artifact.");
    }

    return await rpcClient.request.getArtifactPreview({ sessionId, artifactId });
  };

  const resolveOpenTarget = (openTarget?: PaneOpenTarget | string): string => {
    const createPanelForOpen = (panelId = PRIMARY_CHAT_PANE_ID): string => {
      paneLayout = addDockviewPanel(paneLayout, null, panelId);
      persistWorkspaceUiRestore();
      emit();
      return panelId;
    };

    if (typeof openTarget === "string") {
      if (!paneLayout.panels.some((pane) => pane.panelId === openTarget)) {
        return createPanelForOpen(openTarget);
      }
      return openTarget;
    }
    if (!openTarget || openTarget.kind === "focused-panel") {
      return paneLayout.focusedPanelId ?? paneLayout.panels[0]?.panelId ?? createPanelForOpen();
    }
    if (openTarget.kind === "panel") {
      if (!paneLayout.panels.some((pane) => pane.panelId === openTarget.panelId)) {
        return createPanelForOpen(openTarget.panelId);
      }
      return openTarget.panelId;
    }
    if (openTarget.kind === "split") {
      if (!paneLayout.panels.some((pane) => pane.panelId === openTarget.panelId)) {
        return createPanelForOpen(openTarget.panelId);
      }
      const before = new Set(paneLayout.panels.map((pane) => pane.panelId));
      paneLayout = splitPane(paneLayout, openTarget.panelId, openTarget.direction, {
        size: openTarget.size,
      });
      persistWorkspaceUiRestore();
      emit();
      return (
        paneLayout.panels.find((pane) => !before.has(pane.panelId))?.panelId ?? openTarget.panelId
      );
    }
    if (openTarget.kind === "tab") {
      return createPanelForOpen(createPanelId());
    }
    if (paneLayout.panels.length === 0) {
      return createPanelForOpen();
    }
    const basePaneId =
      paneLayout.focusedPanelId ?? paneLayout.panels[0]?.panelId ?? PRIMARY_CHAT_PANE_ID;
    const before = new Set(paneLayout.panels.map((pane) => pane.panelId));
    const direction =
      openTarget.kind === "new-panel"
        ? openTarget.direction
        : openTarget.kind === "edge"
          ? openTarget.direction
          : "right";
    const size = "size" in openTarget ? openTarget.size : undefined;
    paneLayout = splitPane(paneLayout, basePaneId, direction, { size });
    persistWorkspaceUiRestore();
    emit();
    return paneLayout.panels.find((pane) => !before.has(pane.panelId))?.panelId ?? basePaneId;
  };

  const [defaults, workspaceInfo, initialCatalog, initialAppLogSummary] = await Promise.all([
    rpcClient.request.getDefaults(),
    rpcClient.request.getWorkspaceInfo(),
    rpcClient.request.listSessions(),
    rpcClient.request.getAppLogSummary(),
  ]);
  sessions = initialCatalog.sessions;
  sessionNavigation = initialCatalog.navigation;
  appLogSummary = initialAppLogSummary;

  const syncProviderAuthPromise = syncProviderAuth(defaults.provider);
  await syncProviderAuthPromise;

  const restoreState = await storage.workspaceUiRestore
    .get(workspaceInfo.workspaceId)
    .catch((error) => {
      console.error("Failed to load workspace UI restore state:", error);
      return null;
    });
  let restoredPaneIds: string[] = [];
  if (restoreState?.panels.length) {
    const sessionIds = new Set(initialCatalog.sessions.map((session) => session.id));
    paneLayout = normalizePaneLayout(restoreState);
    const hasOnlyRestorablePanes = paneLayout.panels.every(
      (paneState) =>
        !paneState.binding ||
        paneState.binding.surface === "app-logs" ||
        sessionIds.has(paneState.binding.workspaceSessionId),
    );
    if (!hasOnlyRestorablePanes) {
      paneLayout = createEmptyPaneLayout();
    }
    for (const paneState of paneLayout.panels) {
      if (
        !paneState.binding ||
        (paneState.binding.surface !== "app-logs" &&
          !sessionIds.has(paneState.binding.workspaceSessionId))
      ) {
        continue;
      }

      if (!isPromptTarget(paneState.binding)) {
        restoredPaneIds.push(paneState.panelId);
        continue;
      }

      const target = normalizePromptTarget(paneState.binding);

      try {
        const snapshot =
          target.surface === "orchestrator"
            ? await rpcClient.request.openSession({ sessionId: target.workspaceSessionId })
            : await rpcClient.request.openSurface({ target });
        await bindPaneToSnapshot(paneState.panelId, snapshot, { focus: false, persist: false });
        restoredPaneIds.push(paneState.panelId);
      } catch (error) {
        console.error("Failed to restore workspace pane:", error);
        restoredPaneIds.push(paneState.panelId);
      }
    }
    if (restoredPaneIds.length === 0 && paneLayout.panels.every((paneState) => paneState.binding)) {
      paneLayout = createEmptyPaneLayout();
    }
  }

  if (restoreState?.panels.length && paneLayout.panels.some((paneState) => !paneState.binding)) {
    const focusedPanelId =
      restoreState.focusedPanelId &&
      paneLayout.panels.some((pane) => pane.panelId === restoreState.focusedPanelId)
        ? restoreState.focusedPanelId
        : (paneLayout.panels[0]?.panelId ?? PRIMARY_CHAT_PANE_ID);
    paneLayout = { ...paneLayout, focusedPanelId };
    if (restoredPaneIds.length === 0 && initialCatalog.sessions.length > 0) {
      const [initialSession] = initialCatalog.sessions;
      const snapshot = await rpcClient.request.openSession({ sessionId: initialSession!.id });
      await bindPaneToSnapshot(focusedPanelId, snapshot, { focus: true, persist: false });
    } else if (restoredPaneIds.length === 0) {
      const snapshot = await rpcClient.request.createSession({});
      await bindPaneToSnapshot(focusedPanelId, snapshot, { focus: true, persist: false });
      await refreshSessions();
    }
    persistWorkspaceUiRestore();
    emit();
  } else if (restoredPaneIds.length > 0) {
    paneLayout = {
      ...paneLayout,
      focusedPanelId:
        restoreState?.focusedPanelId && restoredPaneIds.includes(restoreState.focusedPanelId)
          ? restoreState.focusedPanelId
          : restoredPaneIds[0]!,
    };
    persistWorkspaceUiRestore();
    emit();
  } else if (initialCatalog.sessions.length > 0) {
    const [initialSession] = initialCatalog.sessions;
    if (!initialSession) {
      throw new Error("Expected an initial session to open.");
    }
    const snapshot = await rpcClient.request.openSession({ sessionId: initialSession.id });
    await bindPaneToSnapshot(PRIMARY_CHAT_PANE_ID, snapshot);
  } else {
    const snapshot = await rpcClient.request.createSession({});
    await bindPaneToSnapshot(PRIMARY_CHAT_PANE_ID, snapshot);
    await refreshSessions();
  }

  const workspaceSyncListener = (payload: WorkspaceSyncMessage) => {
    sessions = payload.sessions;
    sessionNavigation = payload.navigation;
    emit();
  };

  const surfaceSyncListener = (payload: SurfaceSyncMessage) => {
    syncPaneTargetForSurface(payload.target);
    persistWorkspaceUiRestore();
    if (payload.reason === "surface.closed") {
      for (const pane of paneLayout.panels) {
        if (
          isPromptTarget(pane.binding) &&
          pane.binding.surfacePiSessionId === payload.target.surfacePiSessionId
        ) {
          clearPaneBinding(pane.panelId);
        }
      }

      const existing = surfaceControllers.get(payload.target.surfacePiSessionId);
      if (existing) {
        surfaceControllers.delete(payload.target.surfacePiSessionId);
        existing.dispose();
      }
      emit();
      return;
    }

    if (!payload.snapshot) {
      return;
    }

    upsertSurfaceController(payload.snapshot);
    emit();
  };

  const appLogUpdateListener = (payload: AppLogUpdateMessage) => {
    appLogSummary = payload.summary;
    for (const listener of appLogUpdateListeners) {
      listener(payload);
    }
    emit();
  };

  rpcClient.addMessageListener("sendWorkspaceSync", workspaceSyncListener);
  rpcClient.addMessageListener("sendSurfaceSync", surfaceSyncListener);
  rpcClient.addMessageListener("sendAppLogUpdate", appLogUpdateListener);

  const runtime: ChatRuntime = {
    storage,
    workspaceId: workspaceInfo.workspaceId,
    workspaceLabel: workspaceInfo.workspaceLabel,
    branch: workspaceInfo.branch,
    primaryPaneId: PRIMARY_CHAT_PANE_ID,
    get sessions() {
      return sessions;
    },
    get sessionNavigation() {
      return sessionNavigation;
    },
    get appLogSummary() {
      return structuredClone(appLogSummary);
    },
    get paneLayout() {
      return structuredClone(paneLayout);
    },
    dispose: () => {
      disposed = true;
      rpcClient.removeMessageListener("sendWorkspaceSync", workspaceSyncListener);
      rpcClient.removeMessageListener("sendSurfaceSync", surfaceSyncListener);
      rpcClient.removeMessageListener("sendAppLogUpdate", appLogUpdateListener);
      for (const controller of surfaceControllers.values()) {
        controller.dispose();
      }
      appLogUpdateListeners.clear();
      listeners.clear();
    },
    subscribe: (listener) => {
      listeners.add(listener);
      listener();
      return () => {
        listeners.delete(listener);
      };
    },
    subscribeAppLogUpdate: (listener) => {
      appLogUpdateListeners.add(listener);
      return () => {
        appLogUpdateListeners.delete(listener);
      };
    },
    subscribeAppMenuAction: (listener) => {
      const appMenuListener = ({ action }: { action: AppMenuAction }) => {
        listener(action);
      };
      rpcClient.addMessageListener("sendAppMenuAction", appMenuListener);
      return () => {
        rpcClient.removeMessageListener("sendAppMenuAction", appMenuListener);
      };
    },
    listSessions: refreshSessions,
    getPane: (panelId) => {
      const pane = paneLayout.panels.find((candidate) => candidate.panelId === panelId);
      if (!pane) {
        return undefined;
      }
      return {
        id: pane.panelId,
        target: pane.binding ? { ...pane.binding } : null,
        inspectorSelection: pane.localState.inspectorSelection,
        scroll: pane.localState.scroll,
        timelineDensity: pane.localState.timelineDensity,
      };
    },
    getPaneController: (panelId) => {
      const target = paneLayout.panels.find((pane) => pane.panelId === panelId)?.binding ?? null;
      if (!isPromptTarget(target)) {
        return null;
      }
      return surfaceControllers.get(target.surfacePiSessionId) ?? null;
    },
    getSurfaceController: (surfacePiSessionId) => {
      return surfaceControllers.get(surfacePiSessionId) ?? null;
    },
    focusPane: (panelId) => {
      paneLayout = focusPane(paneLayout, panelId);
      persistWorkspaceUiRestore();
      emit();
    },
    splitPane: async (panelId, direction, splitOptions = {}) => {
      const before = new Set(paneLayout.panels.map((pane) => pane.panelId));
      const sourceBinding =
        paneLayout.panels.find((pane) => pane.panelId === panelId)?.binding ?? null;
      paneLayout = splitPane(paneLayout, panelId, direction, splitOptions);
      const newPane = paneLayout.panels.find((pane) => !before.has(pane.panelId)) ?? null;
      if (!newPane) {
        return null;
      }
      if (splitOptions.duplicateBinding && isPromptTarget(sourceBinding)) {
        surfaceControllers.get(sourceBinding.surfacePiSessionId)?.attachPane(newPane.panelId);
      }
      persistWorkspaceUiRestore();
      emit();
      return newPane.panelId;
    },
    closePane: async (panelId) => {
      const targetPanelId = paneLayout.panels.some((pane) => pane.panelId === panelId)
        ? panelId
        : paneLayout.panels.some((pane) => pane.panelId === paneLayout.focusedPanelId)
          ? paneLayout.focusedPanelId!
          : (paneLayout.panels.at(-1)?.panelId ?? panelId);
      const target =
        paneLayout.panels.find((pane) => pane.panelId === targetPanelId)?.binding ?? null;
      paneLayout = closePane(paneLayout, targetPanelId);
      persistWorkspaceUiRestore();
      emit();
      await releasePaneSurface(targetPanelId, target);
    },
    setDockviewLayout: (dockview, focusedPanelId) => {
      paneLayout = setDockviewSerializedLayout(
        paneLayout,
        dockview,
        focusedPanelId ?? paneLayout.focusedPanelId,
      );
      persistWorkspaceUiRestore();
      emit();
    },
    getCommandInspector,
    listHandlerThreads,
    getHandlerThreadInspector,
    getWorkflowTaskAttemptInspector,
    getWorkflowInspector,
    streamWorkflowInspector,
    getProjectCiStatus,
    getArtifactPreview,
    getAppLogs: (query) => rpcClient.request.getAppLogs(query),
    getAppLogSummary: async () => {
      appLogSummary = await rpcClient.request.getAppLogSummary();
      emit();
      return structuredClone(appLogSummary);
    },
    markAppLogsSeen: async (throughSeq) => {
      if (throughSeq <= appLogSummary.seenSeq) {
        return structuredClone(appLogSummary);
      }
      appLogSummary = await rpcClient.request.markAppLogsSeen({ throughSeq });
      emit();
      return structuredClone(appLogSummary);
    },
    writeClipboardText: async (text) => {
      await rpcClient.request.writeClipboardText({ text });
    },
    createSession: async (request = {}, openTarget) => {
      const nextPaneId = resolveOpenTarget(openTarget);
      const snapshot = await rpcClient.request.createSession(request);
      await bindPaneToSnapshot(nextPaneId, snapshot);
      await refreshSessions();
    },
    openSession: async (sessionId, openTarget) => {
      const nextPaneId = resolveOpenTarget(openTarget);
      const currentTarget =
        paneLayout.panels.find((pane) => pane.panelId === nextPaneId)?.binding ?? null;
      if (
        currentTarget?.workspaceSessionId === sessionId &&
        currentTarget.surface === "orchestrator" &&
        currentTarget.surfacePiSessionId === sessionId
      ) {
        paneLayout = focusPane(paneLayout, nextPaneId);
        persistWorkspaceUiRestore();
        emit();
        return;
      }

      const existingController = surfaceControllers.get(sessionId);
      if (existingController) {
        bindPaneToExistingController(nextPaneId, existingController);
        void rpcClient.request.recordSessionOpened({ sessionId });
        return;
      }

      const snapshot = await rpcClient.request.openSession({ sessionId });
      await bindPaneToSnapshot(nextPaneId, snapshot);
    },
    openSurface: async (target, openTarget) => {
      const nextPaneId = resolveOpenTarget(openTarget);
      if (
        target.surface === "workflow-inspector" ||
        target.surface === "command" ||
        target.surface === "workflow-task-attempt" ||
        target.surface === "artifact" ||
        target.surface === "project-ci-check" ||
        target.surface === "saved-workflow-library" ||
        target.surface === "app-logs"
      ) {
        const previousTarget =
          paneLayout.panels.find((pane) => pane.panelId === nextPaneId)?.binding ?? null;
        if (isPromptTarget(previousTarget)) {
          surfaceControllers.get(previousTarget.surfacePiSessionId)?.detachPane(nextPaneId);
        }
        paneLayout = bindPane(paneLayout, nextPaneId, { ...target });
        persistWorkspaceUiRestore();
        emit();
        return;
      }
      const normalizedTarget = normalizePromptTarget(target);
      const currentTarget =
        paneLayout.panels.find((pane) => pane.panelId === nextPaneId)?.binding ?? null;
      if (
        isPromptTarget(currentTarget) &&
        currentTarget.surfacePiSessionId === normalizedTarget.surfacePiSessionId
      ) {
        paneLayout = bindPane(paneLayout, nextPaneId, normalizedTarget);
        surfaceControllers.get(normalizedTarget.surfacePiSessionId)?.attachPane(nextPaneId);
        persistWorkspaceUiRestore();
        emit();
        return;
      }

      const existingController = surfaceControllers.get(normalizedTarget.surfacePiSessionId);
      if (existingController) {
        bindPaneToExistingController(nextPaneId, existingController);
        return;
      }

      const snapshot = await rpcClient.request.openSurface({ target: normalizedTarget });
      await bindPaneToSnapshot(nextPaneId, snapshot);
    },
    closePaneSurface: async (panelId) => {
      const target = paneLayout.panels.find((pane) => pane.panelId === panelId)?.binding ?? null;
      if (!target) {
        return;
      }

      clearPaneBinding(panelId);
      emit();
      await releasePaneSurface(panelId, target);
    },
    renameSession: async (sessionId, title) => {
      await rpcClient.request.renameSession({ sessionId, title });
      await refreshSessions();
    },
    setSessionMode: async (panelId, mode) => {
      const target = paneLayout.panels.find((pane) => pane.panelId === panelId)?.binding ?? null;
      if (!isPromptTarget(target) || target.surface !== "orchestrator") {
        return;
      }
      const response = await rpcClient.request.setSessionMode({ target, mode });
      if (!response.ok || !response.snapshot) {
        throw new Error(response.error ?? "Session mode update failed.");
      }
      await bindPaneToSnapshot(panelId, response.snapshot);
      await refreshSessions();
    },
    forkSession: async (sessionId, title, openTarget, options) => {
      const nextPaneId = resolveOpenTarget(openTarget);
      const snapshot = await rpcClient.request.forkSession({
        sessionId,
        title,
        messageTimestamp: options?.messageTimestamp,
      });
      await bindPaneToSnapshot(nextPaneId, snapshot);
      await refreshSessions();
    },
    deleteSession: async (sessionId, panelId) => {
      const fallbackPaneId =
        panelId ??
        paneLayout.focusedPanelId ??
        paneLayout.panels[0]?.panelId ??
        PRIMARY_CHAT_PANE_ID;
      const affectedPaneIds = new Set<string>();
      for (const pane of paneLayout.panels) {
        if (pane.binding?.workspaceSessionId === sessionId) {
          affectedPaneIds.add(pane.panelId);
        }
      }

      await rpcClient.request.deleteSession({ sessionId });

      for (const candidatePaneId of affectedPaneIds) {
        clearPaneBinding(candidatePaneId);
      }

      for (const [surfacePiSessionId, controller] of surfaceControllers.entries()) {
        if (controller.target.workspaceSessionId === sessionId) {
          surfaceControllers.delete(surfacePiSessionId);
          controller.dispose();
        }
      }

      await refreshSessions();

      if (affectedPaneIds.has(fallbackPaneId)) {
        const nextSession =
          sessions.find((session) => !session.isArchived) ?? sessions.find((session) => session);
        if (nextSession) {
          await runtime.openSession(nextSession.id, fallbackPaneId);
          return;
        }

        await runtime.createSession({}, fallbackPaneId);
        return;
      }

      emit();
    },
    pinSession: async (sessionId) => {
      await rpcClient.request.pinSession({ sessionId });
      await refreshSessions();
    },
    unpinSession: async (sessionId) => {
      await rpcClient.request.unpinSession({ sessionId });
      await refreshSessions();
    },
    archiveSession: async (sessionId) => {
      await rpcClient.request.archiveSession({ sessionId });
      await refreshSessions();
    },
    unarchiveSession: async (sessionId) => {
      await rpcClient.request.unarchiveSession({ sessionId });
      await refreshSessions();
    },
    setArchivedGroupCollapsed: async (collapsed) => {
      await rpcClient.request.setArchivedGroupCollapsed({ collapsed });
      await refreshSessions();
    },
    setPaneInspectorSelection: (panelId, selection) => {
      paneLayout = setLayoutPaneInspectorSelection(paneLayout, panelId, selection);
      persistWorkspaceUiRestore();
      emit();
    },
    setPaneScroll: (panelId, scroll) => {
      paneLayout = setLayoutPaneScroll(paneLayout, panelId, scroll);
      persistWorkspaceUiRestore();
    },
    sendPromptToTarget: async (target, input) => {
      const text = input.trim();
      if (!text) {
        return;
      }
      await rpcClient.request.sendPrompt({
        streamId: createRpcStreamId(),
        messages: [{ role: "user", content: text } as Message],
        target: normalizePromptTarget(target),
      });
    },
    syncProviderAuth,
    requireProviderAccess,
    listConfiguredProviders,
    listWorkspacePaths: (pathOptions) => rpcClient.request.listWorkspacePaths(pathOptions),
    pickWorkspaceAttachments: async () => {
      const result = await rpcClient.request.pickWorkspaceAttachments();
      return result.entries;
    },
    openWorkspacePath: async (workspaceRelativePath) => {
      const result = await rpcClient.request.openWorkspacePath({ workspaceRelativePath });
      return result.opened;
    },
    getSavedWorkflowLibrary: () => rpcClient.request.getSavedWorkflowLibrary(),
    deleteSavedWorkflowLibraryItem: (path) =>
      rpcClient.request.deleteSavedWorkflowLibraryItem({ path }),
    openWorkflowSourceInEditor: async (path) => {
      const result = await rpcClient.request.openWorkflowSourceInEditor({ path });
      return result.opened;
    },
  };

  return runtime;
}
