<script lang="ts">
	import { createForm } from "@tanstack/svelte-form";
	import type { Model } from "@mariozechner/pi-ai";
	import type { ThinkingLevel } from "@mariozechner/pi-agent-core";
	import { onDestroy } from "svelte";
	import type {
		SessionAgentSettings,
		WorkflowAgentSettings,
	} from "../shared/agent-settings";
	import { getSupportedThinkingLevels } from "./model-thinking";
	import Button from "./ui/Button.svelte";

	type EditableAgentSettings = SessionAgentSettings | WorkflowAgentSettings;

	type ModelOption = {
		key: string;
		provider: string;
		model: Model<any>;
	};

	type Props = {
		title: string;
		summary: string;
		settings: EditableAgentSettings;
		availableModelOptions: ModelOption[];
		onSave: (settings: EditableAgentSettings) => Promise<EditableAgentSettings | void>;
	};

	let { title, summary, settings, availableModelOptions, onSave }: Props = $props();
	let saveMessage = $state("");
	let saveTimer: ReturnType<typeof setTimeout> | null = null;

	type AgentFormValue = {
		modelKey: string;
		reasoningEffort: ThinkingLevel;
		systemPrompt: string;
	};

	function toFormValue(value: EditableAgentSettings): AgentFormValue {
		return {
			modelKey: `${value.provider}:${value.model}`,
			reasoningEffort: value.reasoningEffort,
			systemPrompt: value.systemPrompt,
		};
	}

	function toAgentSettings(value: AgentFormValue): EditableAgentSettings {
		const option = availableModelOptions.find((entry) => entry.key === value.modelKey);
		const [fallbackProvider, fallbackModel] = value.modelKey.split(":");
		return {
			...settings,
			provider: option?.provider ?? fallbackProvider ?? settings.provider,
			model: option?.model.id ?? fallbackModel ?? settings.model,
			reasoningEffort: value.reasoningEffort,
			systemPrompt: value.systemPrompt,
		};
	}

	function selectedModel(value: AgentFormValue): Model<any> | null {
		return availableModelOptions.find((entry) => entry.key === value.modelKey)?.model ?? null;
	}

	function reasoningLevels(value: AgentFormValue): ThinkingLevel[] {
		return getSupportedThinkingLevels(selectedModel(value));
	}

	function modelLabel(option: ModelOption): string {
		return `${option.provider} / ${option.model.name}`;
	}

	function scheduleSaveMessage(message: string, timeoutMs = 0) {
		saveMessage = message;
		if (timeoutMs > 0) {
			setTimeout(() => {
				if (saveMessage === message) saveMessage = "";
			}, timeoutMs);
		}
	}

	const form = createForm(() => ({
		defaultValues: toFormValue(settings),
		validators: {
			onChange: ({ value }) => {
				if (!value.modelKey.trim()) return "Select a model.";
				if (!availableModelOptions.some((entry) => entry.key === value.modelKey)) {
					return "Select a model from a connected provider.";
				}
				if (!reasoningLevels(value).includes(value.reasoningEffort)) {
					return "Select a reasoning level supported by this model.";
				}
				if (!value.systemPrompt.trim()) return "System prompt is required.";
				return undefined;
			},
		},
		onSubmit: async ({ value, formApi }) => {
			scheduleSaveMessage("Saving");
			const saved = await onSave(toAgentSettings(value));
			const nextValue = toFormValue(saved ?? toAgentSettings(value));
			formApi.reset(nextValue);
			scheduleSaveMessage("Saved", 1800);
		},
	}));

	const formState = form.useStore();

	function submitSoon(delayMs: number) {
		if (saveTimer) {
			clearTimeout(saveTimer);
		}
		saveTimer = setTimeout(() => {
			saveTimer = null;
			void form.handleSubmit().catch((error) => {
				scheduleSaveMessage(error instanceof Error ? error.message : "Save failed");
			});
		}, delayMs);
	}

	function submitNow() {
		if (saveTimer) {
			clearTimeout(saveTimer);
			saveTimer = null;
		}
		void form.handleSubmit().catch((error) => {
			scheduleSaveMessage(error instanceof Error ? error.message : "Save failed");
		});
	}

	function resetForm() {
		if (saveTimer) {
			clearTimeout(saveTimer);
			saveTimer = null;
		}
		form.reset();
		saveMessage = "";
	}

	function updateModel(value: string) {
		const nextValue = {
			...form.state.values,
			modelKey: value,
		};
		if (!reasoningLevels(nextValue).includes(nextValue.reasoningEffort)) {
			nextValue.reasoningEffort = "medium";
		}
		form.setFieldValue("modelKey", nextValue.modelKey);
		form.setFieldValue("reasoningEffort", nextValue.reasoningEffort);
		submitNow();
	}

	function updateReasoning(value: string) {
		form.setFieldValue("reasoningEffort", value as ThinkingLevel);
		submitNow();
	}

	onDestroy(() => {
		if (saveTimer) {
			clearTimeout(saveTimer);
		}
	});
