import { afterEach, describe, expect, it, setDefaultTimeout, spyOn } from "bun:test";
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import React from "react";
import * as PiCodingAgent from "@mariozechner/pi-coding-agent";
import { createStructuredSessionStateStore } from "../structured-session-state";
import { createDefaultPromptLibraryState } from "../default-system-prompt";
import { SmithersRuntimeManager, type TestWorkflowDefinition } from "./manager";
import {
  createExecuteTypescriptTaskTestWorkflow,
  createHelloWorldTestWorkflow,
} from "./test-workflows";
import {
  bundledWorkflowRuntimeStoredInputSchema,
  readBundledWorkflowLaunchInput,
} from "./runtime-input";
import { Timer, WaitForEvent, createSmithers, runWorkflow } from "smithers-orchestrator";
import { z } from "zod";
import { Effect } from "effect";

const tempDirs: string[] = [];
const stores: Array<ReturnType<typeof createStructuredSessionStateStore>> = [];
const managers: SmithersRuntimeManager[] = [];
type TestStore = ReturnType<typeof createStructuredSessionStateStore>;
type HandlerAttentionEvent = {
  sessionId: string;
  threadId: string;
  workflowRunId: string;
  smithersRunId: string;
  workflowId: string;
  summary: string;
  reason: string;
};

setDefaultTimeout(30_000);

afterEach(async () => {
  while (managers.length > 0) {
    await managers.pop()?.close();
  }
  while (stores.length > 0) {
    stores.pop()?.close();
  }
  while (tempDirs.length > 0) {
    const dir = tempDirs.pop();
    if (dir) {
      rmSync(dir, { force: true, recursive: true });
    }
  }
});

function createManagerHarness(input: {
  cwd: string;
  agentDir: string;
  store: TestStore;
  structuredStateChanges: string[];
  handlerAttentions: HandlerAttentionEvent[];
  onHandlerAttention?: (event: HandlerAttentionEvent) => boolean | Promise<boolean>;
}): SmithersRuntimeManager {
  const manager = new SmithersRuntimeManager({
    cwd: input.cwd,
    agentDir: input.agentDir,
    store: input.store,
    getTaskAgentDefaults: () => ({
      provider: "openai",
      model: "gpt-5.4",
      reasoningEffort: "medium",
      promptLibraryState: createDefaultPromptLibraryState(),
    }),
    onStructuredStateChanged: async (nextSessionId) => {
      input.structuredStateChanges.push(nextSessionId);
    },
    onHandlerAttention: async (event) => {
      input.handlerAttentions.push(event);
      return (await input.onHandlerAttention?.(event)) ?? false;
    },
  });
  managers.push(manager);
  return manager;
}

function createWorkspaceFixture(
  options: {
    onHandlerAttention?: (event: HandlerAttentionEvent) => boolean | Promise<boolean>;
  } = {},
) {
  const root = mkdtempSync(join(tmpdir(), "svvy-smithers-runtime-"));
  tempDirs.push(root);
  const cwd = join(root, "workspace");
  const agentDir = join(root, "agent");
  mkdirSync(cwd, { recursive: true });
  mkdirSync(agentDir, { recursive: true });
  mkdirSync(join(cwd, ".smithers", "executions"), { recursive: true });
  const databasePath = join(root, "structured-session-state.sqlite");
  const store = createStructuredSessionStateStore({
    workspace: {
      id: cwd,
      label: "svvy",
      cwd,
    },
    databasePath,
  });
  stores.push(store);

  const sessionId = "session-smithers-runtime";
  store.upsertPiSession({
    sessionId,
    title: "Smithers Runtime Session",
    provider: "openai",
    model: "gpt-5.4",
    reasoningEffort: "medium",
    messageCount: 1,
    status: "running",
    createdAt: "2026-04-20T08:00:00.000Z",
    updatedAt: "2026-04-20T08:00:00.000Z",
  });

  const seedTurn = store.startTurn({
    sessionId,
    surfacePiSessionId: sessionId,
    requestSummary: "Open a handler thread for workflow supervision",
  });
  const handlerThread = store.createThread({
    turnId: seedTurn.id,
    surfacePiSessionId: "pi-thread-smithers-runtime",
    title: "Workflow supervisor",
    objective: "Supervise saved and artifact Smithers workflow entries.",
  });
  store.finishTurn({
    turnId: seedTurn.id,
    status: "completed",
  });

  const structuredStateChanges: string[] = [];
  const handlerAttentions: HandlerAttentionEvent[] = [];
  const manager = createManagerHarness({
    cwd,
    agentDir,
    store,
    structuredStateChanges,
    handlerAttentions,
    onHandlerAttention: options.onHandlerAttention,
  });
  registerWorkflow(manager, createHelloWorldTestWorkflow(smithersDbPath(cwd)));
  registerWorkflow(
    manager,
    createExecuteTypescriptTaskTestWorkflow({
      dbPath: smithersDbPath(cwd),
      cwd,
      agentDir,
      artifactDir: taskAgentArtifactDir(cwd),
      store,
      provider: "openai",
      model: "gpt-5.4",
      reasoningEffort: "medium",
      promptLibraryState: createDefaultPromptLibraryState(),
    }),
  );

  return {
    cwd,
    agentDir,
    store,
    manager,
    sessionId,
    threadId: handlerThread.id,
    surfacePiSessionId: handlerThread.surfacePiSessionId,
    structuredStateChanges,
    handlerAttentions,
  };
}

async function waitFor(
  description: string,
  condition: () => boolean | Promise<boolean>,
  timeoutMs = 20_000,
): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (await condition()) {
      return;
    }
    await Bun.sleep(25);
  }

  throw new Error(`Timed out waiting for ${description}.`);
}

function smithersDbPath(cwd: string): string {
  return join(cwd, ".svvy", "smithers-runtime", "smithers.db");
}

function taskAgentArtifactDir(cwd: string): string {
  return join(cwd, ".svvy", "smithers-runtime", "artifacts", "task-agent");
}

function smithersLogPath(cwd: string, runId: string): string {
  return join(cwd, ".smithers", "executions", runId, "logs", "stream.ndjson");
}

function fileContains(path: string, needle: string): boolean {
  try {
    return existsSync(path) && readFileSync(path, "utf8").includes(needle);
  } catch {
    return false;
  }
}

function registerWorkflow(
  manager: SmithersRuntimeManager,
  definition: TestWorkflowDefinition,
): void {
  manager.registerTestWorkflow(definition);
}

function latestEntry<T>(entries: T[] | undefined): T | null {
  return entries && entries.length > 0 ? (entries[entries.length - 1] ?? null) : null;
}

type ApprovalDecision = {
  approved: boolean;
  note: string | null;
  decidedBy: string | null;
  decidedAt: string | null;
};

function createWorkflowCommand(input: {
  store: ReturnType<typeof createStructuredSessionStateStore>;
  sessionId: string;
  threadId: string;
  surfacePiSessionId: string;
  requestSummary: string;
  toolName?: "smithers_run_workflow";
  title: string;
  summary: string;
}): { turnId: string; commandId: string } {
  const turn = input.store.startTurn({
    sessionId: input.sessionId,
    surfacePiSessionId: input.surfacePiSessionId,
    threadId: input.threadId,
    requestSummary: input.requestSummary,
  });
  const command = input.store.createCommand({
    turnId: turn.id,
    surfacePiSessionId: input.surfacePiSessionId,
    threadId: input.threadId,
    toolName: input.toolName ?? "smithers_run_workflow",
    executor: "smithers",
    visibility: "surface",
    title: input.title,
    summary: input.summary,
  });
  input.store.startCommand(command.id);
  return {
    turnId: turn.id,
    commandId: command.id,
  };
}

function createApprovalWorkflowDefinition(dbPath: string): TestWorkflowDefinition {
  const launchSchema = z.object({
    title: z.string().min(1).default("Approve release?"),
  });
  const smithersApi = createSmithers(
    {
      input: bundledWorkflowRuntimeStoredInputSchema,
      approval: z.object({
        approved: z.boolean(),
        note: z.string().nullable(),
        decidedBy: z.string().nullable(),
        decidedAt: z.string().nullable(),
      }),
      approvalResult: z.object({
        approved: z.boolean(),
        note: z.string().nullable(),
      }),
    },
    { dbPath },
  );

  return {
    id: "approval_gate",
    label: "Approval Gate",
    summary: "Waits for an approval decision and finishes after the run is resumed.",
    workflowName: "svvy-approval-gate",
    launchSchema,
    workflow: smithersApi.smithers((ctx) => {
      const workflowInput = readBundledWorkflowLaunchInput(launchSchema, ctx.input);
      const decision = latestEntry<ApprovalDecision>(ctx.outputs.approval);
      return React.createElement(
        smithersApi.Workflow,
        { name: "svvy-approval-gate" },
        React.createElement(
          smithersApi.Sequence,
          null,
          React.createElement(smithersApi.Approval, {
            id: "publish-gate",
            output: smithersApi.outputs.approval,
            request: {
              title: workflowInput.title,
              summary: "The workflow is blocked on explicit handler approval.",
            },
            onDeny: "continue",
          }),
          decision
            ? React.createElement(smithersApi.Task, {
                id: "record-decision",
                output: smithersApi.outputs.approvalResult,
                children: {
                  approved: Boolean(decision.approved),
                  note: decision.note ?? null,
                },
              })
            : null,
        ),
      );
    }),
  };
}

function createContinueAsNewWorkflowDefinition(dbPath: string): TestWorkflowDefinition {
  const launchSchema = z.object({}).passthrough();
  const smithersApi = createSmithers(
    {
      input: bundledWorkflowRuntimeStoredInputSchema,
      continueResult: z.object({
        cursor: z.string().nullable(),
        seenPayload: z.boolean(),
      }),
    },
    { dbPath },
  );

  return {
    id: "continue_once",
    label: "Continue Once",
    summary: "Continues as new exactly once and then produces a result.",
    workflowName: "svvy-continue-once",
    launchSchema,
    workflow: smithersApi.smithers((ctx) => {
      const workflowInput = readBundledWorkflowLaunchInput(launchSchema, ctx.input);
      const continuation = getSmithersContinuation(workflowInput.__smithersContinuation);
      const shouldContinue = !continuation?.payload;

      return React.createElement(
        smithersApi.Workflow,
        { name: "svvy-continue-once" },
        React.createElement(
          smithersApi.Sequence,
          null,
          shouldContinue
            ? React.createElement(smithersApi.ContinueAsNew, {
                state: { cursor: "cursor-after-continue" },
              })
            : null,
          React.createElement(smithersApi.Task, {
            id: "result",
            output: smithersApi.outputs.continueResult,
            children: {
              cursor: continuation?.payload?.cursor ?? null,
              seenPayload: Boolean(continuation?.payload),
            },
          }),
        ),
      );
    }),
  };
}

function getSmithersContinuation(value: unknown): { payload?: { cursor?: string } } | undefined {
  if (!value || typeof value !== "object") {
    return undefined;
  }
  return value as { payload?: { cursor?: string } };
}

function createSignalWorkflowDefinition(dbPath: string): TestWorkflowDefinition {
  const launchSchema = z.object({
    signalName: z.string().min(1).default("deploy.completed"),
  });
  const signalPayloadSchema = z.object({
    environment: z.string(),
    sha: z.string(),
    status: z.enum(["success", "failure"]),
  });
  const resultSchema = z.object({
    summary: z.string(),
    environment: z.string(),
    status: z.enum(["success", "failure"]),
  });
  const smithersApi = createSmithers(
    {
      input: bundledWorkflowRuntimeStoredInputSchema,
      signalPayload: signalPayloadSchema,
      signalResult: resultSchema,
    },
    { dbPath },
  );

  return {
    id: "wait_for_signal",
    label: "Wait For Signal",
    summary: "Waits on a durable Smithers signal and records the delivered payload.",
    workflowName: "svvy-wait-for-signal",
    launchSchema,
    workflow: smithersApi.smithers((ctx) => {
      const workflowInput = readBundledWorkflowLaunchInput(launchSchema, ctx.input);
      const payload = latestEntry<z.infer<typeof signalPayloadSchema>>(ctx.outputs.signalPayload);
      return React.createElement(
        smithersApi.Workflow,
        { name: "svvy-wait-for-signal" },
        React.createElement(
          smithersApi.Sequence,
          null,
          React.createElement(WaitForEvent, {
            id: "wait-signal",
            event: workflowInput.signalName,
            output: smithersApi.outputs.signalPayload,
            outputSchema: signalPayloadSchema,
            label: `wait:${workflowInput.signalName}`,
          }),
          payload
            ? React.createElement(smithersApi.Task, {
                id: "result",
                output: smithersApi.outputs.signalResult,
                children: {
                  summary: `Received ${workflowInput.signalName} for ${payload.environment}.`,
                  environment: payload.environment,
                  status: payload.status,
                },
              })
            : null,
        ),
      );
    }),
  };
}

