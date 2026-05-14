export type AppLogLiveMode = "live" | "frozen";

export const APP_LOG_TAIL_THRESHOLD_PX = 40;

export interface AppLogViewportStateInput {
  scrollOffset: number;
  totalSize: number;
  viewportSize: number;
  thresholdPx?: number;
}

export interface AppLogUpdatePolicyInput {
  bottomPinned: boolean;
  incomingCount: number;
  liveMode: AppLogLiveMode;
}

export interface AppLogUpdatePolicy {
  appendToViewport: boolean;
  showJumpAffordance: boolean;
  scrollToTail: boolean;
}

export function isAppLogViewportBottomPinned(input: AppLogViewportStateInput): boolean {
  const thresholdPx = input.thresholdPx ?? APP_LOG_TAIL_THRESHOLD_PX;
  return input.totalSize - input.scrollOffset - input.viewportSize <= thresholdPx;
}

export function deriveAppLogUpdatePolicy(input: AppLogUpdatePolicyInput): AppLogUpdatePolicy {
  if (input.incomingCount <= 0) {
    return {
      appendToViewport: input.liveMode === "live",
      showJumpAffordance: false,
      scrollToTail: false,
    };
  }

  if (input.liveMode === "frozen") {
    return {
      appendToViewport: false,
      showJumpAffordance: true,
      scrollToTail: false,
    };
  }

  return {
    appendToViewport: true,
    showJumpAffordance: !input.bottomPinned,
    scrollToTail: input.bottomPinned,
  };
}
