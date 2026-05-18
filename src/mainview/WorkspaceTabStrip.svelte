<script lang="ts">
  import PlusIcon from "@lucide/svelte/icons/plus";
  import XIcon from "@lucide/svelte/icons/x";
  import { onDestroy } from "svelte";
  import { flip } from "svelte/animate";
  import type { WorkspaceTabInfo } from "../shared/workspace-contract";
  import Tooltip from "./ui/Tooltip.svelte";
  import {
    formatWorkspaceTabAriaLabel,
    getVisibleWorkspaceTabCounts,
    type WorkspaceTabCounts,
  } from "./workspace-tabs";

  export type WorkspaceTabStripItem = {
    workspace: WorkspaceTabInfo;
    counts: WorkspaceTabCounts;
  };

  type Props = {
    tabs: WorkspaceTabStripItem[];
    activeWorkspaceTabId: string | null;
    openingWorkspace?: boolean;
    onSelectWorkspace: (workspaceTabId: string) => void;
    onCloseWorkspace: (workspaceTabId: string) => void;
    onNewWorkspaceTab: () => void;
    onReorderWorkspace?: (workspaceTabId: string, beforeWorkspaceTabId: string | null) => void;
  };

  let {
    tabs,
    activeWorkspaceTabId,
    openingWorkspace = false,
    onSelectWorkspace,
    onCloseWorkspace,
    onNewWorkspaceTab,
    onReorderWorkspace,
  }: Props = $props();

  let tabsElement = $state<HTMLElement | null>(null);
  let tabDrag = $state<{
    workspaceTabId: string;
    pointerId: number;
    startX: number;
    didMove: boolean;
  } | null>(null);
  let suppressClickWorkspaceTabId: string | null = null;
  let draggedWorkspaceTabId = $state<string | null>(null);
  let dropBeforeWorkspaceTabId = $state<string | null>(null);
  let pendingDragClientX: number | null = null;
  let dragAnimationFrame: number | null = null;

  const workspaceTabLabel = (tab: WorkspaceTabStripItem): string =>
    formatWorkspaceTabAriaLabel(tab.workspace, tab.counts);

  function getDropTarget(clientX: number): string | null {
    if (!tabsElement) return null;

    const tabElements = Array.from(
      tabsElement.querySelectorAll<HTMLElement>("[data-workspace-tab-id]"),
    );
    for (const tabElement of tabElements) {
      if (tabElement.dataset.workspaceTabId === draggedWorkspaceTabId) {
        continue;
      }

      const bounds = tabElement.getBoundingClientRect();
      if (clientX < bounds.left + bounds.width / 2) {
        return tabElement.dataset.workspaceTabId ?? null;
      }
    }

    return null;
  }

  function clearDragFrame() {
    if (dragAnimationFrame === null) return;
    window.cancelAnimationFrame(dragAnimationFrame);
    dragAnimationFrame = null;
    pendingDragClientX = null;
  }

  onDestroy(clearDragFrame);

  function handlePointerDown(event: PointerEvent, workspaceTabId: string) {
    if (event.button !== 0 || !event.isPrimary) return;
    clearDragFrame();
    tabDrag = {
      workspaceTabId,
      pointerId: event.pointerId,
      startX: event.clientX,
      didMove: false,
    };
    draggedWorkspaceTabId = null;
    dropBeforeWorkspaceTabId = null;
    (event.currentTarget as HTMLButtonElement).setPointerCapture(event.pointerId);
  }

  function applyDragMove(clientX: number) {
    if (!tabDrag) return;

    const didMove = tabDrag.didMove || Math.abs(clientX - tabDrag.startX) > 5;
    if (!didMove) return;

    if (!tabDrag.didMove) {
      draggedWorkspaceTabId = tabDrag.workspaceTabId;
    }
    const workspaceTabId = tabDrag.workspaceTabId;
    tabDrag = { ...tabDrag, didMove: true };
    const beforeWorkspaceTabId = getDropTarget(clientX);
    if (beforeWorkspaceTabId !== dropBeforeWorkspaceTabId) {
      dropBeforeWorkspaceTabId = beforeWorkspaceTabId;
      onReorderWorkspace?.(workspaceTabId, beforeWorkspaceTabId);
    }
  }

  function scheduleDragMove(clientX: number) {
    pendingDragClientX = clientX;
    if (dragAnimationFrame !== null) return;

    dragAnimationFrame = window.requestAnimationFrame(() => {
      dragAnimationFrame = null;
      const nextClientX = pendingDragClientX;
      pendingDragClientX = null;
      if (nextClientX !== null) {
        applyDragMove(nextClientX);
      }
    });
  }

  function handlePointerMove(event: PointerEvent) {
    if (!tabDrag || event.pointerId !== tabDrag.pointerId) return;
    scheduleDragMove(event.clientX);
    if (tabDrag.didMove || Math.abs(event.clientX - tabDrag.startX) > 5) {
      event.preventDefault();
    }
  }

  function finishPointerDrag(event: PointerEvent, suppressClick: boolean) {
    if (!tabDrag || event.pointerId !== tabDrag.pointerId) return;

    applyDragMove(event.clientX);
    clearDragFrame();

    const completedDrag = tabDrag.didMove;
    const workspaceTabId = tabDrag.workspaceTabId;
    tabDrag = null;
    draggedWorkspaceTabId = null;
    dropBeforeWorkspaceTabId = null;
    const target = event.currentTarget as HTMLButtonElement;
    if (target.hasPointerCapture(event.pointerId)) {
      target.releasePointerCapture(event.pointerId);
    }

    if (suppressClick && completedDrag) {
      suppressClickWorkspaceTabId = workspaceTabId;
      window.setTimeout(() => {
        if (suppressClickWorkspaceTabId === workspaceTabId) {
          suppressClickWorkspaceTabId = null;
        }
      });
      event.preventDefault();
    }
  }

  function handleSelectWorkspace(workspaceTabId: string) {
    if (suppressClickWorkspaceTabId === workspaceTabId) {
      suppressClickWorkspaceTabId = null;
      return;
    }
    onSelectWorkspace(workspaceTabId);
  }
