<script lang="ts">
	import ChatComposer from "./ChatComposer.svelte";
	import type { ComposerSubmit } from "./ChatComposer.svelte";
  import ChatTranscript from "./ChatTranscript.svelte";
  import RelatedInspectorPane from "./RelatedInspectorPane.svelte";
  import AppLogsPane from "./AppLogsPane.svelte";
  import OpenWorkspacePanel from "./OpenWorkspacePanel.svelte";
  import PromptLibraryPane from "./PromptLibraryPane.svelte";
  import SavedWorkflowLibraryPane from "./SavedWorkflowLibraryPane.svelte";
  import WorkflowInspectorPane from "./WorkflowInspectorPane.svelte";
  import { projectConversation } from "./conversation-projection";
  import { getVisibleCommandRollups } from "./command-inspector";
  import { buildSurfaceContextBudget } from "./context-budget";
  import { getSurfaceDisplayTitle } from "./surface-title";
  import {
    buildTranscriptSemanticBlocks,
    type TranscriptSemanticBlock,
  } from "./transcript-projection";
  import type { PromptHistoryEntry } from "./prompt-history";
  import type { ChatRuntime } from "./chat-runtime";
  import type { ChatSurfaceController } from "./chat-runtime";
  import type { QueuedPrompt } from "./chat-runtime";
  import type {
    WorkspaceHandlerThreadSummary,
    WorkspaceSessionSummary,
    WorkspaceTabInfo,
  } from "../shared/workspace-contract";
  import type { ThinkingLevel } from "@mariozechner/pi-agent-core";
  import { onDestroy, onMount } from "svelte";
  import { listModelComboboxOptions } from "./model-options";

  type Props = {
    runtime: ChatRuntime;
    panelId: string;
    onOpenModelPicker: (panelId: string) => void;
    openingWorkspace?: boolean;
    openWorkspaceError?: string | null;
    recentWorkspaces?: WorkspaceTabInfo[];
    onOpenWorkspace?: () => void;
    onOpenWorkspaceInNewTab?: () => void;
  };

  let {
    runtime,
    panelId,
    onOpenModelPicker,
    openingWorkspace = false,
    openWorkspaceError = null,
    recentWorkspaces = [],
    onOpenWorkspace,
    onOpenWorkspaceInNewTab,
  }: Props = $props();
  let controller = $state<ChatSurfaceController | null>(null);
  let pane = $state<ReturnType<ChatRuntime["getPane"]> | null>(null);
  let sessions = $state<WorkspaceSessionSummary[]>([]);
  let promptHistory = $state<PromptHistoryEntry[]>([]);
  let messages = $state<ChatSurfaceController["agent"]["state"]["messages"]>([]);
  let pendingToolCalls = $state(new Set<string>());
  let queuedMessages = $state<QueuedPrompt[]>([]);
  let promptBinding = $state<ChatSurfaceController["promptBinding"]>(undefined);
  let resolvedSystemPrompt = $state("");
  let isStreaming = $state(false);
  let errorMessage = $state<string | undefined>(undefined);
  let currentModel = $state<ChatSurfaceController["agent"]["state"]["model"] | null>(null);
  let currentThinkingLevel = $state<ThinkingLevel>("off");
  let handlerThreads = $state<WorkspaceHandlerThreadSummary[]>([]);
  let handlerThreadsSessionId = $state<string | null>(null);
  let handlerThreadLoadToken = 0;
  let controllerRevision = $state(0);
  let workspaceMentionPaths = $state<ReadonlySet<string>>(new Set());
  let unsubscribeRuntime = $state<(() => void) | null>(null);
  let unsubscribeController = $state<(() => void) | null>(null);

  const conversation = $derived(projectConversation(messages));
  const currentSession = $derived<WorkspaceSessionSummary | null>(
    controller
      ? (sessions.find(
          (session) => session.id === controller?.target.workspaceSessionId,
        ) ?? null)
      : null,
  );
  const currentCommandRollups = $derived(getVisibleCommandRollups(currentSession));
  const transcriptSemanticBlocks = $derived(
    buildTranscriptSemanticBlocks({
      session: currentSession,
      errorMessage,
      commandRollups: currentCommandRollups,
      handlerThreads,
    }),
  );
  const contextBudget = $derived(currentModel ? buildSurfaceContextBudget(messages, currentModel) : null);
  const surfaceDisplayTitle = $derived(
    getSurfaceDisplayTitle(
      controller?.target,
      sessions,
      pane?.target?.surface === "thread" ? "Handler Thread" : "Orchestrator",
    ),
  );
  const visibleStreamMessage = $derived.by(() => {
    void controllerRevision;
    const message = controller?.agent.state.streamMessage;
    return controller?.promptStatus === "streaming" && message?.role === "assistant"
      ? message
      : undefined;
  });
  const queuedPromptRefresh = $derived(
    queuedMessages.find((message) => message.kind === "prompt_refresh") ?? null,
  );

  function syncSurfaceState() {
    controllerRevision += 1;
    if (!controller) {
      messages = [];
      pendingToolCalls = new Set();
      queuedMessages = [];
      promptBinding = undefined;
      resolvedSystemPrompt = "";
      isStreaming = false;
      errorMessage = undefined;
      currentModel = null;
      currentThinkingLevel = "off";
      handlerThreads = [];
      handlerThreadsSessionId = null;
      handlerThreadLoadToken += 1;
      return;
    }

    messages = [...controller.agent.state.messages];
    pendingToolCalls = new Set(controller.agent.state.pendingToolCalls);
    queuedMessages = [...controller.queuedPrompts];
    promptBinding = controller.promptBinding;
    resolvedSystemPrompt = controller.resolvedSystemPrompt;
    isStreaming = controller.agent.state.isStreaming || controller.promptStatus === "streaming";
    errorMessage = controller.agent.state.error;
    currentModel = controller.agent.state.model;
    currentThinkingLevel = controller.agent.state.thinkingLevel as ThinkingLevel;
  }

  function syncPanel() {
    pane = runtime.getPane(panelId) ?? null;
    sessions = [...runtime.sessions];
    const nextController = runtime.getPaneController(panelId);
    if (nextController !== controller) {
      unsubscribeController?.();
      controller = nextController;
      unsubscribeController = controller?.subscribe(syncSurfaceState) ?? null;
    }
    syncSurfaceState();
    refreshHandlerThreadBlocks();
  }

  function refreshHandlerThreadBlocks() {
    if (!controller || controller.target.surface !== "orchestrator") {
      handlerThreads = [];
      handlerThreadsSessionId = null;
      handlerThreadLoadToken += 1;
      return;
    }

    const sessionId = controller.target.workspaceSessionId;
    if (handlerThreadsSessionId !== sessionId) {
      handlerThreads = [];
    }
    handlerThreadsSessionId = sessionId;
    const loadToken = ++handlerThreadLoadToken;
    void runtime
      .listHandlerThreads(sessionId)
      .then((nextThreads) => {
        if (loadToken !== handlerThreadLoadToken) return;
        handlerThreads = nextThreads;
      })
      .catch(() => {
        if (loadToken !== handlerThreadLoadToken) return;
        handlerThreads = [];
      });
  }

  function transcriptSplitTarget() {
    return { kind: "split" as const, panelId, direction: "right" as const };
  }

	async function send(input: ComposerSubmit): Promise<boolean> {
		if (!controller || (!input.text.trim() && input.attachments.length === 0)) return false;
		await runtime.focusPane(panelId);
		if (input.text.trim()) {
			await runtime.storage.promptHistory.append({
				text: input.text.trim(),
				sentAt: Date.now(),
				workspaceId: runtime.workspaceId,
				sessionId: controller.target.workspaceSessionId,
			});
		}
		await controller.sendPrompt(input);
		return true;
	}

  async function listModelsForComposer() {
    if (!currentModel) return [];
    const configuredProviders = await runtime.listConfiguredProviders().catch(() => []);
    return listModelComboboxOptions(currentModel, runtime.storage, configuredProviders);
  }

  async function forkFromAssistantMessage(messageTimestamp: string | number): Promise<void> {
    if (!controller) return;
    await runtime.forkSession(
      controller.target.workspaceSessionId,
      undefined,
      { kind: "new-panel", direction: "right" },
      { messageTimestamp },
    );
  }

  async function openArtifactFromTranscript(filename: string): Promise<void> {
    await openWorkspacePathFromTranscript(filename);
  }

  async function openWorkspacePathFromTranscript(path: string): Promise<void> {
    const opened = await runtime.openWorkspacePath(path).catch(() => false);
    if (!opened) {
      await runtime.writeClipboardText(path).catch(() => undefined);
    }
  }

  function inspectCommandFromTranscript(commandId: string): void {
    if (!controller) return;
    void runtime.openSurface(
      {
        workspaceSessionId: controller.target.workspaceSessionId,
        surface: "command",
        commandId,
      },
      transcriptSplitTarget(),
    );
  }

  function openHandlerThreadFromTranscript(threadId: string): void {
    if (!controller) return;
    const thread = handlerThreads.find((candidate) => candidate.threadId === threadId);
    if (!thread) return;
    void runtime.openSurface(
      {
        workspaceSessionId: controller.target.workspaceSessionId,
        surface: "thread",
        surfacePiSessionId: thread.surfacePiSessionId,
        threadId: thread.threadId,
      },
      transcriptSplitTarget(),
    );
  }

  function inspectWorkflowFromTranscript(workflowRunId: string): void {
    if (!controller) return;
    void runtime.openSurface(
      {
        workspaceSessionId: controller.target.workspaceSessionId,
        surface: "workflow-inspector",
        workflowRunId,
      },
      transcriptSplitTarget(),
    );
  }

  function inspectWorkflowTaskAttemptFromTranscript(workflowTaskAttemptId: string): void {
    if (!controller) return;
    void runtime.openSurface(
      {
        workspaceSessionId: controller.target.workspaceSessionId,
        surface: "workflow-task-attempt",
        workflowTaskAttemptId,
      },
      transcriptSplitTarget(),
    );
  }

  async function replyToWaitFromTranscript(
    block: TranscriptSemanticBlock & { kind: "wait" },
    text: string,
  ): Promise<void> {
    if (!controller) return;
    const targetThread = block.threadId
      ? handlerThreads.find((thread) => thread.threadId === block.threadId)
      : null;
    if (targetThread) {
      await runtime.sendPromptToTarget(
        {
          workspaceSessionId: controller.target.workspaceSessionId,
          surface: "thread",
          surfacePiSessionId: targetThread.surfacePiSessionId,
          threadId: targetThread.threadId,
        },
        text,
      );
      return;
    }
    await runtime.sendPromptToTarget(controller.target, text);
  }

  async function retryFailureFromTranscript(
    block: TranscriptSemanticBlock & { kind: "failure" },
  ): Promise<void> {
    if (!controller) return;
    await runtime.sendPromptToTarget(
      controller.target,
      `Retry the failed turn and address this failure:\n\n${block.summary}`,
    );
  }

  onMount(() => {
    syncPanel();
    unsubscribeRuntime = runtime.subscribe(syncPanel);
    void runtime.storage.promptHistory
      .list(runtime.workspaceId)
      .then((entries) => {
        promptHistory = entries;
      })
      .catch(() => {
        promptHistory = [];
      });
    void runtime
      .listWorkspacePaths()
      .then((paths) => {
        workspaceMentionPaths = new Set(paths.map((path) => path.workspaceRelativePath));
      })
      .catch(() => {
        workspaceMentionPaths = new Set();
      });
  });

  onDestroy(() => {
    unsubscribeRuntime?.();
    unsubscribeRuntime = null;
    unsubscribeController?.();
    unsubscribeController = null;
  });
