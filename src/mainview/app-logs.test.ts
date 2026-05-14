import { describe, expect, it } from "bun:test";
import type { AppLogEntry } from "../shared/workspace-contract";
import { filterAppLogEntries, formatAppLogCount } from "./app-logs";

function entry(overrides: Partial<AppLogEntry>): AppLogEntry {
  return {
    id: "log-1",
    seq: 1,
    createdAt: "2026-05-13T10:00:00.000Z",
    level: "info",
    source: "workspace",
    message: "Workspace ready",
    ...overrides,
  };
}

describe("formatAppLogCount", () => {
  it("caps large badge counts", () => {
    expect(formatAppLogCount(0)).toBe("0");
    expect(formatAppLogCount(99)).toBe("99");
    expect(formatAppLogCount(100)).toBe("99+");
  });
});

describe("filterAppLogEntries", () => {
  const entries = [
    entry({ id: "1", seq: 1, level: "info", source: "workspace", message: "Workspace ready" }),
    entry({
      id: "2",
      seq: 2,
      level: "warning",
      source: "auth.provider",
      message: "Provider missing",
    }),
    entry({
      id: "3",
      seq: 3,
      level: "error",
      source: "execute-typescript",
      message: "Compile failed",
      commandId: "cmd-1",
    }),
  ];

  it("filters by level and source", () => {
    expect(filterAppLogEntries(entries, { level: "warning", source: "all", query: "" })).toEqual([
      entries[1]!,
    ]);
    expect(
      filterAppLogEntries(entries, { level: "all", source: "execute-typescript", query: "" }),
    ).toEqual([entries[2]!]);
  });

  it("searches messages, sources, and related ids", () => {
    expect(
      filterAppLogEntries(entries, { level: "all", source: "all", query: "provider" }),
    ).toEqual([entries[1]!]);
    expect(filterAppLogEntries(entries, { level: "all", source: "all", query: "cmd-1" })).toEqual([
      entries[2]!,
    ]);
  });
});