function createTimerWorkflowDefinition(dbPath: string): TestWorkflowDefinition {
  const launchSchema = z.object({
    duration: z.string().min(1).default("1h"),
  });
  const resultSchema = z.object({
    summary: z.string(),
  });
  const smithersApi = createSmithers(
    {
      input: bundledWorkflowRuntimeStoredInputSchema,
      timerResult: resultSchema,
    },
    { dbPath },
  );

  return {
    id: "wait_for_timer",
    label: "Wait For Timer",
    summary: "Waits on a durable Smithers timer and records when it fires.",
    workflowName: "svvy-wait-for-timer",
    launchSchema,
    workflow: smithersApi.smithers((ctx) => {
      const workflowInput = readBundledWorkflowLaunchInput(launchSchema, ctx.input);
      return React.createElement(
        smithersApi.Workflow,
        { name: "svvy-wait-for-timer" },
        React.createElement(
          smithersApi.Sequence,
          null,
          React.createElement(Timer, {
            id: "sleep",
            duration: workflowInput.duration,
            label: `timer:${workflowInput.duration}`,
          }),
          React.createElement(smithersApi.Task, {
            id: "result",
            output: smithersApi.outputs.timerResult,
            children: {
              summary: `Timer ${workflowInput.duration} fired.`,
            },
          }),
        ),
      );
    }),
  };
}

function createSlowWorkflowDefinition(dbPath: string): TestWorkflowDefinition {
  const launchSchema = z.object({
    delayMs: z.number().int().min(50).max(10_000).default(500),
    message: z.string().min(1).default("slow workflow"),
  });
  const resultSchema = z.object({
    summary: z.string(),
    message: z.string(),
  });
  const smithersApi = createSmithers(
    {
      input: bundledWorkflowRuntimeStoredInputSchema,
      slowResult: resultSchema,
    },
    { dbPath },
  );

  return {
    id: "slow_resume",
    label: "Slow Resume",
    summary: "Sleeps long enough for restart recovery to resume the same Smithers run.",
    workflowName: "svvy-slow-resume",
    launchSchema,
    workflow: smithersApi.smithers((ctx) => {
      const workflowInput = readBundledWorkflowLaunchInput(launchSchema, ctx.input);
      return React.createElement(
        smithersApi.Workflow,
        { name: "svvy-slow-resume" },
        React.createElement(smithersApi.Task, {
          id: "slow-result",
          output: smithersApi.outputs.slowResult,
          children: async () => {
            await Bun.sleep(workflowInput.delayMs);
            return {
              summary: `Finished after ${workflowInput.delayMs}ms.`,
              message: workflowInput.message,
            };
          },
        }),
      );
    }),
  };
}

function createTranscriptWorkflowDefinition(dbPath: string): TestWorkflowDefinition {
  const launchSchema = z.object({
    prompt: z.string().min(1).default("Summarize the latest transcript probe."),
  });
  const transcriptReplySchema = z.object({
    reply: z.string(),
    promptEcho: z.string(),
  });
  const resultSchema = z.object({
    summary: z.string(),
    reply: z.string(),
  });
  const smithersApi = createSmithers(
    {
      input: bundledWorkflowRuntimeStoredInputSchema,
      transcriptReply: transcriptReplySchema,
      transcriptResult: resultSchema,
    },
    { dbPath },
  );
  const transcriptAgent = {
    id: "svvy-deterministic-transcript-agent",
    async generate(rawArgs: unknown) {
      const args = rawArgs as {
        prompt?: string;
        onStdout?: (chunk: string) => void;
        onStepFinish?: (step: {
          response: { messages: Array<{ role: string; content: string }> };
        }) => void;
      };
      const promptText =
        typeof args.prompt === "string" && args.prompt.trim().length > 0
          ? args.prompt.trim()
          : "No prompt provided.";
      const response = {
        reply: `Handled: ${promptText}`,
        promptEcho: promptText,
      };
      const responseText = JSON.stringify(response);
      args.onStdout?.(responseText);
      args.onStepFinish?.({
        response: {
          messages: [{ role: "assistant", content: responseText }],
        },
      });
      return {
        text: responseText,
        output: response,
        response: {
          messages: [{ role: "assistant", content: responseText }],
        },
      };
    },
  };

  return {
    id: "chat_transcript_probe",
    label: "Chat Transcript Probe",
    summary:
      "Runs a deterministic agent task so transcript inspection can read real attempt history.",
    workflowName: "svvy-chat-transcript-probe",
    launchSchema,
    workflow: smithersApi.smithers((ctx) => {
      const workflowInput = readBundledWorkflowLaunchInput(launchSchema, ctx.input);
      const reply = latestEntry<z.infer<typeof transcriptReplySchema>>(ctx.outputs.transcriptReply);
      return React.createElement(
        smithersApi.Workflow,
        { name: "svvy-chat-transcript-probe" },
        React.createElement(
          smithersApi.Sequence,
          null,
          React.createElement(
            smithersApi.Task,
            {
              id: "assistant",
              output: smithersApi.outputs.transcriptReply,
              agent: transcriptAgent as any,
            },
            workflowInput.prompt,
          ),
          reply
            ? React.createElement(smithersApi.Task, {
                id: "result",
                output: smithersApi.outputs.transcriptResult,
                children: {
                  summary: `Captured transcript reply for prompt "${reply.promptEcho}".`,
                  reply: reply.reply,
                },
              })
            : null,
        ),
      );
    }),
  };
}

const projectCiTerminalResultObjectSchema = z.object({
  status: z.enum(["passed", "failed", "cancelled", "blocked"]),
  summary: z.string().min(1),
  startedAt: z.string().optional(),
  finishedAt: z.string().optional(),
  checks: z.array(
    z.object({
      checkId: z.string().min(1),
      label: z.string().min(1),
      kind: z.string().min(1),
      status: z.enum(["passed", "failed", "cancelled", "skipped", "blocked"]),
      required: z.boolean().default(true),
      command: z.array(z.string()).optional(),
      exitCode: z.number().int().nullable().optional(),
      summary: z.string().min(1),
      artifactIds: z.array(z.string()).default([]),
      startedAt: z.string().nullable().optional(),
      finishedAt: z.string().nullable().optional(),
    }),
  ),
});

function normalizeProjectCiTerminalOutput(value: unknown): unknown {
  const candidate = Array.isArray(value) ? value[0] : value;
  if (!candidate || typeof candidate !== "object") {
    return candidate;
  }
  const result = { ...(candidate as Record<string, unknown>) };
  if (result.startedAt === null) {
    delete result.startedAt;
  }
  if (result.finishedAt === null) {
    delete result.finishedAt;
  }
  return result;
}

const projectCiTerminalResultSchema = z.preprocess(
  normalizeProjectCiTerminalOutput,
  projectCiTerminalResultObjectSchema,
);
const invalidProjectCiTerminalResultSchema = z.preprocess(
  normalizeProjectCiTerminalOutput,
  projectCiTerminalResultObjectSchema.extend({
    validatorToken: z.literal("valid"),
  }),
);

function createProjectCiTerminalOutputWorkflowDefinition(input: {
  dbPath: string;
  validOutput?: boolean;
  id?: string;
  productKind?: "project-ci";
}): TestWorkflowDefinition {
  const launchSchema = z.object({
    scope: z.enum(["fast", "full", "release"]).default("fast"),
    reason: z.string().optional(),
  });
  const workflowId = input.id ?? "project_ci";
  const smithersApi = createSmithers(
    {
      input: bundledWorkflowRuntimeStoredInputSchema,
      output: projectCiTerminalResultObjectSchema,
    },
    { dbPath: input.dbPath },
  );
  return {
    id: workflowId,
    label: input.productKind ? "Project CI" : "Project CI Lookalike",
    summary: input.productKind
      ? "Test Project CI workflow."
      : "Non-CI workflow with CI-shaped output.",
    launchSchema,
    productKind: input.productKind,
    resultSchema: input.productKind
      ? input.validOutput === false
        ? invalidProjectCiTerminalResultSchema
        : projectCiTerminalResultSchema
      : undefined,
    workflow: smithersApi.smithers((ctx) => {
      const workflowInput = readBundledWorkflowLaunchInput(launchSchema, ctx.input);
      const terminalOutput = {
        status: workflowInput.scope === "release" ? "blocked" : "passed",
        summary:
          input.validOutput === false
            ? `Project CI ${workflowInput.scope} checks reported an invalid terminal contract.`
            : `Project CI ${workflowInput.scope} checks passed.`,
        checks: [
          {
            checkId: "typecheck",
            label: "Typecheck",
            kind: "typecheck",
            status: "passed",
            required: true,
            command: ["bun", "run", "typecheck"],
            exitCode: 0,
            summary: "Typecheck passed.",
            artifactIds: [],
          },
        ],
      };
      return React.createElement(
        smithersApi.Workflow,
        { name: input.productKind ? "svvy-project-ci" : "svvy-project-ci-lookalike" },
        React.createElement(smithersApi.Task, {
          id: "result",
          output: smithersApi.outputs.output,
          children: terminalOutput,
        }),
      );
    }),
    sourceScope: "saved",
    entryPath: input.productKind
      ? ".svvy/workflows/entries/ci/project-ci.tsx"
      : ".svvy/workflows/entries/ci-looking-workflow.tsx",
  };
}

function createProjectCiMissingTerminalOutputWorkflowDefinition(
  dbPath: string,
): TestWorkflowDefinition {
  const launchSchema = z.object({
    scope: z.enum(["fast", "full", "release"]).default("fast"),
  });
  const otherOutputSchema = z.object({
    summary: z.string().min(1),
  });
  const smithersApi = createSmithers(
    {
      input: bundledWorkflowRuntimeStoredInputSchema,
      otherOutput: otherOutputSchema,
    },
    { dbPath },
  );

  return {
    id: "missing_project_ci_output",
    label: "Missing Project CI Output",
    summary: "Declared Project CI workflow that finishes without Smithers terminal output.",
    launchSchema,
    productKind: "project-ci",
    resultSchema: projectCiTerminalResultSchema,
    workflow: smithersApi.smithers((ctx) => {
      const workflowInput = readBundledWorkflowLaunchInput(launchSchema, ctx.input);
      return React.createElement(
        smithersApi.Workflow,
        { name: "svvy-missing-project-ci-output" },
        React.createElement(smithersApi.Task, {
          id: "not-terminal-output",
          output: smithersApi.outputs.otherOutput,
          children: {
            summary: `Recorded ${workflowInput.scope} details outside the terminal output table.`,
          },
        }),
      );
    }),
    sourceScope: "saved",
    entryPath: ".svvy/workflows/entries/ci/missing-project-ci-output.tsx",
  };
}

