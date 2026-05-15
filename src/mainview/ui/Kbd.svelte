<script lang="ts">
	import ArrowBigUpIcon from "@lucide/svelte/icons/arrow-big-up";
	import CommandIcon from "@lucide/svelte/icons/command";
	import type { Snippet } from "svelte";
	import type { HTMLAttributes } from "svelte/elements";
	import { getKeyboardShortcutParts } from "./keyboard-shortcut";

	type Props = HTMLAttributes<HTMLElement> & {
		value?: string | null;
		children?: Snippet;
	};

	let { value = null, class: className = "", children, ...rest }: Props = $props();
	const parts = $derived(value ? getKeyboardShortcutParts(value) : []);
</script>

<kbd {...rest} class={`ui-kbd ${parts.length > 0 ? "has-parts" : ""} ${className}`.trim()}>
	{#if parts.length > 0}
		{#each parts as part}
			<span class="ui-kbd-key" aria-label={part.label}>
				{#if part.icon === "command"}
					<CommandIcon class="ui-kbd-icon" aria-hidden="true" size={11} strokeWidth={2} />
				{:else if part.icon === "shift"}
					<ArrowBigUpIcon class="ui-kbd-icon" aria-hidden="true" size={11} strokeWidth={2} />
				{:else}
					{part.text}
				{/if}
			</span>
		{/each}
	{:else if children}
		{@render children()}
	{/if}
</kbd>

<style>
	.ui-kbd {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 0.12rem;
		min-width: 1.18rem;
		min-height: 1rem;
		color: inherit;
		font-family: var(--font-mono);
		font-size: 0.58rem;
		font-weight: 720;
		font-variant-numeric: tabular-nums;
		line-height: 1;
		letter-spacing: 0;
		white-space: nowrap;
	}

	.ui-kbd:not(.has-parts),
	.ui-kbd :global(.ui-kbd-key) {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 1rem;
		min-height: 1rem;
		padding: 0.08rem 0.24rem;
		border: 1px solid color-mix(in oklab, currentColor 24%, transparent);
		border-radius: var(--ui-radius-xs);
		background: color-mix(in oklab, currentColor 10%, transparent);
		box-shadow: var(--ui-keycap-shadow);
	}

	.ui-kbd :global(.ui-kbd-icon) {
		width: 0.68rem;
		height: 0.68rem;
		flex: 0 0 auto;
	}

	.ui-kbd.has-parts {
		min-width: 0;
	}

	.ui-kbd.has-parts :global(.ui-kbd-key:first-child) {
		color: color-mix(in oklab, currentColor 94%, var(--ui-accent) 18%);
	}
</style>
