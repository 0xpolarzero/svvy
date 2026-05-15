<script lang="ts">
	import FileIcon from "@lucide/svelte/icons/file";
	import FolderIcon from "@lucide/svelte/icons/folder";
	import PaperclipIcon from "@lucide/svelte/icons/paperclip";
	import ArrowUpIcon from "@lucide/svelte/icons/arrow-up";
	import SquareIcon from "@lucide/svelte/icons/square";
	import XIcon from "@lucide/svelte/icons/x";
	import { onMount, tick } from "svelte";
	import { supportsXhigh, type Model } from "@mariozechner/pi-ai";
	import type { ThinkingLevel } from "@mariozechner/pi-agent-core";
	import { formatContextBudgetTooltip, type ContextBudget } from "../shared/context-budget";
	import {
		createPromptHistoryNavigationState,
		navigatePromptHistory,
		shouldActivatePromptHistoryNavigation,
		type PromptHistoryDirection,
		type PromptHistoryEntry,
		type PromptHistoryNavigationState,
	} from "./prompt-history";
	import {
		getActiveMentionQuery,
		removeMentionFromDraft,
		searchMentionPaths,
		selectMentionPath,
		serializeComposerDraft,
		type ComposerMentionLink,
		type MentionPickerResult,
		type WorkspacePathIndexEntry,
	} from "./composer-mentions";
	import ContextBudgetBar from "./ContextBudgetBar.svelte";
	import TextArea from "./ui/TextArea.svelte";
	import Tooltip from "./ui/Tooltip.svelte";
	import CompactSelect from "./ui/CompactSelect.svelte";
	import CompactCombobox, { type CompactComboboxOption } from "./ui/CompactCombobox.svelte";
	import { getModelComboboxValue, type ModelComboboxOption } from "./model-options";

	type Props = {
		currentModel: Model<any> | null;
		thinkingLevel: ThinkingLevel;
		isStreaming: boolean;
		promptHistory: PromptHistoryEntry[];
		errorMessage?: string;
		usageText?: string;
		contextBudget?: ContextBudget | null;
		sessionName?: string;
		targetLabel?: string;
		worktreeLabel?: string;
		onAbort: () => void;
		onOpenModelPicker: () => void;
		onListModels: () => Promise<ModelComboboxOption[]>;
		onModelChange: (model: Model<any>) => void;
		onSend: (input: string) => Promise<boolean> | boolean;
		onThinkingChange: (level: ThinkingLevel) => void;
		listWorkspacePaths: (options?: { refresh?: boolean }) => Promise<WorkspacePathIndexEntry[]>;
		pickWorkspaceAttachments: () => Promise<WorkspacePathIndexEntry[]>;
	};

	const BASE_LEVELS: ThinkingLevel[] = ["off", "minimal", "low", "medium", "high"];

	let {
		currentModel,
		thinkingLevel,
		isStreaming,
		promptHistory,
		errorMessage,
		usageText,
		contextBudget,
		sessionName = "Current session",
		targetLabel = "orchestrator",
		worktreeLabel = "worktree",
		onAbort,
		onOpenModelPicker,
		onListModels,
		onModelChange,
		onSend,
		onThinkingChange,
		listWorkspacePaths,
		pickWorkspaceAttachments,
	}: Props = $props();

	let draft = $state("");
	let isSubmitting = $state(false);
	let showThinkingMenu = $state(false);
	let showModelMenu = $state(false);
	let modelOptions = $state<CompactComboboxOption[]>([]);
	let modelOptionModels = $state(new Map<string, Model<any>>());
	let draftElement = $state<HTMLTextAreaElement | null>(null);
	let historyNavigation = $state<PromptHistoryNavigationState>(createPromptHistoryNavigationState());
	let mentionRoot = $state<HTMLDivElement | null>(null);
	let workspacePaths = $state<WorkspacePathIndexEntry[]>([]);
	let workspacePathsLoaded = $state(false);
	let mentionLoading = $state(false);
	let mentionError = $state<string | null>(null);
	let selectedMentions = $state<ComposerMentionLink[]>([]);
	let activeMentionIndex = $state(0);
	let caretPosition = $state(0);
	let dismissedMentionQueryKey = $state<string | null>(null);
	let workspacePathTargetKey = $state("");
	const availableThinkingLevels = $derived(
		currentModel && supportsXhigh(currentModel) ? [...BASE_LEVELS, "xhigh"] : BASE_LEVELS,
	);
	const thinkingOptions = $derived(
		availableThinkingLevels.map((level) => ({ value: level, label: level })),
	);
	const modelValue = $derived(currentModel ? getModelComboboxValue(currentModel) : "no-surface");
	const visibleModelOptions = $derived.by<CompactComboboxOption[]>(() => {
		if (!currentModel) return [{ value: "no-surface", label: "No surface", disabled: true }];
		const currentValue = getModelComboboxValue(currentModel);
		if (modelOptions.some((option) => option.value === currentValue)) return modelOptions;
		return [{ value: currentValue, label: currentModel.name, triggerLabel: currentModel.name }, ...modelOptions];
	});
	const mentionQuery = $derived(getActiveMentionQuery(draft, caretPosition));
	const mentionQueryKey = $derived(mentionQuery ? `${mentionQuery.start}:${mentionQuery.query}` : null);
	const activeMentionIsSelected = $derived(
		Boolean(
			mentionQuery &&
				selectedMentions.some(
					(mention) =>
						mention.workspaceRelativePath === mentionQuery.query &&
						draft.slice(mentionQuery.start, mentionQuery.end) === `@${mention.workspaceRelativePath}`,
				),
		),
	);
	const mentionResults = $derived<MentionPickerResult[]>(
		mentionQuery && workspacePathsLoaded ? searchMentionPaths(workspacePaths, mentionQuery.query, 10) : [],
	);
	const canSubmit = $derived(Boolean(draft.trim() || selectedMentions.length > 0));
	const contextBudgetTooltip = $derived(
		contextBudget
			? [formatContextBudgetTooltip(contextBudget), usageText ? `usage ${usageText}` : ""]
					.filter(Boolean)
					.join(" · ")
			: "Context unavailable",
	);
	const showMentionPicker = $derived(
		Boolean(
			mentionQuery &&
				!activeMentionIsSelected &&
				mentionQueryKey !== dismissedMentionQueryKey &&
				(mentionLoading || mentionError || workspacePathsLoaded || mentionResults.length > 0),
		),
	);

	$effect(() => {
		const targetKey = `${sessionName}\u0000${targetLabel}\u0000${worktreeLabel}`;
		if (targetKey !== workspacePathTargetKey) {
			workspacePathTargetKey = targetKey;
			workspacePaths = [];
			workspacePathsLoaded = false;
			mentionLoading = false;
			mentionError = null;
			activeMentionIndex = 0;
			dismissedMentionQueryKey = null;
		}
		void tick().then(() => {
			draftElement?.focus();
		});
	});

	$effect(() => {
		void draft;
		void tick().then(syncDraftTextareaHeight);
	});

	$effect(() => {
		if (mentionResults.length === 0) {
			activeMentionIndex = 0;
			return;
		}
		if (activeMentionIndex >= mentionResults.length) {
			activeMentionIndex = mentionResults.length - 1;
		}
	});

	onMount(() => {
		syncDraftTextareaHeight();

		const handlePointerDown = (event: PointerEvent) => {
			const target = event.target;
			if (!(target instanceof Node)) return;
			if (mentionRoot?.contains(target) || draftElement?.contains(target)) return;
			closeMentionPicker();
		};

		const handleKeyDown = (event: KeyboardEvent) => {
			if (event.key === "Escape") {
				closeMentionPicker();
			}
		};

		window.addEventListener("pointerdown", handlePointerDown);
		window.addEventListener("keydown", handleKeyDown);

		return () => {
			window.removeEventListener("pointerdown", handlePointerDown);
			window.removeEventListener("keydown", handleKeyDown);
		};
	});

	async function restoreDraftBuffer(nextDraft: string) {
		if (draft !== "") return;
		draft = nextDraft;
		await tick();
		moveCaretToDraftEnd(nextDraft);
	}

	function resetHistoryNavigation() {
		historyNavigation = createPromptHistoryNavigationState();
	}

	function moveCaretToDraftEnd(value: string) {
		draftElement?.focus();
		draftElement?.setSelectionRange(value.length, value.length);
		caretPosition = value.length;
		syncDraftTextareaHeight();
	}

	function syncCaretFromTextarea(target: EventTarget | null) {
		if (!(target instanceof HTMLTextAreaElement)) return;
		caretPosition = target.selectionStart;
		if (getActiveMentionQuery(target.value, target.selectionStart, target.selectionEnd)) {
			void ensureWorkspacePaths();
		}
	}

	function syncDraftTextareaHeight() {
		if (!draftElement) return;

		draftElement.style.height = "auto";
		const maxHeight = Number.parseFloat(getComputedStyle(draftElement).maxHeight);
		const resolvedMaxHeight = Number.isFinite(maxHeight) ? maxHeight : Number.POSITIVE_INFINITY;
		const nextHeight = Math.min(draftElement.scrollHeight, resolvedMaxHeight);
		draftElement.style.height = `${nextHeight}px`;
		draftElement.style.overflowY = draftElement.scrollHeight > resolvedMaxHeight ? "auto" : "hidden";
	}

	function handleDraftInput(event: Event) {
		syncCaretFromTextarea(event.currentTarget);
		syncDraftTextareaHeight();
	}

	async function scrollActiveMentionIntoView() {
		await tick();
		const activeOption = mentionRoot?.querySelector<HTMLElement>(".mention-option.active");
		activeOption?.scrollIntoView({ block: "nearest" });
	}

	async function ensureWorkspacePaths() {
		if (workspacePathsLoaded || mentionLoading) return;
		mentionLoading = true;
		mentionError = null;
		try {
			workspacePaths = await listWorkspacePaths({ refresh: true });
			workspacePathsLoaded = true;
		} catch (error) {
			mentionError = error instanceof Error ? error.message : "Workspace paths unavailable.";
		} finally {
			mentionLoading = false;
		}
	}

	function closeMentionPicker() {
		activeMentionIndex = 0;
		dismissedMentionQueryKey = mentionQueryKey;
	}

	async function chooseMention(result: MentionPickerResult) {
		if (!mentionQuery) return;
		const selection = selectMentionPath(draft, mentionQuery, result);
		draft = selection.draft;
		activeMentionIndex = 0;
		dismissedMentionQueryKey = `${mentionQuery.start}:${result.workspaceRelativePath}`;
		await tick();
		draftElement?.focus();
		draftElement?.setSelectionRange(selection.caret, selection.caret);
		caretPosition = selection.caret;
	}

	async function removeMention(mention: ComposerMentionLink) {
		draft = removeMentionFromDraft(draft, mention);
		selectedMentions = selectedMentions.filter((item) => item.id !== mention.id);
		await tick();
		moveCaretToDraftEnd(draft);
	}

	async function applyPromptHistoryNavigation(direction: PromptHistoryDirection) {
		const navigation = navigatePromptHistory(promptHistory, historyNavigation, draft, direction);
		if (!navigation.changed) return;

		historyNavigation = navigation.nextState;
		draft = navigation.nextDraft;
		await tick();
		moveCaretToDraftEnd(navigation.nextDraft);
	}

	async function submit() {
		if (!canSubmit || isStreaming || isSubmitting) return;
		const nextDraft = serializeComposerDraft(draft, selectedMentions);
		const nextVisibleDraft = draft;
		const nextVisibleMentions = selectedMentions;
		draft = "";
		selectedMentions = [];
		isSubmitting = true;

		try {
			const sent = await onSend(nextDraft);
			if (sent) {
				resetHistoryNavigation();
			} else {
				await restoreDraftBuffer(nextVisibleDraft);
				selectedMentions = nextVisibleMentions;
			}
		} catch {
			await restoreDraftBuffer(nextVisibleDraft);
			selectedMentions = nextVisibleMentions;
		} finally {
			isSubmitting = false;
		}
	}

	function handleKeydown(event: KeyboardEvent) {
		const target = event.currentTarget;
		if (showMentionPicker && mentionQuery) {
			if (event.key === "ArrowDown" || event.key === "ArrowUp") {
				event.preventDefault();
				const direction = event.key === "ArrowDown" ? 1 : -1;
				activeMentionIndex =
					(mentionResults.length + activeMentionIndex + direction) % Math.max(mentionResults.length, 1);
				void scrollActiveMentionIntoView();
				return;
			}
			if ((event.key === "Enter" || event.key === "Tab") && mentionResults[activeMentionIndex]) {
				event.preventDefault();
				void chooseMention(mentionResults[activeMentionIndex]);
				return;
			}
			if (event.key === "Escape") {
				event.preventDefault();
				closeMentionPicker();
				return;
			}
		}

		if (
			target instanceof HTMLTextAreaElement &&
			!event.shiftKey &&
			!event.metaKey &&
			!event.ctrlKey &&
			!event.altKey &&
			(event.key === "ArrowUp" || event.key === "ArrowDown")
		) {
			const direction: PromptHistoryDirection = event.key === "ArrowUp" ? "older" : "newer";
			const shouldNavigateHistory = shouldActivatePromptHistoryNavigation({
				direction,
				value: target.value,
				selectionStart: target.selectionStart,
				selectionEnd: target.selectionEnd,
				higherPriorityUiActive: showThinkingMenu || showMentionPicker,
			});

			if (shouldNavigateHistory) {
				event.preventDefault();
				void applyPromptHistoryNavigation(direction);
				return;
			}
		}

		if (event.key !== "Enter" || event.shiftKey || event.metaKey || event.ctrlKey || event.altKey) return;
		event.preventDefault();
		void submit();
	}

	async function attachPickedWorkspaceFiles() {
		try {
			const entries = await pickWorkspaceAttachments();
			if (entries.length === 0) {
				draftElement?.focus();
				return;
			}

			const nextMentions = new Map(selectedMentions.map((mention) => [mention.id, mention]));
			for (const entry of entries) {
				nextMentions.set(`${entry.kind}:${entry.workspaceRelativePath}`, {
					id: `${entry.kind}:${entry.workspaceRelativePath}`,
					kind: entry.kind,
					label: entry.workspaceRelativePath.split("/").filter(Boolean).at(-1) ?? entry.workspaceRelativePath,
					workspaceRelativePath: entry.workspaceRelativePath,
				});
			}
			selectedMentions = [...nextMentions.values()];
			await tick();
			draftElement?.focus();
		} catch {
			draftElement?.focus();
		}
	}

	async function loadModelOptions() {
		const options = await onListModels();
		modelOptions = options;
		modelOptionModels = new Map(options.map((option) => [option.value, option.model]));
	}

	function selectModel(value: string) {
		const model = modelOptionModels.get(value);
		if (!model) return;
		onModelChange(model);
	}
