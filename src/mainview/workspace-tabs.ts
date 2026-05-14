import type { WorkspaceTabInfo } from "../shared/workspace-contract";

export type WorkspaceTabCounts = {
  running: number;
  unread: number;
  waiting: number;
  error: number;
};

export type WorkspaceTabCountKind = keyof WorkspaceTabCounts;

export type WorkspaceTabVisibleCount = {
  kind: WorkspaceTabCountKind;
  value: number;
  label: string;
};

export const EMPTY_WORKSPACE_TAB_COUNTS: WorkspaceTabCounts = {
  running: 0,
  unread: 0,
  waiting: 0,
  error: 0,
};

export const WORKSPACE_TAB_COUNT_LABELS: Record<WorkspaceTabCountKind, string> = {
  running: "running",
  unread: "unread",
  waiting: "waiting",
  error: "errors",
};

export const WORKSPACE_TAB_COUNT_ORDER: WorkspaceTabCountKind[] = [
  "running",
  "unread",
  "waiting",
  "error",
];

export function getVisibleWorkspaceTabCounts(
  counts: WorkspaceTabCounts,
): WorkspaceTabVisibleCount[] {
  return WORKSPACE_TAB_COUNT_ORDER.flatMap((kind) => {
    const value = counts[kind];
    if (value <= 0) return [];
    return [{ kind, value, label: WORKSPACE_TAB_COUNT_LABELS[kind] }];
  });
}

export function formatWorkspaceTabAriaLabel(
  workspace: Pick<WorkspaceTabInfo, "workspaceLabel">,
  counts: WorkspaceTabCounts,
): string {
  const visibleCounts = getVisibleWorkspaceTabCounts(counts);
  if (!visibleCounts.length) return workspace.workspaceLabel;
  return `${workspace.workspaceLabel}. ${visibleCounts
    .map((count) => `${count.value} ${count.label}`)
    .join(", ")}`;
}

export function reorderWorkspaceTabs<
  T extends { workspace: Pick<WorkspaceTabInfo, "workspaceId"> },
>(tabs: readonly T[], workspaceId: string, beforeWorkspaceId: string | null): T[] {
  if (workspaceId === beforeWorkspaceId) return [...tabs];
  const movingTab = tabs.find((tab) => tab.workspace.workspaceId === workspaceId);
  if (!movingTab) return [...tabs];

  const remainingTabs = tabs.filter((tab) => tab.workspace.workspaceId !== workspaceId);
  const beforeIndex = beforeWorkspaceId
    ? remainingTabs.findIndex((tab) => tab.workspace.workspaceId === beforeWorkspaceId)
    : -1;
  const nextTabs = [...remainingTabs];
  nextTabs.splice(beforeIndex >= 0 ? beforeIndex : nextTabs.length, 0, movingTab);
  return nextTabs;
}
