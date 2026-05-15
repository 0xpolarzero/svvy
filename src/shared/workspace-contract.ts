import type { AgentMessage } from "@mariozechner/pi-agent-core";
import type { AssistantMessage, Message } from "@mariozechner/pi-ai";
import type {
  AgentDefaults,
  AgentSettingsState,
  AppPreferences,
  ReasoningEffort,
  SessionAgentKey,
  SessionAgentSettings,
  SessionMode,
  WorkflowAgentKey,
  WorkflowAgentSettings,
} from "./agent-settings";
import type { AppMenuAction } from "./shortcut-registry";

export type AuthKeyType = "apikey" | "oauth" | "env" | "none";
export type PromptSurfaceKind = "orchestrator" | "thread";

export type AppLogLevel = "info" | "warning" | "error";

export type AppLogSource =
  | "app.lifecycle"
  | "app.bridge"
  | "app.rpc"
  | "auth.provider"
  | "settings"
  | "workspace"
  | "session"
  | "session.title"
  | "surface"
  | "prompt"
  | "thread"
  | "smithers"
  | "workflow.library"
  | "workflow.run"
  | "workflow.task"
  | "project-ci"
  | "direct-tool"
  | "execute-typescript"
  | "artifact"
  | "external-editor"
  | "renderer";

export interface AppLogEntry {
  id: string;
  seq: number;
  createdAt: string;
  level: AppLogLevel;
  source: AppLogSource;
  message: string;
  details?: Record<string, unknown>;
  error?: {
    name?: string;
    message: string;
    stack?: string;
  };
  workspaceSessionId?: string;
  surfacePiSessionId?: string;
  threadId?: string;
  workflowRunId?: string;
  workflowTaskAttemptId?: string;
  commandId?: string;
}

export interface AppLogSummary {
  latestSeq: number;
  seenSeq: number;
  unread: {
    total: number;
    info: number;
    warning: number;
    error: number;
  };
  totals: {
    total: number;
    info: number;
    warning: number;
    error: number;
  };
}

export interface AppLogQuery {
  levels?: AppLogLevel[];
  sources?: AppLogSource[];
  query?: string;
  afterSeq?: number;
  beforeSeq?: number;
  limit?: number;
}

export interface AppLogReadModel {
  entries: AppLogEntry[];
  summary: AppLogSummary;
}

export interface AppLogUpdateMessage {
  entries: AppLogEntry[];
  summary: AppLogSummary;
}

export interface PromptTarget {
  workspaceSessionId: string;
  surface: PromptSurfaceKind;
  surfacePiSessionId: string;
  threadId?: string;
}

export interface WorkflowInspectorPaneTarget {
  workspaceSessionId: string;
  surface: "workflow-inspector";
  workflowRunId: string;
}

export interface SavedWorkflowLibraryPaneTarget {
  workspaceSessionId: string;
  surface: "saved-workflow-library";
}

export interface AppLogsPaneTarget {
  workspaceSessionId?: string;
  surface: "app-logs";
}

export type StaticInspectorPaneTarget =
  | { workspaceSessionId: string; surface: "command"; commandId: string }
  | {
      workspaceSessionId: string;
      surface: "workflow-task-attempt";
      workflowTaskAttemptId: string;
    }
  | { workspaceSessionId: string; surface: "artifact"; artifactId: string }
  | { workspaceSessionId: string; surface: "project-ci-check"; checkResultId: string };

export type WorkspacePaneSurfaceTarget =
  | PromptTarget
  | WorkflowInspectorPaneTarget
  | SavedWorkflowLibraryPaneTarget
  | AppLogsPaneTarget
  | StaticInspectorPaneTarget;

export interface SendPromptRequest {
  messages: Message[];
  provider?: string;
  model?: string;
  reasoningEffort?: ReasoningEffort;
  target: PromptTarget;
  systemPrompt?: string;
}

export interface SendPromptResponse {
  target: PromptTarget;
}

export interface CloseSurfaceRequest {
  target: PromptTarget;
}

export interface SetSurfaceModelRequest {
  target: PromptTarget;
  model: string;
  provider: string;
}

export interface SetSurfaceThoughtLevelRequest {
  target: PromptTarget;
  level: ReasoningEffort;
}

export interface SetSessionModeRequest {
  target: PromptTarget;
  mode: SessionMode;
}

export interface SetSessionModeResponse {
  ok: boolean;
  snapshot?: ConversationSurfaceSnapshot;
  error?: string;
}

