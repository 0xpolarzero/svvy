import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import type { AppWorkspaceUiRestoreState } from "../shared/workspace-contract";

const APP_WORKSPACE_UI_RESTORE_FILENAME = "app-workspace-ui-restore.json";

export type AppWorkspaceUiRestoreStore = {
  getState(workspaceId: string): AppWorkspaceUiRestoreState | null;
  setState(workspaceId: string, state: AppWorkspaceUiRestoreState): AppWorkspaceUiRestoreState;
  getPath(): string;
};

export function createAppWorkspaceUiRestoreStore(input: {
  agentDir: string;
}): AppWorkspaceUiRestoreStore {
  const statePath = join(input.agentDir, APP_WORKSPACE_UI_RESTORE_FILENAME);

  const readAll = (): Record<string, AppWorkspaceUiRestoreState> => {
    if (!existsSync(statePath)) return {};
    try {
      const parsed = JSON.parse(readFileSync(statePath, "utf8"));
      return normalizeAllStates(parsed);
    } catch {
      return {};
    }
  };

  const writeAll = (states: Record<string, AppWorkspaceUiRestoreState>): void => {
    mkdirSync(dirname(statePath), { recursive: true });
    writeFileSync(statePath, `${JSON.stringify(states, null, 2)}\n`);
  };

  return {
    getState: (workspaceId) => readAll()[createWorkspaceUiRestoreKey(workspaceId)] ?? null,
    setState: (workspaceId, state) => {
      const normalized = normalizeWorkspaceUiRestoreState(state) ?? createEmptyRestoreState();
      writeAll({
        ...readAll(),
        [createWorkspaceUiRestoreKey(workspaceId)]: normalized,
      });
      return normalized;
    },
    getPath: () => statePath,
  };
}

function createWorkspaceUiRestoreKey(input: string): string {
  const workspaceId = input.trim();
  if (!workspaceId) {
    throw new Error("Workspace UI restore state requires a workspaceId.");
  }
  return `workspace:${encodeURIComponent(workspaceId)}`;
}

function normalizeAllStates(input: unknown): Record<string, AppWorkspaceUiRestoreState> {
  if (!input || typeof input !== "object" || Array.isArray(input)) {
    return {};
  }

  const output: Record<string, AppWorkspaceUiRestoreState> = {};
  for (const [workspaceId, value] of Object.entries(input)) {
    const normalized = normalizeWorkspaceUiRestoreState(value);
    if (typeof workspaceId === "string" && normalized) {
      output[workspaceId] = normalized;
    }
  }
  return output;
}

function normalizeWorkspaceUiRestoreState(input: unknown): AppWorkspaceUiRestoreState | null {
  if (!input || typeof input !== "object" || Array.isArray(input)) {
    return null;
  }

  const raw = input as Partial<AppWorkspaceUiRestoreState>;
  if (
    raw.version !== 5 ||
    !raw.layouts ||
    typeof raw.layouts !== "object" ||
    Array.isArray(raw.layouts)
  ) {
    return null;
  }

  return {
    version: 5,
    layouts: {
      A: normalizeLayoutValue(raw.layouts.A),
      B: normalizeLayoutValue(raw.layouts.B),
      C: normalizeLayoutValue(raw.layouts.C),
    },
  };
}

function normalizeLayoutValue(value: unknown): unknown | null {
  return value && typeof value === "object" && !Array.isArray(value) ? value : null;
}

function createEmptyRestoreState(): AppWorkspaceUiRestoreState {
  return {
    version: 5,
    layouts: {
      A: null,
      B: null,
      C: null,
    },
  };
}
