import { Agent, type AgentMessage, type StreamFn } from "@mariozechner/pi-agent-core";
import {
  createAssistantMessageEventStream,
  getModel,
  type AssistantMessage,
  type Message,
} from "@mariozechner/pi-ai";
import type {
  AppWorkspaceUiRestoreState,
  AppLogQuery,
  AppLogReadModel,
  AppLogSummary,
  AppLogUpdateMessage,
  ConversationSurfaceSnapshot,
  CreateSessionRequest,
  PromptTarget,
  QueuedSurfaceMessage,
  SendPromptRequest,
  SurfaceSyncMessage,
  WorkspaceBranchInfo,
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
  WorkspaceInfoResponse,
  WorkspaceTabInfo,
} from "../shared/workspace-contract";
import type {
  PromptLibraryActor,
  PromptLibraryExternalSource,
  PromptLibraryGeneratedEntry,
  PromptLibrarySnapshotSummary,
  PromptLibraryState,
  UpdatePromptLibraryRequest,
} from "../shared/prompt-library";
import {
  createChatStorage,
  type ChatStorage,
  type WorkspaceInspectorSelection,
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
  isInitializedPaneLayout,
  normalizePaneLayout,
  PRIMARY_CHAT_PANE_ID,
  setDockviewSerializedLayout,
  setPaneInspectorSelection as setLayoutPaneInspectorSelection,
  setPaneScroll as setLayoutPaneScroll,
  splitPane,
  WORKSPACE_LAYOUT_SLOT_IDS,
  type PaneOpenTarget,
  type DockviewPanelPlacementState,
  type DockviewSplitDirection,
  type WorkspaceDockviewLayoutState,
  type WorkspaceLayoutSlotId,
  type WorkspaceLayoutSlotSummary,
} from "./pane-layout";
import { rpc } from "./rpc";
import { buildWorkspaceSessionNavigation } from "./session-state";

export { PRIMARY_CHAT_PANE_ID } from "./pane-layout";

type WorkspaceUiRestoreState = AppWorkspaceUiRestoreState & {
  layouts: Record<WorkspaceLayoutSlotId, WorkspaceDockviewLayoutState | null>;
};

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

export type QueuedPrompt = QueuedSurfaceMessage;

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
  promptBinding?: ConversationSurfaceSnapshot["promptBinding"];
  externalContextSources: PromptLibraryExternalSource[];
  sessionMode: SessionMode;
  sessionAgentKey: SessionAgentKey;
  promptStatus: PromptStatus;
  queuedPrompts: QueuedPrompt[];
  ownerPaneIds: string[];
  sendPrompt: (input: string) => Promise<void>;
  editQueuedPrompt: (promptId: string) => Promise<string | null>;
  deleteQueuedPrompt: (promptId: string) => Promise<boolean>;
  reorderQueuedPrompt: (promptId: string, beforePromptId: string | null) => Promise<boolean>;
  steerQueuedPrompt: (promptId: string) => Promise<boolean>;
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
    getPromptLibrary: typeof rpc.request.getPromptLibrary;
    getPromptLibraryDefaults: typeof rpc.request.getPromptLibraryDefaults;
    updatePromptLibrary: typeof rpc.request.updatePromptLibrary;
    resetPromptLibrary: typeof rpc.request.resetPromptLibrary;
    listPromptLibrarySnapshots: typeof rpc.request.listPromptLibrarySnapshots;
    createPromptLibrarySnapshot: typeof rpc.request.createPromptLibrarySnapshot;
    renamePromptLibrarySnapshot: typeof rpc.request.renamePromptLibrarySnapshot;
    restorePromptLibrarySnapshot: typeof rpc.request.restorePromptLibrarySnapshot;
    getPromptLibraryExternalSources: typeof rpc.request.getPromptLibraryExternalSources;
    updateSessionAgentDefault: typeof rpc.request.updateSessionAgentDefault;
    updateWorkflowAgent: typeof rpc.request.updateWorkflowAgent;
    updateAppPreferences: typeof rpc.request.updateAppPreferences;
    ensureWorkflowAgentsComponent: typeof rpc.request.ensureWorkflowAgentsComponent;
    getProviderAuthState: typeof rpc.request.getProviderAuthState;
    getOpenWorkspaces: typeof rpc.request.getOpenWorkspaces;
    getWorkspaceInfo: typeof rpc.request.getWorkspaceInfo;
    getWorkspaceUiRestore: typeof rpc.request.getWorkspaceUiRestore;
    setWorkspaceUiRestore: typeof rpc.request.setWorkspaceUiRestore;
    listWorkspaceBranches: typeof rpc.request.listWorkspaceBranches;
    switchWorkspaceBranch: typeof rpc.request.switchWorkspaceBranch;
    getAppLogs: typeof rpc.request.getAppLogs;
    getAppLogSummary: typeof rpc.request.getAppLogSummary;
    markAppLogsSeen: typeof rpc.request.markAppLogsSeen;
    writeClipboardText: typeof rpc.request.writeClipboardText;
    listWorkspacePaths: typeof rpc.request.listWorkspacePaths;
    pickWorkspaceAttachments: typeof rpc.request.pickWorkspaceAttachments;
    openWorkspacePath: typeof rpc.request.openWorkspacePath;
    getSavedWorkflowLibrary: typeof rpc.request.getSavedWorkflowLibrary;
    deleteSavedWorkflowLibraryItem: typeof rpc.request.deleteSavedWorkflowLibraryItem;
    openWorkspaceSourceInEditor: typeof rpc.request.openWorkspaceSourceInEditor;
    openPromptLibraryExternalSourceInEditor: typeof rpc.request.openPromptLibraryExternalSourceInEditor;
    getPromptLibraryGeneratedEntries: typeof rpc.request.getPromptLibraryGeneratedEntries;
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
    markSessionUnread: typeof rpc.request.markSessionUnread;
    markSessionRead: typeof rpc.request.markSessionRead;
    recordFocusedSession: typeof rpc.request.recordFocusedSession;
    setArchivedGroupCollapsed: typeof rpc.request.setArchivedGroupCollapsed;
    setSessionNavigationSectionState: typeof rpc.request.setSessionNavigationSectionState;
    sendPrompt: typeof rpc.request.sendPrompt;
    deleteQueuedSurfaceMessage: typeof rpc.request.deleteQueuedSurfaceMessage;
    editQueuedSurfaceMessage: typeof rpc.request.editQueuedSurfaceMessage;
    reorderQueuedSurfaceMessage: typeof rpc.request.reorderQueuedSurfaceMessage;
    steerQueuedSurfaceMessage: typeof rpc.request.steerQueuedSurfaceMessage;
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
  workspaceInfo?: WorkspaceInfoResponse;
  workspaceId?: string;
  onMissingProviderAccess?: (provider: string) => void;
}

