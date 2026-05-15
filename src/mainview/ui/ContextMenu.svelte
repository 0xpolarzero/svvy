<script lang="ts">
  export type ContextMenuItem = {
    id: string;
    label: string;
    disabled?: boolean;
  };

  type Props = {
    x: number;
    y: number;
    label: string;
    items: ContextMenuItem[];
    onSelect: (item: ContextMenuItem) => void;
    onClose?: () => void;
  };

  let { x, y, label, items, onSelect, onClose }: Props = $props();
  let element = $state<HTMLDivElement | null>(null);

  export function contains(target: Node | null): boolean {
    return Boolean(target && element?.contains(target));
  }

  function selectItem(item: ContextMenuItem) {
    if (item.disabled) return;
    onSelect(item);
    onClose?.();
  }
</script>

<div
  bind:this={element}
  class="context-menu"
  role="menu"
  aria-label={label}
  style={`left: ${x}px; top: ${y}px;`}
  oncontextmenu={(event) => event.preventDefault()}
>
  {#each items as item (item.id)}
    <button
      type="button"
      role="menuitem"
      disabled={item.disabled}
      onclick={() => selectItem(item)}
    >
      {item.label}
    </button>
  {/each}
</div>

<style>
  .context-menu {
    --compact-control-font-family: var(--font-mono);
    --compact-control-font-weight: 500;

    position: fixed;
    z-index: var(--ui-z-overlay);
    display: grid;
    gap: 0;
    min-width: 10rem;
    max-width: min(14rem, calc(100vw - 2rem));
    padding: 0.28rem;
    border: 1px solid var(--ui-border-soft);
    border-radius: var(--ui-radius-md);
    background: color-mix(in oklab, var(--ui-surface-raised) 96%, transparent);
    box-shadow:
      0 18px 48px color-mix(in oklab, var(--ui-shadow) 28%, transparent),
      0 0 0 1px color-mix(in oklab, var(--ui-surface) 60%, transparent);
    backdrop-filter: blur(16px);
    transform-origin: top left;
  }

  .context-menu button {
    display: flex;
    align-items: center;
    width: 100%;
    min-height: 1.8rem;
    padding: 0 0.56rem;
    border: 0;
    border-radius: var(--ui-radius-sm);
    background: transparent;
    color: var(--ui-text-secondary);
    font-family: var(--compact-control-font-family);
    font-size: var(--text-xs);
    font-weight: var(--compact-control-font-weight);
    line-height: 1;
    text-align: left;
    cursor: pointer;
  }

  .context-menu button:hover,
  .context-menu button:focus-visible {
    outline: none;
    background: var(--ui-surface-subtle);
    color: var(--ui-text-primary);
  }

  .context-menu button:disabled {
    color: var(--ui-text-tertiary);
    cursor: default;
    opacity: 1;
  }
</style>
