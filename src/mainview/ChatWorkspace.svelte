<script lang="ts">
  import { onMount } from "svelte";
  import { createHotkeys } from "@tanstack/svelte-hotkeys";
  import PanelLeftIcon from "@lucide/svelte/icons/panel-left";
  import PanelLeftDashedIcon from "@lucide/svelte/icons/panel-left-dashed";
  import type { AssistantMessage, Model } from "@mariozechner/pi-ai";
  import type { ThinkingLevel } from "@mariozechner/pi-agent-core";
  import type { SessionMode } from "../shared/agent-settings";
  import ArtifactsPanel from "./ArtifactsPanel.svelte";
  import { ArtifactsController, type ArtifactsSnapshot } from "./artifacts";
  import CommandPalette from "./CommandPalette.svelte";
  import ContextBudgetBar from "./ContextBudgetBar.svelte";
  import DockviewWorkspace from "./DockviewWorkspace.svelte";
  import { formatTimestamp } from "./chat-format";
  import {
    getCommandInspectorSections,
    getVisibleCommandRollups,
    getWorkspaceCommandStatusPresentation,
  } from "./command-inspector";
  import {
    buildSurfaceContextBudget,
    type ContextBudget,
  } from "./context-budget";
  import {
    projectConversation,
    projectConversationSummary,
  } from "./conversation-projection";
  import {
    buildTranscriptSemanticBlocks,
    type TranscriptSemanticBlock,
  } from "./transcript-projection";
  import { buildSessionTranscriptExport } from "./session-transcript";
  import { getSurfaceDisplayTitle } from "./surface-title";
  import type {
    WorkspaceCommandArtifactLink,
    WorkspaceCommandInspector,
    WorkspaceCommandRollup,
    AppLogSummary,
    WorkspaceHandlerThreadInspector,
    WorkspaceHandlerThreadSummary,
    WorkspaceHandlerThreadWorkflowSummary,
    WorkspaceProjectCiCheckSummary,
    WorkspaceProjectCiPanelStatus,
    WorkspaceProjectCiStatusPanel,
    WorkspaceWorkflowTaskAttemptInspector,
    WorkspaceWorkflowTaskAttemptSummary,
    PromptTarget,
    WorkspacePaneSurfaceTarget,
    WorkspaceSessionNavigationReadModel,
    WorkspaceSessionSummary,
    WorkspaceSidebarHandlerThreadRow,
    WorkspaceSidebarWorkflowRow,
  } from "../shared/workspace-contract";
  import type { WorkspaceInspectorSelection } from "./chat-storage";
  import type { PromptHistoryEntry } from "./prompt-history";
  import {
    clampSidebarWidth,
    getMaxSidebarWidth,
    MIN_SIDEBAR_WIDTH,
  } from "./sidebar-layout";
  import { getViewportClass, shouldUseDesktopInspectorSplit, shouldUseNarrowShell } from "./responsive-layout";
  import SessionSidebar from "./SessionSidebar.svelte";
  import {
    PRIMARY_CHAT_PANE_ID,
    type ChatRuntime,
    type ChatPaneLayoutState,
    type ChatPaneState,
    type ChatSurfaceController,
  } from "./chat-runtime";
  import {
    createEmptyPaneLayout,
    getSidebarSessionOpenTarget,
    type WorkspaceLayoutSlotId,
    type WorkspaceLayoutSlotSummary,
  } from "./pane-layout";
  import {
    buildCommandRegistry,
    executeCommandAction,
    executePaletteFallbackPrompt,
    filterCommandActions,
    getCommandExecutionPaneId,
    getCommandPaletteInitialInput,
    getCommandPalettePlacement,
    type CommandAction,
    type CommandPaletteMode,
  } from "./command-palette";
  import {
    getShortcutHotkey,
    getShortcutReadable,
    shouldShortcutIgnoreInputs,
    type AppMenuAction,
    type ShortcutActionId,
  } from "../shared/shortcut-registry";
  import ModelPickerDialog from "./ModelPickerDialog.svelte";
  import EpisodeCard, { type ReferenceEpisode } from "./reference-cards/EpisodeCard.svelte";
  import VerificationCard, { type ReferenceVerification } from "./reference-cards/VerificationCard.svelte";
  import type { ReferenceStatus } from "./reference-cards/StatusBadge.svelte";
  import type { ReferenceSubagent } from "./reference-cards/SubagentCard.svelte";
  import ThreadCard, { type ReferenceThread } from "./reference-cards/ThreadCard.svelte";
  import WaitingCard from "./reference-cards/WaitingCard.svelte";
  import WorkflowCard, { type ReferenceWorkflow } from "./reference-cards/WorkflowCard.svelte";
  import Dialog from "./ui/Dialog.svelte";
  import Badge from "./ui/Badge.svelte";
  import Button from "./ui/Button.svelte";
  import { rpc } from "./rpc";
  import Input from "./ui/Input.svelte";
  import Tooltip from "./ui/Tooltip.svelte";
  import WorkspaceTabStrip, { type WorkspaceTabStripItem } from "./WorkspaceTabStrip.svelte";

  const DEFAULT_SIDEBAR_WIDTH = 240;

  type Props = {
    runtime: ChatRuntime;
    shortcutsEnabled?: boolean;
    onOpenSettings?: () => void;
    workspaceTabs?: WorkspaceTabStripItem[];
    activeWorkspaceId?: string | null;
    openingWorkspace?: boolean;
    onSelectWorkspace?: (workspaceId: string) => void;
    onCloseWorkspace?: (workspaceId: string) => void;
    onOpenWorkspace?: () => void;
    onReorderWorkspace?: (workspaceId: string, beforeWorkspaceId: string | null) => void;
  };

  let {
    runtime,
    shortcutsEnabled = true,
    onOpenSettings,
    workspaceTabs = [],
    activeWorkspaceId = null,
    openingWorkspace = false,
    onSelectWorkspace,
    onCloseWorkspace,
    onOpenWorkspace,
    onReorderWorkspace,
  }: Props = $props();
  const sidebarToggleShortcut = getShortcutReadable("sidebar.toggle");

  let controller = $state<ArtifactsController | null>(null);
  let messages = $state<ChatSurfaceController["agent"]["state"]["messages"]>([]);
  let streamMessage = $state<AssistantMessage | null>(null);
  let pendingToolCalls = $state(new Set<string>());
  let isStreaming = $state(false);
  let errorMessage = $state<string | undefined>(undefined);
  let currentModel = $state<Model<any> | null>(null);
  let currentThinkingLevel = $state<ThinkingLevel>("off");
  let currentSessionMode = $state<SessionMode>("orchestrator");
  let artifactsSnapshot = $state<ArtifactsSnapshot>({
    activeFilename: null,
    artifacts: [],
    logsByFilename: {},
  });
  let showArtifactsPanel = $state(false);
  let showModelPicker = $state(false);
  let allowedProviders = $state<string[]>([]);
  let promptHistory = $state<PromptHistoryEntry[]>([]);
  let windowWidth = $state(0);
  let isMacWindowChrome = $state(false);
  let dockviewLayoutEpoch = $state(0);
  let sessions = $state<WorkspaceSessionSummary[]>([]);
  let workspaceBranch = $state(runtime.branch);
  let sessionNavigation = $state<WorkspaceSessionNavigationReadModel>({
    pinnedSessions: [],
    activeSessions: [],
    archived: {
      collapsed: true,
      sessions: [],
    },
  });
  let appLogSummary = $state<AppLogSummary>({
    latestSeq: 0,
    seenSeq: 0,
    unread: { total: 0, info: 0, warning: 0, error: 0 },
    totals: { total: 0, info: 0, warning: 0, error: 0 },
  });
  let activeSessionId = $state<string | undefined>(undefined);
  let paneLayout = $state<ChatPaneLayoutState>({
    ...createEmptyPaneLayout(),
    focusedPanelId: PRIMARY_CHAT_PANE_ID,
  });
  let activeLayoutId = $state<WorkspaceLayoutSlotId>("A");
  let layoutSlots = $state<WorkspaceLayoutSlotSummary[]>([]);
  let currentPane = $state<ChatPaneState | null>(null);
  let focusedPanelId = $state(PRIMARY_CHAT_PANE_ID);
  let focusedSurfaceTarget = $state<PromptTarget | null>(null);
  let currentSurfaceController = $state<ChatSurfaceController | null>(null);
  let sidebarError = $state<string | undefined>(undefined);
  let sidebarHidden = $state(false);
  let sidebarWidth = $state(DEFAULT_SIDEBAR_WIDTH);
  let sidebarResizing = $state(false);
  let mutatingSession = $state(false);
  let sendingPrompt = $state(false);
  let renameTarget = $state<WorkspaceSessionSummary | null>(null);
  let renameValue = $state("");
  let sidebarResizeHandle = $state<HTMLDivElement | null>(null);
  let artifactSyncSessionId: string | undefined = undefined;
  let artifactSyncMessageCount = 0;
  let copyTranscriptState = $state<{
    panelId: string | null;
    status: "idle" | "copying" | "copied" | "error";
  }>({
    panelId: null,
    status: "idle",
  });
  let showCommandInspector = $state(false);
  let commandInspector = $state<WorkspaceCommandInspector | null>(null);
  let commandInspectorError = $state<string | undefined>(undefined);
  let commandInspectorLoading = $state(false);
  let commandInspectorCommandId = $state<string | null>(null);
  let handlerThreads = $state<WorkspaceHandlerThreadSummary[]>([]);
  let handlerThreadsLoading = $state(false);
  let handlerThreadsError = $state<string | undefined>(undefined);
  let projectCiStatus = $state<WorkspaceProjectCiStatusPanel | null>(null);
  let projectCiError = $state<string | undefined>(undefined);
  let showThreadInspector = $state(false);
  let threadInspector = $state<WorkspaceHandlerThreadInspector | null>(null);
  let threadInspectorError = $state<string | undefined>(undefined);
  let threadInspectorLoading = $state(false);
  let threadInspectorThreadId = $state<string | null>(null);
  let showWorkflowTaskAttemptInspector = $state(false);
  let workflowTaskAttemptInspector = $state<WorkspaceWorkflowTaskAttemptInspector | null>(null);
  let workflowTaskAttemptInspectorError = $state<string | undefined>(undefined);
  let workflowTaskAttemptInspectorLoading = $state(false);
  let workflowTaskAttemptInspectorId = $state<string | null>(null);
  let paletteOpen = $state(false);
  let paletteInitialInput = $state(getCommandPaletteInitialInput("commands"));
  let paletteError = $state<string | undefined>(undefined);
  let paletteBusy = $state(false);
  let workspaceMentionPaths = $state<ReadonlySet<string>>(new Set());

  let sidebarResizePointerId: number | null = null;
  let sidebarResizeOriginX = 0;
  let sidebarResizeOriginWidth = DEFAULT_SIDEBAR_WIDTH;
  let copyTranscriptResetTimer: ReturnType<typeof setTimeout> | null = null;
  let commandInspectorSessionId: string | null = null;
  let handlerThreadLoadToken = 0;
  let projectCiLoadToken = 0;
  let threadInspectorSessionId: string | null = null;
  let workflowTaskAttemptInspectorSessionId: string | null = null;
  let unsubscribeSurfaceController: (() => void) | null = null;
  let restoredInspectorKey: string | null = null;
  let dockviewLayoutSyncFrame: number | null = null;
  let dockviewLayoutSyncTimer: ReturnType<typeof setTimeout> | null = null;

  const conversation = $derived(projectConversation(messages));
  const conversationSummary = $derived(projectConversationSummary(conversation, streamMessage));
  const artifactCount = $derived(artifactsSnapshot.artifacts.length);
  const hasArtifacts = $derived(artifactCount > 0);
  const showDesktopSplit = $derived(
    shouldUseDesktopInspectorSplit(windowWidth) && showArtifactsPanel && hasArtifacts,
  );
  const showOverlayArtifacts = $derived(
    !shouldUseDesktopInspectorSplit(windowWidth) && showArtifactsPanel && hasArtifacts,
  );
  const viewportClass = $derived(getViewportClass(windowWidth));
  const narrowShell = $derived(shouldUseNarrowShell(windowWidth));
  const effectiveSidebarWidth = $derived(clampSidebarWidth(sidebarWidth, windowWidth));
  const currentSession = $derived(sessions.find((session) => session.id === activeSessionId) ?? null);
  const currentCommandRollups = $derived(getVisibleCommandRollups(currentSession));
  const currentSurface = $derived(focusedSurfaceTarget);
  type SidebarPaneTone = "neutral" | "waiting" | "error";
  type SidebarPaneLocation = {
    paneId: string;
    panelId: string;
    label: string;
    focused: boolean;
    tone: SidebarPaneTone;
    contextBudget: ContextBudget | null;
  };

  function getSidebarPaneTone(
    binding: WorkspacePaneSurfaceTarget | null | undefined,
    paneController: ChatSurfaceController | null,
  ): SidebarPaneTone {
    if (paneController?.agent.state.error) {
      return "error";
    }
    if (!binding?.workspaceSessionId) {
      return "neutral";
    }
    const session = sessions.find((candidate) => candidate.id === binding.workspaceSessionId);
    if (!session) {
      return "neutral";
    }
    if (binding.surface === "orchestrator") {
      if (session.status === "error") return "error";
      if (session.status === "waiting") return "waiting";
      return "neutral";
    }
    if (binding.surface === "thread") {
      const threadId = binding.threadId;
      if (threadId && session.threadIdsByStatus?.waiting.includes(threadId)) {
        return "waiting";
      }
    }
    return "neutral";
  }

  function buildSidebarPaneLocations(
    predicate: (binding: WorkspacePaneSurfaceTarget) => boolean,
  ): SidebarPaneLocation[] {
    return paneLayout.panels
      .filter((panel) => panel.binding && predicate(panel.binding))
      .map((panel, index) => {
        const paneController = runtime.getPaneController(panel.panelId);
        return {
          paneId: panel.panelId,
          panelId: panel.panelId,
          label: panel.restore?.lastKnownLocationLabel ?? (index === 0 ? "Docked" : `Docked ${index + 1}`),
          focused: panel.panelId === paneLayout.focusedPanelId,
          tone: getSidebarPaneTone(panel.binding, paneController),
          contextBudget: getPaneContextBudget(paneController),
        };
      });
  }

  const paneLocationsBySessionId = $derived(
    Object.fromEntries(
      sessions.map((session) => [
        session.id,
        buildSidebarPaneLocations(
          (binding) => binding.workspaceSessionId === session.id && binding.surface === "orchestrator",
        ),
      ]),
    ),
  );
  const paneLocationsByThreadId = $derived(
    Object.fromEntries(
      sessions.flatMap((session) =>
        (session.sidebarThreads ?? []).map((thread) => [
          thread.threadId,
          buildSidebarPaneLocations(
            (binding) =>
              binding.workspaceSessionId === session.id &&
              binding.surface === "thread" &&
              binding.threadId === thread.threadId,
          ),
        ]),
      ),
    ),
  );
  const paneLocationsByWorkflowRunId = $derived(
    Object.fromEntries(
      sessions.flatMap((session) =>
        (session.sidebarThreads ?? []).flatMap((thread) =>
          thread.workflows.map((workflow) => [
            workflow.workflowRunId,
            buildSidebarPaneLocations(
              (binding) =>
                binding.workspaceSessionId === session.id &&
                binding.surface === "workflow-inspector" &&
                binding.workflowRunId === workflow.workflowRunId,
            ),
          ]),
        ),
      ),
    ),
  );
  const currentSurfaceLabel = $derived.by(() => {
    if (currentSurface?.surface === "thread") {
      return `Messaging handler thread ${currentSurface.threadId ?? currentSurface.surfacePiSessionId}`;
    }

    if (currentSessionMode === "dumb") {
      return "Messaging dumb session";
    }

    return "Messaging orchestrator";
  });
  function formatPaneSurfaceLabel(
    paneController: ChatSurfaceController | null,
    binding?: WorkspacePaneSurfaceTarget | null,
  ): string {
    if (binding?.surface === "workflow-inspector") {
      return "Workflow Inspector";
    }
    if (binding?.surface === "saved-workflow-library") {
      return "Saved Workflow Library";
    }
    if (binding?.surface === "app-logs") {
      return "Logs";
    }
    if (binding?.surface === "command") {
      return "Command Inspector";
    }
    if (binding?.surface === "workflow-task-attempt") {
      return "Workflow Task-Agent";
    }
    if (binding?.surface === "artifact") {
      return "Artifact";
    }
    if (binding?.surface === "project-ci-check") {
      return "Project CI Check";
    }
    if (paneController?.target.surface === "thread") {
      return "Handler Thread";
    }
    return paneController?.sessionMode === "dumb" ? "Dumb Session" : "Orchestrator";
  }
  function formatPaneAgentSummary(
    paneController: ChatSurfaceController | null,
    binding?: WorkspacePaneSurfaceTarget | null,
  ): string {
    if (binding?.surface === "workflow-inspector") {
      return binding.workflowRunId;
    }
    if (binding?.surface === "saved-workflow-library") {
      return ".svvy/workflows";
    }
    if (binding?.surface === "app-logs") {
      return "workspace";
    }
    if (binding && binding.surface !== "orchestrator" && binding.surface !== "thread") {
      return binding.workspaceSessionId;
    }
    const model = paneController?.agent.state.model;
    const thinking = paneController?.agent.state.thinkingLevel;
    if (!model) return "No agent";
    return `${model.provider}/${model.id} · ${thinking}`;
  }
  function formatPaneLocationMetadata(
    binding?: WorkspacePaneSurfaceTarget | null,
  ): { label: string; value: string } {
    if (binding?.surface === "workflow-inspector") return { label: "surface", value: "workflow" };
    if (binding?.surface === "saved-workflow-library") return { label: "surface", value: "library" };
    if (binding?.surface === "app-logs") return { label: "surface", value: "logs" };
    if (binding?.surface === "command") return { label: "surface", value: "command" };
    if (binding?.surface === "workflow-task-attempt") return { label: "surface", value: "task" };
    if (binding?.surface === "artifact") return { label: "surface", value: "artifact" };
    if (binding?.surface === "project-ci-check") return { label: "surface", value: "project-ci" };
    if (workspaceBranch) return { label: "worktree", value: workspaceBranch };
    return { label: "workspace", value: runtime.workspaceLabel };
  }
  function getPaneSurfaceStatus(
    paneController: ChatSurfaceController | null,
    binding?: WorkspacePaneSurfaceTarget | null,
  ): string {
    if (paneController?.agent.state.isStreaming || paneController?.promptStatus === "streaming") {
      return "running";
    }
    if (paneController?.agent.state.error) {
      return "failed";
    }
    if (binding?.surface && binding.surface !== "orchestrator" && binding.surface !== "thread") {
      return "active";
    }
    return "idle";
  }
  function getPaneContextBudget(paneController: ChatSurfaceController | null): ContextBudget | null {
    if (!paneController) return null;
    return buildSurfaceContextBudget(
      paneController.agent.state.messages,
      paneController.agent.state.model,
    );
  }
  const summaryMessageCount = $derived(conversationSummary.messageCount);
  const composerErrorMessage = $derived.by(() => {
    const message =
      errorMessage ?? (currentSession?.status === "error" ? currentSession.preview : undefined);
    if (!message) {
      return undefined;
    }

    return message;
  });
  const promptBusy = $derived(isStreaming || sendingPrompt);
  function getCopyTranscriptLabel(panelId: string): string {
    if (copyTranscriptState.panelId !== panelId) {
      return "Copy pane transcript";
    }
    switch (copyTranscriptState.status) {
      case "copying":
        return "Copying...";
      case "copied":
        return "Copied";
      case "error":
        return "Copy failed";
      default:
        return "Copy pane transcript";
    }
  }
  const commandInspectorSections = $derived(getCommandInspectorSections(commandInspector));
  const showHandlerThreadPanel = $derived(
    currentSurface?.surface === "orchestrator" &&
      (handlerThreadsLoading || !!handlerThreadsError || handlerThreads.length > 0),
  );
  const hasActionableProjectCiStatus = $derived(
    !!projectCiError ||
      !!projectCiStatus?.activeWorkflowRun ||
      !!projectCiStatus?.latestRun ||
      (projectCiStatus?.entries.length ?? 0) > 0 ||
      (projectCiStatus?.checks.length ?? 0) > 0,
  );
  const showDetailedProjectCiPanel = $derived(
    currentSurface?.surface === "orchestrator" && currentSession && hasActionableProjectCiStatus,
  );
  const transcriptSemanticBlocks = $derived(
    buildTranscriptSemanticBlocks({
      session: currentSession,
      errorMessage,
      commandRollups: currentCommandRollups,
      handlerThreads,
    }),
  );
  const showNewSessionEmptyState = $derived(
    currentSurface?.surface === "orchestrator" &&
      summaryMessageCount === 0 &&
      !streamMessage &&
      !isStreaming &&
      !errorMessage &&
      !showHandlerThreadPanel &&
      !showDetailedProjectCiPanel,
  );
  const recentSessionSuggestions = $derived(
    sessions.filter((session) => session.id !== activeSessionId).slice(0, 3),
  );
  const threadLocalProjectCiRun = $derived.by(() => {
    if (!threadInspector || !projectCiStatus?.latestRun) {
      return null;
    }
    return projectCiStatus.latestRun.threadId === threadInspector.threadId
      ? projectCiStatus.latestRun
      : null;
  });
  const commandRegistry = $derived(
    buildCommandRegistry({
      sessions,
      focusedSessionId: activeSessionId,
      focusedSurfaceTarget,
      handlerThreads,
      projectCiStatus,
    }),
  );
  const visibleCommandActions = $derived(filterCommandActions(commandRegistry, ""));
  const workspaceHotkeysEnabled = $derived(
    shortcutsEnabled &&
      !paletteOpen &&
      !renameTarget &&
      !showModelPicker &&
      !showCommandInspector &&
      !showThreadInspector &&
      !showWorkflowTaskAttemptInspector,
  );

  createHotkeys(
    () => [
      {
        hotkey: getShortcutHotkey("commandPalette.open"),
        callback: () => openPalette("commands"),
        options: () => workspaceShortcutOptions("commandPalette.open"),
      },
      {
        hotkey: getShortcutHotkey("quickOpen.open"),
        callback: () => openPalette("search"),
        options: () => workspaceShortcutOptions("quickOpen.open"),
      },
      {
        hotkey: getShortcutHotkey("session.new"),
        callback: () => void handleCreateSession(),
        options: () => workspaceShortcutOptions("session.new"),
      },
      {
        hotkey: getShortcutHotkey("session.dumb"),
        callback: () => void handleCreateDumbSession(),
        options: () => workspaceShortcutOptions("session.dumb"),
      },
      {
        hotkey: getShortcutHotkey("sidebar.toggle"),
        callback: () => toggleSidebarVisibility(),
        options: () => workspaceShortcutOptions("sidebar.toggle"),
      },
    ],
    () => ({
      enabled: workspaceHotkeysEnabled,
      preventDefault: true,
      conflictBehavior: "replace",
    }),
  );

  function workspaceShortcutOptions(id: ShortcutActionId) {
    return {
      ignoreInputs: shouldShortcutIgnoreInputs(id),
    };
  }

  function clearCopyTranscriptResetTimer() {
    if (!copyTranscriptResetTimer) return;
    clearTimeout(copyTranscriptResetTimer);
    copyTranscriptResetTimer = null;
  }

  function scheduleCopyTranscriptReset() {
    clearCopyTranscriptResetTimer();
    copyTranscriptResetTimer = window.setTimeout(() => {
      copyTranscriptState = { panelId: null, status: "idle" };
      copyTranscriptResetTimer = null;
    }, 2400);
  }

  async function copyTextToClipboard(text: string): Promise<void> {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
      return;
    }

    const fallback = document.createElement("textarea");
    fallback.value = text;
    fallback.setAttribute("readonly", "true");
    fallback.style.position = "fixed";
    fallback.style.top = "0";
    fallback.style.left = "0";
    fallback.style.opacity = "0";
    document.body.appendChild(fallback);
    fallback.focus();
    fallback.select();

    try {
      const copied = document.execCommand("copy");
      if (!copied) {
        throw new Error("Document copy command was rejected.");
      }
    } finally {
      document.body.removeChild(fallback);
    }
  }

  async function openModelSelector() {
    if (!currentModel) return;
    setTimeout(() => {
      showModelPicker = true;
    }, 0);
    allowedProviders = [currentModel.provider];
    try {
      const configuredProviders = await runtime.listConfiguredProviders();
      allowedProviders = Array.from(new Set([currentModel.provider, ...configuredProviders]));
    } catch {
      allowedProviders = [currentModel.provider];
    }
  }

  function getLatestAssistantFailureMessage(
    messagesSnapshot: ChatSurfaceController["agent"]["state"]["messages"],
  ): string | undefined {
    const lastMessage = messagesSnapshot[messagesSnapshot.length - 1];
    if (!lastMessage || lastMessage.role !== "assistant") return undefined;
    if (lastMessage.stopReason !== "error" && lastMessage.stopReason !== "aborted") return undefined;

    const message =
      lastMessage.errorMessage ??
      lastMessage.content
        .filter((block) => block.type === "text")
        .map((block) => block.text)
        .join("\n")
        .trim();

    return message || undefined;
  }

  function syncArtifacts(snapshot: ArtifactsSnapshot) {
    const createdNewArtifact = snapshot.artifacts.length > artifactsSnapshot.artifacts.length;
    artifactsSnapshot = snapshot;
    if (snapshot.artifacts.length === 0) {
      showArtifactsPanel = false;
      return;
    }
    if (createdNewArtifact) {
      showArtifactsPanel = true;
    }
  }

  async function syncArtifactsFromRuntime(force = false) {
    if (!controller || !currentSurfaceController) return;

    const sessionId = currentSurfaceController.agent.sessionId;
    const nextMessageCount = currentSurfaceController.agent.state.messages.length;
    const sessionChanged = artifactSyncSessionId !== sessionId;
    const cursorWentBackwards = nextMessageCount < artifactSyncMessageCount;

    if (force || sessionChanged || cursorWentBackwards) {
      await controller.syncFromMessages(currentSurfaceController.agent.state.messages, { replace: true });
      artifactSyncSessionId = sessionId;
      artifactSyncMessageCount = nextMessageCount;
      return;
    }

    await controller.syncFromMessages(currentSurfaceController.agent.state.messages);
    artifactSyncMessageCount = nextMessageCount;
  }

  function pulseDockviewLayout() {
    dockviewLayoutEpoch += 1;
  }

  function scheduleDockviewLayoutPulse(delayMs = 0) {
    if (delayMs > 0) {
      if (dockviewLayoutSyncTimer) {
        clearTimeout(dockviewLayoutSyncTimer);
      }
      dockviewLayoutSyncTimer = setTimeout(() => {
        dockviewLayoutSyncTimer = null;
        pulseDockviewLayout();
      }, delayMs);
      return;
    }

    if (dockviewLayoutSyncFrame !== null) {
      window.cancelAnimationFrame(dockviewLayoutSyncFrame);
    }
    dockviewLayoutSyncFrame = window.requestAnimationFrame(() => {
      dockviewLayoutSyncFrame = null;
      pulseDockviewLayout();
    });
  }

  function syncDockviewAfterSidebarToggle() {
    pulseDockviewLayout();
    scheduleDockviewLayoutPulse();
    scheduleDockviewLayoutPulse(260);
  }

  function toggleSidebarVisibility() {
    sidebarHidden = !sidebarHidden;
    syncDockviewAfterSidebarToggle();
  }

  function handleWorkspaceTransitionEnd(event: TransitionEvent) {
    if (event.target !== event.currentTarget || event.propertyName !== "grid-template-columns") return;
    pulseDockviewLayout();
  }

  function setSidebarResizing(nextValue: boolean) {
    sidebarResizing = nextValue;
    document.body.classList.toggle("sidebar-resizing", nextValue);
  }

  function startSidebarResize(event: PointerEvent) {
    if (sidebarHidden || narrowShell || !sidebarResizeHandle) return;
    event.preventDefault();

    sidebarResizePointerId = event.pointerId;
    sidebarResizeOriginX = event.clientX;
    sidebarResizeOriginWidth = effectiveSidebarWidth;
    sidebarResizeHandle.setPointerCapture(event.pointerId);
    setSidebarResizing(true);
  }

  function handleSidebarResizeMove(event: PointerEvent) {
    if (!sidebarResizing || sidebarResizePointerId !== event.pointerId) return;
    const delta = event.clientX - sidebarResizeOriginX;
    sidebarWidth = clampSidebarWidth(sidebarResizeOriginWidth + delta, windowWidth);
  }

  function stopSidebarResize(event?: PointerEvent) {
    if (event && sidebarResizePointerId !== event.pointerId) return;

    if (sidebarResizeHandle && sidebarResizePointerId !== null && sidebarResizeHandle.hasPointerCapture(sidebarResizePointerId)) {
      sidebarResizeHandle.releasePointerCapture(sidebarResizePointerId);
    }

    sidebarResizePointerId = null;
    setSidebarResizing(false);
  }

  async function runSessionMutation(
    action: () => Promise<void>,
    options: { rethrow?: boolean } = {},
  ) {
    if (mutatingSession) return;
    mutatingSession = true;
    sidebarError = undefined;

    try {
      await action();
      syncRuntimeState();
      resubscribeSurfaceController();
      syncSurfaceState();
      await syncArtifactsFromRuntime();
    } catch (error) {
      sidebarError = error instanceof Error ? error.message : "Session update failed.";
      if (options.rethrow) {
        throw error;
      }
    } finally {
      mutatingSession = false;
    }
  }

  async function handleSwitchWorkspaceBranch(branch: string) {
    await runSessionMutation(() => runtime.switchWorkspaceBranch(branch), { rethrow: true });
  }

  async function handleSwitchLayout(layoutId: WorkspaceLayoutSlotId) {
    await runSessionMutation(() => runtime.switchWorkspaceLayout(layoutId));
    scheduleDockviewLayoutPulse();
  }

  function openPalette(mode: CommandPaletteMode) {
    paletteInitialInput = getCommandPaletteInitialInput(mode);
    paletteError = undefined;
    paletteOpen = true;
  }

  function closePalette() {
    paletteOpen = false;
    paletteError = undefined;
    paletteBusy = false;
  }

  function handleAppMenuAction(action: AppMenuAction) {
    if (!shortcutsEnabled) return;
    switch (action) {
      case "commandPalette.open":
        openPalette("commands");
        return;
      case "quickOpen.open":
        openPalette("search");
        return;
      case "session.new":
        void handleCreateSession();
        return;
      case "session.dumb":
        void handleCreateDumbSession();
        return;
      case "sidebar.toggle":
        toggleSidebarVisibility();
        return;
    }
  }

  async function runPaletteMutation(action: () => Promise<void>) {
    if (paletteBusy) return;
    paletteBusy = true;
    paletteError = undefined;
    sidebarError = undefined;
    try {
      await action();
      syncRuntimeState();
      resubscribeSurfaceController();
      syncSurfaceState();
      await syncArtifactsFromRuntime();
      closePalette();
    } catch (error) {
      paletteError = error instanceof Error ? error.message : "Command failed.";
    } finally {
      paletteBusy = false;
    }
  }

  async function handlePaletteExecute(action: CommandAction, event: KeyboardEvent | MouseEvent) {
    const placement = getCommandPalettePlacement(event);
    window.setTimeout(
      () =>
        void runPaletteMutation(() => {
          const panelId =
            action.category === "pane"
              ? (runtime.paneLayout.focusedPanelId ?? focusedPanelId)
              : getCommandExecutionPaneId({
                  placement,
                  focusedPanelId: runtime.paneLayout.focusedPanelId ?? focusedPanelId,
                });
          return executeCommandAction({
            runtime,
            action,
            panelId,
            onOpenSettings: () => onOpenSettings?.(),
            onOpenWorkflowTaskAttempt: ({ workspaceSessionId, workflowTaskAttemptId }) =>
              handleInspectWorkflowTaskAttempt({ workflowTaskAttemptId }, workspaceSessionId),
          });
        }),
      0,
    );
  }

  async function handlePaletteFallbackPrompt(prompt: string, event: KeyboardEvent) {
    const panelId = getCommandExecutionPaneId({
      placement: getCommandPalettePlacement(event),
      focusedPanelId,
    });
    window.setTimeout(
      () =>
        void runPaletteMutation(async () => {
          await executePaletteFallbackPrompt({
            runtime,
            prompt,
            panelId,
            onCreatedTarget: async (target) => {
              await runtime.storage.promptHistory.append({
                text: prompt.trim(),
                sentAt: Date.now(),
                workspaceId: runtime.workspaceId,
                sessionId: target.workspaceSessionId,
              });
            },
          });
          promptHistory = await runtime.storage.promptHistory.list(runtime.workspaceId);
        }),
      0,
    );
  }

  async function handleCreateSession() {
    await runSessionMutation(() => runtime.createSession({}, { kind: "new-panel", direction: "right" }));
  }

  async function handleCreateDumbSession() {
    await runSessionMutation(() =>
      runtime.createSession({ mode: "dumb" }, { kind: "new-panel", direction: "right" }),
    );
  }

  async function handleSelectCurrentSessionMode(mode: SessionMode) {
    if (currentSessionMode === mode) {
      return;
    }
    await runSessionMutation(() => runtime.setSessionMode(focusedPanelId, mode));
  }

  async function handleOpenSession(sessionId: string, event?: MouseEvent) {
    if (
      sessionId === activeSessionId &&
      currentSurface?.surface === "orchestrator" &&
      currentSurface.workspaceSessionId === sessionId
    ) {
      return;
    }
    await runSessionMutation(() => runtime.openSession(sessionId, getSidebarSessionOpenTarget(event)));
  }

  function handleRenameSession(session: WorkspaceSessionSummary) {
    if (session.titleGeneration?.renameLocked) {
      sidebarError = "Session title is being generated. Rename is temporarily locked.";
      return;
    }
    renameTarget = session;
    renameValue = session.title;
  }

  async function confirmRename() {
    if (!renameTarget) return;
    const target = renameTarget;
    const nextTitle = renameValue.trim();
    if (!nextTitle) {
      sidebarError = "Session title cannot be empty.";
      return;
    }

    await runSessionMutation(async () => {
      await runtime.renameSession(target.id, nextTitle);
      renameTarget = null;
      renameValue = "";
    });
  }

  async function handleResetSurfaceTarget() {
    const session = currentSession;
    if (!session) {
      return;
    }
    await runSessionMutation(() => runtime.openSession(session.id, { kind: "focused-panel" }));
  }

  async function handleFocusPane(panelId: string) {
    runtime.focusPane(panelId);
    syncRuntimeState();
    resubscribeSurfaceController();
    syncSurfaceState();
    await syncArtifactsFromRuntime(true);
  }

  async function handleOpenPaneModelPicker(panelId: string) {
    await handleFocusPane(panelId);
    await openModelSelector();
  }

  function handleTranscriptScrollState(panelId: string, scroll: { transcriptAnchorId: string | null; offsetPx: number }) {
    runtime.setPaneScroll(panelId, scroll);
  }

  async function handlePinSession(session: WorkspaceSessionSummary) {
    await runSessionMutation(() => runtime.pinSession(session.id));
  }

  async function handleUnpinSession(session: WorkspaceSessionSummary) {
    await runSessionMutation(() => runtime.unpinSession(session.id));
  }

  async function handleArchiveSession(session: WorkspaceSessionSummary) {
    await runSessionMutation(() => runtime.archiveSession(session.id));
  }

  async function handleUnarchiveSession(session: WorkspaceSessionSummary) {
    await runSessionMutation(() => runtime.unarchiveSession(session.id));
  }

  async function handleMarkSessionUnread(session: WorkspaceSessionSummary) {
    await runSessionMutation(() => runtime.markSessionUnread(session.id));
  }

  async function handleMarkSessionRead(session: WorkspaceSessionSummary) {
    await runSessionMutation(() => runtime.markSessionRead(session.id));
  }

  async function handleToggleArchivedGroup(collapsed: boolean) {
    await runSessionMutation(() => runtime.setArchivedGroupCollapsed(collapsed));
  }

  async function persistPromptHistoryEntry(input: string, target: PromptTarget | null = currentSurface) {
    try {
      const entry = await runtime.storage.promptHistory.append({
        text: input,
        sentAt: Date.now(),
        workspaceId: runtime.workspaceId,
        sessionId: target?.workspaceSessionId ?? currentSession?.id ?? "unknown-session",
      });
      promptHistory = [...promptHistory, entry];
    } catch (error) {
      console.error("Failed to persist prompt history:", error);
    }
  }

  async function handleSend(input: string): Promise<boolean> {
    return handleSendToPane(focusedPanelId, input);
  }

  async function handleSendToPane(panelId: string, input: string): Promise<boolean> {
    const surface = runtime.getPaneController(panelId);
    if (!input.trim() || !surface || surface.promptStatus === "streaming" || sendingPrompt) return false;

    sendingPrompt = true;
    try {
      if (panelId !== focusedPanelId) {
        await runtime.focusPane(panelId);
        syncRuntimeState();
        resubscribeSurfaceController();
        syncSurfaceState();
      }

      await persistPromptHistoryEntry(input, surface.target);

      const hasProviderAccess = await runtime.requireProviderAccess(surface.agent.state.model.provider);
      if (!hasProviderAccess) return false;

      await surface.sendPrompt(input);
      return true;
    } finally {
      sendingPrompt = false;
    }
  }

  function handleOpenArtifact(filename: string) {
    controller?.selectArtifact(filename);
    showArtifactsPanel = true;
  }

  async function handleOpenWorkspacePath(path: string) {
    try {
      const opened = await runtime.openWorkspacePath(path);
      if (!opened) {
        await copyTextToClipboard(path);
      }
    } catch (error) {
      console.error("Failed to open workspace path:", error);
      await copyTextToClipboard(path);
    }
  }

  async function handleCopyPaneTranscript(panelId: string) {
    if (copyTranscriptState.status === "copying") return;

    const paneController = runtime.getPaneController(panelId);
    const agent = paneController?.agent;
    const activeModel = agent?.state.model;
    if (!paneController || !agent || !activeModel) {
      return;
    }
    const session =
      sessions.find((candidate) => candidate.id === paneController.target.workspaceSessionId) ??
      currentSession;
    const exportText = buildSessionTranscriptExport({
      session: {
        id: session?.id ?? agent.sessionId ?? "unknown-session",
        title: getSurfaceDisplayTitle(
          paneController.target,
          sessions,
          session?.title ?? "New Session",
        ),
        status: session?.status ?? "idle",
        createdAt: session?.createdAt ?? new Date(0).toISOString(),
        updatedAt: session?.updatedAt ?? new Date().toISOString(),
      },
      target:
        paneController.target ?? {
          workspaceSessionId: session?.id ?? "unknown-session",
          surface: "orchestrator",
          surfacePiSessionId: agent.sessionId ?? "unknown-surface",
        },
      provider: activeModel.provider,
      model: activeModel.id,
      reasoningEffort: agent.state.thinkingLevel,
      systemPrompt: paneController.resolvedSystemPrompt,
      messages: agent.state.messages,
      streamMessage: agent.state.streamMessage?.role === "assistant" ? agent.state.streamMessage : null,
    });

    copyTranscriptState = { panelId, status: "copying" };

    try {
      await copyTextToClipboard(exportText);
      copyTranscriptState = { panelId, status: "copied" };
      scheduleCopyTranscriptReset();
    } catch (error) {
      console.error("Failed to copy transcript:", error);
      copyTranscriptState = { panelId, status: "error" };
      scheduleCopyTranscriptReset();
    }
  }

  function closeCommandInspector() {
    showCommandInspector = false;
    commandInspector = null;
    commandInspectorError = undefined;
    commandInspectorLoading = false;
    commandInspectorCommandId = null;
    commandInspectorSessionId = null;
  }

  function closeThreadInspector() {
    showThreadInspector = false;
    threadInspector = null;
    threadInspectorError = undefined;
    threadInspectorLoading = false;
    threadInspectorThreadId = null;
    threadInspectorSessionId = null;
    runtime.setPaneInspectorSelection(focusedPanelId, null);
  }

  function closeWorkflowTaskAttemptInspector() {
    showWorkflowTaskAttemptInspector = false;
    workflowTaskAttemptInspector = null;
    workflowTaskAttemptInspectorError = undefined;
    workflowTaskAttemptInspectorLoading = false;
    workflowTaskAttemptInspectorId = null;
    workflowTaskAttemptInspectorSessionId = null;
  }

  function getCommandStatusLabel(
    status: WorkspaceCommandRollup["status"] | WorkspaceCommandInspector["status"],
  ): string {
    return getWorkspaceCommandStatusPresentation(status).label;
  }

  function getCommandStatusTone(
    status: WorkspaceCommandRollup["status"] | WorkspaceCommandInspector["status"],
  ): string {
    return getWorkspaceCommandStatusPresentation(status).tone;
  }

  function getThreadStatusLabel(
    status:
      | WorkspaceHandlerThreadSummary["status"]
      | WorkspaceHandlerThreadInspector["status"]
      | WorkspaceHandlerThreadWorkflowSummary["status"],
  ): string {
    switch (status) {
      case "idle":
        return "Idle";
      case "running-handler":
        return "Handler Running";
      case "running-workflow":
        return "Workflow Running";
      case "waiting":
        return "Waiting";
      case "troubleshooting":
        return "Troubleshooting";
      case "completed":
        return "Completed";
      case "continued":
        return "Continued";
      case "failed":
        return "Failed";
      case "cancelled":
        return "Cancelled";
      default:
        return status;
    }
  }

  function getThreadStatusTone(
    status:
      | WorkspaceHandlerThreadSummary["status"]
      | WorkspaceHandlerThreadInspector["status"]
      | WorkspaceHandlerThreadWorkflowSummary["status"],
  ): "neutral" | "info" | "success" | "warning" | "danger" {
    switch (status) {
      case "idle":
        return "neutral";
      case "running-handler":
      case "running-workflow":
        return "info";
      case "waiting":
        return "info";
      case "completed":
        return "success";
      case "continued":
        return "neutral";
      case "troubleshooting":
      case "failed":
        return "danger";
      default:
        return "neutral";
    }
  }

  function getReferenceThreadStatus(status: WorkspaceHandlerThreadSummary["status"]): ReferenceStatus {
    switch (status) {
      case "idle":
        return "idle";
      case "running-handler":
      case "running-workflow":
        return "running";
      case "waiting":
        return "waiting";
      case "completed":
        return "done";
      case "troubleshooting":
        return "failed";
      default:
        return "idle";
    }
  }

  function getReferenceThread(thread: WorkspaceHandlerThreadSummary): ReferenceThread {
    return {
      id: thread.threadId,
      title: thread.title,
      objective: getHandlerThreadPreview(thread),
      status: getReferenceThreadStatus(thread.status),
      elapsed: formatTimestamp(thread.updatedAt),
      model: "handler-thread",
    };
  }

  function getReferenceWorkflowStatus(status: WorkspaceHandlerThreadWorkflowSummary["status"]): ReferenceStatus {
    switch (status) {
      case "running":
      case "continued":
        return "running";
      case "waiting":
        return "waiting";
      case "completed":
        return "done";
      case "failed":
      case "cancelled":
        return "failed";
      default:
        return "idle";
    }
  }

  function getReferenceWorkflowTaskAttemptStatus(
    status: WorkspaceWorkflowTaskAttemptSummary["status"],
  ): ReferenceStatus {
    switch (status) {
      case "running":
        return "running";
      case "waiting":
        return "waiting";
      case "completed":
        return "done";
      case "failed":
      case "cancelled":
        return "failed";
      default:
        return "idle";
    }
  }

  function getReferenceWorkflowTaskAgents(
    thread: WorkspaceHandlerThreadSummary,
  ): ReferenceSubagent[] {
    return (thread.workflowTaskAttempts ?? []).map((attempt) => ({
      id: attempt.workflowTaskAttemptId,
      type: "workflow-task-agent",
      headline: attempt.title,
      status: getReferenceWorkflowTaskAttemptStatus(attempt.status),
      model: "workflow-task-agent",
      tokens: attempt.contextBudget?.usedTokens,
    }));
  }

  function getReferenceWorkflow(run: WorkspaceHandlerThreadWorkflowSummary): ReferenceWorkflow {
    const status = getReferenceWorkflowStatus(run.status);
    return {
      id: run.workflowRunId,
      name: run.workflowName,
      status,
      elapsed: formatTimestamp(run.updatedAt),
      stepsDone: status === "done" ? 1 : 0,
      stepsTotal: 1,
      currentStep: run.summary,
      runId: run.workflowRunId,
    };
  }

  function getReferenceEpisode(
    episode: NonNullable<WorkspaceHandlerThreadSummary["latestEpisode"]>,
    thread: WorkspaceHandlerThreadSummary,
  ): ReferenceEpisode {
    return {
      id: episode.episodeId,
      title: episode.title,
      summary: episode.summary,
      thread: thread.title,
      verified: thread.status === "completed",
    };
  }

  function getThreadStateDetail(
    thread: WorkspaceHandlerThreadSummary | WorkspaceHandlerThreadInspector,
  ): string {
    switch (thread.status) {
      case "idle":
        return "Handler thread is ready for follow-up.";
      case "running-handler":
        return "Handler is actively reasoning or using thread-local tools.";
      case "running-workflow":
        return "Smithers workflow work is active under this handler.";
      case "waiting":
        return thread.wait
          ? `Waiting on ${thread.wait.owner} ${thread.wait.kind}: ${thread.wait.resumeWhen}`
          : "Waiting for an external prerequisite before continuing.";
      case "troubleshooting":
        return "Thread needs repair or follow-up before it can hand work back.";
      case "completed":
        return "Thread has handed durable output back to the orchestrator.";
      default:
        return thread.objective;
    }
  }

  function getProjectCiStatusLabel(status: WorkspaceProjectCiPanelStatus): string {
    switch (status) {
      case "not-configured":
        return "Not configured";
      case "configured":
        return "Configured";
      case "running":
        return "Running";
      case "passed":
        return "Passed";
      case "failed":
        return "Failed";
      case "blocked":
        return "Blocked";
      case "cancelled":
        return "Cancelled";
      default:
        return status;
    }
  }

  function getProjectCiStatusTone(
    status: WorkspaceProjectCiPanelStatus | WorkspaceProjectCiCheckSummary["status"],
  ): "neutral" | "info" | "success" | "warning" | "danger" {
    switch (status) {
      case "running":
        return "info";
      case "passed":
        return "success";
      case "configured":
      case "not-configured":
      case "skipped":
      case "cancelled":
        return "neutral";
      case "blocked":
        return "info";
      case "failed":
        return "danger";
      default:
        return "neutral";
    }
  }

  function formatProjectCiCommand(command: string[] | null): string | null {
    if (!command || command.length === 0) {
      return null;
    }

    return command.join(" ");
  }

  function formatProjectCiExitCode(exitCode: number | null): string | null {
    if (exitCode === null) {
      return null;
    }

    return `exit code ${exitCode}`;
  }

  function formatProjectCiCheckCounts(status: WorkspaceProjectCiStatusPanel): string {
    const counts = status.checkCounts;
    if (counts.total === 0) {
      return "No checks";
    }
    const failed = counts.failed + counts.blocked + counts.cancelled;
    if (failed > 0) {
      return `${counts.passed}/${counts.total} passed, ${failed} attention`;
    }
    return `${counts.passed}/${counts.total} passed`;
  }

  function getProjectCiVerification(status: WorkspaceProjectCiStatusPanel): ReferenceVerification {
    const testsPassed = status.checkCounts.passed;
    const testsTotal = status.checkCounts.total;
    return {
      id: status.latestRun?.ciRunId ?? status.updatedAt ?? "project-ci",
      passed: status.status === "passed",
      testsPassed,
      testsTotal,
      summary: status.summary,
      checks: status.checks.map((check) => ({
        label: check.label,
        status:
          check.status === "passed"
            ? "pass"
            : check.status === "skipped"
              ? "skip"
              : check.status === "blocked"
                ? "blocked"
                : check.status === "cancelled"
                  ? "cancelled"
              : "fail",
      })),
    };
  }

  function handleInspectLatestProjectCiRun() {
    if (!projectCiStatus?.latestRun) {
      return;
    }
    void openWorkflowInspector(projectCiStatus.latestRun.workflowRunId);
  }

  function openWorkflowInspector(workflowRunId: string, sessionId = activeSessionId): void {
    if (!sessionId) return;
    void runtime.openSurface(
      {
        workspaceSessionId: sessionId,
        surface: "workflow-inspector",
        workflowRunId,
      },
      { kind: "split", panelId: focusedPanelId, direction: "right" },
    );
  }

  function openSavedWorkflowLibrary(sessionId = activeSessionId): void {
    if (!sessionId) return;
    void runtime.openSurface(
      {
        workspaceSessionId: sessionId,
        surface: "saved-workflow-library",
      },
      { kind: "split", panelId: focusedPanelId, direction: "right" },
    );
  }

  function openAppLogs(): void {
    void runtime.openSurface(
      {
        surface: "app-logs",
      },
      { kind: "split", panelId: focusedPanelId, direction: "right" },
    );
    void runtime.markAppLogsSeen(runtime.appLogSummary.latestSeq);
  }

  function getWorkflowTaskAttemptStatusLabel(
    status: WorkspaceWorkflowTaskAttemptSummary["status"] | WorkspaceWorkflowTaskAttemptInspector["status"],
  ): string {
    switch (status) {
      case "running":
        return "Running";
      case "waiting":
        return "Waiting";
      case "completed":
        return "Completed";
      case "failed":
        return "Failed";
      case "cancelled":
        return "Cancelled";
      default:
        return status;
    }
  }

  function getWorkflowTaskAttemptStatusTone(
    status: WorkspaceWorkflowTaskAttemptSummary["status"] | WorkspaceWorkflowTaskAttemptInspector["status"],
  ): "neutral" | "info" | "success" | "warning" | "danger" {
    switch (status) {
      case "running":
        return "info";
      case "waiting":
        return "info";
      case "completed":
        return "success";
      case "failed":
        return "danger";
      case "cancelled":
        return "neutral";
      default:
        return "neutral";
    }
  }

  function getHandlerThreadPreview(
    thread: WorkspaceHandlerThreadSummary | WorkspaceHandlerThreadInspector,
  ): string {
    if (thread.wait) {
      return thread.wait.reason;
    }

    if (thread.latestEpisode) {
      return thread.latestEpisode.summary;
    }

    if (thread.latestWorkflowRun) {
      return thread.latestWorkflowRun.summary;
    }

    return thread.objective;
  }

  function getHandlerThreadArtifactLabel(
    thread: WorkspaceHandlerThreadSummary | WorkspaceHandlerThreadInspector,
  ): string | null {
    if (thread.artifactCount <= 0) {
      return null;
    }
    return `${thread.artifactCount} ${thread.artifactCount === 1 ? "artifact" : "artifacts"}`;
  }

  function formatCommandFacts(facts: Record<string, unknown> | null | undefined): string | null {
    if (!facts || Object.keys(facts).length === 0) {
      return null;
    }

    return JSON.stringify(facts, null, 2);
  }

  function canOpenArtifactLink(artifact: WorkspaceCommandArtifactLink): boolean {
    return (
      artifactsSnapshot.artifacts.some((record) => record.filename === artifact.name) ||
      !artifact.missingFile
    );
  }

  function getInspectorSelectionKey(selection: WorkspaceInspectorSelection | null | undefined): string | null {
    if (!selection) {
      return null;
    }
    switch (selection.kind) {
      case "thread":
        return `thread:${selection.threadId}`;
      case "workflow-run":
        return `workflow-run:${selection.workflowRunId}`;
      case "artifact":
        return `artifact:${selection.artifactId}`;
      case "ci-run":
        return `ci-run:${selection.ciRunId}`;
    }
  }

  async function openStructuredArtifact(
    artifactId: string,
    sessionId: string,
    options: { persistSelection?: boolean } = {},
  ) {
    if (!controller) {
      return;
    }

    const preview = await runtime.getArtifactPreview(artifactId, sessionId);
    if (preview.missingFile && !preview.content) {
      sidebarError = `Artifact file is missing: ${preview.name}`;
      return;
    }

    controller.upsertExternalArtifact({
      filename: preview.name,
      content: preview.content,
      createdAt: Date.parse(preview.createdAt),
      updatedAt: Date.now(),
    });
    showArtifactsPanel = true;
    if (options.persistSelection ?? true) {
      runtime.setPaneInspectorSelection(focusedPanelId, { kind: "artifact", artifactId });
    }
  }

  async function handleOpenStructuredArtifact(artifact: WorkspaceCommandArtifactLink) {
    const session = currentSession;
    if (!session) {
      return;
    }

    try {
      await openStructuredArtifact(artifact.artifactId, session.id);
    } catch (error) {
      sidebarError = error instanceof Error ? error.message : "Unable to open this artifact.";
    }
  }

  async function handleInspectCommand(commandId: string) {
    const session = currentSession;
    if (!session) {
      return;
    }

    showCommandInspector = true;
    commandInspector = null;
    commandInspectorError = undefined;
    commandInspectorLoading = true;
    commandInspectorCommandId = commandId;
    commandInspectorSessionId = session.id;

    try {
      const inspector = await runtime.getCommandInspector(commandId, session.id);
      if (commandInspectorCommandId !== commandId || commandInspectorSessionId !== session.id) {
        return;
      }

      commandInspector = inspector;
    } catch (error) {
      if (commandInspectorCommandId !== commandId || commandInspectorSessionId !== session.id) {
        return;
      }

      commandInspectorError =
        error instanceof Error ? error.message : "Unable to inspect this command.";
    } finally {
      if (commandInspectorCommandId === commandId && commandInspectorSessionId === session.id) {
        commandInspectorLoading = false;
      }
    }
  }

  function handleOpenHandlerThread(
    thread: Pick<WorkspaceHandlerThreadSummary, "threadId" | "surfacePiSessionId">,
  ) {
    const session = currentSession;
    if (!session) {
      return;
    }

    closeThreadInspector();
    setTimeout(() => {
      void runSessionMutation(() =>
        runtime.openSurface(
          {
            workspaceSessionId: session.id,
            surface: "thread",
            surfacePiSessionId: thread.surfacePiSessionId,
            threadId: thread.threadId,
          },
          { kind: "new-panel", direction: "right" },
        ),
      );
    }, 0);
  }

  function handleOpenSidebarHandlerThread(
    sessionId: string,
    thread: Pick<WorkspaceSidebarHandlerThreadRow, "threadId" | "surfacePiSessionId">,
  ) {
    void runSessionMutation(() =>
      runtime.openSurface(
        {
          workspaceSessionId: sessionId,
          surface: "thread",
          surfacePiSessionId: thread.surfacePiSessionId,
          threadId: thread.threadId,
        },
        { kind: "new-panel", direction: "right" },
      ),
    );
  }

  function handleOpenSidebarWorkflowRun(
    sessionId: string,
    workflow: Pick<WorkspaceSidebarWorkflowRow, "workflowRunId">,
  ) {
    openWorkflowInspector(workflow.workflowRunId, sessionId);
  }

  async function sendPromptToHandlerThread(
    thread: Pick<WorkspaceHandlerThreadSummary, "threadId" | "surfacePiSessionId">,
    prompt: string,
  ) {
    const session = currentSession;
    if (!session) {
      return;
    }

    const target = {
      workspaceSessionId: session.id,
      surface: "thread" as const,
      surfacePiSessionId: thread.surfacePiSessionId,
      threadId: thread.threadId,
    };

    await runSessionMutation(async () => {
      await runtime.openSurface(target, focusedPanelId);
      await runtime.sendPromptToTarget(target, prompt);
    });
  }

  async function handleReplyToWait(
    block: TranscriptSemanticBlock & { kind: "wait" },
    text: string,
  ) {
    const targetThread = block.threadId
      ? handlerThreads.find((thread) => thread.threadId === block.threadId)
      : null;

    if (targetThread) {
      await sendPromptToHandlerThread(targetThread, text);
      return;
    }

    const target = currentSurfaceController?.target;
    if (!target) {
      return;
    }

    await runSessionMutation(() => runtime.sendPromptToTarget(target, text));
  }

  async function handleRetryFailure(block: TranscriptSemanticBlock & { kind: "failure" }) {
    const target = currentSurfaceController?.target;
    if (!target) {
      return;
    }

    await runSessionMutation(() =>
      runtime.sendPromptToTarget(
        target,
        `Retry the failed turn and address this failure:\n\n${block.summary}`,
      ),
    );
  }

  async function loadHandlerThreadInspector(threadId: string, sessionId: string) {
    try {
      const inspector = await runtime.getHandlerThreadInspector(threadId, sessionId);
      if (threadInspectorThreadId !== threadId || threadInspectorSessionId !== sessionId) {
        return;
      }

      threadInspector = inspector;
    } catch (error) {
      if (threadInspectorThreadId !== threadId || threadInspectorSessionId !== sessionId) {
        return;
      }

      threadInspectorError =
        error instanceof Error ? error.message : "Unable to inspect this handler thread.";
    } finally {
      if (threadInspectorThreadId === threadId && threadInspectorSessionId === sessionId) {
        threadInspectorLoading = false;
      }
    }
  }

  function handleInspectHandlerThread(thread: WorkspaceHandlerThreadSummary) {
    const session = currentSession;
    if (!session) {
      return;
    }

    runtime.setPaneInspectorSelection(focusedPanelId, { kind: "thread", threadId: thread.threadId });
    showThreadInspector = true;
    threadInspector = null;
    threadInspectorError = undefined;
    threadInspectorLoading = true;
    threadInspectorThreadId = thread.threadId;
    threadInspectorSessionId = session.id;

    setTimeout(() => {
      void loadHandlerThreadInspector(thread.threadId, session.id);
    }, 0);
  }

  async function restoreHandlerThreadInspector(threadId: string, sessionId: string) {
    showThreadInspector = true;
    threadInspector = null;
    threadInspectorError = undefined;
    threadInspectorLoading = true;
    threadInspectorThreadId = threadId;
    threadInspectorSessionId = sessionId;

    try {
      const inspector = await runtime.getHandlerThreadInspector(threadId, sessionId);
      if (threadInspectorThreadId !== threadId || threadInspectorSessionId !== sessionId) {
        return;
      }
      threadInspector = inspector;
    } catch {
      if (threadInspectorThreadId === threadId && threadInspectorSessionId === sessionId) {
        closeThreadInspector();
      }
    } finally {
      if (threadInspectorThreadId === threadId && threadInspectorSessionId === sessionId) {
        threadInspectorLoading = false;
      }
    }
  }

  async function handleInspectThreadCommand(commandId: string) {
    closeThreadInspector();
    closeWorkflowTaskAttemptInspector();
    await handleInspectCommand(commandId);
  }

  async function handleAskHandlerToSaveWorkflow(
    thread: Pick<WorkspaceHandlerThreadSummary, "threadId" | "surfacePiSessionId">,
  ) {
    const session = currentSession;
    if (!session) {
      return;
    }

      const prompt = [
        "Inspect the workflow work owned by this thread.",
        "If there are reusable saved workflow files worth keeping, write them directly into `.svvy/workflows/...` using the direct write or edit tools.",
        "Rely on the automatic workflow validation feedback returned in structured tool output, and keep editing until the final saved workflow state validates cleanly.",
        "If nothing here is worth saving, say so briefly inside the thread.",
      ].join(" ");

    await sendPromptToHandlerThread(thread, prompt);
  }

  async function handleInspectWorkflowTaskAttempt(
    workflowTaskAttempt: Pick<WorkspaceWorkflowTaskAttemptSummary, "workflowTaskAttemptId">,
    sessionId = currentSession?.id,
  ) {
    if (!sessionId) {
      return;
    }

    showWorkflowTaskAttemptInspector = true;
    workflowTaskAttemptInspector = null;
    workflowTaskAttemptInspectorError = undefined;
    workflowTaskAttemptInspectorLoading = true;
    workflowTaskAttemptInspectorId = workflowTaskAttempt.workflowTaskAttemptId;
    workflowTaskAttemptInspectorSessionId = sessionId;

    try {
      const inspector = await runtime.getWorkflowTaskAttemptInspector(
        workflowTaskAttempt.workflowTaskAttemptId,
        sessionId,
      );
      if (
        workflowTaskAttemptInspectorId !== workflowTaskAttempt.workflowTaskAttemptId ||
        workflowTaskAttemptInspectorSessionId !== sessionId
      ) {
        return;
      }

      workflowTaskAttemptInspector = inspector;
    } catch (error) {
      if (
        workflowTaskAttemptInspectorId !== workflowTaskAttempt.workflowTaskAttemptId ||
        workflowTaskAttemptInspectorSessionId !== sessionId
      ) {
        return;
      }

      workflowTaskAttemptInspectorError =
        error instanceof Error ? error.message : "Unable to inspect this workflow task attempt.";
    } finally {
      if (
        workflowTaskAttemptInspectorId === workflowTaskAttempt.workflowTaskAttemptId &&
        workflowTaskAttemptInspectorSessionId === sessionId
      ) {
        workflowTaskAttemptInspectorLoading = false;
      }
    }
  }

  async function handleInspectThreadWorkflowTaskAttempt(
    workflowTaskAttempt: WorkspaceWorkflowTaskAttemptSummary,
  ) {
    closeThreadInspector();
    await handleInspectWorkflowTaskAttempt(workflowTaskAttempt);
  }

  async function handleInspectCommandWorkflowTaskAttempt(workflowTaskAttemptId: string) {
    closeCommandInspector();
    closeThreadInspector();
    await handleInspectWorkflowTaskAttempt({ workflowTaskAttemptId });
  }

  $effect(() => {
    const sessionId = currentSession?.id ?? null;
    if (!commandInspectorSessionId || !sessionId || sessionId === commandInspectorSessionId) {
      return;
    }

    closeCommandInspector();
  });

  $effect(() => {
    const sessionId = currentSession?.id ?? null;
    if (!threadInspectorSessionId || !sessionId || sessionId === threadInspectorSessionId) {
      return;
    }

    closeThreadInspector();
  });

  $effect(() => {
    const sessionId = currentSession?.id ?? null;
    if (
      !workflowTaskAttemptInspectorSessionId ||
      !sessionId ||
      sessionId === workflowTaskAttemptInspectorSessionId
    ) {
      return;
    }

    closeWorkflowTaskAttemptInspector();
  });

  $effect(() => {
    const session = currentSession;
    if (!session) {
      projectCiStatus = null;
      projectCiError = undefined;
      return;
    }

    const loadToken = ++projectCiLoadToken;
    projectCiError = undefined;
    projectCiStatus = null;
    void runtime
      .getProjectCiStatus(session.id)
      .then((status) => {
        if (loadToken !== projectCiLoadToken) {
          return;
        }

        projectCiStatus = status;
      })
      .catch((error) => {
        if (loadToken !== projectCiLoadToken) {
          return;
        }

        projectCiError =
          error instanceof Error ? error.message : "Unable to load Project CI status.";
        projectCiStatus = null;
      })
  });

  $effect(() => {
    const selection = currentPane?.inspectorSelection ?? null;
    const sessionId = currentPane?.target?.workspaceSessionId ?? null;
    const key = getInspectorSelectionKey(selection);
    if (!selection) {
      restoredInspectorKey = null;
      return;
    }
    if (!sessionId || key === restoredInspectorKey) {
      return;
    }

    if (selection.kind === "ci-run") {
      if (!projectCiStatus && !projectCiError) {
        return;
      }
      restoredInspectorKey = key;
      if (projectCiStatus?.latestRun?.ciRunId !== selection.ciRunId) {
        runtime.setPaneInspectorSelection(focusedPanelId, null);
      }
      return;
    }

    restoredInspectorKey = key;
    if (selection.kind === "thread") {
      void restoreHandlerThreadInspector(selection.threadId, sessionId);
      return;
    }

    if (selection.kind === "workflow-run") {
      return;
    }

    if (selection.kind === "artifact") {
      void openStructuredArtifact(selection.artifactId, sessionId, { persistSelection: false }).catch(() => {
        runtime.setPaneInspectorSelection(focusedPanelId, null);
      });
    }
  });

  $effect(() => {
    const session = currentSession;
    const surface = currentSurface?.surface;
    if (!session || surface !== "orchestrator") {
      handlerThreads = [];
      handlerThreadsError = undefined;
      handlerThreadsLoading = false;
      return;
    }

    const loadToken = ++handlerThreadLoadToken;
    handlerThreadsLoading = true;
    handlerThreadsError = undefined;
    void runtime
      .listHandlerThreads(session.id)
      .then((nextThreads) => {
        if (loadToken !== handlerThreadLoadToken) {
          return;
        }

        handlerThreads = nextThreads;
      })
      .catch((error) => {
        if (loadToken !== handlerThreadLoadToken) {
          return;
        }

        handlerThreadsError =
          error instanceof Error ? error.message : "Unable to load delegated handler threads.";
        handlerThreads = [];
      })
      .finally(() => {
        if (loadToken === handlerThreadLoadToken) {
          handlerThreadsLoading = false;
        }
      });
  });

  function syncSurfaceTools() {
    if (!controller || !currentSurfaceController) {
      return;
    }

    currentSurfaceController.agent.state.tools = [controller.tool];
  }

  function syncSurfaceState() {
    const surface = currentSurfaceController;
    if (!surface) {
      messages = [];
      streamMessage = null;
      pendingToolCalls = new Set();
      isStreaming = false;
      errorMessage = undefined;
      currentModel = null;
      currentThinkingLevel = "off";
      currentSessionMode = "orchestrator";
      return;
    }

    const nextMessages = [...surface.agent.state.messages];
    messages = nextMessages;
    streamMessage =
      surface.agent.state.streamMessage?.role === "assistant"
        ? surface.agent.state.streamMessage
        : null;
    pendingToolCalls = new Set(surface.agent.state.pendingToolCalls);
    isStreaming = surface.agent.state.isStreaming || surface.promptStatus === "streaming";
    errorMessage = surface.agent.state.error ?? getLatestAssistantFailureMessage(nextMessages);
    currentModel = surface.agent.state.model;
    currentThinkingLevel = surface.agent.state.thinkingLevel as ThinkingLevel;
    currentSessionMode = surface.sessionMode;
  }

  function syncRuntimeState() {
    sessions = [...runtime.sessions];
    workspaceBranch = runtime.branch;
    sessionNavigation = runtime.sessionNavigation;
    appLogSummary = runtime.appLogSummary;
    paneLayout = runtime.paneLayout;
    activeLayoutId = runtime.activeLayoutId;
    layoutSlots = runtime.layoutSlots;
    focusedPanelId = paneLayout.focusedPanelId ?? PRIMARY_CHAT_PANE_ID;
    currentPane = runtime.getPane(focusedPanelId) ?? null;
    focusedSurfaceTarget = currentPane?.target ?? null;
    activeSessionId = currentPane?.target?.workspaceSessionId;
    currentSurfaceController = runtime.getPaneController(focusedPanelId);
  }

  function resubscribeSurfaceController() {
    unsubscribeSurfaceController?.();
    unsubscribeSurfaceController = null;
    if (!currentSurfaceController) {
      syncSurfaceState();
      return;
    }

    syncSurfaceTools();
    unsubscribeSurfaceController = currentSurfaceController.subscribe(() => {
      focusedSurfaceTarget = currentSurfaceController?.target ?? null;
      activeSessionId = currentSurfaceController?.target.workspaceSessionId;
      syncSurfaceTools();
      syncSurfaceState();
      void syncArtifactsFromRuntime();
    });
  }

  syncRuntimeState();
  syncSurfaceState();

  onMount(() => {
    windowWidth = window.innerWidth;
    isMacWindowChrome = navigator.platform.toLowerCase().includes("mac");
    const nextController = new ArtifactsController();
    controller = nextController;
    const handleResize = () => {
      windowWidth = window.innerWidth;
    };
    const handleAppMenuMessage = ({ action }: { action: AppMenuAction }) => {
      handleAppMenuAction(action);
    };
    window.addEventListener("resize", handleResize);
    const unsubscribeAppMenuAction = runtime.subscribeAppMenuAction(handleAppMenuMessage);

    syncSurfaceTools();
    void runtime.storage.promptHistory
      .list(runtime.workspaceId)
      .then((entries) => {
        promptHistory = entries;
      })
      .catch((error) => {
        console.error("Failed to load prompt history:", error);
      });
    void runtime
      .listWorkspacePaths()
      .then((paths) => {
        workspaceMentionPaths = new Set(paths.map((path) => path.workspaceRelativePath));
      })
      .catch((error) => {
        console.error("Failed to load workspace mention paths:", error);
      });

    const unsubscribeRuntime = runtime.subscribe(() => {
      syncRuntimeState();
      resubscribeSurfaceController();
      syncSurfaceState();
      void syncArtifactsFromRuntime();
    });
    const unsubscribeArtifacts = nextController.subscribe((snapshot) => {
      syncArtifacts(snapshot);
    });
    resubscribeSurfaceController();
    void syncArtifactsFromRuntime(true);

    return () => {
      unsubscribeRuntime();
      unsubscribeArtifacts();
      unsubscribeSurfaceController?.();
      nextController.dispose();
      setSidebarResizing(false);
      clearCopyTranscriptResetTimer();
      if (dockviewLayoutSyncFrame !== null) {
        window.cancelAnimationFrame(dockviewLayoutSyncFrame);
        dockviewLayoutSyncFrame = null;
      }
      if (dockviewLayoutSyncTimer) {
        clearTimeout(dockviewLayoutSyncTimer);
        dockviewLayoutSyncTimer = null;
      }
      window.removeEventListener("resize", handleResize);
      unsubscribeAppMenuAction();
      controller = null;
    };
  });