export interface WorkspaceSyncMessage {
  workspaceId: string;
  reason: "workspace.updated" | "structured.updated";
  sessions: WorkspaceSessionSummary[];
  navigation: WorkspaceSessionNavigationReadModel;
}

export interface CancelPromptRequest {
  target: PromptTarget;
}

export interface ProviderAuthStateRequest {
  providerId?: string;
}

export interface AuthStateResponse {
  connected: boolean;
  accountId?: string;
  message?: string;
}

export interface WorkspaceInfoResponse {
  workspaceId: string;
  cwd: string;
  workspaceLabel: string;
  branch?: string;
}

export interface WorkspaceBranchInfo {
  name: string;
  current: boolean;
}

export interface WorkspaceBranchListResponse {
  branches: WorkspaceBranchInfo[];
  currentBranch?: string;
}

export interface SwitchWorkspaceBranchRequest {
  branch: string;
}

export interface SwitchWorkspaceBranchResponse {
  ok: boolean;
  workspace: WorkspaceInfoResponse;
  error?: string;
}

export interface WorkspaceTabInfo extends WorkspaceInfoResponse {
  openedAt: string;
}

export interface OpenWorkspaceRequest {
  cwd?: string;
}

export interface OpenWorkspaceResponse {
  workspace: WorkspaceInfoResponse | null;
}

export interface WorkspaceScopedRequest {
  workspaceId: string;
}

export type WorkspaceScoped<T extends object = Record<string, never>> = T & WorkspaceScopedRequest;

export type ComposerMentionKind = "file" | "folder";

export interface WorkspacePathIndexEntry {
  kind: ComposerMentionKind;
  workspaceRelativePath: string;
}

export interface OpenWorkspacePathRequest {
  workspaceRelativePath: string;
}

export interface OpenWorkspacePathResponse {
  opened: boolean;
  kind: ComposerMentionKind | "missing";
}

export interface PickWorkspaceAttachmentResponse {
  entries: WorkspacePathIndexEntry[];
  skippedPaths: string[];
}

export interface ProviderAuthInfo {
  provider: string;
  hasKey: boolean;
  keyType: AuthKeyType;
  supportsOAuth: boolean;
}

export type SessionStatus = "idle" | "running" | "waiting" | "error";
export type SessionTitleGenerationStatus =
  | "not-started"
  | "pending"
  | "running"
  | "completed"
  | "failed"
  | "cancelled";

export interface WorkspaceCommandRollupChild {
  commandId: string;
  toolName: string;
  status: "requested" | "running" | "waiting" | "succeeded" | "failed" | "cancelled";
  title: string;
  summary: string;
  error: string | null;
}

export interface WorkspaceCommandRollup {
  commandId: string;
  threadId: string | null;
  workflowRunId?: string | null;
  workflowTaskAttemptId?: string | null;
  toolName: string;
  visibility: "summary" | "surface";
  status: "requested" | "running" | "waiting" | "succeeded" | "failed" | "cancelled";
  title: string;
  summary: string;
  childCount: number;
  summaryChildCount: number;
  traceChildCount: number;
  summaryChildren: WorkspaceCommandRollupChild[];
  updatedAt: string;
}

export interface WorkspaceCommandArtifactLink {
  artifactId: string;
  kind: "text" | "log" | "json" | "file";
  name: string;
  path?: string;
  createdAt: string;
  sourceCommandId?: string;
  workflowRunId?: string;
  workflowName?: string;
  producerLabel?: string;
  missingFile?: boolean;
}

export interface WorkspaceCommandInspectorChild extends WorkspaceCommandRollupChild {
  visibility: "trace" | "summary" | "surface";
  facts: Record<string, unknown> | null;
  startedAt: string;
  updatedAt: string;
  finishedAt: string | null;
  artifacts: WorkspaceCommandArtifactLink[];
}

export interface WorkspaceCommandInspector {
  commandId: string;
  threadId: string | null;
  workflowRunId?: string | null;
  workflowTaskAttemptId?: string | null;
  toolName: string;
  visibility: "trace" | "summary" | "surface";
  status: "requested" | "running" | "waiting" | "succeeded" | "failed" | "cancelled";
  title: string;
  summary: string;
  facts: Record<string, unknown> | null;
  error: string | null;
  startedAt: string;
  updatedAt: string;
  finishedAt: string | null;
  artifacts: WorkspaceCommandArtifactLink[];
  childCount: number;
  summaryChildCount: number;
  traceChildCount: number;
  summaryChildren: WorkspaceCommandInspectorChild[];
  traceChildren: WorkspaceCommandInspectorChild[];
}

