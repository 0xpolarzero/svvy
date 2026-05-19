import type { AgentTool } from "@mariozechner/pi-agent-core";
import { Type } from "@mariozechner/pi-ai";
import type { Static } from "@sinclair/typebox";
import type { PromptExecutionRuntimeHandle } from "./prompt-execution-context";
import type { StructuredSessionStateStore } from "./structured-session-state";

export const RESUME_THREAD_TOOL_NAME = "thread.resume";

export const resumeThreadParamsSchema = Type.Object(
  {
    threadId: Type.String({ minLength: 1 }),
    message: Type.String({ minLength: 1 }),
  },
  { additionalProperties: false },
);

export type ResumeThreadParams = Static<typeof resumeThreadParamsSchema>;

const RESUME_THREAD_DESCRIPTION = [
  "Resume an existing completed delegated handler thread when the orchestrator needs more help from the same specialist context.",
  "This records the orchestrator decision, reopens the handler thread as running, queues the message on that handler surface, and returns once the resume request is durable.",
  "Use thread.list or thread.handoffs first when you need to identify the right thread.",
].join(" ");

export interface ThreadResumeBridge {
  resumeHandlerThread(input: {
    sessionId: string;
    turnId: string;
    threadId: string;
    message: string;
    resumedByCommandId: string;
  }): Promise<{ threadId: string; surfacePiSessionId: string; queuedMessageId: string }>;
}

export function createResumeThreadTool(options: {
  runtime: PromptExecutionRuntimeHandle;
  store: StructuredSessionStateStore;
  bridge: ThreadResumeBridge;
}): AgentTool<typeof resumeThreadParamsSchema, Record<string, unknown>> {
  return {
    label: "Resume Thread",
    name: RESUME_THREAD_TOOL_NAME,
    description: RESUME_THREAD_DESCRIPTION,
    parameters: resumeThreadParamsSchema,
    execute: async (_toolCallId, params) => {
      const runtime = options.runtime.current;
      if (!runtime) {
        throw new Error(`${RESUME_THREAD_TOOL_NAME} can only run during an active prompt.`);
      }
      if (runtime.surfaceKind !== "orchestrator") {
        throw new Error(`${RESUME_THREAD_TOOL_NAME} can only run from the orchestrator surface.`);
      }

      const threadId = params.threadId.trim();
      const message = params.message.trim();
      if (!threadId || !message) {
        throw new Error(`${RESUME_THREAD_TOOL_NAME} requires non-empty threadId and message.`);
      }

      options.store.setTurnDecision({
        turnId: runtime.turnId,
        decision: RESUME_THREAD_TOOL_NAME,
        onlyIfPending: true,
      });

      const command = options.store.createCommand({
        turnId: runtime.turnId,
        surfacePiSessionId: runtime.surfacePiSessionId,
        threadId: runtime.rootThreadId ?? null,
        toolName: RESUME_THREAD_TOOL_NAME,
        executor: "orchestrator",
        visibility: "surface",
        title: `Resume handler thread: ${threadId}`,
        summary: message,
      });
      options.store.startCommand(command.id);

      try {
        const resumed = await options.bridge.resumeHandlerThread({
          sessionId: runtime.sessionId,
          turnId: runtime.turnId,
          threadId,
          message,
          resumedByCommandId: command.id,
        });
        options.store.finishCommand({
          commandId: command.id,
          status: "succeeded",
          summary: `Queued resume message for handler thread ${threadId}.`,
          facts: resumed,
        });

        return {
          content: [
            {
              type: "text",
              text: JSON.stringify({
                ok: true,
                ...resumed,
              }),
            },
          ],
          details: {
            ok: true,
            ...resumed,
          },
        };
      } catch (error) {
        const errorMessage =
          error instanceof Error ? error.message : "Failed to resume delegated handler thread.";
        options.store.finishCommand({
          commandId: command.id,
          status: "failed",
          summary: errorMessage,
          error: errorMessage,
        });

        return {
          content: [
            {
              type: "text",
              text: JSON.stringify({
                ok: false,
                commandId: command.id,
                error: errorMessage,
              }),
            },
          ],
          details: {
            ok: false,
            commandId: command.id,
            error: errorMessage,
          },
        };
      }
    },
  };
}
