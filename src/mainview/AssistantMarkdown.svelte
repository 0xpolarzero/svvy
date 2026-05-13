<script lang="ts">
	import { Marked, Renderer, type Tokens } from "marked";

	type Props = {
		content: string;
		isFinished: boolean;
	};

	let { content, isFinished }: Props = $props();

	const renderer = new Renderer();
	const markdown = new Marked({
		async: false,
		breaks: false,
		gfm: true,
		pedantic: false,
		renderer,
	});

	function escapeHtml(value: string) {
		return value
			.replaceAll("&", "&amp;")
			.replaceAll("<", "&lt;")
			.replaceAll(">", "&gt;")
			.replaceAll('"', "&quot;")
			.replaceAll("'", "&#39;");
	}

	function safeUrl(value: string) {
		try {
			const parsed = new URL(value, "https://svvy.local");
			if (["http:", "https:", "mailto:"].includes(parsed.protocol)) {
				return value;
			}
		} catch {
			if (value.startsWith("#") || value.startsWith("/")) {
				return value;
			}
		}
		return "";
	}

	renderer.html = ({ text }) => escapeHtml(text);
	renderer.link = ({ href, title, tokens }: Tokens.Link) => {
		const safeHref = safeUrl(href);
		const label = markdown.parseInline(tokens) as string;
		if (!safeHref) return label;
		const titleAttribute = title ? ` title="${escapeHtml(title)}"` : "";
		return `<a href="${escapeHtml(safeHref)}"${titleAttribute} target="_blank" rel="noreferrer">${label}</a>`;
	};
	renderer.image = ({ href, title, text }: Tokens.Image) => {
		const safeHref = safeUrl(href);
		if (!safeHref) return escapeHtml(text);
		const titleAttribute = title ? ` title="${escapeHtml(title)}"` : "";
		return `<img src="${escapeHtml(safeHref)}" alt="${escapeHtml(text)}"${titleAttribute} loading="lazy" />`;
	};

	const renderedHtml = $derived(markdown.parse(content) as string);
</script>

<div class="assistant-markdown" data-finished={isFinished}>
	{@html renderedHtml}
</div>

