<script lang="ts">
	import { onDestroy } from "svelte";
	import type { Snippet } from "svelte";
	import type { HTMLAttributes } from "svelte/elements";
	import { appendKeyboardShortcutParts } from "./keyboard-shortcut";

	type TooltipSide = "top" | "bottom" | "left" | "right";
	type TooltipDetailIcon = "mouse-left";
	type TooltipDetail = {
		label: string;
		value?: string | null;
		shortcut?: string | null;
		icon?: TooltipDetailIcon;
	};

	type Props = HTMLAttributes<HTMLSpanElement> & {
		label: string;
		shortcut?: string | null;
		details?: TooltipDetail[];
		delayMs?: number;
		side?: TooltipSide;
		block?: boolean;
		disabled?: boolean;
		children?: Snippet;
	};

	let {
		label,
		shortcut = null,
		details = [],
		delayMs = 500,
		side = "top",
		block = false,
		disabled = false,
		class: className = "",
		children,
		...rest
	}: Props = $props();

	let anchor = $state<HTMLSpanElement | null>(null);
	let tooltipElement = $state<HTMLDivElement | null>(null);
	let open = $state(false);
	let left = $state(0);
	let top = $state(0);
	let timer: ReturnType<typeof setTimeout> | null = null;

	function clearOpenTimer() {
		if (timer) {
			clearTimeout(timer);
			timer = null;
		}
	}

	function renderTooltipElement() {
		if (!open || disabled || (!label && details.length === 0)) return;
		if (!tooltipElement) {
			tooltipElement = document.createElement("div");
			tooltipElement.role = "tooltip";
			document.body.append(tooltipElement);
		}
		tooltipElement.className = `ui-tooltip side-${side}`.trim();
		tooltipElement.replaceChildren();
		if (details.length > 0) {
			const listElement = document.createElement("span");
			listElement.className = "ui-tooltip-details";
			for (const detail of details) {
				const rowElement = document.createElement("span");
				rowElement.className = "ui-tooltip-detail";
				if (detail.shortcut) {
					const shortcutElement = document.createElement("kbd");
					shortcutElement.className = "ui-kbd ui-tooltip-shortcut";
					appendKeyboardShortcutParts(shortcutElement, detail.shortcut);
					rowElement.append(shortcutElement);
				}
				if (detail.icon) {
					rowElement.append(createTooltipDetailIcon(detail.icon));
				}
				const labelElement = document.createElement("span");
				labelElement.className = "ui-tooltip-label";
				labelElement.textContent = detail.label;
				rowElement.append(labelElement);
				if (detail.value) {
					const valueElement = document.createElement("span");
					valueElement.className = "ui-tooltip-value";
					valueElement.textContent = detail.value;
					rowElement.append(valueElement);
				}
				listElement.append(rowElement);
			}
			tooltipElement.append(listElement);
			return;
		}
		const labelElement = document.createElement("span");
		labelElement.className = "ui-tooltip-label";
		labelElement.textContent = label;
		tooltipElement.append(labelElement);
		if (shortcut) {
			const shortcutElement = document.createElement("kbd");
			shortcutElement.className = "ui-kbd ui-tooltip-shortcut";
			appendKeyboardShortcutParts(shortcutElement, shortcut);
			tooltipElement.append(shortcutElement);
		}
	}

	function createTooltipDetailIcon(icon: TooltipDetailIcon): SVGSVGElement {
		const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
		svg.setAttribute("aria-hidden", "true");
		svg.setAttribute("class", `ui-tooltip-detail-icon icon-${icon}`);
		svg.setAttribute("fill", "none");
		svg.setAttribute("focusable", "false");
		svg.setAttribute("stroke", "currentColor");
		svg.setAttribute("stroke-linecap", "round");
		svg.setAttribute("stroke-linejoin", "round");
		svg.setAttribute("stroke-width", "2");
		svg.setAttribute("viewBox", "0 0 24 24");
		const path = document.createElementNS("http://www.w3.org/2000/svg", "path");
		path.setAttribute("d", "M12 7.318V10");
		const body = document.createElementNS("http://www.w3.org/2000/svg", "path");
		body.setAttribute("d", "M5 10v5a7 7 0 0 0 14 0V9c0-3.527-2.608-6.515-6-7");
		const circle = document.createElementNS("http://www.w3.org/2000/svg", "circle");
		circle.setAttribute("cx", "7");
		circle.setAttribute("cy", "4");
		circle.setAttribute("r", "2");
		svg.append(path, body, circle);
		return svg;
	}

	function updatePosition() {
		if (!anchor) return;
		const rect = anchor.getBoundingClientRect();
		const gap = 8;
		const margin = 10;
		const tooltipRect = tooltipElement?.getBoundingClientRect();
		const tooltipWidth = tooltipRect?.width ?? 0;
		const tooltipHeight = tooltipRect?.height ?? 0;
		const preferredLeft =
			side === "left"
				? rect.left - gap
				: side === "right"
					? rect.right + gap
					: rect.left + rect.width / 2;
		const preferredTop =
			side === "bottom"
				? rect.bottom + gap
				: side === "left" || side === "right"
					? rect.top + rect.height / 2
					: rect.top - gap;
		const leftMin = side === "left" ? margin + tooltipWidth : side === "right" ? margin : margin + tooltipWidth / 2;
		const leftMax =
			side === "left" ? window.innerWidth - margin : side === "right" ? window.innerWidth - margin - tooltipWidth : window.innerWidth - margin - tooltipWidth / 2;
		const topMin = side === "top" ? margin + tooltipHeight : side === "bottom" ? margin : margin + tooltipHeight / 2;
		const topMax =
			side === "top" ? window.innerHeight - margin : side === "bottom" ? window.innerHeight - margin - tooltipHeight : window.innerHeight - margin - tooltipHeight / 2;
		left = Math.max(leftMin, Math.min(leftMax, preferredLeft));
		top = Math.max(topMin, Math.min(topMax, preferredTop));
		if (tooltipElement) {
			tooltipElement.style.left = `${left}px`;
			tooltipElement.style.top = `${top}px`;
		}
	}

	function scheduleOpen() {
		if (disabled || (!label && details.length === 0)) return;
		if (timer || open) return;
		clearOpenTimer();
		timer = setTimeout(() => {
			open = true;
			renderTooltipElement();
			updatePosition();
			requestAnimationFrame(updatePosition);
		}, delayMs);
	}

	function handlePointerOut(event: PointerEvent) {
		if (anchor && event.relatedTarget instanceof Node && anchor.contains(event.relatedTarget)) {
			return;
		}
		close();
	}

	function close() {
		clearOpenTimer();
		open = false;
		tooltipElement?.remove();
		tooltipElement = null;
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === "Escape") {
			close();
		}
	}

	function handleWindowPointerMove(event: PointerEvent) {
		if (!anchor) {
			close();
			return;
		}
		const rect = anchor.getBoundingClientRect();
		if (
			event.clientX >= rect.left &&
			event.clientX <= rect.right &&
			event.clientY >= rect.top &&
			event.clientY <= rect.bottom
		) {
			return;
		}
		close();
	}

	$effect(() => {
		if (disabled) {
			close();
		}
	});

	$effect(() => {
		if (!open) return;
		renderTooltipElement();
		updatePosition();
		window.addEventListener("scroll", updatePosition, true);
		window.addEventListener("resize", updatePosition);
		window.addEventListener("pointermove", handleWindowPointerMove, true);
		return () => {
			window.removeEventListener("scroll", updatePosition, true);
			window.removeEventListener("resize", updatePosition);
			window.removeEventListener("pointermove", handleWindowPointerMove, true);
		};
	});

	onDestroy(() => {
		close();
	});
