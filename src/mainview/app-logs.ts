import type { AppLogEntry, AppLogLevel, AppLogSource } from "../shared/workspace-contract";

export const APP_LOG_LEVELS: AppLogLevel[] = ["info", "warning", "error"];

export const APP_LOG_SOURCES: AppLogSource[] = [
  "app.lifecycle",
  "app.bridge",
  "app.rpc",
  "auth.provider",
  "settings",
  "workspace",
  "session",
  "session.title",
  "surface",
  "prompt",
  "thread",
  "smithers",
  "workflow.library",
  "workflow.run",
  "workflow.task",
  "project-ci",
  "direct-tool",
  "execute-typescript",
  "artifact",
  "external-editor",
  "renderer",
];

export function formatAppLogCount(count: number): string {
  return count > 99 ? "99+" : String(count);
}

export function filterAppLogEntries(
  entries: AppLogEntry[],
  filters: { level: AppLogLevel | "all"; source: AppLogSource | "all"; query: string },
): AppLogEntry[] {
  const query = filters.query.trim().toLowerCase();
  return entries.filter((entry) => {
    if (filters.level !== "all" && entry.level !== filters.level) return false;
    if (filters.source !== "all" && entry.source !== filters.source) return false;
    if (!query) return true;
    return [
      entry.message,
      entry.source,
      entry.level,
      entry.workspaceSessionId,
      entry.surfacePiSessionId,
      entry.threadId,
      entry.workflowRunId,
      entry.workflowTaskAttemptId,
      entry.commandId,
      entry.details ? JSON.stringify(entry.details) : "",
      entry.error?.message ?? "",
    ]
      .join(" ")
      .toLowerCase()
      .includes(query);
  });
}