export interface WorkspaceHandlerThreadWorkflowSummary {
  workflowRunId: string;
  workflowName: string;
  status: "running" | "waiting" | "continued" | "completed" | "failed" | "cancelled";
  summary: string;
  updatedAt: string;
  artifacts: WorkspaceCommandArtifactLink[];
}

export interface WorkspaceHandlerThreadEpisodeSummary {
  episodeId: string;
  kind: "analysis" | "change" | "workflow" | "clarification";
  title: string;
  summary: string;
  createdAt: string;
}

export interface WorkspaceProjectCiRunSummary {
  ciRunId: string;
  workflowRunId: string;
  workflowId: string;
  status: "passed" | "failed" | "blocked" | "cancelled";
  summary: string;
  updatedAt: string;
}

export type WorkspaceProjectCiPanelStatus =
  | "not-configured"
  | "configured"
  | "running"
  | WorkspaceProjectCiRunSummary["status"];

export type WorkspaceProjectCiCheckStatus = WorkspaceProjectCiRunSummary["status"] | "skipped";

export interface WorkspaceProjectCiEntrySummary {
  workflowId: string;
  label: string;
  summary: string;
  sourceScope: "saved" | "artifact";
  entryPath: string;
}

export interface WorkspaceProjectCiActiveWorkflowSummary {
  workflowRunId: string;
  workflowId: string;
  entryPath: string | null;
  threadId: string;
  threadTitle: string;
  status: "running" | "waiting";
  summary: string;
  updatedAt: string;
}

export interface WorkspaceProjectCiCheckSummary {
  checkResultId: string;
  checkId: string;
  label: string;
  kind: string;
  status: WorkspaceProjectCiCheckStatus;
  required: boolean;
  command: string[] | null;
  exitCode: number | null;
  summary: string;
  artifactIds: string[];
  artifacts: WorkspaceCommandArtifactLink[];
  startedAt: string | null;
  finishedAt: string | null;
  updatedAt: string;
}

export interface WorkspaceProjectCiRunDetail extends WorkspaceProjectCiRunSummary {
  threadId: string;
  threadTitle: string;
  smithersRunId: string;
  entryPath: string;
  startedAt: string;
  finishedAt: string;
}

export interface WorkspaceProjectCiStatusPanel {
  status: WorkspaceProjectCiPanelStatus;
  summary: string;
  entries: WorkspaceProjectCiEntrySummary[];
  activeWorkflowRun: WorkspaceProjectCiActiveWorkflowSummary | null;
  latestRun: WorkspaceProjectCiRunDetail | null;
  checks: WorkspaceProjectCiCheckSummary[];
  checkCounts: Record<WorkspaceProjectCiCheckStatus, number> & {
    total: number;
  };
  updatedAt: string | null;
}

export type WorkspaceSavedWorkflowLibraryItemKind =
  | "definition"
  | "prompt"
  | "component"
  | "entry"
  | "artifact-workflow";

export type WorkspaceSavedWorkflowLibraryItemScope = "saved" | "artifact";

export interface WorkspaceSavedWorkflowLibraryDiagnostic {
  severity: "error" | "warning";
  message: string;
  path?: string;
  line?: number;
  column?: number;
  code?: string;
}

export interface WorkspaceSavedWorkflowLibraryItem {
  id: string;
  kind: WorkspaceSavedWorkflowLibraryItemKind;
  scope: WorkspaceSavedWorkflowLibraryItemScope;
  title: string;
  summary: string;
  path: string;
  sourcePath: string | null;
  sourcePreview: string | null;
  validationStatus: "valid" | "warning" | "error" | "unknown";
  diagnostics: WorkspaceSavedWorkflowLibraryDiagnostic[];
  workflowId?: string;
  label?: string;
  productKind?: string;
  launchSchema?: string;
  resultSchema?: string;
  groupedAssetRefs?: {
    definitions: string[];
    prompts: string[];
    components: string[];
  };
  assetPaths?: string[];
  artifactWorkflowId?: string;
  entryCount?: number;
  assetCount?: number;
}

export interface WorkspaceSavedWorkflowLibraryReadModel {
  rootPath: string;
  artifactRootPath: string;
  items: WorkspaceSavedWorkflowLibraryItem[];
  counts: Record<WorkspaceSavedWorkflowLibraryItemKind, number>;
  diagnostics: WorkspaceSavedWorkflowLibraryDiagnostic[];
  preferredExternalEditor: AppPreferences["preferredExternalEditor"];
  customExternalEditorCommand: string;
  updatedAt: string;
}

