import { describe, expect, it } from "bun:test";
import { queuedMessageOrderChanged, reorderQueuedMessageItems } from "./queued-message-order";

const items = [{ id: "first" }, { id: "second" }, { id: "third" }];

describe("queued message order helpers", () => {
  it("previews drag-hover order locally without mutating the input", () => {
    const reordered = reorderQueuedMessageItems(items, "third", "first");

    expect(reordered.map((item) => item.id)).toEqual(["third", "first", "second"]);
    expect(items.map((item) => item.id)).toEqual(["first", "second", "third"]);
  });

  it("detects only final dropped orders that change the durable queue", () => {
    expect(queuedMessageOrderChanged(items, "third", "first")).toBe(true);
    expect(queuedMessageOrderChanged(items, "second", "third")).toBe(false);
    expect(queuedMessageOrderChanged(items, "third", null)).toBe(false);
    expect(queuedMessageOrderChanged(items, "third", "third")).toBe(false);
  });
});
