import { afterEach, describe, expect, it } from "bun:test";
import { mkdtempSync, rmSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { createAppWorkspaceTabsStore } from "./app-workspace-tabs-store";

const tempDirs: string[] = [];

afterEach(() => {
  for (const dir of tempDirs.splice(0)) {
    rmSync(dir, { recursive: true, force: true });
  }
});

describe("app workspace tabs store", () => {
  it("persists open workspace tabs across store instances", () => {
    const agentDir = tempAgentDir();
    const store = createAppWorkspaceTabsStore({ agentDir });

    store.setState({
      version: 3,
      activeWorkspaceId: "workspace-b",
      tabs: [
        {
          workspaceId: "workspace-a",
          cwd: "/tmp/workspace-a",
          workspaceLabel: "workspace-a",
          openedAt: "2026-05-15T12:00:00.000Z",
        },
        {
          workspaceId: "workspace-b",
          cwd: "/tmp/workspace-b",
          workspaceLabel: "workspace-b",
          openedAt: "2026-05-15T12:01:00.000Z",
        },
      ],
      knownWorkspaces: [
        {
          workspaceId: "workspace-a",
          cwd: "/tmp/workspace-a",
          workspaceLabel: "workspace-a",
          openedAt: "2026-05-15T12:00:00.000Z",
        },
      ],
    });

    const reloaded = createAppWorkspaceTabsStore({ agentDir });

    expect(reloaded.getState()?.tabs.map((tab) => tab.cwd)).toEqual([
      "/tmp/workspace-a",
      "/tmp/workspace-b",
    ]);
    expect(reloaded.getState()?.activeWorkspaceId).toBe("workspace-b");
  });

  it("drops an active workspace id that is not in the open tab list", () => {
    const store = createAppWorkspaceTabsStore({ agentDir: tempAgentDir() });

    const state = store.setState({
      version: 3,
      activeWorkspaceId: "missing",
      tabs: [],
      knownWorkspaces: [],
    });

    expect(state.activeWorkspaceId).toBeNull();
  });
});

function tempAgentDir(): string {
  const dir = mkdtempSync(join(tmpdir(), "svvy-app-tabs-"));
  tempDirs.push(dir);
  return dir;
}
