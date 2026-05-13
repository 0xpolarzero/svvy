<script lang="ts">
  import ArchiveIcon from "@lucide/svelte/icons/archive";
  import ArchiveRestoreIcon from "@lucide/svelte/icons/archive-restore";
  import PinIcon from "@lucide/svelte/icons/pin";
  import PinOffIcon from "@lucide/svelte/icons/pin-off";
  import type { WorkspaceSessionSummary } from "../shared/workspace-contract";
  import { formatRelativeSessionTime, formatSessionStatusLabel } from "./session-format";
  import Button from "./ui/Button.svelte";
  import Tooltip from "./ui/Tooltip.svelte";

  type Props = {
    session: WorkspaceSessionSummary;
    active: boolean;
    activeSurface?: "orchestrator" | "thread";
    paneLocations?: { paneId: string; label: string; focused: boolean }[];
    disabled?: boolean;
    onOpen: () => void;
    onRename: () => void;
    onPin: () => void;
    onUnpin: () => void;
    onArchive: () => void;
    onUnarchive: () => void;
    onArrowUp?: () => void;
    onArrowDown?: () => void;
  };

  let {
    session,
    active,
    activeSurface,
    paneLocations = [],
    disabled = false,
    onOpen,
    onRename,
    onPin,
    onUnpin,
    onArchive,
    onUnarchive,
    onArrowUp,
    onArrowDown,
  }: Props = $props();

  function handleKeydown(event: KeyboardEvent) {
    if (event.key === "ArrowUp") {
      event.preventDefault();
      onArrowUp?.();
      return;
    }

    if (event.key === "ArrowDown") {
      event.preventDefault();
      onArrowDown?.();
    }
  }

  function getProgressLabels(currentSession: WorkspaceSessionSummary): string[] {
    const labels: string[] = [];
    const counts = currentSession.counts;
    const threadIdsByStatus = currentSession.threadIdsByStatus;

    if (counts) {
      if (counts.workflows > 0) {
        labels.push(`Workflow ${counts.workflows}`);
      }

      if (counts.ciRuns > 0) {
        labels.push(`CI ${counts.ciRuns}`);
      }

      if (counts.threads > 0) {
        labels.push(`Threads ${counts.threads}`);
      }
    }

    if (threadIdsByStatus?.runningHandler.length) {
      labels.push(`Handlers ${threadIdsByStatus.runningHandler.length}`);
    }

    if (threadIdsByStatus?.runningWorkflow.length) {
      labels.push(`Workflows ${threadIdsByStatus.runningWorkflow.length}`);
    }

    if (threadIdsByStatus?.waiting.length) {
      labels.push(
        currentSession.status === "running"
          ? `Blocked ${threadIdsByStatus.waiting.length}`
          : `Waiting ${threadIdsByStatus.waiting.length}`,
      );
    }

    if (threadIdsByStatus?.troubleshooting.length) {
      labels.push(`Troubleshooting ${threadIdsByStatus.troubleshooting.length}`);
    }

    return labels;
  }

  const showingThreadSurface = $derived(active && activeSurface === "thread");
  const renameLocked = $derived(session.titleGeneration?.renameLocked ?? false);
  const focusedPaneLocation = $derived(paneLocations.find((location) => location.focused) ?? null);
  const openPaneSummary = $derived(
    paneLocations.length === 0
      ? ""
      : paneLocations.length === 1
        ? (focusedPaneLocation?.label ?? paneLocations[0]?.label ?? "")
        : `${paneLocations.length} panes`,
  );
</script>

<article
  class={`session-item ${active ? "active" : ""} ${showingThreadSurface ? "active-thread" : ""} ${session.isArchived ? "archived" : ""} ${paneLocations.length > 0 ? "open-in-pane" : ""}`.trim()}
