<script lang="ts">
  import PlusIcon from "@lucide/svelte/icons/plus";
  import SearchIcon from "@lucide/svelte/icons/search";
  import CommandIcon from "@lucide/svelte/icons/command";
  import ChevronDownIcon from "@lucide/svelte/icons/chevron-down";
  import ChevronRightIcon from "@lucide/svelte/icons/chevron-right";
  import SettingsIcon from "@lucide/svelte/icons/settings";
  import LogsIcon from "@lucide/svelte/icons/logs";
  import WorkflowIcon from "@lucide/svelte/icons/workflow";
  import ZapIcon from "@lucide/svelte/icons/zap";
  import type { ContextBudget } from "../shared/context-budget";
  import { getShortcutCompact } from "../shared/shortcut-registry";
  import type {
    AppLogSummary,
    WorkspaceBranchInfo,
    WorkspaceSessionNavigationReadModel,
    WorkspaceSessionSummary,
    WorkspaceSidebarHandlerThreadRow,
    WorkspaceSidebarRowSubtitle,
    WorkspaceSidebarWorkflowRow,
  } from "../shared/workspace-contract";
  import {
    formatAppLogCount,
    formatAppLogUnreadTitle,
    getVisibleAppLogUnreadBadges,
  } from "./app-logs";
  import SessionListItem from "./SessionListItem.svelte";
  import ContextBudgetBar from "./ContextBudgetBar.svelte";
  import Kbd from "./ui/Kbd.svelte";
  import Tooltip from "./ui/Tooltip.svelte";
  import CompactCombobox, { type CompactComboboxOption } from "./ui/CompactCombobox.svelte";
  import ContextMenu, { type ContextMenuItem } from "./ui/ContextMenu.svelte";

  type SidebarPaneLocation = {
    paneId: string;
    label: string;
    focused: boolean;
    tone: "neutral" | "waiting" | "error";
    contextBudget: ContextBudget | null;
  };

  type Props = {
    workspaceLabel: string;
    workspaceBranch?: string;
    navigation: WorkspaceSessionNavigationReadModel;
    activeSessionId?: string;
    activeOrchestratorSessionId?: string;
    activeThreadId?: string;
    paneLocationsBySessionId?: Record<string, SidebarPaneLocation[]>;
    paneLocationsByThreadId?: Record<string, SidebarPaneLocation[]>;
    paneLocationsByWorkflowRunId?: Record<string, SidebarPaneLocation[]>;
    appLogSummary?: AppLogSummary | null;
    busy?: boolean;
    errorMessage?: string;
    onCreateSession: () => void;
    onCreateDumbSession: () => void;
    onOpenSession: (sessionId: string, event: MouseEvent) => void;
    onOpenHandlerThread?: (sessionId: string, thread: WorkspaceSidebarHandlerThreadRow) => void;
    onOpenWorkflowRun?: (sessionId: string, workflow: WorkspaceSidebarWorkflowRow) => void;
    onRenameSession: (session: WorkspaceSessionSummary) => void;
    onPinSession: (session: WorkspaceSessionSummary) => void;
    onUnpinSession: (session: WorkspaceSessionSummary) => void;
    onArchiveSession: (session: WorkspaceSessionSummary) => void;
    onUnarchiveSession: (session: WorkspaceSessionSummary) => void;
    onMarkSessionUnread: (session: WorkspaceSessionSummary) => void;
    onMarkSessionRead: (session: WorkspaceSessionSummary) => void;
    onToggleArchivedGroup: (collapsed: boolean) => void;
    onOpenSearch?: () => void;
    onOpenCommandPalette?: () => void;
    onOpenWorkflowLibrary?: () => void;
    onOpenAppLogs?: () => void;
    onOpenSettings?: () => void;
    onListWorkspaceBranches?: () => Promise<WorkspaceBranchInfo[]>;
    onSwitchWorkspaceBranch?: (branch: string) => Promise<void>;
  };

  let {
    workspaceLabel,
    workspaceBranch,
    navigation,
    activeSessionId,
    activeOrchestratorSessionId,
    activeThreadId,
    paneLocationsBySessionId = {},
    paneLocationsByThreadId = {},
    paneLocationsByWorkflowRunId = {},
    appLogSummary = null,
    busy = false,
    errorMessage,
    onCreateSession,
    onCreateDumbSession,
    onOpenSession,
    onOpenHandlerThread,
    onOpenWorkflowRun,
    onRenameSession,
    onPinSession,
    onUnpinSession,
    onArchiveSession,
    onUnarchiveSession,
    onMarkSessionUnread,
    onMarkSessionRead,
    onToggleArchivedGroup,
    onOpenSearch,
    onOpenCommandPalette,
    onOpenWorkflowLibrary,
    onOpenAppLogs,
    onOpenSettings,
    onListWorkspaceBranches,
    onSwitchWorkspaceBranch,
  }: Props = $props();

  const footerWorkspaceLabel = $derived(workspaceBranch ?? workspaceLabel);
  const footerShowsBranch = $derived(Boolean(workspaceBranch));
  const branchControlEnabled = $derived(
    footerShowsBranch && Boolean(onListWorkspaceBranches && onSwitchWorkspaceBranch),
  );
  const branchSelectOptions = $derived.by<CompactComboboxOption[]>(() => {
    if (branchMenuError) return [{ value: "__error", label: branchMenuError, disabled: true }];
    if (branchOptions.length === 0) {
      return [{ value: "__empty", label: "No local branches found.", disabled: true }];
    }
    return branchOptions.map((branch) => ({ value: branch.name, label: branch.name }));
  });

  let showNewSessionMenu = $state(false);
  let shortcutAction = $state<string | null>(null);
  let relativeTimeNow = $state(Date.now());
  let branchMenuOpen = $state(false);
  let branchMenuLoading = $state(false);
  let branchMenuError = $state<string | null>(null);
  let branchOptions = $state<WorkspaceBranchInfo[]>([]);
  let sessionContextMenu = $state<{
    session: WorkspaceSessionSummary;
    x: number;
    y: number;
  } | null>(null);
  let sessionContextMenuElement = $state<ContextMenu | null>(null);
  let relativeTimeTimeout: ReturnType<typeof window.setTimeout> | null = null;
  let relativeTimeInterval: ReturnType<typeof window.setInterval> | null = null;
  const newSessionDisplayShortcut = getShortcutCompact("session.new");
  const dumbSessionDisplayShortcut = getShortcutCompact("session.dumb");
  const quickOpenDisplayShortcut = getShortcutCompact("quickOpen.open");
  const commandPaletteDisplayShortcut = getShortcutCompact("commandPalette.open");

  $effect(() => {
    function updateRelativeTimeNow() {
      relativeTimeNow = Date.now();
    }

    const msUntilNextMinute = 60000 - (Date.now() % 60000);
    relativeTimeTimeout = window.setTimeout(() => {
      updateRelativeTimeNow();
      relativeTimeInterval = window.setInterval(updateRelativeTimeNow, 60000);
    }, msUntilNextMinute);

    return () => {
      if (relativeTimeTimeout) {
        window.clearTimeout(relativeTimeTimeout);
        relativeTimeTimeout = null;
      }
      if (relativeTimeInterval) {
        window.clearInterval(relativeTimeInterval);
        relativeTimeInterval = null;
      }
    };
  });

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
      sessionContextMenu = null;
    }
  }

  function handleWindowPointerDown(event: PointerEvent) {
    const target = event.target as Node | null;
    if (sessionContextMenuElement?.contains(target)) {
      return;
    }
    sessionContextMenu = null;
  }

  async function loadBranchOptions() {
    if (!onListWorkspaceBranches) return;
    branchMenuError = null;
    branchMenuLoading = true;
    try {
      branchOptions = await onListWorkspaceBranches();
    } catch (error) {
      branchMenuError = error instanceof Error ? error.message : "Unable to load branches.";
    } finally {
      branchMenuLoading = false;
    }
  }

  async function switchBranch(branch: string) {
    if (branch === workspaceBranch || !onSwitchWorkspaceBranch) return;
    branchMenuError = null;
    branchMenuLoading = true;
    try {
      await onSwitchWorkspaceBranch(branch);
      branchOptions = branchOptions.map((option) => ({
        ...option,
        current: option.name === branch,
      }));
    } catch (error) {
      branchMenuError = error instanceof Error ? error.message : "Unable to switch branch.";
    } finally {
      branchMenuLoading = false;
    }
  }

  function openSessionContextMenu(
    session: WorkspaceSessionSummary,
    event: MouseEvent | KeyboardEvent,
  ) {
    const currentTarget = event.currentTarget as HTMLElement | null;
    const fallbackRect = currentTarget?.getBoundingClientRect();
    const x = "clientX" in event && event.clientX > 0 ? event.clientX : (fallbackRect?.left ?? 0);
    const y =
      "clientY" in event && event.clientY > 0 ? event.clientY : (fallbackRect?.bottom ?? 0);
    sessionContextMenu = {
      session,
      x: Math.max(8, Math.min(x, window.innerWidth - 220)),
      y: Math.max(8, Math.min(y, window.innerHeight - 190)),
    };
  }

  function closeSessionContextMenu() {
    sessionContextMenu = null;
  }

  function runSessionContextAction(action: () => void) {
    action();
    closeSessionContextMenu();
  }

  function getSessionContextMenuItems(session: WorkspaceSessionSummary): ContextMenuItem[] {
    const renameLocked = session.titleGeneration?.renameLocked ?? false;
    return [
      {
        id: "read-state",
        label: session.isUnread ? "Mark as Read" : "Mark as Unread",
      },
      {
        id: "pin-state",
        label: session.isPinned ? "Unpin" : "Pin",
      },
      {
        id: "rename",
        label: "Rename",
        disabled: renameLocked,
      },
      {
        id: "archive-state",
        label: session.isArchived ? "Unarchive" : "Archive",
      },
    ];
  }

  function selectSessionContextMenuItem(session: WorkspaceSessionSummary, item: ContextMenuItem) {
    if (item.id === "read-state") {
      runSessionContextAction(() =>
        session.isUnread ? onMarkSessionRead(session) : onMarkSessionUnread(session),
      );
      return;
    }
    if (item.id === "pin-state") {
      runSessionContextAction(() =>
        session.isPinned ? onUnpinSession(session) : onPinSession(session),
      );
      return;
    }
    if (item.id === "rename") {
      runSessionContextAction(() => onRenameSession(session));
      return;
    }
    if (item.id === "archive-state") {
      runSessionContextAction(() =>
        session.isArchived ? onUnarchiveSession(session) : onArchiveSession(session),
      );
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
    return formatAppLogUnreadTitle(appLogSummary);
  });
  const appLogUnreadBadges = $derived(getVisibleAppLogUnreadBadges(appLogSummary));

  function isThreadWorking(thread: WorkspaceSidebarHandlerThreadRow): boolean {
    return thread.status === "running-handler" || thread.status === "running-workflow";
  }

  function isWorkflowWorking(workflow: WorkspaceSidebarWorkflowRow): boolean {
    return workflow.status === "running";
  }

  function getSubtitleClass(
    subtitle: WorkspaceSidebarRowSubtitle | null,
    working = false,
  ): string {
    return `${subtitle ? `sidebar-child-subtitle tone-${subtitle.tone}` : "sidebar-child-subtitle"} ${
      working ? "blinking" : ""
    }`.trim();
  }

  function getPaneTone(paneLocations: SidebarPaneLocation[]): SidebarPaneLocation["tone"] {
    return (
      paneLocations.find((location) => location.tone === "error")?.tone ??
      paneLocations.find((location) => location.tone === "waiting")?.tone ??
      "neutral"
    );
  }

  function getPrimaryPaneLocation(paneLocations: SidebarPaneLocation[]): SidebarPaneLocation | null {
    return paneLocations.find((location) => location.focused) ?? paneLocations[0] ?? null;
  }
