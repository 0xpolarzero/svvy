import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import type {
  AppWorkspaceUiRestoreState,
  WorkspaceLayoutSlotId,
} from "../shared/workspace-contract";

const APP_WORKSPACE_UI_RESTORE_FILENAME = "app-workspace-ui-restore.json";
const WORKSPACE_LAYOUT_SLOT_IDS: WorkspaceLayoutSlotId[] = ["A", "B", "C"];

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
    getState: (workspaceId) => readAll()[workspaceId] ?? null,
    setState: (workspaceId, state) => {
      const normalized = normalizeWorkspaceUiRestoreState(state) ?? createEmptyRestoreState();
      writeAll({
        ...readAll(),
        [workspaceId]: normalized,
      });
      return normalized;
    },
    getPath: () => statePath,
  };
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
    raw.version !== 4 ||
    !isWorkspaceLayoutSlotId(raw.activeLayoutId) ||
    !raw.layouts ||
    typeof raw.layouts !== "object" ||
    Array.isArray(raw.layouts)
  ) {
    return null;
  }

  return {
    version: 4,
    activeLayoutId: raw.activeLayoutId,
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

function isWorkspaceLayoutSlotId(value: unknown): value is WorkspaceLayoutSlotId {
  return WORKSPACE_LAYOUT_SLOT_IDS.includes(value as WorkspaceLayoutSlotId);
}

function createEmptyRestoreState(): AppWorkspaceUiRestoreState {
  return {
    version: 4,
    activeLayoutId: "A",
    layouts: {
      A: null,
      B: null,
      C: null,
    },
  };
}