</script>

<div class="composer-shell">
	<div class="composer-frame expanded">
		{#if errorMessage}
			<p class="composer-error">{errorMessage}</p>
		{/if}

		{#if showMentionPicker}
			<div bind:this={mentionRoot} class="mention-picker" role="listbox" aria-label="Workspace paths">
				{#if mentionLoading}
					<div class="mention-empty">Indexing workspace paths...</div>
				{:else if mentionError}
					<div class="mention-empty danger">{mentionError}</div>
				{:else if mentionResults.length === 0}
					<div class="mention-empty">No indexed file or folder matches @{mentionQuery?.query}</div>
				{:else}
					{#each mentionResults as result, index (result.id)}
						<button
							class={`mention-option ${index === activeMentionIndex ? "active" : ""}`.trim()}
							type="button"
							role="option"
							aria-selected={index === activeMentionIndex}
							onmousedown={(event) => event.preventDefault()}
							onclick={() => void chooseMention(result)}
						>
							{#if result.kind === "folder"}
								<FolderIcon size={14} aria-hidden="true" />
							{:else}
								<FileIcon size={14} aria-hidden="true" />
							{/if}
							<span>{result.basename}</span>
							<small>{result.disambiguation || result.workspaceRelativePath}</small>
						</button>
					{/each}
				{/if}
			</div>
		{/if}

		<div class="composer-main-row">
			<div class="composer-input-wrap">
				{#if selectedMentions.length > 0}
					<section class="composer-context-row" aria-label="Attached file and context items">
						<div class="mention-chip-row">
							{#each selectedMentions as mention (mention.id)}
								<Tooltip label={`Remove ${mention.workspaceRelativePath}`}>
									<button
										class="mention-chip"
										type="button"
										aria-label={`Remove attached context ${mention.workspaceRelativePath}`}
										onclick={() => void removeMention(mention)}
									>
										{#if mention.kind === "folder"}
											<FolderIcon size={12} aria-hidden="true" />
										{:else}
											<FileIcon size={12} aria-hidden="true" />
										{/if}
										<span>{mention.workspaceRelativePath}</span>
										<XIcon size={11} aria-hidden="true" />
									</button>
								</Tooltip>
							{/each}
						</div>
					</section>
				{/if}
				<TextArea
					bind:value={draft}
					bind:element={draftElement}
					resize="vertical"
					rows={1}
					placeholder="Ask svvy to inspect the repo, make a change, or run Project CI."
					onkeydown={handleKeydown}
					oninput={handleDraftInput}
					onkeyup={(event) => syncCaretFromTextarea(event.currentTarget)}
					onclick={(event) => syncCaretFromTextarea(event.currentTarget)}
					onselect={(event) => syncCaretFromTextarea(event.currentTarget)}
				/>

				<div class="composer-row-actions">
					<div class="composer-control-cluster" aria-label="Runtime controls">
						<CompactCombobox
							bind:open={showModelMenu}
							value={modelValue}
							options={visibleModelOptions}
							ariaLabel="Change model"
							placeholder="Search models"
							emptyLabel="No models match."
							disabled={!currentModel}
							triggerClass="model-pill model-control"
							menuClass="model-menu"
							optionClass="model-option"
							onBeforeOpen={loadModelOptions}
							onSelect={selectModel}
						/>
						<CompactSelect
							bind:open={showThinkingMenu}
							value={thinkingLevel}
							options={thinkingOptions}
							ariaLabel="Thinking level"
							triggerClass="ghost-select thinking-field"
							textTransform="lowercase"
							onSelect={(level) => onThinkingChange(level as ThinkingLevel)}
						/>
						<Tooltip label={contextBudgetTooltip} disabled={!contextBudget} delayMs={250}>
							<div class="compact-budget">
								<ContextBudgetBar budget={contextBudget ?? null} variant="compact" label="Context" />
							</div>
						</Tooltip>
					</div>
					<div class="composer-action-cluster" aria-label="Composer actions">
						<Tooltip label="Attach file context">
							<button
								class="composer-icon-button"
								type="button"
								aria-label="Attach file context"
								onclick={() => void attachPickedWorkspaceFiles()}
							>
								<PaperclipIcon size={15} aria-hidden="true" />
							</button>
						</Tooltip>
						{#if isStreaming}
							<Tooltip label="Stop generation">
								<button class="composer-submit danger" type="button" aria-label="Stop" onclick={onAbort}>
									<SquareIcon size={13} aria-hidden="true" />
								</button>
							</Tooltip>
						{:else}
							<Tooltip label="Send message" disabled={!currentModel || !canSubmit || isSubmitting}>
								<button
									class="composer-submit"
									type="button"
									aria-label="Send"
									onclick={() => void submit()}
									disabled={!currentModel || !canSubmit || isSubmitting}
								>
									<ArrowUpIcon size={15} aria-hidden="true" />
								</button>
							</Tooltip>
						{/if}
					</div>
				</div>
			</div>
		</div>

		{#if isStreaming}
			<div class="composer-streaming-row">
				<span class="streaming-dot pulse-dot"></span>
				<span>{targetLabel} is working...</span>
				{#if contextBudget}
					<strong>{contextBudget.label} context used</strong>
				{/if}
			</div>
		{/if}
	</div>
</div>

<style>
	.composer-shell {
		container-type: inline-size;
		padding: 0;
		background: transparent;
	}

	.composer-frame {
		display: grid;
		gap: 0;
		transition: background-color 160ms cubic-bezier(0.19, 1, 0.22, 1);
	}

	.composer-context-row,
	.composer-streaming-row {
		border-top: 1px solid var(--ui-border-soft);
	}

	.composer-context-row {
		padding: 0.42rem 0.52rem 0.38rem;
		border-top: 0;
		border-bottom: 1px solid color-mix(in oklab, var(--ui-border-soft) 70%, transparent);
	}

	.thinking-wrap {
		position: relative;
	}

	.mention-chip-row {
		display: flex;
		align-items: center;
		gap: 0.38rem;
		flex-wrap: wrap;
		min-width: 0;
	}

	.mention-chip {
		display: inline-flex;
		align-items: center;
		gap: 0.3rem;
		max-width: min(100%, 18rem);
		min-height: 1.32rem;
		padding: 0.12rem 0.36rem;
		border: 1px solid color-mix(in oklab, var(--ui-border-accent) 58%, var(--ui-border-soft));
		border-radius: var(--ui-radius-sm);
		background: color-mix(in oklab, var(--ui-accent-soft) 54%, var(--ui-surface));
		color: color-mix(in oklab, var(--ui-accent) 78%, var(--ui-text-primary));
		font: inherit;
		font-family: var(--font-mono);
		font-size: 0.62rem;
		cursor: pointer;
	}

	.mention-chip span {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-weight: 600;
	}

	.mention-chip:hover,
	.mention-chip:focus-visible {
		outline: none;
		border-color: color-mix(in oklab, var(--ui-accent) 62%, var(--ui-border-strong));
		background: color-mix(in oklab, var(--ui-accent-soft) 54%, var(--ui-surface-raised));
		color: var(--ui-text-primary);
	}

	.composer-main-row {
		display: grid;
		grid-template-columns: minmax(0, 1fr);
		align-items: start;
		width: min(100%, 45.5rem);
		margin: 0 auto;
		min-height: 0;
		padding: 0.58rem 0.72rem 0.48rem;
	}

	.composer-input-wrap {
		display: flex;
		flex-direction: column;
		min-width: 0;
		min-height: 0;
		overflow: visible;
		border: 1px solid color-mix(in oklab, var(--ui-border-soft) 72%, transparent);
		border-radius: var(--ui-radius-md);
		background: color-mix(in oklab, var(--ui-surface-subtle) 38%, transparent);
		transition:
			border-color 150ms cubic-bezier(0.19, 1, 0.22, 1),
			background-color 150ms cubic-bezier(0.19, 1, 0.22, 1),
			box-shadow 150ms cubic-bezier(0.19, 1, 0.22, 1);
	}

	.composer-input-wrap:hover {
		border-color: color-mix(in oklab, var(--ui-border-strong) 70%, var(--ui-border-soft));
		background: color-mix(in oklab, var(--ui-surface-subtle) 54%, transparent);
	}

	.composer-input-wrap:focus-within {
		border-color: color-mix(in oklab, var(--ui-border-strong) 82%, var(--ui-accent));
		background: color-mix(in oklab, var(--ui-surface) 86%, var(--ui-surface-subtle));
		box-shadow: inset 0 0 0 1px color-mix(in oklab, var(--ui-accent) 14%, transparent);
	}

	:global(.composer-shell .ui-textarea) {
		flex: 1 1 auto;
		min-height: 2.35rem;
		max-height: 10.5rem;
		padding: 0.46rem 0.52rem 0.38rem;
		border: 0;
		border-radius: 0;
		background: transparent;
		box-shadow: none;
		color: var(--ui-text-primary);
		font-size: 0.81rem;
		line-height: 1.45;
		resize: none;
	}

	:global(.composer-shell .ui-textarea:hover),
	:global(.composer-shell .ui-textarea:focus-visible) {
		border-color: transparent;
		background: transparent;
		box-shadow: none;
	}

	.composer-row-actions,
	.composer-control-cluster,
	.composer-action-cluster {
		display: flex;
		align-items: center;
		min-width: 0;
	}

	.composer-row-actions {
		margin-top: 0;
		padding: 0.28rem 0.32rem 0.32rem 0.44rem;
		border-radius: 0 0 var(--ui-radius-md) var(--ui-radius-md);
		background: transparent;
		justify-content: space-between;
		align-content: center;
		gap: 0.4rem;
		flex-wrap: wrap;
		width: 100%;
	}

	.composer-control-cluster,
	.composer-action-cluster {
		flex: 0 0 auto;
		gap: 0.36rem;
		padding: 0;
		border: 0;
		border-radius: 0;
		background: transparent;
	}

	.composer-control-cluster {
		max-width: 18rem;
	}

	.composer-action-cluster {
		margin-left: auto;
	}

	.composer-icon-button,
	.composer-submit {
		position: relative;
		display: inline-grid;
		place-items: center;
		flex: 0 0 auto;
		width: 1.9rem;
		height: 1.9rem;
		border: 1px solid transparent;
		border-radius: var(--ui-radius-md);
		background: transparent;
		color: var(--ui-text-tertiary);
		cursor: pointer;
		transition:
			border-color 150ms cubic-bezier(0.19, 1, 0.22, 1),
			background-color 150ms cubic-bezier(0.19, 1, 0.22, 1),
			color 150ms cubic-bezier(0.19, 1, 0.22, 1);
	}

	.composer-icon-button:hover,
	.composer-icon-button:focus-visible {
		outline: none;
		border-color: var(--ui-border-soft);
		background: var(--ui-surface-subtle);
		color: var(--ui-text-primary);
	}

	.composer-submit {
		border-color: color-mix(in oklab, var(--ui-accent) 68%, var(--ui-border-accent));
		background: var(--ui-accent);
		color: var(--ui-accent-ink);
	}

	.composer-submit:hover,
	.composer-submit:focus-visible {
		outline: none;
		background: var(--ui-accent-strong);
	}

	.composer-submit:disabled {
		border-color: var(--ui-border-soft);
		background: var(--ui-surface-muted);
		color: var(--ui-text-tertiary);
		cursor: not-allowed;
	}

	.composer-submit.danger {
		border-color: color-mix(in oklab, var(--ui-danger) 42%, var(--ui-border-soft));
		background: color-mix(in oklab, var(--ui-danger-soft) 82%, var(--ui-surface));
		color: var(--ui-danger);
	}

	.model-pill {
		display: inline-flex;
		align-items: center;
		gap: 0.28rem;
		min-width: 0;
		max-width: 12.5rem;
		border: 1px solid transparent;
		border-radius: var(--ui-radius-md);
		background: transparent;
		color: var(--ui-text-tertiary);
		font-family: var(--font-mono);
		font-size: 0.62rem;
		line-height: 1;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.model-pill {
		position: relative;
		min-height: 1.45rem;
		padding: 0.08rem 0.44rem;
		cursor: pointer;
		transition:
			border-color 150ms cubic-bezier(0.19, 1, 0.22, 1),
			background-color 150ms cubic-bezier(0.19, 1, 0.22, 1),
			color 150ms cubic-bezier(0.19, 1, 0.22, 1);
	}

	.model-pill strong {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: 0.62rem;
		font-weight: 500;
	}

	.model-pill.thinking-field {
		max-width: 7rem;
		padding-right: 1.62rem;
	}

	.model-control {
		max-width: 12.5rem;
	}

	.model-pill:hover,
	.model-pill:focus-visible {
		outline: none;
		border-color: var(--ui-border-soft);
		background: var(--ui-surface-subtle);
		color: var(--ui-text-primary);
	}

	.model-pill:disabled {
		border-color: var(--ui-border-soft);
		background: var(--ui-surface-muted);
		color: var(--ui-text-tertiary);
		cursor: not-allowed;
	}

	.model-pill:focus-visible,
	.composer-icon-button:focus-visible,
	.composer-submit:focus-visible,
	.mention-chip:focus-visible {
		box-shadow: var(--ui-focus-ring);
	}

	.compact-budget {
		position: relative;
		display: flex;
		align-items: center;
		width: 5.8rem;
		height: 1.45rem;
		flex: 0 0 5.8rem;
		margin-left: 0.64rem;
	}

	.compact-budget :global(.context-budget-compact) {
		position: static;
		width: 100%;
	}

	.compact-budget :global(.context-budget-track) {
		transform: translateY(0.08rem);
	}

	.mention-picker {
		display: grid;
		gap: 0.12rem;
		width: min(100%, 45.5rem);
		max-height: 15rem;
		overflow: auto;
		margin: 0 auto 0.35rem;
		padding: 0.24rem;
		border: 1px solid color-mix(in oklab, var(--ui-border-soft) 92%, transparent);
		border-radius: var(--ui-radius-md);
		background: var(--ui-surface-raised);
		box-shadow: var(--ui-shadow-strong);
	}

	.mention-option {
		display: grid;
		grid-template-columns: 1rem minmax(4rem, max-content) minmax(0, 1fr);
		align-items: center;
		gap: 0.5rem;
		min-height: 1.9rem;
		padding: 0.32rem 0.44rem;
		border: 1px solid transparent;
		border-radius: var(--ui-radius-sm);
		background: transparent;
		color: var(--ui-text-primary);
		font: inherit;
		text-align: left;
		cursor: pointer;
	}

	.mention-option span,
	.mention-option small {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.mention-option span {
		font-size: 0.73rem;
		font-weight: 650;
	}

	.mention-option small {
		font-family: var(--font-mono);
		font-size: 0.63rem;
		color: var(--ui-text-secondary);
	}

	.mention-option:hover,
	.mention-option:focus-visible,
	.mention-option.active {
		outline: none;
		border-color: color-mix(in oklab, var(--ui-border-accent) 70%, var(--ui-border-soft));
		background: color-mix(in oklab, var(--ui-accent-soft) 62%, var(--ui-surface-raised));
	}

	.mention-empty {
		padding: 0.55rem 0.6rem;
		font-size: 0.72rem;
		color: var(--ui-text-secondary);
	}

	.mention-empty.danger {
		color: var(--ui-danger);
	}

	.thinking-field {
		width: auto;
	}

	.composer-error {
		margin: 0;
		padding: 0.55rem 0.72rem;
		border: 1px solid color-mix(in oklab, var(--ui-danger) 22%, var(--ui-border-soft));
		border-width: 0 0 1px;
		border-radius: 0;
		background: color-mix(in oklab, var(--ui-danger-soft) 74%, transparent);
		color: color-mix(in oklab, var(--ui-danger) 82%, var(--ui-text-primary));
		font-size: 0.78rem;
		line-height: 1.5;
	}

	.composer-streaming-row {
		display: flex;
		align-items: center;
		gap: 0.45rem;
		min-width: 0;
		padding: 0.36rem 0.72rem;
		color: var(--ui-text-tertiary);
		font-family: var(--font-mono);
		font-size: 0.6rem;
	}

	.composer-streaming-row {
		color: var(--ui-text-secondary);
	}

	.streaming-dot {
		width: 0.42rem;
		height: 0.42rem;
		border-radius: 999px;
		background: var(--ui-accent);
	}

	.composer-streaming-row strong {
		color: var(--ui-text-tertiary);
		font-weight: 500;
	}

	@media (max-width: 760px) {
		.composer-main-row {
			grid-template-columns: minmax(0, 1fr);
			padding: 0.5rem;
		}

		.composer-icon-button,
		.composer-submit,
		.model-pill,
		.mention-option {
			min-height: 2.75rem;
		}

		.composer-icon-button,
		.composer-submit {
			width: 2.75rem;
			height: 2.75rem;
		}

		.model-pill,
		.compact-budget {
			display: none;
		}

		.composer-control-cluster {
			display: none;
		}

		.composer-row-actions {
			align-items: flex-start;
			flex-direction: column;
			justify-content: flex-start;
		}
	}

	@container (max-width: 420px) {
		.composer-main-row {
			grid-template-columns: 1fr;
			min-height: 6.1rem;
			padding: 0.48rem 0.56rem 0.42rem;
		}

		:global(.composer-shell .ui-textarea) {
			min-height: 4.2rem;
			font-size: 0.78rem;
		}

		.composer-row-actions {
			justify-content: flex-start;
			gap: 0.3rem;
		}

		.model-pill,
		.compact-budget {
			display: none;
		}

		.composer-control-cluster {
			display: none;
		}

	}
</style>
