import { afterEach, describe, expect, it } from "bun:test";
import { mkdtempSync, realpathSync, rmSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { SessionManager } from "@mariozechner/pi-coding-agent";
import { WorkspaceRuntimeRegistry } from "./workspace-runtime-registry";
import { getSvvySessionDir } from "./session-catalog";

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
  it("does not open the initial cwd unless startup opening is requested", () => {
    const cwd = tempWorkspace("no-startup-open");
    const registry = createRegistry(cwd);

    expect(registry.listOpenWorkspaces()).toEqual([]);
    expect(registry.getActiveWorkspaceId()).toBeNull();
  });

  it("opens the initial cwd when startup opening is requested", () => {
    const cwd = tempWorkspace("startup-open");
    const registry = createRegistry(cwd, tempWorkspace("agent-dir"), {
      openInitialWorkspace: true,
    });

    const [workspace] = registry.listOpenWorkspaces();

    expect(workspace).toBeDefined();
    if (!workspace) throw new Error("Expected startup workspace to open.");

    expect(workspace?.cwd).toBe(realpathSync.native(cwd));
    expect(registry.getActiveWorkspaceId()).toBe(workspace.workspaceId);
  });

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

  it("reopens a persisted workspace tab with the saved runtime id", () => {
    const cwd = tempWorkspace("persisted-runtime-id");
    const registry = createRegistry(cwd);
    const workspaceId = `${realpathSync.native(cwd)}#saved-tab`;

    const restored = registry.openWorkspace(cwd, { workspaceId });

    expect(restored.workspaceId).toBe(workspaceId);
    expect(registry.getRuntime(workspaceId).cwd).toBe(realpathSync.native(cwd));
  });

  it("ignores a mismatched persisted workspace id for a different cwd", () => {
    const cwd = tempWorkspace("mismatched-runtime-id");
    const otherCwd = tempWorkspace("other-runtime-id");
    const registry = createRegistry(cwd);

    const restored = registry.openWorkspace(cwd, {
      workspaceId: `${realpathSync.native(otherCwd)}#saved-tab`,
    });

    expect(restored.workspaceId).toStartWith(`${realpathSync.native(cwd)}#`);
    expect(restored.workspaceId).not.toContain("saved-tab");
  });

  it("shares durable session storage across separate runtime ids for the same cwd", async () => {
    const cwd = tempWorkspace("shared-session-cwd");
    const agentDir = tempWorkspace("agent-dir");
    const sessionManager = SessionManager.create(
      cwd,
      getSvvySessionDir(realpathSync.native(cwd), agentDir),
    );
    sessionManager.appendSessionInfo("Persistent Session");
    sessionManager.appendMessage({
      role: "user",
      timestamp: Date.now(),
      content: [{ type: "text", text: "Remember this session" }],
    });
    sessionManager.appendMessage({
      role: "assistant",
      timestamp: Date.now(),
      api: "openai-responses",
      content: [{ type: "text", text: "Remembered." }],
      provider: "openai",
      model: "gpt-4o",
      usage: {
        input: 0,
        output: 0,
        cacheRead: 0,
        cacheWrite: 0,
        totalTokens: 0,
        cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0, total: 0 },
      },
      stopReason: "stop",
    });
    const registry = createRegistry(cwd, agentDir);

    const first = registry.openWorkspace(cwd);
    const firstListed = await first.catalog.listSessions();
    await registry.closeWorkspace(first.workspaceId);

    const second = registry.openWorkspace(join(cwd, "."));
    const listed = await second.catalog.listSessions();

    expect(second.workspaceId).not.toBe(first.workspaceId);
    expect(firstListed.sessions.map((session) => session.id)).toContain(
      sessionManager.getSessionId(),
    );
    expect(listed.sessions.map((session) => session.id)).toContain(sessionManager.getSessionId());
  });
});

function createRegistry(
  initialCwd: string,
  agentDir = tempWorkspace("agent-dir"),
  options: { openInitialWorkspace?: boolean } = {},
): WorkspaceRuntimeRegistry {
  const registry = new WorkspaceRuntimeRegistry({
    initialCwd,
    agentDir,
    ...options,
  });
  registries.push(registry);
  return registry;
}

function tempWorkspace(name: string): string {
  const dir = mkdtempSync(join(tmpdir(), `svvy-${name}-`));
  tempDirs.push(dir);
  return dir;
}
