<script lang="ts">
  import CheckIcon from "@lucide/svelte/icons/check";
  import PencilIcon from "@lucide/svelte/icons/pencil";
  import SaveIcon from "@lucide/svelte/icons/save";
  import { onMount, tick } from "svelte";
  import {
    getPromptLibraryContentKey,
    type PromptLibrarySnapshotSummary,
  } from "../shared/prompt-library";
  import type { ChatRuntime } from "./chat-runtime";
  import Button from "./ui/Button.svelte";
  import CompactCombobox, { type CompactComboboxOption } from "./ui/CompactCombobox.svelte";
  import Tooltip from "./ui/Tooltip.svelte";

  type Props = {
    runtime: ChatRuntime;
    panelId: string;
  };

  let { runtime, panelId }: Props = $props();

  let snapshots = $state<PromptLibrarySnapshotSummary[]>([]);
  let selectedSnapshotId = $state("");
  let snapshotPopoverOpen = $state(false);
  let snapshotName = $state("");
  let renamingSnapshotId = $state<string | null>(null);
  let renameSnapshotName = $state("");
  let root = $state<HTMLDivElement | null>(null);
  let snapshotNameInput = $state<HTMLInputElement | null>(null);
  let renameSnapshotInput = $state<HTMLInputElement | null>(null);
  let currentContentKey = $state("");
  let saving = $state(false);

  const snapshotOptions = $derived(
    snapshots.map((snapshot) => ({
      value: snapshot.id,
      label: snapshot.name,
      triggerLabel: snapshot.name,
      searchText: `${snapshot.name} ${formatSnapshotTime(snapshot.createdAt)}`,
    })),
  );
  const selectedSnapshot = $derived(
    snapshots.find((snapshot) => snapshot.id === selectedSnapshotId) ?? null,
  );
  const currentSnapshot = $derived(
    snapshots.find((snapshot) => snapshot.contentKey === currentContentKey) ?? null,
  );
  const displayedSnapshotId = $derived(currentSnapshot?.id ?? "");
  const snapshotPlaceholder = $derived(
    snapshots.length > 0 ? "Current context" : "No snapshots",
  );
  const loadSnapshotTooltip = $derived(
    currentSnapshot
      ? `Current Context matches ${currentSnapshot.name}`
      : snapshots.length > 0
      ? "Current Context is not saved as a snapshot; choose one to restore"
      : "No saved Context snapshots yet",
  );

  function defaultSnapshotName(date = new Date()): string {
    return `Snapshot ${new Intl.DateTimeFormat(undefined, {
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    }).format(date)}`;
  }

  function formatSnapshotTime(value: string): string {
    return new Intl.DateTimeFormat(undefined, {
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    }).format(new Date(value));
  }

  async function flushPromptLibraryPane(): Promise<void> {
    let flushPromise: Promise<void> | null = null;
    window.dispatchEvent(
      new CustomEvent("svvy:prompt-library-flush", {
        detail: {
          panelId,
          register: (promise: Promise<void>) => {
            flushPromise = promise;
          },
        },
      }),
    );
    await (flushPromise ?? Promise.resolve());
  }

  function notifyPromptLibraryPaneChanged(): void {
    window.dispatchEvent(
      new CustomEvent("svvy:prompt-library-reload", {
        detail: { panelId },
      }),
    );
  }

  function closeSnapshotPopover(): void {
    snapshotPopoverOpen = false;
    renamingSnapshotId = null;
  }

  async function refreshSnapshotState() {
    const [snapshotList, currentState] = await Promise.all([
      runtime.listPromptLibrarySnapshots(),
      runtime.getPromptLibrary(),
    ]);
    snapshots = snapshotList;
    currentContentKey = getPromptLibraryContentKey(currentState);
    const matchingSnapshot = snapshotList.find((snapshot) => snapshot.contentKey === currentContentKey);
    selectedSnapshotId = matchingSnapshot?.id ?? "";
    if (selectedSnapshotId && !snapshotList.some((snapshot) => snapshot.id === selectedSnapshotId)) {
      selectedSnapshotId = "";
    }
  }

  async function openSnapshotPopover() {
    await flushPromptLibraryPane();
    snapshotName = defaultSnapshotName();
    snapshotPopoverOpen = true;
    renamingSnapshotId = null;
    await tick();
    snapshotNameInput?.focus();
    snapshotNameInput?.select();
  }

  async function createSnapshot() {
    const name = snapshotName.trim();
    if (!name) return;
    saving = true;
    try {
      await flushPromptLibraryPane();
      const snapshot = await runtime.createPromptLibrarySnapshot(name);
      await refreshSnapshotState();
      selectedSnapshotId = snapshot.id;
      snapshotPopoverOpen = false;
    } finally {
      saving = false;
    }
  }

  async function loadSnapshot(snapshotId: string) {
    await flushPromptLibraryPane();
    saving = true;
    try {
      await runtime.restorePromptLibrarySnapshot(snapshotId);
      selectedSnapshotId = snapshotId;
      await refreshSnapshotState();
      notifyPromptLibraryPaneChanged();
    } finally {
      saving = false;
    }
  }

  async function startRenameSnapshot() {
    if (!selectedSnapshot) return;
    renamingSnapshotId = selectedSnapshot.id;
    renameSnapshotName = selectedSnapshot.name;
    snapshotPopoverOpen = false;
    await tick();
    renameSnapshotInput?.focus();
    renameSnapshotInput?.select();
  }

  async function renameSnapshot() {
    const snapshotId = renamingSnapshotId;
    const name = renameSnapshotName.trim();
    if (!snapshotId || !name) return;
    saving = true;
    try {
      const renamed = await runtime.renamePromptLibrarySnapshot(snapshotId, name);
      await refreshSnapshotState();
      selectedSnapshotId = renamed.id;
      renamingSnapshotId = null;
    } finally {
      saving = false;
    }
  }

  onMount(() => {
    const handlePointerDown = (event: PointerEvent) => {
      if (!snapshotPopoverOpen && !renamingSnapshotId) return;
      const target = event.target;
      if (target instanceof Node && root?.contains(target)) return;
      closeSnapshotPopover();
    };
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      if (!snapshotPopoverOpen && !renamingSnapshotId) return;
      event.preventDefault();
      closeSnapshotPopover();
    };
    const handlePromptLibraryChanged = (event: Event) => {
      const detail = (event as CustomEvent<{ panelId?: string; unsnapshotted?: boolean }>).detail;
      if (detail?.panelId !== panelId) return;
      if (detail.unsnapshotted) {
        selectedSnapshotId = "";
        currentContentKey = "";
        return;
      }
      void refreshSnapshotState();
    };

    document.addEventListener("pointerdown", handlePointerDown, true);
    window.addEventListener("keydown", handleKeyDown, true);
    window.addEventListener("svvy:prompt-library-current-changed", handlePromptLibraryChanged);

    return () => {
      document.removeEventListener("pointerdown", handlePointerDown, true);
      window.removeEventListener("keydown", handleKeyDown, true);
      window.removeEventListener("svvy:prompt-library-current-changed", handlePromptLibraryChanged);
    };
  });

  $effect(() => {
    void refreshSnapshotState();
  });
