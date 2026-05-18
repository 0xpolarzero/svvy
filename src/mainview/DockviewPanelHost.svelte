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
  import { buildSurfaceContextBudget } from "./context-budget";
  import { getSurfaceDisplayTitle } from "./surface-title";
  import type { PromptHistoryEntry } from "./prompt-history";
  import type { ChatRuntime } from "./chat-runtime";
  import type { ChatSurfaceController } from "./chat-runtime";
  import type { QueuedPrompt } from "./chat-runtime";
  import type { WorkspaceTabInfo } from "../shared/workspace-contract";
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
  let promptHistory = $state<PromptHistoryEntry[]>([]);
  let messages = $state<ChatSurfaceController["agent"]["state"]["messages"]>([]);
  let streamMessage = $state<ChatSurfaceController["agent"]["state"]["streamMessage"]>(null);
  let pendingToolCalls = $state(new Set<string>());
  let queuedMessages = $state<QueuedPrompt[]>([]);
  let isStreaming = $state(false);
  let errorMessage = $state<string | undefined>(undefined);
  let currentModel = $state<ChatSurfaceController["agent"]["state"]["model"] | null>(null);
  let currentThinkingLevel = $state<ThinkingLevel>("off");
  let unsubscribeRuntime = $state<(() => void) | null>(null);
  let unsubscribeController = $state<(() => void) | null>(null);

  const conversation = $derived(projectConversation(messages));
  const contextBudget = $derived(currentModel ? buildSurfaceContextBudget(messages, currentModel) : null);
  const surfaceDisplayTitle = $derived(
    getSurfaceDisplayTitle(
      controller?.target,
      runtime.sessions,
      pane?.target?.surface === "thread" ? "Handler Thread" : "Orchestrator",
    ),
  );
  const visibleStreamMessage = $derived(
    controller?.promptStatus === "streaming" && streamMessage?.role === "assistant"
      ? streamMessage
      : undefined,
  );

  function syncSurfaceState() {
    if (!controller) {
      messages = [];
      streamMessage = null;
      pendingToolCalls = new Set();
      queuedMessages = [];
      isStreaming = false;
      errorMessage = undefined;
      currentModel = null;
      currentThinkingLevel = "off";
      return;
    }

    messages = [...controller.agent.state.messages];
    streamMessage = controller.agent.state.streamMessage;
    pendingToolCalls = new Set(controller.agent.state.pendingToolCalls);
    queuedMessages = [...controller.queuedPrompts];
    isStreaming = controller.agent.state.isStreaming || controller.promptStatus === "streaming";
    errorMessage = controller.agent.state.error;
    currentModel = controller.agent.state.model;
    currentThinkingLevel = controller.agent.state.thinkingLevel as ThinkingLevel;
  }

  function syncPanel() {
    pane = runtime.getPane(panelId) ?? null;
    const nextController = runtime.getPaneController(panelId);
    if (nextController !== controller) {
      unsubscribeController?.();
      controller = nextController;
      unsubscribeController = controller?.subscribe(syncSurfaceState) ?? null;
    }
    syncSurfaceState();
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
  <section class="dockview-chat-panel" class:has-prompt-banner={controller.promptBinding?.stale} data-testid="workspace-pane">
    {#if controller.promptBinding?.stale}
      <div class="prompt-stale-banner" role="status">
        This session is using older instructions than the current Context settings.
      </div>
    {/if}
    <ChatTranscript
      {conversation}
      target={controller.target}
      sessionId={controller.agent.sessionId ?? controller.target.surfacePiSessionId}
      systemPrompt={controller.resolvedSystemPrompt}
      streamMessage={visibleStreamMessage}
      currentModel={currentModel ?? controller.agent.state.model}
      {pendingToolCalls}
      {isStreaming}
      workspaceMentionPaths={new Set()}
      initialScroll={pane?.scroll ?? null}
      onScrollStateChange={(scroll) => runtime.setPaneScroll(panelId, scroll)}
      onForkAssistantMessage={(message) => void forkFromAssistantMessage(message.timestamp)}
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
  <section class="dockview-empty-panel" data-testid="workspace-pane">
    <p>Surface unavailable</p>
  </section>
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
    padding: 0.45rem 0.75rem;
    border-bottom: 1px solid color-mix(in oklab, var(--ui-warning-border, var(--ui-border-soft)) 84%, transparent);
    background: color-mix(in oklab, var(--ui-warning-surface, var(--ui-surface-subtle)) 88%, transparent);
    color: var(--ui-text-secondary);
    font-size: var(--text-xs);
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