export interface DeleteSavedWorkflowLibraryItemRequest {
  path: string;
}

export interface OpenWorkflowSourceInEditorRequest {
  path: string;
}

export interface OpenWorkflowSourceInEditorResponse {
  opened: boolean;
  editor: string;
  path: string;
}

export interface WorkspaceWorkflowTaskAttemptTranscriptMessage {
  messageId: string;
  role: "user" | "assistant" | "stderr";
  source: "prompt" | "event" | "responseText";
  text: string;
  createdAt: string;
}

export interface WorkspaceWorkflowTaskAttemptSummary {
  workflowTaskAttemptId: string;
  workflowRunId: string;
  smithersRunId: string;
  nodeId: string;
  iteration: number;
  attempt: number;
  title: string;
  kind: "agent" | "compute" | "static" | "unknown";
  status: "running" | "waiting" | "completed" | "failed" | "cancelled";
  summary: string;
  updatedAt: string;
  commandCount: number;
  artifactCount: number;
  transcriptMessageCount: number;
  contextBudget: {
    usedTokens: number;
    maxTokens: number;
    percent: number;
    tone: "neutral" | "orange" | "red";
    label: string;
    detail: string;
  } | null;
}

export interface WorkspaceWorkflowTaskAttemptInspector extends WorkspaceWorkflowTaskAttemptSummary {
  surfacePiSessionId: string | null;
  smithersState: string;
  prompt: string | null;
  responseText: string | null;
  error: string | null;
  cached: boolean;
  jjPointer: string | null;
  jjCwd: string | null;
  heartbeatAt: string | null;
  agentId: string | null;
  agentModel: string | null;
  agentEngine: string | null;
  agentResume: string | null;
  meta: Record<string, unknown> | null;
  startedAt: string;
  finishedAt: string | null;
  transcript: WorkspaceWorkflowTaskAttemptTranscriptMessage[];
  commandRollups: WorkspaceCommandRollup[];
  artifacts: WorkspaceCommandArtifactLink[];
}

export type WorkspaceWorkflowInspectorNodeType =
  | "workflow"
  | "sequence"
  | "parallel"
  | "loop"
  | "conditional"
  | "approval"
  | "task-agent"
  | "script"
  | "project-ci-check"
  | "wait"
  | "retry"
  | "terminal-result"
  | "unknown";

export type WorkspaceWorkflowInspectorNodeStatus =
  | "pending"
  | "running"
  | "waiting"
  | "retrying"
  | "completed"
  | "failed"
  | "cancelled"
  | "skipped";

export type WorkspaceWorkflowInspectorMode =
  | { kind: "live" }
  | { kind: "historical"; frameNo: number };

export type WorkspaceWorkflowInspectorRelatedSurfaceTarget =
  | { kind: "handler-thread"; threadId: string }
  | { kind: "task-agent"; workflowTaskAttemptId: string }
  | { kind: "command"; commandId: string }
  | { kind: "artifact"; artifactId: string }
  | { kind: "project-ci-check"; checkResultId: string };

export interface WorkspaceWorkflowInspectorNodeDetail {
  status: WorkspaceWorkflowInspectorNodeStatus;
  objectiveOrLabel: string;
  latestOutput: string | null;
  partialOutput: string | null;
  relatedArtifacts: WorkspaceCommandArtifactLink[];
  workflowAgent: string | null;
  taskAttempt: {
    workflowTaskAttemptId: string;
    iteration: number;
    attempt: number;
    status: string;
    responseText: string | null;
    error: string | null;
  } | null;
  command: {
    commandId: string;
    toolName: string;
    status: string;
    summary: string;
  } | null;
  worktree: string | null;
  timing: {
    startedAt: string | null;
    finishedAt: string | null;
    updatedAt: string | null;
    elapsedMs: number | null;
  };
  waitReason: string | null;
}

export interface WorkspaceWorkflowInspectorNode {
  key: string;
  smithersNodeId: string | null;
  parentKey: string | null;
  type: WorkspaceWorkflowInspectorNodeType;
  label: string;
  status: WorkspaceWorkflowInspectorNodeStatus;
  props: Record<string, unknown>;
  launchArguments?: Record<string, unknown>;
  task?: {
    nodeId: string;
    kind: string;
    agent?: string;
    iteration?: number;
    attempt?: number;
    workflowTaskAttemptId?: string;
  };
  projectCi?: {
    checkId: string;
    required: boolean;
    command: string | null;
    checkResultId?: string;
  };
  timing: {
    startedAt: string | null;
    finishedAt: string | null;
    updatedAt: string | null;
    elapsedMs: number | null;
  };
  waitReason: string | null;
  latestActivity: string | null;
  outputPreview: string | null;
  detail: WorkspaceWorkflowInspectorNodeDetail;
  hasFailedDescendant: boolean;
  hasWaitingDescendant: boolean;
  relatedSurfaceTargets: WorkspaceWorkflowInspectorRelatedSurfaceTarget[];
  raw: unknown;
}

