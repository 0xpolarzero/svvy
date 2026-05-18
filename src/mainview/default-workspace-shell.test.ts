import { describe, expect, it } from "bun:test";
import { readFile } from "node:fs/promises";

describe("default workspace renderer shell", () => {
  it("renders the Open Workspace surface as a workbench panel", async () => {
    const panelSource = await readFile(
      new URL("./OpenWorkspacePanel.svelte", import.meta.url),
      "utf8",
    );
    const dockviewHostSource = await readFile(
      new URL("./DockviewPanelHost.svelte", import.meta.url),
      "utf8",
    );

    expect(panelSource).toContain("Open Workspace");
    expect(panelSource).toContain("Choose a local repository or folder to work in.");
    expect(panelSource).toContain("Open in New Tab");
    expect(dockviewHostSource).toContain("OpenWorkspacePanel");
    expect(dockviewHostSource).toContain('surface === "open-workspace"');
  });

  it("does not keep the removed standalone no-workspace picker page", async () => {
    const appSource = await readFile(new URL("./App.svelte", import.meta.url), "utf8");

    expect(appSource).not.toContain('class="workspace-picker"');
    expect(appSource).not.toContain("workspace-picker-button");
    expect(appSource).not.toContain("No workspace is open.");
    expect(appSource).toContain("activeWorkspaceTabId");
    expect(appSource).toContain("workspaceTabId");
  });

  it("routes workspace opening commands to current-tab and new-tab flows", async () => {
    const appSource = await readFile(new URL("./App.svelte", import.meta.url), "utf8");
    const workspaceSource = await readFile(
      new URL("./ChatWorkspace.svelte", import.meta.url),
      "utf8",
    );

    expect(appSource).toContain('placement: "current-tab"');
    expect(appSource).toContain('placement: "new-tab"');
    expect(appSource).toContain("createDefaultWorkspaceTab");
    expect(workspaceSource).toContain('workspaceAction === "open"');
    expect(workspaceSource).toContain('workspaceAction === "new-tab"');
    expect(workspaceSource).toContain('workspaceAction === "open-in-new-tab"');
  });

  it("renders a static workspace footer when the workspace is not a git repo", async () => {
    const sidebarSource = await readFile(
      new URL("./SessionSidebar.svelte", import.meta.url),
      "utf8",
    );

    expect(sidebarSource).toContain("{#if branchControlEnabled}");
    expect(sidebarSource).toContain('<Tooltip label="Switch branch">');
    expect(sidebarSource).toContain('class="workspace-path-static"');
    expect(sidebarSource).toContain('aria-label="Workspace"');
    expect(sidebarSource).not.toContain('label={footerShowsBranch ? "Switch branch"');
    expect(sidebarSource).not.toContain("showCaret");
  });

  it("refreshes existing Dockview panel content when a pane changes surface", async () => {
    const dockviewSource = await readFile(
      new URL("./DockviewWorkspace.svelte", import.meta.url),
      "utf8",
    );

    expect(dockviewSource).toContain("getPanelRenderKey");
    expect(dockviewSource).toContain("existingPanel.update");
    expect(dockviewSource).toContain("existingPanel.setRenderer");
  });

  it("remounts workspace shell state for each active workspace tab", async () => {
    const appSource = await readFile(new URL("./App.svelte", import.meta.url), "utf8");
    const panelHostSource = await readFile(
      new URL("./DockviewPanelHost.svelte", import.meta.url),
      "utf8",
    );

    expect(appSource).toContain(
      "{#key `${activeTab.workspace.workspaceTabId}:${activeTab.workspace.workspaceId}`}",
    );
    expect(panelHostSource).not.toContain("Surface unavailable");
  });

  it("mutes layout slot controls for the default workspace", async () => {
    const runtimeSource = await readFile(new URL("./chat-runtime.ts", import.meta.url), "utf8");
    const workspaceSource = await readFile(
      new URL("./ChatWorkspace.svelte", import.meta.url),
      "utf8",
    );

    expect(runtimeSource).toContain(
      'const durableLayoutEnabled = workspaceInfo.kind !== "default"',
    );
    expect(runtimeSource).toContain("get layoutSlotsEnabled()");
    expect(workspaceSource).toContain("disabled={!layoutSlotsEnabled}");
    expect(workspaceSource).toContain("Layout slots are unavailable in the default workspace");
  });
});
