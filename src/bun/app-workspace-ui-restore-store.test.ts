import { mkdtempSync, realpathSync, rmSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { afterEach, describe, expect, test } from "bun:test";
import { createAppWorkspaceUiRestoreStore } from "./app-workspace-ui-restore-store";

const tempDirs: string[] = [];

afterEach(() => {
  for (const dir of tempDirs.splice(0)) {
    rmSync(dir, { recursive: true, force: true });
  }
});

describe("app workspace UI restore store", () => {
  test("persists view-local restore state by workspace tab id", () => {
    const agentDir = tempAgentDir();
    const workspaceId = "/repo/app#workspace-1";
    const state = {
      version: 4 as const,
      activeLayoutId: "B" as const,
      layouts: {
        A: null,
        B: {
          panels: [{ panelId: "primary", binding: { surface: "saved-workflow-library" } }],
          dockview: null,
          compactSurfaces: [],
          focusedPanelId: "primary",
          updatedAt: "2026-01-01T00:00:00.000Z",
        },
        C: null,
      },
    };

    createAppWorkspaceUiRestoreStore({ agentDir }).setState(workspaceId, state);

    const reloaded = createAppWorkspaceUiRestoreStore({ agentDir });
    expect(reloaded.getState(workspaceId)).toEqual(state);
  });

  test("keeps view-local states for duplicate same-cwd tabs separate", () => {
    const store = createAppWorkspaceUiRestoreStore({ agentDir: tempAgentDir() });
    const cwd = "/repo/app";
    const first = `${cwd}#first`;
    const second = `${cwd}#second`;

    store.setState(first, {
      version: 4,
      activeLayoutId: "A",
      layouts: { A: { focusedPanelId: "primary" }, B: null, C: null },
    });
    store.setState(second, {
      version: 4,
      activeLayoutId: "C",
      layouts: { A: null, B: null, C: { focusedPanelId: "logs" } },
    });

    expect(store.getState(first)?.activeLayoutId).toBe("A");
    expect(store.getState(second)?.activeLayoutId).toBe("C");
  });

  test("ignores malformed saved data", () => {
    const store = createAppWorkspaceUiRestoreStore({ agentDir: tempAgentDir() });

    expect(
      store.setState("/repo/app#bad", {
        version: 999,
        activeLayoutId: "A",
        layouts: { A: null, B: null, C: null },
      } as never),
    ).toEqual({
      version: 4,
      activeLayoutId: "A",
      layouts: { A: null, B: null, C: null },
    });
  });
});

function tempAgentDir(): string {
  const dir = realpathSync.native(mkdtempSync(join(tmpdir(), "svvy-ui-restore-")));
  tempDirs.push(dir);
  return dir;
}
