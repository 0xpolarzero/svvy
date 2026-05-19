import { afterEach, describe, expect, it } from "bun:test";
import type { PromptExecutionRuntimeHandle } from "./prompt-execution-context";
import { createResumeThreadTool } from "./thread-resume-tool";
import {
  createStructuredSessionStateStore,
  type StructuredSessionStateStore,
} from "./structured-session-state";

const WORKSPACE = {
  id: "/repo/svvy",
  label: "svvy",
  cwd: "/repo/svvy",
} as const;

const stores: StructuredSessionStateStore[] = [];

afterEach(() => {
  while (stores.length > 0) {
    stores.pop()?.close();
  }
});

function createStore() {
  const store = createStructuredSessionStateStore({
    workspace: WORKSPACE,
  });
  store.upsertPiSession({
    sessionId: "session-thread-resume-tool",
    title: "Thread Resume Tool Session",
    provider: "openai",
    model: "gpt-5.4",
    reasoningEffort: "medium",
    messageCount: 1,
    status: "running",
    createdAt: "2026-04-24T09:00:00.000Z",
    updatedAt: "2026-04-24T09:00:00.000Z",
  });
  stores.push(store);
  return store;
}

function createRuntime(
  store: StructuredSessionStateStore,
  surfaceKind: "orchestrator" | "handler" = "orchestrator",
): PromptExecutionRuntimeHandle {
  const turn = store.startTurn({
    sessionId: "session-thread-resume-tool",
    surfacePiSessionId: "session-thread-resume-tool",
    requestSummary: "Resume handler thread",
  });
  return {
    current: {
      sessionId: "session-thread-resume-tool",
      turnId: turn.id,
      surfacePiSessionId: "session-thread-resume-tool",
      surfaceThreadId: surfaceKind === "handler" ? "thread-handler" : null,
      surfaceKind,
      defaultEpisodeKind: "analysis",
      rootThreadId: surfaceKind === "handler" ? "thread-handler" : null,
      promptText: "Resume handler thread",
      rootEpisodeKind: "analysis",
      sessionWaitApplied: false,
      threadWasTerminalAtStart: false,
    },
  };
}

describe("thread_resume tool", () => {
  it("records a command and delegates durable resume enqueueing to the catalog bridge", async () => {
    const store = createStore();
    const runtime = createRuntime(store);
    let observedCommandId: string | null = null;

    const tool = createResumeThreadTool({
      runtime,
      store,
      bridge: {
        async resumeHandlerThread(input) {
          observedCommandId = input.resumedByCommandId;
          return {
            threadId: input.threadId,
            surfacePiSessionId: "pi-thread-resume-001",
            queuedMessageId: "surface-queue-001",
          };
        },
      },
    });

    const result = await tool.execute("tool-call-thread-resume", {
      threadId: "thread-resume-001",
      message: "Inspect the prior handoff and continue with the missing detail.",
    });

    const snapshot = store.getSessionState("session-thread-resume-tool");
    const command = snapshot.commands.find((entry) => entry.toolName === "thread_resume");

    expect(observedCommandId as string | null).toBe(command?.id ?? null);
    expect(result.details).toMatchObject({
      ok: true,
      threadId: "thread-resume-001",
      surfacePiSessionId: "pi-thread-resume-001",
      queuedMessageId: "surface-queue-001",
    });
    expect(command).toMatchObject({
      status: "succeeded",
      facts: expect.objectContaining({
        threadId: "thread-resume-001",
        surfacePiSessionId: "pi-thread-resume-001",
        queuedMessageId: "surface-queue-001",
      }),
    });
  });

  it("is orchestrator-only", async () => {
    const store = createStore();
    const tool = createResumeThreadTool({
      runtime: createRuntime(store, "handler"),
      store,
      bridge: {
        async resumeHandlerThread() {
          throw new Error("unexpected resume");
        },
      },
    });

    await expect(
      tool.execute("tool-call-thread-resume", {
        threadId: "thread-resume-001",
        message: "Continue.",
      }),
    ).rejects.toThrow("thread_resume can only run from the orchestrator surface.");
  });
});
