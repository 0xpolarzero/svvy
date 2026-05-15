<script lang="ts">
	import { onMount } from "svelte";
	import { HotkeysProvider } from "@tanstack/svelte-hotkeys";
	import ChatWorkspace from "./ChatWorkspace.svelte";
	import { createChatRuntime, type ChatRuntime } from "./chat-runtime";
	import { createChatStorage, type ChatStorage } from "./chat-storage";
	import { rpc } from "./rpc";
	import Settings from "./Settings.svelte";
	import { applyAppAppearance } from "./theme";
	import StatusCard from "./ui/StatusCard.svelte";
	import WorkspaceTabStrip, {
		type WorkspaceTabStripItem,
	} from "./WorkspaceTabStrip.svelte";
	import {
		EMPTY_WORKSPACE_TAB_COUNTS,
		reorderWorkspaceTabs,
		type WorkspaceTabCounts,
	} from "./workspace-tabs";
	import type { WorkspaceInfoResponse, WorkspaceTabInfo } from "../shared/workspace-contract";
	import type { AppAppearance } from "../shared/agent-settings";

	type OpenWorkspaceTab = {
		workspace: WorkspaceTabInfo;
		runtime: ChatRuntime;
		counts: WorkspaceTabCounts;
		unsubscribe: () => void;
	};

	const storage: ChatStorage = createChatStorage();
	let tabs = $state<OpenWorkspaceTab[]>([]);
	let activeWorkspaceId = $state<string | null>(null);
	let bootstrapError = $state<string | null>(null);
	let openingError = $state<string | null>(null);
	let restoring = $state(true);
	let openingWorkspace = $state(false);
	let showSettings = $state(false);
	let knownWorkspaces = $state<WorkspaceTabInfo[]>([]);
	let disposed = false;
	let disposeAppearanceSync: (() => void) | null = null;
	const activeTab = $derived(
		tabs.find((tab) => tab.workspace.workspaceId === activeWorkspaceId) ?? null,
	);
	const workspaceTabItems = $derived<WorkspaceTabStripItem[]>(
		tabs.map((tab) => ({ workspace: tab.workspace, counts: tab.counts })),
	);

	function summarizeWorkspace(runtime: ChatRuntime): WorkspaceTabCounts {
		const counts = { ...EMPTY_WORKSPACE_TAB_COUNTS };
		for (const session of runtime.sessions) {
			if (session.status === "running") counts.running += 1;
			if (session.isUnread) counts.unread += 1;
			if (session.status === "waiting") counts.waiting += 1;
			if (session.status === "error") counts.error += 1;
			counts.waiting += session.threadIdsByStatus?.waiting.length ?? 0;
			counts.error += session.threadIdsByStatus?.troubleshooting.length ?? 0;
			counts.running +=
				(session.threadIdsByStatus?.runningHandler.length ?? 0) +
				(session.threadIdsByStatus?.runningWorkflow.length ?? 0);
		}
		return counts;
	}

	function toWorkspaceTabInfo(
		workspace: WorkspaceInfoResponse | WorkspaceTabInfo,
		openedAt = new Date().toISOString(),
	): WorkspaceTabInfo {
		return {
			...workspace,
			openedAt: "openedAt" in workspace ? workspace.openedAt : openedAt,
		};
	}

	function workspaceHistoryKey(workspace: WorkspaceTabInfo): string {
		return workspace.cwd.trim() || workspace.workspaceId;
	}

	function mergeKnownWorkspaces(
		existing: readonly WorkspaceTabInfo[],
		incoming: readonly WorkspaceTabInfo[],
	): WorkspaceTabInfo[] {
		const byKey = new Map<string, WorkspaceTabInfo>();
		for (const workspace of existing) {
			byKey.set(workspaceHistoryKey(workspace), workspace);
		}
		for (const workspace of incoming) {
			byKey.set(workspaceHistoryKey(workspace), workspace);
		}
		return [...byKey.values()].toSorted((left, right) =>
			left.workspaceLabel.localeCompare(right.workspaceLabel),
		);
	}

	function persistWorkspaceTabs() {
		const openTabs = tabs.map((tab) => tab.workspace);
		knownWorkspaces = mergeKnownWorkspaces(knownWorkspaces, openTabs);
		void storage.appWorkspaceTabs
			.set({
				version: 3,
				activeWorkspaceId,
				tabs: openTabs,
				knownWorkspaces,
			})
			.catch((error) => console.error("Failed to persist workspace tabs:", error));
	}

	function openSettings() {
		if (showSettings) return;
		setTimeout(() => {
			if (!disposed) {
				showSettings = true;
			}
		}, 0);
	}

	function setAppAppearance(appearance: AppAppearance) {
		disposeAppearanceSync?.();
		disposeAppearanceSync = applyAppAppearance(appearance);
	}

	async function refreshAppAppearance() {
		try {
			const settings = await rpc.request.getAgentSettings();
			setAppAppearance(settings.appPreferences.appAppearance);
		} catch (error) {
			console.error("Failed to load app appearance:", error);
			setAppAppearance("system");
		}
	}

	async function createWorkspaceTab(workspace: WorkspaceInfoResponse | WorkspaceTabInfo): Promise<OpenWorkspaceTab> {
		const workspaceTab = toWorkspaceTabInfo(workspace);
		const runtime = await createChatRuntime(
			{
				workspaceInfo: workspaceTab,
				onMissingProviderAccess: () => {
					openSettings();
				},
			},
			undefined,
			storage,
		);
		const tab: OpenWorkspaceTab = {
			workspace: workspaceTab,
			runtime,
			counts: summarizeWorkspace(runtime),
			unsubscribe: () => {},
		};
		const unsubscribe = runtime.subscribe(() => {
			tab.counts = summarizeWorkspace(runtime);
			tabs = [...tabs];
		});
		tab.unsubscribe = unsubscribe;
		return tab;
	}

	async function setActiveWorkspace(workspaceId: string | null) {
		activeWorkspaceId = workspaceId;
		if (!workspaceId) {
			persistWorkspaceTabs();
			return;
		}
		try {
			await rpc.request.setActiveWorkspace({ workspaceId });
			await refreshAppAppearance();
		} catch (error) {
			console.error("Failed to set active workspace:", error);
		}
		persistWorkspaceTabs();
	}

	async function restoreWorkspaceTabs() {
		try {
			const restoreState = await storage.appWorkspaceTabs.get();
			knownWorkspaces = restoreState?.knownWorkspaces ?? [];
			const tabsToRestore = restoreState?.tabs.length
				? restoreState.tabs
				: await rpc.request.getOpenWorkspaces();
			knownWorkspaces = mergeKnownWorkspaces(knownWorkspaces, tabsToRestore);
			if (!tabsToRestore.length) {
				persistWorkspaceTabs();
				restoring = false;
				return;
			}

			const restoredTabs: OpenWorkspaceTab[] = [];
			for (const savedTab of tabsToRestore) {
				if (disposed) return;
				try {
					const workspaceInfo = restoreState?.tabs.length
						? (await rpc.request.openWorkspace({ cwd: savedTab.cwd })).workspace
						: await rpc.request.getWorkspaceInfo({
								workspaceId: savedTab.workspaceId,
							});
					if (!workspaceInfo) {
						continue;
					}
					restoredTabs.push(await createWorkspaceTab(toWorkspaceTabInfo(workspaceInfo, savedTab.openedAt)));
				} catch (error) {
					console.error("Failed to restore workspace tab:", error);
				}
			}

			if (disposed) {
				for (const tab of restoredTabs) {
					tab.unsubscribe();
					tab.runtime.dispose();
				}
				return;
			}

			tabs = restoredTabs;
			knownWorkspaces = mergeKnownWorkspaces(
				knownWorkspaces,
				restoredTabs.map((tab) => tab.workspace),
			);
			const savedActiveIndex = restoreState?.activeWorkspaceId
				? tabsToRestore.findIndex((tab) => tab.workspaceId === restoreState.activeWorkspaceId)
				: -1;
			const restoredActive =
				savedActiveIndex >= 0
					? (restoredTabs[savedActiveIndex]?.workspace.workspaceId ?? restoredTabs[0]?.workspace.workspaceId ?? null)
					: (restoredTabs[0]?.workspace.workspaceId ?? null);
			await setActiveWorkspace(restoredActive);
			bootstrapError = null;
		} catch (error) {
			if (!disposed) {
				bootstrapError = error instanceof Error ? error.message : "Unable to initialize svvy.";
			}
		} finally {
			if (!disposed) {
				restoring = false;
			}
		}
	}

	async function openWorkspace() {
		if (openingWorkspace) return;
		openingWorkspace = true;
		openingError = null;
		try {
			const response = await rpc.request.openWorkspace({});
			const workspaceInfo = response.workspace;
			if (!workspaceInfo) {
				return;
			}

			const tab = await createWorkspaceTab(workspaceInfo);
			if (disposed) {
				tab.unsubscribe();
				tab.runtime.dispose();
				return;
			}
			knownWorkspaces = mergeKnownWorkspaces(knownWorkspaces, [tab.workspace]);
			tabs = [...tabs, tab];
			await setActiveWorkspace(tab.workspace.workspaceId);
			bootstrapError = null;
		} catch (error) {
			openingError = error instanceof Error ? error.message : "Unable to open workspace.";
		} finally {
			openingWorkspace = false;
		}
	}

	async function closeWorkspaceTab(workspaceId: string) {
		const tab = tabs.find((candidate) => candidate.workspace.workspaceId === workspaceId);
		if (!tab) return;
		const index = tabs.indexOf(tab);
		tab.unsubscribe();
		tab.runtime.dispose();
		tabs = tabs.filter((candidate) => candidate.workspace.workspaceId !== workspaceId);
		try {
			await rpc.request.closeWorkspace({ workspaceId });
		} catch (error) {
			console.error("Failed to close workspace:", error);
		}
		if (activeWorkspaceId === workspaceId) {
			const nextTab = tabs[index] ?? tabs[index - 1] ?? null;
			await setActiveWorkspace(nextTab?.workspace.workspaceId ?? null);
			return;
		}
		persistWorkspaceTabs();
	}

	function reorderWorkspaceTab(workspaceId: string, beforeWorkspaceId: string | null) {
		const nextTabs = reorderWorkspaceTabs(tabs, workspaceId, beforeWorkspaceId);
		if (nextTabs.map((tab) => tab.workspace.workspaceId).join("\0") === tabs.map((tab) => tab.workspace.workspaceId).join("\0")) {
			return;
		}
		tabs = nextTabs;
		persistWorkspaceTabs();
	}

	async function handleProviderAuthChanged(providerId: string) {
		await Promise.all(tabs.map((tab) => tab.runtime.syncProviderAuth(providerId)));
	}

	onMount(() => {
		setAppAppearance("system");
		void restoreWorkspaceTabs();

		return () => {
			disposed = true;
			disposeAppearanceSync?.();
			disposeAppearanceSync = null;
			for (const tab of tabs) {
				tab.unsubscribe();
				tab.runtime.dispose();
			}
			tabs = [];
			activeWorkspaceId = null;
		};
	});
