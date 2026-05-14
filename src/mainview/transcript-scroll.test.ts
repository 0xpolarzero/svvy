import { describe, expect, it } from "bun:test";
import {
  deriveTranscriptUserScrollState,
  shouldAdjustTranscriptScrollForMeasuredItem,
} from "./transcript-scroll";

describe("transcript scroll policy", () => {
  it("keeps auto-scroll enabled near the bottom without rewriting the current anchor", () => {
    const state = deriveTranscriptUserScrollState({
      scrollTop: 489,
      scrollHeight: 900,
      clientHeight: 400,
      shouldVirtualize: true,
      currentAnchorIndex: 14,
      getIndexAtOffset: () => 99,
    });

    expect(state).toEqual({
      stickToBottom: true,
      autoScroll: true,
      anchorIndex: 14,
    });
  });

  it("captures a new anchor when the user scrolls away from the bottom", () => {
    const state = deriveTranscriptUserScrollState({
      scrollTop: 180,
      scrollHeight: 900,
      clientHeight: 400,
      shouldVirtualize: true,
      currentAnchorIndex: 14,
      getIndexAtOffset: (offset) => Math.floor(offset / 12),
    });

    expect(state).toEqual({
      stickToBottom: false,
      autoScroll: false,
      anchorIndex: 15,
    });
  });

  it("lets TanStack adjust scroll only for measured rows above the current anchor", () => {
    expect(
      shouldAdjustTranscriptScrollForMeasuredItem({
        index: 5,
        anchorIndex: 18,
        stickToBottom: false,
      }),
    ).toBe(true);
  });

  it("ignores measurement churn at or below the anchor and while pinned to bottom", () => {
    expect(
      shouldAdjustTranscriptScrollForMeasuredItem({
        index: 18,
        anchorIndex: 18,
        stickToBottom: false,
      }),
    ).toBe(false);

    expect(
      shouldAdjustTranscriptScrollForMeasuredItem({
        index: 5,
        anchorIndex: 18,
        stickToBottom: true,
      }),
    ).toBe(false);
  });
});
