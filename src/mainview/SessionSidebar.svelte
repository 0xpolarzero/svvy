<script lang="ts">
  import PlusIcon from "@lucide/svelte/icons/plus";
  import GitBranchIcon from "@lucide/svelte/icons/git-branch";
  import ChevronDownIcon from "@lucide/svelte/icons/chevron-down";
  import ChevronRightIcon from "@lucide/svelte/icons/chevron-right";
  import FolderGit2Icon from "@lucide/svelte/icons/folder-git-2";
  import SettingsIcon from "@lucide/svelte/icons/settings";
  import WorkflowIcon from "@lucide/svelte/icons/workflow";
  import { getKeybindingDisplayShortcut, getKeybindingShortcut } from "../shared/keybindings";
  import type { WorkspaceSessionNavigationReadModel, WorkspaceSessionSummary } from "../shared/workspace-contract";
  import SessionListItem from "./SessionListItem.svelte";
  import Button from "./ui/Button.svelte";

  type Props = {
    workspaceLabel: string;
    branch?: string;
    navigation: WorkspaceSessionNavigationReadModel;
    activeSessionId?: string;
    activeSurface?: "orchestrator" | "thread";
    paneLocationsBySessionId?: Record<string, { paneId: string; label: string; focused: boolean }[]>;
    busy?: boolean;
    errorMessage?: string;
    onCreateSession: () => void;
    onCreateDumbSession: () => void;
    onOpenSession: (sessionId: string) => void;
    onRenameSession: (session: WorkspaceSessionSummary) => void;
    onForkSession: (session: WorkspaceSessionSummary) => void;
    onDeleteSession: (session: WorkspaceSessionSummary) => void;
    onPinSession: (session: WorkspaceSessionSummary) => void;
    onUnpinSession: (session: WorkspaceSessionSummary) => void;
    onArchiveSession: (session: WorkspaceSessionSummary) => void;
    onUnarchiveSession: (session: WorkspaceSessionSummary) => void;
    onToggleArchivedGroup: (collapsed: boolean) => void;
    onOpenWorkflowLibrary?: () => void;
    onOpenSettings?: () => void;
  };

  let {
    workspaceLabel,
    branch,
    navigation,
    activeSessionId,
    activeSurface,
    paneLocationsBySessionId = {},
    busy = false,
    errorMessage,
    onCreateSession,
    onCreateDumbSession,
    onOpenSession,
    onRenameSession,
    onForkSession,
    onDeleteSession,
    onPinSession,
    onUnpinSession,
    onArchiveSession,
    onUnarchiveSession,
    onToggleArchivedGroup,
    onOpenWorkflowLibrary,
    onOpenSettings,
  }: Props = $props();

  let showNewSessionMenu = $state(false);
  const newSessionShortcut = getKeybindingShortcut("session.new");
  const dumbSessionShortcut = getKeybindingShortcut("session.dumb");
  const newSessionDisplayShortcut = getKeybindingDisplayShortcut("session.new");
  const dumbSessionDisplayShortcut = getKeybindingDisplayShortcut("session.dumb");

  const sessionCount = $derived(
    navigation.pinnedSessions.length +
      navigation.activeSessions.length +
      navigation.archived.sessions.length,
  );

  function handleNewSessionMenuFocusOut(event: FocusEvent) {
    const current = event.currentTarget as HTMLElement | null;
    const next = event.relatedTarget as Node | null;
    if (current && next && current.contains(next)) {
      return;
    }
    showNewSessionMenu = false;
  }

  function handleWindowKeydown(event: KeyboardEvent) {
    if (event.key === "Escape") {
      showNewSessionMenu = false;
    }
  }
</script>

<svelte:window onkeydown={handleWindowKeydown} />