</script>

<HotkeysProvider defaultOptions={{ hotkey: { preventDefault: true, ignoreInputs: true } }}>
	<div class="app-shell">
		<div class="app-frame">
			<main class="workspace">
				{#if !activeTab}
					<header class="workspace-picker-chrome electrobun-webkit-app-region-drag">
						<WorkspaceTabStrip
							tabs={workspaceTabItems}
							{activeWorkspaceId}
							{openingWorkspace}
							onSelectWorkspace={(workspaceId) => void setActiveWorkspace(workspaceId)}
							onCloseWorkspace={(workspaceId) => void closeWorkspaceTab(workspaceId)}
							onOpenWorkspace={() => void openWorkspace()}
							onReorderWorkspace={reorderWorkspaceTab}
						/>
					</header>
				{/if}
				<div class="workspace-body">
					{#if bootstrapError}
						<StatusCard
							tone="error"
							eyebrow="Runtime Error"
							title="Startup failed"
							message={bootstrapError}
						/>
					{:else if activeTab}
						<ChatWorkspace
							runtime={activeTab.runtime}
							shortcutsEnabled={!showSettings}
							onOpenSettings={openSettings}
							workspaceTabs={workspaceTabItems}
							{activeWorkspaceId}
							{openingWorkspace}
							onSelectWorkspace={(workspaceId) => void setActiveWorkspace(workspaceId)}
							onCloseWorkspace={(workspaceId) => void closeWorkspaceTab(workspaceId)}
							onOpenWorkspace={() => void openWorkspace()}
							onReorderWorkspace={reorderWorkspaceTab}
						/>
					{/if}
					{#if restoring && !bootstrapError}
						<StatusCard
							eyebrow="Boot Sequence"
							title="Starting svvy"
							message="Restoring open workspace tabs."
						/>
					{:else if !activeTab && !bootstrapError}
						<section class="workspace-picker">
							<StatusCard
								eyebrow="Workspace"
								title="Open a repository"
								message={openingError ?? "Choose a workspace to start a repository-scoped svvy runtime."}
							/>
							<button class="workspace-picker-button" type="button" disabled={openingWorkspace} onclick={() => void openWorkspace()}>
								{openingWorkspace ? "Opening..." : "Open Workspace"}
							</button>
						</section>
					{/if}
				</div>
			</main>
		</div>
	</div>
</HotkeysProvider>

{#if showSettings}
	<Settings
		onClose={() => (showSettings = false)}
		onProviderAuthChanged={handleProviderAuthChanged}
		onAppAppearanceChanged={setAppAppearance}
	/>
{/if}

<style>
	.app-shell {
		height: 100%;
		min-height: 0;
		overflow: hidden;
	}

	.app-frame {
		display: grid;
		grid-template-rows: minmax(0, 1fr);
		height: 100%;
		min-height: 0;
		background: transparent;
		overflow: hidden;
	}

	.workspace {
		position: relative;
		display: grid;
		grid-template-rows: minmax(0, 1fr);
		--workspace-inset: 0.72rem;
		height: 100%;
		padding: 0;
		min-height: 0;
		overflow: hidden;
	}

	.workspace-picker-chrome {
		display: flex;
		align-items: center;
		position: absolute;
		top: 0;
		left: 0;
		right: 0;
		z-index: 3;
		min-height: 2.4rem;
		padding: 0.4rem 0.7rem 0.32rem;
	}

	.workspace-picker-button:disabled {
		cursor: default;
		opacity: 0.58;
	}

	.workspace-body {
		display: grid;
		grid-template-rows: minmax(0, 1fr);
		height: 100%;
		min-height: 0;
		overflow: hidden;
	}

	.workspace-picker {
		display: grid;
		place-content: center;
		gap: 0.8rem;
		height: 100%;
		padding: 1rem;
	}

	.workspace-picker-button {
		justify-self: center;
		height: 2rem;
		padding: 0 0.86rem;
		border: 1px solid var(--ui-border-strong);
		border-radius: var(--ui-radius-md);
		background: var(--ui-accent);
		color: var(--ui-accent-contrast);
		font: inherit;
		font-size: var(--text-base);
		font-weight: 600;
		cursor: pointer;
	}

	@media (max-width: 760px) {
		.workspace {
			--workspace-inset: 0rem;
		}
	}
</style>