</script>

<article class="provider-row agent-row">
	<div class="provider-main">
		<div class="provider-heading">
			<span class="provider-name">{title}</span>
			<span class="model-chip">{toAgentSettings(formState.current.values).provider} / {toAgentSettings(formState.current.values).model}</span>
			<span class="provider-status tone-info">{formState.current.values.reasoningEffort}</span>
			{#if saveMessage}
				<span class="provider-status">{saveMessage}</span>
			{/if}
			{#if formState.current.isDirty}
				<span class="provider-status tone-warning">Dirty</span>
			{/if}
		</div>
		<p class="provider-meta">{summary}</p>
		<div class="agent-meta-grid">
			<div><span>Model</span><strong>{toAgentSettings(formState.current.values).model}</strong></div>
			<div><span>Provider</span><strong>{toAgentSettings(formState.current.values).provider}</strong></div>
			<div><span>Reasoning</span><strong>{formState.current.values.reasoningEffort}</strong></div>
		</div>
		<div class="agent-grid">
			<label class="agent-field">
				<span>Model</span>
				<select
					value={formState.current.values.modelKey}
					disabled={availableModelOptions.length === 0 || formState.current.isSubmitting}
					onchange={(event) => updateModel(event.currentTarget.value)}
				>
					{#if !availableModelOptions.some((option) => option.key === formState.current.values.modelKey)}
						<option value={formState.current.values.modelKey}>
							{toAgentSettings(formState.current.values).provider} / {toAgentSettings(formState.current.values).model}
						</option>
					{/if}
					{#each availableModelOptions as option (option.key)}
						<option value={option.key}>{modelLabel(option)}</option>
					{/each}
				</select>
			</label>
			<label class="agent-field">
				<span>Reasoning</span>
				<select
					value={formState.current.values.reasoningEffort}
					disabled={formState.current.isSubmitting}
					onchange={(event) => updateReasoning(event.currentTarget.value)}
				>
					{#each reasoningLevels(formState.current.values) as level}
						<option value={level}>{level}</option>
					{/each}
				</select>
			</label>
		</div>
		<textarea
			value={formState.current.values.systemPrompt}
			class="agent-prompt"
			rows="5"
			aria-label={`${title} system prompt`}
			disabled={formState.current.isSubmitting}
			oninput={(event) => {
				form.setFieldValue("systemPrompt", event.currentTarget.value);
				submitSoon(450);
			}}
		></textarea>
		{#if formState.current.errors.length > 0}
			<p class="save-msg tone-danger">{formState.current.errors.join(" ")}</p>
		{/if}
		{#if formState.current.isDirty}
			<div class="agent-form-actions">
				<Button variant="ghost" size="xs" onclick={resetForm} disabled={formState.current.isSubmitting}>
					Reset
				</Button>
				<Button variant="primary" size="xs" onclick={submitNow} disabled={!formState.current.canSubmit || formState.current.isSubmitting}>
					{formState.current.isSubmitting ? "Saving" : "Save"}
				</Button>
			</div>
		{/if}
	</div>
</article>

<style>
	.provider-row {
		display: grid;
		grid-template-columns: minmax(0, 1fr);
		align-items: center;
		gap: 0.55rem 0.75rem;
		padding: 0.42rem 0.58rem;
		border: 1px solid color-mix(in oklab, var(--ui-border-soft) 88%, transparent);
		border-radius: var(--ui-radius-sm);
		background: color-mix(in oklab, var(--ui-surface) 92%, transparent);
		box-shadow: none;
	}

	.provider-main {
		display: grid;
		grid-template-columns: minmax(0, 1fr);
		gap: 0.32rem;
		min-width: 0;
	}

	.provider-heading {
		display: flex;
		align-items: center;
		gap: 0.35rem;
		flex-wrap: wrap;
		min-width: 0;
	}

	.provider-name {
		font-size: var(--text-base);
		font-weight: 600;
		letter-spacing: 0;
	}

	.provider-meta,
	.save-msg {
		margin: 0;
		min-width: 0;
		font-size: var(--text-xs);
		color: var(--ui-text-secondary);
	}

	.save-msg.tone-danger {
		color: color-mix(in oklab, var(--ui-danger) 84%, var(--ui-text-primary));
	}

	.provider-status {
		font-size: var(--text-xs);
		font-family: var(--font-mono);
		font-variant-numeric: tabular-nums;
		color: var(--ui-text-secondary);
		border: 1px solid color-mix(in oklab, var(--ui-border-soft) 84%, transparent);
		border-radius: var(--ui-radius-sm);
		padding: 0.04rem 0.26rem;
	}

	.provider-status.tone-info {
		border-color: color-mix(in oklab, var(--ui-accent) 22%, var(--ui-border-soft));
		background: color-mix(in oklab, var(--ui-accent-soft) 68%, transparent);
	}

	.provider-status.tone-warning {
		border-color: color-mix(in oklab, var(--ui-warning) 28%, var(--ui-border-soft));
		background: color-mix(in oklab, var(--ui-warning-soft) 72%, transparent);
	}

	.model-chip,
	.agent-meta-grid strong {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-family: var(--font-mono);
		font-size: var(--text-xs);
		font-weight: 500;
		color: var(--ui-text-secondary);
	}

	.agent-meta-grid {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr));
		gap: 0.25rem 0.8rem;
		margin-top: 0.2rem;
		padding: 0.26rem 0;
	}

	.agent-meta-grid div {
		display: flex;
		align-items: center;
		gap: 0.38rem;
		min-width: 0;
	}

	.agent-meta-grid span,
	.agent-field span {
		font-size: var(--text-xs);
		color: var(--ui-text-tertiary);
	}

	.agent-grid {
		display: grid;
		grid-template-columns: minmax(0, 2fr) minmax(9rem, 1fr);
		gap: 0.45rem;
		margin-top: 0.22rem;
	}

	.agent-field {
		display: grid;
		gap: 0.28rem;
		min-width: 0;
	}

	.agent-field select {
		width: 100%;
		min-width: 0;
		border: 1px solid color-mix(in oklab, var(--ui-border-soft) 88%, transparent);
		border-radius: var(--ui-radius-sm);
		padding: 0.38rem 0.48rem;
		background: color-mix(in oklab, var(--ui-surface-subtle) 82%, transparent);
		color: var(--ui-text-primary);
		font: inherit;
		font-size: var(--text-sm);
	}

	.agent-prompt {
		width: 100%;
		min-width: 0;
		margin-top: 0.26rem;
		resize: vertical;
		border: 1px solid color-mix(in oklab, var(--ui-border-soft) 88%, transparent);
		border-radius: var(--ui-radius-sm);
		padding: 0.5rem;
		background: color-mix(in oklab, var(--ui-code) 92%, transparent);
		color: var(--ui-text-primary);
		font-family: var(--font-mono);
		font-size: var(--text-sm);
		line-height: 1.52;
	}

	.agent-form-actions {
		display: flex;
		justify-content: flex-end;
		gap: 0.36rem;
	}
</style>
