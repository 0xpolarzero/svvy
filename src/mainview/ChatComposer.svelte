<script lang="ts">
	import FileIcon from "@lucide/svelte/icons/file";
	import FolderIcon from "@lucide/svelte/icons/folder";
	import ImageIcon from "@lucide/svelte/icons/image";
	import PaperclipIcon from "@lucide/svelte/icons/paperclip";
	import TriangleAlertIcon from "@lucide/svelte/icons/triangle-alert";
	import ArrowUpIcon from "@lucide/svelte/icons/arrow-up";
	import ClockIcon from "@lucide/svelte/icons/clock";
	import XIcon from "@lucide/svelte/icons/x";
	import { onMount, tick } from "svelte";
	import type { Model } from "@mariozechner/pi-ai";
	import type { ThinkingLevel } from "@mariozechner/pi-agent-core";
	import type { ContextBudget } from "../shared/context-budget";
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
		searchMentionPaths,
		selectMentionPath,
		serializeComposerDraft,
		type MentionPickerResult,
		type WorkspacePathIndexEntry,
	} from "./composer-mentions";
	import ContextBudgetBar from "./ContextBudgetBar.svelte";
	import TextArea from "./ui/TextArea.svelte";
	import Tooltip from "./ui/Tooltip.svelte";
	import CompactSelect from "./ui/CompactSelect.svelte";
	import CompactCombobox, { type CompactComboboxOption } from "./ui/CompactCombobox.svelte";
	import { getModelComboboxValue, type ModelComboboxOption } from "./model-options";
	import { getSupportedThinkingLevels } from "./model-thinking";
	import { formatWorkingElapsed, formatWorkingElapsedTooltip } from "./working-timer";
	import QueuedMessagesStrip from "./QueuedMessagesStrip.svelte";
	import type { QueuedPrompt } from "./chat-runtime";
	import type { ComposerAttachment, ComposerDraft } from "../shared/workspace-contract";

	export type ComposerSubmit = {
		text: string;
		attachments: ComposerAttachment[];
		editMessageTimestamp?: string | number;
	};

	export type ComposerEditDraft = {
		messageTimestamp: string | number;
		text: string;
	};

	type Props = {
		currentModel: Model<any> | null;
		thinkingLevel: ThinkingLevel;
		isStreaming: boolean;
		activeTurnStartedAt: string | null;
		promptHistory: PromptHistoryEntry[];
		errorMessage?: string;
		contextBudget?: ContextBudget | null;
		queuedMessages?: QueuedPrompt[];
		composerDraft?: ComposerDraft;
		draftStorageKey?: string;
		editDraft?: ComposerEditDraft | null;
		sessionName?: string;
		targetLabel?: string;
		worktreeLabel?: string;
		onOpenModelPicker: () => void;
		onListModels: () => Promise<ModelComboboxOption[]>;
		onModelChange: (model: Model<any>) => void;
		onSend: (input: ComposerSubmit) => Promise<boolean> | boolean;
		onDraftChange?: (draft: { text: string; attachments: ComposerAttachment[] }) => void;
		onBufferChange?: (draft: { text: string; attachments: ComposerAttachment[] }) => void;
		onEditQueuedMessage?: (promptId: string) => Promise<string | null> | string | null;
		onDeleteQueuedMessage?: (promptId: string) => void;
		onSteerQueuedMessage?: (promptId: string) => void;
		onReorderQueuedMessage?: (promptId: string, beforePromptId: string | null) => void;
		onCancelEditMessage?: () => void;
		onThinkingChange: (level: ThinkingLevel) => void;
		listWorkspacePaths: (options?: { refresh?: boolean }) => Promise<WorkspacePathIndexEntry[]>;
		pickWorkspaceAttachments: () => Promise<ComposerAttachment[]>;
		importComposerAttachments: (files: File[]) => Promise<ComposerAttachment[]>;
	};

	let {
		currentModel,
		thinkingLevel,
		isStreaming,
		activeTurnStartedAt,
		promptHistory,
		errorMessage,
		contextBudget,
		queuedMessages = [],
		composerDraft = { text: "", attachments: [], updatedAt: null },
		draftStorageKey = "composer",
		editDraft = null,
		sessionName = "Current session",
		targetLabel = "orchestrator",
		worktreeLabel = "worktree",
		onOpenModelPicker,
		onListModels,
		onModelChange,
		onSend,
		onDraftChange = () => {},
		onBufferChange = () => {},
		onEditQueuedMessage = () => {},
		onDeleteQueuedMessage = () => {},
		onSteerQueuedMessage = () => {},
		onReorderQueuedMessage = () => {},
		onCancelEditMessage = () => {},
		onThinkingChange,
		listWorkspacePaths,
		pickWorkspaceAttachments,
		importComposerAttachments,
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
	let attachments = $state<ComposerAttachment[]>([]);
	let isDragActive = $state(false);
	let workingTimerNow = $state(Date.now());
	let activeMentionIndex = $state(0);
	let caretPosition = $state(0);
	let dismissedMentionQueryKey = $state<string | null>(null);
	let workspacePathTargetKey = $state("");
	let loadedEditDraftKey = $state<string | null>(null);
	let loadedComposerDraftKey = $state<string | null>(null);
	let lastPersistedDraftPayloadKey = $state<string | null>(null);
	let draftPersistenceReady = $state(false);
	const availableThinkingLevels = $derived(getSupportedThinkingLevels(currentModel));
	const thinkingOptions = $derived(
		availableThinkingLevels.map((level) => ({ value: level, label: level })),
	);
	const modelValue = $derived(currentModel ? getModelComboboxValue(currentModel) : "no-surface");

	function cloneComposerAttachments(input: readonly ComposerAttachment[]): ComposerAttachment[] {
		return input.map((attachment) => ({ ...attachment }));
	}
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
				draft.slice(mentionQuery.start, mentionQuery.end) === `@${mentionQuery.query}` &&
				workspacePaths.some((entry) => entry.workspaceRelativePath === mentionQuery.query),
		),
	);
	const mentionResults = $derived<MentionPickerResult[]>(
		mentionQuery && workspacePathsLoaded ? searchMentionPaths(workspacePaths, mentionQuery.query, 10) : [],
	);
	const hasImageAttachments = $derived(attachments.some((attachment) => attachment.kind === "image"));
	const modelSupportsImages = $derived(Boolean((currentModel as unknown as { input?: string[] } | null)?.input?.includes("image")));
	const showImageModelWarning = $derived(Boolean(hasImageAttachments && currentModel && !modelSupportsImages));
	const canSubmit = $derived(Boolean(draft.trim() || attachments.length > 0));
	const contextBudgetTooltip = $derived(contextBudget ? "" : "Context unavailable");
	const contextBudgetTooltipDetails = $derived(
		contextBudget ? buildContextBudgetTooltipDetails(contextBudget) : [],
	);
	const workingElapsedLabel = $derived(formatWorkingElapsed(activeTurnStartedAt, workingTimerNow));
	const workingElapsedTooltip = $derived(
		formatWorkingElapsedTooltip(activeTurnStartedAt, workingTimerNow),
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
		if (!isStreaming) return;
		workingTimerNow = Date.now();
		const timer = window.setInterval(() => {
			workingTimerNow = Date.now();
		}, 1000);
		return () => window.clearInterval(timer);
	});

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
		void draft;
		void attachments;
		onBufferChange({ text: draft, attachments: cloneComposerAttachments(attachments) });
	});

	$effect(() => {
		if (editDraft) return;
		const storageKey = draftStorageKey;
		const updatedAt = composerDraft.updatedAt;
		const attachmentsKey = JSON.stringify(composerDraft.attachments);
		const incomingKey = `${storageKey}\u0000${updatedAt ?? ""}\u0000${composerDraft.text}\u0000${attachmentsKey}`;
		if (incomingKey === loadedComposerDraftKey) return;
		loadedComposerDraftKey = incomingKey;
		lastPersistedDraftPayloadKey = `${composerDraft.text}\u0000${attachmentsKey}`;
		if (draft === composerDraft.text && JSON.stringify(attachments) === attachmentsKey) {
			draftPersistenceReady = true;
			return;
		}
		draft = composerDraft.text;
		attachments = cloneComposerAttachments(composerDraft.attachments);
		resetHistoryNavigation();
		draftPersistenceReady = true;
		void tick().then(() => moveCaretToDraftEnd(composerDraft.text));
	});

	$effect(() => {
		if (editDraft) return;
		if (!draftPersistenceReady) return;
		void draftStorageKey;
		void draft;
		void attachments;
		const payloadKey = `${draft}\u0000${JSON.stringify(attachments)}`;
		if (payloadKey === lastPersistedDraftPayloadKey) return;
		lastPersistedDraftPayloadKey = payloadKey;
		onDraftChange({ text: draft, attachments: cloneComposerAttachments(attachments) });
	});

	$effect(() => {
		const editKey = editDraft ? String(editDraft.messageTimestamp) : null;
		if (!editDraft || editKey === loadedEditDraftKey) return;
		loadedEditDraftKey = editKey;
		draft = editDraft.text;
		attachments = [];
		resetHistoryNavigation();
		void tick().then(() => moveCaretToDraftEnd(editDraft.text));
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

	async function editQueuedMessage(promptId: string) {
		const text = await onEditQueuedMessage(promptId);
		if (text) {
			await restoreDraftBuffer(text);
		}
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

	async function applyPromptHistoryNavigation(direction: PromptHistoryDirection) {
		const navigation = navigatePromptHistory(promptHistory, historyNavigation, draft, direction);
		if (!navigation.changed) return;

		historyNavigation = navigation.nextState;
		draft = navigation.nextDraft;
		await tick();
		moveCaretToDraftEnd(navigation.nextDraft);
	}

	async function submit() {
		if (!canSubmit || isSubmitting) return;
		const editingMessageTimestamp = editDraft?.messageTimestamp;
		const nextDraft = serializeComposerDraft(draft);
		const nextVisibleDraft = draft;
		const nextAttachments = attachments;
		draft = "";
		attachments = [];
		isSubmitting = true;

		try {
			const sent = await onSend({
				text: nextDraft,
				attachments: nextAttachments,
				editMessageTimestamp: editingMessageTimestamp,
			});
			if (sent) {
				resetHistoryNavigation();
				if (editingMessageTimestamp !== undefined) {
					loadedEditDraftKey = null;
					onCancelEditMessage();
				}
			} else {
				await restoreDraftBuffer(nextVisibleDraft);
				attachments = nextAttachments;
			}
		} catch {
			await restoreDraftBuffer(nextVisibleDraft);
			attachments = nextAttachments;
		} finally {
			isSubmitting = false;
		}
	}

	function cancelEditMessage() {
		loadedEditDraftKey = null;
		draft = "";
		attachments = [];
		onCancelEditMessage();
		draftElement?.focus();
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
			const picked = await pickWorkspaceAttachments();
			if (picked.length === 0) {
				draftElement?.focus();
				return;
			}

			addAttachments(picked);
			await tick();
			draftElement?.focus();
		} catch {
			draftElement?.focus();
		}
	}

	function addAttachments(nextAttachments: ComposerAttachment[]) {
		const byId = new Map(attachments.map((attachment) => [attachment.id, attachment]));
		for (const attachment of nextAttachments) {
			byId.set(attachment.id, attachment);
		}
		attachments = [...byId.values()];
	}

	function removeAttachment(attachmentId: string) {
		attachments = attachments.filter((attachment) => attachment.id !== attachmentId);
		draftElement?.focus();
	}

	async function importFiles(files: File[]) {
		const importable = files.filter((file) => file.size > 0 || file.type);
		if (importable.length === 0) return;
		addAttachments(await importComposerAttachments(importable));
		await tick();
		draftElement?.focus();
	}

	function clipboardFiles(event: ClipboardEvent): File[] {
		const files = Array.from(event.clipboardData?.files ?? []);
		if (files.length > 0) return files;
		return Array.from(event.clipboardData?.items ?? [])
			.filter((item) => item.kind === "file")
			.map((item) => item.getAsFile())
			.filter((file): file is File => Boolean(file));
	}

	function handlePaste(event: ClipboardEvent) {
		const files = clipboardFiles(event);
		if (files.length === 0) return;
		event.preventDefault();
		void importFiles(files);
	}

	function handleDragOver(event: DragEvent) {
		if (!Array.from(event.dataTransfer?.types ?? []).includes("Files")) return;
		event.preventDefault();
		isDragActive = true;
	}

	function handleDragLeave(event: DragEvent) {
		if (event.currentTarget !== event.target) return;
		isDragActive = false;
	}

	function handleDrop(event: DragEvent) {
		const files = Array.from(event.dataTransfer?.files ?? []);
		if (files.length === 0) return;
		event.preventDefault();
		isDragActive = false;
		void importFiles(files);
	}

	function attachmentLabel(attachment: ComposerAttachment): string {
		return attachment.workspaceRelativePath ?? attachment.path;
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

	function exactTokenCount(count: number): string {
		return count.toLocaleString("en-US");
	}

	function buildContextBudgetTooltipDetails(budget: ContextBudget) {
		const availableTokens = Math.max(0, budget.maxTokens - budget.usedTokens);
		return [
			{ label: "Context", value: `${exactTokenCount(budget.usedTokens)} tok` },
			{ label: "Window", value: `${exactTokenCount(budget.maxTokens)} tok` },
			{ label: "Available", value: `${exactTokenCount(availableTokens)} tok` },
			{ label: "Used", value: `${budget.percent}%` },
		];
	}
</script>

<div
	role="group"
	aria-label="Message composer"
	class={`composer-shell ${isDragActive ? "drag-active" : ""}`.trim()}
	ondragover={handleDragOver}
	ondragleave={handleDragLeave}
	ondrop={handleDrop}
>
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
				{#if editDraft}
					<section class="composer-edit-row" aria-label="Editing message">
						<span>Editing message</span>
						<button type="button" onclick={cancelEditMessage}>Cancel</button>
					</section>
				{/if}
				{#if queuedMessages.length > 0}
					<QueuedMessagesStrip
						{queuedMessages}
						onEdit={(promptId) => void editQueuedMessage(promptId)}
						onDelete={onDeleteQueuedMessage}
						onSteer={onSteerQueuedMessage}
						onReorder={onReorderQueuedMessage}
					/>
				{/if}
				{#if attachments.length > 0 || showImageModelWarning}
					<section class="composer-context-row" aria-label="Attached file and context items">
						{#if showImageModelWarning}
							<div class="composer-attachment-warning" role="status">
								<TriangleAlertIcon size={13} aria-hidden="true" />
								<span>Current model is not listed as image-capable. Image attachments may be ignored or rejected.</span>
							</div>
						{/if}
						<div class="mention-chip-row">
							{#each attachments as attachment (attachment.id)}
								<Tooltip label={`Remove ${attachmentLabel(attachment)}`}>
									<button
										class="mention-chip"
										type="button"
										aria-label={`Remove attachment ${attachmentLabel(attachment)}`}
										onclick={() => removeAttachment(attachment.id)}
									>
										{#if attachment.kind === "image" && attachment.dataBase64 && attachment.mimeType}
											<img
												class="attachment-thumb"
												src={`data:${attachment.mimeType};base64,${attachment.dataBase64}`}
												alt=""
											/>
										{:else if attachment.kind === "folder"}
											<FolderIcon size={12} aria-hidden="true" />
										{:else if attachment.kind === "image"}
											<ImageIcon size={12} aria-hidden="true" />
										{:else}
											<FileIcon size={12} aria-hidden="true" />
										{/if}
										<span>{attachmentLabel(attachment)}</span>
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
					onpaste={handlePaste}
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
						<div class="compact-budget">
							<ContextBudgetBar
								budget={contextBudget ?? null}
								variant="compact"
								label="Context"
								tooltipLabel={contextBudgetTooltip}
								tooltipDetails={contextBudgetTooltipDetails}
							/>
						</div>
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
							<Tooltip label={workingElapsedTooltip}>
								<span class="composer-working-timer" role="status" aria-label={workingElapsedTooltip}>
									<ClockIcon size={14} aria-hidden="true" />
									<span>{workingElapsedLabel}</span>
								</span>
							</Tooltip>
						{/if}
						<Tooltip label={isStreaming ? "Queue message" : "Send message"} disabled={!currentModel || !canSubmit || isSubmitting}>
							<button
								class="composer-submit"
								type="button"
								aria-label={isStreaming ? "Queue message" : "Send"}
								onclick={() => void submit()}
								disabled={!currentModel || !canSubmit || isSubmitting}
							>
								<ArrowUpIcon size={15} aria-hidden="true" />
							</button>
						</Tooltip>
					</div>
				</div>
			</div>
		</div>

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

	.composer-context-row {
		border-top: 1px solid var(--ui-border-soft);
	}

	.composer-edit-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.5rem;
		padding: 0.34rem 0.52rem 0.32rem;
		border-bottom: 1px solid color-mix(in oklab, var(--ui-border-accent) 48%, var(--ui-border-soft));
		background: color-mix(in oklab, var(--ui-accent-soft) 52%, transparent);
		color: var(--ui-text-secondary);
		font-family: var(--font-mono);
		font-size: var(--text-xs);
	}

	.composer-edit-row button {
		border: 0;
		background: transparent;
		color: var(--ui-accent);
		font: inherit;
		font-weight: 600;
		cursor: pointer;
	}

	.composer-edit-row button:hover,
	.composer-edit-row button:focus-visible {
		color: var(--ui-text-primary);
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
		font-size: var(--text-xs);
		cursor: pointer;
	}

	.mention-chip span {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-weight: 600;
	}

	.attachment-thumb {
		width: 1.15rem;
		height: 1.15rem;
		border-radius: var(--ui-radius-xs);
		object-fit: cover;
		background: var(--ui-code);
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

	.drag-active .composer-input-wrap {
		border-color: color-mix(in oklab, var(--ui-accent) 72%, var(--ui-border-strong));
		background: color-mix(in oklab, var(--ui-accent-soft) 28%, var(--ui-surface));
		box-shadow: inset 0 0 0 1px color-mix(in oklab, var(--ui-accent) 22%, transparent);
	}

	.composer-attachment-warning {
		display: flex;
		align-items: center;
		gap: 0.32rem;
		margin-bottom: 0.34rem;
		color: var(--ui-warning);
		font-size: var(--text-xs);
		font-weight: 600;
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
		font-size: var(--text-base);
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
		gap: 0.46rem;
		padding: 0;
		border: 0;
		border-radius: 0;
		background: transparent;
	}

	.composer-control-cluster {
		max-width: 21rem;
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

	.composer-working-timer {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 0.25rem;
		flex: 0 0 auto;
		min-width: 3.55rem;
		height: 1.9rem;
		padding: 0 0.44rem;
		border: 1px solid color-mix(in oklab, var(--ui-accent) 28%, var(--ui-border-soft));
		border-radius: var(--ui-radius-md);
		background: color-mix(in oklab, var(--ui-accent) 9%, transparent);
		color: var(--ui-accent);
		font-family: var(--font-mono);
		font-size: var(--text-xs);
		font-weight: 600;
		font-variant-numeric: tabular-nums;
		line-height: 1;
	}

	.composer-icon-button:focus-visible,
	.composer-submit:focus-visible,
	.mention-chip:focus-visible {
		box-shadow: var(--ui-focus-ring);
	}

	.compact-budget {
		position: relative;
		display: flex;
		align-items: center;
		width: 7.35rem;
		height: 1.7rem;
		flex: 0 0 7.35rem;
		margin-left: 0.74rem;
	}

	.compact-budget :global(.context-budget-compact) {
		position: static;
		width: 100%;
	}

	.compact-budget :global(.context-budget-track) {
		height: 0.32rem;
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
		font-size: var(--text-sm);
		font-weight: 600;
	}

	.mention-option small {
		font-family: var(--font-mono);
		font-size: var(--text-xs);
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
		font-size: var(--text-sm);
		color: var(--ui-text-secondary);
	}

	.mention-empty.danger {
		color: var(--ui-danger);
	}

	.composer-error {
		margin: 0;
		padding: 0.55rem 0.72rem;
		border: 1px solid color-mix(in oklab, var(--ui-danger) 22%, var(--ui-border-soft));
		border-width: 0 0 1px;
		border-radius: 0;
		background: color-mix(in oklab, var(--ui-danger-soft) 74%, transparent);
		color: color-mix(in oklab, var(--ui-danger) 82%, var(--ui-text-primary));
		font-size: var(--text-base);
		line-height: 1.5;
	}

	@media (max-width: 760px) {
		.composer-main-row {
			grid-template-columns: minmax(0, 1fr);
			padding: 0.5rem;
		}

		.composer-icon-button,
		.composer-submit,
		.mention-option {
			min-height: 2.75rem;
		}

		.composer-icon-button,
		.composer-submit {
			width: 2.75rem;
			height: 2.75rem;
		}

		.composer-working-timer {
			height: 2.75rem;
			min-width: 4.45rem;
		}

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
			font-size: var(--text-base);
		}

		.composer-row-actions {
			justify-content: flex-start;
			gap: 0.3rem;
		}

		.compact-budget {
			display: none;
		}

		.composer-control-cluster {
			display: none;
		}

	}
</style>
