<script lang="ts">
  import CheckIcon from "@lucide/svelte/icons/check";
  import ChevronDownIcon from "@lucide/svelte/icons/chevron-down";
  import ChevronRightIcon from "@lucide/svelte/icons/chevron-right";
  import CopyPlusIcon from "@lucide/svelte/icons/copy-plus";
  import GripVerticalIcon from "@lucide/svelte/icons/grip-vertical";
  import LockIcon from "@lucide/svelte/icons/lock";
  import PlusIcon from "@lucide/svelte/icons/plus";
  import Trash2Icon from "@lucide/svelte/icons/trash-2";
  import { getModel, type Model } from "@mariozechner/pi-ai";
  import { onDestroy } from "svelte";
  import { flip } from "svelte/animate";
  import type {
    AgentProfileId,
    AgentProfileSettings,
    AgentSettingsState,
    ReasoningEffort,
  } from "../shared/agent-settings";
  import { getSupportedThinkingLevels } from "./model-thinking";
  import {
    getModelComboboxValue,
    listModelComboboxOptions,
    type ModelComboboxOption,
  } from "./model-options";
  import type { ChatRuntime } from "./chat-runtime";
  import { rpc } from "./rpc";
  import Button from "./ui/Button.svelte";
  import Checkbox from "./ui/Checkbox.svelte";
  import CompactCombobox from "./ui/CompactCombobox.svelte";
  import CompactSelect, { type CompactSelectOption } from "./ui/CompactSelect.svelte";
  import Input from "./ui/Input.svelte";
  import Tooltip from "./ui/Tooltip.svelte";
  import { dismissConfirmation } from "./ui/dismiss-confirmation";
  import { queuedMessageOrderChanged, reorderQueuedMessageItems } from "./queued-message-order";

  type Props = {
    runtime: ChatRuntime;
    panelId: string;
    onSettingsChanged?: (settings: AgentSettingsState) => void;
  };

  let { runtime, panelId, onSettingsChanged }: Props = $props();

  let settings = $state<AgentSettingsState | null>(null);
  let loading = $state(true);
  let errorMessage = $state<string | null>(null);
  let savingProfileId = $state<string | null>(null);
  let deletingProfileId = $state<string | null>(null);
  let confirmingDeleteProfileId = $state<string | null>(null);
  let expandedProfileIds = $state<Set<string>>(new Set());
  let modelOptionsByProfileId = $state<Record<string, ModelComboboxOption[]>>({});
  let configuredProviders = $state<string[]>([]);
  let orchestratorRowsElement = $state<HTMLElement | null>(null);
  let profileDrag = $state<{
    profileId: string;
    pointerId: number;
    startY: number;
    lastY: number;
    didMove: boolean;
  } | null>(null);
  let dragCaptureElement: HTMLElement | null = null;
  let draggedProfileId = $state<string | null>(null);
  let dropBeforeProfileId = $state<string | null>(null);
  let pendingDragClientY: number | null = null;
  let dragAnimationFrame: number | null = null;

  const orchestrators = $derived(settings?.agents.orchestrators ?? []);
  const displayedOrchestrators = $derived(
    reorderQueuedMessageItems(orchestrators, draggedProfileId, dropBeforeProfileId),
  );
  const threadHandler = $derived(settings?.agents.special.threadHandler ?? null);

  async function loadSettings() {
    loading = true;
    errorMessage = null;
    try {
      const [nextSettings, nextConfiguredProviders] = await Promise.all([
        runtime.getAgentSettings(),
        runtime.listConfiguredProviders().catch(() => []),
      ]);
      settings = nextSettings;
      onSettingsChanged?.(nextSettings);
      configuredProviders = nextConfiguredProviders;
    } catch (error) {
      errorMessage = error instanceof Error ? error.message : "Unable to load agent profiles.";
    } finally {
      loading = false;
    }
  }

  function profileModel(profile: AgentProfileSettings): Model<any> {
    try {
      return getModel(
        profile.provider as Parameters<typeof getModel>[0],
        profile.model as Parameters<typeof getModel>[1],
      );
    } catch {
      return {
        id: profile.model,
        name: profile.model,
        provider: profile.provider,
        reasoning: false,
      } as Model<any>;
    }
  }

  function modelValue(profile: AgentProfileSettings): string {
    return getModelComboboxValue(profileModel(profile));
  }

  function reasoningOptions(profile: AgentProfileSettings): CompactSelectOption[] {
    return getSupportedThinkingLevels(profileModel(profile)).map((level) => ({
      value: level,
      label: level,
    }));
  }

  async function loadModelOptions(profile: AgentProfileSettings) {
    modelOptionsByProfileId = {
      ...modelOptionsByProfileId,
      [profile.id]: await listModelComboboxOptions(
        profileModel(profile),
        runtime.storage,
        configuredProviders,
      ),
    };
  }

  function mutateProfile(profile: AgentProfileSettings): AgentProfileSettings {
    return {
      ...profile,
      extensions: [...profile.extensions],
    };
  }

  async function saveProfile(profile: AgentProfileSettings) {
    savingProfileId = profile.id;
    errorMessage = null;
    try {
      settings = await rpc.request.updateAgentProfile({
        workspaceId: runtime.workspaceId,
        profile: mutateProfile(profile),
      });
      onSettingsChanged?.(settings);
    } catch (error) {
      errorMessage = error instanceof Error ? error.message : "Unable to save agent profile.";
    } finally {
      savingProfileId = null;
    }
  }

  function createProfileId(baseName: string): string {
    const slug = baseName
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/^-+|-+$/g, "")
      .slice(0, 36);
    const prefix = slug || "orchestrator";
    const existingIds = new Set(orchestrators.map((profile) => profile.id));
    let index = orchestrators.length + 1;
    let id = `${prefix}-${index}`;
    while (existingIds.has(id)) {
      index += 1;
      id = `${prefix}-${index}`;
    }
    return id;
  }

  async function createOrchestratorProfile(source?: AgentProfileSettings) {
    const baseProfile = source ?? orchestrators[0];
    if (!baseProfile) return;
    const name = source ? `${source.name} copy` : `Orchestrator ${orchestrators.length + 1}`;
    const profile: AgentProfileSettings = {
      ...mutateProfile(baseProfile),
      id: createProfileId(name),
      kind: "orchestrator",
      name,
      builtin: false,
      locked: false,
    };
    await saveProfile(profile);
    expandedProfileIds.add(profile.id);
    expandedProfileIds = new Set(expandedProfileIds);
  }

  function requestDeleteProfile(profile: AgentProfileSettings) {
    if (profile.locked || deletingProfileId) return;
    confirmingDeleteProfileId = profile.id;
  }

  async function deleteProfile(profile: AgentProfileSettings) {
    if (profile.locked || deletingProfileId || confirmingDeleteProfileId !== profile.id) return;
    deletingProfileId = profile.id;
    errorMessage = null;
    try {
      settings = await rpc.request.deleteAgentProfile({
        workspaceId: runtime.workspaceId,
        id: profile.id,
      });
      onSettingsChanged?.(settings);
      confirmingDeleteProfileId = null;
      expandedProfileIds.delete(profile.id);
      expandedProfileIds = new Set(expandedProfileIds);
    } catch (error) {
      errorMessage = error instanceof Error ? error.message : "Unable to delete agent profile.";
    } finally {
      deletingProfileId = null;
    }
  }

  function cancelDeleteProfileConfirmation() {
    confirmingDeleteProfileId = null;
  }

  function getDropTarget(clientY: number): string | null {
    if (!orchestratorRowsElement) return null;

    const rowElements = Array.from(
      orchestratorRowsElement.querySelectorAll<HTMLElement>("[data-reorderable='true']"),
    );
    for (const rowElement of rowElements) {
      if (rowElement.dataset.profileId === draggedProfileId) continue;
      const bounds = rowElement.getBoundingClientRect();
      if (clientY < bounds.top + bounds.height / 2) {
        return rowElement.dataset.profileId ?? null;
      }
    }

    return null;
  }

  function clearDragFrame() {
    if (dragAnimationFrame === null) return;
    window.cancelAnimationFrame(dragAnimationFrame);
    dragAnimationFrame = null;
    pendingDragClientY = null;
  }

  onDestroy(() => {
    clearDragFrame();
    removeDragListeners();
  });

  function addDragListeners() {
    window.addEventListener("pointermove", handleWindowPointerMove);
    window.addEventListener("pointerup", handleWindowPointerUp);
    window.addEventListener("pointercancel", handleWindowPointerCancel);
    window.addEventListener("blur", cancelPointerDrag);
  }

  function removeDragListeners() {
    window.removeEventListener("pointermove", handleWindowPointerMove);
    window.removeEventListener("pointerup", handleWindowPointerUp);
    window.removeEventListener("pointercancel", handleWindowPointerCancel);
    window.removeEventListener("blur", cancelPointerDrag);
  }

  function handlePointerDown(event: PointerEvent, profile: AgentProfileSettings) {
    if (event.button !== 0 || !event.isPrimary) return;
    if (profile.locked) return;
    clearDragFrame();
    removeDragListeners();
    profileDrag = {
      profileId: profile.id,
      pointerId: event.pointerId,
      startY: event.clientY,
      lastY: event.clientY,
      didMove: false,
    };
    draggedProfileId = null;
    dropBeforeProfileId = null;
    dragCaptureElement = event.currentTarget as HTMLElement;
    dragCaptureElement.setPointerCapture(event.pointerId);
    addDragListeners();
  }

  function applyDragMove(clientY: number) {
    if (!profileDrag) return;

    const didMove = profileDrag.didMove || Math.abs(clientY - profileDrag.startY) > 5;
    if (!didMove) return;

    if (!profileDrag.didMove) {
      draggedProfileId = profileDrag.profileId;
    }
    profileDrag = { ...profileDrag, didMove: true, lastY: clientY };
    const beforeProfileId = getDropTarget(clientY);
    if (beforeProfileId !== dropBeforeProfileId) {
      dropBeforeProfileId = beforeProfileId;
    }
  }

  function scheduleDragMove(clientY: number) {
    if (profileDrag) {
      profileDrag = { ...profileDrag, lastY: clientY };
    }
    pendingDragClientY = clientY;
    if (dragAnimationFrame !== null) return;

    dragAnimationFrame = window.requestAnimationFrame(() => {
      dragAnimationFrame = null;
      const nextClientY = pendingDragClientY;
      pendingDragClientY = null;
      if (nextClientY !== null) {
        applyDragMove(nextClientY);
      }
    });
  }

  function handleWindowPointerMove(event: PointerEvent) {
    if (!profileDrag || event.pointerId !== profileDrag.pointerId) return;
    scheduleDragMove(event.clientY);
    if (profileDrag.didMove || Math.abs(event.clientY - profileDrag.startY) > 5) {
      event.preventDefault();
    }
  }

  function handleWindowPointerUp(event: PointerEvent) {
    if (!profileDrag || event.pointerId !== profileDrag.pointerId) return;
    void finishPointerDrag(event.clientY);
  }

  function handleWindowPointerCancel(event: PointerEvent) {
    if (!profileDrag || event.pointerId !== profileDrag.pointerId) return;
    cancelPointerDrag();
  }

  function cancelPointerDrag() {
    if (!profileDrag) return;
    releasePointerCapture(profileDrag.pointerId);
    clearDragFrame();
    removeDragListeners();
    profileDrag = null;
    draggedProfileId = null;
    dropBeforeProfileId = null;
  }

  function releasePointerCapture(pointerId: number) {
    if (dragCaptureElement?.hasPointerCapture(pointerId)) {
      dragCaptureElement.releasePointerCapture(pointerId);
    }
    dragCaptureElement = null;
  }

  async function finishPointerDrag(clientY: number) {
    if (!profileDrag) return;

    applyDragMove(clientY);
    clearDragFrame();

    const completedDrag = profileDrag.didMove;
    const profileId = profileDrag.profileId;
    const pointerId = profileDrag.pointerId;
    const beforeProfileId = dropBeforeProfileId;
    const shouldCommitReorder =
      completedDrag && queuedMessageOrderChanged(orchestrators, profileId, beforeProfileId);
    profileDrag = null;
    draggedProfileId = null;
    dropBeforeProfileId = null;
    releasePointerCapture(pointerId);
    removeDragListeners();
    if (!shouldCommitReorder) return;

    savingProfileId = profileId;
    errorMessage = null;
    try {
      const nextIds = reorderQueuedMessageItems(orchestrators, profileId, beforeProfileId).map(
        (candidate) => candidate.id,
      );
      settings = await rpc.request.reorderOrchestratorAgents({
        workspaceId: runtime.workspaceId,
        ids: nextIds,
      });
      onSettingsChanged?.(settings);
    } catch (error) {
      errorMessage = error instanceof Error ? error.message : "Unable to reorder profiles.";
    } finally {
      savingProfileId = null;
    }
  }

  function toggleExpanded(profileId: string) {
    if (expandedProfileIds.has(profileId)) {
      expandedProfileIds.delete(profileId);
    } else {
      expandedProfileIds.add(profileId);
    }
    expandedProfileIds = new Set(expandedProfileIds);
  }

  async function updateName(profile: AgentProfileSettings, name: string) {
    const nextName = name.trim();
    if (!nextName || nextName === profile.name) return;
    await saveProfile({ ...profile, name: nextName });
  }

  async function updateModel(profile: AgentProfileSettings, value: string) {
    const option = modelOptionsByProfileId[profile.id]?.find((candidate) => candidate.value === value);
    const [provider, model] = value.split(":");
    const nextModel = option?.model ?? profileModel({ ...profile, provider, model });
    const supported = getSupportedThinkingLevels(nextModel);
    const reasoningEffort = supported.includes(profile.reasoningEffort)
      ? profile.reasoningEffort
      : ((supported.includes("medium") ? "medium" : (supported[0] ?? "off")) as ReasoningEffort);
    await saveProfile({
      ...profile,
      provider: nextModel.provider,
      model: nextModel.id,
      reasoningEffort,
    });
  }

  async function updateReasoning(profile: AgentProfileSettings, value: string) {
    await saveProfile({ ...profile, reasoningEffort: value as ReasoningEffort });
  }

  async function updateExtensions(profile: AgentProfileSettings, extensions: string[]) {
    await saveProfile({ ...profile, extensions });
  }

  async function updateComposerSync(profile: AgentProfileSettings, updateFromComposer: boolean) {
    await saveProfile({ ...profile, updateFromComposer });
  }

  $effect(() => {
    void panelId;
    void loadSettings();
  });
