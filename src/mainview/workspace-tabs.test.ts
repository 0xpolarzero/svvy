import { describe, expect, it } from "bun:test";
import {
  formatWorkspaceTabAriaLabel,
  getVisibleWorkspaceTabCounts,
  reorderWorkspaceTabs,
  type WorkspaceTabCounts,
} from "./workspace-tabs";

const workspace = { workspaceLabel: "svvy" };

describe("workspace tab counts", () => {
  it("hides zero counts", () => {
    const counts: WorkspaceTabCounts = {
      running: 0,
      unread: 0,
      waiting: 0,
      error: 0,
    };

    expect(getVisibleWorkspaceTabCounts(counts)).toEqual([]);
    expect(formatWorkspaceTabAriaLabel(workspace, counts)).toBe("svvy");
  });

  it("shows positive counts as number-only status badges in stable order", () => {
    const counts: WorkspaceTabCounts = {
      running: 2,
      unread: 5,
      waiting: 1,
      error: 3,
    };

    expect(getVisibleWorkspaceTabCounts(counts)).toEqual([
      { kind: "running", value: 2, label: "running" },
      { kind: "unread", value: 5, label: "unread" },
      { kind: "waiting", value: 1, label: "waiting" },
      { kind: "error", value: 3, label: "errors" },
    ]);
    expect(formatWorkspaceTabAriaLabel(workspace, counts)).toBe(
      "svvy. 2 running, 5 unread, 1 waiting, 3 errors",
    );
  });
});

describe("workspace tab reorder", () => {
  const tabs = [
    { workspace: { workspaceId: "workspace-a", cwd: "/same", workspaceLabel: "same" } },
    { workspace: { workspaceId: "workspace-b", cwd: "/same", workspaceLabel: "same" } },
    { workspace: { workspaceId: "workspace-c", cwd: "/other", workspaceLabel: "other" } },
  ];

  const ids = (items: typeof tabs): string[] => items.map((tab) => tab.workspace.workspaceId);

  it("moves a later tab before an earlier tab", () => {
    expect(ids(reorderWorkspaceTabs(tabs, "workspace-c", "workspace-a"))).toEqual([
      "workspace-c",
      "workspace-a",
      "workspace-b",
    ]);
  });

  it("moves an earlier tab before a later tab", () => {
    expect(ids(reorderWorkspaceTabs(tabs, "workspace-a", "workspace-c"))).toEqual([
      "workspace-b",
      "workspace-a",
      "workspace-c",
    ]);
  });

  it("moves a tab to the end when no target is provided", () => {
    expect(ids(reorderWorkspaceTabs(tabs, "workspace-a", null))).toEqual([
      "workspace-b",
      "workspace-c",
      "workspace-a",
    ]);
  });

  it("keeps order when ids are missing or unchanged", () => {
    expect(ids(reorderWorkspaceTabs(tabs, "workspace-a", "workspace-a"))).toEqual([
      "workspace-a",
      "workspace-b",
      "workspace-c",
    ]);
    expect(ids(reorderWorkspaceTabs(tabs, "missing", "workspace-a"))).toEqual([
      "workspace-a",
      "workspace-b",
      "workspace-c",
    ]);
    expect(ids(reorderWorkspaceTabs(tabs, "workspace-a", "missing"))).toEqual([
      "workspace-b",
      "workspace-c",
      "workspace-a",
    ]);
  });
});
