import { describe, expect, it } from "bun:test";
import type { SerializedDockview } from "dockview-core";
import {
  createPanelChrome,
  createEmptyPaneLayout,
  getSidebarSessionOpenTarget,
  normalizePaneLayout,
  removeDockviewPanel,
  splitPane,
} from "./pane-layout";

describe("getSidebarSessionOpenTarget", () => {
  it("opens normal sidebar session clicks in the focused pane", () => {
    expect(getSidebarSessionOpenTarget({ metaKey: false })).toEqual({
      kind: "focused-panel",
    });
  });

  it("opens command-clicked sidebar sessions in a new right pane", () => {
    expect(getSidebarSessionOpenTarget({ metaKey: true })).toEqual({
      kind: "new-panel",
      direction: "right",
    });
  });
});

describe("createPanelChrome", () => {
  it("labels library panes with the current sidebar names", () => {
    expect(createPanelChrome({ surface: "saved-workflow-library" })).toMatchObject({
      title: "Workflows",
      kind: "saved-workflow-library",
    });
    expect(createPanelChrome({ surface: "prompt-library" })).toMatchObject({
      title: "Context",
      kind: "prompt-library",
    });
  });
});

describe("pane layout normalization", () => {
  it("represents no panes without creating a visible empty pane", () => {
    expect(createEmptyPaneLayout()).toMatchObject({
      panels: [],
      focusedPanelId: null,
      dockview: null,
    });
  });

  it("drops restored panes without a surface binding", () => {
    const layout = normalizePaneLayout({
      dockview: null,
      panels: [
        {
          panelId: "empty",
          binding: null,
          localState: {
            scroll: null,
            timelineDensity: "comfortable",
          },
        },
        {
          panelId: "logs",
          binding: { surface: "app-logs" },
          localState: {
            scroll: null,
            timelineDensity: "comfortable",
          },
        },
      ],
      compactSurfaces: [],
      focusedPanelId: "empty",
      updatedAt: "2026-05-15T00:00:00.000Z",
    });

    expect(layout.panels.map((panel) => panel.panelId)).toEqual(["logs"]);
    expect(layout.focusedPanelId).toBe("logs");
  });

  it("drops restored prompt panes without a valid surface target", () => {
    const layout = normalizePaneLayout({
      dockview: null,
      panels: [
        {
          panelId: "invalid-orchestrator",
          binding: {
            surface: "orchestrator",
            workspaceSessionId: "session-1",
          } as never,
          localState: {
            scroll: null,
            timelineDensity: "comfortable",
          },
        },
        {
          panelId: "invalid-thread",
          binding: {
            surface: "thread",
            workspaceSessionId: "session-1",
            surfacePiSessionId: "thread-session-1",
          } as never,
          localState: {
            scroll: null,
            timelineDensity: "comfortable",
          },
        },
        {
          panelId: "logs",
          binding: { surface: "app-logs" },
          localState: {
            scroll: null,
            timelineDensity: "comfortable",
          },
        },
      ],
      compactSurfaces: [],
      focusedPanelId: "invalid-orchestrator",
      updatedAt: "2026-05-15T00:00:00.000Z",
    });

    expect(layout.panels.map((panel) => panel.panelId)).toEqual(["logs"]);
    expect(layout.focusedPanelId).toBe("logs");
  });

  it("drops serialized Dockview geometry that references panels outside the svvy pane bindings", () => {
    const layout = normalizePaneLayout({
      dockview: {
        grid: { root: { type: "leaf", data: { views: ["logs", "stale"] } } },
        panels: {
          logs: {},
          stale: {},
        },
      } as unknown as SerializedDockview,
      panels: [
        {
          panelId: "logs",
          binding: { surface: "app-logs" },
          localState: {
            scroll: null,
            timelineDensity: "comfortable",
          },
        },
      ],
      compactSurfaces: [],
      focusedPanelId: "logs",
      updatedAt: "2026-05-15T00:00:00.000Z",
    });

    expect(layout.dockview).toBeNull();
  });

  it("does not split into an unbound pane", () => {
    const layout = normalizePaneLayout({
      dockview: null,
      panels: [
        {
          panelId: "logs",
          binding: { surface: "app-logs" },
          localState: {
            scroll: null,
            timelineDensity: "comfortable",
          },
        },
      ],
      compactSurfaces: [],
      focusedPanelId: "logs",
      updatedAt: "2026-05-15T00:00:00.000Z",
    });

    expect(splitPane(layout, "logs", "right").panels).toHaveLength(1);
    expect(splitPane(layout, "logs", "right", { duplicateBinding: true }).panels).toHaveLength(2);
  });

  it("drops serialized Dockview geometry when a pane is removed by runtime state", () => {
    const layout = normalizePaneLayout({
      dockview: {
        grid: { root: { type: "leaf", data: { views: ["primary", "logs"] } } },
        panels: {
          primary: {},
          logs: {},
        },
      } as unknown as SerializedDockview,
      panels: [
        {
          panelId: "primary",
          binding: {
            workspaceSessionId: "session-1",
            surface: "orchestrator",
            surfacePiSessionId: "session-1",
          },
          localState: {
            scroll: null,
            timelineDensity: "comfortable",
          },
        },
        {
          panelId: "logs",
          binding: { surface: "app-logs" },
          localState: {
            scroll: null,
            timelineDensity: "comfortable",
          },
        },
      ],
      compactSurfaces: [],
      focusedPanelId: "primary",
      updatedAt: "2026-05-15T00:00:00.000Z",
    });

    const next = removeDockviewPanel(layout, "primary");

    expect(next.panels.map((panel) => panel.panelId)).toEqual(["logs"]);
    expect(next.dockview).toBeNull();
    expect(next.focusedPanelId).toBe("logs");
  });
});
