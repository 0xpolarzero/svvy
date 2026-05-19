import type { ThinkingLevel } from "@mariozechner/pi-agent-core";
import type { Model } from "@mariozechner/pi-ai";

export const BASE_THINKING_LEVELS: ThinkingLevel[] = ["off", "minimal", "low", "medium", "high"];

export function modelSupportsThinking(model: Model<any> | null): boolean {
  return model?.reasoning === true;
}

export function getSupportedThinkingLevels(model: Model<any> | null): ThinkingLevel[] {
  if (!model || !modelSupportsThinking(model)) return ["off"];

  const visibleBaseLevels = BASE_THINKING_LEVELS.filter(
    (level) => model.thinkingLevelMap?.[level] !== null,
  );
  return model.thinkingLevelMap?.xhigh != null
    ? [...visibleBaseLevels, "xhigh"]
    : visibleBaseLevels;
}
