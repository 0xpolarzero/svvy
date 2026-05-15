import { describe, expect, it } from "bun:test";
import type { SerializedDockview } from "dockview-core";
import {
  createEmptyPaneLayout,
  getSidebarSessionOpenTarget,
  normalizePaneLayout,
  splitPane,
} from "./pane-layout";

describe("getSidebarSessionOpenTarget", () => {
  it("opens normal sidebar session clicks in a new right pane", () => {
    expect(getSidebarSessionOpenTarget({ metaKey: false })).toEqual({
      kind: "new-panel",
      direction: "right",
    });
  });

  it("opens command-clicked sidebar sessions in the focused pane", () => {
    expect(getSidebarSessionOpenTarget({ metaKey: true })).toEqual({
      kind: "focused-panel",
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
            inspectorSelection: null,
            scroll: null,
            timelineDensity: "comfortable",
          },
        },
        {
          panelId: "logs",
          binding: { surface: "app-logs" },
          localState: {
            inspectorSelection: null,
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
            inspectorSelection: null,
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
            inspectorSelection: null,
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
});
