<script lang="ts">
  import PlusIcon from "@lucide/svelte/icons/plus";
  import SearchIcon from "@lucide/svelte/icons/search";
  import CommandIcon from "@lucide/svelte/icons/command";
  import ChevronDownIcon from "@lucide/svelte/icons/chevron-down";
  import ChevronRightIcon from "@lucide/svelte/icons/chevron-right";
  import FolderGit2Icon from "@lucide/svelte/icons/folder-git-2";
  import SettingsIcon from "@lucide/svelte/icons/settings";
  import LogsIcon from "@lucide/svelte/icons/logs";
  import WorkflowIcon from "@lucide/svelte/icons/workflow";
  import ZapIcon from "@lucide/svelte/icons/zap";
  import { getShortcutCompact } from "../shared/shortcut-registry";
  import type { AppLogSummary, WorkspaceSessionNavigationReadModel, WorkspaceSessionSummary } from "../shared/workspace-contract";
  import { formatAppLogCount } from "./app-logs";
  import SessionListItem from "./SessionListItem.svelte";
  import Kbd from "./ui/Kbd.svelte";
  import Tooltip from "./ui/Tooltip.svelte";

  type Props = {
    workspaceLabel: string;
    navigation: WorkspaceSessionNavigationReadModel;
    activeSessionId?: string;
    activeSurface?: "orchestrator" | "thread";
    paneLocationsBySessionId?: Record<string, { paneId: string; label: string; focused: boolean }[]>;
    appLogSummary?: AppLogSummary | null;
    busy?: boolean;
    errorMessage?: string;
    onCreateSession: () => void;
    onCreateDumbSession: () => void;
    onOpenSession: (sessionId: string) => void;
    onRenameSession: (session: WorkspaceSessionSummary) => void;
    onPinSession: (session: WorkspaceSessionSummary) => void;
    onUnpinSession: (session: WorkspaceSessionSummary) => void;
    onArchiveSession: (session: WorkspaceSessionSummary) => void;
    onUnarchiveSession: (session: WorkspaceSessionSummary) => void;
    onToggleArchivedGroup: (collapsed: boolean) => void;
    onOpenSearch?: () => void;
    onOpenCommandPalette?: () => void;
    onOpenWorkflowLibrary?: () => void;
    onOpenAppLogs?: () => void;
    onOpenSettings?: () => void;
  };

  let {
    workspaceLabel,
    navigation,
    activeSessionId,
    activeSurface,
    paneLocationsBySessionId = {},
    appLogSummary = null,
    busy = false,
    errorMessage,
    onCreateSession,
    onCreateDumbSession,
    onOpenSession,
    onRenameSession,
    onPinSession,
    onUnpinSession,
    onArchiveSession,
    onUnarchiveSession,
    onToggleArchivedGroup,
    onOpenSearch,
    onOpenCommandPalette,
    onOpenWorkflowLibrary,
    onOpenAppLogs,
    onOpenSettings,
  }: Props = $props();

  let showNewSessionMenu = $state(false);
  let shortcutAction = $state<string | null>(null);
  const newSessionDisplayShortcut = getShortcutCompact("session.new");
  const dumbSessionDisplayShortcut = getShortcutCompact("session.dumb");
  const quickOpenDisplayShortcut = getShortcutCompact("quickOpen.open");
  const commandPaletteDisplayShortcut = getShortcutCompact("commandPalette.open");

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
      shortcutAction = null;
    }
  }

  function showShortcut(action: string) {
    shortcutAction = action;
  }

  function hideShortcut(action: string) {
    if (shortcutAction === action) {
      shortcutAction = null;
    }
  }

  const appLogUnreadTitle = $derived.by(() => {
    const unread = appLogSummary?.unread;
    if (!unread || unread.total === 0) return "Open app logs";
    return `Open app logs: ${unread.error} errors, ${unread.warning} warnings, ${unread.info} info unread`;
  });
</script>

<svelte:window onkeydown={handleWindowKeydown} />

