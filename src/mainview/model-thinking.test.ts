import { describe, expect, it } from "bun:test";
import type { Model } from "@mariozechner/pi-ai";
import { getSupportedThinkingLevels, modelSupportsThinking } from "./model-thinking";

function model(overrides: Partial<Model<any>> = {}): Model<any> {
  return {
    id: "test-model",
    name: "Test Model",
    api: "openai-responses",
    provider: "test",
    baseUrl: "https://example.invalid",
    reasoning: true,
    input: ["text"],
    cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 },
    contextWindow: 128000,
    maxTokens: 32000,
    ...overrides,
  };
}

describe("model thinking metadata", () => {
  it("uses pi model.reasoning as the reasoning capability gate", () => {
    expect(modelSupportsThinking(model({ reasoning: true }))).toBe(true);
    expect(modelSupportsThinking(model({ reasoning: false }))).toBe(false);
    expect(modelSupportsThinking(null)).toBe(false);
  });

  it("returns only off for non-reasoning models", () => {
    expect(getSupportedThinkingLevels(model({ reasoning: false }))).toEqual(["off"]);
  });

  it("honors pi thinkingLevelMap when exposing selectable levels", () => {
    expect(
      getSupportedThinkingLevels(
        model({
          thinkingLevelMap: {
            off: null,
            minimal: "low",
            low: "low",
            medium: null,
            high: "high",
            xhigh: "xhigh",
          },
        }),
      ),
    ).toEqual(["minimal", "low", "high", "xhigh"]);
  });
});
