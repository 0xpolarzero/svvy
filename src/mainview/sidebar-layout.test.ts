import { describe, expect, it } from "bun:test";
import {
  clampSidebarWidth,
  getMaxSidebarWidth,
  MAX_SIDEBAR_WIDTH,
  MIN_SIDEBAR_WIDTH,
} from "./sidebar-layout";
import { getShortcut } from "../shared/shortcut-registry";

describe("getMaxSidebarWidth", () => {
  it("caps the sidebar against the viewport ratio on narrow windows", () => {
    expect(getMaxSidebarWidth(700)).toBe(294);
  });

  it("never grows beyond the desktop max width", () => {
    expect(getMaxSidebarWidth(1800)).toBe(MAX_SIDEBAR_WIDTH);
  });
});

describe("clampSidebarWidth", () => {
  it("does not allow widths below the minimum", () => {
    expect(clampSidebarWidth(120, 1400)).toBe(MIN_SIDEBAR_WIDTH);
  });

  it("does not allow widths above the viewport-aware maximum", () => {
    expect(clampSidebarWidth(900, 760)).toBe(getMaxSidebarWidth(760));
  });
});

describe("sidebar shortcut registry", () => {
  it("declares the shell-scoped sidebar toggle chord", () => {
    expect(getShortcut("sidebar.toggle")).toMatchObject({
      hotkey: "Mod+B",
      readableShortcut: "Cmd+B",
      compactShortcut: "⌘B",
      scope: "workspace-shell",
      inputPolicy: "allow-while-typing",
    });
  });
});
