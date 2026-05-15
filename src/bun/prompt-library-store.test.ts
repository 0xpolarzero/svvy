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
});
