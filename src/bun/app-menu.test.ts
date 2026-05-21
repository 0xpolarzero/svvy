import { describe, expect, it } from "bun:test";
import { readFile } from "node:fs/promises";
import {
  getShortcutAccelerator,
  isAppMenuAction,
  type AppMenuAction,
} from "../shared/shortcut-registry";

describe("native app menu workspace actions", () => {
  it("keeps workspace actions on the typed app-menu dispatch path", () => {
    const actions: AppMenuAction[] = [
      "workspace.open",
      "workspace.newTab",
      "workspace.openInNewTab",
      "session.new",
      "session.newPane",
      "session.dumb",
    ];

    for (const action of actions) {
      expect(isAppMenuAction(action)).toBe(true);
    }

    expect(getShortcutAccelerator("session.new")).toBe("CommandOrControl+N");
    expect(getShortcutAccelerator("session.newPane")).toBe("CommandOrControl+Shift+N");
    expect(getShortcutAccelerator("session.dumb")).toBeNull();
  });

  it("wires File menu workspace actions from the shortcut registry", async () => {
    const source = await readFile(new URL("./index.ts", import.meta.url), "utf8");

    expect(source).toContain('appMenuItem("workspace.open")');
    expect(source).toContain('appMenuItem("workspace.newTab")');
    expect(source).toContain('appMenuItem("workspace.openInNewTab")');
    expect(source).toContain('appMenuItem("session.new")');
    expect(source).toContain('appMenuItem("session.newPane")');
    expect(source).toContain('appMenuItem("session.dumb")');
    expect(source).toContain("sendAppMenuAction");
  });
});
