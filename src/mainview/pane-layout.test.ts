import { describe, expect, it } from "bun:test";
import { getSidebarSessionOpenTarget } from "./pane-layout";

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