</script>

<div class={`workspace-shell ${isMacWindowChrome ? "mac-window-chrome" : ""}`.trim()} style={`--sidebar-width: ${effectiveSidebarWidth}px;`}>
  <header
    class={`workspace-titlebar electrobun-webkit-app-region-drag ${sidebarHidden ? "sidebar-titlebar-hidden" : ""}`.trim()}
  >
    <div class="workspace-titlebar-start">
      <Tooltip
        class="electrobun-webkit-app-region-no-drag"
        label={sidebarHidden ? "Show sidebar" : "Hide sidebar"}
        shortcut={sidebarToggleShortcut}
        side="bottom"
      >
        <button
          class="titlebar-icon electrobun-webkit-app-region-no-drag"
          type="button"
          aria-pressed={!sidebarHidden}
          aria-label={sidebarHidden ? "Show sidebar" : "Hide sidebar"}
          onclick={toggleSidebarVisibility}
        >
          <span class="titlebar-icon-glyph">
            {#if sidebarHidden}
              <PanelLeftDashedIcon aria-hidden="true" size={14} strokeWidth={1.85} />
            {:else}
              <PanelLeftIcon aria-hidden="true" size={14} strokeWidth={1.85} />
            {/if}
          </span>
        </button>
      </Tooltip>
      <p class="workspace-titlebar-title">svvy</p>
    </div>
    <div class="workspace-titlebar-tabs">
      <WorkspaceTabStrip
        tabs={workspaceTabs}
        {activeWorkspaceId}
        {openingWorkspace}
        onSelectWorkspace={(workspaceId) => onSelectWorkspace?.(workspaceId)}
        onCloseWorkspace={(workspaceId) => onCloseWorkspace?.(workspaceId)}
        onOpenWorkspace={() => onOpenWorkspace?.()}
        onReorderWorkspace={(workspaceId, beforeWorkspaceId) =>
          onReorderWorkspace?.(workspaceId, beforeWorkspaceId)}
      />
    </div>
    <div class="workspace-titlebar-actions electrobun-webkit-app-region-no-drag">
      <div class="workspace-main-meta" role="toolbar" aria-label="Workspace actions">
        {#if projectCiStatus && hasActionableProjectCiStatus}
          <div class="project-ci-compact" aria-label="Project CI summary">
            <Badge tone={getProjectCiStatusTone(projectCiStatus.status)}>
              CI {getProjectCiStatusLabel(projectCiStatus.status)}
            </Badge>
            <span>{projectCiStatus.summary}</span>
            <span>{formatProjectCiCheckCounts(projectCiStatus)}</span>
            {#if projectCiStatus.latestRun}
              <Button
                variant="ghost"
                size="sm"
                onclick={handleInspectLatestProjectCiRun}
              >
                Inspect
              </Button>
            {/if}
          </div>
        {/if}
      </div>
      <div class="workspace-layout-switcher" role="tablist" aria-label="Workspace layouts">
        {#each layoutSlots as slot (slot.id)}
          <Tooltip
            label={`Layout ${slot.id}: ${slot.initialized ? "switch to this saved pane arrangement" : "start a new pane arrangement"}`}
            side="bottom"
          >
            <button
              type="button"
              role="tab"
              aria-label={`Layout ${slot.id}: ${slot.initialized ? "switch to this saved pane arrangement" : "start a new pane arrangement"}`}
              aria-selected={slot.id === activeLayoutId}
              class={`workspace-layout-tab ${slot.id === activeLayoutId ? "active" : ""} ${slot.initialized ? "initialized" : "empty"}`.trim()}
              onclick={() => void handleSwitchLayout(slot.id)}
            >
              {slot.id}
            </button>
          </Tooltip>
        {/each}
      </div>
    </div>
  </header>

  <div
    class={`chat-workspace ${showDesktopSplit ? "split" : ""} ${sidebarHidden ? "sidebar-hidden" : ""} viewport-${viewportClass}`.trim()}
    style={`--sidebar-width: ${effectiveSidebarWidth}px;`}
    ontransitionend={handleWorkspaceTransitionEnd}
  >
    {#if !narrowShell}
      <aside class="workspace-sidebar" aria-hidden={sidebarHidden} inert={sidebarHidden}>
        <div class="sidebar-surface">
          <SessionSidebar
            workspaceLabel={runtime.workspaceLabel}
            {workspaceBranch}
            navigation={sessionNavigation}
            {activeSessionId}
            activeOrchestratorSessionId={currentSurface?.surface === "orchestrator" ? activeSessionId : undefined}
            activeThreadId={currentSurface?.threadId}
            {paneLocationsBySessionId}
            {paneLocationsByThreadId}
            {paneLocationsByWorkflowRunId}
            {appLogSummary}
            busy={mutatingSession}
            errorMessage={sidebarError}
            onCreateSession={handleCreateSession}
            onCreateDumbSession={handleCreateDumbSession}
            onOpenSession={handleOpenSession}
            onOpenHandlerThread={handleOpenSidebarHandlerThread}
            onOpenWorkflowRun={handleOpenSidebarWorkflowRun}
            onRenameSession={handleRenameSession}
            onPinSession={handlePinSession}
            onUnpinSession={handleUnpinSession}
            onArchiveSession={handleArchiveSession}
            onUnarchiveSession={handleUnarchiveSession}
            onMarkSessionUnread={handleMarkSessionUnread}
            onMarkSessionRead={handleMarkSessionRead}
            onToggleArchivedGroup={handleToggleArchivedGroup}
            onOpenSearch={() => openPalette("search")}
            onOpenCommandPalette={() => openPalette("commands")}
            onOpenAppLogs={openAppLogs}
            onOpenWorkflowLibrary={() => openSavedWorkflowLibrary()}
            onOpenSettings={onOpenSettings}
            onListWorkspaceBranches={runtime.listWorkspaceBranches}
            onSwitchWorkspaceBranch={handleSwitchWorkspaceBranch}
          />
        </div>
      </aside>
    {/if}

    {#if !sidebarHidden && !narrowShell}
      <div
        bind:this={sidebarResizeHandle}
        class={`sidebar-resize-handle ${sidebarResizing ? "dragging" : ""}`.trim()}
        role="separator"
        aria-orientation="vertical"
        aria-label="Resize session sidebar"
        aria-valuemin={MIN_SIDEBAR_WIDTH}
        aria-valuemax={getMaxSidebarWidth(windowWidth)}
        aria-valuenow={effectiveSidebarWidth}
        onpointerdown={startSidebarResize}
        onpointermove={handleSidebarResizeMove}
        onpointerup={stopSidebarResize}
        onpointercancel={stopSidebarResize}
        onlostpointercapture={() => stopSidebarResize()}
      ></div>
    {/if}

    <section class="workspace-main">
      <header class="workspace-main-header" aria-hidden="true"></header>
      <DockviewWorkspace
        {runtime}
        panels={paneLayout.panels}
        dockviewLayout={paneLayout.dockview}
        focusedPanelId={focusedPanelId}
        layoutEpoch={dockviewLayoutEpoch}
        onFocusPanel={(panelId) => void handleFocusPane(panelId)}
        onOpenModelPicker={(panelId) => void handleOpenPaneModelPicker(panelId)}
        onPersistDockview={(dockview, panelId) => runtime.setDockviewLayout(dockview, panelId)}
      />
    </section>

    {#if controller && hasArtifacts}
      {#if showDesktopSplit}
        <aside class="artifacts-slot desktop-open">
          <ArtifactsPanel
            {controller}
            snapshot={artifactsSnapshot}
            onClose={() => (showArtifactsPanel = false)}
          />
        </aside>
      {/if}

      {#if showOverlayArtifacts}
        <aside class="artifacts-slot mobile-slot">
          <div class="mobile-overlay">
            <ArtifactsPanel
              {controller}
              snapshot={artifactsSnapshot}
              overlay
              onClose={() => (showArtifactsPanel = false)}
            />
          </div>
        </aside>
      {/if}
    {/if}
  </div>

</div>

<CommandPalette
  open={paletteOpen}
  initialInput={paletteInitialInput}
  actions={visibleCommandActions}
  busy={paletteBusy}
  errorMessage={paletteError}
  onClose={closePalette}
  onExecute={(action, event) => void handlePaletteExecute(action, event)}
  onFallbackPrompt={(prompt, event) => void handlePaletteFallbackPrompt(prompt, event)}
/>

{#if showModelPicker}
  <ModelPickerDialog
    currentModel={currentModel}
    allowedProviders={allowedProviders}
    storage={runtime.storage}
    onClose={() => (showModelPicker = false)}
    onSelect={(model) => {
      currentModel = model;
      currentSurfaceController?.agent.setModel(model);
      showModelPicker = false;
    }}
  />
{/if}

{#if renameTarget}
  <Dialog
    eyebrow="Session"
    title="Rename Session"
    description="Update the durable session name used throughout the workspace navigator."
    width="md"
    onClose={() => {
      renameTarget = null;
      renameValue = "";
    }}
  >
    <div class="session-dialog">
      <Input bind:value={renameValue} placeholder="Session title" />
      <div class="session-dialog-actions">
        <Button variant="ghost" size="sm" onclick={() => {
          renameTarget = null;
          renameValue = "";
        }}>
          Cancel
        </Button>
        <Button variant="primary" size="sm" onclick={() => void confirmRename()} disabled={mutatingSession}>
          Save
        </Button>
      </div>
    </div>
  </Dialog>
{/if}

{#if showThreadInspector}
  <Dialog
    eyebrow="Handler Thread"
    title={threadInspector?.title ?? "Inspect Handler Thread"}
    description="Inspect the delegated thread state, handoff history, workflow runs, artifacts, and command rollups without making full thread inspection the default orchestrator reconciliation path."
    width="lg"
    onClose={closeThreadInspector}
  >
    <div class="thread-inspector">
      {#if threadInspectorLoading}
        <p class="thread-inspector-empty">Loading delegated thread detail…</p>
      {:else if threadInspectorError}
        <p class="thread-inspector-empty error">{threadInspectorError}</p>
      {:else if threadInspector}
        <section class="thread-inspector-summary">
          <div class="thread-inspector-summary-top">
            <div class="thread-inspector-summary-copy">
              <strong>{threadInspector.title}</strong>
              <p>{threadInspector.objective}</p>
            </div>
            <div class="thread-inspector-summary-meta">
              <Badge tone={getThreadStatusTone(threadInspector.status)}>
                {getThreadStatusLabel(threadInspector.status)}
              </Badge>
              <span>{formatTimestamp(threadInspector.updatedAt)}</span>
            </div>
          </div>

          <div class="thread-inspector-pills">
            <span>
              {threadInspector.workflowRunCount}
              {threadInspector.workflowRunCount === 1 ? " workflow" : " workflows"}
            </span>
            <span>
              {threadInspector.episodeCount}
              {threadInspector.episodeCount === 1 ? " handoff" : " handoffs"}
            </span>
            <span>
              {threadInspector.commandCount}
              {threadInspector.commandCount === 1 ? " command" : " commands"}
            </span>
            {#if (threadInspector.workflowTaskAttemptCount ?? 0) > 0}
              <span>
                {threadInspector.workflowTaskAttemptCount}
                {threadInspector.workflowTaskAttemptCount === 1 ? " task attempt" : " task attempts"}
              </span>
            {/if}
            {#if threadInspector.ciRunCount > 0}
              <span>
                {threadInspector.ciRunCount}
                {threadInspector.ciRunCount === 1 ? " CI run" : " CI runs"}
              </span>
            {/if}
            {#if threadInspector.loadedContextKeys.length > 0}
              <span>Context {threadInspector.loadedContextKeys.join(", ")}</span>
            {/if}
            <span>{threadInspector.threadId}</span>
          </div>

            {#if threadInspector.wait}
            <article class="thread-inspector-wait">
              <strong>Waiting on {threadInspector.wait.owner} {threadInspector.wait.kind}</strong>
              <span>Since {formatTimestamp(threadInspector.wait.since)}</span>
              <p>{threadInspector.wait.reason}</p>
              <p>{threadInspector.wait.resumeWhen}</p>
            </article>
          {/if}

          <div class="thread-inspector-metadata" aria-label="Thread runtime metadata">
            <div>
              <span>Surface</span>
              <strong>{threadInspector.surfacePiSessionId}</strong>
            </div>
            <div>
              <span>System prompt</span>
              <strong>Open the thread surface to inspect the active pi system prompt.</strong>
            </div>
            <div>
              <span>Workflow ownership</span>
              <strong>
                {threadInspector.workflowRunCount}
                {threadInspector.workflowRunCount === 1 ? " run" : " runs"}
                owned by this handler
              </strong>
            </div>
            <div>
              <span>Context packs</span>
              <strong>{threadInspector.loadedContextKeys.length > 0 ? threadInspector.loadedContextKeys.join(", ") : "none loaded"}</strong>
            </div>
          </div>

          {#if threadInspector.latestEpisode}
            <div class="thread-inspector-highlight">
              <span>Latest handoff</span>
              <p>{threadInspector.latestEpisode.summary}</p>
            </div>
          {/if}

          {#if threadInspector.latestWorkflowRun}
            <div class="thread-inspector-highlight">
              <span>Latest workflow</span>
              <p>{threadInspector.latestWorkflowRun.summary}</p>
            </div>
          {/if}

          {#if threadLocalProjectCiRun}
            <div class="thread-inspector-highlight">
              <span>Project CI</span>
              <p>{threadLocalProjectCiRun.summary}</p>
              {#if projectCiStatus}
                <p>{formatProjectCiCheckCounts(projectCiStatus)}</p>
              {/if}
            </div>
          {/if}

          <div class="thread-inspector-actions">
            <Button
              variant="primary"
              size="sm"
              disabled={promptBusy || mutatingSession}
              onclick={() => void handleOpenHandlerThread(threadInspector)}
            >
              Open thread
            </Button>
            <Button
              variant="ghost"
              size="sm"
              disabled={promptBusy || mutatingSession}
              onclick={() => void handleAskHandlerToSaveWorkflow(threadInspector)}
            >
              Ask to save workflow
            </Button>
          </div>
        </section>

        {#if threadInspector.commandRollups.length > 0}
          <section class="thread-inspector-section">
            <header class="thread-inspector-section-header">
              <div>
                <h3>Command Rollups</h3>
                <p>Inspect thread-local parent commands without flattening child steps into the main session timeline.</p>
              </div>
              <span>{threadInspector.commandRollups.length}</span>
            </header>

            <div class="thread-inspector-command-list">
              {#each threadInspector.commandRollups as rollup (rollup.commandId)}
                <article class="thread-inspector-command">
                  <div class="thread-inspector-command-top">
                    <div class="thread-inspector-command-copy">
                      <strong>{rollup.title}</strong>
                      <span>{rollup.toolName}</span>
                    </div>
                    <div class="thread-inspector-command-meta">
                      <span class={`structured-command-status tone-${getCommandStatusTone(rollup.status)}`.trim()}>
                        {getCommandStatusLabel(rollup.status)}
                      </span>
                      <span>{formatTimestamp(rollup.updatedAt)}</span>
                    </div>
                  </div>
                  <p>{rollup.summary}</p>
                  <div class="thread-inspector-command-footer">
                    <span>
                      {rollup.summaryChildCount}
                      {rollup.summaryChildCount === 1 ? " rollup detail" : " rollup details"}
                    </span>
                    <span>
                      {rollup.traceChildCount}
                      {rollup.traceChildCount === 1 ? " trace step" : " trace steps"}
                    </span>
                    <Button
                      variant="ghost"
                      size="sm"
                      onclick={() => void handleInspectThreadCommand(rollup.commandId)}
                    >
                      Inspect command
                    </Button>
                  </div>
                </article>
              {/each}
            </div>
          </section>
        {/if}

        {#if threadInspector.workflowRuns.length > 0}
          <section class="thread-inspector-section">
            <header class="thread-inspector-section-header">
              <div>
                <h3>Workflow Runs</h3>
                <p>Each workflow run stays attached to the supervising handler thread lifecycle.</p>
              </div>
              <span>{threadInspector.workflowRuns.length}</span>
            </header>

            <div class="thread-inspector-timeline">
              {#each threadInspector.workflowRuns as workflowRun (workflowRun.workflowRunId)}
                <article class="thread-inspector-timeline-item">
                  <div class="thread-inspector-timeline-top">
                    <strong>{workflowRun.workflowName}</strong>
                    <Badge tone={getThreadStatusTone(workflowRun.status)}>
                      {getThreadStatusLabel(workflowRun.status)}
                    </Badge>
                  </div>
                  <p>{workflowRun.summary}</p>
                  <span>{formatTimestamp(workflowRun.updatedAt)}</span>
                  <div class="handler-thread-actions">
                    <Button
                      variant="primary"
                      size="sm"
                      onclick={() => openWorkflowInspector(workflowRun.workflowRunId, threadInspectorSessionId ?? activeSessionId)}
                    >
                      Open inspector
                    </Button>
                  </div>
                  {#if workflowRun.artifacts.length > 0}
                    <div class="command-inspector-artifact-list compact">
                      {#each workflowRun.artifacts as artifact (artifact.artifactId)}
                        <div class="command-inspector-artifact">
                          <div class="command-inspector-artifact-copy">
                            <strong>{artifact.name}</strong>
                            <span>{artifact.kind}</span>
                            {#if artifact.missingFile}
                              <span class="artifact-missing">Missing file</span>
                            {/if}
                          </div>
                          {#if canOpenArtifactLink(artifact)}
                            <Button
                              variant="ghost"
                              size="sm"
                              onclick={() => void handleOpenStructuredArtifact(artifact)}
                            >
                              Open
                            </Button>
                          {/if}
                        </div>
                      {/each}
                    </div>
                  {/if}
                </article>
              {/each}
            </div>
          </section>
        {/if}

        {#if (threadInspector.workflowTaskAttempts?.length ?? 0) > 0}
          <section class="thread-inspector-section">
            <header class="thread-inspector-section-header">
              <div>
                <h3>Workflow Task-Agents</h3>
                <p>Inspect the Smithers workflow task-agent attempts under this thread without promoting them into a top-level surface.</p>
              </div>
              <span>{threadInspector.workflowTaskAttempts?.length ?? 0}</span>
            </header>

            <div class="thread-inspector-command-list">
              {#each threadInspector.workflowTaskAttempts ?? [] as workflowTaskAttempt (workflowTaskAttempt.workflowTaskAttemptId)}
                <article class="thread-inspector-command">
                  <div class="thread-inspector-command-top">
                    <div class="thread-inspector-command-copy">
                      <strong>{workflowTaskAttempt.title}</strong>
                      <span>
                        {workflowTaskAttempt.nodeId}
                        · attempt {workflowTaskAttempt.attempt}
                        {#if workflowTaskAttempt.iteration > 0}
                          · iteration {workflowTaskAttempt.iteration}
                        {/if}
                      </span>
                    </div>
                    <div class="thread-inspector-command-meta">
                      <Badge tone={getWorkflowTaskAttemptStatusTone(workflowTaskAttempt.status)}>
                        {getWorkflowTaskAttemptStatusLabel(workflowTaskAttempt.status)}
                      </Badge>
                      <span>{formatTimestamp(workflowTaskAttempt.updatedAt)}</span>
                    </div>
                  </div>
                  <p>{workflowTaskAttempt.summary}</p>
                  <ContextBudgetBar budget={workflowTaskAttempt.contextBudget} label="Context" />
                  <div class="thread-inspector-command-footer">
                    <span>
                      {workflowTaskAttempt.transcriptMessageCount}
                      {workflowTaskAttempt.transcriptMessageCount === 1 ? " transcript message" : " transcript messages"}
                    </span>
                    <span>
                      {workflowTaskAttempt.commandCount}
                      {workflowTaskAttempt.commandCount === 1 ? " command" : " commands"}
                    </span>
                    {#if workflowTaskAttempt.artifactCount > 0}
                      <span>
                        {workflowTaskAttempt.artifactCount}
                        {workflowTaskAttempt.artifactCount === 1 ? " artifact" : " artifacts"}
                      </span>
                    {/if}
                    <Button
                      variant="ghost"
                      size="sm"
                      onclick={() => void handleInspectThreadWorkflowTaskAttempt(workflowTaskAttempt)}
                    >
                      Inspect attempt
                    </Button>
                  </div>
                </article>
              {/each}
            </div>
          </section>
        {/if}

        {#if threadInspector.episodes.length > 0}
          <section class="thread-inspector-section">
            <header class="thread-inspector-section-header">
              <div>
                <h3>Handoff History</h3>
                <p>Earlier handoff points remain durable so follow-up work can reuse the same thread.</p>
              </div>
              <span>{threadInspector.episodes.length}</span>
            </header>

            <div class="thread-inspector-timeline">
              {#each threadInspector.episodes as episode (episode.episodeId)}
                <article class="thread-inspector-timeline-item">
                  <div class="thread-inspector-timeline-top">
                    <strong>{episode.title}</strong>
                    <span>{episode.kind}</span>
                  </div>
                  <p>{episode.summary}</p>
                  <span>{formatTimestamp(episode.createdAt)}</span>
                </article>
              {/each}
            </div>
          </section>
        {/if}

        {#if threadInspector.artifacts.length > 0}
          <section class="thread-inspector-section">
            <header class="thread-inspector-section-header">
              <div>
                <h3>Artifacts</h3>
                <p>Thread-linked artifacts remain available even after the thread hands back control.</p>
              </div>
              <span>{threadInspector.artifacts.length}</span>
            </header>

            <div class="command-inspector-artifact-list">
              {#each threadInspector.artifacts as artifact (artifact.artifactId)}
                <div class="command-inspector-artifact">
                  <div class="command-inspector-artifact-copy">
                    <strong>{artifact.name}</strong>
                    <span>{artifact.kind}</span>
                    {#if artifact.producerLabel}
                      <span>{artifact.producerLabel}</span>
                    {/if}
                    {#if artifact.missingFile}
                      <span class="artifact-missing">Missing file</span>
                    {/if}
                    {#if artifact.path}
                      <code>{artifact.path}</code>
                    {/if}
                  </div>
                  {#if canOpenArtifactLink(artifact)}
                    <Button
                      variant="ghost"
                      size="sm"
                      onclick={() => void handleOpenStructuredArtifact(artifact)}
                    >
                      Open
                    </Button>
                  {/if}
                </div>
              {/each}
            </div>
          </section>
        {/if}
      {/if}
    </div>
  </Dialog>
{/if}

{#if showCommandInspector}
  <Dialog
    eyebrow="Command"
    title={commandInspector?.title ?? "Inspect Command"}
    description="Inspect the durable parent rollup and its nested child commands without promoting child steps into the main session timeline."
    width="lg"
    onClose={closeCommandInspector}
  >
    <div class="command-inspector">
      {#if commandInspectorLoading}
        <p class="command-inspector-empty">Loading structured command detail…</p>
      {:else if commandInspectorError}
        <p class="command-inspector-empty error">{commandInspectorError}</p>
      {:else if commandInspector}
        <section class="command-inspector-summary">
          <div class="command-inspector-summary-top">
            <div class="command-inspector-summary-copy">
              <strong>{commandInspector.title}</strong>
              <p>{commandInspector.summary}</p>
            </div>
            <div class="command-inspector-summary-meta">
              <span
                class={`structured-command-status tone-${getCommandStatusTone(commandInspector.status)}`.trim()}
              >
                {getCommandStatusLabel(commandInspector.status)}
              </span>
              <span>{commandInspector.toolName}</span>
              <span>{formatTimestamp(commandInspector.updatedAt)}</span>
            </div>
          </div>

          <div class="command-inspector-pills">
            <span>
              {commandInspector.summaryChildCount}
              {commandInspector.summaryChildCount === 1 ? " rollup detail" : " rollup details"}
            </span>
            <span>
              {commandInspector.traceChildCount}
              {commandInspector.traceChildCount === 1 ? " trace step" : " trace steps"}
            </span>
            {#if commandInspector.threadId}
              <span>{commandInspector.threadId}</span>
            {/if}
            {#if commandInspector.workflowTaskAttemptId}
              <Button
                variant="ghost"
                size="sm"
                onclick={() => void handleInspectCommandWorkflowTaskAttempt(commandInspector.workflowTaskAttemptId!)}
              >
                Inspect task attempt
              </Button>
            {/if}
          </div>

          {#if commandInspector.error}
            <p class="command-inspector-error">{commandInspector.error}</p>
          {/if}

          {#if formatCommandFacts(commandInspector.facts)}
            <div class="command-inspector-facts">
              <span>Facts</span>
              <pre>{formatCommandFacts(commandInspector.facts)}</pre>
            </div>
          {/if}

          {#if commandInspector.artifacts.length > 0}
            <div class="command-inspector-artifacts">
              <span>Artifacts</span>
              <div class="command-inspector-artifact-list">
                {#each commandInspector.artifacts as artifact (artifact.artifactId)}
                  <div class="command-inspector-artifact">
                    <div class="command-inspector-artifact-copy">
                      <strong>{artifact.name}</strong>
                      <span>{artifact.kind}</span>
                      {#if artifact.producerLabel}
                        <span>{artifact.producerLabel}</span>
                      {/if}
                      {#if artifact.missingFile}
                        <span class="artifact-missing">Missing file</span>
                      {/if}
                      {#if artifact.path}
                        <code>{artifact.path}</code>
                      {/if}
                    </div>
                    {#if canOpenArtifactLink(artifact)}
                      <Button
                        variant="ghost"
                        size="sm"
                        onclick={() => void handleOpenStructuredArtifact(artifact)}
                      >
                        Open
                      </Button>
                    {/if}
                  </div>
                {/each}
              </div>
            </div>
          {/if}
        </section>

        {#if commandInspectorSections.length > 0}
          <div class="command-inspector-sections">
            {#each commandInspectorSections as section (section.id)}
              <section class="command-inspector-section">
                <header class="command-inspector-section-header">
                  <div>
                    <h3>{section.title}</h3>
                    <p>{section.description}</p>
                  </div>
                  <span>{section.children.length}</span>
                </header>

                <div class="command-inspector-child-list">
                  {#each section.children as child (child.commandId)}
                    <article class="command-inspector-child">
                      <div class="command-inspector-child-top">
                        <div class="command-inspector-child-copy">
                          <strong>{child.title}</strong>
                          <span>{child.toolName}</span>
                        </div>
                        <div class="command-inspector-child-meta">
                          <span
                            class={`structured-command-status tone-${getCommandStatusTone(child.status)}`.trim()}
                          >
                            {getCommandStatusLabel(child.status)}
                          </span>
                          <span>{formatTimestamp(child.updatedAt)}</span>
                        </div>
                      </div>

                      <p class="command-inspector-child-summary">{child.summary}</p>

                      {#if child.error}
                        <p class="command-inspector-error">{child.error}</p>
                      {/if}

                      {#if formatCommandFacts(child.facts)}
                        <div class="command-inspector-facts child-facts">
                          <span>Facts</span>
                          <pre>{formatCommandFacts(child.facts)}</pre>
                        </div>
                      {/if}

                      <div class="command-inspector-child-footer">
                        <span>{child.visibility}</span>
                        <span>{formatTimestamp(child.startedAt)}</span>
                        {#if child.finishedAt}
                          <span>{formatTimestamp(child.finishedAt)}</span>
                        {/if}
                      </div>

                      {#if child.artifacts.length > 0}
                        <div class="command-inspector-artifact-list compact">
                          {#each child.artifacts as artifact (artifact.artifactId)}
                            <div class="command-inspector-artifact">
                              <div class="command-inspector-artifact-copy">
                                <strong>{artifact.name}</strong>
                                <span>{artifact.kind}</span>
                                {#if artifact.producerLabel}
                                  <span>{artifact.producerLabel}</span>
                                {/if}
                                {#if artifact.missingFile}
                                  <span class="artifact-missing">Missing file</span>
                                {/if}
                              </div>
                              {#if canOpenArtifactLink(artifact)}
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  onclick={() => void handleOpenStructuredArtifact(artifact)}
                                >
                                  Open
                                </Button>
                              {/if}
                            </div>
                          {/each}
                        </div>
                      {/if}
                    </article>
                  {/each}
                </div>
              </section>
            {/each}
          </div>
        {:else}
          <p class="command-inspector-empty">No child command detail was recorded for this command.</p>
        {/if}
      {/if}
    </div>
  </Dialog>
{/if}

{#if showWorkflowTaskAttemptInspector}
  <Dialog
    eyebrow="Workflow Task-Agent"
    title={workflowTaskAttemptInspector?.title ?? "Inspect Workflow Task-Agent"}
    description="Inspect the durable workflow task attempt transcript, nested command rollups, and artifacts without opening a separate interactive surface."
    width="lg"
    onClose={closeWorkflowTaskAttemptInspector}
  >
    <div class="command-inspector">
      {#if workflowTaskAttemptInspectorLoading}
        <p class="command-inspector-empty">Loading workflow task attempt detail…</p>
      {:else if workflowTaskAttemptInspectorError}
        <p class="command-inspector-empty error">{workflowTaskAttemptInspectorError}</p>
      {:else if workflowTaskAttemptInspector}
        <section class="command-inspector-summary">
          <div class="command-inspector-summary-top">
            <div class="command-inspector-summary-copy">
              <strong>{workflowTaskAttemptInspector.title}</strong>
              <p>{workflowTaskAttemptInspector.summary}</p>
            </div>
            <div class="command-inspector-summary-meta">
              <Badge tone={getWorkflowTaskAttemptStatusTone(workflowTaskAttemptInspector.status)}>
                {getWorkflowTaskAttemptStatusLabel(workflowTaskAttemptInspector.status)}
              </Badge>
              <span>{formatTimestamp(workflowTaskAttemptInspector.updatedAt)}</span>
            </div>
          </div>

          <div class="command-inspector-pills">
            <span>{workflowTaskAttemptInspector.nodeId}</span>
            <span>attempt {workflowTaskAttemptInspector.attempt}</span>
            {#if workflowTaskAttemptInspector.iteration > 0}
              <span>iteration {workflowTaskAttemptInspector.iteration}</span>
            {/if}
            <span>{workflowTaskAttemptInspector.smithersRunId}</span>
            <span>{workflowTaskAttemptInspector.smithersState}</span>
          </div>

          <ContextBudgetBar budget={workflowTaskAttemptInspector.contextBudget} label="Context" />

          {#if workflowTaskAttemptInspector.error}
            <p class="command-inspector-error">{workflowTaskAttemptInspector.error}</p>
          {/if}

          {#if formatCommandFacts(workflowTaskAttemptInspector.meta)}
            <div class="command-inspector-facts">
              <span>Meta</span>
              <pre>{formatCommandFacts(workflowTaskAttemptInspector.meta)}</pre>
            </div>
          {/if}
        </section>

        {#if workflowTaskAttemptInspector.transcript.length > 0}
          <section class="command-inspector-section">
            <header class="command-inspector-section-header">
              <div>
                <h3>Transcript</h3>
                <p>Durable prompt and reply messages for this task attempt.</p>
              </div>
              <span>{workflowTaskAttemptInspector.transcript.length}</span>
            </header>

            <div class="command-inspector-child-list">
              {#each workflowTaskAttemptInspector.transcript as message (message.messageId)}
                <article class="command-inspector-child">
                  <div class="command-inspector-child-top">
                    <div class="command-inspector-child-copy">
                      <strong>{message.role}</strong>
                      <span>{message.source}</span>
                    </div>
                    <div class="command-inspector-child-meta">
                      <span>{formatTimestamp(message.createdAt)}</span>
                    </div>
                  </div>

                  <p class="command-inspector-child-summary transcript-body">{message.text}</p>
                </article>
              {/each}
            </div>
          </section>
        {/if}

        {#if workflowTaskAttemptInspector.commandRollups.length > 0}
          <section class="command-inspector-section">
            <header class="command-inspector-section-header">
              <div>
                <h3>Commands</h3>
                <p>Nested durable command rollups attached to this task attempt.</p>
              </div>
              <span>{workflowTaskAttemptInspector.commandRollups.length}</span>
            </header>

            <div class="thread-inspector-command-list">
              {#each workflowTaskAttemptInspector.commandRollups as rollup (rollup.commandId)}
                <article class="thread-inspector-command">
                  <div class="thread-inspector-command-top">
                    <div class="thread-inspector-command-copy">
                      <strong>{rollup.title}</strong>
                      <span>{rollup.toolName}</span>
                    </div>
                    <div class="thread-inspector-command-meta">
                      <span class={`structured-command-status tone-${getCommandStatusTone(rollup.status)}`.trim()}>
                        {getCommandStatusLabel(rollup.status)}
                      </span>
                      <span>{formatTimestamp(rollup.updatedAt)}</span>
                    </div>
                  </div>
                  <p>{rollup.summary}</p>
                  <div class="thread-inspector-command-footer">
                    <span>
                      {rollup.summaryChildCount}
                      {rollup.summaryChildCount === 1 ? " rollup detail" : " rollup details"}
                    </span>
                    <span>
                      {rollup.traceChildCount}
                      {rollup.traceChildCount === 1 ? " trace step" : " trace steps"}
                    </span>
                    <Button
                      variant="ghost"
                      size="sm"
                      onclick={() => void handleInspectThreadCommand(rollup.commandId)}
                    >
                      Inspect command
                    </Button>
                  </div>
                </article>
              {/each}
            </div>
          </section>
        {/if}

        {#if workflowTaskAttemptInspector.artifacts.length > 0}
          <section class="command-inspector-section">
            <header class="command-inspector-section-header">
              <div>
                <h3>Artifacts</h3>
                <p>Artifacts created directly by this workflow task attempt.</p>
              </div>
              <span>{workflowTaskAttemptInspector.artifacts.length}</span>
            </header>

            <div class="command-inspector-artifact-list">
              {#each workflowTaskAttemptInspector.artifacts as artifact (artifact.artifactId)}
                <div class="command-inspector-artifact">
                  <div class="command-inspector-artifact-copy">
                    <strong>{artifact.name}</strong>
                    <span>{artifact.kind}</span>
                    {#if artifact.producerLabel}
                      <span>{artifact.producerLabel}</span>
                    {/if}
                    {#if artifact.missingFile}
                      <span class="artifact-missing">Missing file</span>
                    {/if}
                    {#if artifact.path}
                      <code>{artifact.path}</code>
                    {/if}
                  </div>
                  {#if canOpenArtifactLink(artifact)}
                    <Button
                      variant="ghost"
                      size="sm"
                      onclick={() => void handleOpenStructuredArtifact(artifact)}
                    >
                      Open
                    </Button>
                  {/if}
                </div>
              {/each}
            </div>
          </section>
        {/if}
      {/if}
    </div>
  </Dialog>
{/if}

<style>
  .workspace-shell {
    --mac-traffic-light-top: 13px;
    --mac-traffic-light-size: 16px;
    --workspace-chrome-height: calc((var(--mac-traffic-light-top) * 2) + var(--mac-traffic-light-size));
    --workspace-chrome-control-size: 28px;
    position: relative;
    display: grid;
    grid-template-rows: minmax(0, 1fr);
    height: 100%;
    min-height: 0;
    margin-inline: 0;
    background: var(--ui-chrome);
  }

  .workspace-titlebar {
    --titlebar-inline-padding: 0.72rem;
    --mac-titlebar-control-offset: 5.25rem;
    --workspace-titlebar-start-width: var(--sidebar-width, 240px);
    position: absolute;
    top: 0;
    left: 0;
    z-index: 12;
    display: grid;
    grid-template-columns: var(--workspace-titlebar-start-width) minmax(0, 1fr) max-content;
    align-items: center;
    column-gap: 0;
    width: 100%;
    height: var(--workspace-chrome-height);
    padding: 0 var(--titlebar-inline-padding) 0 0;
    border: 0;
    background: transparent;
    pointer-events: auto;
    transition: width 230ms cubic-bezier(0.19, 1, 0.22, 1);
  }

  .workspace-titlebar.sidebar-titlebar-hidden {
    width: 100%;
    --workspace-titlebar-start-width: 7.25rem;
  }

  .workspace-shell.mac-window-chrome .workspace-titlebar {
    padding-left: 0;
  }

  .workspace-titlebar-start {
    display: flex;
    align-items: center;
    gap: 0.48rem;
    width: var(--workspace-titlebar-start-width);
    min-width: 7.25rem;
    transition: width 230ms cubic-bezier(0.19, 1, 0.22, 1);
  }

  .workspace-shell.mac-window-chrome .workspace-titlebar-start {
    padding-left: var(--mac-titlebar-control-offset);
  }

  .workspace-titlebar-title {
    display: none;
  }

  .workspace-titlebar-tabs {
    display: flex;
    align-items: center;
    min-width: 0;
    justify-self: stretch;
    overflow: hidden;
  }

  .workspace-titlebar-actions {
    display: inline-flex;
    align-items: center;
    justify-content: flex-end;
    gap: 0.58rem;
    justify-self: end;
    min-width: max-content;
  }

  .workspace-layout-tab {
    border: 1px solid var(--ui-border-soft);
    border-radius: var(--ui-radius-md);
    background: color-mix(in oklab, var(--ui-panel) 82%, transparent);
    color: var(--ui-text-secondary);
    font: inherit;
    cursor: pointer;
    transition:
      background-color 150ms cubic-bezier(0.19, 1, 0.22, 1),
      border-color 150ms cubic-bezier(0.19, 1, 0.22, 1),
      color 150ms cubic-bezier(0.19, 1, 0.22, 1),
      opacity 150ms cubic-bezier(0.19, 1, 0.22, 1);
  }

  .titlebar-icon {
    position: relative;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    padding: 0;
    border: 0;
    border-radius: var(--ui-radius-md);
    background: transparent;
    color: var(--ui-text-tertiary);
    cursor: pointer;
    transition:
      background-color 150ms cubic-bezier(0.19, 1, 0.22, 1),
      color 150ms cubic-bezier(0.19, 1, 0.22, 1),
      opacity 150ms cubic-bezier(0.19, 1, 0.22, 1),
      transform 120ms cubic-bezier(0.22, 1, 0.36, 1);
  }

  .titlebar-icon {
    width: 1.45rem;
    height: 1.45rem;
  }

  .chat-workspace {
    --sidebar-width: 240px;
    position: relative;
    display: grid;
    grid-template-columns: var(--sidebar-width) minmax(0, 1fr);
    height: 100%;
    min-height: 0;
    padding: 0;
    transition: grid-template-columns 230ms cubic-bezier(0.19, 1, 0.22, 1);
  }

  .chat-workspace.sidebar-hidden {
    grid-template-columns: 0rem minmax(0, 1fr);
  }

  .chat-workspace.split {
    grid-template-columns: var(--sidebar-width) minmax(0, 1fr) minmax(22rem, 28rem);
  }

  .chat-workspace.sidebar-hidden.split {
    grid-template-columns: 0rem minmax(0, 1fr) minmax(22rem, 28rem);
  }

  .workspace-sidebar,
  .workspace-main,
  .artifacts-slot {
    min-height: 0;
    min-width: 0;
  }

  .workspace-sidebar {
    position: relative;
    z-index: 2;
    overflow: visible;
    opacity: 1;
    transform: translateX(0);
    transform-origin: 0 50%;
    pointer-events: auto;
    transition:
      opacity 155ms cubic-bezier(0.19, 1, 0.22, 1),
      transform 230ms cubic-bezier(0.19, 1, 0.22, 1);
    will-change: opacity, transform;
  }

  .workspace-sidebar:has(:global(.context-menu)) {
    z-index: var(--ui-z-overlay);
  }

  .chat-workspace.sidebar-hidden .workspace-sidebar {
    opacity: 0;
    transform: translateX(-0.42rem);
    pointer-events: none;
  }

  .sidebar-surface {
    height: 100%;
    min-height: 0;
    padding: 0;
    background: var(--ui-chrome);
  }

  .sidebar-resize-handle {
    position: absolute;
    top: var(--workspace-chrome-height);
    bottom: 0;
    left: calc(var(--sidebar-width) - 0.2rem);
    z-index: 9;
    width: 0.4rem;
    cursor: col-resize;
    touch-action: none;
  }

  .sidebar-resize-handle::before {
    content: "";
    position: absolute;
    top: 0;
    bottom: 0;
    left: 50%;
    width: 1px;
    transform: translateX(-50%);
    border-radius: 999px;
    background: color-mix(in oklab, var(--ui-shell-edge) 52%, transparent);
    transition: background-color 160ms cubic-bezier(0.19, 1, 0.22, 1);
  }

  .sidebar-resize-handle:hover::before,
  .sidebar-resize-handle.dragging::before {
    background: color-mix(in oklab, var(--ui-accent) 32%, var(--ui-border-strong));
  }

  .workspace-main {
    display: grid;
    grid-template-rows: auto minmax(0, 1fr);
    gap: 0;
    min-height: 0;
    padding: 0;
    background: var(--ui-bg);
  }
  .workspace-main-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    min-height: var(--workspace-chrome-height);
    height: var(--workspace-chrome-height);
    padding: 0 0.78rem;
    border-bottom: 0;
    background: var(--ui-chrome);
    box-shadow: none;
    pointer-events: none;
  }

  .chat-workspace.sidebar-hidden .workspace-main-header {
    padding-left: 7.3rem;
  }

  .workspace-main-meta {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    flex-wrap: nowrap;
    gap: 0.5rem;
    min-height: var(--workspace-chrome-control-size);
    min-width: 0;
    font-size: var(--text-xs);
    color: var(--ui-text-tertiary);
  }

  .workspace-layout-switcher {
    display: inline-flex;
    align-items: center;
    gap: 0.24rem;
    min-width: max-content;
  }

  .workspace-layout-tab {
    display: inline-grid;
    place-items: center;
    width: 1.65rem;
    height: 1.45rem;
    padding: 0;
    font-size: var(--text-xs);
    font-weight: 700;
  }

  .workspace-layout-tab.empty {
    opacity: 0.48;
  }

  .workspace-layout-tab.empty:hover,
  .workspace-layout-tab.empty:focus-visible,
  .workspace-layout-tab.active {
    opacity: 1;
  }

  .workspace-layout-tab.active {
    border-color: color-mix(in oklab, var(--ui-accent) 52%, var(--ui-border-strong));
    background: color-mix(in oklab, var(--ui-accent) 16%, var(--ui-panel));
    color: var(--ui-text-primary);
  }

  .workspace-main-meta :global(.ui-metadata-chip) {
    max-width: 13rem;
  }

  .workspace-main-chips {
    display: contents;
  }

  .workspace-main-meta :global(.context-budget-compact) {
    position: static;
    width: 5.25rem;
    flex: 0 0 5.25rem;
  }

  .workspace-main-stat {
    display: inline-flex;
    align-items: center;
    min-height: 1.35rem;
    max-width: 10.5rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--ui-text-tertiary);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    line-height: 1;
  }

  .header-icon-button {
    position: relative;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 1.45rem;
    height: 1.45rem;
    padding: 0;
    border: 1px solid var(--ui-border-soft);
    border-radius: var(--ui-radius-sm);
    background: transparent;
    color: var(--ui-text-tertiary);
    cursor: pointer;
    transition:
      border-color 140ms cubic-bezier(0.19, 1, 0.22, 1),
      background-color 140ms cubic-bezier(0.19, 1, 0.22, 1),
      color 140ms cubic-bezier(0.19, 1, 0.22, 1),
      transform 120ms cubic-bezier(0.22, 1, 0.36, 1);
  }

  .header-icon-button:hover,
  .header-icon-button:focus-visible,
  .header-icon-button[aria-pressed="true"] {
    border-color: var(--ui-border-strong);
    background: var(--ui-surface-subtle);
    color: var(--ui-text-primary);
  }

  .titlebar-icon:active:not(:disabled),
  .header-icon-button:active:not(:disabled) {
    transform: translateY(1px) scale(0.94);
  }

  .inline-titlebar-action {
    color: var(--ui-text-tertiary);
  }

  .inline-titlebar-action:hover,
  .inline-titlebar-action:focus-visible {
    outline: none;
    background: var(--ui-surface-subtle);
    color: var(--ui-text-primary);
  }

  .project-ci-compact {
    display: inline-flex;
    align-items: center;
    gap: 0.42rem;
    max-width: min(38rem, 100%);
    min-height: 1.72rem;
    padding: 0.18rem 0.28rem;
    border-radius: var(--ui-radius-md);
    background: color-mix(in oklab, var(--ui-surface-subtle) 70%, transparent);
  }

  .project-ci-compact span {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .chat-pane {
    min-height: 0;
  }

  .chat-pane-shell {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    overflow: hidden;
    border: 0;
    border-radius: 0;
    background: var(--ui-bg);
  }

  .new-session-empty {
    display: grid;
    align-content: start;
    justify-items: center;
    gap: 0.72rem;
    flex: 1 1 auto;
    min-height: 0;
    overflow: auto;
    padding: clamp(1.4rem, 8vh, 3.1rem) 1.25rem 1rem;
    color: var(--ui-text-secondary);
  }

  .new-session-intro {
    display: grid;
    justify-items: center;
    gap: 0.46rem;
    width: min(32rem, 100%);
  }

  .new-session-watermark {
    margin: 0;
    color: color-mix(in oklab, var(--ui-text-tertiary) 22%, transparent);
    font-size: var(--text-heading-sm);
    font-weight: 700;
    letter-spacing: 0;
  }

  .new-session-heading {
    display: grid;
    justify-items: center;
    gap: 0.36rem;
    text-align: center;
  }

  .new-session-heading h2,
  .new-session-heading p {
    margin: 0;
  }

  .new-session-heading h2 {
    color: var(--ui-text-primary);
    font-size: var(--text-base);
    font-weight: 700;
  }

  .new-session-heading p {
    color: var(--ui-text-tertiary);
    font-size: var(--text-sm);
  }

  .new-session-controls,
  .new-session-recent {
    width: min(32rem, 100%);
  }

  .new-session-controls {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.72rem;
    min-height: 2.35rem;
    padding: 0.38rem 0.48rem;
    border: 1px solid var(--ui-border-soft);
    border-radius: var(--ui-radius-md);
    background: color-mix(in oklab, var(--ui-surface-subtle) 54%, transparent);
  }

  .new-session-mode-toggle,
  .new-session-mode-buttons {
    display: inline-flex;
    align-items: center;
    min-width: 0;
  }

  .new-session-mode-toggle {
    gap: 0.5rem;
  }

  .new-session-mode-toggle > span {
    color: var(--ui-text-tertiary);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    font-weight: 500;
    text-transform: uppercase;
  }

  .new-session-mode-buttons {
    gap: 0.16rem;
    padding: 0.12rem;
    border: 1px solid var(--ui-border-soft);
    border-radius: var(--ui-radius-sm);
    background: color-mix(in oklab, var(--ui-surface) 64%, transparent);
  }

  .new-session-mode-buttons button {
    min-height: 1.42rem;
    padding: 0 0.5rem;
    border: 0;
    border-radius: var(--ui-radius-sm);
    background: transparent;
    color: var(--ui-text-tertiary);
    font-size: var(--text-xs);
    font-weight: 600;
    cursor: pointer;
  }

  .new-session-mode-buttons button:hover:not(:disabled),
  .new-session-mode-buttons button:focus-visible {
    outline: none;
    color: var(--ui-text-primary);
    background: var(--ui-surface-subtle);
  }

  .new-session-mode-buttons button.active {
    background: color-mix(in oklab, var(--ui-accent) 18%, var(--ui-surface-subtle));
    color: var(--ui-text-primary);
  }

  .new-session-mode-buttons button:disabled {
    cursor: not-allowed;
    opacity: 0.55;
  }

  .new-session-mode-note {
    margin: 0;
    min-width: 0;
    overflow: hidden;
    color: var(--ui-text-tertiary);
    font-size: var(--text-xs);
    line-height: 1.3;
    text-align: right;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .new-session-recent {
    display: grid;
    gap: 0.3rem;
  }

  .new-session-recent p {
    margin: 0;
    color: var(--ui-text-tertiary);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    text-transform: uppercase;
  }

  .new-session-recent button {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto;
    align-items: center;
    gap: 0.5rem;
    min-height: 1.7rem;
    padding: 0 0.38rem;
    border: 0;
    border-radius: var(--ui-radius-sm);
    background: transparent;
    color: var(--ui-text-secondary);
    text-align: left;
    cursor: pointer;
  }

  .new-session-recent button:hover,
  .new-session-recent button:focus-visible {
    background: var(--ui-surface-subtle);
    color: var(--ui-text-primary);
  }

  .new-session-recent span:not(.status-dot) {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--text-xs);
  }

  .new-session-recent small {
    color: var(--ui-text-tertiary);
    font-size: var(--text-xs);
  }

  .project-ci-panel,
  .handler-thread-panel,
  .structured-command-panel {
    display: grid;
    flex: 0 0 auto;
    gap: 0.72rem;
    padding: 0.72rem 0.9rem 0.66rem;
    border-bottom: 1px solid color-mix(in oklab, var(--ui-shell-edge) 66%, transparent);
    background:
      linear-gradient(
        180deg,
        color-mix(in oklab, var(--ui-surface-raised) 78%, transparent),
        transparent
      ),
      color-mix(in oklab, var(--ui-surface-subtle) 54%, transparent);
  }

  .project-ci-header,
  .handler-thread-header,
  .structured-command-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 1rem;
    min-width: 0;
  }

  .project-ci-header > div,
  .handler-thread-header > div,
  .structured-command-header > div {
    min-width: 0;
    flex: 0 0 auto;
  }

  .project-ci-header h3,
  .project-ci-eyebrow,
  .handler-thread-header h3,
  .handler-thread-eyebrow,
  .handler-thread-copy,
  .structured-command-header h3,
  .structured-command-eyebrow,
  .structured-command-copy {
    margin: 0;
  }

  .project-ci-eyebrow,
  .handler-thread-eyebrow,
  .structured-command-eyebrow {
    font-size: var(--text-xs);
    font-family: var(--font-mono);
    letter-spacing: var(--tracking-wide);
    text-transform: uppercase;
    color: var(--ui-text-tertiary);
  }

  .project-ci-header h3,
  .handler-thread-header h3,
  .structured-command-header h3 {
    margin-top: 0.18rem;
    font-size: var(--text-base);
    font-weight: 600;
    letter-spacing: 0;
    color: var(--ui-text-primary);
  }

  .handler-thread-copy,
  .structured-command-copy {
    max-width: 42rem;
    min-width: 0;
    text-align: right;
  }

  .handler-thread-copy,
  .structured-command-copy {
    max-width: 28rem;
    font-size: var(--text-sm);
    line-height: 1.5;
    color: var(--ui-text-secondary);
  }

  .project-ci-body {
    display: grid;
    gap: 0.56rem;
  }

  .project-ci-summary,
  .project-ci-muted,
  .project-ci-empty,
  .project-ci-run-card p,
  .project-ci-check p {
    margin: 0;
    font-size: var(--text-sm);
    line-height: 1.52;
    color: var(--ui-text-secondary);
  }

  .project-ci-summary {
    color: var(--ui-text-primary);
  }

  .project-ci-muted {
    color: var(--ui-text-tertiary);
  }

  .project-ci-entries,
  .project-ci-check-list {
    display: grid;
    gap: 0.45rem;
  }

  .project-ci-entry,
  .project-ci-run-card,
  .project-ci-check {
    border: 1px solid color-mix(in oklab, var(--ui-border-soft) 82%, transparent);
    border-radius: var(--ui-radius-md);
    background: color-mix(in oklab, var(--ui-surface) 94%, transparent);
  }

  .project-ci-entry {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    padding: 0.55rem 0.62rem;
  }

  .project-ci-entry strong,
  .project-ci-run-card strong,
  .project-ci-check-copy strong {
    font-size: var(--text-sm);
    font-weight: 600;
    color: var(--ui-text-primary);
  }

  .project-ci-entry span,
  .project-ci-run-card span,
  .project-ci-check-copy span,
  .project-ci-check-meta {
    font-size: var(--text-xs);
    color: var(--ui-text-tertiary);
  }

  .project-ci-entry span,
  .project-ci-run-card code,
  .project-ci-check-meta code {
    overflow-wrap: anywhere;
    word-break: break-word;
  }

  .project-ci-run-card,
  .project-ci-check {
    display: grid;
    gap: 0.45rem;
    padding: 0.68rem 0.72rem;
  }

  .project-ci-run-card-top,
  .project-ci-check-top,
  .project-ci-check-meta {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 0.72rem;
  }

  .project-ci-run-card-top > div,
  .project-ci-check-copy {
    display: flex;
    min-width: 0;
    flex-direction: column;
    gap: 0.15rem;
  }

  .project-ci-run-card code,
  .project-ci-check-meta code {
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--ui-text-secondary);
  }

  .project-ci-check-meta {
    justify-content: flex-start;
    flex-wrap: wrap;
  }

  .handler-thread-list {
    display: grid;
    gap: 0.55rem;
    max-height: 18rem;
    overflow: auto;
    padding-right: 0.1rem;
  }

  .handler-thread-reference-entry {
    display: grid;
    gap: 0.5rem;
  }

  .handler-thread-reference-entry :global(.reference-thread-card) {
    box-shadow: none;
  }

  .reference-pills {
    justify-content: flex-start;
    padding-inline: 0.1rem;
  }

  .structured-command-list {
    display: grid;
    gap: 0.5rem;
    max-height: 6.5rem;
    overflow: auto;
    padding-right: 0.1rem;
  }

  .structured-command-card {
    display: grid;
    gap: 0.55rem;
    width: 100%;
    padding: 0.78rem 0.85rem;
    border: 1px solid color-mix(in oklab, var(--ui-border-soft) 84%, transparent);
    border-radius: var(--ui-radius-md);
    background: color-mix(in oklab, var(--ui-surface-raised) 92%, transparent);
    color: inherit;
    text-align: left;
    cursor: pointer;
    transition:
      border-color 160ms cubic-bezier(0.19, 1, 0.22, 1),
      background-color 160ms cubic-bezier(0.19, 1, 0.22, 1),
      box-shadow 160ms cubic-bezier(0.19, 1, 0.22, 1);
  }

  .structured-command-card:hover {
    border-color: color-mix(in oklab, var(--ui-border-accent) 70%, transparent);
    background:
      linear-gradient(180deg, color-mix(in oklab, var(--ui-accent-soft) 30%, transparent), transparent),
      color-mix(in oklab, var(--ui-surface-raised) 94%, transparent);
  }

  .structured-command-card:focus-visible {
    outline: none;
    box-shadow: var(--ui-focus-ring);
  }

  .structured-command-card-top,
  .structured-command-card-footer,
  .thread-inspector-summary-top,
  .thread-inspector-command-top,
  .thread-inspector-timeline-top,
  .thread-inspector-section-header,
  .command-inspector-summary-top,
  .command-inspector-child-top,
  .command-inspector-artifact {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 0.85rem;
  }

  .structured-command-card-copy,
  .thread-inspector-summary-copy,
  .thread-inspector-command-copy,
  .command-inspector-summary-copy,
  .command-inspector-child-copy,
  .command-inspector-artifact-copy {
    display: flex;
    flex-direction: column;
    gap: 0.18rem;
    min-width: 0;
  }

  .structured-command-card-copy strong,
  .thread-inspector-summary-copy strong,
  .thread-inspector-command-copy strong,
  .command-inspector-summary-copy strong,
  .command-inspector-child-copy strong {
    font-size: var(--text-base);
    font-weight: 600;
    letter-spacing: 0;
    color: var(--ui-text-primary);
  }

  .handler-thread-preview,
  .structured-command-card-copy span,
  .thread-inspector-summary-copy p,
  .thread-inspector-highlight p,
  .thread-inspector-command p,
  .thread-inspector-timeline-item p,
  .command-inspector-summary-copy p,
  .structured-command-summary,
  .command-inspector-child-summary,
  .command-inspector-artifact-copy span {
    margin: 0;
    font-size: var(--text-sm);
    line-height: 1.5;
    color: var(--ui-text-secondary);
  }

  .handler-thread-pills,
  .handler-thread-actions,
  .structured-command-card-meta,
  .thread-inspector-summary-meta,
  .thread-inspector-pills,
  .thread-inspector-actions,
  .thread-inspector-command-meta,
  .thread-inspector-command-footer,
  .command-inspector-summary-meta,
  .command-inspector-child-meta,
  .command-inspector-child-footer,
  .command-inspector-pills {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    flex-wrap: wrap;
    justify-content: flex-end;
    font-size: var(--text-xs);
    color: var(--ui-text-tertiary);
  }

  .structured-command-status {
    font-size: var(--text-xs);
    font-family: var(--font-mono);
    font-variant-numeric: tabular-nums;
  }

  .structured-command-status.tone-success {
    color: color-mix(in oklab, var(--ui-success) 78%, var(--ui-text-primary));
  }

  .structured-command-status.tone-info {
    color: color-mix(in oklab, var(--ui-status-waiting) 78%, var(--ui-text-primary));
  }

  .structured-command-status.tone-warning {
    color: color-mix(in oklab, var(--ui-warning) 84%, var(--ui-text-primary));
  }

  .structured-command-status.tone-danger {
    color: color-mix(in oklab, var(--ui-danger) 82%, var(--ui-text-primary));
  }

  .structured-command-status.tone-neutral {
    color: var(--ui-text-tertiary);
  }

  .structured-command-summary,
  .command-inspector-child-summary {
    color: var(--ui-text-primary);
  }

  .handler-thread-preview,
  .thread-inspector-highlight p,
  .thread-inspector-command p,
  .thread-inspector-timeline-item p {
    color: var(--ui-text-primary);
  }

  .handler-thread-pills span,
  .thread-inspector-pills span,
  .thread-inspector-command-footer span,
  .command-inspector-pills span,
  .command-inspector-child-footer span {
    display: inline-flex;
    align-items: center;
    min-height: 1rem;
    padding: 0.14rem 0.42rem;
    border-radius: 999px;
    background: color-mix(in oklab, var(--ui-surface-subtle) 84%, transparent);
  }

  .handler-thread-actions,
  .thread-inspector-actions,
  .thread-inspector-command-footer {
    justify-content: flex-end;
  }

  .structured-command-highlights,
  .thread-inspector-highlight,
  .thread-inspector-metadata,
  .thread-inspector-command-list,
  .thread-inspector-timeline,
  .command-inspector-sections,
  .command-inspector-child-list,
  .command-inspector-artifact-list {
    display: grid;
    gap: 0.45rem;
  }

  .thread-inspector-summary,
  .thread-inspector-section,
  .thread-inspector-metadata,
  .thread-inspector-command,
  .thread-inspector-timeline-item,
  .structured-command-highlight,
  .command-inspector-child,
  .command-inspector-summary,
  .command-inspector-section,
  .command-inspector-artifact {
    border: 1px solid color-mix(in oklab, var(--ui-border-soft) 82%, transparent);
    border-radius: var(--ui-radius-md);
    background: color-mix(in oklab, var(--ui-surface) 94%, transparent);
  }

  .thread-inspector-highlight,
  .thread-inspector-metadata,
  .thread-inspector-command,
  .thread-inspector-timeline-item {
    padding: 0.72rem 0.76rem;
  }

  .thread-inspector-metadata {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .thread-inspector-metadata div {
    display: grid;
    gap: 0.18rem;
    min-width: 0;
  }

  .thread-inspector-metadata span,
  .thread-inspector-wait span {
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--ui-text-tertiary);
  }

  .thread-inspector-metadata strong {
    overflow: hidden;
    text-overflow: ellipsis;
    color: var(--ui-text-primary);
    font-size: var(--text-sm);
    font-weight: 500;
    white-space: nowrap;
  }

  .thread-inspector-highlight span,
  .thread-inspector-section-header p,
  .thread-inspector-timeline-item span,
  .thread-inspector-timeline-top span {
    font-size: var(--text-sm);
    color: var(--ui-text-secondary);
  }

  .thread-inspector-section-header {
    margin-bottom: 0.72rem;
  }

  .thread-inspector-section-header h3,
  .thread-inspector-section-header p {
    margin: 0;
  }

  .thread-inspector-section-header h3 {
    font-size: var(--text-base);
    font-weight: 600;
    color: var(--ui-text-primary);
  }

  .thread-inspector-section-header > span {
    font-size: var(--text-sm);
    color: var(--ui-text-tertiary);
  }

  .structured-command-highlight {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    padding: 0.5rem 0.56rem;
    font-size: var(--text-sm);
    color: var(--ui-text-secondary);
  }

  .structured-command-highlight-tool {
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--ui-text-tertiary);
  }

  .structured-command-card-footer {
    font-size: var(--text-xs);
    color: var(--ui-text-tertiary);
  }

  .thread-inspector,
  .command-inspector {
    display: grid;
    gap: 0.85rem;
  }

  .thread-inspector-summary,
  .thread-inspector-section,
  .command-inspector-summary,
  .command-inspector-section {
    padding: 0.84rem 0.9rem;
  }

  .thread-inspector-summary-copy p,
  .command-inspector-summary-copy p {
    max-width: 44rem;
  }

  .handler-thread-empty,
  .project-ci-empty,
  .thread-inspector-empty,
  .thread-inspector-wait,
  .command-inspector-error,
  .command-inspector-empty {
    margin: 0;
    font-size: var(--text-sm);
    line-height: 1.55;
    color: var(--ui-text-secondary);
  }

  .thread-inspector-wait {
    display: grid;
    gap: 0.2rem;
    padding: 0.64rem 0.7rem;
    border: 1px solid color-mix(in oklab, var(--ui-status-waiting) 38%, var(--ui-border-soft));
    border-radius: var(--ui-radius-md);
    background: color-mix(in oklab, var(--ui-status-waiting-soft) 58%, var(--ui-surface));
  }

  .thread-inspector-wait strong,
  .thread-inspector-wait p {
    margin: 0;
  }

  .thread-inspector-wait strong {
    color: var(--ui-text-primary);
    font-size: var(--text-sm);
  }

  .command-inspector-error {
    color: color-mix(in oklab, var(--ui-danger) 80%, var(--ui-text-primary));
  }

  .handler-thread-empty,
  .project-ci-empty,
  .thread-inspector-empty,
  .command-inspector-empty {
    padding: 0.9rem;
    border-radius: var(--ui-radius-md);
    border: 1px dashed color-mix(in oklab, var(--ui-border-soft) 82%, transparent);
    background: color-mix(in oklab, var(--ui-surface-subtle) 72%, transparent);
  }

  .handler-thread-empty.error,
  .project-ci-empty.error,
  .thread-inspector-empty.error,
  .command-inspector-empty.error {
    border-color: color-mix(in oklab, var(--ui-danger) 32%, transparent);
    background: color-mix(in oklab, var(--ui-danger-soft) 72%, transparent);
  }

  .command-inspector-facts,
  .command-inspector-artifacts {
    display: grid;
    gap: 0.42rem;
    margin-top: 0.72rem;
  }

  .command-inspector-facts span,
  .command-inspector-artifacts span,
  .command-inspector-section-header p {
    font-size: var(--text-sm);
    color: var(--ui-text-secondary);
  }

  .command-inspector-facts pre {
    margin: 0;
    overflow: auto;
    padding: 0.72rem 0.76rem;
    border-radius: var(--ui-radius-sm);
    border: 1px solid color-mix(in oklab, var(--ui-border-soft) 84%, transparent);
    background: color-mix(in oklab, var(--ui-code) 92%, transparent);
    font-size: var(--text-sm);
    line-height: 1.56;
    color: var(--ui-text-primary);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    word-break: break-word;
  }

  .command-inspector-section-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 0.85rem;
    margin-bottom: 0.72rem;
  }

  .command-inspector-section-header h3,
  .command-inspector-section-header p {
    margin: 0;
  }

  .command-inspector-section-header h3 {
    font-size: var(--text-base);
    font-weight: 600;
    color: var(--ui-text-primary);
  }

  .command-inspector-section-header > span {
    font-size: var(--text-sm);
    color: var(--ui-text-tertiary);
  }

  .command-inspector-child {
    padding: 0.76rem 0.8rem;
  }

  .command-inspector-artifact {
    padding: 0.55rem 0.62rem;
  }

  .command-inspector-artifact-copy strong {
    font-size: var(--text-sm);
    font-weight: 600;
    color: var(--ui-text-primary);
  }

  .command-inspector-artifact-copy code {
    font-size: var(--text-xs);
    color: var(--ui-text-tertiary);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    word-break: break-word;
  }

  .artifact-missing {
    color: color-mix(in oklab, var(--ui-warning) 84%, var(--ui-text-primary));
  }

  .command-inspector-artifact-list.compact {
    margin-top: 0.72rem;
  }

  .session-dialog {
    display: grid;
    gap: 0.9rem;
  }

  .session-dialog-actions {
    display: flex;
    justify-content: flex-end;
    gap: 0.5rem;
  }

  .desktop-open {
    min-height: 0;
    padding-left: 0.72rem;
  }

  .mobile-slot {
    position: fixed;
    inset: 0;
    z-index: var(--ui-z-overlay);
  }

  .mobile-overlay {
    position: absolute;
    inset: 0;
    display: flex;
    justify-content: flex-end;
    padding: 0.8rem;
    background: color-mix(in oklab, var(--ui-overlay) 42%, transparent);
    backdrop-filter: blur(8px);
  }

  .titlebar-icon:hover {
    background: color-mix(in oklab, var(--ui-surface-subtle) 74%, transparent);
    color: var(--ui-text-primary);
  }

  .titlebar-icon-glyph {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 0;
  }

  .titlebar-icon:focus-visible {
    outline: none;
    box-shadow: var(--ui-focus-ring);
  }

  :global(body.sidebar-resizing) {
    cursor: col-resize;
    user-select: none;
  }

  :global(body.sidebar-resizing) .chat-workspace {
    transition: none;
  }

  @media (prefers-reduced-motion: reduce) {
    .chat-workspace,
    .workspace-titlebar,
    .workspace-sidebar,
    .sidebar-surface,
    .titlebar-icon,
    .header-icon-button,
    .pane-chrome-actions button {
      transition:
        grid-template-columns 0.01ms linear,
        width 0.01ms linear,
        transform 0.01ms linear,
        border-color 0.01ms linear,
        background-color 0.01ms linear,
        color 0.01ms linear,
        opacity 0.01ms linear;
    }

    .titlebar-icon:active:not(:disabled),
    .header-icon-button:active:not(:disabled),
    .pane-chrome-actions button:active:not(:disabled) {
      transform: none;
    }
  }

  @media (max-width: 1220px) {
    .chat-workspace.split {
      grid-template-columns: var(--sidebar-width) 0.72rem minmax(0, 1fr);
    }

    .chat-workspace.sidebar-hidden.split {
      grid-template-columns: 0rem 0rem minmax(0, 1fr);
    }
  }

  @media (max-width: 980px) {
    .workspace-main {
      padding-top: 0.2rem;
    }

    .workspace-main-header {
      flex-direction: column;
      align-items: stretch;
      padding-left: 0.4rem;
    }

    .workspace-main-meta {
      justify-content: flex-start;
    }

    .project-ci-header,
    .project-ci-entry,
    .project-ci-run-card-top,
    .project-ci-check-top,
    .project-ci-check-meta,
    .structured-command-header,
    .structured-command-card-top,
    .structured-command-card-footer,
    .command-inspector-summary-top,
    .command-inspector-child-top,
    .command-inspector-artifact,
    .command-inspector-section-header {
      flex-direction: column;
      align-items: stretch;
    }

    .structured-command-copy,
    .command-inspector-summary-copy p {
      max-width: none;
    }

    .project-ci-check-meta,
    .structured-command-card-meta,
    .command-inspector-summary-meta,
    .command-inspector-child-meta {
      justify-content: flex-start;
    }
  }

  @media (max-width: 760px) {
    .workspace-titlebar {
      padding-inline: 0.32rem;
    }

    .workspace-titlebar-start {
      padding-left: 0;
    }

    .chat-workspace {
      grid-template-columns: minmax(0, 1fr) !important;
      padding: 0 0 0.32rem;
      padding-bottom: 0;
    }

    .workspace-shell {
      margin-inline: 0;
    }

    .workspace-main {
      gap: 0.42rem;
      padding: 0.32rem 0.42rem 0;
    }

    .workspace-main-header {
      gap: 0.52rem;
      padding: 0;
    }

    .workspace-main-subtitle {
      white-space: normal;
      overflow-wrap: anywhere;
    }

    .workspace-main-meta {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      width: 100%;
      gap: 0.35rem;
    }

    .workspace-main-meta > span,
    .workspace-main-meta :global(.ui-button),
    .project-ci-compact {
      width: 100%;
      max-width: 100%;
      justify-content: center;
    }

    .workspace-main-chips {
      display: grid;
      grid-column: 1 / -1;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 0.35rem;
      min-width: 0;
    }

    .workspace-main-meta :global(.ui-metadata-chip),
    .workspace-main-meta :global(.context-budget-compact) {
      width: 100%;
      max-width: 100%;
      min-width: 0;
      flex: none;
    }

    .project-ci-compact {
      grid-column: 1 / -1;
      flex-wrap: wrap;
      justify-content: flex-start;
      padding: 0.38rem;
    }

    .new-session-empty {
      padding-inline: 0.72rem;
    }

    .new-session-controls {
      display: grid;
      justify-items: stretch;
    }

    .new-session-mode-toggle,
    .new-session-mode-buttons {
      width: 100%;
    }

    .new-session-mode-buttons button {
      flex: 1 1 0;
    }

    .new-session-mode-note {
      text-align: left;
      white-space: normal;
    }

    .structured-command-card-top,
    .structured-command-card-footer,
    .thread-inspector-summary-top,
    .thread-inspector-command-top,
    .thread-inspector-timeline-top,
    .thread-inspector-section-header,
    .command-inspector-summary-top,
    .command-inspector-child-top,
    .command-inspector-artifact {
      flex-direction: column;
      align-items: stretch;
    }

    .handler-thread-actions,
    .thread-inspector-actions,
    .thread-inspector-command-footer,
    .handler-thread-pills,
    .structured-command-card-meta,
    .thread-inspector-summary-meta,
    .thread-inspector-pills,
    .thread-inspector-command-meta,
    .command-inspector-summary-meta,
    .command-inspector-child-meta,
    .command-inspector-child-footer,
    .command-inspector-pills {
      justify-content: flex-start;
    }

    .thread-inspector-metadata {
      grid-template-columns: minmax(0, 1fr);
    }

    .mobile-overlay {
      padding: 0.42rem;
    }
  }
</style>
