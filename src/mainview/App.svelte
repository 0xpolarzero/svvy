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
	import type { WorkspaceTabStripItem } from "./WorkspaceTabStrip.svelte";
	import {
		EMPTY_WORKSPACE_TAB_COUNTS,
		reorderWorkspaceTabs,
		type WorkspaceTabCounts,
	} from "./workspace-tabs";
	import type {
		AppWorkspaceTabsState,
		WorkspaceInfoResponse,
		WorkspaceTabInfo,
	} from "../shared/workspace-contract";
	import type { AppAppearance } from "../shared/agent-settings";

	type OpenWorkspaceTab = {
		workspace: WorkspaceTabInfo;
		runtime: ChatRuntime;
		counts: WorkspaceTabCounts;
		unsubscribe: () => void;
	};

	const storage: ChatStorage = createChatStorage();
	let tabs = $state<OpenWorkspaceTab[]>([]);
	let bootstrapError = $state<string | null>(null);
	let openingError = $state<string | null>(null);
	let restoring = $state(true);
	let openingWorkspace = $state(false);
	let showSettings = $state(false);
	let knownWorkspaces = $state<WorkspaceTabInfo[]>([]);
	let disposed = false;
	let disposeAppearanceSync: (() => void) | null = null;
	const createWorkspaceTabId = () => `workspace-tab-${crypto.randomUUID()}`;
	let activeWorkspaceTabId = $state<string | null>(null);
	const activeTab = $derived(
		tabs.find((tab) => tab.workspace.workspaceTabId === activeWorkspaceTabId) ?? null,
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
		workspaceTabId = "workspaceTabId" in workspace ? workspace.workspaceTabId : createWorkspaceTabId(),
	): WorkspaceTabInfo {
		return {
			...workspace,
			workspaceTabId,
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
			if (workspace.kind === "default") continue;
			byKey.set(workspaceHistoryKey(workspace), workspace);
		}
		for (const workspace of incoming) {
			if (workspace.kind === "default") continue;
			byKey.set(workspaceHistoryKey(workspace), workspace);
		}
		return [...byKey.values()].toSorted((left, right) =>
			left.workspaceLabel.localeCompare(right.workspaceLabel),
		);
	}

	async function persistWorkspaceTabs() {
		const openTabs = tabs.map((tab) => tab.workspace);
		knownWorkspaces = mergeKnownWorkspaces(knownWorkspaces, openTabs);
		const state: AppWorkspaceTabsState = {
			version: 4,
			activeWorkspaceTabId,
			tabs: openTabs,
			knownWorkspaces,
		};
		await rpc.request
			.setAppWorkspaceTabs(state)
			.catch((error) => console.error("Failed to persist app workspace tabs:", error));
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
		if (!activeTab) {
			setAppAppearance("system");
			return;
		}
		try {
			const preferences = await rpc.request.getAppPreferences({ workspaceId: activeTab.workspace.workspaceId });
			setAppAppearance(preferences.appAppearance);
		} catch (error) {
			console.error("Failed to load app appearance:", error);
			setAppAppearance("system");
		}
	}

	async function createWorkspaceTab(
		workspace: WorkspaceInfoResponse | WorkspaceTabInfo,
		workspaceTabId?: string,
	): Promise<OpenWorkspaceTab> {
		const workspaceTab = toWorkspaceTabInfo(workspace, new Date().toISOString(), workspaceTabId);
		const runtime = await createChatRuntime(
			{
				workspaceInfo: workspaceTab,
				workspaceTabId: workspaceTab.workspaceTabId,
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

	async function setActiveWorkspace(workspaceTabId: string | null) {
		activeWorkspaceTabId = workspaceTabId;
		const tab = tabs.find((candidate) => candidate.workspace.workspaceTabId === workspaceTabId) ?? null;
		if (!tab) {
			await persistWorkspaceTabs();
			return;
		}
		try {
			await rpc.request.setActiveWorkspace({ workspaceId: tab.workspace.workspaceId });
			await refreshAppAppearance();
		} catch (error) {
			console.error("Failed to set active workspace:", error);
		}
		await persistWorkspaceTabs();
	}

	async function restoreWorkspaceTabs() {
		try {
			const restoreState = await rpc.request.getAppWorkspaceTabs().catch((error) => {
				console.error("Failed to load app workspace tabs:", error);
				return null;
			});
			knownWorkspaces = restoreState?.knownWorkspaces ?? [];
			const tabsToRestore = restoreState?.tabs.length ? restoreState.tabs : [];
			knownWorkspaces = mergeKnownWorkspaces(knownWorkspaces, tabsToRestore);

			const restoredTabs: OpenWorkspaceTab[] = [];
			for (const savedTab of tabsToRestore) {
				if (disposed) return;
				try {
					const workspaceInfo =
						savedTab.kind === "default"
							? await rpc.request.getDefaultWorkspace()
							: (await rpc.request.openWorkspace({ cwd: savedTab.cwd, workspaceTabId: savedTab.workspaceTabId })).workspace;
					if (!workspaceInfo) {
						continue;
					}
					restoredTabs.push(await createWorkspaceTab(toWorkspaceTabInfo(workspaceInfo, savedTab.openedAt, savedTab.workspaceTabId), savedTab.workspaceTabId));
				} catch (error) {
					console.error("Failed to restore workspace tab:", error);
					const fallback = await rpc.request.getDefaultWorkspace();
					restoredTabs.push(await createWorkspaceTab(toWorkspaceTabInfo(fallback, savedTab.openedAt, savedTab.workspaceTabId), savedTab.workspaceTabId));
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
			if (!tabs.length) {
				const defaultInfo = await rpc.request.getDefaultWorkspace();
				tabs = [await createWorkspaceTab(defaultInfo)];
			}
			knownWorkspaces = mergeKnownWorkspaces(
				knownWorkspaces,
				restoredTabs.map((tab) => tab.workspace),
			);
			const savedActiveIndex = restoreState?.activeWorkspaceTabId
				? tabsToRestore.findIndex((tab) => tab.workspaceTabId === restoreState.activeWorkspaceTabId)
				: -1;
			const restoredActive =
				savedActiveIndex >= 0
					? (tabs[savedActiveIndex]?.workspace.workspaceTabId ?? tabs[0]?.workspace.workspaceTabId ?? null)
					: (tabs[0]?.workspace.workspaceTabId ?? null);
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

	async function openWorkspace(placement: "current-tab" | "new-tab" = "current-tab") {
		if (openingWorkspace) return;
		openingWorkspace = true;
		openingError = null;
		try {
			const response =
				placement === "new-tab"
					? await rpc.request.openWorkspace({ placement: "new-tab" })
					: await rpc.request.openWorkspace({ placement: "current-tab" });
			const workspaceInfo = response.workspace;
			if (!workspaceInfo) {
				return;
			}

			const tab = await createWorkspaceTab(workspaceInfo, placement === "current-tab" ? activeWorkspaceTabId ?? undefined : undefined);
			if (disposed) {
				tab.unsubscribe();
				tab.runtime.dispose();
				return;
			}
			knownWorkspaces = mergeKnownWorkspaces(knownWorkspaces, [tab.workspace]);
			if (placement === "new-tab" || !activeWorkspaceTabId) {
				const activeIndex = tabs.findIndex((candidate) => candidate.workspace.workspaceTabId === activeWorkspaceTabId);
				tabs = [
					...tabs.slice(0, activeIndex + 1),
					tab,
					...tabs.slice(activeIndex + 1),
				];
			} else {
				const oldTab = tabs.find((candidate) => candidate.workspace.workspaceTabId === activeWorkspaceTabId);
				tabs = tabs.map((candidate) =>
					candidate.workspace.workspaceTabId === activeWorkspaceTabId ? tab : candidate,
				);
				oldTab?.unsubscribe();
				oldTab?.runtime.dispose();
				if (oldTab) {
					void rpc.request.closeWorkspace({ workspaceId: oldTab.workspace.workspaceId });
				}
			}
			await setActiveWorkspace(tab.workspace.workspaceTabId);
			bootstrapError = null;
		} catch (error) {
			openingError = error instanceof Error ? error.message : "Unable to open workspace.";
		} finally {
			openingWorkspace = false;
		}
	}

	async function createDefaultWorkspaceTab() {
		const defaultInfo = await rpc.request.getDefaultWorkspace();
		const tab = await createWorkspaceTab(defaultInfo);
		const activeIndex = tabs.findIndex((candidate) => candidate.workspace.workspaceTabId === activeWorkspaceTabId);
		tabs = [
			...tabs.slice(0, activeIndex + 1),
			tab,
			...tabs.slice(activeIndex + 1),
		];
		await setActiveWorkspace(tab.workspace.workspaceTabId);
	}

	async function closeWorkspaceTab(workspaceTabId: string) {
		const tab = tabs.find((candidate) => candidate.workspace.workspaceTabId === workspaceTabId);
		if (!tab) return;
		const index = tabs.indexOf(tab);
		tab.unsubscribe();
		tab.runtime.dispose();
		tabs = tabs.filter((candidate) => candidate.workspace.workspaceTabId !== workspaceTabId);
		try {
			await rpc.request.closeWorkspace({ workspaceId: tab.workspace.workspaceId });
		} catch (error) {
			console.error("Failed to close workspace:", error);
		}
		if (!tabs.length) {
			const defaultInfo = await rpc.request.getDefaultWorkspace();
			tabs = [await createWorkspaceTab(defaultInfo)];
			await setActiveWorkspace(tabs[0]!.workspace.workspaceTabId);
			return;
		}
		if (activeWorkspaceTabId === workspaceTabId) {
			const nextTab = tabs[index] ?? tabs[index - 1] ?? null;
			await setActiveWorkspace(nextTab?.workspace.workspaceTabId ?? null);
			return;
		}
		await persistWorkspaceTabs();
	}

	function reorderWorkspaceTab(workspaceTabId: string, beforeWorkspaceTabId: string | null) {
		const nextTabs = reorderWorkspaceTabs(tabs, workspaceTabId, beforeWorkspaceTabId);
		if (nextTabs.map((tab) => tab.workspace.workspaceTabId).join("\0") === tabs.map((tab) => tab.workspace.workspaceTabId).join("\0")) {
			return;
		}
		tabs = nextTabs;
		void persistWorkspaceTabs();
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
			activeWorkspaceTabId = null;
		};
	});
</script>

<HotkeysProvider defaultOptions={{ hotkey: { preventDefault: true, ignoreInputs: true } }}>
	<div class="app-shell">
		<div class="app-frame">
			<main class="workspace">
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
							{activeWorkspaceTabId}
							{openingWorkspace}
							openWorkspaceError={openingError}
							{knownWorkspaces}
							onSelectWorkspace={(workspaceTabId) => void setActiveWorkspace(workspaceTabId)}
							onCloseWorkspace={(workspaceTabId) => void closeWorkspaceTab(workspaceTabId)}
							onOpenWorkspace={() => void openWorkspace("current-tab")}
							onNewWorkspaceTab={() => void createDefaultWorkspaceTab()}
							onOpenWorkspaceInNewTab={() => void openWorkspace("new-tab")}
							onReorderWorkspace={reorderWorkspaceTab}
						/>
					{/if}
					{#if restoring && !bootstrapError}
						<StatusCard
							eyebrow="Boot Sequence"
							title="Starting svvy"
							message="Restoring open workspace tabs."
						/>
					{/if}
				</div>
			</main>
		</div>
	</div>
</HotkeysProvider>

{#if showSettings}
	<Settings
		workspaceId={activeTab?.workspace.workspaceId ?? null}
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

	.workspace-body {
		display: grid;
		grid-template-rows: minmax(0, 1fr);
		height: 100%;
		min-height: 0;
		overflow: hidden;
	}

	@media (max-width: 760px) {
		.workspace {
			--workspace-inset: 0rem;
		}
	}
</style>
