import { describe, expect, it } from "bun:test";
import { resolveEffectiveTheme } from "./theme";

describe("app theme", () => {
  it("uses the system preference when appearance is system", () => {
    expect(resolveEffectiveTheme("system", true)).toBe("dark");
    expect(resolveEffectiveTheme("system", false)).toBe("light");
  });

  it("uses explicit light and dark choices regardless of system preference", () => {
    expect(resolveEffectiveTheme("light", true)).toBe("light");
    expect(resolveEffectiveTheme("dark", false)).toBe("dark");
  });
});