</script>

{#if pane?.target?.surface === "workflow-inspector"}
  <WorkflowInspectorPane {runtime} sessionId={pane.target.workspaceSessionId} workflowRunId={pane.target.workflowRunId} {panelId} />
{:else if pane?.target?.surface === "app-logs"}
  <AppLogsPane {runtime} {panelId} />
{:else if pane?.target?.surface === "prompt-library"}
  <PromptLibraryPane {runtime} {panelId} />
{:else if pane?.target?.surface === "saved-workflow-library"}
  <SavedWorkflowLibraryPane {runtime} />
{:else if pane?.target?.surface === "open-workspace"}
  <OpenWorkspacePanel
    {openingWorkspace}
    errorMessage={openWorkspaceError}
    {recentWorkspaces}
    onOpenWorkspace={() => onOpenWorkspace?.()}
    onOpenWorkspaceInNewTab={onOpenWorkspaceInNewTab ? () => onOpenWorkspaceInNewTab?.() : undefined}
  />
{:else if pane?.target?.surface === "command" || pane?.target?.surface === "workflow-task-attempt" || pane?.target?.surface === "artifact" || pane?.target?.surface === "project-ci-check"}
  <RelatedInspectorPane {runtime} target={pane.target} />
{:else if controller}
  <section
    class="dockview-chat-panel"
    class:has-prompt-banner={promptBinding?.stale}
    data-testid="workspace-pane"
    data-panel-id={panelId}
  >
    {#if promptBinding?.stale}
      <div class="prompt-stale-banner" role="status">
        <span>
          {queuedPromptRefresh
            ? "Context update queued for this surface."
            : "This surface is using older instructions than the current Context settings."}
        </span>
        {#if queuedPromptRefresh}
          <button type="button" onclick={() => void controller.deleteQueuedPrompt(queuedPromptRefresh.id)}>
            Cancel update
          </button>
        {:else}
          <button type="button" onclick={() => void controller.queuePromptRefresh()}>
            Update system prompt
          </button>
        {/if}
      </div>
    {/if}
    <ChatTranscript
      {conversation}
      target={controller.target}
      sessionId={controller.agent.sessionId ?? controller.target.surfacePiSessionId}
      systemPrompt={resolvedSystemPrompt}
      streamMessage={visibleStreamMessage}
      currentModel={currentModel ?? controller.agent.state.model}
      {pendingToolCalls}
      {isStreaming}
      semanticBlocks={transcriptSemanticBlocks}
      {workspaceMentionPaths}
      initialScroll={pane?.scroll ?? null}
      onScrollStateChange={(scroll) => runtime.setPaneScroll(panelId, scroll)}
      onOpenArtifact={(filename) => void openArtifactFromTranscript(filename)}
      onOpenWorkspacePath={(path) => void openWorkspacePathFromTranscript(path)}
      onInspectCommand={inspectCommandFromTranscript}
      onOpenHandlerThread={openHandlerThreadFromTranscript}
      onInspectWorkflow={inspectWorkflowFromTranscript}
      onInspectWorkflowTaskAttempt={inspectWorkflowTaskAttemptFromTranscript}
      onForkAssistantMessage={(message) => void forkFromAssistantMessage(message.timestamp)}
      onReplyToWait={(block, text) => void replyToWaitFromTranscript(block, text)}
      onRetryFailure={(block) => void retryFailureFromTranscript(block)}
    />
    <ChatComposer
      currentModel={currentModel ?? controller.agent.state.model}
      thinkingLevel={currentThinkingLevel}
      {isStreaming}
      {errorMessage}
      {promptHistory}
      {queuedMessages}
      {contextBudget}
      sessionName={surfaceDisplayTitle}
      targetLabel={pane?.target?.surface === "thread" ? "Messaging handler thread" : "Messaging orchestrator"}
      worktreeLabel={runtime.branch ?? runtime.workspaceLabel}
      onAbort={() => void controller?.abort()}
      onOpenModelPicker={() => onOpenModelPicker(panelId)}
      onListModels={listModelsForComposer}
      onModelChange={(model) => {
        currentModel = model;
        controller?.agent.setModel(model);
      }}
      onSend={send}
      onEditQueuedMessage={(promptId) => controller.editQueuedPrompt(promptId)}
      onDeleteQueuedMessage={(promptId) => void controller.deleteQueuedPrompt(promptId)}
      onSteerQueuedMessage={(promptId) => void controller.steerQueuedPrompt(promptId)}
      onReorderQueuedMessage={(promptId, beforePromptId) =>
        void controller.reorderQueuedPrompt(promptId, beforePromptId)}
      onThinkingChange={(level) => controller?.agent.setThinkingLevel(level)}
      listWorkspacePaths={(options) => runtime.listWorkspacePaths(options)}
      pickWorkspaceAttachments={() => runtime.pickWorkspaceAttachments()}
      importComposerAttachments={(files) => runtime.importComposerAttachments(files)}
    />
  </section>
{:else}
  <section
    class="dockview-empty-panel"
    aria-hidden="true"
    data-testid="workspace-pane"
    data-panel-id={panelId}
  ></section>
{/if}

<style>
  .dockview-chat-panel {
    display: grid;
    grid-template-rows: minmax(0, 1fr) auto;
    height: 100%;
    min-height: 0;
    overflow: hidden;
  }

  .dockview-chat-panel.has-prompt-banner {
    grid-template-rows: auto minmax(0, 1fr) auto;
  }

  .prompt-stale-banner {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    padding: 0.45rem 0.75rem;
    border-bottom: 1px solid color-mix(in oklab, var(--ui-warning-border, var(--ui-border-soft)) 84%, transparent);
    background: color-mix(in oklab, var(--ui-warning-surface, var(--ui-surface-subtle)) 88%, transparent);
    color: var(--ui-text-secondary);
    font-size: var(--text-xs);
  }

  .prompt-stale-banner span {
    min-width: 0;
  }

  .prompt-stale-banner button {
    flex: 0 0 auto;
    border: 1px solid color-mix(in oklab, var(--ui-warning-border, var(--ui-border-soft)) 82%, transparent);
    border-radius: 0.25rem;
    background: var(--ui-surface);
    color: var(--ui-text-primary);
    font: inherit;
    font-weight: 700;
    padding: 0.25rem 0.5rem;
    cursor: pointer;
  }

  .prompt-stale-banner button:hover {
    border-color: var(--ui-border-strong);
    background: var(--ui-surface-hover);
  }

  .dockview-empty-panel {
    display: grid;
    place-items: center;
    height: 100%;
    min-height: 0;
    color: var(--ui-text-muted);
    background: var(--ui-panel);
  }
</style>