</script>

<svelte:window onkeydown={handleWindowKeydown} onpointerdown={handleWindowPointerDown} />

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

  {#snippet sessionChildren(session: WorkspaceSessionSummary)}
    {#if (session.sidebarThreads?.length ?? 0) > 0}
      <div class="sidebar-child-list" aria-label={`Handler threads for ${session.title}`}>
        {#each session.sidebarThreads ?? [] as thread (thread.threadId)}
          {@const threadPaneLocations = paneLocationsByThreadId[thread.threadId] ?? []}
          {@const threadPrimaryPane = getPrimaryPaneLocation(threadPaneLocations)}
          {@const threadWorking = isThreadWorking(thread)}
          <button
            type="button"
            class={`sidebar-child-row handler-row status-${thread.status} ${session.id === activeSessionId && thread.threadId === activeThreadId ? "active" : ""} ${threadPaneLocations.length > 0 ? "open-in-pane" : ""} open-tone-${getPaneTone(threadPaneLocations)} ${threadWorking ? "working" : ""}`.trim()}
            onclick={() => onOpenHandlerThread?.(session.id, thread)}
            disabled={busy}
          >
            <span class="sidebar-child-content">
              <span class="sidebar-child-title">{thread.title}</span>
              {#if thread.subtitle}
                <span class={getSubtitleClass(thread.subtitle, threadWorking)}>
                  <span>{thread.subtitle.badge}</span>
                  <span>{thread.subtitle.text}</span>
                </span>
              {/if}
              {#if threadPrimaryPane?.contextBudget}
                <span class="sidebar-child-context" aria-hidden="true">
                  <ContextBudgetBar budget={threadPrimaryPane.contextBudget} variant="compact" label="Context" />
                </span>
              {/if}
            </span>
          </button>
          {#if thread.workflows.length > 0}
            <div class="sidebar-workflow-list" aria-label={`Workflow runs for ${thread.title}`}>
              {#each thread.workflows as workflow (workflow.workflowRunId)}
                {@const workflowPaneLocations = paneLocationsByWorkflowRunId[workflow.workflowRunId] ?? []}
                {@const workflowWorking = isWorkflowWorking(workflow)}
                <button
                  type="button"
                  class={`sidebar-child-row workflow-row status-${workflow.status} ${workflowPaneLocations.length > 0 ? "open-in-pane" : ""} open-tone-${getPaneTone(workflowPaneLocations)} ${workflowWorking ? "working" : ""}`.trim()}
                  onclick={() => onOpenWorkflowRun?.(session.id, workflow)}
                  disabled={busy}
                >
                  <span class="sidebar-child-content">
                    <span class="sidebar-child-title">{workflow.workflowName}</span>
                    {#if workflow.subtitle}
                      <span class={getSubtitleClass(workflow.subtitle, workflowWorking)}>
                        <span>{workflow.subtitle.badge}</span>
                        <span>{workflow.subtitle.text}</span>
                      </span>
                    {/if}
                  </span>
                </button>
              {/each}
            </div>
          {/if}
        {/each}
      </div>
    {/if}
  {/snippet}

  <div class="sidebar-sections">
    <div class="sidebar-list">
      {#if navigation.pinnedSessions.length > 0}
        <section class="sidebar-section" aria-label="Pinned sessions">
          <p class="sidebar-section-label">Pinned</p>
          {#each navigation.pinnedSessions as session (session.id)}
            <SessionListItem
              active={session.id === activeOrchestratorSessionId}
              disabled={busy && session.id !== activeSessionId}
              paneLocations={paneLocationsBySessionId[session.id] ?? []}
              {relativeTimeNow}
              {session}
              onOpen={(event) => onOpenSession(session.id, event)}
              onRename={() => onRenameSession(session)}
              onPin={() => onPinSession(session)}
              onUnpin={() => onUnpinSession(session)}
              onArchive={() => onArchiveSession(session)}
              onUnarchive={() => onUnarchiveSession(session)}
              onContextMenu={(event) => openSessionContextMenu(session, event)}
            />
            {@render sessionChildren(session)}
          {/each}
        </section>
      {/if}

      {#if navigation.activeSessions.length > 0}
        <section class="sidebar-section" aria-label="Active sessions">
          <p class="sidebar-section-label">Active</p>
          {#each navigation.activeSessions as session (session.id)}
            <SessionListItem
              active={session.id === activeOrchestratorSessionId}
              disabled={busy && session.id !== activeSessionId}
              paneLocations={paneLocationsBySessionId[session.id] ?? []}
              {relativeTimeNow}
              {session}
              onOpen={(event) => onOpenSession(session.id, event)}
              onRename={() => onRenameSession(session)}
              onPin={() => onPinSession(session)}
              onUnpin={() => onUnpinSession(session)}
              onArchive={() => onArchiveSession(session)}
              onUnarchive={() => onUnarchiveSession(session)}
              onContextMenu={(event) => openSessionContextMenu(session, event)}
            />
            {@render sessionChildren(session)}
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
                active={session.id === activeOrchestratorSessionId}
                disabled={busy && session.id !== activeSessionId}
                paneLocations={paneLocationsBySessionId[session.id] ?? []}
                {relativeTimeNow}
                {session}
                onOpen={(event) => onOpenSession(session.id, event)}
                onRename={() => onRenameSession(session)}
                onPin={() => onPinSession(session)}
                onUnpin={() => onUnpinSession(session)}
                onArchive={() => onArchiveSession(session)}
                onUnarchive={() => onUnarchiveSession(session)}
                onContextMenu={(event) => openSessionContextMenu(session, event)}
              />
              {@render sessionChildren(session)}
            {/each}
          {/if}
        </section>
      {/if}

    </div>
  </div>

  {#if sessionContextMenu}
    {@const menuSession = sessionContextMenu.session}
    <ContextMenu
      bind:this={sessionContextMenuElement}
      x={sessionContextMenu.x}
      y={sessionContextMenu.y}
      label={`Session actions for ${menuSession.title}`}
      items={getSessionContextMenuItems(menuSession)}
      onSelect={(item) => selectSessionContextMenuItem(menuSession, item)}
      onClose={closeSessionContextMenu}
    />
  {/if}

  {#if onOpenWorkflowLibrary || onOpenAppLogs}
    <div class="sidebar-lower-nav">
      {#if onOpenAppLogs}
        <Tooltip label={appLogUnreadTitle} side="right" block>
          <button
            class={`sidebar-action-row reference-nav-row logs-nav-row ${(appLogSummary?.unread.error ?? 0) > 0 ? "has-errors" : ""}`.trim()}
            type="button"
            aria-label={appLogUnreadTitle}
            onclick={onOpenAppLogs}
          >
            <span class="sidebar-action-icon"><LogsIcon size={15} aria-hidden="true" strokeWidth={1.9} /></span>
            <span class="sidebar-action-label">Logs</span>
            <span class="log-unread-badges" aria-hidden="true">
              {#each appLogUnreadBadges as badge (badge.level)}
                <small class={`log-badge ${badge.level}`}>{formatAppLogCount(badge.count)}</small>
              {/each}
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
    <Tooltip label="Switch branch">
      <CompactCombobox
        bind:open={branchMenuOpen}
        value={footerWorkspaceLabel}
        options={branchSelectOptions}
        ariaLabel="Switch branch"
        placeholder="Search branches"
        emptyLabel="No branches match."
        disabled={!branchControlEnabled || busy}
        triggerClass="workspace-path"
        menuClass="branch-menu"
        optionClass="branch-option"
        leadingIcon={footerShowsBranch ? "branch" : "workspace"}
        onBeforeOpen={loadBranchOptions}
        onSelect={switchBranch}
      />
    </Tooltip>
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
    min-height: var(--workspace-chrome-height, 42px);
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
    font-size: var(--text-sm);
    line-height: 1.5;
  }

  .sidebar-actions {
    display: grid;
    gap: 0.08rem;
    padding: 0.08rem 0.72rem 0.7rem;
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
    font-size: var(--text-sm);
    font-weight: 460;
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
    background: var(--ui-hover-bg);
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
    font-size: var(--text-xs);
    font-weight: 600;
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
    font-size: var(--text-sm);
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

  .sidebar-child-list,
  .sidebar-workflow-list {
    display: grid;
    gap: 0.08rem;
    min-width: 0;
  }

  .sidebar-child-list {
    margin: -0.02rem 0 0.18rem 0.72rem;
    padding-left: 0.44rem;
  }

  .sidebar-workflow-list {
    margin-left: 0.62rem;
    padding-left: 0.5rem;
    border-left: 1px solid color-mix(in oklab, var(--ui-border-soft) 52%, transparent);
  }

  .sidebar-child-row {
    position: relative;
    display: grid;
    align-items: start;
    width: 100%;
    min-width: 0;
    padding: 0.32rem 0.42rem 0.3rem 0.82rem;
    overflow: hidden;
    border: 1px solid transparent;
    border-radius: var(--ui-radius-sm);
    background: transparent;
    color: var(--ui-text-secondary);
    font: inherit;
    text-align: left;
    cursor: pointer;
    transition:
      background-color 150ms cubic-bezier(0.19, 1, 0.22, 1),
      border-color 150ms cubic-bezier(0.19, 1, 0.22, 1),
      color 150ms cubic-bezier(0.19, 1, 0.22, 1),
      opacity 150ms cubic-bezier(0.19, 1, 0.22, 1);
  }

  .sidebar-child-row::before {
    content: "";
    position: absolute;
    top: 0;
    bottom: 0;
    left: 0;
    width: 0.12rem;
    border-radius: var(--ui-radius-sm) 0 0 var(--ui-radius-sm);
    background: transparent;
    transition: background-color 150ms cubic-bezier(0.19, 1, 0.22, 1);
  }

  .sidebar-child-row:hover:not(:disabled),
  .sidebar-child-row:focus-visible {
    outline: none;
    background: color-mix(in oklab, var(--ui-surface-subtle) 62%, transparent);
    color: var(--ui-text-primary);
  }

  .sidebar-child-row:focus-visible {
    box-shadow: var(--ui-focus-ring);
  }

  .sidebar-child-row.active {
    border-color: transparent;
    background: color-mix(in oklab, var(--ui-surface-subtle) 82%, transparent);
  }

  .sidebar-child-row.active::before {
    background: color-mix(in oklab, var(--ui-accent) 84%, transparent);
  }

  .sidebar-child-row.open-in-pane:not(.active) {
    border-color: transparent;
    background: color-mix(in oklab, var(--ui-surface-subtle) 34%, transparent);
  }

  .sidebar-child-row.open-in-pane:not(.active)::before {
    background: color-mix(in oklab, var(--ui-text-tertiary) 42%, transparent);
  }

  .sidebar-child-row.open-tone-waiting:not(.active) {
    border-color: transparent;
    background: color-mix(in oklab, var(--ui-status-waiting-soft) 28%, transparent);
  }

  .sidebar-child-row.open-tone-waiting:not(.active)::before {
    background: color-mix(in oklab, var(--ui-status-waiting) 54%, transparent);
  }

  .sidebar-child-row.open-tone-error:not(.active) {
    border-color: transparent;
    background: color-mix(in oklab, var(--ui-danger-soft) 30%, transparent);
  }

  .sidebar-child-row.open-tone-error:not(.active)::before {
    background: color-mix(in oklab, var(--ui-danger) 56%, transparent);
  }

  .sidebar-child-row:disabled {
    cursor: not-allowed;
    opacity: 0.55;
  }

  .workflow-row {
    padding-block: 0.25rem;
    color: var(--ui-text-tertiary);
  }

  .sidebar-child-content {
    display: grid;
    gap: 0.12rem;
    min-width: 0;
  }

  .sidebar-child-title {
    min-width: 0;
    overflow: hidden;
    color: inherit;
    font-size: var(--text-xs);
    font-weight: 500;
    line-height: 1.22;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .workflow-row .sidebar-child-title {
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    font-weight: 500;
  }

  .sidebar-child-subtitle {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    align-items: baseline;
    gap: 0.28rem;
    min-width: 0;
    overflow: hidden;
    font-size: var(--text-xs);
    line-height: 1.25;
    color: var(--ui-text-tertiary);
  }

  .sidebar-child-subtitle span:first-child {
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    font-weight: 600;
    text-transform: lowercase;
    white-space: nowrap;
  }

  .sidebar-child-subtitle span:last-child {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .sidebar-child-subtitle.tone-waiting {
    color: color-mix(in oklab, var(--ui-status-waiting) 76%, var(--ui-text-primary));
  }

  .sidebar-child-subtitle.tone-error {
    color: color-mix(in oklab, var(--ui-danger) 82%, var(--ui-text-primary));
  }

  .sidebar-child-subtitle.text-only {
    grid-template-columns: minmax(0, 1fr);
  }

  .sidebar-child-subtitle.text-only span {
    min-width: 0;
    overflow: hidden;
    color: var(--ui-text-tertiary);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    font-weight: 600;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .sidebar-child-context {
    display: block;
    margin-top: 0.1rem;
  }

  .sidebar-child-context :global(.context-budget-compact) {
    position: static;
    width: 100%;
    grid-template-columns: minmax(0, 1fr);
  }

  .sidebar-child-context :global(.context-budget-compact-label) {
    display: none;
  }

  .blinking {
    animation: sidebar-working-blink 1.8s ease-in-out infinite;
  }

  @keyframes sidebar-working-blink {
    0%,
    100% {
      opacity: 0.38;
    }
    50% {
      opacity: 1;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .blinking {
      animation: none;
    }
  }

  .sidebar-section-label {
    margin: 0.38rem 0 0.12rem;
    padding-inline: 0.72rem;
    font-size: var(--text-xs);
    font-family: var(--font-mono);
    letter-spacing: var(--tracking-wide);
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
    font-size: var(--text-xs);
    font-weight: 600;
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
    font-size: var(--text-sm);
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
    font-size: var(--text-xs);
    font-weight: 700;
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
    position: relative;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.42rem;
    min-height: 2rem;
    padding: 0.34rem 0.42rem 0.34rem 0.6rem;
    border-top: 1px solid var(--ui-shell-edge);
    color: var(--ui-text-tertiary);
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
