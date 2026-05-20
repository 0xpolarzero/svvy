import { describe, expect, it } from "bun:test";
import {
  formatTurnDuration,
  formatTurnDurationTooltip,
  formatWorkingElapsed,
  formatWorkingElapsedTooltip,
} from "./working-timer";

describe("formatWorkingElapsed", () => {
  it("formats elapsed assistant work below and above one hour", () => {
    const now = Date.parse("2026-04-10T11:04:05.000Z");

    expect(formatWorkingElapsed("2026-04-10T11:03:23.000Z", now)).toBe("0:42");
    expect(formatWorkingElapsed("2026-04-10T10:51:57.000Z", now)).toBe("12:08");
    expect(formatWorkingElapsed("2026-04-10T09:59:32.000Z", now)).toBe("1:04:33");
  });

  it("clamps future or missing starts to a stable zero label", () => {
    const now = Date.parse("2026-04-10T11:04:05.000Z");

    expect(formatWorkingElapsed("2026-04-10T11:04:35.000Z", now)).toBe("0:00");
    expect(formatWorkingElapsed(null, now)).toBe("0:00");
  });
});

describe("formatWorkingElapsedTooltip", () => {
  it("formats tooltip copy for the shared app tooltip", () => {
    const now = Date.parse("2026-04-10T11:04:05.000Z");

    expect(formatWorkingElapsedTooltip("2026-04-10T11:04:04.000Z", now)).toBe(
      "Assistant working for 1 second",
    );
    expect(formatWorkingElapsedTooltip("2026-04-10T09:59:32.000Z", now)).toBe(
      "Assistant working for 1 hour 4 minutes 33 seconds",
    );
  });
});

describe("formatTurnDuration", () => {
  it("formats completed assistant turn durations from persisted timestamps", () => {
    expect(formatTurnDuration("2026-04-10T10:00:00.000Z", "2026-04-10T10:02:09.000Z")).toBe("2:09");
    expect(formatTurnDuration("2026-04-10T09:00:00.000Z", "2026-04-10T10:02:09.000Z")).toBe(
      "1:02:09",
    );
  });

  it("formats completed assistant turn tooltip copy", () => {
    expect(formatTurnDurationTooltip("2026-04-10T10:00:00.000Z", "2026-04-10T10:02:09.000Z")).toBe(
      "Assistant worked for 2 minutes 9 seconds",
    );
  });
});