>
  <button
    class="session-main"
    type="button"
    aria-current={active ? "true" : undefined}
    disabled={disabled}
    onclick={onOpen}
    ondblclick={onRename}
    onkeydown={handleKeydown}
    title={renameLocked ? session.title : `${session.title} · double-click to rename`}
  >
    <div class="session-main-top">
      <strong>{session.title}</strong>
      <div class="session-main-top-meta">
        {#if openPaneSummary}
          <span
            class="session-open-marker"
            title={`Open in ${paneLocations.map((location) => location.label).join(", ")}`}
          >
            {openPaneSummary}
          </span>
        {/if}
        <span>{formatRelativeSessionTime(session.updatedAt)}</span>
      </div>
    </div>
    <div class="session-main-body">
      <div class="session-main-preview">{session.preview}</div>
      {#if getProgressLabels(session).length > 0}
        <div class="session-main-progress" aria-label="Structured workflow progress">
          {#each getProgressLabels(session) as label}
            <span class="session-progress-pill">{label}</span>
          {/each}
        </div>
      {/if}
    </div>

    {#if showingThreadSurface || session.status !== "idle" || session.parentSessionId}
      <div class="session-main-meta">
        {#if showingThreadSurface}
          <span class="session-surface">Thread Open</span>
        {/if}
        {#if session.status !== "idle"}
          <span class={`session-status status-${session.status}`.trim()}>
            <span class="session-status-dot"></span>
            {formatSessionStatusLabel(session)}
          </span>
        {/if}
        {#if session.parentSessionId}
          <span class="session-branch">Fork</span>
        {/if}
      </div>
    {/if}

    {#if session.isPinned || session.isArchived}
      <div class="session-main-meta session-navigation-meta">
        {#if session.isPinned}
          <span class="session-branch">Pinned</span>
        {/if}
        {#if session.isArchived}
          <span class="session-branch">Archived</span>
        {/if}
      </div>
    {/if}
  </button>

  <div class="session-inline-actions" role="toolbar" aria-label={`Actions for ${session.title}`}>
    <Tooltip label={session.isArchived ? "Unarchive session" : "Archive session"}>
      <Button
        variant="ghost"
        size="xs"
        iconOnly
        class="session-inline-action"
        aria-label={session.isArchived ? `Unarchive ${session.title}` : `Archive ${session.title}`}
        onclick={(event) => {
          event.stopPropagation();
          if (session.isArchived) {
            onUnarchive();
          } else {
            onArchive();
          }
        }}
      >
        {#if session.isArchived}
          <ArchiveRestoreIcon aria-hidden="true" size={13} strokeWidth={1.9} />
        {:else}
          <ArchiveIcon aria-hidden="true" size={13} strokeWidth={1.9} />
        {/if}
      </Button>
    </Tooltip>

    <Tooltip label={session.isPinned ? "Unpin session" : "Pin session"}>
      <Button
        variant="ghost"
        size="xs"
        iconOnly
        class="session-inline-action"
        aria-label={session.isPinned ? `Unpin ${session.title}` : `Pin ${session.title}`}
        onclick={(event) => {
          event.stopPropagation();
          if (session.isPinned) {
            onUnpin();
          } else {
            onPin();
          }
        }}
      >
        {#if session.isPinned}
          <PinOffIcon aria-hidden="true" size={13} strokeWidth={1.9} />
        {:else}
          <PinIcon aria-hidden="true" size={13} strokeWidth={1.9} />
        {/if}
      </Button>
    </Tooltip>
  </div>
</article>

<style>
  .session-item {
    position: relative;
    display: block;
    width: 100%;
    min-width: 0;
    max-width: 100%;
    border-radius: var(--ui-radius-md);
  }

  .session-main {
    position: relative;
    box-sizing: border-box;
    width: 100%;
    min-width: 0;
    padding: 0.42rem 0.58rem 0.44rem 0.9rem;
    border-radius: var(--ui-radius-md);
    border: 1px solid transparent;
    background: transparent;
    color: inherit;
    text-align: left;
    cursor: pointer;
    transition:
      border-color 160ms cubic-bezier(0.19, 1, 0.22, 1),
      background-color 160ms cubic-bezier(0.19, 1, 0.22, 1),
      box-shadow 160ms cubic-bezier(0.19, 1, 0.22, 1),
      color 160ms cubic-bezier(0.19, 1, 0.22, 1);
  }

  .session-main::before {
    content: "";
    position: absolute;
    top: 0.42rem;
    bottom: 0.42rem;
    left: 0.36rem;
    width: 0.16rem;
    border-radius: 999px;
    background: transparent;
    transition: background-color 160ms cubic-bezier(0.19, 1, 0.22, 1);
  }

  .session-main:hover:not(:disabled) {
    background: color-mix(in oklab, var(--ui-surface-subtle) 82%, transparent);
  }

  .session-main:focus-visible {
    outline: none;
    box-shadow: var(--ui-focus-ring);
  }

  .session-main:disabled {
    opacity: 0.62;
    cursor: not-allowed;
  }

  .active .session-main {
    border-color: transparent;
    background: color-mix(in oklab, var(--ui-surface-subtle) 82%, transparent);
  }

  .active .session-main::before {
    background: color-mix(in oklab, var(--ui-accent) 84%, transparent);
  }

  .active-thread .session-main {
    border-color: color-mix(in oklab, var(--ui-accent) 22%, transparent);
  }

  .open-in-pane:not(.active) .session-main {
    border-color: color-mix(in oklab, var(--ui-border-soft) 62%, transparent);
    background: color-mix(in oklab, var(--ui-surface-subtle) 34%, transparent);
  }

  .open-in-pane:not(.active) .session-main::before {
    background: color-mix(in oklab, var(--ui-text-tertiary) 42%, transparent);
  }

  .archived .session-main {
    opacity: 0.74;
  }

  .session-main-top {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 0.6rem;
  }

  .session-main-top strong {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 0.68rem;
    font-weight: 600;
    letter-spacing: 0;
  }

  .session-main-top-meta {
    display: inline-flex;
    align-items: center;
    justify-content: flex-end;
    gap: 0.28rem;
    flex-shrink: 0;
    min-width: 0;
    font-family: var(--font-mono);
    font-size: 0.54rem;
    color: var(--ui-text-tertiary);
    transition:
      opacity 120ms cubic-bezier(0.19, 1, 0.22, 1),
      visibility 120ms cubic-bezier(0.19, 1, 0.22, 1);
  }

  .session-item:hover .session-main-top-meta,
  .session-item:focus-within .session-main-top-meta {
    opacity: 0;
    visibility: hidden;
  }

  .session-main-top-meta span {
    flex-shrink: 0;
  }

  .session-open-marker {
    max-width: 4.4rem;
    overflow: hidden;
    color: color-mix(in oklab, var(--ui-accent) 66%, var(--ui-text-tertiary));
    font-family: var(--font-mono);
    font-size: 0.5rem;
    font-weight: 650;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .session-main-body {
    display: grid;
    gap: 0.16rem;
    margin-top: 0.18rem;
    min-width: 0;
  }

  .session-main-preview {
    min-width: 0;
    overflow: hidden;
    display: -webkit-box;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 1;
    font-size: 0.64rem;
    line-height: 1.22;
    color: var(--ui-text-secondary);
  }

  .session-main-progress {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.22rem;
  }

  .session-progress-pill {
    display: inline-flex;
    align-items: center;
    min-height: 0.9rem;
    padding: 0 0.24rem;
    border: 1px solid var(--ui-border-soft);
    border-radius: var(--ui-radius-sm);
    background: transparent;
    color: var(--ui-text-tertiary);
    font-family: var(--font-mono);
    font-size: 0.52rem;
    font-weight: 500;
    letter-spacing: 0;
    white-space: nowrap;
  }

  .session-main-meta {
    display: flex;
    align-items: center;
    gap: 0.32rem;
    flex-wrap: nowrap;
    margin-top: 0.22rem;
    overflow: hidden;
  }

  .session-navigation-meta {
    margin-top: 0.24rem;
  }

  .session-status,
  .session-branch,
  .session-surface {
    display: inline-flex;
    align-items: center;
    gap: 0.26rem;
    min-height: 0.9rem;
    font-family: var(--font-mono);
    font-size: 0.54rem;
    font-weight: 500;
    letter-spacing: 0;
    line-height: 1;
    white-space: nowrap;
    color: var(--ui-text-tertiary);
  }

  .session-status-dot {
    width: 0.34rem;
    height: 0.34rem;
    border-radius: 999px;
    background: currentColor;
  }

  .status-running {
    color: color-mix(in oklab, var(--ui-warning) 82%, var(--ui-text-primary));
  }

  .status-waiting {
    color: color-mix(in oklab, var(--ui-info) 78%, var(--ui-text-primary));
  }

  .status-error {
    color: color-mix(in oklab, var(--ui-danger) 82%, var(--ui-text-primary));
  }

  .session-branch {
    color: var(--ui-text-tertiary);
  }

  .session-surface {
    max-width: 7rem;
    overflow: hidden;
    color: var(--ui-text-tertiary);
    text-overflow: ellipsis;
  }

  .session-inline-actions {
    position: absolute;
    top: 0.32rem;
    right: 0.32rem;
    z-index: 2;
    display: inline-flex;
    align-items: center;
    gap: 0.08rem;
    padding: 0.08rem;
    border: 1px solid color-mix(in oklab, var(--ui-border-soft) 70%, transparent);
    border-radius: var(--ui-radius-md);
    background: color-mix(in oklab, var(--ui-surface-raised) 72%, transparent);
    box-shadow: 0 0 0 1px color-mix(in oklab, var(--ui-surface) 58%, transparent);
    backdrop-filter: blur(10px);
    opacity: 0;
    pointer-events: none;
    transform: translateY(-0.08rem);
    transition:
      opacity 140ms cubic-bezier(0.19, 1, 0.22, 1),
      transform 140ms cubic-bezier(0.19, 1, 0.22, 1);
  }

  .session-item:hover .session-inline-actions,
  .session-item:focus-within .session-inline-actions {
    opacity: 1;
    pointer-events: auto;
    transform: translateY(0);
  }

  .session-inline-action {
    min-width: 1.3rem;
    min-height: 1.3rem;
    color: var(--ui-text-tertiary);
    border-radius: var(--ui-radius-sm);
  }

  .session-inline-action:hover,
  .session-inline-action:focus-visible {
    background: color-mix(in oklab, var(--ui-surface-subtle) 88%, transparent);
    color: var(--ui-text-primary);
  }

  @media (max-width: 760px) {
    .session-inline-actions {
      top: 0.24rem;
      right: 0.24rem;
      opacity: 1;
      pointer-events: auto;
      transform: none;
    }

    .session-inline-action {
      min-width: 2.75rem;
      min-height: 2.75rem;
    }
  }
</style>
