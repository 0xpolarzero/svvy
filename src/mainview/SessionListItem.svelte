<script lang="ts">
  import EllipsisVerticalIcon from "@lucide/svelte/icons/ellipsis-vertical";
  import { onMount } from "svelte";
  import type { WorkspaceSessionSummary } from "../shared/workspace-contract";
  import { formatRelativeSessionTime, formatSessionStatusLabel } from "./session-format";
  import Button from "./ui/Button.svelte";

  const SESSION_MENU_OPEN_EVENT = "svvy-session-menu-open";

  type Props = {
    session: WorkspaceSessionSummary;
    active: boolean;
    activeSurface?: "orchestrator" | "thread";
    paneLocations?: { paneId: string; label: string; focused: boolean }[];
    disabled?: boolean;
    onOpen: () => void;
    onRename: () => void;
    onFork: () => void;
    onDelete: () => void;
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
    onFork,
    onDelete,
    onPin,
    onUnpin,
    onArchive,
    onUnarchive,
    onArrowUp,
    onArrowDown,
  }: Props = $props();

  let menuOpen = $state(false);
  let menuRoot = $state<HTMLDivElement | null>(null);

  onMount(() => {
    function closeWhenAnotherMenuOpens(event: Event) {
      const nextSessionId = event instanceof CustomEvent ? event.detail?.sessionId : undefined;
      if (nextSessionId !== session.id) {
        menuOpen = false;
      }
    }

    window.addEventListener(SESSION_MENU_OPEN_EVENT, closeWhenAnotherMenuOpens);
    return () => window.removeEventListener(SESSION_MENU_OPEN_EVENT, closeWhenAnotherMenuOpens);
  });

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

  function closeIfFocusLeaves(nextTarget: EventTarget | null) {
    if (!(nextTarget instanceof Node) || !menuRoot?.contains(nextTarget)) {
      menuOpen = false;
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
  class={`session-item ${active ? "active" : ""} ${showingThreadSurface ? "active-thread" : ""} ${session.isArchived ? "archived" : ""} ${paneLocations.length > 0 ? "open-in-pane" : ""} ${menuOpen ? "menu-open" : ""}`.trim()}
>
  <button
    class="session-main"
    type="button"
    aria-current={active ? "true" : undefined}
    disabled={disabled}
    onclick={onOpen}
    onkeydown={handleKeydown}
    title={session.title}
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

  <div
    bind:this={menuRoot}
    class="session-menu-wrap"
    onfocusout={(event) => closeIfFocusLeaves(event.relatedTarget)}
  >
    <Button
      variant="ghost"
      size="sm"
      class="session-menu-trigger"
      aria-label={`Session actions for ${session.title}`}
      onclick={(event) => {
        event.stopPropagation();
        if (!menuOpen) {
          window.dispatchEvent(new CustomEvent(SESSION_MENU_OPEN_EVENT, { detail: { sessionId: session.id } }));
        }
        menuOpen = !menuOpen;
      }}
    >
      <EllipsisVerticalIcon aria-hidden="true" size={15} strokeWidth={1.9} />
    </Button>

    {#if menuOpen}
      <div class="session-menu" role="menu" aria-label={`Actions for ${session.title}`}>
        {#if session.isArchived}
          <button role="menuitem" type="button" onclick={() => { menuOpen = false; onUnarchive(); }}>Unarchive</button>
        {:else}
          {#if session.isPinned}
            <button role="menuitem" type="button" onclick={() => { menuOpen = false; onUnpin(); }}>Unpin</button>
          {:else}
            <button role="menuitem" type="button" onclick={() => { menuOpen = false; onPin(); }}>Pin</button>
          {/if}
          <button role="menuitem" type="button" onclick={() => { menuOpen = false; onArchive(); }}>Archive</button>
        {/if}
        <button
          role="menuitem"
          type="button"
          disabled={renameLocked}
          title={renameLocked ? "Title generation is running" : "Rename"}
          onclick={() => { menuOpen = false; onRename(); }}
        >
          Rename
        </button>
        <button role="menuitem" type="button" onclick={() => { menuOpen = false; onFork(); }}>Fork</button>
        <button class="danger" role="menuitem" type="button" onclick={() => { menuOpen = false; onDelete(); }}>Delete</button>
      </div>
    {/if}
  </div>
</article>

<style>
  .session-item {
    position: relative;
    display: block;
    border-radius: var(--ui-radius-md);
  }

  .session-main {
    position: relative;
    width: 100%;
    min-width: 0;
    padding: 0.42rem 2rem 0.44rem 0.9rem;
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

  .session-menu-wrap {
    position: absolute;
    top: 0.32rem;
    right: 0.32rem;
    z-index: 2;
    opacity: 0;
    pointer-events: none;
    transition: opacity 140ms cubic-bezier(0.19, 1, 0.22, 1);
  }

  .session-item:hover .session-menu-wrap,
  .session-item:focus-within .session-menu-wrap,
  .session-item.menu-open .session-menu-wrap {
    opacity: 1;
    pointer-events: auto;
  }

  .session-menu-trigger {
    min-width: 1.42rem;
    min-height: 1.42rem;
    padding-inline: 0.16rem;
    border-radius: var(--ui-radius-sm);
    background: color-mix(in oklab, var(--ui-surface-raised) 64%, transparent);
    color: var(--ui-text-tertiary);
    box-shadow: 0 0 0 1px color-mix(in oklab, var(--ui-border-soft) 72%, transparent);
    backdrop-filter: blur(10px);
  }

  .session-menu-trigger:hover,
  .session-menu-trigger:focus-visible {
    background: var(--ui-surface-raised);
    color: var(--ui-text-primary);
  }

  .session-menu {
    position: absolute;
    top: calc(100% + 0.24rem);
    right: 0;
    z-index: var(--ui-z-overlay);
    display: grid;
    gap: 0.08rem;
    min-width: 9.2rem;
    padding: 0.28rem;
    border-radius: var(--ui-radius-md);
    border: 1px solid color-mix(in oklab, var(--ui-border-soft) 86%, transparent);
    background:
      linear-gradient(
        180deg,
        color-mix(in oklab, var(--ui-surface-raised) 96%, white 4%),
        var(--ui-surface-raised)
      );
    box-shadow:
      0 18px 42px color-mix(in oklab, black 18%, transparent),
      0 4px 14px color-mix(in oklab, black 12%, transparent);
    transform-origin: top right;
    animation: session-menu-enter 150ms cubic-bezier(0.19, 1, 0.22, 1);
  }

  .session-menu button {
    display: flex;
    align-items: center;
    min-height: 1.72rem;
    padding: 0.34rem 0.48rem;
    border: 1px solid transparent;
    border-radius: var(--ui-radius-sm);
    background: transparent;
    color: var(--ui-text-secondary);
    font-size: 0.62rem;
    font-weight: 560;
    line-height: 1.1;
    text-align: left;
    cursor: pointer;
    transition:
      border-color 120ms cubic-bezier(0.19, 1, 0.22, 1),
      background-color 120ms cubic-bezier(0.19, 1, 0.22, 1),
      color 120ms cubic-bezier(0.19, 1, 0.22, 1);
  }

  .session-menu button:hover:not(:disabled),
  .session-menu button:focus-visible {
    border-color: color-mix(in oklab, var(--ui-border-soft) 72%, transparent);
    background: color-mix(in oklab, var(--ui-surface-subtle) 78%, transparent);
    color: var(--ui-text-primary);
    outline: none;
  }

  .session-menu button:disabled {
    color: color-mix(in oklab, var(--ui-text-tertiary) 58%, transparent);
    cursor: not-allowed;
  }

  .session-menu .danger {
    color: color-mix(in oklab, var(--ui-danger) 86%, var(--ui-text-primary));
  }

  .session-menu .danger:hover,
  .session-menu .danger:focus-visible {
    border-color: color-mix(in oklab, var(--ui-danger) 24%, transparent);
    background: color-mix(in oklab, var(--ui-danger) 10%, transparent);
    color: color-mix(in oklab, var(--ui-danger) 92%, var(--ui-text-primary));
  }

  @keyframes session-menu-enter {
    from {
      opacity: 0;
      transform: translateY(-0.18rem) scale(0.985);
    }

    to {
      opacity: 1;
      transform: translateY(0) scale(1);
    }
  }

  @media (max-width: 760px) {
    .session-menu-wrap {
      top: 0.24rem;
      right: 0.24rem;
      opacity: 1;
      pointer-events: auto;
    }

    .session-menu-trigger {
      min-width: 2.75rem;
      min-height: 2.75rem;
    }

    .session-menu button {
      min-height: 2.75rem;
    }
  }
</style>
