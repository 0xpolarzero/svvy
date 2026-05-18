import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { randomUUID } from "node:crypto";
import type { AppWorkspaceTabsState, WorkspaceTabInfo } from "../shared/workspace-contract";

const APP_WORKSPACE_TABS_FILENAME = "app-workspace-tabs.json";

export type AppWorkspaceTabsStore = {
  getState(): AppWorkspaceTabsState | null;
  setState(state: AppWorkspaceTabsState): AppWorkspaceTabsState;
  getPath(): string;
};

export function createAppWorkspaceTabsStore(input: { agentDir: string }): AppWorkspaceTabsStore {
  const statePath = join(input.agentDir, APP_WORKSPACE_TABS_FILENAME);

  const readState = (): AppWorkspaceTabsState | null => {
    if (!existsSync(statePath)) return null;
    try {
      return normalizeAppWorkspaceTabsState(JSON.parse(readFileSync(statePath, "utf8")));
    } catch {
      return null;
    }
  };

  const writeState = (state: AppWorkspaceTabsState): AppWorkspaceTabsState => {
    const normalized = normalizeAppWorkspaceTabsState(state) ?? {
      version: 4,
      activeWorkspaceTabId: null,
      tabs: [],
      knownWorkspaces: [],
    };
    mkdirSync(dirname(statePath), { recursive: true });
    writeFileSync(statePath, `${JSON.stringify(normalized, null, 2)}\n`);
    return normalized;
  };

  return {
    getState: readState,
    setState: writeState,
    getPath: () => statePath,
  };
}

function normalizeAppWorkspaceTabsState(input: unknown): AppWorkspaceTabsState | null {
  if (!input || typeof input !== "object") return null;
  const raw = input as Partial<AppWorkspaceTabsState>;
  if (raw.version !== 4 || !Array.isArray(raw.tabs) || !Array.isArray(raw.knownWorkspaces)) {
    return null;
  }

  const tabs = raw.tabs.map(normalizeWorkspaceTab).filter((tab): tab is WorkspaceTabInfo => !!tab);
  const knownWorkspaces = raw.knownWorkspaces
    .map(normalizeWorkspaceTab)
    .filter((tab): tab is WorkspaceTabInfo => !!tab);
  const activeWorkspaceTabId =
    typeof raw.activeWorkspaceTabId === "string" &&
    tabs.some((tab) => tab.workspaceTabId === raw.activeWorkspaceTabId)
      ? raw.activeWorkspaceTabId
      : null;

  return {
    version: 4,
    activeWorkspaceTabId,
    tabs,
    knownWorkspaces,
  };
}

function normalizeWorkspaceTab(input: unknown): WorkspaceTabInfo | null {
  if (!input || typeof input !== "object") return null;
  const raw = input as Partial<WorkspaceTabInfo>;
  if (
    typeof raw.workspaceId !== "string" ||
    typeof raw.cwd !== "string" ||
    typeof raw.workspaceLabel !== "string" ||
    (raw.kind !== "default" && raw.kind !== "user") ||
    typeof raw.openedAt !== "string"
  ) {
    return null;
  }
  return {
    workspaceTabId:
      typeof raw.workspaceTabId === "string" && raw.workspaceTabId.trim()
        ? raw.workspaceTabId
        : `workspace-tab-${randomUUID()}`,
    workspaceId: raw.workspaceId,
    cwd: raw.cwd,
    workspaceLabel: raw.workspaceLabel,
    kind: raw.kind,
    openedAt: raw.openedAt,
    ...(typeof raw.branch === "string" ? { branch: raw.branch } : {}),
    ...(raw.activeLayoutId === "A" || raw.activeLayoutId === "B" || raw.activeLayoutId === "C"
      ? { activeLayoutId: raw.activeLayoutId }
      : {}),
  };
}