describe("SmithersRuntimeManager", () => {
  it("publishes workflow discovery metadata with input-side defaults", () => {
    const { cwd, manager } = createWorkspaceFixture();

    const helloWorldWorkflow = manager
      .listWorkflows()
      .find((workflow) => workflow.workflowId === "hello_world");

    expect(helloWorldWorkflow).toMatchObject({
      workflowId: "hello_world",
      label: "Hello World",
      summary: "Smoke-test workflow used by Smithers runtime tests.",
      sourceScope: "saved",
      entryPath: ".svvy/workflows/entries/hello-world.tsx",
      launchInputSchema: {
        type: "object",
        properties: {
          message: {
            default: "hello world",
            type: "string",
            minLength: 1,
          },
        },
      },
    });

    registerWorkflow(manager, createApprovalWorkflowDefinition(smithersDbPath(cwd)));
    expect(manager.listWorkflows().map((workflow) => workflow.workflowId)).toEqual(
      expect.arrayContaining(["approval_gate"]),
    );
  });

  it("exposes declared Project CI entries with result schema discovery metadata", () => {
    const { cwd, manager } = createWorkspaceFixture();
    registerWorkflow(
      manager,
      createProjectCiTerminalOutputWorkflowDefinition({
        dbPath: smithersDbPath(cwd),
        productKind: "project-ci",
      }),
    );

    const workflows = manager.listWorkflows({ productKind: "project-ci" });

    expect(workflows).toHaveLength(1);
    expect(workflows[0]).toMatchObject({
      workflowId: "project_ci",
      productKind: "project-ci",
      entryPath: ".svvy/workflows/entries/ci/project-ci.tsx",
      resultSchema: {
        type: "object",
        required: ["status", "summary", "checks"],
        properties: expect.objectContaining({
          status: expect.objectContaining({
            enum: ["passed", "failed", "cancelled", "blocked"],
          }),
          checks: expect.objectContaining({
            type: "array",
          }),
        }),
      },
    });
  });

  it("records Project CI results only for declared entries with schema-valid terminal output", async () => {
    const fixture = createWorkspaceFixture();
    const { cwd, store, manager, sessionId, threadId, surfacePiSessionId } = fixture;
    registerWorkflow(
      manager,
      createProjectCiTerminalOutputWorkflowDefinition({
        dbPath: smithersDbPath(cwd),
        productKind: "project-ci",
      }),
    );
    registerWorkflow(
      manager,
      createProjectCiTerminalOutputWorkflowDefinition({
        dbPath: smithersDbPath(cwd),
        id: "ci_shaped_non_ci",
      }),
    );
    registerWorkflow(
      manager,
      createProjectCiTerminalOutputWorkflowDefinition({
        dbPath: smithersDbPath(cwd),
        id: "invalid_project_ci",
        productKind: "project-ci",
        validOutput: false,
      }),
    );

    const validCommand = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Run declared Project CI",
      title: "Run Project CI",
      summary: "Launch the declared Project CI workflow.",
    });
    const validCiRun = await manager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "project_ci",
      launchInput: { scope: "fast" },
      commandId: validCommand.commandId,
    });

    await waitFor("valid Project CI completion", async () => {
      const run = await manager.getRun(validCiRun.runId);
      return run.status === "finished";
    });
    await waitFor("valid Project CI record", async () => {
      await manager.getRun(validCiRun.runId);
      const snapshot = store.getSessionState(sessionId);
      return snapshot.ciRuns.length === 1 && snapshot.ciCheckResults.length === 1;
    });

    const firstSnapshot = store.getSessionState(sessionId);
    const firstCiRunId = firstSnapshot.ciRuns[0]?.id;
    const firstCheckResultId = firstSnapshot.ciCheckResults[0]?.id;
    expect(firstSnapshot.ciRuns[0]).toMatchObject({
      workflowId: "project_ci",
      entryPath: ".svvy/workflows/entries/ci/project-ci.tsx",
      status: "passed",
      summary: "Project CI fast checks passed.",
    });
    expect(firstSnapshot.ciCheckResults[0]).toMatchObject({
      ciRunId: firstCiRunId,
      checkId: "typecheck",
      status: "passed",
      command: ["bun", "run", "typecheck"],
      exitCode: 0,
    });

    await manager.getRun(validCiRun.runId);
    await manager.getRun(validCiRun.runId);
    const repeatedSnapshot = store.getSessionState(sessionId);
    expect(repeatedSnapshot.ciRuns).toHaveLength(1);
    expect(repeatedSnapshot.ciCheckResults).toHaveLength(1);
    expect(repeatedSnapshot.ciRuns[0]?.id).toBe(firstCiRunId);
    expect(repeatedSnapshot.ciCheckResults[0]?.id).toBe(firstCheckResultId);

    const nonCiCommand = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Run CI-shaped non-CI workflow",
      title: "Run CI-shaped workflow",
      summary: "Launch a workflow that emits CI-shaped output without declaring Project CI.",
    });
    const nonCiRun = await manager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "ci_shaped_non_ci",
      launchInput: { scope: "full" },
      commandId: nonCiCommand.commandId,
    });

    await waitFor("CI-shaped non-CI completion", async () => {
      const run = await manager.getRun(nonCiRun.runId);
      return run.status === "finished";
    });
    const afterNonCiSnapshot = store.getSessionState(sessionId);
    expect(afterNonCiSnapshot.ciRuns).toHaveLength(1);
    expect(afterNonCiSnapshot.ciCheckResults).toHaveLength(1);

    const invalidCiCommand = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Run invalid declared Project CI workflow",
      title: "Run invalid Project CI",
      summary: "Launch a declared Project CI workflow that emits invalid output.",
    });
    const invalidCiRun = await manager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "invalid_project_ci",
      launchInput: { scope: "fast" },
      commandId: invalidCiCommand.commandId,
    });

    await waitFor("invalid Project CI failure projection", async () => {
      const run = await manager.getRun(invalidCiRun.runId);
      return (
        run.status === "finished" &&
        run.summary.includes("did not validate against the entry result schema")
      );
    });
    const afterInvalidSnapshot = store.getSessionState(sessionId);
    expect(afterInvalidSnapshot.ciRuns).toHaveLength(1);
    expect(afterInvalidSnapshot.ciCheckResults).toHaveLength(1);
  });

  it("projects Project CI from durable Smithers output after restart when Smithers finished first", async () => {
    const fixture = createWorkspaceFixture();
    const { cwd, agentDir, store, manager, sessionId, threadId, surfacePiSessionId } = fixture;
    const definition = createProjectCiTerminalOutputWorkflowDefinition({
      dbPath: smithersDbPath(cwd),
      productKind: "project-ci",
    });
    registerWorkflow(manager, definition);

    const runId = "smithers-finished-before-svvy-project-ci-projection";
    await Effect.runPromise(
      runWorkflow(definition.workflow, {
        runId,
        input: { scope: "full" },
        rootDir: cwd,
      }),
    );

    const command = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Recover a finished Project CI workflow",
      title: "Recover Project CI",
      summary: "Project a Smithers run that finished before svvy reconciled it.",
    });
    const structuredRun = store.recordWorkflow({
      threadId,
      commandId: command.commandId,
      smithersRunId: runId,
      workflowName: "project_ci",
      workflowSource: "saved",
      entryPath: ".svvy/workflows/entries/ci/project-ci.tsx",
      savedEntryId: "project_ci",
      status: "running",
      smithersStatus: "running",
      waitKind: null,
      continuedFromRunIds: [],
      activeDescendantRunId: null,
      lastEventSeq: null,
      pendingAttentionSeq: null,
      lastAttentionSeq: null,
      heartbeatAt: null,
      summary: "Recovering Project CI.",
    });

    await manager.close();
    const restoredManager = createManagerHarness({
      cwd,
      agentDir,
      store,
      structuredStateChanges: fixture.structuredStateChanges,
      handlerAttentions: fixture.handlerAttentions,
    });
    registerWorkflow(restoredManager, definition);

    await restoredManager.restoreSessionSupervision(sessionId, { emitAttention: false });

    const snapshot = store.getSessionState(sessionId);
    expect(snapshot.workflowRuns.find((entry) => entry.id === structuredRun.id)).toMatchObject({
      status: "running",
      smithersStatus: "running",
      summary: "Recovering Project CI.",
    });
    expect(await restoredManager.getRun(runId)).toMatchObject({
      runId,
      status: "finished",
      summary: "Project CI full checks passed.",
    });
    expect(snapshot.ciRuns).toHaveLength(1);
    expect(snapshot.ciRuns[0]).toMatchObject({
      workflowRunId: structuredRun.id,
      status: "passed",
      summary: "Project CI full checks passed.",
    });
    expect(snapshot.ciCheckResults).toHaveLength(1);
    expect(snapshot.ciCheckResults[0]).toMatchObject({
      ciRunId: snapshot.ciRuns[0]?.id,
      checkId: "typecheck",
    });
  });

  it("marks declared Project CI troubleshooting when durable terminal output is missing", async () => {
    const { cwd, store, manager, sessionId, threadId, surfacePiSessionId } =
      createWorkspaceFixture();
    registerWorkflow(
      manager,
      createProjectCiMissingTerminalOutputWorkflowDefinition(smithersDbPath(cwd)),
    );

    const command = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Run Project CI without terminal output",
      title: "Run Project CI without terminal output",
      summary: "Launch a declared Project CI workflow that omits the terminal output table.",
    });
    const launched = await manager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "missing_project_ci_output",
      launchInput: { scope: "fast" },
      commandId: command.commandId,
    });

    await waitFor("missing Project CI output troubleshooting projection", async () => {
      const run = await manager.getRun(launched.runId);
      const snapshot = store.getSessionState(sessionId);
      const thread = snapshot.threads.find((entry) => entry.id === threadId);
      return (
        run.status === "finished" &&
        run.summary.includes("finished without durable terminal output") &&
        thread?.status === "troubleshooting" &&
        snapshot.workflowRuns.some((entry) => entry.smithersRunId === launched.runId)
      );
    });

    const snapshot = store.getSessionState(sessionId);
    expect(snapshot.ciRuns).toHaveLength(0);
    expect(snapshot.ciCheckResults).toHaveLength(0);
  });

  it("runs the saved hello_world fixture workflow through the real Smithers runtime and projects completion back to the handler thread", async () => {
    const {
      cwd,
      store,
      manager,
      sessionId,
      threadId,
      surfacePiSessionId,
      structuredStateChanges,
      handlerAttentions,
    } = createWorkspaceFixture();

    expect(manager.listWorkflows().map((workflow) => workflow.workflowId)).toEqual(
      expect.arrayContaining(["hello_world", "execute_typescript_task"]),
    );

    const launchCommand = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Launch hello world",
      title: "Run hello_world",
      summary: "Launch the hello_world workflow.",
      toolName: "smithers_run_workflow",
    });
    const launched = await manager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "hello_world",
      launchInput: { message: "bonjour smithers" },
      commandId: launchCommand.commandId,
    });

    expect(launched).toMatchObject({
      workflowId: "hello_world",
      status: "running",
      smithersStatus: "running",
    });
    expect(launched.runId).toMatch(/^smithers-/);
    expect(existsSync(smithersDbPath(cwd))).toBe(true);
    expect(
      store.getSessionState(sessionId).threads.find((thread) => thread.id === threadId)?.status,
    ).toBe("running-workflow");

    await waitFor("hello_world completion", async () => {
      try {
        const run = await manager.getRun(launched.runId);
        return run.status === "finished";
      } catch {
        return false;
      }
    });
    await waitFor("hello_world handler attention", () =>
      handlerAttentions.some((event) =>
        event.reason.includes("finished and the handler must reconcile"),
      ),
    );

    const snapshot = store.getSessionState(sessionId);
    const workflowRun = snapshot.workflowRuns.find(
      (entry) => entry.smithersRunId === launched.runId,
    );
    expect(workflowRun).toMatchObject({
      threadId,
      workflowName: "hello_world",
      workflowSource: "saved",
      entryPath: ".svvy/workflows/entries/hello-world.tsx",
      savedEntryId: "hello_world",
      status: "running",
      smithersStatus: "running",
      waitKind: null,
    });
    expect(snapshot.threads.find((thread) => thread.id === threadId)).toMatchObject({
      id: threadId,
      status: "running-handler",
      wait: null,
    });
    expect(snapshot.session.wait).toBeNull();
    expect(structuredStateChanges).toContain(sessionId);
    expect(
      handlerAttentions.some((event) =>
        event.reason.includes("finished and the handler must reconcile"),
      ),
    ).toBe(true);

    const runs = await manager.listRuns({ workflowId: "hello_world" });
    expect(runs).toHaveLength(1);
    expect(runs[0]).toMatchObject({
      runId: launched.runId,
      workflowName: "hello_world",
      workflowId: "hello_world",
      workflowSource: "saved",
      entryPath: ".svvy/workflows/entries/hello-world.tsx",
      status: "finished",
      sessionId,
      threadId,
    });

    const run = await manager.getRun(launched.runId);
    expect(run).toMatchObject({
      runId: launched.runId,
      workflowName: "hello_world",
      workflowId: "hello_world",
      workflowSource: "saved",
      entryPath: ".svvy/workflows/entries/hello-world.tsx",
      status: "finished",
      structuredWorkflowRunId: workflowRun?.id,
      threadId,
      waitKind: null,
    });

    const explanation = await manager.explainRun(launched.runId);
    expect(explanation.summary).toContain("finished");
    expect(explanation.diagnosis).toMatchObject({
      runId: launched.runId,
      status: "finished",
    });

    const events = await manager.getRunEvents({ runId: launched.runId });
    expect(events.map((event: { type: string }) => event.type)).toEqual(
      expect.arrayContaining(["RunStarted", "RunFinished"]),
    );

    const helloWorldLogPath = smithersLogPath(cwd, launched.runId);
    await waitFor("hello_world execution log", () =>
      fileContains(helloWorldLogPath, '"type":"RunFinished"'),
    );
    expect(readFileSync(helloWorldLogPath, "utf8")).toContain('"type":"RunFinished"');

    const nodeDetail = await manager.getNodeDetail({
      runId: launched.runId,
      nodeId: "result",
    });
    expect(nodeDetail.node.nodeId).toBe("result");
    expect(nodeDetail.node.outputTable).toBeTruthy();
    expect(nodeDetail.attempts.length).toBeGreaterThan(0);

    const artifacts = await manager.listArtifacts({ runId: launched.runId });
    expect(artifacts).toMatchObject({
      artifacts: expect.any(Array),
    });
    expect(artifacts).not.toHaveProperty("outputs");
    expect(artifacts).not.toHaveProperty("frames");
  });

  it("cancels a paused approval run by terminalizing it like Smithers server cancellation", async () => {
    const { cwd, store, manager, sessionId, threadId, surfacePiSessionId, handlerAttentions } =
      createWorkspaceFixture();
    registerWorkflow(manager, createApprovalWorkflowDefinition(smithersDbPath(cwd)));

    const command = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Launch approval workflow",
      title: "Run approval_gate",
      summary: "Launch the approval_gate workflow.",
      toolName: "smithers_run_workflow",
    });
    const launched = await manager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "approval_gate",
      launchInput: { title: "Approve before cancel?" },
      commandId: command.commandId,
    });

    await waitFor("approval workflow wait", async () => {
      const run = await manager.getRun(launched.runId);
      return run.status === "waiting-approval";
    });

    const cancelled = await manager.cancelRun(launched.runId);

    expect(cancelled).toMatchObject({
      ok: true,
      runId: launched.runId,
      status: "cancelled",
    });
    expect(await manager.getRun(launched.runId)).toMatchObject({
      runId: launched.runId,
      status: "cancelled",
      waitKind: null,
    });
    const events = await manager.getRunEvents({ runId: launched.runId });
    expect(events.map((event: { type: string }) => event.type)).toContain("RunCancelled");

    const snapshot = store.getSessionState(sessionId);
    expect(
      snapshot.workflowRuns.find((entry) => entry.smithersRunId === launched.runId),
    ).toMatchObject({
      status: "running",
      smithersStatus: "running",
      waitKind: null,
    });
    expect(snapshot.threads.find((thread) => thread.id === threadId)).toMatchObject({
      status: "troubleshooting",
      wait: null,
    });
    expect(snapshot.session.wait).toBeNull();
    expect(handlerAttentions.some((event) => event.reason.includes("was cancelled"))).toBe(true);
  });

  it("cancels a paused timer run by cancelling the waiting timer attempt before RunCancelled", async () => {
    const { cwd, store, manager, sessionId, threadId, surfacePiSessionId } =
      createWorkspaceFixture();
    registerWorkflow(manager, createTimerWorkflowDefinition(smithersDbPath(cwd)));

    const command = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Launch timer workflow",
      title: "Run wait_for_timer",
      summary: "Launch the wait_for_timer workflow.",
      toolName: "smithers_run_workflow",
    });
    const launched = await manager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "wait_for_timer",
      launchInput: { duration: "1h" },
      commandId: command.commandId,
    });

    await waitFor("timer workflow wait", async () => {
      const run = await manager.getRun(launched.runId);
      return run.status === "waiting-timer";
    });

    await manager.cancelRun(launched.runId);

    expect(await manager.getRun(launched.runId)).toMatchObject({
      runId: launched.runId,
      status: "cancelled",
      waitKind: null,
    });
    const detail = await manager.getNodeDetail({
      runId: launched.runId,
      nodeId: "sleep",
    });
    expect(detail.node).toMatchObject({
      nodeId: "sleep",
      state: "cancelled",
    });
    expect(detail.attempts).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          state: "cancelled",
        }),
      ]),
    );
    const events = await manager.getRunEvents({ runId: launched.runId });
    expect(events.map((event: { type: string }) => event.type)).toEqual(
      expect.arrayContaining(["TimerCancelled", "RunCancelled"]),
    );
    const snapshot = store.getSessionState(sessionId);
    expect(
      snapshot.workflowRuns.find((entry) => entry.smithersRunId === launched.runId),
    ).toMatchObject({
      status: "running",
      smithersStatus: "running",
      waitKind: null,
    });
  });

  it("does not invent direct cancellation for waiting-event runs because Smithers does not", async () => {
    const { cwd, manager, sessionId, threadId, surfacePiSessionId, store } =
      createWorkspaceFixture();
    registerWorkflow(manager, createSignalWorkflowDefinition(smithersDbPath(cwd)));

    const command = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Launch signal workflow",
      title: "Run wait_for_signal",
      summary: "Launch the wait_for_signal workflow.",
      toolName: "smithers_run_workflow",
    });
    const launched = await manager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "wait_for_signal",
      launchInput: { signalName: "deploy.completed" },
      commandId: command.commandId,
    });

    await waitFor("signal workflow wait", async () => {
      const run = await manager.getRun(launched.runId);
      return run.status === "waiting-event";
    });

    await expect(manager.cancelRun(launched.runId)).rejects.toThrow("Run is not currently active");
    expect(await manager.getRun(launched.runId)).toMatchObject({
      runId: launched.runId,
      status: "waiting-event",
    });
  });

  it("resumes exactly the supplied runId instead of inferring a run from workflowId", async () => {
    const { cwd, store, manager, sessionId, threadId, surfacePiSessionId } =
      createWorkspaceFixture();
    registerWorkflow(manager, createSlowWorkflowDefinition(smithersDbPath(cwd)));

    const launchCommand = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Launch slow workflow",
      title: "Run slow_resume",
      summary: "Launch the slow_resume workflow.",
      toolName: "smithers_run_workflow",
    });
    const launched = await manager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "slow_resume",
      launchInput: { delayMs: 1_000, message: "explicit resume target" },
      commandId: launchCommand.commandId,
    });

    await waitFor("slow workflow running before explicit resume", async () => {
      const run = await manager.getRun(launched.runId);
      return run.status === "running";
    });

    const resumeCommand = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Resume slow workflow",
      title: "Resume slow_resume",
      summary: "Resume the slow_resume workflow.",
      toolName: "smithers_run_workflow",
    });
    const resumed = await manager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "slow_resume",
      launchInput: { delayMs: 1_000, message: "explicit resume target" },
      commandId: resumeCommand.commandId,
      runId: launched.runId,
    });

    expect(resumed.runId).toBe(launched.runId);
    expect(resumed.resumedRunId).toBe(launched.runId);
    expect(resumed.structuredWorkflowRunId).toBe(launched.structuredWorkflowRunId);
    expect(
      store
        .getSessionState(sessionId)
        .workflowRuns.filter((entry) => entry.workflowName === "slow_resume"),
    ).toHaveLength(1);
  });

  it("derives finished run output from Smithers even when structured execution fields are stale", async () => {
    const fixture = createWorkspaceFixture();
    const { manager, store, cwd, sessionId, threadId } = fixture;

    registerWorkflow(manager, createHelloWorldTestWorkflow(smithersDbPath(cwd)));
    await manager.refreshWorkflowRegistry();

    const workflowCommand = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId: fixture.surfacePiSessionId,
      requestSummary: "Run hello_world and then inspect the finished run.",
      title: "Run smithers_run_workflow",
      summary: "Launch hello_world for structured state sync coverage.",
    });

    const launched = await manager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "hello_world",
      launchInput: {
        message: "sync structured state before handler handoff",
      },
      commandId: workflowCommand.commandId,
    });

    await waitFor("hello_world Smithers completion", async () => {
      const run = await manager.getRun(launched.runId);
      return run.status === "finished";
    });

    const completedWorkflowRun = store
      .getSessionState(sessionId)
      .workflowRuns.find((run) => run.smithersRunId === launched.runId);
    expect(completedWorkflowRun).not.toBeUndefined();

    store.updateWorkflow({
      workflowId: completedWorkflowRun!.id,
      commandId: completedWorkflowRun!.commandId,
      status: "running",
      smithersStatus: "running",
      waitKind: "approval",
      heartbeatAt: "2001-01-01T00:00:00.000Z",
      summary: "Stale structured state before getRun sync.",
    });

    const run = await manager.getRun(launched.runId);
    expect(run.status).toBe("finished");
    expect(run.waitKind).toBeNull();
    expect(run.heartbeatAt).not.toBe("2001-01-01T00:00:00.000Z");
    expect(run.summary).toContain("svvy-hello-world is completed");

    const refreshedWorkflowRun = store
      .getSessionState(sessionId)
      .workflowRuns.find((entry) => entry.id === completedWorkflowRun!.id);
    expect(refreshedWorkflowRun?.status).toBe("running");
    expect(refreshedWorkflowRun?.smithersStatus).toBe("running");
    expect(refreshedWorkflowRun?.waitKind).toBe("approval");
    expect(refreshedWorkflowRun?.summary).toBe("Stale structured state before getRun sync.");
  });

  it("lists workspace-global runs with svvy session and thread ownership metadata", async () => {
    const { manager, store, sessionId, threadId, surfacePiSessionId } = createWorkspaceFixture();

    const secondarySessionId = "session-smithers-runtime-secondary";
    store.upsertPiSession({
      sessionId: secondarySessionId,
      title: "Secondary Smithers Runtime Session",
      provider: "openai",
      model: "gpt-5.4",
      reasoningEffort: "medium",
      messageCount: 1,
      status: "running",
      createdAt: "2026-04-20T08:05:00.000Z",
      updatedAt: "2026-04-20T08:05:00.000Z",
    });
    const secondarySeedTurn = store.startTurn({
      sessionId: secondarySessionId,
      surfacePiSessionId: secondarySessionId,
      requestSummary: "Open a second handler thread for workflow supervision",
    });
    const secondaryHandlerThread = store.createThread({
      turnId: secondarySeedTurn.id,
      surfacePiSessionId: "pi-thread-smithers-runtime-secondary",
      title: "Secondary workflow supervisor",
      objective: "Supervise additional Smithers workflow entries.",
    });
    store.finishTurn({
      turnId: secondarySeedTurn.id,
      status: "completed",
    });

    const firstLaunchCommand = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Launch primary hello world",
      title: "Run primary hello_world",
      summary: "Launch the primary hello_world workflow.",
      toolName: "smithers_run_workflow",
    });
    const secondLaunchCommand = createWorkflowCommand({
      store,
      sessionId: secondarySessionId,
      threadId: secondaryHandlerThread.id,
      surfacePiSessionId: secondaryHandlerThread.surfacePiSessionId,
      requestSummary: "Launch secondary hello world",
      title: "Run secondary hello_world",
      summary: "Launch the secondary hello_world workflow.",
      toolName: "smithers_run_workflow",
    });

    const primaryLaunch = await manager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "hello_world",
      launchInput: { message: "primary ownership" },
      commandId: firstLaunchCommand.commandId,
    });
    const secondaryLaunch = await manager.launchWorkflow({
      sessionId: secondarySessionId,
      threadId: secondaryHandlerThread.id,
      workflowId: "hello_world",
      launchInput: { message: "secondary ownership" },
      commandId: secondLaunchCommand.commandId,
    });

    await waitFor("workspace-global hello_world completion", async () => {
      try {
        const [primaryRun, secondaryRun] = await Promise.all([
          manager.getRun(primaryLaunch.runId),
          manager.getRun(secondaryLaunch.runId),
        ]);
        return primaryRun.status === "finished" && secondaryRun.status === "finished";
      } catch {
        return false;
      }
    });

    const runs = await manager.listRuns({ workflowId: "hello_world" });
    expect(runs).toHaveLength(2);
    expect(runs).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          runId: primaryLaunch.runId,
          workflowName: "hello_world",
          workflowId: "hello_world",
          workflowSource: "saved",
          status: "finished",
          sessionId,
          threadId,
        }),
        expect.objectContaining({
          runId: secondaryLaunch.runId,
          workflowName: "hello_world",
          workflowId: "hello_world",
          workflowSource: "saved",
          status: "finished",
          sessionId: secondarySessionId,
          threadId: secondaryHandlerThread.id,
        }),
      ]),
    );
  });

  it("rejects omitted runId when the same handler already owns a nonterminal run for that workflow", async () => {
    const { cwd, store, manager, sessionId, threadId, surfacePiSessionId } =
      createWorkspaceFixture();
    registerWorkflow(manager, createApprovalWorkflowDefinition(smithersDbPath(cwd)));

    const firstCommand = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Launch approval workflow",
      title: "Run approval_gate",
      summary: "Launch the approval_gate workflow.",
      toolName: "smithers_run_workflow",
    });
    const launched = await manager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "approval_gate",
      launchInput: { title: "Approve the release?" },
      commandId: firstCommand.commandId,
    });

    await waitFor("approval workflow to reach nonterminal wait", async () => {
      try {
        const run = await manager.getRun(launched.runId);
        return run.status === "waiting-approval";
      } catch {
        return false;
      }
    });

    const secondCommand = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Launch approval workflow again",
      title: "Run approval_gate again",
      summary: "Attempt a fresh approval_gate workflow launch.",
      toolName: "smithers_run_workflow",
    });
    await expect(
      manager.launchWorkflow({
        sessionId,
        threadId,
        workflowId: "approval_gate",
        launchInput: { title: "Launch another release approval?" },
        commandId: secondCommand.commandId,
      }),
    ).rejects.toThrow(
      `already owns nonterminal Smithers run ${launched.runId} for workflow approval_gate`,
    );

    const snapshot = store.getSessionState(sessionId);
    expect(
      snapshot.workflowRuns.filter((entry) => entry.workflowName === "approval_gate"),
    ).toHaveLength(1);
    expect(await manager.getRun(launched.runId)).toMatchObject({
      runId: launched.runId,
      status: "waiting-approval",
    });
  });

  it("allows one handler to launch different workflowIds concurrently when runId is omitted", async () => {
    const { cwd, store, manager, sessionId, threadId, surfacePiSessionId } =
      createWorkspaceFixture();
    registerWorkflow(manager, createApprovalWorkflowDefinition(smithersDbPath(cwd)));
    registerWorkflow(manager, createSignalWorkflowDefinition(smithersDbPath(cwd)));

    const approvalCommand = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Launch approval workflow",
      title: "Run approval_gate",
      summary: "Launch the approval_gate workflow.",
      toolName: "smithers_run_workflow",
    });
    const approvalLaunch = await manager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "approval_gate",
      launchInput: { title: "Approve alongside signal?" },
      commandId: approvalCommand.commandId,
    });

    await waitFor("approval workflow to wait before concurrent launch", async () => {
      try {
        const run = await manager.getRun(approvalLaunch.runId);
        return run.status === "waiting-approval";
      } catch {
        return false;
      }
    });

    const signalCommand = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Launch signal workflow",
      title: "Run wait_for_signal",
      summary: "Launch the wait_for_signal workflow.",
      toolName: "smithers_run_workflow",
    });
    const signalLaunch = await manager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "wait_for_signal",
      launchInput: { signalName: "deploy.completed" },
      commandId: signalCommand.commandId,
    });

    expect(signalLaunch.runId).not.toBe(approvalLaunch.runId);
    expect(signalLaunch.resumedRunId).toBeNull();

    await waitFor("signal workflow to wait without cancelling approval workflow", async () => {
      try {
        const [approvalRun, signalRun] = await Promise.all([
          manager.getRun(approvalLaunch.runId),
          manager.getRun(signalLaunch.runId),
        ]);
        return approvalRun.status === "waiting-approval" && signalRun.status === "waiting-event";
      } catch {
        return false;
      }
    });

    const snapshot = store.getSessionState(sessionId);
    expect(snapshot.workflowRuns).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          smithersRunId: approvalLaunch.runId,
          workflowName: "approval_gate",
        }),
        expect.objectContaining({
          smithersRunId: signalLaunch.runId,
          workflowName: "wait_for_signal",
        }),
      ]),
    );
  });

  it("treats a replayed terminal workflow projection as a no-op after the handler already handed off", async () => {
    const { store, manager, sessionId, threadId, surfacePiSessionId, handlerAttentions } =
      createWorkspaceFixture();

    const launchCommand = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Launch hello world",
      title: "Run hello_world",
      summary: "Launch the hello_world workflow.",
      toolName: "smithers_run_workflow",
    });
    const launched = await manager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "hello_world",
      launchInput: { message: "bonjour smithers" },
      commandId: launchCommand.commandId,
    });

    await waitFor("hello_world completion", async () => {
      try {
        const run = await manager.getRun(launched.runId);
        return run.status === "finished";
      } catch {
        return false;
      }
    });
    await waitFor("hello_world handler attention", () =>
      handlerAttentions.some((event) =>
        event.reason.includes("finished and the handler must reconcile"),
      ),
    );

    store.updateThread({
      threadId,
      status: "completed",
      wait: null,
    });
    const priorAttentionCount = handlerAttentions.length;

    await (manager as any).flushRunEvents(launched.runId);

    const snapshot = store.getSessionState(sessionId);
    expect(snapshot.threads.find((thread) => thread.id === threadId)).toMatchObject({
      status: "completed",
      wait: null,
    });
    expect(snapshot.session.wait).toBeNull();
    expect(handlerAttentions).toHaveLength(priorAttentionCount);
  });

  it("keeps handler attention pending until an explicit delivery succeeds", async () => {
    let shouldDeliverAttention = false;
    const { store, manager, sessionId, threadId, surfacePiSessionId, handlerAttentions } =
      createWorkspaceFixture({
        onHandlerAttention: async () => shouldDeliverAttention,
      });

    const launchCommand = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Launch hello world",
      title: "Run hello_world",
      summary: "Launch the hello_world workflow.",
      toolName: "smithers_run_workflow",
    });
    const launched = await manager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "hello_world",
      launchInput: { message: "leave attention pending" },
      commandId: launchCommand.commandId,
    });

    await waitFor("hello_world completion with pending attention", async () => {
      try {
        const run = await manager.getRun(launched.runId);
        return run.status === "finished" && run.pendingAttentionSeq !== null;
      } catch {
        return false;
      }
    });
    await waitFor("first undelivered attention attempt", () => handlerAttentions.length === 1);

    let snapshot = store.getSessionState(sessionId);
    let workflowRun = snapshot.workflowRuns.find((entry) => entry.smithersRunId === launched.runId);
    expect(workflowRun).toMatchObject({
      pendingAttentionSeq: expect.any(Number),
      lastAttentionSeq: null,
    });

    shouldDeliverAttention = true;
    await manager.deliverPendingHandlerAttention(sessionId, threadId);

    snapshot = store.getSessionState(sessionId);
    workflowRun = snapshot.workflowRuns.find((entry) => entry.smithersRunId === launched.runId);
    expect(handlerAttentions).toHaveLength(2);
    expect(workflowRun).toMatchObject({
      pendingAttentionSeq: null,
      lastAttentionSeq: expect.any(Number),
    });

    const run = await manager.getRun(launched.runId);
    expect(run).toMatchObject({
      pendingAttentionSeq: null,
      lastAttentionSeq: workflowRun?.lastAttentionSeq,
      structuredWorkflowRunId: workflowRun?.id,
    });
  });

  it("lets a handler attention turn inspect the same terminal run without deadlocking delivery", async () => {
    let managerDuringAttention: SmithersRuntimeManager | null = null;
    const inspectedStatuses: string[] = [];
    const { store, manager, sessionId, threadId, surfacePiSessionId, handlerAttentions } =
      createWorkspaceFixture({
        onHandlerAttention: async (event) => {
          const run = await managerDuringAttention?.getRun(event.smithersRunId);
          if (run) {
            inspectedStatuses.push(run.status);
          }
          return true;
        },
      });
    managerDuringAttention = manager;

    const launchCommand = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Launch hello world",
      title: "Run hello_world",
      summary: "Launch the hello_world workflow.",
      toolName: "smithers_run_workflow",
    });
    const launched = await manager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "hello_world",
      launchInput: { message: "inspect during attention" },
      commandId: launchCommand.commandId,
    });

    await waitFor("handler attention inspection", () => inspectedStatuses.includes("finished"));

    const snapshot = store.getSessionState(sessionId);
    const workflowRun = snapshot.workflowRuns.find(
      (entry) => entry.smithersRunId === launched.runId,
    );
    expect(handlerAttentions).toHaveLength(1);
    expect(workflowRun).toMatchObject({
      pendingAttentionSeq: null,
      lastAttentionSeq: expect.any(Number),
    });
  });

  it("restores pending workflow supervision from durable state after recreating the manager", async () => {
    let shouldDeliverAttention = false;
    const {
      cwd,
      agentDir,
      store,
      manager,
      sessionId,
      threadId,
      surfacePiSessionId,
      structuredStateChanges,
    } = createWorkspaceFixture({
      onHandlerAttention: async () => shouldDeliverAttention,
    });

    const launchCommand = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Launch hello world",
      title: "Run hello_world",
      summary: "Launch the hello_world workflow.",
      toolName: "smithers_run_workflow",
    });
    const launched = await manager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "hello_world",
      launchInput: { message: "recover pending attention" },
      commandId: launchCommand.commandId,
    });

    await waitFor("hello_world completion before manager restart", async () => {
      try {
        const run = await manager.getRun(launched.runId);
        return run.status === "finished" && run.pendingAttentionSeq !== null;
      } catch {
        return false;
      }
    });

    const beforeRestart = store
      .getSessionState(sessionId)
      .workflowRuns.find((entry) => entry.smithersRunId === launched.runId);
    expect(beforeRestart).toMatchObject({
      pendingAttentionSeq: expect.any(Number),
      lastAttentionSeq: null,
    });

    await manager.close();
    managers.splice(managers.indexOf(manager), 1);

    shouldDeliverAttention = true;
    const restoredStructuredStateChanges: string[] = [];
    const restoredHandlerAttentions: HandlerAttentionEvent[] = [];
    const restoredManager = createManagerHarness({
      cwd,
      agentDir,
      store,
      structuredStateChanges: restoredStructuredStateChanges,
      handlerAttentions: restoredHandlerAttentions,
      onHandlerAttention: async () => shouldDeliverAttention,
    });

    await restoredManager.restoreSessionSupervision(sessionId);

    const afterRestore = store
      .getSessionState(sessionId)
      .workflowRuns.find((entry) => entry.smithersRunId === launched.runId);
    expect(restoredHandlerAttentions).toHaveLength(1);
    expect(afterRestore).toMatchObject({
      id: beforeRestart?.id,
      pendingAttentionSeq: null,
      lastAttentionSeq: beforeRestart?.pendingAttentionSeq,
    });
    expect(restoredStructuredStateChanges).toContain(sessionId);
    expect(structuredStateChanges).toContain(sessionId);

    const restoredRun = await restoredManager.getRun(launched.runId);
    expect(restoredRun).toMatchObject({
      runId: launched.runId,
      structuredWorkflowRunId: beforeRestart?.id,
      threadId,
      pendingAttentionSeq: null,
      lastAttentionSeq: beforeRestart?.pendingAttentionSeq,
    });
  });

  it("reattaches a running workflow after manager restart and resumes the same Smithers run", async () => {
    let shouldDeliverAttention = false;
    const { cwd, agentDir, store, manager, sessionId, threadId, surfacePiSessionId } =
      createWorkspaceFixture({
        onHandlerAttention: async () => shouldDeliverAttention,
      });
    registerWorkflow(manager, createSlowWorkflowDefinition(smithersDbPath(cwd)));

    const launchCommand = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Launch slow workflow",
      title: "Run slow_resume",
      summary: "Launch the slow_resume workflow.",
      toolName: "smithers_run_workflow",
    });
    const launched = await manager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "slow_resume",
      launchInput: { delayMs: 1_500, message: "recover running run" },
      commandId: launchCommand.commandId,
    });

    await waitFor("slow workflow running before restart", async () => {
      try {
        const run = await manager.getRun(launched.runId);
        return run.status === "running";
      } catch {
        return false;
      }
    });

    (manager as any).monitorByRunId.get(launched.runId)?.abortController.abort();
    await (manager as any).db.updateRun(launched.runId, {
      runtimeOwnerId: "pid:0:svvy-test-dead-owner",
      heartbeatAtMs: Date.now() - 60_000,
    });

    shouldDeliverAttention = true;
    const restoredHandlerAttentions: HandlerAttentionEvent[] = [];
    const restoredManager = createManagerHarness({
      cwd,
      agentDir,
      store,
      structuredStateChanges: [],
      handlerAttentions: restoredHandlerAttentions,
      onHandlerAttention: async () => shouldDeliverAttention,
    });
    registerWorkflow(restoredManager, createSlowWorkflowDefinition(smithersDbPath(cwd)));

    await restoredManager.restoreSessionSupervision(sessionId);

    await waitFor("slow workflow completion after restart resume", async () => {
      try {
        const run = await restoredManager.getRun(launched.runId);
        return run.status === "finished";
      } catch {
        return false;
      }
    });

    const snapshot = store.getSessionState(sessionId);
    const workflowRuns = snapshot.workflowRuns.filter(
      (entry) => entry.smithersRunId === launched.runId,
    );
    expect(workflowRuns).toHaveLength(1);
    expect(workflowRuns[0]).toMatchObject({
      id: launched.structuredWorkflowRunId,
    });
    expect(await restoredManager.getRun(launched.runId)).toMatchObject({
      status: "finished",
      structuredWorkflowRunId: launched.structuredWorkflowRunId,
    });

    shouldDeliverAttention = false;
  });

  it("restores approval attention after restart without creating a duplicate workflow projection", async () => {
    let shouldDeliverAttention = false;
    const { cwd, agentDir, store, manager, sessionId, threadId, surfacePiSessionId } =
      createWorkspaceFixture({
        onHandlerAttention: async () => shouldDeliverAttention,
      });
    registerWorkflow(manager, createApprovalWorkflowDefinition(smithersDbPath(cwd)));

    const launchCommand = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Launch approval workflow",
      title: "Run approval_gate",
      summary: "Launch the approval_gate workflow.",
      toolName: "smithers_run_workflow",
    });
    const launched = await manager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "approval_gate",
      launchInput: { title: "Approve after restart?" },
      commandId: launchCommand.commandId,
    });

    await waitFor("approval wait before restart", async () => {
      try {
        const run = await manager.getRun(launched.runId);
        return run.status === "waiting-approval";
      } catch {
        return false;
      }
    });

    await manager.close();
    managers.splice(managers.indexOf(manager), 1);

    shouldDeliverAttention = true;
    const restoredHandlerAttentions: HandlerAttentionEvent[] = [];
    const restoredManager = createManagerHarness({
      cwd,
      agentDir,
      store,
      structuredStateChanges: [],
      handlerAttentions: restoredHandlerAttentions,
      onHandlerAttention: async () => shouldDeliverAttention,
    });
    registerWorkflow(restoredManager, createApprovalWorkflowDefinition(smithersDbPath(cwd)));

    await restoredManager.restoreSessionSupervision(sessionId);
    await restoredManager.restoreSessionSupervision(sessionId);

    let snapshot = store.getSessionState(sessionId);
    expect(
      snapshot.workflowRuns.filter((entry) => entry.smithersRunId === launched.runId),
    ).toHaveLength(1);
    expect(restoredHandlerAttentions).toHaveLength(1);
    expect(restoredHandlerAttentions[0]).toMatchObject({
      smithersRunId: launched.runId,
      reason: expect.stringContaining("waiting on approval"),
    });
    expect(
      snapshot.workflowRuns.find((entry) => entry.smithersRunId === launched.runId),
    ).toMatchObject({
      pendingAttentionSeq: null,
      lastAttentionSeq: expect.any(Number),
    });

    await restoredManager.resolveApproval({
      runId: launched.runId,
      nodeId: "publish-gate",
      action: "approve",
      note: "Approved after restart.",
    });
    const resumeCommand = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Resume approval workflow",
      title: "Resume approval_gate",
      summary: "Resume the approval_gate workflow.",
      toolName: "smithers_run_workflow",
    });
    await restoredManager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "approval_gate",
      launchInput: { title: "Approve after restart?" },
      commandId: resumeCommand.commandId,
      runId: launched.runId,
    });
    await waitFor("approval workflow completion after restart", async () => {
      const run = await restoredManager.getRun(launched.runId);
      return run.status === "finished";
    });

    snapshot = store.getSessionState(sessionId);
    expect(
      snapshot.workflowRuns.find((entry) => entry.smithersRunId === launched.runId),
    ).toMatchObject({
      status: "running",
      smithersStatus: "running",
    });
    expect(await restoredManager.getRun(launched.runId)).toMatchObject({
      status: "finished",
    });

    shouldDeliverAttention = false;
  });

  it("restores signal attention after restart and resumes the same run after the signal arrives", async () => {
    let shouldDeliverAttention = false;
    const { cwd, agentDir, store, manager, sessionId, threadId, surfacePiSessionId } =
      createWorkspaceFixture({
        onHandlerAttention: async () => shouldDeliverAttention,
      });
    registerWorkflow(manager, createSignalWorkflowDefinition(smithersDbPath(cwd)));

    const launchCommand = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Launch signal workflow",
      title: "Run wait_for_signal",
      summary: "Launch the wait_for_signal workflow.",
      toolName: "smithers_run_workflow",
    });
    const launched = await manager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "wait_for_signal",
      launchInput: { signalName: "deploy.completed" },
      commandId: launchCommand.commandId,
    });

    await waitFor("signal wait before restart", async () => {
      try {
        const run = await manager.getRun(launched.runId);
        return run.status === "waiting-event";
      } catch {
        return false;
      }
    });

    await manager.close();
    managers.splice(managers.indexOf(manager), 1);

    shouldDeliverAttention = true;
    const restoredHandlerAttentions: HandlerAttentionEvent[] = [];
    const restoredManager = createManagerHarness({
      cwd,
      agentDir,
      store,
      structuredStateChanges: [],
      handlerAttentions: restoredHandlerAttentions,
      onHandlerAttention: async () => shouldDeliverAttention,
    });
    registerWorkflow(restoredManager, createSignalWorkflowDefinition(smithersDbPath(cwd)));

    await restoredManager.restoreSessionSupervision(sessionId);

    expect(restoredHandlerAttentions).toHaveLength(1);
    expect(restoredHandlerAttentions[0]).toMatchObject({
      smithersRunId: launched.runId,
      reason: expect.stringContaining("waiting on an external event or signal"),
    });

    await restoredManager.sendSignal({
      runId: launched.runId,
      signalName: "deploy.completed",
      data: {
        environment: "production",
        sha: "restart123",
        status: "success",
      },
    });
    const resumeCommand = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Resume signal workflow",
      title: "Resume wait_for_signal",
      summary: "Resume the wait_for_signal workflow.",
      toolName: "smithers_run_workflow",
    });
    const resumed = await restoredManager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "wait_for_signal",
      launchInput: { signalName: "deploy.completed" },
      commandId: resumeCommand.commandId,
      runId: launched.runId,
    });

    expect(resumed.structuredWorkflowRunId).toBe(launched.structuredWorkflowRunId);
    await waitFor("signal workflow completion after restart", async () => {
      const run = await restoredManager.getRun(launched.runId);
      return run.status === "finished";
    });

    const snapshot = store.getSessionState(sessionId);
    expect(
      snapshot.workflowRuns.filter((entry) => entry.smithersRunId === launched.runId),
    ).toHaveLength(1);
    expect(
      snapshot.workflowRuns.find((entry) => entry.smithersRunId === launched.runId),
    ).toMatchObject({
      status: "running",
      smithersStatus: "running",
      waitKind: null,
    });
    expect(await restoredManager.getRun(launched.runId)).toMatchObject({
      status: "finished",
      waitKind: null,
    });
  });

  it("does not resume terminal workflow runs during restart restoration", async () => {
    const { cwd, agentDir, store, manager, sessionId, threadId, surfacePiSessionId } =
      createWorkspaceFixture({
        onHandlerAttention: async () => true,
      });

    const launchCommand = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Launch hello world",
      title: "Run hello_world",
      summary: "Launch the hello_world workflow.",
      toolName: "smithers_run_workflow",
    });
    const launched = await manager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "hello_world",
      launchInput: { message: "terminal no resume" },
      commandId: launchCommand.commandId,
    });

    await waitFor("hello_world terminal before restart", async () => {
      const run = await manager.getRun(launched.runId);
      return run.status === "finished";
    });
    await waitFor("terminal attention delivered before restart", () => {
      const workflowRun = store
        .getSessionState(sessionId)
        .workflowRuns.find((entry) => entry.smithersRunId === launched.runId);
      return workflowRun?.pendingAttentionSeq === null && workflowRun?.lastAttentionSeq !== null;
    });

    await manager.close();
    managers.splice(managers.indexOf(manager), 1);

    const restoredHandlerAttentions: HandlerAttentionEvent[] = [];
    const restoredManager = createManagerHarness({
      cwd,
      agentDir,
      store,
      structuredStateChanges: [],
      handlerAttentions: restoredHandlerAttentions,
      onHandlerAttention: async () => true,
    });
    registerWorkflow(restoredManager, createHelloWorldTestWorkflow(smithersDbPath(cwd)));

    await restoredManager.restoreSessionSupervision(sessionId);

    expect((restoredManager as any).activeWorkflowPromiseByRunId.size).toBe(0);
    expect(restoredHandlerAttentions).toHaveLength(0);
    expect(store.getSessionState(sessionId).workflowRuns).toHaveLength(1);
    expect(
      store
        .getSessionState(sessionId)
        .workflowRuns.find((entry) => entry.smithersRunId === launched.runId),
    ).toMatchObject({
      id: launched.structuredWorkflowRunId,
      status: "running",
      smithersStatus: "running",
    });
  });

  it("waits on approval, resumes the same run after approval, and finishes with the real Smithers monitor path", async () => {
    const { cwd, store, manager, sessionId, threadId, surfacePiSessionId, handlerAttentions } =
      createWorkspaceFixture();
    registerWorkflow(manager, createApprovalWorkflowDefinition(smithersDbPath(cwd)));

    const launchCommand = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Launch approval workflow",
      title: "Run approval_gate",
      summary: "Launch the approval_gate workflow.",
      toolName: "smithers_run_workflow",
    });
    const launched = await manager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "approval_gate",
      launchInput: { title: "Approve the release?" },
      commandId: launchCommand.commandId,
    });

    await waitFor("approval wait state", async () => {
      try {
        const run = await manager.getRun(launched.runId);
        return run.status === "waiting-approval";
      } catch {
        return false;
      }
    });

    let snapshot = store.getSessionState(sessionId);
    let workflowRun = snapshot.workflowRuns.find((entry) => entry.smithersRunId === launched.runId);
    expect(workflowRun).toMatchObject({
      status: "running",
      smithersStatus: "running",
      waitKind: null,
    });
    expect(snapshot.threads.find((thread) => thread.id === threadId)).toMatchObject({
      id: threadId,
      status: "waiting",
      wait: expect.objectContaining({
        owner: "workflow",
        kind: "approval",
      }),
    });
    expect(snapshot.session.wait).toMatchObject({
      owner: { kind: "thread", threadId },
      kind: "approval",
    });

    const approvals = await manager.listPendingApprovals({ runId: launched.runId });
    expect(approvals).toHaveLength(1);
    expect(approvals[0]).toMatchObject({
      runId: launched.runId,
      nodeId: "publish-gate",
      status: "requested",
    });

    const explanation = await manager.explainRun(launched.runId);
    expect(explanation.summary).toContain("waiting-approval");
    expect(explanation.diagnosis.blockers.length).toBeGreaterThan(0);

    await manager.resolveApproval({
      runId: launched.runId,
      nodeId: "publish-gate",
      action: "approve",
      note: "Ship it.",
    });

    await waitFor("post-approval run status", async () => {
      try {
        const run = await manager.getRun(launched.runId);
        return run.status === "waiting-event";
      } catch {
        return false;
      }
    });

    const resumeCommand = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Resume approval workflow",
      title: "Resume approval_gate",
      summary: "Resume the approval_gate workflow.",
      toolName: "smithers_run_workflow",
    });
    const resumed = await manager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "approval_gate",
      launchInput: { title: "Approve the release?" },
      commandId: resumeCommand.commandId,
      runId: launched.runId,
    });

    expect(resumed.structuredWorkflowRunId).toBe(launched.structuredWorkflowRunId);

    await waitFor("approval workflow completion", async () => {
      try {
        const run = await manager.getRun(launched.runId);
        return run.status === "finished";
      } catch {
        return false;
      }
    });
    await waitFor("approval workflow final attention", () =>
      handlerAttentions.some((event) =>
        event.reason.includes("finished and the handler must reconcile"),
      ),
    );

    snapshot = store.getSessionState(sessionId);
    workflowRun = snapshot.workflowRuns.find((entry) => entry.smithersRunId === launched.runId);
    expect(workflowRun).toMatchObject({
      status: "running",
      smithersStatus: "running",
      waitKind: null,
    });
    expect(snapshot.threads.find((thread) => thread.id === threadId)).toMatchObject({
      id: threadId,
      status: "running-handler",
      wait: null,
    });
    expect(snapshot.session.wait).toBeNull();

    const run = await manager.getRun(launched.runId);
    expect(run.status).toBe("finished");

    const events = await manager.getRunEvents({ runId: launched.runId });
    expect(events.map((event: { type: string }) => event.type)).toEqual(
      expect.arrayContaining(["ApprovalRequested", "ApprovalGranted", "RunFinished"]),
    );

    const approvalLogPath = smithersLogPath(cwd, launched.runId);
    await waitFor("approval execution log", () =>
      fileContains(approvalLogPath, '"type":"RunFinished"'),
    );
    expect(readFileSync(approvalLogPath, "utf8")).toContain('"type":"RunFinished"');

    const detail = await manager.getNodeDetail({
      runId: launched.runId,
      nodeId: "record-decision",
    });
    expect(detail.node.nodeId).toBe("record-decision");
    expect(detail.node.outputTable).toBeTruthy();
    expect(detail.attempts.length).toBeGreaterThan(0);

    expect(handlerAttentions.some((event) => event.reason.includes("waiting on approval"))).toBe(
      true,
    );
    expect(
      handlerAttentions.some((event) =>
        event.reason.includes("finished and the handler must reconcile"),
      ),
    ).toBe(true);
  });

  it("cancels a restored waiting approval run directly and clears structured waits", async () => {
    const { cwd, agentDir, store, manager, sessionId, threadId, surfacePiSessionId } =
      createWorkspaceFixture();
    registerWorkflow(manager, createApprovalWorkflowDefinition(smithersDbPath(cwd)));

    const launchCommand = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Launch approval workflow",
      title: "Run approval_gate",
      summary: "Launch the approval_gate workflow.",
      toolName: "smithers_run_workflow",
    });
    const launched = await manager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "approval_gate",
      launchInput: { title: "Cancel while waiting?" },
      commandId: launchCommand.commandId,
    });

    await waitFor("approval wait before cancellation", async () => {
      try {
        const run = await manager.getRun(launched.runId);
        return run.status === "waiting-approval";
      } catch {
        return false;
      }
    });

    await manager.close();
    managers.splice(managers.indexOf(manager), 1);

    const restoredHandlerAttentions: HandlerAttentionEvent[] = [];
    const restoredManager = createManagerHarness({
      cwd,
      agentDir,
      store,
      structuredStateChanges: [],
      handlerAttentions: restoredHandlerAttentions,
    });
    registerWorkflow(restoredManager, createApprovalWorkflowDefinition(smithersDbPath(cwd)));

    await restoredManager.cancelRun(launched.runId);

    const run = await restoredManager.getRun(launched.runId);
    expect(run.status).toBe("cancelled");
    const events = await restoredManager.getRunEvents({ runId: launched.runId });
    expect(events.map((event: { type: string }) => event.type)).toContain("RunCancelled");

    const snapshot = store.getSessionState(sessionId);
    expect(
      snapshot.workflowRuns.find((entry) => entry.smithersRunId === launched.runId),
    ).toMatchObject({
      status: "running",
      smithersStatus: "running",
      waitKind: null,
    });
    expect(snapshot.threads.find((thread) => thread.id === threadId)).toMatchObject({
      id: threadId,
      status: "troubleshooting",
      wait: null,
    });
    expect(snapshot.session.wait).toBeNull();
    expect(restoredHandlerAttentions).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          smithersRunId: launched.runId,
          reason: expect.stringContaining("was cancelled"),
        }),
      ]),
    );
  });

  it("cancels a restored waiting timer run with Smithers timer cleanup", async () => {
    const { cwd, agentDir, store, manager, sessionId, threadId, surfacePiSessionId } =
      createWorkspaceFixture();
    registerWorkflow(manager, createTimerWorkflowDefinition(smithersDbPath(cwd)));

    const launchCommand = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Launch timer workflow",
      title: "Run wait_for_timer",
      summary: "Launch the wait_for_timer workflow.",
      toolName: "smithers_run_workflow",
    });
    const launched = await manager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "wait_for_timer",
      launchInput: { duration: "1h" },
      commandId: launchCommand.commandId,
    });

    await waitFor("timer wait before cancellation", async () => {
      try {
        const run = await manager.getRun(launched.runId);
        return run.status === "waiting-timer";
      } catch {
        return false;
      }
    });

    await manager.close();
    managers.splice(managers.indexOf(manager), 1);

    const restoredManager = createManagerHarness({
      cwd,
      agentDir,
      store,
      structuredStateChanges: [],
      handlerAttentions: [],
    });
    registerWorkflow(restoredManager, createTimerWorkflowDefinition(smithersDbPath(cwd)));

    await restoredManager.cancelRun(launched.runId);

    const run = await restoredManager.getRun(launched.runId);
    expect(run.status).toBe("cancelled");
    const detail = await restoredManager.getNodeDetail({
      runId: launched.runId,
      nodeId: "sleep",
    });
    expect(detail.node.state).toBe("cancelled");
    expect(detail.attempts).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          state: "cancelled",
          finishedAtMs: expect.any(Number),
        }),
      ]),
    );
    const events = await restoredManager.getRunEvents({ runId: launched.runId });
    expect(events.map((event: { type: string }) => event.type)).toEqual(
      expect.arrayContaining(["TimerCancelled", "RunCancelled"]),
    );

    const snapshot = store.getSessionState(sessionId);
    expect(
      snapshot.workflowRuns.find((entry) => entry.smithersRunId === launched.runId),
    ).toMatchObject({
      status: "running",
      smithersStatus: "running",
      waitKind: null,
    });
    expect(snapshot.session.wait).toBeNull();
  });

  it("does not invent direct cancellation for restored waiting-event runs", async () => {
    const { cwd, agentDir, store, manager, sessionId, threadId, surfacePiSessionId } =
      createWorkspaceFixture();
    registerWorkflow(manager, createSignalWorkflowDefinition(smithersDbPath(cwd)));

    const launchCommand = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Launch signal workflow",
      title: "Run wait_for_signal",
      summary: "Launch the wait_for_signal workflow.",
      toolName: "smithers_run_workflow",
    });
    const launched = await manager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "wait_for_signal",
      launchInput: { signalName: "deploy.completed" },
      commandId: launchCommand.commandId,
    });

    await waitFor("signal wait before cancellation attempt", async () => {
      try {
        const run = await manager.getRun(launched.runId);
        return run.status === "waiting-event";
      } catch {
        return false;
      }
    });

    await manager.close();
    managers.splice(managers.indexOf(manager), 1);

    const restoredManager = createManagerHarness({
      cwd,
      agentDir,
      store,
      structuredStateChanges: [],
      handlerAttentions: [],
    });
    registerWorkflow(restoredManager, createSignalWorkflowDefinition(smithersDbPath(cwd)));

    await expect(restoredManager.cancelRun(launched.runId)).rejects.toThrow(
      "Run is not currently active",
    );
    const run = await restoredManager.getRun(launched.runId);
    expect(run.status).toBe("waiting-event");
    const events = await restoredManager.getRunEvents({ runId: launched.runId });
    expect(events.map((event: { type: string }) => event.type)).not.toContain("RunCancelled");
  });

  it("keeps running cancellation on the live cancel-request path", async () => {
    const { cwd, manager, sessionId, threadId, surfacePiSessionId, store } =
      createWorkspaceFixture();
    registerWorkflow(manager, createSlowWorkflowDefinition(smithersDbPath(cwd)));

    const launchCommand = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Launch slow workflow",
      title: "Run slow_resume",
      summary: "Launch the slow_resume workflow.",
      toolName: "smithers_run_workflow",
    });
    const launched = await manager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "slow_resume",
      launchInput: { delayMs: 10_000, message: "cancel me" },
      commandId: launchCommand.commandId,
    });

    await waitFor("running workflow heartbeat before cancellation", async () => {
      try {
        const run = await manager.getRun(launched.runId);
        return run.status === "running" && run.heartbeatAt !== null;
      } catch {
        return false;
      }
    });

    await expect(manager.cancelRun(launched.runId)).resolves.toMatchObject({
      ok: true,
      runId: launched.runId,
      status: "cancel-requested",
    });
    await waitFor("running workflow cancellation to terminalize", async () => {
      try {
        const run = await manager.getRun(launched.runId);
        return run.status === "cancelled";
      } catch {
        return false;
      }
    });
  });

  it("tracks continue-as-new lineage with parent and descendant structured workflow runs", async () => {
    const { cwd, store, manager, sessionId, threadId, surfacePiSessionId, handlerAttentions } =
      createWorkspaceFixture();
    registerWorkflow(manager, createContinueAsNewWorkflowDefinition(smithersDbPath(cwd)));

    const launchCommand = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Launch continue-as-new workflow",
      title: "Run continue_once",
      summary: "Launch the continue_once workflow.",
      toolName: "smithers_run_workflow",
    });
    const launched = await manager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "continue_once",
      launchInput: {},
      commandId: launchCommand.commandId,
    });

    await waitFor("continued child run and final completion", async () => {
      const runs = store
        .getSessionState(sessionId)
        .workflowRuns.filter((entry) => entry.savedEntryId === "continue_once");
      const child = runs.find((entry) => entry.continuedFromRunIds.length > 0);
      if (!child) {
        return false;
      }
      const childRun = await manager.getRun(child.smithersRunId);
      return runs.length === 2 && childRun.status === "finished";
    });

    const snapshot = store.getSessionState(sessionId);
    const runs = snapshot.workflowRuns.filter((entry) => entry.savedEntryId === "continue_once");
    expect(runs).toHaveLength(2);
    const parent = runs.find((entry) => entry.activeDescendantRunId !== null);
    const child = runs.find((entry) => entry.continuedFromRunIds.length > 0);
    expect(parent).toBeDefined();
    expect(child).toBeDefined();
    expect(parent?.activeDescendantRunId).toBe(child?.id);
    expect(child?.continuedFromRunIds).toEqual([parent!.id]);
    expect(snapshot.threads.find((thread) => thread.id === threadId)).toMatchObject({
      id: threadId,
      status: "running-handler",
      wait: null,
    });

    const listed = await manager.listRuns({ workflowId: "continue_once" });
    expect(listed).toHaveLength(2);
    expect(listed.map((entry: { status: string }) => entry.status)).toEqual(
      expect.arrayContaining(["continued", "finished"]),
    );

    const parentEvents = await manager.getRunEvents({ runId: parent!.smithersRunId });
    expect(parentEvents.map((event: { type: string }) => event.type)).toContain(
      "RunContinuedAsNew",
    );
    await waitFor("continue-as-new final attention", () =>
      handlerAttentions.some((event) =>
        event.reason.includes("finished and the handler must reconcile"),
      ),
    );

    const childRun = await manager.getRun(child!.smithersRunId);
    expect(childRun).toMatchObject({
      runId: child!.smithersRunId,
      status: "finished",
      structuredWorkflowRunId: child!.id,
      continuedFromRunIds: [parent!.id],
    });

    const childLogPath = smithersLogPath(cwd, child!.smithersRunId);
    await waitFor("continue-as-new child execution log", () =>
      fileContains(childLogPath, '"type":"RunFinished"'),
    );
    expect(readFileSync(childLogPath, "utf8")).toContain('"type":"RunFinished"');

    expect(
      handlerAttentions.some((event) =>
        event.reason.includes("continued the workflow as a new run"),
      ),
    ).toBe(true);
    expect(
      handlerAttentions.some((event) =>
        event.reason.includes("finished and the handler must reconcile"),
      ),
    ).toBe(true);
    const parentSmithersRunId = parent?.smithersRunId;
    expect(parentSmithersRunId).toBeTruthy();
    expect(launched.runId).toBe(parentSmithersRunId!);
  });

  it("diagnoses signal waits and exposes real frame plus DevTools inspection for fixture runs", async () => {
    const { cwd, store, manager, sessionId, threadId, surfacePiSessionId } =
      createWorkspaceFixture();
    registerWorkflow(manager, createSignalWorkflowDefinition(smithersDbPath(cwd)));

    const launchCommand = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Launch signal workflow",
      title: "Run wait_for_signal",
      summary: "Launch the wait_for_signal workflow.",
      toolName: "smithers_run_workflow",
    });
    const launched = await manager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "wait_for_signal",
      launchInput: { signalName: "deploy.completed" },
      commandId: launchCommand.commandId,
    });

    await waitFor("signal wait state", async () => {
      try {
        const run = await manager.getRun(launched.runId);
        return run.status === "waiting-event" && run.waitKind === "event";
      } catch {
        return false;
      }
    });

    let snapshot = store.getSessionState(sessionId);
    expect(
      snapshot.workflowRuns.find((entry) => entry.smithersRunId === launched.runId),
    ).toMatchObject({
      status: "running",
      smithersStatus: "running",
      waitKind: null,
    });
    expect(snapshot.threads.find((thread) => thread.id === threadId)).toMatchObject({
      id: threadId,
      status: "waiting",
      wait: expect.objectContaining({
        owner: "workflow",
        kind: "signal",
      }),
    });
    expect(snapshot.session.wait).toMatchObject({
      owner: { kind: "thread", threadId },
      kind: "signal",
    });

    const watch = await manager.watchRun({
      runId: launched.runId,
      timeoutMs: 0,
    });
    expect(watch).toMatchObject({
      runId: launched.runId,
      reachedTerminal: false,
      timedOut: true,
      finalRun: {
        status: "waiting-event",
      },
    });

    const explanation = await manager.explainRun(launched.runId);
    expect(explanation.summary).toContain("waiting-event");
    expect(explanation.diagnosis).toMatchObject({
      runId: launched.runId,
      status: "waiting-event",
    });
    expect(
      explanation.diagnosis.blockers.some(
        (blocker: { signalName?: string | null }) => blocker.signalName === "deploy.completed",
      ),
    ).toBe(true);

    const delivered = await manager.sendSignal({
      runId: launched.runId,
      signalName: "deploy.completed",
      data: {
        environment: "production",
        sha: "abc123",
        status: "success",
      },
    });
    expect(delivered).toMatchObject({
      ok: true,
      runId: launched.runId,
      signalName: "deploy.completed",
    });

    const resumeCommand = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Resume signal workflow",
      title: "Resume wait_for_signal",
      summary: "Resume the wait_for_signal workflow.",
      toolName: "smithers_run_workflow",
    });
    await manager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "wait_for_signal",
      launchInput: { signalName: "deploy.completed" },
      commandId: resumeCommand.commandId,
      runId: launched.runId,
    });

    await waitFor("signal workflow completion", async () => {
      try {
        const run = await manager.getRun(launched.runId);
        return run.status === "finished";
      } catch {
        return false;
      }
    });

    const filteredEvents = await manager.getRunEvents({
      runId: launched.runId,
      types: ["RunFinished"],
    });
    expect(filteredEvents).toHaveLength(1);
    expect(filteredEvents[0]).toMatchObject({
      type: "RunFinished",
    });

    const frames = await manager.listFrames({
      runId: launched.runId,
      limit: 20,
    });
    expect(frames.length).toBeGreaterThan(0);
    expect(frames[0]).toMatchObject({
      runId: launched.runId,
      frameNo: expect.any(Number),
      xml: expect.anything(),
    });

    const devToolsSnapshot = await manager.getDevToolsSnapshot({
      runId: launched.runId,
    });
    expect(devToolsSnapshot).toMatchObject({
      version: 1,
      runId: launched.runId,
      frameNo: expect.any(Number),
      root: expect.any(Object),
    });

    const devToolsStream = await manager.streamDevTools({
      runId: launched.runId,
      afterSeq: 0,
      timeoutMs: 150,
      maxEvents: 10,
    });
    expect(devToolsStream.events.length).toBeGreaterThan(0);
    expect(devToolsStream.events[0]).toMatchObject({
      kind: "snapshot",
    });

    const signalLogPath = smithersLogPath(cwd, launched.runId);
    await waitFor("signal workflow execution log", () =>
      fileContains(signalLogPath, '"type":"RunFinished"'),
    );
    expect(readFileSync(signalLogPath, "utf8")).toContain('"type":"RunFinished"');

    snapshot = store.getSessionState(sessionId);
    expect(snapshot.threads.find((thread) => thread.id === threadId)).toMatchObject({
      id: threadId,
      status: "running-handler",
      wait: null,
    });
    expect(snapshot.session.wait).toBeNull();
  });

  it("replays workflow history across multiple Smithers event batches", async () => {
    const { store, manager, sessionId, threadId, surfacePiSessionId } = createWorkspaceFixture();

    const launchCommand = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Launch hello world",
      title: "Run hello_world",
      summary: "Launch the hello_world workflow.",
      toolName: "smithers_run_workflow",
    });
    const launched = await manager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "hello_world",
      launchInput: { message: "drain every event batch" },
      commandId: launchCommand.commandId,
    });

    await waitFor("hello_world completion", async () => {
      try {
        const run = await manager.getRun(launched.runId);
        return run.status === "finished";
      } catch {
        return false;
      }
    });

    const before = await manager.getRun(launched.runId);
    const db = (manager as any).db as {
      insertEventWithNextSeq(input: {
        runId: string;
        timestampMs: number;
        type: string;
        payloadJson: string;
      }): Promise<number>;
    };
    let lastInsertedSeq = before.lastEventSeq;
    for (let index = 0; index < 401; index += 1) {
      lastInsertedSeq = await db.insertEventWithNextSeq({
        runId: launched.runId,
        timestampMs: Date.now() + index,
        type: "SyntheticEvent",
        payloadJson: JSON.stringify({
          type: "SyntheticEvent",
          index,
        }),
      });
    }

    await (manager as any).flushRunEvents(launched.runId, {
      emitAttention: false,
      source: "bootstrap",
    });

    const after = await manager.getRun(launched.runId);
    expect(after.lastEventSeq).toBe(lastInsertedSeq);
    expect(after.lastEventSeq).toBeGreaterThan((before.lastEventSeq ?? -1) + 200);
  });

  it("returns grouped transcript messages for a deterministic real Smithers agent task", async () => {
    const { cwd, store, manager, sessionId, threadId, surfacePiSessionId } =
      createWorkspaceFixture();
    registerWorkflow(manager, createTranscriptWorkflowDefinition(smithersDbPath(cwd)));

    const launchCommand = createWorkflowCommand({
      store,
      sessionId,
      threadId,
      surfacePiSessionId,
      requestSummary: "Launch transcript workflow",
      title: "Run chat_transcript_probe",
      summary: "Launch the chat_transcript_probe workflow.",
      toolName: "smithers_run_workflow",
    });
    const launched = await manager.launchWorkflow({
      sessionId,
      threadId,
      workflowId: "chat_transcript_probe",
      launchInput: { prompt: "Summarize the transcript probe." },
      commandId: launchCommand.commandId,
    });

    await waitFor("transcript workflow completion", async () => {
      try {
        const run = await manager.getRun(launched.runId);
        return run.status === "finished";
      } catch {
        return false;
      }
    });

    const transcript = await manager.getChatTranscript({
      runId: launched.runId,
      all: true,
    });
    expect(transcript.attempts).toHaveLength(1);
    expect(transcript.messages.map((message) => message.role)).toEqual(
      expect.arrayContaining(["user", "assistant"]),
    );
    expect(
      transcript.messages.some((message) =>
        String(message.text).includes("Summarize the transcript probe."),
      ),
    ).toBe(true);
    expect(transcript.messages.some((message) => String(message.text).includes("Handled:"))).toBe(
      true,
    );

    const assistantDetail = await manager.getNodeDetail({
      runId: launched.runId,
      nodeId: "assistant",
    });
    expect(assistantDetail.attempts.length).toBeGreaterThan(0);
    expect(assistantDetail.node.outputTable).toBeTruthy();
  });

  it("runs the execute_typescript_task workflow through the real task-agent path with direct tools and code mode", async () => {
    const createAgentSessionSpy = spyOn(PiCodingAgent, "createAgentSession");

    const root = mkdtempSync(join(tmpdir(), "svvy-smithers-runtime-task-agent-"));
    tempDirs.push(root);
    const cwd = join(root, "workspace");
    const agentDir = join(root, "agent");
    mkdirSync(cwd, { recursive: true });
    mkdirSync(agentDir, { recursive: true });
    mkdirSync(join(cwd, ".smithers", "executions"), { recursive: true });
    const databasePath = join(root, "structured-session-state.sqlite");
    const store = createStructuredSessionStateStore({
      workspace: {
        id: cwd,
        label: "svvy",
        cwd,
      },
      databasePath,
    });
    stores.push(store);

    const sessionId = "session-task-agent";
    store.upsertPiSession({
      sessionId,
      title: "Task Agent Session",
      provider: "openai",
      model: "gpt-5.4",
      reasoningEffort: "medium",
      messageCount: 1,
      status: "running",
      createdAt: "2026-04-20T09:00:00.000Z",
      updatedAt: "2026-04-20T09:00:00.000Z",
    });
    const seedTurn = store.startTurn({
      sessionId,
      surfacePiSessionId: sessionId,
      requestSummary: "Open a workflow task agent thread",
    });
    const handlerThread = store.createThread({
      turnId: seedTurn.id,
      surfacePiSessionId: "pi-thread-task-agent",
      title: "Workflow task agent thread",
      objective: "Run a task-agent workflow.",
    });
    store.finishTurn({
      turnId: seedTurn.id,
      status: "completed",
    });

    const handlerAttentions: string[] = [];
    const manager = new SmithersRuntimeManager({
      cwd,
      agentDir,
      store,
      getTaskAgentDefaults: () => ({
        provider: "openai",
        model: "gpt-5.4",
        reasoningEffort: "medium",
        promptLibraryState: createDefaultPromptLibraryState(),
      }),
      onHandlerAttention: async (event) => {
        handlerAttentions.push(event.reason);
        return false;
      },
    });
    managers.push(manager);
    registerWorkflow(manager, createHelloWorldTestWorkflow(smithersDbPath(cwd)));
    registerWorkflow(
      manager,
      createExecuteTypescriptTaskTestWorkflow({
        dbPath: smithersDbPath(cwd),
        cwd,
        agentDir,
        artifactDir: taskAgentArtifactDir(cwd),
        store,
        provider: "openai",
        model: "gpt-5.4",
        reasoningEffort: "medium",
        promptLibraryState: createDefaultPromptLibraryState(),
      }),
    );

    createAgentSessionSpy.mockImplementation(async (options: any) => {
      const subscribers = new Set<(event: Record<string, unknown>) => void>();
      const messages: Array<{ role: string; content: Array<{ type: string; text: string }> }> = [];

      return {
        session: {
          agent: {
            state: {
              messages,
            },
            async prompt(
              promptMessages: Array<{
                role: string;
                content: Array<{ type: string; text: string }>;
              }>,
            ) {
              messages.push(...promptMessages);

              const executeTypescript = options.customTools.find(
                (tool: { name: string }) => tool.name === "execute_typescript",
              );
              if (!executeTypescript) {
                throw new Error("Expected execute_typescript in the custom task tool surface.");
              }

              subscribers.forEach((callback) =>
                callback({
                  type: "tool_execution_start",
                  toolCallId: "tool-call-workflow-task",
                  toolName: "execute_typescript",
                }),
              );

              const toolResult = await executeTypescript.execute(
                "tool-call-workflow-task",
                {
                  typescriptCode: [
                    'await api.bash({ command: "printf workflow-validated > workflow-task-output.txt" });',
                    "return {",
                    '  summary: "Completed the workflow task and wrote the output file.",',
                    '  filesChanged: ["workflow-task-output.txt"],',
                    '  validationRan: ["echo workflow-validated"],',
                    "  unresolvedIssues: [],",
                    "};",
                  ].join("\n"),
                },
                undefined,
                undefined,
              );

              subscribers.forEach((callback) =>
                callback({
                  type: "tool_execution_end",
                  toolCallId: "tool-call-workflow-task",
                  toolName: "execute_typescript",
                  isError: false,
                  result: toolResult,
                }),
              );

              const taskResult = (toolResult.details as any).result as {
                summary: string;
                filesChanged: string[];
                validationRan: string[];
                unresolvedIssues: string[];
              };
              messages.push({
                role: "assistant",
                content: [
                  {
                    type: "text",
                    text: JSON.stringify({
                      status: "completed",
                      summary: taskResult.summary,
                      filesChanged: taskResult.filesChanged,
                      validationRan: taskResult.validationRan,
                      unresolvedIssues: taskResult.unresolvedIssues,
                    }),
                  },
                ],
              });
              subscribers.forEach((callback) =>
                callback({
                  type: "turn_end",
                  message: messages[messages.length - 1],
                  toolResults: [],
                }),
              );
            },
          },
          getActiveToolNames() {
            return options.customTools.map((tool: { name: string }) => tool.name);
          },
          subscribe(callback: (event: Record<string, unknown>) => void) {
            subscribers.add(callback);
            return () => {
              subscribers.delete(callback);
            };
          },
          async abort() {},
          dispose() {},
        },
      } as any;
    });

    const launchCommand = createWorkflowCommand({
      store,
      sessionId,
      threadId: handlerThread.id,
      surfacePiSessionId: handlerThread.surfacePiSessionId,
      requestSummary: "Launch execute_typescript_task",
      title: "Run execute_typescript_task",
      summary: "Launch the execute_typescript_task workflow.",
      toolName: "smithers_run_workflow",
    });
    try {
      const launched = await manager.launchWorkflow({
        sessionId,
        threadId: handlerThread.id,
        workflowId: "execute_typescript_task",
        launchInput: {
          objective: "Write a file through execute_typescript and report the result.",
          successCriteria: ["Create workflow-task-output.txt with the validation output."],
          validationCommands: ["echo workflow-validated"],
        },
        commandId: launchCommand.commandId,
      });

      await waitFor("workflow task completion", async () => {
        try {
          const run = await manager.getRun(launched.runId);
          return run.status === "finished";
        } catch {
          return false;
        }
      });
      await waitFor("workflow task handler attention", () =>
        handlerAttentions.some((reason) =>
          reason.includes("finished and the handler must reconcile"),
        ),
      );

      const taskAgentLogPath = smithersLogPath(cwd, launched.runId);
      await waitFor("workflow task execution log", () =>
        fileContains(taskAgentLogPath, '"type":"RunFinished"'),
      );
      expect(readFileSync(taskAgentLogPath, "utf8")).toContain('"type":"RunFinished"');

      const outputPath = join(cwd, "workflow-task-output.txt");
      expect(existsSync(outputPath)).toBe(true);
      expect(readFileSync(outputPath, "utf8")).toBe("workflow-validated");

      const [createAgentSessionOptions] = createAgentSessionSpy.mock.calls[0] ?? [];
      expect(createAgentSessionOptions?.noTools).toBe("builtin");
      expect(
        createAgentSessionOptions?.customTools?.map((tool: { name: string }) => tool.name),
      ).toEqual([
        "list_tools",
        "cx_overview",
        "cx_symbols",
        "cx_definition",
        "cx_references",
        "cx_lang_list",
        "cx_lang_add",
        "cx_lang_remove",
        "cx_cache_path",
        "cx_cache_clean",
        "read",
        "grep",
        "find",
        "ls",
        "edit",
        "write",
        "bash",
        "artifact_write_text",
        "artifact_write_json",
        "artifact_attach_file",
        "execute_typescript",
      ]);

      const snapshot = store.getSessionState(sessionId);
      const workflowRun = snapshot.workflowRuns.find(
        (entry) => entry.smithersRunId === launched.runId,
      );
      expect(workflowRun).toMatchObject({
        workflowName: "execute_typescript_task",
        workflowSource: "saved",
        entryPath: ".svvy/workflows/entries/execute-typescript-task.tsx",
        savedEntryId: "execute_typescript_task",
        status: "running",
        smithersStatus: "running",
      });
      expect(snapshot.threads.find((thread) => thread.id === handlerThread.id)).toMatchObject({
        id: handlerThread.id,
        status: "running-handler",
        wait: null,
      });
      const workflowTaskAttempt = snapshot.workflowTaskAttempts.find(
        (entry) => entry.workflowRunId === workflowRun?.id && entry.nodeId === "task",
      );
      expect(workflowTaskAttempt).toBeTruthy();
      expect(workflowTaskAttempt).toMatchObject({
        nodeId: "task",
        iteration: 0,
        attempt: 1,
        kind: "agent",
        status: "completed",
        smithersState: "finished",
        agentEngine: "pi",
        workflowRunId: workflowRun?.id,
      });
      expect(
        snapshot.workflowTaskMessages.filter(
          (message) => message.workflowTaskAttemptId === workflowTaskAttempt?.id,
        ),
      ).not.toHaveLength(0);
      expect(
        snapshot.commands.some(
          (command) =>
            command.workflowTaskAttemptId === workflowTaskAttempt?.id &&
            command.toolName === "execute_typescript",
        ),
      ).toBe(true);

      const taskNodeDetail = await manager.getNodeDetail({
        runId: launched.runId,
        nodeId: "task",
      });
      expect(taskNodeDetail.node.nodeId).toBe("task");
      expect(taskNodeDetail.attempts.length).toBeGreaterThan(0);
      expect(taskNodeDetail.node.outputTable).toBeTruthy();
      const taskAttemptMeta =
        (taskNodeDetail.attempts[0] as { meta?: { agentResume?: string | null } } | undefined)
          ?.meta ?? null;
      const parsedTaskAttemptMeta = taskAttemptMeta;
      expect(parsedTaskAttemptMeta?.agentResume).toEqual(expect.any(String));
      expect(parsedTaskAttemptMeta?.agentResume).toContain("/task-agent-sessions/");
      const taskAttemptArtifacts = snapshot.artifacts.filter(
        (artifact) => artifact.workflowTaskAttemptId === workflowTaskAttempt?.id,
      );
      expect(taskAttemptArtifacts.length).toBeGreaterThan(0);
      for (const artifact of taskAttemptArtifacts) {
        if (artifact.path) {
          expect(existsSync(artifact.path)).toBe(true);
        }
      }
      expect(
        handlerAttentions.some((reason) =>
          reason.includes("finished and the handler must reconcile"),
        ),
      ).toBe(true);
    } finally {
      createAgentSessionSpy.mockRestore();
    }
  });
});
