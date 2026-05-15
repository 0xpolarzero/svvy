import { describe, expect, it } from "bun:test";
import type { AssistantMessage } from "@mariozechner/pi-ai";
import {
  buildContextBudgetFromUsage,
  buildSurfaceContextBudget,
  formatContextBudgetTooltip,
  getContextBudgetTone,
} from "./context-budget";

function assistantWithInput(input: number): AssistantMessage {
  return {
    role: "assistant",
    content: [{ type: "text", text: "ok" }],
    api: "test",
    provider: "openai",
    model: "gpt-test",
    timestamp: Date.now(),
    stopReason: "stop",
    usage: {
      input,
      output: 1,
      cacheRead: 0,
      cacheWrite: 0,
      totalTokens: input + 1,
      cost: {
        input: 0,
        output: 0,
        cacheRead: 0,
        cacheWrite: 0,
        total: 0,
      },
    },
  };
}

describe("context budget", () => {
  it("uses the adopted neutral, orange, and red thresholds", () => {
    expect(getContextBudgetTone(39)).toBe("neutral");
    expect(getContextBudgetTone(40)).toBe("orange");
    expect(getContextBudgetTone(59)).toBe("orange");
    expect(getContextBudgetTone(60)).toBe("red");
  });

  it("projects the latest processed tokens as active context percentage", () => {
    expect(
      buildContextBudgetFromUsage({ input: 300, output: 45, cacheRead: 30, cacheWrite: 30 }, 1000),
    ).toMatchObject({
      usedTokens: 405,
      maxTokens: 1000,
      percent: 40.5,
      tone: "orange",
      label: "40.5% context",
    });
  });

  it("formats exact token usage for hover detail", () => {
    const budget = buildContextBudgetFromUsage(
      { input: 12345, output: 0, cacheRead: 0, cacheWrite: 0 },
      200000,
    );

    expect(budget && formatContextBudgetTooltip(budget)).toBe("12,345 / 200,000 tokens");
  });

  it("shows an empty live surface as 0 percent when the model has a context window", () => {
    expect(buildSurfaceContextBudget([], { contextWindow: 200000 })).toMatchObject({
      usedTokens: 0,
      maxTokens: 200000,
      percent: 0,
      tone: "neutral",
      label: "0% context",
      detail: "0 of 200k tokens",
    });
  });

  it("uses the latest assistant usage for a live surface", () => {
    const budget = buildSurfaceContextBudget(
      [
        { role: "user", content: "first", timestamp: Date.now() },
        assistantWithInput(20),
        { role: "user", content: "second", timestamp: Date.now() },
        assistantWithInput(60),
      ],
      { contextWindow: 100 },
    );

    expect(budget).toMatchObject({
      usedTokens: 61,
      percent: 61,
      tone: "red",
    });
  });
});