<div class="session-sidebar">
  <div class="sidebar-window-row electrobun-webkit-app-region-drag" aria-hidden="true"></div>

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
      <div class="sidebar-action-row new-session-row">
        <button
          type="button"
          class={`sidebar-action-main ${shortcutAction === "new" ? "shortcut-open" : ""}`.trim()}
          onmouseenter={() => showShortcut("new")}
          onmouseleave={() => hideShortcut("new")}
          onfocus={() => showShortcut("new")}
          onblur={() => hideShortcut("new")}
          onclick={() => {
            showNewSessionMenu = false;
            onCreateSession();
          }}
          disabled={busy}
          aria-label="Create a new session"
        >
          <span class="sidebar-action-icon"><PlusIcon aria-hidden="true" size={15} strokeWidth={1.9} /></span>
          <span class="sidebar-action-label">New session</span>
          <Kbd value={newSessionDisplayShortcut} class="sidebar-action-shortcut" />
        </button>
      </div>
      <div class="new-session-accordion" aria-hidden={!showNewSessionMenu}>
        <div class="new-session-accordion-inner">
          <button
            type="button"
            class={`sidebar-action-row new-session-child ${shortcutAction === "dumb" ? "shortcut-open" : ""}`.trim()}
            disabled={busy}
            tabindex={showNewSessionMenu ? 0 : -1}
            onmouseenter={() => showShortcut("dumb")}
            onmouseleave={() => hideShortcut("dumb")}
            onfocus={() => showShortcut("dumb")}
            onblur={() => hideShortcut("dumb")}
            onclick={() => {
              showNewSessionMenu = false;
              onCreateDumbSession();
            }}
          >
            <span class="sidebar-action-icon"><ZapIcon aria-hidden="true" size={14} strokeWidth={1.9} /></span>
            <span class="sidebar-action-label">New dumb session</span>
            <Kbd value={dumbSessionDisplayShortcut} class="sidebar-action-shortcut" />
          </button>
        </div>
      </div>
    </div>
    {#if onOpenSearch}
      <button
        class={`sidebar-action-row ${shortcutAction === "search" ? "shortcut-open" : ""}`.trim()}
        type="button"
        aria-label="Open quick open"
        onmouseenter={() => showShortcut("search")}
        onmouseleave={() => hideShortcut("search")}
        onfocus={() => showShortcut("search")}
        onblur={() => hideShortcut("search")}
        onclick={onOpenSearch}
      >
        <span class="sidebar-action-icon"><SearchIcon size={15} aria-hidden="true" strokeWidth={1.9} /></span>
        <span class="sidebar-action-label">Search</span>
        <Kbd value={quickOpenDisplayShortcut} class="sidebar-action-shortcut" />
      </button>
    {/if}
    {#if onOpenCommandPalette}
      <button
        class={`sidebar-action-row ${shortcutAction === "commands" ? "shortcut-open" : ""}`.trim()}
        type="button"
        aria-label="Open command palette"
        onmouseenter={() => showShortcut("commands")}
        onmouseleave={() => hideShortcut("commands")}
        onfocus={() => showShortcut("commands")}
        onblur={() => hideShortcut("commands")}
        onclick={onOpenCommandPalette}
      >
        <span class="sidebar-action-icon"><CommandIcon size={15} aria-hidden="true" strokeWidth={1.9} /></span>
        <span class="sidebar-action-label">Command palette</span>
        <Kbd value={commandPaletteDisplayShortcut} class="sidebar-action-shortcut" />
      </button>
    {/if}
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
                onPin={() => onPinSession(session)}
                onUnpin={() => onUnpinSession(session)}
                onArchive={() => onArchiveSession(session)}
                onUnarchive={() => onUnarchiveSession(session)}
              />
            {/each}
          {/if}
        </section>
      {/if}

    </div>
  </div>

  {#if onOpenWorkflowLibrary || onOpenAppLogs}
    <div class="sidebar-lower-nav">
      {#if onOpenAppLogs}
        <Tooltip label={appLogUnreadTitle} side="right" block>
          <button
            class={`sidebar-action-row reference-nav-row logs-nav-row ${(appLogSummary?.unread.error ?? 0) > 0 ? "has-errors" : ""}`.trim()}
            type="button"
            aria-label={appLogUnreadTitle}
            title={appLogUnreadTitle}
            onclick={onOpenAppLogs}
          >
            <span class="sidebar-action-icon"><LogsIcon size={15} aria-hidden="true" strokeWidth={1.9} /></span>
            <span class="sidebar-action-label">Logs</span>
            <span class="log-unread-badges" aria-hidden="true">
              {#if (appLogSummary?.unread.error ?? 0) > 0}
                <small class="log-badge error">{formatAppLogCount(appLogSummary!.unread.error)}</small>
              {/if}
              {#if (appLogSummary?.unread.warning ?? 0) > 0}
                <small class="log-badge warning">{formatAppLogCount(appLogSummary!.unread.warning)}</small>
              {/if}
              {#if (appLogSummary?.unread.info ?? 0) > 0}
                <small class="log-badge info">{formatAppLogCount(appLogSummary!.unread.info)}</small>
              {/if}
            </span>
          </button>
        </Tooltip>
      {/if}
      {#if onOpenWorkflowLibrary}
        <Tooltip label="Open saved workflow assets" side="right" block>
          <button class="sidebar-action-row reference-nav-row" type="button" aria-label="Open saved workflows" onclick={onOpenWorkflowLibrary}>
            <span class="sidebar-action-icon"><WorkflowIcon size={15} aria-hidden="true" strokeWidth={1.9} /></span>
            <span class="sidebar-action-label">Saved workflows</span>
            <small class="sidebar-action-shortcut">.svvy</small>
          </button>
        </Tooltip>
      {/if}
    </div>
  {/if}

  <footer class="sidebar-footer">
    <div class="workspace-path" title={workspaceLabel}>
      <FolderGit2Icon size={12} aria-hidden="true" />
      <span>{workspaceLabel}</span>
    </div>
    {#if onOpenSettings}
      <Tooltip label="Open settings">
        <button
          class="sidebar-footer-button"
          type="button"
          aria-label="Open settings"
          onclick={onOpenSettings}
        >
          <SettingsIcon size={14} aria-hidden="true" />
        </button>
      </Tooltip>
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

  .sidebar-window-row {
    flex: 0 0 auto;
    min-height: 2.45rem;
  }

  .sidebar-error {
    margin: 0;
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
    gap: 0.08rem;
    padding: 0.2rem 0.72rem 0.7rem;
    position: relative;
    z-index: 8;
  }

  .new-session-menu-shell {
    display: grid;
    min-width: 0;
  }

  .sidebar-action-row {
    --sidebar-row-height: 1.82rem;
    display: grid;
    grid-template-columns: 1.1rem minmax(0, 1fr) minmax(1.24rem, auto);
    align-items: center;
    gap: 0.48rem;
    width: 100%;
    min-height: var(--sidebar-row-height);
    padding: 0.26rem 0.34rem;
    border: 0;
    border-radius: var(--ui-radius-sm);
    background: transparent;
    color: var(--ui-text-secondary);
    font-size: 0.76rem;
    font-weight: 500;
    line-height: 1.25;
    text-align: left;
    cursor: pointer;
    min-width: 0;
    transition:
      background-color 150ms cubic-bezier(0.19, 1, 0.22, 1),
      color 150ms cubic-bezier(0.19, 1, 0.22, 1),
      opacity 150ms cubic-bezier(0.19, 1, 0.22, 1);
  }

  .sidebar-action-row:hover,
  .sidebar-action-row:focus-visible,
  .new-session-row:focus-within,
  .new-session-menu-shell.menu-open .new-session-row {
    outline: none;
    background: color-mix(in oklab, var(--ui-surface-subtle) 68%, transparent);
    color: var(--ui-text-primary);
  }

  .sidebar-action-row:focus-visible,
  .new-session-row:has(:focus-visible) {
    box-shadow: var(--ui-focus-ring);
  }

  .sidebar-action-row:disabled {
    cursor: not-allowed;
    opacity: 0.5;
  }

  .sidebar-action-icon {
    display: grid;
    place-items: center;
    width: 1.1rem;
    color: inherit;
  }

  .sidebar-action-label {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  :global(.sidebar-action-shortcut) {
    justify-self: end;
    flex: 0 0 auto;
    min-width: max-content;
    max-width: 4.9rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--ui-text-tertiary);
    font-family: var(--font-mono);
    font-size: 0.52rem;
    font-weight: 650;
    line-height: 1.2;
    opacity: 0;
    transform: translateX(0.12rem);
    transition:
      opacity 110ms cubic-bezier(0.19, 1, 0.22, 1),
      transform 110ms cubic-bezier(0.19, 1, 0.22, 1);
  }

  .sidebar-action-row:hover :global(.sidebar-action-shortcut),
  .sidebar-action-row:focus-visible :global(.sidebar-action-shortcut),
  .sidebar-action-row:focus-within :global(.sidebar-action-shortcut),
  .sidebar-action-main:hover :global(.sidebar-action-shortcut),
  .sidebar-action-main:focus-visible :global(.sidebar-action-shortcut),
  .shortcut-open :global(.sidebar-action-shortcut),
  .new-session-menu-shell.menu-open .new-session-row :global(.sidebar-action-shortcut) {
    opacity: 1;
    transform: translateX(0);
  }

  .new-session-row {
    grid-template-columns: minmax(0, 1fr);
    padding: 0;
    color: var(--ui-text-primary);
  }

  .sidebar-action-main {
    grid-column: 1 / -1;
    display: grid;
    grid-template-columns: 1.1rem minmax(0, 1fr) minmax(1.24rem, auto);
    align-items: center;
    gap: 0.48rem;
    width: 100%;
    min-width: 0;
    min-height: var(--sidebar-row-height);
    padding: 0.26rem 0.34rem;
    border: 0;
    background: transparent;
    color: inherit;
    font: inherit;
    text-align: left;
    cursor: pointer;
  }

  .sidebar-action-main:focus-visible {
    outline: none;
  }

  .sidebar-action-main:disabled {
    cursor: not-allowed;
  }

  .new-session-accordion {
    display: grid;
    grid-template-rows: 0fr;
    opacity: 0;
    transform: translateY(-0.16rem);
    pointer-events: none;
    transition:
      grid-template-rows 170ms cubic-bezier(0.22, 1, 0.36, 1),
      opacity 140ms cubic-bezier(0.19, 1, 0.22, 1),
      transform 170ms cubic-bezier(0.22, 1, 0.36, 1);
  }

  .new-session-menu-shell.menu-open .new-session-accordion {
    grid-template-rows: 1fr;
    opacity: 1;
    transform: translateY(0);
    pointer-events: auto;
  }

  .new-session-accordion-inner {
    overflow: hidden;
    min-height: 0;
    padding: 0.08rem 0 0;
  }

  .new-session-child {
    color: var(--ui-text-secondary);
    font-size: 0.72rem;
  }

  .new-session-child .sidebar-action-icon {
    color: var(--ui-text-tertiary);
  }

  .sidebar-sections {
    flex: 1;
    min-height: 0;
    overflow-x: hidden;
    overflow-y: auto;
    padding: 0.16rem 0.24rem 0.62rem;
  }

  .sidebar-list {
    display: grid;
    gap: 0.42rem;
    min-width: 0;
  }

  .sidebar-section {
    display: grid;
    gap: 0.12rem;
    min-width: 0;
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

  .sidebar-lower-nav {
    display: grid;
    flex: 0 0 auto;
    padding: 0.25rem 0.72rem 0.42rem;
  }

  .reference-nav-row {
    font-size: 0.76rem;
    font-weight: 500;
  }

  .logs-nav-row.has-errors {
    background: color-mix(in oklab, var(--ui-danger-soft) 54%, transparent);
    color: color-mix(in oklab, var(--ui-danger) 30%, var(--ui-text-secondary));
  }

  .log-unread-badges {
    display: inline-flex;
    align-items: center;
    justify-self: end;
    gap: 0.16rem;
    min-width: 0;
    font-variant-numeric: tabular-nums;
  }

  .log-badge {
    display: inline-grid;
    place-items: center;
    min-width: 1rem;
    height: 0.92rem;
    padding: 0 0.18rem;
    border-radius: var(--ui-radius-xs);
    font-family: var(--font-mono);
    font-size: 0.52rem;
    font-weight: 750;
    line-height: 1;
  }

  .log-badge.info {
    color: var(--ui-info);
    background: var(--ui-info-soft);
  }

  .log-badge.warning {
    color: var(--ui-warning);
    background: var(--ui-warning-soft);
  }

  .log-badge.error {
    color: var(--ui-danger);
    background: var(--ui-danger-soft);
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

  @media (prefers-reduced-motion: reduce) {
    .sidebar-action-row,
    .sidebar-action-shortcut,
    .new-session-accordion {
      transition: none;
      animation: none;
    }
  }
</style>