</script>

<span
	{...rest}
	bind:this={anchor}
	class={`ui-tooltip-anchor ${block ? "is-block" : ""} ${className}`.trim()}
	onpointerenter={scheduleOpen}
	onpointerover={scheduleOpen}
	onpointerleave={close}
	onpointerout={handlePointerOut}
	onfocusin={scheduleOpen}
	onfocusout={close}
	onclick={close}
	onkeydown={handleKeydown}
>
	{#if children}
		{@render children()}
	{/if}
</span>

<style>
	.ui-tooltip-anchor {
		display: inline-flex;
		min-width: 0;
	}

	.ui-tooltip-anchor.is-block {
		display: block;
		width: 100%;
	}

	:global(.ui-tooltip) {
		position: fixed;
		z-index: var(--ui-z-dialog);
		display: inline-flex;
		align-items: flex-start;
		gap: 0.44rem;
		width: max-content;
		max-width: min(18rem, calc(100vw - 1.25rem));
		padding: 0.34rem 0.46rem;
		border: 1px solid var(--ui-tooltip-border);
		border-radius: var(--ui-radius-sm);
		background: var(--ui-tooltip-bg);
		color: var(--ui-text-primary);
		box-shadow:
			0 18px 36px color-mix(in oklab, var(--ui-shadow) 28%, transparent),
			0 2px 8px color-mix(in oklab, var(--ui-shadow) 18%, transparent);
		font-size: var(--text-xs);
		font-weight: 500;
		line-height: 1.25;
		pointer-events: none;
		animation: ui-tooltip-in 110ms cubic-bezier(0.19, 1, 0.22, 1);
	}

	:global(.ui-tooltip.side-top) {
		transform: translate(-50%, -100%);
	}

	:global(.ui-tooltip.side-bottom) {
		transform: translate(-50%, 0);
	}

	:global(.ui-tooltip.side-left) {
		transform: translate(-100%, -50%);
	}

	:global(.ui-tooltip.side-right) {
		transform: translate(0, -50%);
	}

	:global(.ui-tooltip-label) {
		min-width: 0;
		overflow-wrap: anywhere;
	}

	:global(.ui-tooltip-details) {
		display: grid;
		gap: 0.22rem;
		min-width: 12rem;
	}

	:global(.ui-tooltip-detail) {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		align-items: center;
		gap: 0.36rem;
		min-width: 0;
	}

	:global(.ui-tooltip-value) {
		color: var(--ui-text-primary);
		font-family: var(--font-mono);
		font-variant-numeric: tabular-nums;
		font-weight: 700;
		white-space: nowrap;
	}

	:global(.ui-tooltip-detail-icon) {
		width: 0.78rem;
		height: 0.78rem;
		flex: 0 0 auto;
		color: var(--ui-text-secondary);
	}

	:global(.ui-tooltip-shortcut) {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		flex: 0 0 auto;
		gap: 0.12rem;
		color: var(--ui-text-secondary);
		font-family: var(--font-mono);
		font-size: var(--text-xs);
		font-weight: 700;
		font-variant-numeric: tabular-nums;
		line-height: 1;
		letter-spacing: 0;
		white-space: nowrap;
	}

	:global(.ui-tooltip-shortcut .ui-kbd-key) {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 1rem;
		min-height: 1rem;
		padding: 0.08rem 0.24rem;
		color: var(--ui-keycap-text);
		border: 1px solid var(--ui-keycap-border);
		border-radius: var(--ui-radius-xs);
		background: var(--ui-keycap-bg);
		box-shadow: var(--ui-keycap-shadow);
	}

	:global(.ui-tooltip-shortcut .ui-kbd-icon) {
		width: 0.68rem;
		height: 0.68rem;
		flex: 0 0 auto;
	}

	@keyframes ui-tooltip-in {
		from {
			opacity: 0;
			filter: blur(2px);
		}
		to {
			opacity: 1;
			filter: blur(0);
		}
	}

	@media (prefers-reduced-motion: reduce) {
		:global(.ui-tooltip) {
			animation: none;
		}
	}
</style>