</script>

<div
  bind:this={root}
  class="prompt-snapshot-controls"
  role="presentation"
  onpointerdown={(event) => event.stopPropagation()}
>
  <Tooltip label={loadSnapshotTooltip}>
    <CompactCombobox
      value={displayedSnapshotId}
      options={snapshotOptions}
      ariaLabel="Load saved Context snapshot"
      placeholder={snapshotPlaceholder}
      emptyLabel="No snapshots saved."
      disabled={saving || snapshots.length === 0}
      triggerClass="dock-snapshot-select"
      menuClass="dock-snapshot-menu"
      optionClass="dock-snapshot-option"
      placement="below"
      onBeforeOpen={refreshSnapshotState}
      onSelect={(snapshotId) => void loadSnapshot(snapshotId)}
    />
  </Tooltip>
  <Tooltip label="Rename selected Context snapshot" disabled={!selectedSnapshot || saving}>
    <Button
      class="dock-snapshot-button"
      variant="ghost"
      size="xs"
      iconOnly
      disabled={!selectedSnapshot || saving}
      aria-label="Rename selected snapshot"
      onclick={() => void startRenameSnapshot()}
    >
      <PencilIcon aria-hidden="true" size={13} strokeWidth={1.9} />
    </Button>
  </Tooltip>
  <Tooltip label="Save current instructions, context packs, actors, and scopes">
    <Button
      class="dock-snapshot-button"
      variant="ghost"
      size="xs"
      iconOnly
      disabled={saving}
          aria-label="Save current Context snapshot"
      onclick={() => void openSnapshotPopover()}
    >
      <SaveIcon aria-hidden="true" size={13} strokeWidth={1.9} />
    </Button>
  </Tooltip>

  {#if snapshotPopoverOpen}
    <div class="snapshot-popover" role="dialog" aria-label="Create context snapshot">
      <input
        bind:this={snapshotNameInput}
        class="snapshot-name-input"
        bind:value={snapshotName}
        aria-label="Snapshot name"
        onkeydown={(event) => {
          if (event.key === "Enter") void createSnapshot();
          if (event.key === "Escape") closeSnapshotPopover();
        }}
      />
      <Tooltip label="Save current Context snapshot">
        <Button
          class="dock-snapshot-button"
          variant="ghost"
          size="xs"
          iconOnly
          disabled={saving || !snapshotName.trim()}
          aria-label="Save snapshot"
          onclick={() => void createSnapshot()}
        >
          <CheckIcon aria-hidden="true" size={13} strokeWidth={2.1} />
        </Button>
      </Tooltip>
    </div>
  {/if}

  {#if renamingSnapshotId}
    <div class="snapshot-popover" role="dialog" aria-label="Rename context snapshot">
      <input
        bind:this={renameSnapshotInput}
        class="snapshot-name-input"
        bind:value={renameSnapshotName}
        aria-label="Snapshot name"
        onkeydown={(event) => {
          if (event.key === "Enter") void renameSnapshot();
          if (event.key === "Escape") closeSnapshotPopover();
        }}
      />
      <Tooltip label="Rename snapshot">
        <Button
          class="dock-snapshot-button"
          variant="ghost"
          size="xs"
          iconOnly
          disabled={saving || !renameSnapshotName.trim()}
          aria-label="Rename snapshot"
          onclick={() => void renameSnapshot()}
        >
          <CheckIcon aria-hidden="true" size={13} strokeWidth={2.1} />
        </Button>
      </Tooltip>
    </div>
  {/if}
</div>

<style>
  .prompt-snapshot-controls {
    position: relative;
    display: inline-flex;
    align-items: center;
    gap: 0.16rem;
    min-width: 0;
    height: 100%;
    margin-right: 1.25rem;
  }

  :global(.dock-snapshot-select) {
    justify-content: space-between;
    width: 8.5rem;
    min-width: 0;
    min-height: 1.42rem;
    padding: 0.1rem 0.22rem 0.1rem 0.42rem;
    border: 0;
    border-radius: var(--ui-radius-sm);
    background: color-mix(in oklab, var(--ui-surface-muted) 30%, transparent);
    color: var(--ui-text-secondary);
    font-size: var(--text-xs);
    font-weight: 600;
  }

  :global(.dock-snapshot-select .compact-combobox-label) {
    flex: 1 1 auto;
    text-align: left;
  }

  :global(.dock-snapshot-select:hover:not(:disabled)),
  :global(.dock-snapshot-select:focus-visible) {
    outline: none;
    background: color-mix(in oklab, var(--ui-surface-muted) 46%, transparent);
    color: var(--ui-text-primary);
  }

  :global(.dock-snapshot-select:disabled) {
    opacity: 0.6;
  }

  :global(.dock-snapshot-menu) {
    min-width: 20rem;
    max-width: min(34rem, calc(100vw - 2rem));
  }

  :global(.dock-snapshot-option) {
    font-size: var(--text-xs);
  }

  :global(.dock-snapshot-button) {
    width: 1.42rem;
    height: 1.42rem;
    min-height: 1.42rem;
    padding: 0;
    border: 0;
    border-radius: var(--ui-radius-sm);
    background: transparent;
    color: var(--ui-text-tertiary);
    box-shadow: none;
  }

  :global(.dock-snapshot-button:hover:not(:disabled)),
  :global(.dock-snapshot-button:focus-visible) {
    outline: none;
    background: color-mix(in oklab, var(--ui-surface-muted) 46%, transparent);
    color: var(--ui-text-primary);
  }

  .snapshot-popover {
    position: absolute;
    z-index: var(--ui-z-dialog);
    top: calc(100% + 0.22rem);
    right: 0;
    display: grid;
    grid-template-columns: minmax(13rem, 1fr) max-content;
    align-items: center;
    gap: 0.16rem;
    width: min(22rem, calc(100vw - 1rem));
    padding: 0.2rem;
    border: 1px solid color-mix(in oklab, var(--ui-border-soft) 72%, transparent);
    border-radius: var(--ui-radius-sm);
    background: color-mix(in oklab, var(--ui-surface-raised) 96%, transparent);
    box-shadow:
      0 10px 24px color-mix(in oklab, var(--ui-shadow) 18%, transparent),
      0 0 0 1px color-mix(in oklab, var(--ui-surface) 42%, transparent);
  }

  .snapshot-name-input {
    width: 100%;
    min-height: 1.55rem;
    padding: 0.18rem 0.42rem;
    border: 1px solid transparent;
    border-radius: var(--ui-radius-sm);
    background: color-mix(in oklab, var(--ui-surface-muted) 34%, transparent);
    color: var(--ui-text-primary);
    font-size: var(--text-sm);
    line-height: 1.2;
  }

  .snapshot-name-input:focus-visible {
    outline: none;
    border-color: color-mix(in oklab, var(--ui-accent) 36%, transparent);
    box-shadow: 0 0 0 1px color-mix(in oklab, var(--ui-accent) 16%, transparent);
  }
</style>