</script>

<section class="agents-pane" data-testid="agents-pane" data-panel-id={panelId}>
  <header class="agents-header">
    <div>
      <h2>Agents</h2>
      <p>Profiles used by orchestrators and delegated handler threads.</p>
    </div>
  </header>

  {#if loading}
    <p class="agents-status">Loading agent profiles...</p>
  {:else if errorMessage}
    <p class="agents-error">{errorMessage}</p>
  {:else if settings}
    <div class="agent-category">
      <div class="agent-category-heading">
        <span>Orchestrators</span>
        <div class="agent-category-actions">
          <small>{orchestrators.length}</small>
          <Button
            variant="ghost"
            size="xs"
            class="category-action"
            disabled={savingProfileId !== null}
            onclick={() => void createOrchestratorProfile()}
          >
            <PlusIcon size={13} aria-hidden="true" />
            New
          </Button>
        </div>
      </div>
      <div class="agent-rows" bind:this={orchestratorRowsElement}>
        {#each displayedOrchestrators as profile (profile.id)}
          {@const expanded = expandedProfileIds.has(profile.id)}
          <article
            class={`agent-profile-row ${expanded ? "expanded" : ""} ${profile.id === draggedProfileId ? "dragging" : ""}`.trim()}
            data-profile-id={profile.id}
            data-reorderable={profile.locked ? "false" : "true"}
            animate:flip={{ duration: 170 }}
          >
            {@render profileRowContent(profile, "orchestrator", expanded)}
          </article>
        {/each}
      </div>
    </div>

    <div class="agent-category">
      <div class="agent-category-heading">
        <span>Special Profiles</span>
        <small>built in</small>
      </div>
      <div class="agent-rows">
        {#if threadHandler}
          {@const expanded = expandedProfileIds.has(threadHandler.id)}
          <article
            class={`agent-profile-row ${expanded ? "expanded" : ""}`.trim()}
            data-profile-id={threadHandler.id}
            data-reorderable="false"
          >
            {@render profileRowContent(threadHandler, "special", expanded)}
          </article>
        {/if}
      </div>
      <p class="category-todo">TODO: default workflow agents.</p>
    </div>
  {:else}
    <p class="agents-error">Agent settings are unavailable.</p>
  {/if}
</section>

{#snippet profileRowContent(
  profile: AgentProfileSettings,
  category: "orchestrator" | "special",
  expanded: boolean,
)}
  {@const modelOptions = modelOptionsByProfileId[profile.id] ?? []}
  <div class="agent-profile-main">
    {#if category === "orchestrator"}
      <button
        class="agent-drag-handle"
        type="button"
        aria-label={profile.locked ? `${profile.name} stays first` : `Reorder ${profile.name}`}
        disabled={profile.locked}
        onpointerdown={(event) => handlePointerDown(event, profile)}
      >
        {#if profile.locked}
          <LockIcon size={12} aria-hidden="true" />
        {:else}
          <GripVerticalIcon size={13} aria-hidden="true" />
        {/if}
      </button>
    {:else}
      <span class="agent-drag-placeholder"><LockIcon size={12} aria-hidden="true" /></span>
    {/if}
    {#if profile.locked}
      <span class="agent-locked-name">{category === "orchestrator" ? "Default" : profile.name}</span>
    {:else}
      <Input
        value={profile.name}
        class="agent-name-input"
        aria-label={`${profile.name} name`}
        disabled={savingProfileId === profile.id}
        onblur={(event) => void updateName(profile, event.currentTarget.value)}
        onkeydown={(event) => {
          if (event.key === "Enter") {
            event.currentTarget.blur();
          }
        }}
      />
    {/if}
    <div class="agent-middle-controls">
      <div class="agent-controls">
        <CompactCombobox
          value={modelValue(profile)}
          options={modelOptions.length > 0 ? modelOptions : [{ value: modelValue(profile), label: profile.model, triggerLabel: profile.model }]}
          ariaLabel={`${profile.name} model`}
          placeholder="Model"
          triggerClass="model-pill agent-model-field"
          menuClass="model-menu"
          placement="below"
          disabled={savingProfileId === profile.id}
          onBeforeOpen={() => loadModelOptions(profile)}
          onSelect={(value) => updateModel(profile, value)}
        />
        <CompactSelect
          value={profile.reasoningEffort}
          options={reasoningOptions(profile)}
          ariaLabel={`${profile.name} reasoning`}
          triggerClass="model-pill agent-reasoning-field"
          menuClass="thinking-menu"
          textTransform="lowercase"
          placement="below"
          disabled={savingProfileId === profile.id}
          onSelect={(value) => updateReasoning(profile, value)}
        />
        <CompactCombobox
          values={profile.extensions}
          multiple
          options={[]}
          ariaLabel={`${profile.name} extensions`}
          placeholder="Extensions"
          emptyLabel="No extensions available."
          triggerClass="model-pill extensions-field"
          menuClass="extensions-menu"
          placement="below"
          disabled={savingProfileId === profile.id}
          onMultiSelect={(values) => updateExtensions(profile, values)}
        />
      </div>
      <Tooltip
        label="Composer inheritance"
        details={[
          {
            label:
              "When enabled, new orchestrators created from this profile use the latest model and reasoning picked directly in the composer.",
          },
        ]}
      >
        <label class="composer-sync-field">
          <Checkbox
            size="sm"
            checked={profile.updateFromComposer}
            disabled={savingProfileId === profile.id}
            onchange={(event) => void updateComposerSync(profile, event.currentTarget.checked)}
          />
          <span>Follow composer</span>
        </label>
      </Tooltip>
      <div
        class="agent-row-actions"
        use:dismissConfirmation={{
          active: confirmingDeleteProfileId === profile.id,
          onDismiss: cancelDeleteProfileConfirmation,
        }}
      >
        {#if category === "orchestrator"}
          <Tooltip label="Duplicate profile">
            <button
              type="button"
              class="agent-icon-button"
              aria-label={`Duplicate ${profile.name}`}
              disabled={savingProfileId === profile.id}
              onclick={() => void createOrchestratorProfile(profile)}
            >
              <CopyPlusIcon size={13} aria-hidden="true" />
            </button>
          </Tooltip>
        {:else}
          <span class="agent-action-spacer" aria-hidden="true"></span>
        {/if}
        {#if confirmingDeleteProfileId === profile.id}
          <Tooltip label="Confirm delete">
            <button
              type="button"
              class="agent-icon-button danger"
              aria-label={`Confirm deleting ${profile.name}`}
              disabled={deletingProfileId === profile.id || savingProfileId === profile.id}
              onclick={() => void deleteProfile(profile)}
            >
              <CheckIcon size={13} aria-hidden="true" />
            </button>
          </Tooltip>
        {:else}
          <Tooltip label={profile.locked ? "Locked profile cannot be deleted" : "Delete profile"}>
            <button
              type="button"
              class="agent-icon-button danger"
              aria-label={`Delete ${profile.name}`}
              disabled={profile.locked || deletingProfileId === profile.id || savingProfileId === profile.id}
              onclick={() => requestDeleteProfile(profile)}
            >
              <Trash2Icon size={13} aria-hidden="true" />
            </button>
          </Tooltip>
        {/if}
      </div>
    </div>
    <button
      type="button"
      class="agent-expand-button"
      aria-expanded={expanded}
      aria-label={expanded ? `Collapse ${profile.name}` : `Expand ${profile.name}`}
      onclick={() => toggleExpanded(profile.id)}
    >
      {#if expanded}
        <ChevronDownIcon size={14} strokeWidth={1.9} aria-hidden="true" />
      {:else}
        <ChevronRightIcon size={14} strokeWidth={1.9} aria-hidden="true" />
      {/if}
    </button>
  </div>
  {#if expanded}
    <div class="agent-profile-expanded">
      <p>TODO: expanded profile prompt, extension, and generated contract preview.</p>
    </div>
  {/if}
{/snippet}

<style>
  .agents-pane {
    display: flex;
    flex-direction: column;
    gap: 0.72rem;
    height: 100%;
    min-height: 0;
    overflow-x: hidden;
    overflow-y: auto;
    padding: 0.72rem;
    background: var(--ui-panel);
    color: var(--ui-text-primary);
  }

  .agents-header {
    display: flex;
    align-items: flex-end;
    justify-content: space-between;
    gap: 0.72rem;
    padding-bottom: 0.48rem;
    border-bottom: 1px solid var(--ui-border-soft);
  }

  .agents-header h2 {
    margin: 0;
    font-size: var(--text-base);
    font-weight: 650;
    line-height: 1.2;
  }

  .agents-header p,
  .agents-status,
  .category-todo {
    margin: 0.18rem 0 0;
    color: var(--ui-text-tertiary);
    font-size: var(--text-sm);
    line-height: 1.45;
  }

  .agents-error {
    margin: 0;
    padding: 0.62rem 0.7rem;
    border: 1px solid color-mix(in oklab, var(--ui-danger) 30%, var(--ui-border-soft));
    border-radius: var(--ui-radius-sm);
    background: color-mix(in oklab, var(--ui-danger-soft) 64%, transparent);
    color: var(--ui-danger);
    font-size: var(--text-sm);
  }

  .agent-category {
    display: grid;
    gap: 0.36rem;
    min-width: 0;
  }

  .agent-category-heading {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    color: var(--ui-text-secondary);
    font-size: var(--text-xs);
    font-weight: 650;
    text-transform: uppercase;
  }

  .agent-category-heading small {
    color: var(--ui-text-tertiary);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    font-weight: 500;
    text-transform: none;
  }

  .agent-category-actions {
    display: inline-flex;
    align-items: center;
    gap: 0.36rem;
  }

  :global(.category-action) {
    min-height: 1.42rem;
  }

  .agent-rows {
    display: grid;
    gap: 0.22rem;
    min-width: 0;
  }

  .agent-profile-row {
    display: grid;
    gap: 0.28rem;
    min-width: 0;
    overflow-x: clip;
    overflow-y: visible;
    padding: 0.3rem 0.36rem;
    border: 1px solid color-mix(in oklab, var(--ui-border-soft) 82%, transparent);
    border-radius: var(--ui-radius-sm);
    background: color-mix(in oklab, var(--ui-surface) 82%, transparent);
    transition:
      opacity 150ms cubic-bezier(0.19, 1, 0.22, 1),
      transform 170ms cubic-bezier(0.19, 1, 0.22, 1),
      border-color 150ms cubic-bezier(0.19, 1, 0.22, 1),
      background-color 150ms cubic-bezier(0.19, 1, 0.22, 1);
  }

  .agent-profile-row.expanded {
    border-color: color-mix(in oklab, var(--ui-accent) 22%, var(--ui-border-soft));
    background: color-mix(in oklab, var(--ui-surface-raised) 78%, transparent);
  }

  .agent-profile-row.dragging {
    opacity: 0.58;
  }

  .agent-profile-main {
    --agent-row-line-height: 1.45rem;

    display: flex;
    align-items: flex-start;
    gap: 0.36rem;
    min-width: 0;
  }

  .agent-drag-handle,
  .agent-drag-placeholder,
  .agent-expand-button {
    display: grid;
    place-items: center;
    flex: 0 0 auto;
    width: 1.24rem;
    height: var(--agent-row-line-height);
    border: 0;
    border-radius: var(--ui-radius-sm);
    background: transparent;
    color: var(--ui-text-tertiary);
  }

  .agent-drag-handle {
    cursor: grab;
    touch-action: none;
  }

  .agent-drag-handle:disabled,
  .agent-drag-placeholder {
    cursor: default;
    opacity: 0.72;
  }

  .agent-profile-row.dragging .agent-drag-handle {
    cursor: grabbing;
  }

  .agent-expand-button {
    cursor: pointer;
  }

  .agent-drag-handle:not(:disabled):hover,
  .agent-drag-handle:not(:disabled):focus-visible,
  .agent-expand-button:hover,
  .agent-expand-button:focus-visible {
    outline: none;
    background: var(--ui-hover-bg);
    color: var(--ui-text-primary);
  }

  .agent-drag-handle:not(:disabled):focus-visible,
  .agent-expand-button:focus-visible,
  .agent-icon-button:focus-visible {
    box-shadow: var(--ui-focus-ring);
  }

  .agent-locked-name {
    display: flex;
    align-items: center;
    flex: 1 1 4.5rem;
    box-sizing: border-box;
    min-width: 0;
    height: var(--agent-row-line-height);
    min-height: 0;
    overflow: hidden;
    padding: 0 0.34rem;
    color: var(--ui-text-primary);
    font-size: var(--text-sm);
    font-weight: 600;
    line-height: 1.25;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  :global(.agent-name-input.ui-input) {
    box-sizing: border-box;
    flex: 1 1 4.5rem;
    min-width: 4.5rem;
    height: var(--agent-row-line-height);
    min-height: 0;
    padding: 0 0.34rem;
    border-color: transparent;
    background: transparent;
    font-weight: 600;
  }

  :global(.agent-name-input.ui-input:hover),
  :global(.agent-name-input.ui-input:focus-visible) {
    border-color: var(--ui-border-soft);
    background: var(--ui-bg-elevated);
  }

  .agent-middle-controls {
    display: flex;
    align-items: center;
    align-content: flex-start;
    justify-content: flex-end;
    gap: 0.36rem;
    flex: 0 1 auto;
    flex-wrap: wrap;
    min-width: 0;
    min-height: var(--agent-row-line-height);
    row-gap: 0.18rem;
  }

  .agent-controls {
    display: inline-flex;
    align-items: center;
    align-content: flex-start;
    gap: 0.36rem;
    flex: 0 1 auto;
    flex-wrap: wrap;
    min-width: 0;
    row-gap: 0.18rem;
  }

  .agent-controls :global(.compact-combobox),
  .agent-controls :global(.compact-select) {
    flex: 0 1 auto;
    min-width: 0;
  }

  :global(.compact-combobox-trigger.agent-model-field) {
    width: fit-content;
    max-width: clamp(6.6rem, 13vw, 8.8rem);
  }

  :global(.compact-select-trigger.agent-reasoning-field) {
    width: fit-content;
    max-width: clamp(4.9rem, 9vw, 5.8rem);
  }

  :global(.compact-combobox-trigger.extensions-field) {
    width: fit-content;
    max-width: clamp(5.8rem, 10vw, 6.8rem);
  }

  :global(.extensions-menu) {
    min-width: 10rem;
    max-width: min(15rem, calc(100vw - 2rem));
  }

  .composer-sync-field {
    display: inline-flex;
    align-items: center;
    gap: 0.28rem;
    flex: 0 0 auto;
    min-height: var(--agent-row-line-height);
    color: var(--ui-text-tertiary);
    font-size: var(--text-xs);
    line-height: 1;
    white-space: nowrap;
  }

  .agent-row-actions {
    display: inline-flex;
    align-items: center;
    gap: 0.08rem;
    flex: 0 0 auto;
    min-height: var(--agent-row-line-height);
  }

  .agent-icon-button {
    display: grid;
    place-items: center;
    width: 1.32rem;
    height: var(--agent-row-line-height);
    border: 0;
    border-radius: var(--ui-radius-sm);
    background: transparent;
    color: var(--ui-text-tertiary);
    cursor: pointer;
  }

  .agent-icon-button:hover,
  .agent-icon-button:focus-visible {
    outline: none;
    background: var(--ui-hover-bg);
    color: var(--ui-text-primary);
  }

  .agent-icon-button.danger:hover,
  .agent-icon-button.danger:focus-visible {
    background: var(--ui-danger-soft);
    color: var(--ui-danger);
  }

  .agent-icon-button:disabled {
    cursor: default;
    opacity: 0.36;
  }

  .agent-action-spacer {
    width: 1.32rem;
    height: 1.32rem;
  }

  .agent-profile-expanded {
    margin-left: 1.46rem;
    padding: 0.5rem 0.58rem;
    border: 1px dashed color-mix(in oklab, var(--ui-border-soft) 82%, transparent);
    border-radius: var(--ui-radius-sm);
    color: var(--ui-text-tertiary);
    font-size: var(--text-sm);
  }

  .agent-profile-expanded p {
    margin: 0;
  }
</style>
