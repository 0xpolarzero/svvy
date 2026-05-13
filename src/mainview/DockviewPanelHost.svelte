<script lang="ts">
  import ChatComposer from "./ChatComposer.svelte";
  import ChatTranscript from "./ChatTranscript.svelte";
  import RelatedInspectorPane from "./RelatedInspectorPane.svelte";
  import SavedWorkflowLibraryPane from "./SavedWorkflowLibraryPane.svelte";
  import WorkflowInspectorPane from "./WorkflowInspectorPane.svelte";
  import { projectConversation } from "./conversation-projection";
  import { buildSurfaceContextBudget } from "./context-budget";
  import { formatUsage } from "./chat-format";
  import type { PromptHistoryEntry } from "./prompt-history";
  import type { ChatRuntime } from "./chat-runtime";
  import type { ChatSurfaceController } from "./chat-runtime";
  import type { ThinkingLevel } from "@mariozechner/pi-agent-core";
  import { onDestroy, onMount } from "svelte";

  type Props = {
    runtime: ChatRuntime;
    panelId: string;
  };

  let { runtime, panelId }: Props = $props();
  let controller = $state<ChatSurfaceController | null>(null);
  let pane = $state<ReturnType<ChatRuntime["getPane"]> | null>(null);
  let promptHistory = $state<PromptHistoryEntry[]>([]);
  let messages = $state<ChatSurfaceController["agent"]["state"]["messages"]>([]);
  let streamMessage = $state<ChatSurfaceController["agent"]["state"]["streamMessage"]>(null);
  let pendingToolCalls = $state(new Set<string>());
  let isStreaming = $state(false);
  let errorMessage = $state<string | undefined>(undefined);
  let currentModel = $state<ChatSurfaceController["agent"]["state"]["model"] | null>(null);
  let currentThinkingLevel = $state<ThinkingLevel>("off");
  let unsubscribeRuntime = $state<(() => void) | null>(null);
  let unsubscribeController = $state<(() => void) | null>(null);

  const conversation = $derived(projectConversation(messages));
  const contextBudget = $derived(currentModel ? buildSurfaceContextBudget(messages, currentModel) : null);
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
      isStreaming = false;
      errorMessage = undefined;
      currentModel = null;
      currentThinkingLevel = "off";
      return;
    }

    messages = [...controller.agent.state.messages];
    streamMessage = controller.agent.state.streamMessage;
    pendingToolCalls = new Set(controller.agent.state.pendingToolCalls);
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

  async function send(input: string): Promise<boolean> {
    if (!controller || !input.trim()) return false;
    await runtime.focusPane(panelId);
    await runtime.storage.promptHistory.append({
      text: input.trim(),
      sentAt: Date.now(),
      workspaceId: runtime.workspaceId,
      sessionId: controller.target.workspaceSessionId,
    });
    await controller.agent.prompt(input);
    return true;
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
{:else if pane?.target?.surface === "saved-workflow-library"}
  <SavedWorkflowLibraryPane {runtime} />
{:else if pane?.target?.surface === "command" || pane?.target?.surface === "workflow-task-attempt" || pane?.target?.surface === "artifact" || pane?.target?.surface === "project-ci-check"}
  <RelatedInspectorPane {runtime} target={pane.target} />
{:else if controller}
  <section class="dockview-chat-panel" data-testid="workspace-pane">
    <ChatTranscript
      {conversation}
      sessionId={controller.agent.sessionId ?? controller.target.surfacePiSessionId}
      systemPrompt={controller.resolvedSystemPrompt}
      streamMessage={visibleStreamMessage}
      {pendingToolCalls}
      {isStreaming}
      workspaceMentionPaths={new Set()}
      onScrollStateChange={(scroll) => runtime.setPaneScroll(panelId, scroll)}
    />
    <ChatComposer
      currentModel={currentModel ?? controller.agent.state.model}
      thinkingLevel={currentThinkingLevel}
      {isStreaming}
      {errorMessage}
      {promptHistory}
      usageText={formatUsage(conversation.usage) || undefined}
      {contextBudget}
      sessionName={pane?.target?.surface === "thread" ? "Handler Thread" : "Orchestrator"}
      targetLabel={pane?.target?.surface === "thread" ? "Messaging handler thread" : "Messaging orchestrator"}
      worktreeLabel={runtime.branch ?? runtime.workspaceLabel}
      onAbort={() => void controller?.abort()}
      onSend={send}
      onThinkingChange={(level) => controller?.agent.setThinkingLevel(level)}
      listWorkspacePaths={() => runtime.listWorkspacePaths()}
      pickWorkspaceAttachments={() => runtime.pickWorkspaceAttachments()}
    />
  </section>
{:else}
  <section class="dockview-empty-panel" data-testid="workspace-pane">
    <p>{pane?.target ? "Surface unavailable" : "Empty panel"}</p>
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

  .dockview-empty-panel {
    display: grid;
    place-items: center;
    height: 100%;
    min-height: 0;
    color: var(--ui-text-muted);
    background: var(--ui-panel);
  }
</style>
