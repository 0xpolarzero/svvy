<script lang="ts">
  import FolderOpenIcon from "@lucide/svelte/icons/folder-open";
  import SquarePlusIcon from "@lucide/svelte/icons/square-plus";
  import Button from "./ui/Button.svelte";

  type Props = {
    openingWorkspace?: boolean;
    errorMessage?: string | null;
    recentWorkspaces?: unknown[];
    onOpenWorkspace: () => void;
    onOpenWorkspaceInNewTab?: () => void;
  };

  let {
    openingWorkspace = false,
    errorMessage = null,
    recentWorkspaces = [],
    onOpenWorkspace,
    onOpenWorkspaceInNewTab,
  }: Props = $props();
</script>

<section class="open-workspace-panel" data-testid="open-workspace-panel">
  <div class="open-workspace-content">
    <div class="open-workspace-icon" aria-hidden="true">
      <FolderOpenIcon size={22} strokeWidth={1.8} />
    </div>
    <div class="open-workspace-copy">
      <h2>Open Workspace</h2>
      <p>Choose a local repository or folder to work in.</p>
      {#if errorMessage}
        <p class="open-workspace-error" role="alert">{errorMessage}</p>
      {/if}
    </div>
    <div class="open-workspace-actions">
      <Button
        variant="primary"
        loading={openingWorkspace}
        onclick={onOpenWorkspace}
      >
        <FolderOpenIcon size={15} strokeWidth={2} aria-hidden="true" />
        Open Workspace
      </Button>
      <Button
        variant="secondary"
        disabled={openingWorkspace}
        onclick={() => onOpenWorkspaceInNewTab?.()}
      >
        <SquarePlusIcon size={15} strokeWidth={2} aria-hidden="true" />
        Open in New Tab
      </Button>
    </div>
    {#if recentWorkspaces.length > 0}
      <div class="open-workspace-recents" aria-label="Recent workspaces"></div>
    {/if}
  </div>
</section>

<style>
  .open-workspace-panel {
    display: grid;
    align-items: start;
    height: 100%;
    min-height: 0;
    padding: 2.4rem;
    overflow: auto;
    background: var(--ui-panel);
    color: var(--ui-text-primary);
  }

  .open-workspace-content {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr);
    gap: 0.9rem 1rem;
    max-width: 34rem;
  }

  .open-workspace-icon {
    display: grid;
    place-items: center;
    width: 2.25rem;
    height: 2.25rem;
    border: 1px solid var(--ui-border-soft);
    border-radius: var(--ui-radius-md);
    background: var(--ui-surface-subtle);
    color: var(--ui-text-secondary);
  }

  .open-workspace-copy {
    display: grid;
    gap: 0.25rem;
    min-width: 0;
  }

  .open-workspace-copy h2 {
    margin: 0;
    font-size: var(--text-lg);
    font-weight: 650;
    line-height: 1.2;
    letter-spacing: 0;
  }

  .open-workspace-copy p {
    margin: 0;
    color: var(--ui-text-secondary);
    font-size: var(--text-sm);
    line-height: 1.45;
  }

  .open-workspace-error {
    color: var(--ui-danger-text, var(--ui-text-primary));
  }

  .open-workspace-actions {
    grid-column: 2;
    display: flex;
    flex-wrap: wrap;
    gap: 0.55rem;
    padding-top: 0.2rem;
  }

  @media (max-width: 620px) {
    .open-workspace-panel {
      padding: 1.2rem;
    }

    .open-workspace-content {
      grid-template-columns: 1fr;
    }

    .open-workspace-actions {
      grid-column: 1;
    }
  }
</style>