export interface ChatRuntime {
  storage: ChatStorage;
  workspaceId: string;
  workspaceLabel: string;
  cwd: string;
  branch?: string;
  appLogSummary: AppLogSummary;
  sessions: WorkspaceSessionSummary[];
  sessionNavigation: WorkspaceSessionNavigationReadModel;
  paneLayout: ChatPaneLayoutState;
  activeLayoutId: WorkspaceLayoutSlotId;
  layoutSlots: WorkspaceLayoutSlotSummary[];
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
  switchWorkspaceLayout: (layoutId: WorkspaceLayoutSlotId) => Promise<void>;
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
  markSessionUnread: (sessionId: string) => Promise<void>;
  markSessionRead: (sessionId: string) => Promise<void>;
  setArchivedGroupCollapsed: (collapsed: boolean) => Promise<void>;
  setSessionNavigationSectionState: (
    section: "pinned" | "active" | "archived",
    state: { collapsed?: boolean; sizePx?: number },
  ) => Promise<void>;
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
  listOpenWorkspaces: () => Promise<WorkspaceTabInfo[]>;
  listWorkspaceBranches: () => Promise<WorkspaceBranchInfo[]>;
  switchWorkspaceBranch: (branch: string) => Promise<void>;
  listWorkspacePaths: (options?: { refresh?: boolean }) => Promise<WorkspacePathIndexEntry[]>;
  pickWorkspaceAttachments: () => Promise<WorkspacePathIndexEntry[]>;
  openWorkspacePath: (workspaceRelativePath: string) => Promise<boolean>;
  getSavedWorkflowLibrary: () => Promise<WorkspaceSavedWorkflowLibraryReadModel>;
  deleteSavedWorkflowLibraryItem: (path: string) => Promise<WorkspaceSavedWorkflowLibraryReadModel>;
  openWorkspaceSourceInEditor: (path: string) => Promise<boolean>;
  openPromptLibraryExternalSourceInEditor: (path: string) => Promise<boolean>;
  getPromptLibrary: () => Promise<PromptLibraryState>;
  getPromptLibraryDefaults: () => Promise<PromptLibraryState>;
  getPromptLibraryGeneratedEntries: () => Promise<
    Record<PromptLibraryActor, PromptLibraryGeneratedEntry[]>
  >;
  getPromptLibraryExternalSources: () => Promise<PromptLibraryExternalSource[]>;
  updatePromptLibrary: (request: UpdatePromptLibraryRequest) => Promise<PromptLibraryState>;
  resetPromptLibrary: () => Promise<PromptLibraryState>;
  listPromptLibrarySnapshots: () => Promise<PromptLibrarySnapshotSummary[]>;
  createPromptLibrarySnapshot: (name: string) => Promise<PromptLibrarySnapshotSummary>;
  renamePromptLibrarySnapshot: (
    snapshotId: string,
    name: string,
  ) => Promise<PromptLibrarySnapshotSummary>;
  restorePromptLibrarySnapshot: (snapshotId: string) => Promise<PromptLibraryState>;
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
  agent.replaceMessages(buildDisplayMessages(payload));
  agent.state.streamMessage = payload.streamMessage ? structuredClone(payload.streamMessage) : null;
  agent.state.isStreaming = payload.promptStatus === "streaming";
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
      messages: buildDisplayMessages(snapshot),
      streamMessage: snapshot.streamMessage ? structuredClone(snapshot.streamMessage) : null,
      isStreaming: snapshot.promptStatus === "streaming",
      tools: [],
    },
    convertToLlm,
    streamFn,
  });
  agent.sessionId = snapshot.target.surfacePiSessionId;
  return agent;
}

function buildDisplayMessages(snapshot: ConversationSurfaceSnapshot): AgentMessage[] {
  const messages = structuredClone(snapshot.messages);
  if (!snapshot.pendingUserMessage) {
    return messages;
  }
  return [...messages, structuredClone(snapshot.pendingUserMessage)];
}

class SurfaceControllerImpl implements ChatSurfaceControllerInternal {
  agent: Agent;
  target: PromptTarget;
  resolvedSystemPrompt: string;
  promptBinding?: ConversationSurfaceSnapshot["promptBinding"];
  externalContextSources: PromptLibraryExternalSource[];
  sessionMode: SessionMode;
  sessionAgentKey: SessionAgentKey;
  promptStatus: PromptStatus;
  queuedPrompts: QueuedPrompt[] = [];

  private listeners = new Set<ChatRuntimeListener>();
  private panelIds = new Set<string>();
  private disposed = false;
  private promptDispatchInFlight = false;
  private applyingSnapshot = false;
  private suppressSurfaceMutationSync = false;

