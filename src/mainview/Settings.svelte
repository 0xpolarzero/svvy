<script lang="ts">
	import CheckCircle2Icon from "@lucide/svelte/icons/check-circle-2";
	import CircleIcon from "@lucide/svelte/icons/circle";
	import ExternalLinkIcon from "@lucide/svelte/icons/external-link";
	import InfoIcon from "@lucide/svelte/icons/info";
	import KeyIcon from "@lucide/svelte/icons/key";
	import ShieldIcon from "@lucide/svelte/icons/shield";
	import { getModels, getProviders, type Model } from "@mariozechner/pi-ai";
	import { onMount } from "svelte";
	import { searchScore } from "./chat-format";
	import type { ProviderAuthInfo } from "../shared/workspace-contract";
	import type {
		AgentSettingsState,
		AppAppearance,
		AppPreferences,
		PreferredExternalEditor,
		SessionAgentKey,
		SessionAgentSettings,
		WorkflowAgentKey,
		WorkflowAgentSettings,
		WebProviderId,
	} from "../shared/agent-settings";
	import { rpc } from "./rpc";
	import AgentSettingsForm from "./AgentSettingsForm.svelte";
	import ProviderApiKeyForm from "./ProviderApiKeyForm.svelte";
	import Button from "./ui/Button.svelte";
	import Dialog from "./ui/Dialog.svelte";
	import Input from "./ui/Input.svelte";

	type Props = {
		onClose: () => void;
		onProviderAuthChanged?: (providerId: string) => void | Promise<void>;
		onAppAppearanceChanged?: (appearance: AppAppearance) => void;
	};

	type SettingsSection = "general" | "providers" | "web" | "agents" | "workflow-agents";
	type ModelOption = {
		key: string;
		provider: string;
		model: Model<any>;
	};

	const WEB_PROVIDER_OPTIONS: Array<{ id: WebProviderId | null; label: string; summary: string }> = [
		{ id: null, label: "None", summary: "Do not expose web tools or api.web helpers." },
		{ id: "tinyfish", label: "TinyFish", summary: "TinyFish Search and Fetch with a stored TinyFish API key." },
		{ id: "firecrawl", label: "Firecrawl", summary: "Firecrawl Search and Scrape with a stored Firecrawl API key." },
	];
	const APPEARANCE_OPTIONS: Array<{ value: AppAppearance; label: string; summary: string }> = [
		{ value: "system", label: "System", summary: "Follow macOS" },
		{ value: "light", label: "Light", summary: "Always light" },
		{ value: "dark", label: "Dark", summary: "Always dark" },
	];
	const EXTERNAL_EDITOR_OPTIONS: Array<{ value: PreferredExternalEditor; label: string }> = [
		{ value: "system", label: "System default" },
		{ value: "code", label: "Visual Studio Code" },
		{ value: "cursor", label: "Cursor" },
		{ value: "zed", label: "Zed" },
		{ value: "sublime", label: "Sublime Text" },
		{ value: "custom", label: "Custom command" },
	];

	let { onClose, onProviderAuthChanged, onAppAppearanceChanged }: Props = $props();

	let activeSection = $state<SettingsSection>("general");
	let providers = $state<ProviderAuthInfo[]>([]);
	let agentSettings = $state<AgentSettingsState | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let searchQuery = $state("");
	let editingProvider = $state<string | null>(null);
	let confirmingProviderRemoval = $state<string | null>(null);
	let oauthLoading = $state<Record<string, boolean>>({});
	let saveMessage = $state<Record<string, string>>({});
	let preferencesSaveMessage = $state("");

	const connectedProviderIds = $derived(
		new Set(providers.filter((provider) => provider.hasKey).map((provider) => provider.provider)),
	);

	const availableModelOptions = $derived.by(() => {
		const options: ModelOption[] = [];
		for (const provider of getProviders()) {
			if (!connectedProviderIds.has(provider)) continue;
			for (const model of getModels(provider)) {
				options.push({
					key: `${provider}:${model.id}`,
					provider,
					model,
				});
			}
		}
		return options.toSorted((left, right) => {
			const providerComparison = left.provider.localeCompare(right.provider);
			return providerComparison === 0 ? left.model.name.localeCompare(right.model.name) : providerComparison;
		});
	});

	async function refreshProviders() {
		error = null;
		try {
			providers = await rpc.request.listProviderAuths();
		} catch (err) {
			error = err instanceof Error ? err.message : "Failed to load providers";
		}
	}

	async function refreshAgentSettings() {
		agentSettings = await rpc.request.getAgentSettings();
	}

	async function notifyAuthChanged(providerId: string) {
		await onProviderAuthChanged?.(providerId);
	}

	function setTimedSaveMessage(providerId: string, message: string, timeoutMs: number) {
		saveMessage[providerId] = message;
		setTimeout(() => {
			saveMessage[providerId] = "";
		}, timeoutMs);
	}

	function providerStatus(info: ProviderAuthInfo) {
		if (!info.hasKey) return { text: "Not configured", tone: "neutral" as const };
		if (info.keyType === "oauth") return { text: "OAuth", tone: "success" as const };
		if (info.keyType === "env") return { text: "Env var", tone: "warning" as const };
		return { text: "API key", tone: "info" as const };
	}

	function providerCredentialLabel(info: ProviderAuthInfo): string {
		if (!info.hasKey) return info.supportsOAuth ? "OAuth or API key available" : "API key required";
		if (info.keyType === "env") return "Loaded from environment";
		if (info.keyType === "oauth") return "Connected with OAuth";
		return "Stored API key";
	}

	function providerSectionLabel(info: ProviderAuthInfo): string {
		if (info.hasKey && info.keyType === "env") return "Environment-backed credentials";
		if (info.hasKey && info.keyType === "oauth") return "OAuth connections";
		return "AI providers";
	}

	function providerInfo(providerId: string): ProviderAuthInfo | null {
		return providers.find((provider) => provider.provider === providerId) ?? null;
	}

	function webProviderReady(providerId: WebProviderId | null): { text: string; tone: "success" | "neutral" | "warning" } {
		if (!providerId) return { text: "No web tools", tone: "neutral" };
		const info = providerInfo(providerId);
		if (info?.hasKey) return { text: "Ready", tone: "success" };
		return { text: "API key required", tone: "warning" };
	}

	const sessionAgentLabels = {
		defaultSession: "Default Session",
		dumbOrchestrator: "Dumb Orchestrator",
		namer: "Namer",
	} satisfies Record<SessionAgentKey, string>;

	const sessionAgentSummaries = {
		defaultSession: "Used for normal repository sessions and long-running orchestrator turns.",
		dumbOrchestrator: "Used when a new dumb session should use the lightweight orchestrator defaults.",
		namer: "Generates session and handler-thread titles from the saved naming instruction.",
	} satisfies Record<SessionAgentKey, string>;

	const workflowAgentSummaries = {
		explorer: "Conventional saved workflow agent for bounded investigation tasks.",
		implementer: "Conventional saved workflow agent for production code changes.",
		reviewer: "Conventional saved workflow agent for verification and review tasks.",
	} satisfies Record<WorkflowAgentKey, string>;

	const filteredProviders = $derived.by(() => {
		if (!searchQuery.trim()) {
			return [...providers].toSorted((left, right) => {
				if (left.hasKey !== right.hasKey) return left.hasKey ? -1 : 1;
				return left.provider.localeCompare(right.provider);
			});
		}

		return providers
			.map((info) => {
				const status = providerStatus(info);
				const haystack = [
					info.provider,
					status.text,
					info.supportsOAuth ? "oauth api key" : "api key only",
					info.keyType,
				]
					.join(" ")
					.toLowerCase();

				return {
					info,
					score: searchScore(searchQuery, [haystack]),
				};
			})
			.filter((entry) => entry.score > 0)
			.toSorted((left, right) => {
				if (right.score !== left.score) return right.score - left.score;
				if (left.info.hasKey !== right.info.hasKey) return left.info.hasKey ? -1 : 1;
				return left.info.provider.localeCompare(right.info.provider);
			})
			.map((entry) => entry.info);
	});

	const providerGroups = $derived.by(() => {
		const aiProviders = filteredProviders.filter((info) => info.keyType !== "env");
		const envProviders = filteredProviders.filter((info) => info.keyType === "env");
		return [
			{ title: "AI Providers", providers: aiProviders, warning: false },
			{ title: "Environment-backed Credentials", providers: envProviders, warning: true },
		].filter((group) => group.providers.length > 0);
	});

	onMount(async () => {
		await Promise.all([refreshProviders(), refreshAgentSettings()]);
		loading = false;
	});

	async function saveSessionAgent(key: SessionAgentKey, settings: SessionAgentSettings) {
		if (!agentSettings) return settings;
		const nextSettings = await rpc.request.updateSessionAgentDefault({
			key,
			settings: structuredClone(settings),
		});
		agentSettings.sessionAgents[key] = structuredClone(nextSettings.sessionAgents[key]);
		return agentSettings.sessionAgents[key];
	}

	async function saveWorkflowAgent(key: WorkflowAgentKey, settings: WorkflowAgentSettings) {
		if (!agentSettings) return settings;
		const nextSettings = await rpc.request.updateWorkflowAgent({
			key,
			settings: structuredClone(settings),
		});
		agentSettings.workflowAgents[key] = structuredClone(nextSettings.workflowAgents[key]);
		return agentSettings.workflowAgents[key];
	}

	async function saveAppPreferences(preferences: AppPreferences) {
		try {
			preferencesSaveMessage = "Saving";
			agentSettings = await rpc.request.updateAppPreferences(structuredClone(preferences));
			onAppAppearanceChanged?.(agentSettings.appPreferences.appAppearance);
			preferencesSaveMessage = "Saved";
			setTimeout(() => {
				if (preferencesSaveMessage === "Saved") {
					preferencesSaveMessage = "";
				}
			}, 1800);
		} catch (err) {
			preferencesSaveMessage = err instanceof Error ? err.message : "Save failed";
		}
	}

	async function setAppAppearance(appearance: AppAppearance) {
		if (!agentSettings || agentSettings.appPreferences.appAppearance === appearance) return;
		await saveAppPreferences({
			...agentSettings.appPreferences,
			appAppearance: appearance,
		});
	}

	async function setPreferredExternalEditor(preferredExternalEditor: PreferredExternalEditor) {
		if (!agentSettings || agentSettings.appPreferences.preferredExternalEditor === preferredExternalEditor) return;
		await saveAppPreferences({
			...agentSettings.appPreferences,
			preferredExternalEditor,
		});
	}

	async function setCustomExternalEditorCommand(customExternalEditorCommand: string) {
		if (!agentSettings || agentSettings.appPreferences.customExternalEditorCommand === customExternalEditorCommand) return;
		await saveAppPreferences({
			...agentSettings.appPreferences,
			customExternalEditorCommand,
		});
	}

	async function seedWorkflowAgents() {
		await rpc.request.ensureWorkflowAgentsComponent();
		await refreshAgentSettings();
	}

	async function handleSaveApiKey(providerId: string, apiKey: string) {
		try {
			await rpc.request.setProviderApiKey({ providerId, apiKey });
			editingProvider = null;
			await refreshProviders();
			await notifyAuthChanged(providerId);
			setTimedSaveMessage(providerId, "Saved", 2000);
		} catch (err) {
			saveMessage[providerId] = err instanceof Error ? err.message : "Failed";
		}
	}

	async function handleOAuth(providerId: string) {
		oauthLoading[providerId] = true;
		saveMessage[providerId] = "";
		try {
			const result = await rpc.request.startOAuth({ providerId });
			if (result.ok) {
				await refreshProviders();
				await notifyAuthChanged(providerId);
				setTimedSaveMessage(providerId, "Connected", 3000);
			} else {
				saveMessage[providerId] = result.error ?? "OAuth failed";
			}
		} catch (err) {
			saveMessage[providerId] = err instanceof Error ? err.message : "OAuth failed";
		} finally {
			oauthLoading[providerId] = false;
		}
	}

	async function handleRemove(providerId: string) {
		if (confirmingProviderRemoval !== providerId) {
			confirmingProviderRemoval = providerId;
			saveMessage[providerId] = "Click Confirm remove to revoke this credential.";
			return;
		}
		try {
			await rpc.request.removeProviderAuth({ providerId });
			confirmingProviderRemoval = null;
			await refreshProviders();
			await notifyAuthChanged(providerId);
			setTimedSaveMessage(providerId, "Removed", 2000);
		} catch (err) {
			saveMessage[providerId] = err instanceof Error ? err.message : "Failed to remove provider";
		}
	}
