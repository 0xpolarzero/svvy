import { describe, expect, it } from "bun:test";
import {
  CONSTRAINED_DESKTOP_MAX_WIDTH,
  DESKTOP_SPLIT_BREAKPOINT,
  getViewportClass,
  isSidebarEffectivelyHidden,
  NARROW_SHELL_MAX_WIDTH,
  shouldUseDesktopInspectorSplit,
  shouldUseNarrowShell,
  toggleSidebarVisibility,
} from "./responsive-layout";

describe("getViewportClass", () => {
  it("classifies the artifact-derived narrow shell below 768px", () => {
    expect(getViewportClass(320)).toBe("narrow");
    expect(getViewportClass(NARROW_SHELL_MAX_WIDTH)).toBe("narrow");
    expect(shouldUseNarrowShell(640)).toBe(true);
  });

  it("classifies constrained desktop between narrow and split desktop", () => {
    expect(getViewportClass(NARROW_SHELL_MAX_WIDTH + 1)).toBe("constrained-desktop");
    expect(getViewportClass(CONSTRAINED_DESKTOP_MAX_WIDTH)).toBe("constrained-desktop");
    expect(shouldUseNarrowShell(900)).toBe(false);
  });

  it("classifies full desktop at the inspector split breakpoint", () => {
    expect(getViewportClass(DESKTOP_SPLIT_BREAKPOINT)).toBe("full-desktop");
    expect(getViewportClass(1600)).toBe("full-desktop");
    expect(shouldUseDesktopInspectorSplit(DESKTOP_SPLIT_BREAKPOINT)).toBe(true);
    expect(shouldUseDesktopInspectorSplit(CONSTRAINED_DESKTOP_MAX_WIDTH)).toBe(false);
  });
});

describe("sidebar visibility", () => {
  it("auto-hides on narrow shells while allowing a narrow override", () => {
    expect(
      isSidebarEffectivelyHidden({
        sidebarHidden: false,
        narrowShell: true,
        narrowSidebarOpen: false,
      }),
    ).toBe(true);
    expect(
      isSidebarEffectivelyHidden({
        sidebarHidden: false,
        narrowShell: true,
        narrowSidebarOpen: true,
      }),
    ).toBe(false);
    expect(
      isSidebarEffectivelyHidden({
        sidebarHidden: true,
        narrowShell: true,
        narrowSidebarOpen: true,
      }),
    ).toBe(false);
  });

  it("keeps desktop manual hidden state independent from narrow toggles", () => {
    expect(
      toggleSidebarVisibility({
        sidebarHidden: false,
        narrowShell: true,
        narrowSidebarOpen: false,
      }),
    ).toEqual({ sidebarHidden: false, narrowSidebarOpen: true });

    expect(
      toggleSidebarVisibility({
        sidebarHidden: false,
        narrowShell: false,
        narrowSidebarOpen: true,
      }),
    ).toEqual({ sidebarHidden: true, narrowSidebarOpen: true });
  });
});
