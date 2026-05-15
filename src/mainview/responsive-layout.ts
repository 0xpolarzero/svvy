export type ViewportClass = "narrow" | "constrained-desktop" | "full-desktop";

export const NARROW_SHELL_MAX_WIDTH = 767;
export const CONSTRAINED_DESKTOP_MAX_WIDTH = 1219;
export const DESKTOP_SPLIT_BREAKPOINT = 1220;

export function getViewportClass(width: number): ViewportClass {
  if (width <= NARROW_SHELL_MAX_WIDTH) {
    return "narrow";
  }

  if (width <= CONSTRAINED_DESKTOP_MAX_WIDTH) {
    return "constrained-desktop";
  }

  return "full-desktop";
}

export function shouldUseNarrowShell(width: number): boolean {
  return getViewportClass(width) === "narrow";
}

export function shouldUseDesktopInspectorSplit(width: number): boolean {
  return width >= DESKTOP_SPLIT_BREAKPOINT;
}

export interface SidebarVisibilityInput {
  sidebarHidden: boolean;
  narrowShell: boolean;
  narrowSidebarOpen: boolean;
}

export function isSidebarEffectivelyHidden(input: SidebarVisibilityInput): boolean {
  return input.narrowShell ? !input.narrowSidebarOpen : input.sidebarHidden;
}

export function toggleSidebarVisibility(input: SidebarVisibilityInput): {
  sidebarHidden: boolean;
  narrowSidebarOpen: boolean;
} {
  if (input.narrowShell) {
    return {
      sidebarHidden: input.sidebarHidden,
      narrowSidebarOpen: !input.narrowSidebarOpen,
    };
  }

  return {
    sidebarHidden: !input.sidebarHidden,
    narrowSidebarOpen: input.narrowSidebarOpen,
  };
}
