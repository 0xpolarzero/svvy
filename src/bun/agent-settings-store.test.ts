import { mkdtempSync, readFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, expect, it } from "bun:test";
import { createAgentSettingsStore } from "./agent-settings-store";
import {
  DEFAULT_ORCHESTRATOR_PROFILE_ID,
  DEFAULT_THREAD_HANDLER_PROFILE_ID,
} from "../shared/agent-settings";

describe("agent profile settings", () => {
  it("persists agent profile state and seeds conventional workflow agents", () => {
    const root = mkdtempSync(join(tmpdir(), "svvy-agent-settings-"));
    const store = createAgentSettingsStore({
      cwd: root,
      agentDir: join(root, ".agent"),
    });

    const updated = store.setAgentProfile({
      id: "repo-orchestrator",
      kind: "orchestrator",
      name: "Repo orchestrator",
      provider: "openai",
      model: "gpt-5.4-mini",
      reasoningEffort: "low",
      systemPrompt: "Own repository strategy.",
      extensions: [],
      updateFromComposer: false,
      builtin: false,
      locked: false,
    });

    expect(updated.agents.orchestrators.map((agent) => agent.id)).toContain(
      DEFAULT_ORCHESTRATOR_PROFILE_ID,
    );
    expect(updated.agents.orchestrators).toContainEqual(
      expect.objectContaining({
        id: "repo-orchestrator",
        provider: "openai",
        model: "gpt-5.4-mini",
        reasoningEffort: "low",
        systemPrompt: "Own repository strategy.",
      }),
    );
    expect(updated.agents.special.threadHandler.id).toBe(DEFAULT_THREAD_HANDLER_PROFILE_ID);
    expect(updated.agents.titleNamer.provider).toBe("openai-codex");
    expect(updated.agents.titleNamer.model).toBe("gpt-5.4-mini");
    expect(updated.agents.titleNamer.reasoningEffort).toBe("low");

    const agentsPath = store.ensureWorkflowAgentsComponent();
    const agentsSource = readFileSync(agentsPath, "utf8");
    expect(agentsSource).toContain("@svvyAssetKind component");
    expect(agentsSource).toContain("export const explorer");
    expect(agentsSource).toContain("export const implementer");
    expect(agentsSource).toContain("export const reviewer");
    expect(agentsSource).toContain('"toolSurface": [');
    expect(agentsSource).toContain('"cx_overview"');
    expect(agentsSource).toContain('"execute_typescript"');
  });

  it("updates the locked thread handler profile through the special profile slot", () => {
    const root = mkdtempSync(join(tmpdir(), "svvy-thread-handler-settings-"));
    const store = createAgentSettingsStore({
      cwd: root,
      agentDir: join(root, ".agent"),
    });

    const updated = store.setAgentProfile({
      ...store.getState().agents.special.threadHandler,
      provider: "anthropic",
      model: "claude-sonnet-4",
      reasoningEffort: "high",
      systemPrompt: "Supervise delegated workflow work.",
    });

    expect(updated.agents.special.threadHandler).toEqual(
      expect.objectContaining({
        id: DEFAULT_THREAD_HANDLER_PROFILE_ID,
        provider: "anthropic",
        model: "claude-sonnet-4",
        reasoningEffort: "high",
        systemPrompt: "Supervise delegated workflow work.",
        locked: true,
      }),
    );
  });

  it("allows settings edits on the locked default orchestrator while preserving lock policy", () => {
    const root = mkdtempSync(join(tmpdir(), "svvy-default-orchestrator-settings-"));
    const store = createAgentSettingsStore({
      cwd: root,
      agentDir: join(root, ".agent"),
    });

    const updated = store.setAgentProfile({
      ...store.getState().agents.orchestrators[0]!,
      name: "Default strategy",
      provider: "anthropic",
      model: "claude-sonnet-4",
      reasoningEffort: "high",
      systemPrompt: "Own the top-level plan.",
      locked: false,
      builtin: false,
    });

    expect(updated.agents.orchestrators[0]).toEqual(
      expect.objectContaining({
        id: DEFAULT_ORCHESTRATOR_PROFILE_ID,
        name: "Default strategy",
        provider: "anthropic",
        model: "claude-sonnet-4",
        reasoningEffort: "high",
        systemPrompt: "Own the top-level plan.",
        locked: true,
        builtin: true,
      }),
    );
  });

  it("creates additional orchestrator profiles as unlocked user profiles", () => {
    const root = mkdtempSync(join(tmpdir(), "svvy-new-orchestrator-settings-"));
    const store = createAgentSettingsStore({
      cwd: root,
      agentDir: join(root, ".agent"),
    });

    const updated = store.setAgentProfile({
      id: "custom-orchestrator",
      kind: "orchestrator",
      name: "Custom orchestrator",
      provider: "openai",
      model: "gpt-5.4",
      reasoningEffort: "medium",
      systemPrompt: "Own custom strategy.",
      extensions: [],
      updateFromComposer: false,
      builtin: true,
      locked: true,
    });

    expect(updated.agents.orchestrators).toContainEqual(
      expect.objectContaining({
        id: "custom-orchestrator",
        builtin: false,
        locked: false,
      }),
    );
  });

  it("keeps workflow agent settings synchronized with agents.ts", () => {
    const root = mkdtempSync(join(tmpdir(), "svvy-workflow-agent-settings-"));
    const store = createAgentSettingsStore({
      cwd: root,
      agentDir: join(root, ".agent"),
    });

    store.setWorkflowAgent("reviewer", {
      id: "reviewer",
      label: "Reviewer",
      provider: "anthropic",
      model: "claude-sonnet-4",
      reasoningEffort: "high",
      systemPrompt: "Review strictly.",
      toolSurface: ["execute_typescript"],
    });

    const agentsSource = readFileSync(
      join(root, ".svvy", "workflows", "components", "agents.ts"),
      "utf8",
    );
    expect(agentsSource).toContain('"provider": "anthropic"');
    expect(agentsSource).toContain('"model": "claude-sonnet-4"');
    expect(agentsSource).toContain('"reasoningEffort": "high"');
    expect(store.getState().workflowAgents.reviewer.systemPrompt).toBe("Review strictly.");
  });

  it("preserves canonical cx tools when normalizing workflow agent settings", () => {
    const root = mkdtempSync(join(tmpdir(), "svvy-workflow-agent-cx-settings-"));
    const store = createAgentSettingsStore({
      cwd: root,
      agentDir: join(root, ".agent"),
    });

    const updated = store.setWorkflowAgent("explorer", {
      id: "explorer",
      label: "Explorer",
      provider: "openai",
      model: "gpt-5.4",
      reasoningEffort: "medium",
      systemPrompt: "Explore.",
      toolSurface: ["cx_overview", "cx_symbols", "cx_cache_path", "execute_typescript"],
    });

    expect(updated.workflowAgents.explorer.toolSurface).toEqual([
      "cx_overview",
      "cx_symbols",
      "cx_cache_path",
      "execute_typescript",
    ]);
  });

  it("persists preferred external editor preferences", () => {
    const root = mkdtempSync(join(tmpdir(), "svvy-editor-settings-"));
    const store = createAgentSettingsStore({
      cwd: root,
      agentDir: join(root, ".agent"),
    });

    const updated = store.setAppPreferences({
      appAppearance: "dark",
      preferredExternalEditor: "custom",
      customExternalEditorCommand: "code --reuse-window",
      webProvider: "tinyfish",
    });

    expect(updated.appPreferences).toEqual({
      appAppearance: "dark",
      preferredExternalEditor: "custom",
      customExternalEditorCommand: "code --reuse-window",
      webProvider: "tinyfish",
    });
    expect(store.getState().appPreferences.preferredExternalEditor).toBe("custom");
  });

  it("defaults invalid appearance preferences to system", () => {
    const root = mkdtempSync(join(tmpdir(), "svvy-appearance-settings-"));
    const store = createAgentSettingsStore({
      cwd: root,
      agentDir: join(root, ".agent"),
    });

    const updated = store.setAppPreferences({
      appAppearance: "invalid" as never,
      preferredExternalEditor: "system",
      customExternalEditorCommand: "",
      webProvider: null,
    });

    expect(updated.appPreferences.appAppearance).toBe("system");
  });
});
