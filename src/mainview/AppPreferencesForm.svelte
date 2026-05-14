<script lang="ts">
	import { createForm } from "@tanstack/svelte-form";
	import type { AppPreferences, PreferredExternalEditor } from "../shared/agent-settings";
	import Button from "./ui/Button.svelte";

	type Props = {
		preferences: AppPreferences;
		onSave: (preferences: AppPreferences) => Promise<AppPreferences | void>;
	};

	let { preferences, onSave }: Props = $props();
	let saveMessage = $state("");

	const form = createForm(() => ({
		defaultValues: structuredClone(preferences),
		validators: {
			onChange: ({ value }) => {
				if (value.preferredExternalEditor === "custom" && !value.customExternalEditorCommand.trim()) {
					return "Custom editor command is required.";
				}
				return undefined;
			},
		},
		onSubmit: async ({ value, formApi }) => {
			saveMessage = "Saving";
			const saved = await onSave(structuredClone(value));
			formApi.reset(structuredClone(saved ?? value));
			saveMessage = "Saved";
			setTimeout(() => {
				if (saveMessage === "Saved") saveMessage = "";
			}, 1800);
		},
	}));

	const formState = form.useStore();

	function submit() {
		void form.handleSubmit().catch((error) => {
			saveMessage = error instanceof Error ? error.message : "Save failed";
		});
	}
</script>

<article class="provider-row agent-row">
	<div class="provider-main">
		<div class="provider-heading">
			<span class="provider-name">External Editor</span>
			<span class="provider-status tone-info">
				{formState.current.values.preferredExternalEditor}
			</span>
			{#if saveMessage}
				<span class="provider-status">{saveMessage}</span>
			{/if}
			{#if formState.current.isDirty}
				<span class="provider-status tone-warning">Dirty</span>
			{/if}
		</div>
		<p class="provider-meta">
			Workflow source opens in this editor from read-only library and artifact surfaces.
		</p>
		<div class="agent-grid">
			<label class="agent-field">
				<span>Editor</span>
				<select
					value={formState.current.values.preferredExternalEditor}
					disabled={formState.current.isSubmitting}
					onchange={(event) => {
						form.setFieldValue("preferredExternalEditor", event.currentTarget.value as PreferredExternalEditor);
						submit();
					}}
				>
					<option value="system">System default</option>
					<option value="code">Visual Studio Code</option>
					<option value="cursor">Cursor</option>
					<option value="zed">Zed</option>
					<option value="sublime">Sublime Text</option>
					<option value="custom">Custom command</option>
				</select>
			</label>
			<label class="agent-field">
				<span>Custom command</span>
				<input
					value={formState.current.values.customExternalEditorCommand}
					placeholder="editor-command --reuse-window"
					disabled={formState.current.values.preferredExternalEditor !== "custom" || formState.current.isSubmitting}
					oninput={(event) => form.setFieldValue("customExternalEditorCommand", event.currentTarget.value)}
					onchange={submit}
				/>
			</label>
		</div>
		{#if formState.current.errors.length > 0}
			<p class="save-msg tone-danger">{formState.current.errors.join(" ")}</p>
		{/if}
		{#if formState.current.isDirty}
			<div class="agent-form-actions">
				<Button variant="ghost" size="xs" onclick={() => form.reset()} disabled={formState.current.isSubmitting}>
					Reset
				</Button>
				<Button variant="primary" size="xs" onclick={submit} disabled={!formState.current.canSubmit || formState.current.isSubmitting}>
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
		padding: 0.42rem 0.58rem;
		border: 1px solid color-mix(in oklab, var(--ui-border-soft) 88%, transparent);
		border-radius: var(--ui-radius-sm);
		background: color-mix(in oklab, var(--ui-surface) 92%, transparent);
	}

	.provider-main {
		display: grid;
		gap: 0.32rem;
		min-width: 0;
	}

	.provider-heading {
		display: flex;
		align-items: center;
		gap: 0.35rem;
		flex-wrap: wrap;
	}

	.provider-name {
		font-size: 0.78rem;
		font-weight: 660;
	}

	.provider-meta,
	.save-msg {
		margin: 0;
		font-size: 0.68rem;
		color: var(--ui-text-secondary);
	}

	.save-msg.tone-danger {
		color: color-mix(in oklab, var(--ui-danger) 84%, var(--ui-text-primary));
	}

	.provider-status {
		font-size: 0.64rem;
		font-family: var(--font-mono);
		border: 1px solid color-mix(in oklab, var(--ui-border-soft) 84%, transparent);
		border-radius: var(--ui-radius-sm);
		padding: 0.04rem 0.26rem;
		color: var(--ui-text-secondary);
	}

	.provider-status.tone-info {
		border-color: color-mix(in oklab, var(--ui-accent) 22%, var(--ui-border-soft));
		background: color-mix(in oklab, var(--ui-accent-soft) 68%, transparent);
	}

	.provider-status.tone-warning {
		border-color: color-mix(in oklab, var(--ui-warning) 28%, var(--ui-border-soft));
		background: color-mix(in oklab, var(--ui-warning-soft) 72%, transparent);
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

	.agent-field span {
		font-size: 0.68rem;
		font-family: var(--font-mono);
		color: var(--ui-text-secondary);
	}

	.agent-field select,
	.agent-field input {
		width: 100%;
		min-width: 0;
		border: 1px solid color-mix(in oklab, var(--ui-border-soft) 88%, transparent);
		border-radius: var(--ui-radius-sm);
		padding: 0.38rem 0.48rem;
		background: color-mix(in oklab, var(--ui-surface-subtle) 82%, transparent);
		color: var(--ui-text-primary);
		font: inherit;
		font-size: 0.72rem;
	}

	.agent-form-actions {
		display: flex;
		justify-content: flex-end;
		gap: 0.36rem;
	}
</style>
