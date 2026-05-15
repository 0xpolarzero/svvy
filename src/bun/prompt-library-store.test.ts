import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, describe, expect, it } from "bun:test";
import { buildSystemPrompt, createDefaultPromptLibraryState } from "./default-system-prompt";
import { createPromptLibraryStore } from "./prompt-library-store";

const tempDirs: string[] = [];

function createTempAgentDir(): string {
  const dir = mkdtempSync(join(tmpdir(), "svvy-prompt-library-"));
  tempDirs.push(dir);
  return dir;
}

afterEach(() => {
  for (const dir of tempDirs.splice(0)) {
    rmSync(dir, { recursive: true, force: true });
  }
});

describe("prompt library store", () => {
  it("seeds app-wide JSON with the current default prompt composition", () => {
    const store = createPromptLibraryStore({ agentDir: createTempAgentDir() });
    const state = store.getState();

    expect(state.revision).toBe(1);
    expect(buildSystemPrompt("orchestrator", { promptLibraryState: state })).toBe(
      buildSystemPrompt("orchestrator"),
    );
    expect(
      buildSystemPrompt("handler", {
        loadedContextKeys: ["ci"],
        promptLibraryState: state,
      }),
    ).toBe(buildSystemPrompt("handler", { loadedContextKeys: ["ci"] }));
  });

  it("persists updates and reset restores the seeded default", () => {
    const agentDir = createTempAgentDir();
    const store = createPromptLibraryStore({ agentDir });
    const state = store.getState();
    const updated = structuredClone(state);
    updated.instructionBlocks.common!.body = "Use the persisted prompt library.";

    const saved = store.updateState(updated);
    expect(saved.revision).toBe(2);
    expect(buildSystemPrompt("orchestrator", { promptLibraryState: saved })).toContain(
      "Use the persisted prompt library.",
    );

    const reloaded = createPromptLibraryStore({ agentDir });
    expect(reloaded.getState().instructionBlocks.common!.body).toBe(
      "Use the persisted prompt library.",
    );

    const reset = store.resetState();
    expect(reset).toEqual(createDefaultPromptLibraryState(reset.updatedAt, saved.revision + 1));
    expect(buildSystemPrompt("orchestrator", { promptLibraryState: reset })).toBe(
      buildSystemPrompt("orchestrator"),
    );
  });

  it("creates, renames, and restores named snapshots", () => {
    const agentDir = createTempAgentDir();
    const store = createPromptLibraryStore({ agentDir });
    const initial = store.getState();
    const changed = structuredClone(initial);
    changed.instructionBlocks.common!.body = "Snapshot this instruction.";
    const saved = store.updateState(changed);

    const snapshot = store.createSnapshot("Stable prompt");
    expect(snapshot.name).toBe("Stable prompt");
    expect(snapshot.revision).toBe(saved.revision);
    expect(store.listSnapshots()).toEqual([snapshot]);

    const renamed = store.renameSnapshot(snapshot.id, "Release prompt");
    expect(renamed.name).toBe("Release prompt");
    expect(store.listSnapshots()[0]?.name).toBe("Release prompt");

    const next = structuredClone(saved);
    next.instructionBlocks.common!.body = "A later autosaved edit.";
    store.updateState(next);

    const restored = store.restoreSnapshot(snapshot.id);
    expect(restored.revision).toBe(4);
    expect(restored.instructionBlocks.common!.body).toBe("Snapshot this instruction.");

    const reloaded = createPromptLibraryStore({ agentDir });
    expect(reloaded.listSnapshots()[0]?.name).toBe("Release prompt");
    expect(reloaded.getState().instructionBlocks.common!.body).toBe("Snapshot this instruction.");
  });
});
