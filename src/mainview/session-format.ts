import type { SessionStatus, WorkspaceSessionSummary } from "../shared/workspace-contract";

export interface SessionSidebarSubtitle {
  badge: "waiting" | "error" | null;
  text: string;
  tone: "muted" | "waiting" | "error";
  blinking: boolean;
}

function formatRelativeUnit(value: number, unit: "min" | "hr" | "day"): string {
  if (value === 0) {
    return "just now";
  }

  const absoluteValue = Math.abs(value);
  const unitLabel = unit === "day" && absoluteValue !== 1 ? "days" : unit;
  const label = `${absoluteValue} ${unitLabel}`;
  return value < 0 ? `${label} ago` : `in ${label}`;
}

function formatCompactRelativeUnit(value: number, unit: "min" | "hr" | "day"): string {
  if (value === 0) {
    return "now";
  }

  const absoluteValue = Math.abs(value);
  const unitLabel = unit === "day" && absoluteValue !== 1 ? "days" : unit;
  const label = `${absoluteValue} ${unitLabel}`;
  return value < 0 ? label : `in ${label}`;
}

export function formatRelativeSessionTime(value: string | number | Date): string {
  const timestamp = new Date(value).getTime();
  if (!Number.isFinite(timestamp)) {
    return "";
  }

  const diffMs = timestamp - Date.now();
  if (Math.abs(diffMs) < 60000) {
    return "just now";
  }

  const diffMinutes = Math.round(diffMs / 60000);
  if (Math.abs(diffMinutes) < 60) {
    return formatRelativeUnit(diffMinutes, "min");
  }

  const diffHours = Math.round(diffMs / 3600000);
  if (Math.abs(diffHours) < 24) {
    return formatRelativeUnit(diffHours, "hr");
  }

  const diffDays = Math.round(diffMs / 86400000);
  return formatRelativeUnit(diffDays, "day");
}

export function formatCompactRelativeSessionTime(value: string | number | Date): string {
  const timestamp = new Date(value).getTime();
  if (!Number.isFinite(timestamp)) {
    return "";
  }

  const diffMs = timestamp - Date.now();
  if (Math.abs(diffMs) < 60000) {
    return "now";
  }

  const diffMinutes = Math.round(diffMs / 60000);
  if (Math.abs(diffMinutes) < 60) {
    return formatCompactRelativeUnit(diffMinutes, "min");
  }

  const diffHours = Math.round(diffMs / 3600000);
  if (Math.abs(diffHours) < 24) {
    return formatCompactRelativeUnit(diffHours, "hr");
  }

  const diffDays = Math.round(diffMs / 86400000);
  return formatCompactRelativeUnit(diffDays, "day");
}

export function shouldShowSessionUpdatedAt(summary: Pick<WorkspaceSessionSummary, "messageCount">): boolean {
  return summary.messageCount > 0;
}

export function sessionStatusTone(
  status: SessionStatus,
): "neutral" | "info" | "warning" | "danger" {
  switch (status) {
    case "running":
      return "warning";
    case "waiting":
      return "info";
    case "error":
      return "danger";
    default:
      return "neutral";
  }
}

export function formatSessionStatusLabel(summary: WorkspaceSessionSummary): string {
  switch (summary.status) {
    case "running":
      return summary.threadIdsByStatus?.runningWorkflow.length ||
        summary.threadIdsByStatus?.runningHandler.length
        ? "Threading"
        : "Running";
    case "waiting":
      return "Waiting";
    case "error":
      return "Error";
    default:
      return "Idle";
  }
}

export function getSessionSidebarSubtitle(summary: WorkspaceSessionSummary): SessionSidebarSubtitle | null {
  if (summary.status === "waiting" && summary.wait?.reason.trim()) {
    return {
      badge: "waiting",
      text: summary.wait.reason.trim(),
      tone: "waiting",
      blinking: false,
    };
  }

  if (summary.status === "error" && summary.preview.trim()) {
    return {
      badge: "error",
      text: summary.preview.trim(),
      tone: "error",
      blinking: false,
    };
  }

  return null;
}

export function formatSessionModel(summary: WorkspaceSessionSummary): string {
  if (summary.provider && summary.modelId) {
    return `${summary.provider}:${summary.modelId}`;
  }
  if (summary.modelId) {
    return summary.modelId;
  }
  return "";
}