<div class="session-sidebar">
  <header class="sidebar-header electrobun-webkit-app-region-drag">
    <div class="sidebar-header-copy">
      <div class="sidebar-brand-row">
        <span class="sidebar-brand">svvy</span>
      </div>
      <h2 class="sidebar-workspace-heading" title={workspaceLabel}>{workspaceLabel}</h2>
      <p class="sidebar-context">
        {#if branch}
          <span class="branch-pill"><GitBranchIcon size={9} aria-hidden="true" /> {branch.split("/").at(-1) ?? branch}</span>
        {/if}
        <span>{sessionCount} sessions</span>
      </p>
    </div>
  </header>

  <div class="sidebar-actions" aria-label="Session actions">
    <div
      class="new-session-menu-shell"
      class:menu-open={showNewSessionMenu}
      role="group"
      aria-label="New session options"
      onmouseenter={() => (showNewSessionMenu = true)}
      onmouseleave={() => (showNewSessionMenu = false)}
      onfocusin={() => (showNewSessionMenu = true)}
      onfocusout={handleNewSessionMenuFocusOut}
    >
      <Button
        variant="primary"
        size="sm"
        class="new-session"
        onclick={() => {
          showNewSessionMenu = false;
          onCreateSession();
        }}
        disabled={busy}
        aria-label="Create a new session"
        title={`New Session (${newSessionShortcut})`}
        aria-haspopup="menu"
        aria-expanded={showNewSessionMenu}
      >
        <PlusIcon aria-hidden="true" size={13} strokeWidth={2} />
        <span>New session</span>
        <kbd>{newSessionDisplayShortcut}</kbd>
        <ChevronDownIcon class="new-session-chevron" aria-hidden="true" size={12} strokeWidth={1.9} />
      </Button>
      {#if showNewSessionMenu}
        <div class="new-session-menu" role="menu" aria-label="New session variants">
          <button
            type="button"
            class="new-session-menu-item"
            role="menuitem"
            disabled={busy}
            onclick={() => {
              showNewSessionMenu = false;
              onCreateDumbSession();
            }}
          >
            <span>New dumb session</span>
            <kbd>{dumbSessionDisplayShortcut}</kbd>
          </button>
        </div>
      {/if}
    </div>
  </div>

  {#if errorMessage}
    <p class="sidebar-error">{errorMessage}</p>
  {/if}

  <div class="sidebar-sections">
    <div class="sidebar-list">
      {#if navigation.pinnedSessions.length > 0}
        <section class="sidebar-section" aria-label="Pinned sessions">
          <p class="sidebar-section-label">Pinned</p>
          {#each navigation.pinnedSessions as session (session.id)}
            <SessionListItem
              active={session.id === activeSessionId}
              activeSurface={session.id === activeSessionId ? activeSurface : undefined}
              disabled={busy && session.id !== activeSessionId}
              paneLocations={paneLocationsBySessionId[session.id] ?? []}
              {session}
              onOpen={() => onOpenSession(session.id)}
              onRename={() => onRenameSession(session)}
              onFork={() => onForkSession(session)}
              onDelete={() => onDeleteSession(session)}
              onPin={() => onPinSession(session)}
              onUnpin={() => onUnpinSession(session)}
              onArchive={() => onArchiveSession(session)}
              onUnarchive={() => onUnarchiveSession(session)}
            />
          {/each}
        </section>
      {/if}

      {#if navigation.activeSessions.length > 0}
        <section class="sidebar-section" aria-label="Active sessions">
          <p class="sidebar-section-label">Active</p>
          {#each navigation.activeSessions as session (session.id)}
            <SessionListItem
              active={session.id === activeSessionId}
              activeSurface={session.id === activeSessionId ? activeSurface : undefined}
              disabled={busy && session.id !== activeSessionId}
              paneLocations={paneLocationsBySessionId[session.id] ?? []}
              {session}
              onOpen={() => onOpenSession(session.id)}
              onRename={() => onRenameSession(session)}
              onFork={() => onForkSession(session)}
              onDelete={() => onDeleteSession(session)}
              onPin={() => onPinSession(session)}
              onUnpin={() => onUnpinSession(session)}
              onArchive={() => onArchiveSession(session)}
              onUnarchive={() => onUnarchiveSession(session)}
            />
          {/each}
        </section>
      {/if}

      {#if navigation.archived.sessions.length > 0}
        <section class="sidebar-section archived-section" aria-label="Archived sessions">
          <button
            class="archived-toggle"
            type="button"
            aria-expanded={!navigation.archived.collapsed}
            onclick={() => onToggleArchivedGroup(!navigation.archived.collapsed)}
          >
            {#if navigation.archived.collapsed}
              <ChevronRightIcon aria-hidden="true" size={14} strokeWidth={1.9} />
            {:else}
              <ChevronDownIcon aria-hidden="true" size={14} strokeWidth={1.9} />
            {/if}
            <span>Archived</span>
            <span>{navigation.archived.sessions.length}</span>
          </button>

          {#if !navigation.archived.collapsed}
            {#each navigation.archived.sessions as session (session.id)}
              <SessionListItem
                active={session.id === activeSessionId}
                activeSurface={session.id === activeSessionId ? activeSurface : undefined}
                disabled={busy && session.id !== activeSessionId}
                paneLocations={paneLocationsBySessionId[session.id] ?? []}
                {session}
                onOpen={() => onOpenSession(session.id)}
                onRename={() => onRenameSession(session)}
                onFork={() => onForkSession(session)}
                onDelete={() => onDeleteSession(session)}
                onPin={() => onPinSession(session)}
                onUnpin={() => onUnpinSession(session)}
                onArchive={() => onArchiveSession(session)}
                onUnarchive={() => onUnarchiveSession(session)}
              />
            {/each}
          {/if}
        </section>
      {/if}

      {#if onOpenWorkflowLibrary}
        <section class="sidebar-section reference-nav-section" aria-label="Workflow library">
          <button class="reference-nav-row" type="button" onclick={onOpenWorkflowLibrary}>
            <WorkflowIcon size={13} aria-hidden="true" />
            <span>Saved workflows</span>
            <small>.svvy</small>
          </button>
        </section>
      {/if}

    </div>
  </div>

  <footer class="sidebar-footer">
    <div class="workspace-path" title={workspaceLabel}>
      <FolderGit2Icon size={12} aria-hidden="true" />
      <span>{workspaceLabel}</span>
    </div>
    {#if onOpenSettings}
      <button
        class="sidebar-footer-button"
        type="button"
        title="Settings"
        aria-label="Open settings"
        onclick={onOpenSettings}
      >
        <SettingsIcon size={14} aria-hidden="true" />
      </button>
    {/if}
  </footer>
</div>

<style>
  .session-sidebar {
    display: flex;
    flex-direction: column;
    gap: 0;
    height: 100%;
    min-height: 0;
  }

  .sidebar-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    min-height: 3.2rem;
    padding: 0.38rem 0.72rem 0.42rem 4.35rem;
    border-bottom: 1px solid var(--ui-shell-edge);
  }

  .sidebar-header-copy,
  .sidebar-context,
  .sidebar-error {
    margin: 0;
  }

  .sidebar-header-copy {
    display: grid;
    gap: 0.1rem;
    min-width: 0;
  }

  .sidebar-brand-row {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    min-width: 0;
  }

  .sidebar-brand {
    flex: 0 0 auto;
    color: var(--ui-accent);
    font-family: var(--font-mono);
    font-size: 0.68rem;
    font-weight: 700;
    letter-spacing: 0;
  }

  .sidebar-workspace-heading {
    min-width: 0;
    margin: 0;
    padding: 0;
    overflow: hidden;
    border: 0;
    color: var(--ui-text-primary);
    font-size: 0.74rem;
    font-weight: 650;
    line-height: 1.12;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .sidebar-context {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    flex-wrap: wrap;
    font-family: var(--font-mono);
    font-size: 0.56rem;
    color: var(--ui-text-tertiary);
  }

  .branch-pill {
    display: inline-flex;
    align-items: center;
    gap: 0.18rem;
    max-width: 8rem;
    min-height: 1rem;
    padding: 0 0.28rem;
    border: 1px solid var(--ui-border-soft);
    border-radius: var(--ui-radius-sm);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .sidebar-error {
    margin: 0.5rem 0.68rem 0;
    padding: 0.55rem 0.62rem;
    border-radius: var(--ui-radius-md);
    background: color-mix(in oklab, var(--ui-danger-soft) 86%, transparent);
    color: color-mix(in oklab, var(--ui-danger) 82%, var(--ui-text-primary));
    font-size: 0.74rem;
    line-height: 1.5;
  }

  .sidebar-actions {
    display: grid;
    gap: 0.28rem;
    padding: 0.5rem 0.72rem;
    border-bottom: 1px solid var(--ui-shell-edge);
    position: relative;
    z-index: 8;
  }

  .new-session-menu-shell {
    position: relative;
    display: grid;
    min-width: 0;
  }

  :global(button.new-session) {
    justify-content: flex-start;
    width: 100%;
    min-height: 1.76rem;
    padding-inline: 0.52rem;
    border-radius: var(--ui-radius-md);
    font-size: 0.68rem;
  }

  :global(button.new-session .new-session-chevron) {
    flex: 0 0 auto;
    transition: transform 160ms cubic-bezier(0.22, 1, 0.36, 1);
  }

  .new-session-menu-shell.menu-open :global(button.new-session .new-session-chevron) {
    transform: rotate(180deg);
  }

  :global(button.new-session span) {
    flex: 1;
    text-align: left;
  }

  :global(button.new-session kbd),
  .new-session-menu-item kbd {
    flex: 0 0 auto;
    min-width: max-content;
    padding: 0.08rem 0.26rem;
    border: 1px solid color-mix(in oklab, currentColor 24%, transparent);
    border-radius: var(--ui-radius-xs);
    background: color-mix(in oklab, currentColor 10%, transparent);
    font-family: var(--font-mono);
    font-size: 0.52rem;
    font-weight: 650;
    line-height: 1.2;
  }

  :global(button.new-session kbd) {
    color: color-mix(in oklab, var(--ui-accent-ink) 86%, var(--ui-accent));
  }

  .new-session-menu-shell.menu-open :global(button.new-session) {
    border-color: color-mix(in oklab, var(--ui-accent) 44%, var(--ui-border-soft));
    box-shadow: var(--ui-focus-ring);
  }

  .new-session-menu {
    position: absolute;
    top: 100%;
    left: 0;
    z-index: 12;
    width: 100%;
    padding-top: 0.28rem;
    transform-origin: 50% 0;
    animation: new-session-menu-in 150ms cubic-bezier(0.22, 1, 0.36, 1) both;
    will-change: opacity, transform;
  }

  .new-session-menu-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    width: 100%;
    min-height: 1.82rem;
    padding: 0.36rem 0.6rem;
    border: 1px solid color-mix(in oklab, var(--ui-border-strong) 76%, transparent);
    border-radius: var(--ui-radius-md);
    background: linear-gradient(
      180deg,
      color-mix(in oklab, var(--ui-panel) 98%, var(--ui-shell)),
      color-mix(in oklab, var(--ui-panel) 90%, var(--ui-shell))
    );
    box-shadow:
      inset 0 1px 0 color-mix(in oklab, var(--ui-text-primary) 8%, transparent),
      0 0.48rem 1.05rem color-mix(in oklab, var(--ui-bg) 45%, transparent);
    color: var(--ui-text-secondary);
    font-size: 0.66rem;
    font-weight: 600;
    text-align: left;
    cursor: pointer;
    transition:
      border-color 170ms cubic-bezier(0.19, 1, 0.22, 1),
      background-color 170ms cubic-bezier(0.19, 1, 0.22, 1),
      color 170ms cubic-bezier(0.19, 1, 0.22, 1),
      box-shadow 170ms cubic-bezier(0.19, 1, 0.22, 1),
      transform 120ms cubic-bezier(0.22, 1, 0.36, 1);
  }

  .new-session-menu-item span {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .new-session-menu-item:hover,
  .new-session-menu-item:focus-visible {
    outline: none;
    border-color: color-mix(in oklab, var(--ui-accent) 42%, var(--ui-border-soft));
    background: color-mix(in oklab, var(--ui-accent) 9%, var(--ui-panel));
    box-shadow:
      var(--ui-focus-ring),
      inset 0 1px 0 color-mix(in oklab, var(--ui-text-primary) 9%, transparent),
      0 0.55rem 1.2rem color-mix(in oklab, var(--ui-bg) 48%, transparent);
    color: var(--ui-text-primary);
    transform: translateY(-1px);
  }

  .new-session-menu-item:disabled {
    cursor: default;
    opacity: 0.55;
  }

  @keyframes new-session-menu-in {
    from {
      opacity: 0;
      transform: translateY(-0.22rem) scale(0.985);
    }

    to {
      opacity: 1;
      transform: translateY(0) scale(1);
    }
  }

  .sidebar-sections {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 0.36rem 0.24rem 0.62rem;
  }

  .sidebar-list {
    display: grid;
    gap: 0.42rem;
  }

  .sidebar-section {
    display: grid;
    gap: 0.12rem;
  }

  .sidebar-section-label {
    margin: 0.38rem 0 0.12rem;
    padding-inline: 0.72rem;
    font-size: 0.56rem;
    font-family: var(--font-mono);
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--ui-text-tertiary);
  }

  .archived-toggle {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto;
    align-items: center;
    gap: 0.32rem;
    width: 100%;
    min-height: 1.5rem;
    padding: 0.28rem 0.36rem;
    border: 0;
    border-radius: var(--ui-radius-sm);
    background: transparent;
    color: var(--ui-text-tertiary);
    font-size: 0.6rem;
    font-weight: 650;
    text-align: left;
    cursor: pointer;
  }

  .archived-toggle:hover {
    background: color-mix(in oklab, var(--ui-surface-subtle) 78%, transparent);
    color: var(--ui-text-secondary);
  }

  .archived-toggle:focus-visible {
    outline: none;
    box-shadow: var(--ui-focus-ring);
  }

  .reference-nav-section {
    margin-top: 0.28rem;
    padding-top: 0.18rem;
  }

  .reference-nav-row {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto;
    align-items: center;
    gap: 0.38rem;
    width: 100%;
    min-height: 1.65rem;
    padding: 0.24rem 0.72rem;
    border: 1px solid transparent;
    border-radius: var(--ui-radius-sm);
    background: transparent;
    color: var(--ui-text-secondary);
    text-align: left;
    cursor: pointer;
  }

  .reference-nav-row.static {
    cursor: default;
  }

  .reference-nav-row:hover:not(.static) {
    border-color: color-mix(in oklab, var(--ui-border-soft) 72%, transparent);
    background: color-mix(in oklab, var(--ui-surface-subtle) 66%, transparent);
    color: var(--ui-text-primary);
  }

  .reference-nav-row span {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 0.62rem;
    font-weight: 500;
  }

  .reference-nav-row small {
    color: var(--ui-text-tertiary);
    font-family: var(--font-mono);
    font-size: 0.52rem;
  }

  .sidebar-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.42rem;
    min-height: 2rem;
    padding: 0.34rem 0.42rem 0.34rem 0.6rem;
    border-top: 1px solid var(--ui-shell-edge);
    color: var(--ui-text-tertiary);
  }

  .workspace-path {
    display: inline-flex;
    align-items: center;
    gap: 0.32rem;
    min-width: 0;
    font-family: var(--font-mono);
    font-size: 0.56rem;
  }

  .workspace-path span {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .sidebar-footer-button {
    display: inline-grid;
    place-items: center;
    flex: 0 0 auto;
    width: 1.45rem;
    height: 1.45rem;
    border: 0;
    border-radius: var(--ui-radius-sm);
    background: transparent;
    color: inherit;
    cursor: pointer;
  }

  .sidebar-footer-button:hover,
  .sidebar-footer-button:focus-visible {
    outline: none;
    background: var(--ui-surface-subtle);
    color: var(--ui-text-primary);
  }
</style>