  constructor(
    snapshot: ConversationSurfaceSnapshot,
    private readonly rpcClient: ChatRuntimeRpcClient,
    private readonly workspaceId: string,
  ) {
    this.target = normalizePromptTarget(snapshot.target);
    this.resolvedSystemPrompt = snapshot.resolvedSystemPrompt;
    this.promptBinding = snapshot.promptBinding;
    this.externalContextSources = structuredClone(snapshot.externalContextSources ?? []);
    this.sessionMode = snapshot.sessionMode;
    this.sessionAgentKey = snapshot.sessionAgentKey;
    this.promptStatus = snapshot.promptStatus;
    this.queuedPrompts = structuredClone(snapshot.queuedMessages ?? []);
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

    this.target = normalizePromptTarget(snapshot.target);
    this.resolvedSystemPrompt = snapshot.resolvedSystemPrompt;
    this.promptBinding = snapshot.promptBinding;
    this.externalContextSources = structuredClone(snapshot.externalContextSources ?? []);
    this.sessionMode = snapshot.sessionMode;
    this.sessionAgentKey = snapshot.sessionAgentKey;
    this.promptStatus = snapshot.promptStatus;
    this.queuedPrompts = structuredClone(snapshot.queuedMessages ?? []);

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
      await this.rpcClient.request.cancelPrompt({
        workspaceId: this.workspaceId,
        target: this.target,
      });
    } catch (error) {
      console.error("Failed to cancel prompt:", error);
    } finally {
      this.agent.abort();
    }
  }

  async sendPrompt(input: string): Promise<void> {
    const text = input.trim();
    if (!text) {
      return;
    }

    if (this.promptDispatchInFlight || this.promptStatus === "streaming") {
      await this.enqueuePrompt(text);
      return;
    }

    await this.dispatchPrompt(text);
  }

  async editQueuedPrompt(promptId: string): Promise<string | null> {
    const response = await this.rpcClient.request.editQueuedSurfaceMessage({
      workspaceId: this.workspaceId,
      target: this.target,
      queuedMessageId: promptId,
    });
    if (response.snapshot) {
      this.applySnapshot(response.snapshot);
    }
    return response.text ?? null;
  }

  async deleteQueuedPrompt(promptId: string): Promise<boolean> {
    const response = await this.rpcClient.request.deleteQueuedSurfaceMessage({
      workspaceId: this.workspaceId,
      target: this.target,
      queuedMessageId: promptId,
    });
    if (response.snapshot) {
      this.applySnapshot(response.snapshot);
    }
    return response.ok;
  }

  async reorderQueuedPrompt(promptId: string, beforePromptId: string | null): Promise<boolean> {
    const response = await this.rpcClient.request.reorderQueuedSurfaceMessage({
      workspaceId: this.workspaceId,
      target: this.target,
      queuedMessageId: promptId,
      beforeQueuedMessageId: beforePromptId,
    });
    if (response.snapshot) {
      this.applySnapshot(response.snapshot);
    }
    return response.ok;
  }

  async steerQueuedPrompt(promptId: string): Promise<boolean> {
    const response = await this.rpcClient.request.steerQueuedSurfaceMessage({
      workspaceId: this.workspaceId,
      target: this.target,
      queuedMessageId: promptId,
    });
    if (response.snapshot) {
      this.applySnapshot(response.snapshot);
    }
    return response.ok;
  }

  private async enqueuePrompt(text: string): Promise<void> {
    const userMessage: Message = {
      role: "user",
      content: [{ type: "text", text }],
      timestamp: Date.now(),
    };
    const provider = this.agent.state.model?.provider ?? DEFAULT_AGENT_SETTINGS.provider;
    const model = this.agent.state.model?.id ?? DEFAULT_AGENT_SETTINGS.model;
    const reasoningEffort =
      (this.agent.state.thinkingLevel as ReasoningEffort | undefined) ??
      DEFAULT_AGENT_SETTINGS.reasoningEffort;
    const response = await this.rpcClient.request.sendPrompt({
      messages: [...(this.agent.state.messages as Message[]), userMessage],
      provider,
      model,
      reasoningEffort,
      target: this.target,
      systemPrompt: this.agent.state.systemPrompt,
      queueOnly: true,
      workspaceId: this.workspaceId,
    });
    this.target = normalizePromptTarget(response.target);
    this.agent.sessionId = response.target.surfacePiSessionId;
    if (response.snapshot) {
      this.applySnapshot(response.snapshot);
    }
  }

  private async dispatchPrompt(text: string): Promise<void> {
    const userMessage: Message = {
      role: "user",
      content: [{ type: "text", text }],
      timestamp: Date.now(),
    };
    const provider = this.agent.state.model?.provider ?? DEFAULT_AGENT_SETTINGS.provider;
    const model = this.agent.state.model?.id ?? DEFAULT_AGENT_SETTINGS.model;
    const reasoningEffort =
      (this.agent.state.thinkingLevel as ReasoningEffort | undefined) ??
      DEFAULT_AGENT_SETTINGS.reasoningEffort;
    const request: SendPromptRequest = {
      messages: [...(this.agent.state.messages as Message[]), userMessage],
      provider,
      model,
      reasoningEffort,
      target: this.target,
      systemPrompt: this.agent.state.systemPrompt,
    };

    this.promptDispatchInFlight = true;
    this.promptStatus = "streaming";
    this.agent.state.isStreaming = true;
    this.agent.state.streamMessage = null;
    this.agent.replaceMessages(
      buildDisplayMessages({ ...this.snapshotFromState(), pendingUserMessage: userMessage }),
    );
    this.emit();

    try {
      const response = await this.rpcClient.request.sendPrompt({
        ...request,
        workspaceId: this.workspaceId,
      });
      this.target = normalizePromptTarget(response.target);
      this.agent.sessionId = response.target.surfacePiSessionId;
    } catch (error) {
      const failure = createFailureMessage(error, provider, model, "error");
      this.agent.state.error = failure.errorMessage;
      this.promptStatus = "idle";
      this.agent.state.isStreaming = false;
      this.agent.state.streamMessage = null;
      throw error;
    } finally {
      this.promptDispatchInFlight = false;
      this.emit();
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
    return async (model) => {
      const stream = createAssistantMessageEventStream();
      Promise.resolve().then(() => {
        const failure = createFailureMessage(
          new Error("Surface prompts are dispatched through the surface controller."),
          model.provider,
          model.id,
          "error",
        );
        this.promptDispatchInFlight = false;
        this.promptStatus = "idle";
        stream.push({
          type: "error",
          reason: "error",
          error: failure,
        });
        this.emit();
      });
      return stream;
    };
  }

  private snapshotFromState(): ConversationSurfaceSnapshot {
    return {
      target: this.target,
      messages: structuredClone(this.agent.state.messages),
      pendingUserMessage: null,
      queuedMessages: structuredClone(this.queuedPrompts),
      streamMessage:
        this.agent.state.streamMessage?.role === "assistant"
          ? structuredClone(this.agent.state.streamMessage)
          : null,
      provider: this.agent.state.model?.provider ?? DEFAULT_AGENT_SETTINGS.provider,
      model: this.agent.state.model?.id ?? DEFAULT_AGENT_SETTINGS.model,
      reasoningEffort:
        (this.agent.state.thinkingLevel as ReasoningEffort | undefined) ??
        DEFAULT_AGENT_SETTINGS.reasoningEffort,
      sessionMode: this.sessionMode,
      sessionAgentKey: this.sessionAgentKey,
      systemPrompt: this.agent.state.systemPrompt,
      resolvedSystemPrompt: this.resolvedSystemPrompt,
      externalContextSources: structuredClone(this.externalContextSources),
      promptBinding: this.promptBinding,
      promptStatus: this.promptStatus,
    };
  }

  private async syncSurfaceModel(providerId: string, modelId: string): Promise<void> {
    try {
      const response = await this.rpcClient.request.setSurfaceModel({
        workspaceId: this.workspaceId,
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
        workspaceId: this.workspaceId,
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
  const workspaceInfo =
    options.workspaceInfo ??
    (options.workspaceId
      ? await rpcClient.request.getWorkspaceInfo({ workspaceId: options.workspaceId })
      : null);
  if (!workspaceInfo) {
    throw new Error("createChatRuntime requires workspaceInfo or workspaceId.");
  }
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
  let activeLayoutId: WorkspaceLayoutSlotId = "A";
  let savedLayouts: Record<WorkspaceLayoutSlotId, WorkspaceDockviewLayoutState | null> = {
    A: null,
    B: null,
    C: null,
  };
  let workspaceBranch = workspaceInfo.branch;
  let disposed = false;

  const emit = () => {
    if (disposed) {
      return;
    }

    for (const listener of listeners) {
      listener();
    }
  };

  const scoped = <T extends object>(request?: T): T & { workspaceId: string } => ({
    ...(request ?? ({} as T)),
    workspaceId: workspaceInfo.workspaceId,
  });

  const currentLayoutSlots = (): WorkspaceLayoutSlotSummary[] =>
    WORKSPACE_LAYOUT_SLOT_IDS.map((id) => {
      const layout = id === activeLayoutId ? paneLayout : savedLayouts[id];
      return {
        id,
        initialized: !!layout && isInitializedPaneLayout(layout),
        active: id === activeLayoutId,
        updatedAt: layout?.updatedAt ?? null,
      };
    });

  const captureActiveLayout = (): void => {
    savedLayouts = {
      ...savedLayouts,
      [activeLayoutId]: structuredClone(paneLayout),
    };
  };

  const persistWorkspaceUiRestore = (): void => {
    if (disposed) {
      return;
    }

    captureActiveLayout();
    const state: WorkspaceUiRestoreState = {
      version: 4,
      activeLayoutId,
      layouts: structuredClone(savedLayouts),
    };

    void rpcClient.request
      .setWorkspaceUiRestore(scoped({ state }))
      .catch((error: unknown) =>
        console.error("Failed to persist workspace UI restore state:", error),
      );
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

    const controller = new SurfaceControllerImpl(snapshot, rpcClient, workspaceInfo.workspaceId);
    surfaceControllers.set(surfacePiSessionId, controller);
    return controller;
  };

  const removePaneForSurface = (panelId: string): void => {
    const target = paneLayout.panels.find((pane) => pane.panelId === panelId)?.binding ?? null;
    if (!target) {
      return;
    }

    paneLayout = closePane(paneLayout, panelId);
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
      await rpcClient.request.closeSurface(scoped({ target }));
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
      recordFocusedSession();
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
    recordFocusedSession();

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
    recordFocusedSession();

    if (
      isPromptTarget(previousTarget) &&
      previousTarget.surfacePiSessionId !== nextTarget.surfacePiSessionId
    ) {
      surfaceControllers.get(previousTarget.surfacePiSessionId)?.detachPane(panelId);
    }
    persistWorkspaceUiRestore();
  };

  const refreshSessions = async (): Promise<WorkspaceSessionSummary[]> => {
    const response = await rpcClient.request.listSessions(scoped());
    sessions = response.sessions;
    sessionNavigation = response.navigation;
    emit();
    return sessions;
  };

  let lastRecordedFocusedSessionId: string | null | undefined = undefined;
  let lastRecordedFocusedSurfacePiSessionId: string | null | undefined = undefined;
  const recordFocusedSession = (): void => {
    const focusedTarget =
      paneLayout.panels.find((pane) => pane.panelId === paneLayout.focusedPanelId)?.binding ?? null;
    const focusedSessionId = isPromptTarget(focusedTarget)
      ? focusedTarget.workspaceSessionId
      : null;
    const focusedSurfacePiSessionId = isPromptTarget(focusedTarget)
      ? focusedTarget.surfacePiSessionId
      : null;
    if (
      focusedSessionId === lastRecordedFocusedSessionId &&
      focusedSurfacePiSessionId === lastRecordedFocusedSurfacePiSessionId
    ) {
      return;
    }

    lastRecordedFocusedSessionId = focusedSessionId;
    lastRecordedFocusedSurfacePiSessionId = focusedSurfacePiSessionId;
    void rpcClient.request
      .recordFocusedSession({
        workspaceId: workspaceInfo.workspaceId,
        sessionId: focusedSessionId,
        surfacePiSessionId: focusedSurfacePiSessionId,
      })
      .catch((error) => {
        console.error("Failed to record focused session:", error);
      });
  };

  const getSelectedSessionId = (sessionId?: string): string | undefined => {
    if (sessionId) {
      return sessionId;
    }

    const focusedTarget =
      paneLayout.panels.find((pane) => pane.panelId === paneLayout.focusedPanelId)?.binding ?? null;
    return focusedTarget && "workspaceSessionId" in focusedTarget
      ? focusedTarget.workspaceSessionId
      : undefined;
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

    const inspector = await rpcClient.request.getCommandInspector(
      scoped({
        sessionId,
        commandId,
      }),
    );
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

    return await rpcClient.request.listHandlerThreads(scoped({ sessionId }));
  };

  const getHandlerThreadInspector = async (
    threadId: string,
    sessionId = getSelectedSessionId(),
  ): Promise<WorkspaceHandlerThreadInspector> => {
    if (!sessionId) {
      throw new Error("Expected a workspace session before inspecting a handler thread.");
    }

    const inspector = await rpcClient.request.getHandlerThreadInspector(
      scoped({
        sessionId,
        threadId,
      }),
    );
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

    const inspector = await rpcClient.request.getWorkflowTaskAttemptInspector(
      scoped({
        sessionId,
        workflowTaskAttemptId,
      }),
    );
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

    const inspector = await rpcClient.request.getWorkflowInspector(
      scoped({
        sessionId,
        workflowRunId,
        selectedNodeKey: request.selectedNodeKey,
        expandedNodeKeys: request.expandedNodeKeys,
        userCollapsedNodeKeys: request.userCollapsedNodeKeys,
        searchQuery: request.searchQuery,
        mode: request.mode,
      }),
    );
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
    const update = await rpcClient.request.streamWorkflowInspector(
      scoped({
        sessionId,
        workflowRunId,
        selectedNodeKey: request.selectedNodeKey,
        expandedNodeKeys: request.expandedNodeKeys,
        userCollapsedNodeKeys: request.userCollapsedNodeKeys,
        searchQuery: request.searchQuery,
        mode: request.mode,
        fromSeq: request.fromSeq,
      }),
    );
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

    return await rpcClient.request.getProjectCiStatus(scoped({ sessionId }));
  };

  const getArtifactPreview = async (
    artifactId: string,
    sessionId = getSelectedSessionId(),
  ): Promise<WorkspaceArtifactPreview> => {
    if (!sessionId) {
      throw new Error("Expected a workspace session before opening an artifact.");
    }

    return await rpcClient.request.getArtifactPreview(scoped({ sessionId, artifactId }));
  };

  const getFallbackPanelId = (): string | null =>
    paneLayout.focusedPanelId &&
    paneLayout.panels.some((pane) => pane.panelId === paneLayout.focusedPanelId)
      ? paneLayout.focusedPanelId
      : (paneLayout.panels[0]?.panelId ?? null);

  const addBoundPanel = (
    binding: WorkspacePaneSurfaceTarget,
    panelId = paneLayout.panels.length === 0 ? PRIMARY_CHAT_PANE_ID : createPanelId(),
    placement: DockviewPanelPlacementState | null = null,
  ): string => {
    paneLayout = addDockviewPanel(paneLayout, binding, panelId, placement);
    persistWorkspaceUiRestore();
    emit();
    return panelId;
  };

  const resolveOpenTarget = (
    binding: WorkspacePaneSurfaceTarget,
    openTarget?: PaneOpenTarget | string,
  ): string => {
    if (typeof openTarget === "string") {
      if (!paneLayout.panels.some((pane) => pane.panelId === openTarget)) {
        return addBoundPanel(binding, openTarget);
      }
      return openTarget;
    }
    if (!openTarget || openTarget.kind === "focused-panel") {
      return getFallbackPanelId() ?? addBoundPanel(binding);
    }
    if (openTarget.kind === "panel") {
      if (!paneLayout.panels.some((pane) => pane.panelId === openTarget.panelId)) {
        return addBoundPanel(binding, openTarget.panelId);
      }
      return openTarget.panelId;
    }
    if (openTarget.kind === "split") {
      const referencePanelId = paneLayout.panels.some((pane) => pane.panelId === openTarget.panelId)
        ? openTarget.panelId
        : getFallbackPanelId();
      return referencePanelId
        ? addBoundPanel(binding, createPanelId(), {
            referencePanelId,
            direction: openTarget.direction,
            size: openTarget.size,
          })
        : addBoundPanel(binding);
    }
    if (openTarget.kind === "tab") {
      return addBoundPanel(binding);
    }
    const basePaneId = getFallbackPanelId();
    if (!basePaneId) {
      return addBoundPanel(binding);
    }
    const before = new Set(paneLayout.panels.map((pane) => pane.panelId));
    const direction =
      openTarget.kind === "new-panel"
        ? openTarget.direction
        : openTarget.kind === "edge"
          ? openTarget.direction
          : "right";
    const size = "size" in openTarget ? openTarget.size : undefined;
    const nextPanelId = addBoundPanel(binding, createPanelId(), {
      referencePanelId: basePaneId,
      direction,
      size,
    });
    return paneLayout.panels.find((pane) => !before.has(pane.panelId))?.panelId ?? nextPanelId;
  };

  const [defaults, initialCatalog, initialAppLogSummary] = await Promise.all([
    rpcClient.request.getDefaults(),
    rpcClient.request.listSessions({ workspaceId: workspaceInfo.workspaceId }),
    rpcClient.request.getAppLogSummary({ workspaceId: workspaceInfo.workspaceId }),
  ]);
  sessions = initialCatalog.sessions;
  sessionNavigation = initialCatalog.navigation;
  appLogSummary = initialAppLogSummary;

  const syncProviderAuthPromise = syncProviderAuth(defaults.provider);
  await syncProviderAuthPromise;

  const restoreState = (await rpcClient.request
    .getWorkspaceUiRestore(scoped())
    .catch((error: unknown) => {
      console.error("Failed to load workspace UI restore state:", error);
      return null;
    })) as WorkspaceUiRestoreState | null;
  if (restoreState) {
    activeLayoutId = restoreState.activeLayoutId;
    savedLayouts = {
      A: restoreState.layouts.A ? normalizePaneLayout(restoreState.layouts.A) : null,
      B: restoreState.layouts.B ? normalizePaneLayout(restoreState.layouts.B) : null,
      C: restoreState.layouts.C ? normalizePaneLayout(restoreState.layouts.C) : null,
    };
  }
  const activeRestoreLayout = restoreState?.layouts[activeLayoutId] ?? null;
  let restoredPaneIds: string[] = [];
  if (activeRestoreLayout?.panels.length) {
    const sessionIds = new Set(initialCatalog.sessions.map((session) => session.id));
    paneLayout = normalizePaneLayout(activeRestoreLayout);
    const hasOnlyRestorablePanes = paneLayout.panels.every(
      (paneState) =>
        !paneState.binding ||
        paneState.binding.surface === "app-logs" ||
        paneState.binding.surface === "prompt-library" ||
        paneState.binding.surface === "saved-workflow-library" ||
        sessionIds.has(paneState.binding.workspaceSessionId),
    );
    if (!hasOnlyRestorablePanes) {
      paneLayout = createEmptyPaneLayout();
    }
    for (const paneState of paneLayout.panels) {
      if (
        !paneState.binding ||
        (paneState.binding.surface !== "app-logs" &&
          paneState.binding.surface !== "prompt-library" &&
          paneState.binding.surface !== "saved-workflow-library" &&
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
            ? await rpcClient.request.openSession(scoped({ sessionId: target.workspaceSessionId }))
            : await rpcClient.request.openSurface(scoped({ target }));
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

  if (
    activeRestoreLayout?.panels.length &&
    paneLayout.panels.some((paneState) => !paneState.binding)
  ) {
    const focusedPanelId =
      activeRestoreLayout.focusedPanelId &&
      paneLayout.panels.some((pane) => pane.panelId === activeRestoreLayout.focusedPanelId)
        ? activeRestoreLayout.focusedPanelId
        : (paneLayout.panels[0]?.panelId ?? PRIMARY_CHAT_PANE_ID);
    paneLayout = { ...paneLayout, focusedPanelId };
    if (restoredPaneIds.length === 0 && initialCatalog.sessions.length > 0) {
      const [initialSession] = initialCatalog.sessions;
      const snapshot = await rpcClient.request.openSession(
        scoped({ sessionId: initialSession!.id }),
      );
      await bindPaneToSnapshot(focusedPanelId, snapshot, { focus: true, persist: false });
    } else if (restoredPaneIds.length === 0) {
      const snapshot = await rpcClient.request.createSession(scoped({}));
      await bindPaneToSnapshot(focusedPanelId, snapshot, { focus: true, persist: false });
      await refreshSessions();
    }
    persistWorkspaceUiRestore();
    emit();
  } else if (restoredPaneIds.length > 0) {
    paneLayout = {
      ...paneLayout,
      focusedPanelId:
        activeRestoreLayout?.focusedPanelId &&
        restoredPaneIds.includes(activeRestoreLayout.focusedPanelId)
          ? activeRestoreLayout.focusedPanelId
          : restoredPaneIds[0]!,
    };
    persistWorkspaceUiRestore();
    emit();
  } else if (initialCatalog.sessions.length > 0) {
    const [initialSession] = initialCatalog.sessions;
    if (!initialSession) {
      throw new Error("Expected an initial session to open.");
    }
    const snapshot = await rpcClient.request.openSession(scoped({ sessionId: initialSession.id }));
    const panelId = resolveOpenTarget(normalizePromptTarget(snapshot.target), PRIMARY_CHAT_PANE_ID);
    await bindPaneToSnapshot(panelId, snapshot);
  } else {
    const snapshot = await rpcClient.request.createSession(scoped({}));
    const panelId = resolveOpenTarget(normalizePromptTarget(snapshot.target), PRIMARY_CHAT_PANE_ID);
    await bindPaneToSnapshot(panelId, snapshot);
    await refreshSessions();
  }

  const workspaceSyncListener = (payload: WorkspaceSyncMessage) => {
    if (payload.workspaceId !== workspaceInfo.workspaceId) {
      return;
    }
    sessions = payload.sessions;
    sessionNavigation = payload.navigation;
    emit();
  };

  const surfaceSyncListener = (payload: SurfaceSyncMessage) => {
    if (payload.workspaceId !== workspaceInfo.workspaceId) {
      return;
    }
    syncPaneTargetForSurface(payload.target);
    persistWorkspaceUiRestore();
    if (payload.reason === "surface.closed") {
      for (const pane of paneLayout.panels) {
        if (
          isPromptTarget(pane.binding) &&
          pane.binding.surfacePiSessionId === payload.target.surfacePiSessionId
        ) {
          removePaneForSurface(pane.panelId);
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
  recordFocusedSession();

  const runtime: ChatRuntime = {
    storage,
    workspaceId: workspaceInfo.workspaceId,
    workspaceLabel: workspaceInfo.workspaceLabel,
    cwd: workspaceInfo.cwd,
    get branch() {
      return workspaceBranch;
    },
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
    get activeLayoutId() {
      return activeLayoutId;
    },
    get layoutSlots() {
      return currentLayoutSlots();
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
      recordFocusedSession();
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
      recordFocusedSession();
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
      recordFocusedSession();
    },
    switchWorkspaceLayout: async (layoutId) => {
      if (layoutId === activeLayoutId) {
        return;
      }
      captureActiveLayout();
      activeLayoutId = layoutId;
      paneLayout = savedLayouts[layoutId]
        ? normalizePaneLayout(savedLayouts[layoutId]!)
        : createEmptyPaneLayout();
      savedLayouts = {
        ...savedLayouts,
        [layoutId]: structuredClone(paneLayout),
      };
      persistWorkspaceUiRestore();
      emit();
      recordFocusedSession();
    },
    getCommandInspector,
    listHandlerThreads,
    getHandlerThreadInspector,
    getWorkflowTaskAttemptInspector,
    getWorkflowInspector,
    streamWorkflowInspector,
    getProjectCiStatus,
    getArtifactPreview,
    getAppLogs: (query) => rpcClient.request.getAppLogs(scoped(query ?? {})),
    getAppLogSummary: async () => {
      appLogSummary = await rpcClient.request.getAppLogSummary(scoped());
      emit();
      return structuredClone(appLogSummary);
    },
    markAppLogsSeen: async (throughSeq) => {
      if (throughSeq <= appLogSummary.seenSeq) {
        return structuredClone(appLogSummary);
      }
      appLogSummary = await rpcClient.request.markAppLogsSeen(scoped({ throughSeq }));
      emit();
      return structuredClone(appLogSummary);
    },
    writeClipboardText: async (text) => {
      await rpcClient.request.writeClipboardText({ text });
    },
    createSession: async (request = {}, openTarget) => {
      const snapshot = await rpcClient.request.createSession(scoped(request));
      const nextPaneId = resolveOpenTarget(normalizePromptTarget(snapshot.target), openTarget);
      await bindPaneToSnapshot(nextPaneId, snapshot);
      await refreshSessions();
    },
    openSession: async (sessionId, openTarget) => {
      const existingController = surfaceControllers.get(sessionId);
      const target = existingController?.target ?? {
        workspaceSessionId: sessionId,
        surface: "orchestrator" as const,
        surfacePiSessionId: sessionId,
      };
      const nextPaneId = resolveOpenTarget(normalizePromptTarget(target), openTarget);
      const currentTarget =
        paneLayout.panels.find((pane) => pane.panelId === nextPaneId)?.binding ?? null;
      if (
        existingController &&
        existingController.ownerPaneIds.length > 0 &&
        currentTarget &&
        "workspaceSessionId" in currentTarget &&
        currentTarget?.workspaceSessionId === sessionId &&
        currentTarget.surface === "orchestrator" &&
        currentTarget.surfacePiSessionId === sessionId
      ) {
        paneLayout = focusPane(paneLayout, nextPaneId);
        persistWorkspaceUiRestore();
        emit();
        recordFocusedSession();
        return;
      }

      if (existingController) {
        if (existingController.ownerPaneIds.length === 0) {
          const snapshot = await rpcClient.request.openSession(scoped({ sessionId }));
          await bindPaneToSnapshot(nextPaneId, snapshot);
          return;
        }
        bindPaneToExistingController(nextPaneId, existingController);
        void rpcClient.request.recordSessionOpened(scoped({ sessionId }));
        return;
      }

      const snapshot = await rpcClient.request.openSession(scoped({ sessionId }));
      await bindPaneToSnapshot(nextPaneId, snapshot);
    },
    openSurface: async (target, openTarget) => {
      if (
        target.surface === "workflow-inspector" ||
        target.surface === "command" ||
        target.surface === "workflow-task-attempt" ||
        target.surface === "artifact" ||
        target.surface === "project-ci-check" ||
        target.surface === "saved-workflow-library" ||
        target.surface === "prompt-library" ||
        target.surface === "app-logs"
      ) {
        const nextPaneId = resolveOpenTarget({ ...target }, openTarget);
        const previousTarget =
          paneLayout.panels.find((pane) => pane.panelId === nextPaneId)?.binding ?? null;
        if (isPromptTarget(previousTarget)) {
          surfaceControllers.get(previousTarget.surfacePiSessionId)?.detachPane(nextPaneId);
        }
        paneLayout = bindPane(paneLayout, nextPaneId, { ...target });
        persistWorkspaceUiRestore();
        emit();
        recordFocusedSession();
        return;
      }
      const normalizedTarget = normalizePromptTarget(target);
      const nextPaneId = resolveOpenTarget(normalizedTarget, openTarget);
      const currentTarget =
        paneLayout.panels.find((pane) => pane.panelId === nextPaneId)?.binding ?? null;
      const existingController = surfaceControllers.get(normalizedTarget.surfacePiSessionId);
      if (
        existingController &&
        existingController.ownerPaneIds.length > 0 &&
        isPromptTarget(currentTarget) &&
        currentTarget.surfacePiSessionId === normalizedTarget.surfacePiSessionId
      ) {
        paneLayout = bindPane(paneLayout, nextPaneId, normalizedTarget);
        existingController.attachPane(nextPaneId);
        persistWorkspaceUiRestore();
        emit();
        recordFocusedSession();
        return;
      }

      if (existingController) {
        if (existingController.ownerPaneIds.length === 0) {
          const snapshot = await rpcClient.request.openSurface(
            scoped({ target: normalizedTarget }),
          );
          await bindPaneToSnapshot(nextPaneId, snapshot);
          return;
        }
        bindPaneToExistingController(nextPaneId, existingController);
        return;
      }

      const snapshot = await rpcClient.request.openSurface(scoped({ target: normalizedTarget }));
      await bindPaneToSnapshot(nextPaneId, snapshot);
    },
    closePaneSurface: async (panelId) => {
      const target = paneLayout.panels.find((pane) => pane.panelId === panelId)?.binding ?? null;
      if (!target) {
        return;
      }

      removePaneForSurface(panelId);
      emit();
      recordFocusedSession();
      await releasePaneSurface(panelId, target);
    },
    renameSession: async (sessionId, title) => {
      await rpcClient.request.renameSession(scoped({ sessionId, title }));
      await refreshSessions();
    },
    setSessionMode: async (panelId, mode) => {
      const target = paneLayout.panels.find((pane) => pane.panelId === panelId)?.binding ?? null;
      if (!isPromptTarget(target) || target.surface !== "orchestrator") {
        return;
      }
      const response = await rpcClient.request.setSessionMode(scoped({ target, mode }));
      if (!response.ok || !response.snapshot) {
        throw new Error(response.error ?? "Session mode update failed.");
      }
      await bindPaneToSnapshot(panelId, response.snapshot);
      await refreshSessions();
    },
    forkSession: async (sessionId, title, openTarget, forkOptions) => {
      const snapshot = await rpcClient.request.forkSession(
        scoped({
          sessionId,
          title,
          messageTimestamp: forkOptions?.messageTimestamp,
        }),
      );
      const nextPaneId = resolveOpenTarget(normalizePromptTarget(snapshot.target), openTarget);
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
        if (
          pane.binding &&
          "workspaceSessionId" in pane.binding &&
          pane.binding.workspaceSessionId === sessionId
        ) {
          affectedPaneIds.add(pane.panelId);
        }
      }

      await rpcClient.request.deleteSession(scoped({ sessionId }));

      for (const candidatePaneId of affectedPaneIds) {
        removePaneForSurface(candidatePaneId);
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
      await rpcClient.request.pinSession(scoped({ sessionId }));
      await refreshSessions();
    },
    unpinSession: async (sessionId) => {
      await rpcClient.request.unpinSession(scoped({ sessionId }));
      await refreshSessions();
    },
    archiveSession: async (sessionId) => {
      await rpcClient.request.archiveSession(scoped({ sessionId }));
      await refreshSessions();
    },
    unarchiveSession: async (sessionId) => {
      await rpcClient.request.unarchiveSession(scoped({ sessionId }));
      await refreshSessions();
    },
    markSessionUnread: async (sessionId) => {
      await rpcClient.request.markSessionUnread(scoped({ sessionId }));
      lastRecordedFocusedSessionId = undefined;
      lastRecordedFocusedSurfacePiSessionId = undefined;
      await refreshSessions();
    },
    markSessionRead: async (sessionId) => {
      await rpcClient.request.markSessionRead(scoped({ sessionId }));
      lastRecordedFocusedSessionId = undefined;
      lastRecordedFocusedSurfacePiSessionId = undefined;
      await refreshSessions();
    },
    setArchivedGroupCollapsed: async (collapsed) => {
      await rpcClient.request.setArchivedGroupCollapsed(scoped({ collapsed }));
      await refreshSessions();
    },
    setSessionNavigationSectionState: async (section, state) => {
      await rpcClient.request.setSessionNavigationSectionState(scoped({ section, ...state }));
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
      await rpcClient.request.sendPrompt(
        scoped({
          messages: [{ role: "user", content: text } as Message],
          target: normalizePromptTarget(target),
        }),
      );
    },
    syncProviderAuth,
    requireProviderAccess,
    listConfiguredProviders,
    listOpenWorkspaces: () => rpcClient.request.getOpenWorkspaces(),
    listWorkspaceBranches: async () => {
      const result = await rpcClient.request.listWorkspaceBranches(scoped());
      return result.branches;
    },
    switchWorkspaceBranch: async (branch) => {
      const result = await rpcClient.request.switchWorkspaceBranch(scoped({ branch }));
      if (!result.ok) {
        throw new Error(result.error ?? "Unable to switch branch.");
      }
      workspaceBranch = result.workspace.branch;
      emit();
    },
    listWorkspacePaths: (pathOptions) =>
      rpcClient.request.listWorkspacePaths(scoped(pathOptions ?? {})),
    pickWorkspaceAttachments: async () => {
      const result = await rpcClient.request.pickWorkspaceAttachments(scoped());
      return result.entries;
    },
    openWorkspacePath: async (workspaceRelativePath) => {
      const result = await rpcClient.request.openWorkspacePath(scoped({ workspaceRelativePath }));
      return result.opened;
    },
    getSavedWorkflowLibrary: () => rpcClient.request.getSavedWorkflowLibrary(scoped()),
    deleteSavedWorkflowLibraryItem: (path) =>
      rpcClient.request.deleteSavedWorkflowLibraryItem(scoped({ path })),
    openWorkspaceSourceInEditor: async (path) => {
      const result = await rpcClient.request.openWorkspaceSourceInEditor(scoped({ path }));
      return result.opened;
    },
    openPromptLibraryExternalSourceInEditor: async (path) => {
      const result = await rpcClient.request.openPromptLibraryExternalSourceInEditor(
        scoped({ path }),
      );
      return result.opened;
    },
    getPromptLibrary: () => rpcClient.request.getPromptLibrary(scoped()),
    getPromptLibraryDefaults: () => rpcClient.request.getPromptLibraryDefaults(scoped()),
    getPromptLibraryGeneratedEntries: () =>
      rpcClient.request.getPromptLibraryGeneratedEntries(scoped()),
    getPromptLibraryExternalSources: () =>
      rpcClient.request.getPromptLibraryExternalSources(scoped()),
    updatePromptLibrary: (request) => rpcClient.request.updatePromptLibrary(scoped(request)),
    resetPromptLibrary: () => rpcClient.request.resetPromptLibrary(scoped()),
    listPromptLibrarySnapshots: () => rpcClient.request.listPromptLibrarySnapshots(scoped()),
    createPromptLibrarySnapshot: (name) =>
      rpcClient.request.createPromptLibrarySnapshot(scoped({ name })),
    renamePromptLibrarySnapshot: (snapshotId, name) =>
      rpcClient.request.renamePromptLibrarySnapshot(scoped({ snapshotId, name })),
    restorePromptLibrarySnapshot: (snapshotId) =>
      rpcClient.request.restorePromptLibrarySnapshot(scoped({ snapshotId })),
  };

  return runtime;
}
