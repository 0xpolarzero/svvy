export const TRANSCRIPT_STICK_TO_BOTTOM_THRESHOLD_PX = 12;

export interface TranscriptUserScrollInput {
  clientHeight: number;
  currentAnchorIndex: number;
  getIndexAtOffset: (offset: number) => number;
  scrollHeight: number;
  scrollTop: number;
  shouldVirtualize: boolean;
  thresholdPx?: number;
}

export interface TranscriptUserScrollState {
  anchorIndex: number;
  autoScroll: boolean;
  stickToBottom: boolean;
}

export interface TranscriptMeasurementAdjustmentInput {
  anchorIndex: number;
  index: number;
  stickToBottom: boolean;
}

export function deriveTranscriptUserScrollState(
  input: TranscriptUserScrollInput,
): TranscriptUserScrollState {
  const thresholdPx = input.thresholdPx ?? TRANSCRIPT_STICK_TO_BOTTOM_THRESHOLD_PX;
  const distanceFromBottom = input.scrollHeight - input.scrollTop - input.clientHeight;
  const stickToBottom = distanceFromBottom < thresholdPx;

  return {
    stickToBottom,
    autoScroll: stickToBottom,
    anchorIndex:
      !stickToBottom && input.shouldVirtualize
        ? input.getIndexAtOffset(input.scrollTop)
        : input.currentAnchorIndex,
  };
}

export function shouldAdjustTranscriptScrollForMeasuredItem(
  input: TranscriptMeasurementAdjustmentInput,
): boolean {
  return !input.stickToBottom && input.index < input.anchorIndex;
}