export interface WorkspaceWorkflowInspectorLiveUpdate {
  workflowRunId: string;
  smithersRunId: string;
  fromSeq: number | null;
  lastSeq: number | null;
  events: unknown[];
  inspector: WorkspaceWorkflowInspectorReadModel;
}

export interface WorkspaceWorkflowInspectorFrame {
  frameNo: number;
  seq: number | null;
  createdAt: string | null;
  label: string;
}

export interface WorkspaceWorkflowInspectorDetailTab {
  id: "output" | "diff" | "logs" | "transcript" | "command" | "events" | "raw";
  label: string;
  content: unknown;
  empty: boolean;
}

export interface WorkspaceWorkflowInspectorReadModel {
  surfaceId: string;
  workflowRunId: string;
  smithersRunId: string;
  owningSessionId: string;
  owningThreadId: string;
  selectedNodeKey: string | null;
  expandedNodeKeys: string[];
  mode: WorkspaceWorkflowInspectorMode;
  runHeader: {
    svvyStatus: WorkspaceHandlerThreadWorkflowSummary["status"];
    smithersStatus: string;
    runId: string;
    workflowId: string | null;
    workflowLabel: string;
    owningHandlerThreadTitle: string;
    startedAt: string | null;
    finishedAt: string | null;
    updatedAt: string | null;
    heartbeatAt: string | null;
    lastEventAt: string | null;
    frameNo: number | null;
    frameCount: number;
    lastSeq: number | null;
  };
  tree: {
    nodes: WorkspaceWorkflowInspectorNode[];
    visibleNodeKeys: string[];
    searchQuery: string;
    matchedNodeKeys: string[];
  };
  frames: WorkspaceWorkflowInspectorFrame[];
  selectedNode: WorkspaceWorkflowInspectorNode | null;
  detailTabs: WorkspaceWorkflowInspectorDetailTab[];
  rawSnapshot: unknown;
}

export interface WorkspaceHandlerThreadSummary {
  threadId: string;
  surfacePiSessionId: string;
  title: string;
  objective: string;
  status:
    | "idle"
    | "running-handler"
    | "running-workflow"
    | "waiting"
    | "troubleshooting"
    | "completed";
  wait: {
    owner: "handler" | "workflow";
    kind: "user" | "external" | "approval" | "signal" | "timer";
    reason: string;
    resumeWhen: string;
    since: string;
  } | null;
  startedAt: string;
  updatedAt: string;
  finishedAt: string | null;
  commandCount: number;
  workflowRunCount: number;
  workflowTaskAttemptCount?: number;
  episodeCount: number;
  artifactCount: number;
  ciRunCount: number;
  loadedContextKeys: string[];
  latestWorkflowRun: WorkspaceHandlerThreadWorkflowSummary | null;
  latestCiRun: WorkspaceProjectCiRunSummary | null;
  latestEpisode: WorkspaceHandlerThreadEpisodeSummary | null;
  workflowTaskAttempts?: WorkspaceWorkflowTaskAttemptSummary[];
}

export interface WorkspaceHandlerThreadInspector extends WorkspaceHandlerThreadSummary {
  commandRollups: WorkspaceCommandRollup[];
  workflowRuns: WorkspaceHandlerThreadWorkflowSummary[];
  workflowTaskAttempts?: WorkspaceWorkflowTaskAttemptSummary[];
  episodes: WorkspaceHandlerThreadEpisodeSummary[];
  artifacts: WorkspaceCommandArtifactLink[];
}

export interface WorkspaceSidebarRowSubtitle {
  badge: "waiting" | "error" | "workflow" | "text";
  text: string;
  tone: "muted" | "waiting" | "error";
}

export interface WorkspaceSidebarWorkflowRow {
  workflowRunId: string;
  workflowName: string;
  status: WorkspaceHandlerThreadWorkflowSummary["status"];
  subtitle: WorkspaceSidebarRowSubtitle | null;
  updatedAt: string;
}

