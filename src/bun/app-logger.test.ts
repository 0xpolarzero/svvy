import { describe, expect, it } from "bun:test";
import { createAppLogStore } from "./app-log-store";
import { createAppLogger } from "./app-logger";

describe("app logger", () => {
  it("forwards the same redacted entry shape used by app logs", () => {
    const store = createAppLogStore();
    const forwarded: unknown[] = [];
    const logger = createAppLogger({
      store,
      forwardBridgeLog: (_level, message, _source, details, error) => {
        forwarded.push({ message, details, error });
      },
    });

    logger.error(
      "auth.provider",
      "Authorization=Bearer abcdefghijklmnopqrstuvwxyzABCDEF1234567890",
      new Error("Bearer abcdefghijklmnopqrstuvwxyzABCDEF1234567890 failed"),
      {
        apiKey: "sk-abcdefghijklmnopqrstuvwxyzABCDEF1234567890",
        workspaceSessionId: "session-1",
      },
    );

    expect(JSON.stringify(forwarded[0])).not.toContain(
      "abcdefghijklmnopqrstuvwxyzABCDEF1234567890",
    );
    expect(forwarded[0]).toMatchObject({
      message: "Authorization=[REDACTED] [REDACTED]",
      details: {
        apiKey: "[REDACTED]",
        workspaceSessionId: "session-1",
      },
      error: {
        message: "Bearer [REDACTED] failed",
      },
    });
    store.close();
  });
});
