<script lang="ts">
  import AlertOctagonIcon from "@lucide/svelte/icons/alert-octagon";
  import ExternalLinkIcon from "@lucide/svelte/icons/external-link";
  import RefreshCwIcon from "@lucide/svelte/icons/refresh-cw";

  type Props = {
    title?: string;
    testsPassed: number;
    testsTotal: number;
    errorSnippet?: string;
    class?: string;
    onretry?: () => void;
    onreportopen?: () => void;
  };

  let {
    title = "Verification failed",
    testsPassed,
    testsTotal,
    errorSnippet,
    class: className = "",
    onretry,
    onreportopen,
  }: Props = $props();

  const testsFailed = $derived(Math.max(0, testsTotal - testsPassed));
</script>

<article class={`reference-failed-card ${className}`.trim()} data-testid="failed-card">
  <header>
    <AlertOctagonIcon size={14} strokeWidth={2} />
    <strong>{title}</strong>
    <span>{testsFailed} failed · {testsPassed} passed</span>
  </header>
  <div class="failed-body">
    {#if errorSnippet}
      <pre>{errorSnippet}</pre>
    {/if}
    <div class="failed-actions">
      {#if onreportopen}
        <button type="button" onclick={onreportopen}><ExternalLinkIcon size={12} />View full report</button>
      {/if}
      {#if onretry}
        <button type="button" onclick={onretry}><RefreshCwIcon size={12} />Retry</button>
      {/if}
    </div>
  </div>
</article>

<style>
  .reference-failed-card {
    border: 1px solid color-mix(in oklab, var(--ui-danger) 34%, var(--ui-border-soft));
    border-radius: var(--ui-radius-md);
    background: color-mix(in oklab, var(--ui-surface) 94%, var(--ui-danger-soft));
    overflow: hidden;
  }

  header,
  .failed-actions {
    display: flex;
    align-items: center;
  }

  header {
    gap: 0.45rem;
    padding: 0.62rem 0.72rem;
    border-bottom: 1px solid var(--ui-border-soft);
    color: color-mix(in oklab, var(--ui-danger) 84%, var(--ui-text-primary));
  }

  strong {
    color: color-mix(in oklab, var(--ui-danger) 84%, var(--ui-text-primary));
    font-size: var(--text-sm);
    font-weight: 600;
  }

  header span {
    margin-left: auto;
    padding: 0.08rem 0.34rem;
    border: 1px solid color-mix(in oklab, var(--ui-danger) 24%, var(--ui-border-soft));
    border-radius: var(--ui-radius-sm);
    background: var(--ui-danger-soft);
    color: color-mix(in oklab, var(--ui-danger) 82%, var(--ui-text-primary));
    font-family: var(--font-mono);
    font-size: var(--text-xs);
  }

  .failed-body {
    display: grid;
    gap: 0.62rem;
    padding: 0.7rem 0.78rem;
  }

  pre {
    margin: 0;
    max-height: 7rem;
    overflow: auto;
    padding: 0.58rem 0.64rem;
    border: 1px solid color-mix(in oklab, var(--ui-danger) 20%, var(--ui-border-soft));
    border-radius: var(--ui-radius-sm);
    background: color-mix(in oklab, var(--ui-danger-soft) 48%, var(--ui-code));
    color: color-mix(in oklab, var(--ui-danger) 80%, var(--ui-text-primary));
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    line-height: 1.5;
    white-space: pre-wrap;
  }

  .failed-actions {
    gap: 0.42rem;
    flex-wrap: wrap;
  }

  button {
    position: relative;
    display: inline-flex;
    align-items: center;
    gap: 0.26rem;
    min-height: 1.65rem;
    padding: 0.22rem 0.52rem;
    border: 1px solid var(--ui-border-soft);
    border-radius: var(--ui-radius-sm);
    background: transparent;
    color: var(--ui-text-secondary);
    font-size: var(--text-xs);
    cursor: pointer;
  }

  button::before {
    content: "";
    position: absolute;
    inset: 50% auto auto 50%;
    width: max(100%, 1.85rem);
    height: max(100%, 1.85rem);
    transform: translate(-50%, -50%);
    border-radius: inherit;
  }

  button:hover {
    border-color: color-mix(in oklab, var(--ui-danger) 24%, var(--ui-border-soft));
    color: var(--ui-text-primary);
  }

  button:focus-visible {
    outline: none;
    box-shadow: var(--ui-focus-ring);
  }
</style>
