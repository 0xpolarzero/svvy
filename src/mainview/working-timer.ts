export function parseWorkingStartedAt(startedAt: string | null | undefined): number | null {
  if (!startedAt) return null;
  const parsed = Date.parse(startedAt);
  return Number.isFinite(parsed) ? parsed : null;
}

function formatDurationParts(totalSeconds: number): {
  compact: string;
  readable: string;
} {
  const seconds = totalSeconds % 60;
  const totalMinutes = Math.floor(totalSeconds / 60);
  const minutes = totalMinutes % 60;
  const hours = Math.floor(totalMinutes / 60);
  const parts: string[] = [];

  if (hours > 0) parts.push(`${hours} ${hours === 1 ? "hour" : "hours"}`);
  if (minutes > 0) parts.push(`${minutes} ${minutes === 1 ? "minute" : "minutes"}`);
  if (seconds > 0 || parts.length === 0) {
    parts.push(`${seconds} ${seconds === 1 ? "second" : "seconds"}`);
  }

  return {
    compact:
      hours > 0
        ? `${hours}:${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`
        : `${minutes}:${String(seconds).padStart(2, "0")}`,
    readable: parts.join(" "),
  };
}

export function formatWorkingElapsed(
  startedAt: string | null | undefined,
  now = Date.now(),
): string {
  const startedAtMs = parseWorkingStartedAt(startedAt);
  if (startedAtMs === null) return "0:00";

  return formatDurationParts(Math.max(0, Math.floor((now - startedAtMs) / 1000))).compact;
}

export function formatWorkingElapsedTooltip(
  startedAt: string | null | undefined,
  now = Date.now(),
): string {
  const startedAtMs = parseWorkingStartedAt(startedAt);
  if (startedAtMs === null) return "Assistant working";

  return `Assistant working for ${
    formatDurationParts(Math.max(0, Math.floor((now - startedAtMs) / 1000))).readable
  }`;
}

export function formatTurnDuration(startedAt: string, finishedAt: string): string {
  return formatDurationParts(elapsedSeconds(startedAt, finishedAt)).compact;
}

export function formatTurnDurationTooltip(startedAt: string, finishedAt: string): string {
  return `Assistant worked for ${formatDurationParts(elapsedSeconds(startedAt, finishedAt)).readable}`;
}

function elapsedSeconds(startedAt: string, finishedAt: string): number {
  const startedAtMs = parseWorkingStartedAt(startedAt);
  const finishedAtMs = parseWorkingStartedAt(finishedAt);
  if (startedAtMs === null || finishedAtMs === null) return 0;
  return Math.max(0, Math.floor((finishedAtMs - startedAtMs) / 1000));
}