<style>
	.assistant-markdown {
		min-width: 0;
		overflow-wrap: anywhere;
		word-break: break-word;
		font-size: 0.81rem;
		line-height: 1.58;
		color: var(--ui-text-primary);
	}

	.assistant-markdown :global(*:first-child) {
		margin-top: 0;
	}

	.assistant-markdown :global(*:last-child) {
		margin-bottom: 0;
	}

	.assistant-markdown :global(p) {
		margin: 0;
		white-space: pre-wrap;
	}

	.assistant-markdown :global(p + p),
	.assistant-markdown :global(p + ul),
	.assistant-markdown :global(p + ol),
	.assistant-markdown :global(ul + p),
	.assistant-markdown :global(ol + p),
	.assistant-markdown :global(pre + p),
	.assistant-markdown :global(p + pre),
	.assistant-markdown :global(table + p),
	.assistant-markdown :global(p + table),
	.assistant-markdown :global(blockquote + p),
	.assistant-markdown :global(p + blockquote) {
		margin-top: 0.72rem;
	}

	.assistant-markdown :global(h1),
	.assistant-markdown :global(h2),
	.assistant-markdown :global(h3),
	.assistant-markdown :global(h4),
	.assistant-markdown :global(h5),
	.assistant-markdown :global(h6) {
		margin: 0.95rem 0 0.45rem;
		font-size: 0.92rem;
		line-height: 1.35;
		font-weight: 680;
		letter-spacing: 0;
		color: var(--ui-text-primary);
	}

	.assistant-markdown :global(h1) {
		font-size: 1.05rem;
	}

	.assistant-markdown :global(h2) {
		font-size: 0.98rem;
	}

	.assistant-markdown :global(ul),
	.assistant-markdown :global(ol) {
		margin: 0.52rem 0 0;
		padding-left: 1.15rem;
	}

	.assistant-markdown :global(ul ul),
	.assistant-markdown :global(ul ol),
	.assistant-markdown :global(ol ul),
	.assistant-markdown :global(ol ol) {
		margin-top: 0.22rem;
	}

	.assistant-markdown :global(li) {
		margin: 0.22rem 0;
		padding-left: 0.08rem;
	}

	.assistant-markdown :global(.contains-task-list),
	.assistant-markdown :global(ul:has(> li > input[type="checkbox"])) {
		padding-left: 0;
		list-style: none;
	}

	.assistant-markdown :global(.task-list-item),
	.assistant-markdown :global(li:has(> input[type="checkbox"])) {
		display: flex;
		gap: 0.42rem;
		align-items: flex-start;
		padding-left: 0;
	}

	.assistant-markdown :global(.task-list-item input),
	.assistant-markdown :global(li > input[type="checkbox"]) {
		width: 0.82rem;
		height: 0.82rem;
		margin: 0.29rem 0 0;
		accent-color: var(--ui-accent);
	}

	.assistant-markdown :global(code) {
		padding: 0.08rem 0.26rem;
		border-radius: var(--ui-radius-xs);
		background: color-mix(in oklab, var(--ui-surface-muted) 72%, transparent);
		font-family: var(--font-mono);
		font-size: 0.86em;
		color: color-mix(in oklab, var(--ui-text-primary) 94%, var(--ui-accent));
	}

	.assistant-markdown :global(pre) {
		margin: 0.72rem 0 0;
		overflow-x: auto;
		border-radius: var(--ui-radius-sm);
		border: 1px solid color-mix(in oklab, var(--ui-border-soft) 84%, transparent);
		background: color-mix(in oklab, var(--ui-code) 92%, var(--ui-surface));
		padding: 0.62rem 0.68rem;
	}

	.assistant-markdown :global(pre code) {
		display: block;
		min-width: max-content;
		padding: 0;
		border-radius: 0;
		background: transparent;
		color: var(--ui-text-primary);
		font-family: var(--font-mono);
		font-size: 0.74rem;
		line-height: 1.5;
		white-space: pre;
	}

	.assistant-markdown :global(blockquote) {
		margin: 0.72rem 0 0;
		padding: 0.1rem 0 0.1rem 0.7rem;
		border-left: 2px solid color-mix(in oklab, var(--ui-accent) 62%, var(--ui-border-strong));
		color: var(--ui-text-secondary);
	}

	.assistant-markdown :global(blockquote p) {
		white-space: normal;
	}

	.assistant-markdown :global(table) {
		display: block;
		width: max-content;
		max-width: 100%;
		margin-top: 0.72rem;
		overflow-x: auto;
		border-collapse: collapse;
		font-size: 0.76rem;
	}

	.assistant-markdown :global(th),
	.assistant-markdown :global(td) {
		padding: 0.38rem 0.5rem;
		border: 1px solid color-mix(in oklab, var(--ui-border-soft) 84%, transparent);
		text-align: left;
		vertical-align: top;
	}

	.assistant-markdown :global(th) {
		background: color-mix(in oklab, var(--ui-surface-muted) 64%, transparent);
		font-weight: 650;
	}

	.assistant-markdown :global(hr) {
		height: 1px;
		margin: 0.72rem 0;
		border: none;
		background: color-mix(in oklab, var(--ui-border-soft) 84%, transparent);
	}

	.assistant-markdown :global(del) {
		color: var(--ui-text-tertiary);
	}

	.assistant-markdown :global(a) {
		color: color-mix(in oklab, var(--ui-accent) 82%, var(--ui-text-primary));
		text-decoration: underline;
		text-decoration-thickness: 1px;
		text-underline-offset: 0.18em;
	}

	.assistant-markdown :global(a:hover),
	.assistant-markdown :global(a:focus-visible) {
		outline: none;
		color: var(--ui-text-primary);
		background: color-mix(in oklab, var(--ui-accent-soft) 72%, transparent);
	}
</style>