export interface WorkspaceSidebarHandlerThreadRow {
  threadId: string;
  surfacePiSessionId: string;
  title: string;
  objective: string;
  status: WorkspaceHandlerThreadSummary["status"];
  subtitle: WorkspaceSidebarRowSubtitle | null;
  updatedAt: string;
  workflows: WorkspaceSidebarWorkflowRow[];
}

export interface WorkspaceSessionSummary {
  id: string;
  title: string;
  preview: string;
  createdAt: string;
  updatedAt: string;
  messageCount: number;
  status: SessionStatus;
  isPinned: boolean;
  pinnedAt: string | null;
  isArchived: boolean;
  archivedAt: string | null;
  isUnread: boolean;
  unreadAt: string | null;
  unreadReason: "assistant-turn-finished" | "manual" | null;
  lastReadAt: string | null;
  sessionFile?: string;
  parentSessionId?: string;
  parentSessionFile?: string;
  modelId?: string;
  provider?: string;
  thinkingLevel?: string;
  wait?: {
    threadId?: string;
    kind: "user" | "external" | "approval" | "signal" | "timer";
    reason: string;
    resumeWhen: string;
    since: string;
  } | null;
  counts?: {
    turns: number;
    threads: number;
    commands: number;
    episodes: number;
    ciRuns: number;
    ciChecks: number;
    workflows: number;
    artifacts: number;
    events: number;
  };
  threadIdsByStatus?: {
    runningHandler: string[];
    runningWorkflow: string[];
    waiting: string[];
    troubleshooting: string[];
  };
  threadIds?: string[];
  sidebarThreads?: WorkspaceSidebarHandlerThreadRow[];
  commandRollups?: WorkspaceCommandRollup[];
  titleGeneration?: {
    status: SessionTitleGenerationStatus;
    renameLocked: boolean;
    autoFrozen: boolean;
    manualOverride: boolean;
    triggeredAt: string | null;
    finishedAt: string | null;
    error: string | null;
  };
}

export interface WorkspaceSessionNavigationReadModel {
  pinnedSessions: WorkspaceSessionSummary[];
  activeSessions: WorkspaceSessionSummary[];
  archived: {
    collapsed: boolean;
    sessions: WorkspaceSessionSummary[];
  };
}

export interface WorkspaceArtifactPreview {
  artifactId: string;
  sessionId: string;
  kind: WorkspaceCommandArtifactLink["kind"];
  name: string;
  path?: string;
  createdAt: string;
  sourceCommandId?: string;
  workflowRunId?: string;
  workflowName?: string;
  producerLabel?: string;
  missingFile: boolean;
  content: string;
}

export interface ConversationSurfaceSnapshot {
  target: PromptTarget;
  messages: AgentMessage[];
  pendingUserMessage?: AgentMessage | null;
  streamMessage?: AssistantMessage | null;
  provider: string;
  model: string;
  reasoningEffort: ReasoningEffort;
  sessionMode: SessionMode;
  sessionAgentKey: SessionAgentKey;
  systemPrompt: string;
  resolvedSystemPrompt: string;
  promptStatus: "idle" | "streaming";
}

export interface SurfaceSyncMessage {
  workspaceId: string;
  reason: "surface.updated" | "prompt.settled" | "background.started" | "surface.closed";
  target: PromptTarget;
  snapshot?: ConversationSurfaceSnapshot;
}

export interface ListSessionsResponse {
  sessions: WorkspaceSessionSummary[];
  navigation: WorkspaceSessionNavigationReadModel;
}

export interface CreateSessionRequest {
  title?: string;
  parentSessionId?: string;
  mode?: SessionMode;
}

export interface UpdateSessionAgentDefaultRequest {
  key: SessionAgentKey;
  settings: SessionAgentSettings;
}

export interface UpdateWorkflowAgentRequest {
  key: WorkflowAgentKey;
  settings: WorkflowAgentSettings;
}

export interface OpenSessionRequest {
  sessionId: string;
}

export interface OpenSurfaceRequest {
  target: PromptTarget;
}

export interface RenameSessionRequest {
  sessionId: string;
  title: string;
}

export interface ForkSessionRequest {
  sessionId: string;
  title?: string;
  messageTimestamp?: string | number;
}

export interface WorkspaceMutationResponse {
  ok: boolean;
}

export interface WriteClipboardTextRequest {
  text: string;
}

export interface SurfaceMutationResponse {
  ok: boolean;
  target: PromptTarget;
}