</script>

<Dialog
	title="Settings"
	eyebrow="Workbench"
	description="App preferences and credentials stay local. Environment variables override saved keys."
	width="lg"
	class="settings-dialog"
	onClose={onClose}
>
	<div class="settings-shell">
		<aside class="settings-nav" aria-label="Settings sections">
			<p class="settings-nav-label">Sections</p>
			<button
				class={`settings-nav-item ${activeSection === "general" ? "active" : ""}`.trim()}
				type="button"
				aria-current={activeSection === "general" ? "page" : undefined}
				onclick={() => (activeSection = "general")}
			>
				<span>General</span>
				<span>{agentSettings?.appPreferences.appAppearance ?? "system"}</span>
			</button>
			<button
				class={`settings-nav-item ${activeSection === "providers" ? "active" : ""}`.trim()}
				type="button"
				aria-current={activeSection === "providers" ? "page" : undefined}
				onclick={() => (activeSection = "providers")}
			>
				<span>Providers</span>
				<span>{providers.length}</span>
			</button>
			<button
				class={`settings-nav-item ${activeSection === "web" ? "active" : ""}`.trim()}
				type="button"
				aria-current={activeSection === "web" ? "page" : undefined}
				onclick={() => (activeSection = "web")}
			>
				<span>Web</span>
				<span>{agentSettings?.appPreferences.webProvider ?? "none"}</span>
			</button>
			<button
				class={`settings-nav-item ${activeSection === "agents" ? "active" : ""}`.trim()}
				type="button"
				aria-current={activeSection === "agents" ? "page" : undefined}
				onclick={() => (activeSection = "agents")}
			>
				<span>Session Agents</span>
				<span>3</span>
			</button>
			<button
				class={`settings-nav-item ${activeSection === "workflow-agents" ? "active" : ""}`.trim()}
				type="button"
				aria-current={activeSection === "workflow-agents" ? "page" : undefined}
				onclick={() => (activeSection = "workflow-agents")}
			>
				<span>Workflow Agents</span>
				<span>3</span>
			</button>
		</aside>

		<section class="settings-pane">
			{#if activeSection === "general"}
				{#if loading || !agentSettings}
					<p class="loading">Loading settings...</p>
				{:else}
					<div class="settings-row-stack">
						<article class="provider-row general-row">
							<div class="provider-main general-main">
								<div class="provider-heading">
									<span class="provider-name">Appearance</span>
									<span class="provider-status tone-info">{agentSettings.appPreferences.appAppearance}</span>
									{#if preferencesSaveMessage}
										<span class="provider-status">{preferencesSaveMessage}</span>
									{/if}
								</div>
								<p class="provider-meta general-meta">Choose the app color theme.</p>
							</div>
							<div class="appearance-options" role="radiogroup" aria-label="Appearance">
								{#each APPEARANCE_OPTIONS as option (option.value)}
									<label class={`appearance-option ${agentSettings.appPreferences.appAppearance === option.value ? "selected" : ""}`.trim()}>
										<input
											type="radio"
											name="appAppearance"
											value={option.value}
											checked={agentSettings.appPreferences.appAppearance === option.value}
											disabled={preferencesSaveMessage === "Saving"}
											onchange={() => void setAppAppearance(option.value)}
										/>
										<span>{option.label}</span>
										<small>{option.summary}</small>
									</label>
								{/each}
							</div>
						</article>
						<article class="provider-row general-row">
							<div class="provider-main general-main">
								<div class="provider-heading">
									<span class="provider-name">External Editor</span>
									<span class="provider-status tone-info">{agentSettings.appPreferences.preferredExternalEditor}</span>
									{#if preferencesSaveMessage}
										<span class="provider-status">{preferencesSaveMessage}</span>
									{/if}
								</div>
								<p class="provider-meta general-meta">Choose which editor opens workspace files from product surfaces.</p>
							</div>
							<div class="editor-grid">
								<label class="settings-field">
									<span>Editor</span>
									<select
										value={agentSettings.appPreferences.preferredExternalEditor}
										disabled={preferencesSaveMessage === "Saving"}
										onchange={(event) => void setPreferredExternalEditor(event.currentTarget.value as PreferredExternalEditor)}
									>
										{#each EXTERNAL_EDITOR_OPTIONS as option (option.value)}
											<option value={option.value}>{option.label}</option>
										{/each}
									</select>
								</label>
								<label class="settings-field">
									<span>Custom command</span>
									<input
										value={agentSettings.appPreferences.customExternalEditorCommand}
										placeholder="editor-command --reuse-window"
										disabled={agentSettings.appPreferences.preferredExternalEditor !== "custom" || preferencesSaveMessage === "Saving"}
										onchange={(event) => void setCustomExternalEditorCommand(event.currentTarget.value)}
									/>
								</label>
							</div>
						</article>
					</div>
				{/if}
			{/if}
			{#if activeSection === "providers"}
				<div class="settings-search">
					<Input bind:value={searchQuery} placeholder="Search providers, auth types, or access state" />
					<p class="settings-search-summary">
						{filteredProviders.length} match{filteredProviders.length === 1 ? "" : "es"}
					</p>
				</div>

				{#if loading}
					<p class="loading">Loading providers...</p>
				{:else if error}
					<p class="error">{error}</p>
				{:else}
					<div class="provider-list" role="list">
						{#if filteredProviders.length === 0}
							<p class="provider-empty">No providers match the current search.</p>
						{/if}

						{#each providerGroups as group (group.title)}
							<section class="settings-group" aria-label={group.title}>
								<div class="settings-group-heading">
									<h3>{group.title}</h3>
									<span>{group.providers.length}</span>
								</div>
								{#if group.warning}
									<div class="settings-section-note tone-warning">
										<ShieldIcon aria-hidden="true" size={15} strokeWidth={1.8} />
										<p>Loaded from the shell environment. Edit them outside svvy.</p>
									</div>
								{/if}
								<div class="settings-row-stack">
									{#each group.providers as info (info.provider)}
										{@const status = providerStatus(info)}
										{@const isEditing = editingProvider === info.provider}
										{@const isConfirmingRemoval = confirmingProviderRemoval === info.provider}
										<article class="provider-row">
											<div class="provider-main">
												<div class="provider-heading">
													<span class={`provider-icon tone-${status.tone}`} aria-hidden="true">
														{#if info.hasKey}
															<CheckCircle2Icon size={14} strokeWidth={1.9} />
														{:else}
															<CircleIcon size={14} strokeWidth={1.9} />
														{/if}
													</span>
													<span class="provider-name">{info.provider}</span>
													<span class={`provider-status tone-${status.tone}`.trim()}>{status.text}</span>
												</div>
												<p class="provider-meta">
													<span>{providerSectionLabel(info)}</span>
													<span>{providerCredentialLabel(info)}</span>
												</p>
												{#if saveMessage[info.provider]}
													<p class={`save-msg ${isConfirmingRemoval ? "tone-danger" : ""}`.trim()}>
														{saveMessage[info.provider]}
													</p>
												{/if}
											</div>

											<div class="provider-actions">
												{#if isEditing}
													<ProviderApiKeyForm
														onSave={(apiKey) => handleSaveApiKey(info.provider, apiKey)}
														onCancel={() => (editingProvider = null)}
													/>
												{:else}
													{#if info.hasKey && info.keyType !== "env"}
														<Button
															variant={isConfirmingRemoval ? "danger" : "ghost"}
															size="xs"
															class="row-action action-danger"
															onclick={() => handleRemove(info.provider)}
															aria-label={isConfirmingRemoval
																? `Confirm removing ${info.provider} credentials`
																: `Remove ${info.provider} credentials`}
														>
															{isConfirmingRemoval ? "Confirm remove" : "Remove"}
														</Button>
													{/if}
													{#if info.keyType !== "env"}
														<Button
															variant="ghost"
															size="xs"
															class="row-action"
															aria-label={info.hasKey ? "Change API key" : "Add API key"}
															onclick={() => {
																editingProvider = info.provider;
															}}
														>
															<KeyIcon aria-hidden="true" size={12} strokeWidth={1.9} />
															{info.hasKey ? "Key" : "Add key"}
														</Button>
													{/if}
													{#if info.supportsOAuth && info.keyType !== "env"}
														<Button
															variant="ghost"
															size="xs"
															class="row-action action-success"
															disabled={oauthLoading[info.provider]}
															onclick={() => handleOAuth(info.provider)}
														>
															<ExternalLinkIcon aria-hidden="true" size={12} strokeWidth={1.9} />
															{oauthLoading[info.provider] ? "Waiting" : "OAuth"}
														</Button>
													{/if}
												{/if}
											</div>
										</article>
									{/each}
								</div>
							</section>
						{/each}
					</div>
				{/if}
			{/if}
			{#if activeSection === "web" && agentSettings}
				<div class="settings-section-note">
					<ShieldIcon aria-hidden="true" size={15} strokeWidth={1.8} />
					<p>Select TinyFish or Firecrawl and configure an API key. Until a selected provider is ready, svvy exposes no web tools or api.web helpers.</p>
				</div>
				<div class="settings-row-stack">
					{#each WEB_PROVIDER_OPTIONS as option (option.id ?? "none")}
						{@const readiness = webProviderReady(option.id)}
						{@const info = option.id ? providerInfo(option.id) : null}
						<article class="provider-row">
							<div class="provider-main">
								<div class="provider-heading">
									<input
										type="radio"
										name="web-provider"
										checked={agentSettings.appPreferences.webProvider === option.id}
										onchange={() => {
											agentSettings.appPreferences.webProvider = option.id;
											void saveAppPreferences(agentSettings.appPreferences);
										}}
									/>
									<span class="provider-name">{option.label}</span>
									<span class={`provider-status tone-${readiness.tone}`.trim()}>{readiness.text}</span>
									{#if agentSettings.appPreferences.webProvider === option.id}
										<span class="provider-status tone-info">Active</span>
									{/if}
								</div>
								<p class="provider-meta">{option.summary}</p>
								{#if option.id && saveMessage[option.id]}
									<p class="save-msg">{saveMessage[option.id]}</p>
								{/if}
							</div>
							{#if option.id}
								<div class="provider-actions">
									{#if editingProvider === option.id}
										<ProviderApiKeyForm
											placeholder={`Paste ${option.label} API key...`}
											onSave={(apiKey) => handleSaveApiKey(option.id, apiKey)}
											onCancel={() => (editingProvider = null)}
										/>
									{:else}
										{#if info?.hasKey && info.keyType !== "env"}
											<Button variant="ghost" size="xs" class="row-action action-danger" onclick={() => handleRemove(option.id)}>
												Remove
											</Button>
										{/if}
										<Button
											variant="ghost"
											size="xs"
											class="row-action"
											onclick={() => {
												editingProvider = option.id;
											}}
										>
											<KeyIcon aria-hidden="true" size={12} strokeWidth={1.9} />
											{info?.hasKey ? "Key" : "Add key"}
										</Button>
									{/if}
								</div>
							{/if}
						</article>
					{/each}
				</div>
			{/if}
			{#if activeSection === "agents" && agentSettings}
				<div class="settings-section-note">
					<InfoIcon aria-hidden="true" size={15} strokeWidth={1.8} />
					<p>Session agent changes save directly to workspace settings.</p>
				</div>
				<div class="agent-list">
					{#each ["defaultSession", "dumbOrchestrator", "namer"] as key (key)}
						{@const settings = agentSettings.sessionAgents[key as SessionAgentKey]}
						<AgentSettingsForm
							title={sessionAgentLabels[key as SessionAgentKey]}
							summary={sessionAgentSummaries[key as SessionAgentKey]}
							{settings}
							{availableModelOptions}
							onSave={(nextSettings) => saveSessionAgent(key as SessionAgentKey, nextSettings as SessionAgentSettings)}
						/>
					{/each}
				</div>
			{/if}
			{#if activeSection === "workflow-agents" && agentSettings}
				<div class="settings-search">
					<Button variant="primary" size="sm" onclick={seedWorkflowAgents}>Seed agents.ts</Button>
					<p class="settings-search-summary">Sync conventional workflow agents to .svvy/workflows/components/agents.ts</p>
				</div>
				<div class="agent-list">
					{#each ["explorer", "implementer", "reviewer"] as key (key)}
						{@const settings = agentSettings.workflowAgents[key as WorkflowAgentKey]}
						<AgentSettingsForm
							title={settings.label}
							summary={workflowAgentSummaries[key as WorkflowAgentKey]}
							{settings}
							{availableModelOptions}
							onSave={(nextSettings) => saveWorkflowAgent(key as WorkflowAgentKey, nextSettings as WorkflowAgentSettings)}
						/>
					{/each}
				</div>
			{/if}
		</section>
	</div>
</Dialog>

<style>
	:global(.settings-dialog.ui-dialog-panel) {
		width: min(96vw, 980px);
		max-width: 980px;
		max-height: min(92vh, 64rem);
	}

	:global(.settings-dialog .ui-dialog-header) {
		padding-block: 0.76rem 0.62rem;
	}

	:global(.settings-dialog .ui-dialog-description) {
		max-width: 42rem;
	}

	.settings-shell {
		display: grid;
		grid-template-columns: minmax(10.5rem, 11.5rem) minmax(0, 42rem);
		gap: 0.72rem;
		min-height: 0;
		justify-content: start;
	}

	.settings-nav {
		display: grid;
		align-content: start;
		gap: 0.12rem;
		padding: 0.2rem 0.2rem 0 0;
		border-right: 1px solid color-mix(in oklab, var(--ui-border-soft) 88%, transparent);
	}

	.settings-nav-label,
	.settings-search-summary,
	.save-msg {
		margin: 0;
		font-size: var(--text-xs);
		font-family: var(--font-mono);
		color: var(--ui-text-secondary);
	}

	.settings-nav-item {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		align-items: center;
		gap: 0.55rem;
		padding: 0.42rem 0.55rem;
		border: 1px solid transparent;
		border-radius: var(--ui-radius-sm);
		background: transparent;
		color: var(--ui-text-primary);
		font: inherit;
		text-align: left;
		cursor: pointer;
		transition:
			border-color 170ms cubic-bezier(0.19, 1, 0.22, 1),
			background-color 170ms cubic-bezier(0.19, 1, 0.22, 1),
			color 170ms cubic-bezier(0.19, 1, 0.22, 1);
	}

	.settings-nav-item span:first-child {
		font-size: var(--text-sm);
		font-weight: 600;
	}

	.settings-nav-item span:last-child {
		font-size: var(--text-xs);
		font-family: var(--font-mono);
		color: var(--ui-text-tertiary);
	}

	.settings-nav-item:hover,
	.settings-nav-item:focus-visible {
		outline: none;
		border-color: color-mix(in oklab, var(--ui-border-strong) 76%, transparent);
		background: color-mix(in oklab, var(--ui-surface-raised) 72%, transparent);
	}

	.settings-nav-item.active {
		border-color: color-mix(in oklab, var(--ui-border-soft) 86%, transparent);
		background: color-mix(in oklab, var(--ui-surface-raised) 86%, transparent);
	}

	.settings-pane {
		display: grid;
		align-content: start;
		gap: 0.72rem;
		min-width: 0;
		min-height: 0;
	}

	.settings-search {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		align-items: center;
		gap: 0.6rem;
		position: sticky;
		top: 0;
		z-index: var(--ui-z-sticky);
		padding: 0.38rem 0.48rem;
		border: 1px solid color-mix(in oklab, var(--ui-border-soft) 88%, transparent);
		border-radius: var(--ui-radius-sm);
		background: color-mix(in oklab, var(--ui-surface-subtle) 92%, transparent);
		box-shadow: none;
	}

	.loading,
	.error,
	.provider-empty {
		margin: 0;
		font-size: var(--text-base);
		color: var(--ui-text-secondary);
	}

	.error {
		color: color-mix(in oklab, var(--ui-danger) 84%, var(--ui-text-primary));
	}

	.provider-list {
		display: flex;
		flex-direction: column;
		gap: 0.78rem;
	}

	.settings-group {
		display: grid;
		gap: 0.34rem;
	}

	.settings-group-heading {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.7rem;
	}

	.settings-group-heading h3 {
		margin: 0;
		font-family: var(--font-mono);
		font-size: var(--text-xs);
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0;
		color: var(--ui-text-secondary);
	}

	.settings-group-heading span {
		font-family: var(--font-mono);
		font-size: var(--text-xs);
		color: var(--ui-text-tertiary);
	}

	.settings-row-stack {
		display: grid;
		border: 1px solid color-mix(in oklab, var(--ui-border-soft) 88%, transparent);
		border-radius: var(--ui-radius-sm);
		overflow: hidden;
	}

	.provider-row {
		display: grid;
		grid-template-columns: minmax(0, 1fr) minmax(11.25rem, 13rem);
		align-items: center;
		gap: 0.55rem 0.75rem;
		padding: 0.42rem 0.58rem;
		border: 0;
		border-bottom: 1px solid color-mix(in oklab, var(--ui-border-soft) 88%, transparent);
		border-radius: 0;
		background: var(--ui-surface);
		box-shadow: none;
		transition:
			background-color 170ms cubic-bezier(0.19, 1, 0.22, 1),
			border-color 170ms cubic-bezier(0.19, 1, 0.22, 1);
	}

	.provider-row:last-child {
		border-bottom: 0;
	}

	.provider-row:hover {
		background: color-mix(in oklab, var(--ui-surface-raised) 84%, var(--ui-surface));
	}

	.provider-main {
		display: grid;
		grid-template-columns: minmax(8.5rem, 1.1fr) minmax(7rem, 0.78fr) minmax(9rem, 1fr);
		align-items: center;
		gap: 0.24rem 0.68rem;
		min-width: 0;
	}

	.general-row {
		grid-template-columns: minmax(0, 1fr);
		align-items: start;
		gap: 0.7rem;
	}

	.general-main {
		grid-template-columns: minmax(0, 1fr);
		align-items: start;
		gap: 0.2rem;
	}

	.agent-list {
		display: grid;
		gap: 0.62rem;
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

	.provider-icon {
		display: inline-flex;
		align-items: center;
		color: var(--ui-text-tertiary);
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

	.provider-status.tone-success {
		border-color: color-mix(in oklab, var(--ui-success) 24%, var(--ui-border-soft));
		background: color-mix(in oklab, var(--ui-success-soft) 72%, transparent);
	}

	.provider-status.tone-warning {
		border-color: color-mix(in oklab, var(--ui-warning) 28%, var(--ui-border-soft));
		background: color-mix(in oklab, var(--ui-warning-soft) 72%, transparent);
	}

	.provider-status.tone-info {
		border-color: color-mix(in oklab, var(--ui-info) 24%, var(--ui-border-soft));
		background: color-mix(in oklab, var(--ui-info-soft) 72%, transparent);
	}

	.provider-status.tone-success,
	.provider-icon.tone-success {
		color: color-mix(in oklab, var(--ui-success) 78%, var(--ui-text-primary));
	}

	.provider-status.tone-warning,
	.provider-icon.tone-warning {
		color: color-mix(in oklab, var(--ui-warning) 82%, var(--ui-text-primary));
	}

	.provider-status.tone-info,
	.provider-icon.tone-info {
		color: color-mix(in oklab, var(--ui-info) 78%, var(--ui-text-primary));
	}

	.provider-status.tone-neutral,
	.provider-icon.tone-neutral {
		color: var(--ui-text-tertiary);
	}

	.provider-meta {
		margin: 0;
		display: contents;
		font-size: var(--text-sm);
		line-height: 1.35;
		color: var(--ui-text-secondary);
	}

	.provider-meta.general-meta {
		display: block;
	}

	.appearance-options {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr));
		gap: 0.45rem;
	}

	.appearance-option {
		display: grid;
		grid-template-columns: auto minmax(0, 1fr);
		gap: 0.06rem 0.42rem;
		align-items: center;
		min-width: 0;
		padding: 0.5rem 0.58rem;
		border: 1px solid color-mix(in oklab, var(--ui-border-soft) 88%, transparent);
		border-radius: var(--ui-radius-sm);
		background: color-mix(in oklab, var(--ui-surface-subtle) 68%, transparent);
		cursor: pointer;
		transition:
			border-color 150ms cubic-bezier(0.19, 1, 0.22, 1),
			background-color 150ms cubic-bezier(0.19, 1, 0.22, 1);
	}

	.appearance-option:hover {
		border-color: color-mix(in oklab, var(--ui-border-strong) 82%, transparent);
		background: var(--ui-hover-bg);
	}

	.appearance-option.selected {
		border-color: var(--ui-selected-border);
		background: var(--ui-selected-bg);
	}

	.appearance-option input {
		grid-row: 1 / span 2;
		accent-color: var(--ui-accent);
	}

	.appearance-option span,
	.appearance-option small {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.appearance-option span {
		font-size: var(--text-sm);
		font-weight: 600;
		color: var(--ui-text-primary);
	}

	.appearance-option small {
		font-size: var(--text-xs);
		color: var(--ui-text-secondary);
	}

	.editor-grid {
		display: grid;
		grid-template-columns: minmax(0, 1fr) minmax(12rem, 1fr);
		gap: 0.54rem;
	}

	.settings-field {
		display: grid;
		gap: 0.28rem;
		min-width: 0;
	}

	.settings-field span {
		font-size: var(--text-xs);
		font-family: var(--font-mono);
		color: var(--ui-text-secondary);
	}

	.settings-field select,
	.settings-field input {
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

	.settings-field select:disabled,
	.settings-field input:disabled {
		color: var(--ui-text-tertiary);
		cursor: not-allowed;
		opacity: 0.72;
	}

	.provider-meta span {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.save-msg {
		grid-column: 1 / -1;
		color: var(--ui-accent-strong);
	}

	.save-msg.tone-danger {
		color: color-mix(in oklab, var(--ui-danger) 84%, var(--ui-text-primary));
	}

	.provider-actions {
		display: flex;
		align-items: center;
		justify-content: flex-end;
		gap: 0.18rem;
		min-width: 11.25rem;
		padding-left: 0.62rem;
		border-left: 1px solid color-mix(in oklab, var(--ui-border-soft) 72%, transparent);
		flex-wrap: wrap;
	}

	.provider-actions :global(.row-action.ui-button) {
		box-shadow: none;
		font-weight: 500;
	}

	.provider-actions :global(.action-success.ui-button) {
		color: color-mix(in oklab, var(--ui-success) 78%, var(--ui-text-primary));
	}

	.provider-actions :global(.action-danger.ui-button:not(.variant-danger)) {
		color: color-mix(in oklab, var(--ui-danger) 78%, var(--ui-text-primary));
	}

	.key-input-row {
		display: flex;
		align-items: center;
		gap: 0.24rem;
		flex-wrap: wrap;
		justify-content: flex-end;
	}

	.settings-section-note {
		display: flex;
		align-items: flex-start;
		gap: 0.42rem;
		padding: 0.42rem 0.55rem;
		border: 1px solid color-mix(in oklab, var(--ui-info) 18%, var(--ui-border-soft));
		border-radius: var(--ui-radius-sm);
		background: color-mix(in oklab, var(--ui-info-soft) 58%, transparent);
		color: var(--ui-text-secondary);
	}

	.settings-section-note.tone-warning {
		border-color: color-mix(in oklab, var(--ui-warning) 20%, var(--ui-border-soft));
		background: color-mix(in oklab, var(--ui-warning-soft) 60%, transparent);
	}

	.settings-section-note p {
		margin: 0;
		font-size: var(--text-sm);
		line-height: 1.38;
	}

	:global(.key-input-row .ui-input) {
		font-size: var(--text-sm);
		width: min(260px, 70vw);
	}

	@media (max-width: 760px) {
		.settings-shell {
			grid-template-columns: minmax(0, 1fr);
		}

		.settings-search {
			grid-template-columns: minmax(0, 1fr);
		}

		.settings-nav {
			padding: 0 0 0.2rem;
			border-right: none;
			border-bottom: 1px solid color-mix(in oklab, var(--ui-border-soft) 88%, transparent);
		}

		.provider-row {
			grid-template-columns: 1fr;
		}

		.provider-main {
			grid-template-columns: 1fr;
		}

		.provider-meta {
			display: flex;
			gap: 0.38rem;
			flex-wrap: wrap;
		}

		.provider-meta span {
			white-space: normal;
		}

		.agent-grid {
			grid-template-columns: 1fr;
		}

		.editor-grid {
			grid-template-columns: 1fr;
		}

		.agent-meta-grid {
			grid-template-columns: 1fr;
		}

		.provider-actions {
			width: 100%;
			min-width: 0;
			justify-content: flex-start;
			padding-left: 0;
			border-left: none;
		}

		.key-input-row {
			width: 100%;
			justify-content: flex-start;
		}

		:global(.key-input-row .ui-input) {
			width: 100%;
		}
	}
</style>
