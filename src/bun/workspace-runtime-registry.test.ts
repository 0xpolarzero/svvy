import { afterEach, describe, expect, it } from "bun:test";
import { mkdtempSync, realpathSync, rmSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { WorkspaceRuntimeRegistry } from "./workspace-runtime-registry";

const tempDirs: string[] = [];
const registries: WorkspaceRuntimeRegistry[] = [];

afterEach(async () => {
  for (const registry of registries.splice(0)) {
    await Promise.all(
      registry
        .listOpenWorkspaces()
        .map((workspace) => registry.closeWorkspace(workspace.workspaceId)),
    );
  }
  for (const dir of tempDirs.splice(0)) {
    rmSync(dir, { recursive: true, force: true });
  }
});

describe("WorkspaceRuntimeRegistry", () => {
  it("opens the same cwd as separate workspace runtime tabs", () => {
    const cwd = tempWorkspace("duplicate-cwd");
    const registry = createRegistry(cwd);

    const first = registry.openWorkspace(cwd);
    const second = registry.openWorkspace(join(cwd, "."));

    expect(first.cwd).toBe(realpathSync.native(cwd));
    expect(second.cwd).toBe(first.cwd);
    expect(second.workspaceId).not.toBe(first.workspaceId);
    expect(registry.listOpenWorkspaces().map((workspace) => workspace.workspaceId)).toEqual([
      first.workspaceId,
      second.workspaceId,
    ]);
  });
});

function createRegistry(initialCwd: string): WorkspaceRuntimeRegistry {
  const registry = new WorkspaceRuntimeRegistry({
    initialCwd,
    agentDir: tempWorkspace("agent-dir"),
  });
  registries.push(registry);
  return registry;
}

function tempWorkspace(name: string): string {
  const dir = mkdtempSync(join(tmpdir(), `svvy-${name}-`));
  tempDirs.push(dir);
  return dir;
}