export interface ChatRPCSchema {
  bun: {
    requests: {
      getDefaults: {
        params: undefined;
        response: AgentDefaults;
      };
      getAgentSettings: {
        params: undefined;
        response: AgentSettingsState;
      };
      updateSessionAgentDefault: {
        params: UpdateSessionAgentDefaultRequest;
        response: AgentSettingsState;
      };
      updateWorkflowAgent: {
        params: UpdateWorkflowAgentRequest;
        response: AgentSettingsState;
      };
      updateAppPreferences: {
        params: AppPreferences;
        response: AgentSettingsState;
      };
      ensureWorkflowAgentsComponent: {
        params: undefined;
        response: { path: string };
      };
      getProviderAuthState: {
        params: ProviderAuthStateRequest;
        response: AuthStateResponse;
      };
      openWorkspace: {
        params: OpenWorkspaceRequest;
        response: OpenWorkspaceResponse;
      };
      getOpenWorkspaces: {
        params: undefined;
        response: WorkspaceTabInfo[];
      };
      setActiveWorkspace: {
        params: WorkspaceScopedRequest;
        response: WorkspaceMutationResponse;
      };
      closeWorkspace: {
        params: WorkspaceScopedRequest;
        response: WorkspaceMutationResponse;
      };
      getWorkspaceInfo: {
        params: WorkspaceScopedRequest;
        response: WorkspaceInfoResponse;
      };
      listWorkspaceBranches: {
        params: WorkspaceScopedRequest;
        response: WorkspaceBranchListResponse;
      };
      switchWorkspaceBranch: {
        params: WorkspaceScoped<SwitchWorkspaceBranchRequest>;
        response: SwitchWorkspaceBranchResponse;
      };
      getAppLogs: {
        params: WorkspaceScoped<AppLogQuery> | undefined;
        response: AppLogReadModel;
      };
      getAppLogSummary: {
        params: WorkspaceScopedRequest;
        response: AppLogSummary;
      };
      markAppLogsSeen: {
        params: WorkspaceScoped<{ throughSeq: number }>;
        response: AppLogSummary;
      };
      writeClipboardText: {
        params: WriteClipboardTextRequest;
        response: WorkspaceMutationResponse;
      };
      listWorkspacePaths: {
        params: WorkspaceScoped<{ refresh?: boolean }>;
        response: WorkspacePathIndexEntry[];
      };
      pickWorkspaceAttachments: {
        params: WorkspaceScopedRequest;
        response: PickWorkspaceAttachmentResponse;
      };
      openWorkspacePath: {
        params: WorkspaceScoped<OpenWorkspacePathRequest>;
        response: OpenWorkspacePathResponse;
      };
      getSavedWorkflowLibrary: {
        params: WorkspaceScopedRequest;
        response: WorkspaceSavedWorkflowLibraryReadModel;
      };
      deleteSavedWorkflowLibraryItem: {
        params: WorkspaceScoped<DeleteSavedWorkflowLibraryItemRequest>;
        response: WorkspaceSavedWorkflowLibraryReadModel;
      };
      openWorkflowSourceInEditor: {
        params: WorkspaceScoped<OpenWorkflowSourceInEditorRequest>;
        response: OpenWorkflowSourceInEditorResponse;
      };
      listSessions: {
        params: WorkspaceScopedRequest;
        response: ListSessionsResponse;
      };
      getCommandInspector: {
        params: WorkspaceScoped<{ sessionId: string; commandId: string }>;
        response: WorkspaceCommandInspector | null;
      };
      listHandlerThreads: {
        params: WorkspaceScoped<{ sessionId: string }>;
        response: WorkspaceHandlerThreadSummary[];
      };
      getHandlerThreadInspector: {
        params: WorkspaceScoped<{ sessionId: string; threadId: string }>;
        response: WorkspaceHandlerThreadInspector | null;
      };
      getWorkflowTaskAttemptInspector: {
        params: WorkspaceScoped<{ sessionId: string; workflowTaskAttemptId: string }>;
        response: WorkspaceWorkflowTaskAttemptInspector | null;
      };
      getWorkflowInspector: {
        params: WorkspaceScoped<{
          sessionId: string;
          workflowRunId: string;
          selectedNodeKey?: string | null;
          expandedNodeKeys?: string[];
          userCollapsedNodeKeys?: string[];
          searchQuery?: string;
          mode?: WorkspaceWorkflowInspectorMode;
        }>;
        response: WorkspaceWorkflowInspectorReadModel | null;
      };
      streamWorkflowInspector: {
        params: WorkspaceScoped<{
          sessionId: string;
          workflowRunId: string;
          selectedNodeKey?: string | null;
          expandedNodeKeys?: string[];
          userCollapsedNodeKeys?: string[];
          searchQuery?: string;
          mode?: WorkspaceWorkflowInspectorMode;
          fromSeq?: number | null;
        }>;
        response: WorkspaceWorkflowInspectorLiveUpdate | null;
      };
      getProjectCiStatus: {
        params: WorkspaceScoped<{ sessionId: string }>;
        response: WorkspaceProjectCiStatusPanel;
      };
      getArtifactPreview: {
        params: WorkspaceScoped<{ sessionId: string; artifactId: string }>;
        response: WorkspaceArtifactPreview;
      };
      createSession: {
        params: WorkspaceScoped<CreateSessionRequest>;
        response: ConversationSurfaceSnapshot;
      };
      openSession: {
        params: WorkspaceScoped<OpenSessionRequest>;
        response: ConversationSurfaceSnapshot;
      };
      recordSessionOpened: {
        params: WorkspaceScoped<OpenSessionRequest>;
        response: WorkspaceMutationResponse;
      };
      openSurface: {
        params: WorkspaceScoped<OpenSurfaceRequest>;
        response: ConversationSurfaceSnapshot;
      };
      closeSurface: {
        params: WorkspaceScoped<CloseSurfaceRequest>;
        response: WorkspaceMutationResponse;
      };
      renameSession: {
        params: WorkspaceScoped<RenameSessionRequest>;
        response: WorkspaceMutationResponse;
      };
      setSessionMode: {
        params: WorkspaceScoped<SetSessionModeRequest>;
        response: SetSessionModeResponse;
      };
      forkSession: {
        params: WorkspaceScoped<ForkSessionRequest>;
        response: ConversationSurfaceSnapshot;
      };
      deleteSession: {
        params: WorkspaceScoped<{ sessionId: string }>;
        response: WorkspaceMutationResponse;
      };
      pinSession: {
        params: WorkspaceScoped<{ sessionId: string }>;
        response: WorkspaceMutationResponse;
      };
      unpinSession: {
        params: WorkspaceScoped<{ sessionId: string }>;
        response: WorkspaceMutationResponse;
      };
      archiveSession: {
        params: WorkspaceScoped<{ sessionId: string }>;
        response: WorkspaceMutationResponse;
      };
      unarchiveSession: {
        params: WorkspaceScoped<{ sessionId: string }>;
        response: WorkspaceMutationResponse;
      };
      markSessionUnread: {
        params: WorkspaceScoped<{ sessionId: string }>;
        response: WorkspaceMutationResponse;
      };
      markSessionRead: {
        params: WorkspaceScoped<{ sessionId: string }>;
        response: WorkspaceMutationResponse;
      };
      recordFocusedSession: {
        params: WorkspaceScoped<{ sessionId: string | null; surfacePiSessionId?: string | null }>;
        response: WorkspaceMutationResponse;
      };
      setArchivedGroupCollapsed: {
        params: WorkspaceScoped<{ collapsed: boolean }>;
        response: WorkspaceMutationResponse;
      };
      sendPrompt: {
        params: WorkspaceScoped<SendPromptRequest>;
        response: SendPromptResponse;
      };
      setSurfaceModel: {
        params: WorkspaceScoped<SetSurfaceModelRequest>;
        response: SurfaceMutationResponse;
      };
      setSurfaceThoughtLevel: {
        params: WorkspaceScoped<SetSurfaceThoughtLevelRequest>;
        response: SurfaceMutationResponse;
      };
      cancelPrompt: {
        params: WorkspaceScoped<CancelPromptRequest>;
        response: WorkspaceMutationResponse;
      };
      listProviderAuths: {
        params: undefined;
        response: ProviderAuthInfo[];
      };
      setProviderApiKey: {
        params: { providerId: string; apiKey: string };
        response: { ok: boolean };
      };
      startOAuth: {
        params: { providerId: string };
        response: { ok: boolean; error?: string };
      };
      removeProviderAuth: {
        params: { providerId: string };
        response: { ok: boolean };
      };
    };
    messages: Record<string, never>;
  };
  webview: {
    requests: Record<string, never>;
    messages: {
      sendWorkspaceSync: WorkspaceSyncMessage;
      sendSurfaceSync: SurfaceSyncMessage;
      sendAppLogUpdate: AppLogUpdateMessage;
      sendAppMenuAction: { action: AppMenuAction };
    };
  };
}