</script>

<div class="workspace-tabs-strip electrobun-webkit-app-region-no-drag" aria-label="Workspace tabs">
  <div class="workspace-tabs-scroll electrobun-webkit-app-region-no-drag" role="list" bind:this={tabsElement}>
    {#each tabs as tab (tab.workspace.workspaceTabId)}
      <div
        class={`workspace-tab electrobun-webkit-app-region-no-drag ${tab.workspace.workspaceTabId === activeWorkspaceTabId ? "active" : ""} ${tab.workspace.workspaceTabId === draggedWorkspaceTabId ? "dragging" : ""}`.trim()}
        data-workspace-tab-id={tab.workspace.workspaceTabId}
        role="listitem"
        animate:flip={{ duration: 170 }}
      >
        <button
          class="workspace-tab-main electrobun-webkit-app-region-no-drag"
          type="button"
          aria-current={tab.workspace.workspaceTabId === activeWorkspaceTabId ? "page" : undefined}
          aria-label={workspaceTabLabel(tab)}
          onpointerdown={(event) => handlePointerDown(event, tab.workspace.workspaceTabId)}
          onpointermove={handlePointerMove}
          onpointerup={(event) => finishPointerDrag(event, true)}
          onpointercancel={(event) => finishPointerDrag(event, false)}
          onclick={() => handleSelectWorkspace(tab.workspace.workspaceTabId)}
        >
          <span class="workspace-tab-label">{tab.workspace.workspaceLabel}</span>
          <span class="workspace-tab-counts" aria-label="Workspace status counts">
            {#each getVisibleWorkspaceTabCounts(tab.counts) as count}
              <Tooltip label={`${count.value} ${count.label}`}>
                <span class={`workspace-tab-count kind-${count.kind}`.trim()}>
                  {count.value}
                </span>
              </Tooltip>
            {/each}
          </span>
        </button>
        <Tooltip class="electrobun-webkit-app-region-no-drag" label={`Close ${tab.workspace.workspaceLabel}`}>
          <button
            class="workspace-tab-close electrobun-webkit-app-region-no-drag"
            type="button"
            aria-label={`Close ${tab.workspace.workspaceLabel}`}
            onclick={() => onCloseWorkspace(tab.workspace.workspaceTabId)}
          >
            <XIcon size={12} strokeWidth={2} aria-hidden="true" />
          </button>
        </Tooltip>
      </div>
    {/each}
  </div>
  <Tooltip class="electrobun-webkit-app-region-no-drag" label="New tab">
    <button
      class="workspace-tab-add electrobun-webkit-app-region-no-drag"
      type="button"
      aria-label="New tab"
      disabled={openingWorkspace}
      onclick={onNewWorkspaceTab}
    >
      <PlusIcon size={14} strokeWidth={2} aria-hidden="true" />
    </button>
  </Tooltip>
</div>

<style>
  .workspace-tabs-strip {
    display: flex;
    align-items: center;
    gap: 0.24rem;
    min-width: 0;
    width: 100%;
    max-width: 100%;
    pointer-events: auto;
    user-select: none;
  }

  .workspace-tabs-scroll {
    display: flex;
    align-items: center;
    gap: 0.24rem;
    min-width: 0;
    overflow-x: auto;
    overflow-y: hidden;
    scrollbar-width: none;
  }

  .workspace-tabs-scroll::-webkit-scrollbar {
    display: none;
  }

  .workspace-tab {
    position: relative;
    display: inline-grid;
    grid-template-columns: minmax(0, 1fr) 1.18rem;
    align-items: center;
    flex: 0 0 auto;
    width: max-content;
    min-width: 6.15rem;
    max-width: min(16rem, 32vw);
    height: 1.68rem;
    overflow: hidden;
    border-radius: var(--ui-radius-md);
    background: color-mix(in oklab, var(--ui-hover-bg) 42%, transparent);
    box-shadow: none;
    color: var(--ui-text-secondary);
    -webkit-user-drag: none;
    transition:
      opacity 150ms cubic-bezier(0.19, 1, 0.22, 1),
      transform 170ms cubic-bezier(0.19, 1, 0.22, 1),
      background-color 150ms cubic-bezier(0.19, 1, 0.22, 1),
      color 150ms cubic-bezier(0.19, 1, 0.22, 1);
  }

  .workspace-tab.dragging {
    opacity: 0.58;
  }

  .workspace-tab.active {
    background: var(--ui-selected-bg);
    color: var(--ui-text-primary);
  }

  .workspace-tab:hover:not(.active),
  .workspace-tab:focus-within:not(.active) {
    background: var(--ui-hover-bg);
  }

  .workspace-tab-main,
  .workspace-tab-close,
  .workspace-tab-add {
    appearance: none;
    border: 0;
    background: transparent;
    box-shadow: none;
    color: inherit;
    font: inherit;
    cursor: pointer;
  }

  .workspace-tab-main {
    display: inline-flex;
    align-items: center;
    gap: 0.42rem;
    min-width: 0;
    height: 100%;
    padding: 0 0.34rem 0 0.56rem;
    text-align: left;
    cursor: grab;
    touch-action: pan-y;
  }

  .workspace-tab.dragging .workspace-tab-main {
    cursor: grabbing;
  }

  .workspace-tab-main:focus-visible,
  .workspace-tab-close:focus-visible,
  .workspace-tab-add:focus-visible {
    outline: none;
    box-shadow: var(--ui-focus-ring);
  }

  .workspace-tab-label {
    min-width: 0;
    max-width: 8.5rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--text-sm);
    font-weight: var(--vscode-font-weight);
    line-height: 1;
  }

  .workspace-tab-counts {
    display: inline-flex;
    align-items: center;
    gap: 0.18rem;
    flex: 0 0 auto;
    color: var(--ui-text-tertiary);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    font-variant-numeric: tabular-nums;
  }

  .workspace-tab-count {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 0.82rem;
    height: 0.94rem;
    padding: 0 0.18rem;
    border-radius: var(--ui-radius-sm);
  }

  .workspace-tab-count.kind-running {
    color: var(--ui-accent);
    background: color-mix(in oklab, var(--ui-accent) 14%, var(--ui-bg));
  }

  .workspace-tab-count.kind-unread,
  .workspace-tab-count.kind-waiting {
    color: var(--ui-status-waiting);
    background: var(--ui-status-waiting-soft);
  }

  .workspace-tab-count.kind-error {
    color: var(--ui-danger);
    background: var(--ui-danger-soft);
  }

  .workspace-tab-close {
    display: inline-grid;
    place-items: center;
    width: 1.18rem;
    height: 100%;
    color: var(--ui-text-tertiary);
    opacity: 0;
    pointer-events: none;
    transition:
      background-color 150ms cubic-bezier(0.19, 1, 0.22, 1),
      color 150ms cubic-bezier(0.19, 1, 0.22, 1),
      opacity 150ms cubic-bezier(0.19, 1, 0.22, 1);
  }

  .workspace-tab:hover .workspace-tab-close,
  .workspace-tab:focus-within .workspace-tab-close,
  .workspace-tab.active .workspace-tab-close {
    opacity: 1;
    pointer-events: auto;
  }

  .workspace-tab-close:hover,
  .workspace-tab-close:focus-visible {
    background: var(--ui-hover-bg);
    color: var(--ui-text-primary);
  }

  .workspace-tab-add {
    display: inline-grid;
    place-items: center;
    width: 1.68rem;
    height: 1.68rem;
    flex: 0 0 auto;
    border-radius: var(--ui-radius-md);
    background: color-mix(in oklab, var(--ui-hover-bg) 34%, transparent);
    box-shadow: none;
    color: var(--ui-text-tertiary);
    flex: 0 0 auto;
    transition:
      background-color 150ms cubic-bezier(0.19, 1, 0.22, 1),
      color 150ms cubic-bezier(0.19, 1, 0.22, 1),
      opacity 150ms cubic-bezier(0.19, 1, 0.22, 1);
  }

  .workspace-tab-add:hover,
  .workspace-tab-add:focus-visible {
    background: var(--ui-hover-bg);
    color: var(--ui-text-primary);
  }

  .workspace-tab-add:disabled {
    cursor: default;
    opacity: 0.58;
  }
</style>
