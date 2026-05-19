import { Database } from "bun:sqlite";
import type { AgentMessage, AgentTool } from "@mariozechner/pi-agent-core";
import { getModel, getProviders } from "@mariozechner/pi-ai";
import {
  AuthStorage,
  createAgentSession,
  DefaultResourceLoader,
  ModelRegistry,
  SessionManager,
  SettingsManager,
  type ToolDefinition,
} from "@mariozechner/pi-coding-agent";
import type { AgentLike } from "smithers-orchestrator";
import { getToolContext } from "smithers-orchestrator/tools";
import { createHash } from "node:crypto";
import { existsSync, mkdirSync } from "node:fs";
import { join } from "node:path";
import type { PromptLibraryState } from "../../shared/prompt-library";
import { resolveApiKey } from "../auth-store";
import {
  EXECUTE_TYPESCRIPT_TOOL_NAME,
  executeTypescriptParamsSchema,
  runExecuteTypescript,
  type ExecuteTypescriptCommandStore,
  type ExecuteTypescriptResult,
} from "../execute-typescript-tool";
import { createCxTools } from "../cx-tools";
import type { StructuredSessionStateStore } from "../structured-session-state";
import { createSvvyDirectTools } from "../svvy-direct-tools";
import { createListToolsTool } from "../list-tools-tool";
import { createWebProvider } from "../web-runtime/provider-registry";
import { createSessionAgentSettingsStore } from "../session-agent-settings";
import { buildSystemPrompt, WORKFLOW_TASK_SYSTEM_PROMPT } from "../default-system-prompt";
import { createWorkflowLibrary } from "./workflow-library";
import {
  createDefaultWorkflowTaskAgentConfig,
  type WorkflowTaskAgentConfig,
} from "./workflow-task-agent-config";
import {
  WORKFLOW_TASK_TOOL_REGISTRY,
  type WorkflowTaskToolName,
} from "./workflow-authoring-contract";

type WorkflowTaskAgentOptions = {
  workspaceRoot: string;
  agentDir: string;
  artifactDir: string;
  store: StructuredSessionStateStore;
  config?: WorkflowTaskAgentConfig;
  promptLibraryState: PromptLibraryState;
};

type WorkflowTaskAgentGenerateArgs = {
  prompt?: string;
  messages?: AgentMessage[];
  resumeSession?: string;
  rootDir?: string;
  abortSignal?: AbortSignal;
  timeout?: number | { totalMs?: number; idleMs?: number };
  options?: unknown;
  onEvent?: (event: Record<string, unknown>) => void;
  onStepFinish?: (step: { response: { messages: AgentMessage[] } }) => void;
  onStdout?: (chunk: string) => void;
  onStderr?: (chunk: string) => void;
  outputSchema?: unknown;
};

type WorkflowTaskAttemptProjectionContext = {
  threadId: string;
  workflowRunId: string;
  workflowTaskAttemptId: string;
  surfacePiSessionId: string;
  smithersRunId: string;
  nodeId: string;
  iteration: number;
  attempt: number;
};

type SmithersTaskAttemptIdentity = {
  runId: string;
  nodeId: string;
  iteration: number;
  attempt: number;
};

type WorkflowTaskPromptBinding = {
  actor: "workflow-task";
  promptRevisionId: string;
  resolvedPromptHash: string;
  resolvedPromptTextArtifactId: string | null;
  boundExternalSourceHashes: string[];
  boundAt: string;
};

const taskAttemptIdentityByResumeHandle = new Map<string, SmithersTaskAttemptIdentity>();

export function createWorkflowTaskAgent(options: WorkflowTaskAgentOptions): AgentLike {
  return {
    id: "svvy-workflow-task-agent",
    async generate(rawArgs: unknown) {
      const args = normalizeWorkflowTaskAgentGenerateArgs(rawArgs);
      const taskRoot = resolveWorkflowTaskRoot(args);
      const smithersIdentity = readSmithersTaskAttemptIdentity({
        workspaceRoot: options.workspaceRoot,
        rootDir: taskRoot,
        resumeSession: args.resumeSession,
      });
      const config = options.config ?? createDefaultWorkflowTaskAgentConfig();
      const webProvider = createWebProvider(
        {
          provider: createSessionAgentSettingsStore({
            cwd: options.workspaceRoot,
            agentDir: options.agentDir,
          }).getState().appPreferences.webProvider,
        },
        {
          tinyfishApiKey: resolveApiKey("tinyfish"),
          firecrawlApiKey: resolveApiKey("firecrawl"),
        },
      );
      const baseSystemPrompt = buildSystemPrompt("workflow-task", {
        promptLibraryState: options.promptLibraryState,
        workspaceKey: options.workspaceRoot,
        webProvider,
      });
      const configSystemPrompt = config.systemPrompt.trim();
      const resolvedSystemPrompt =
        !configSystemPrompt || configSystemPrompt === WORKFLOW_TASK_SYSTEM_PROMPT
          ? baseSystemPrompt
          : `${baseSystemPrompt}\n\n## Workflow Task Agent Override\n${configSystemPrompt}`;
      const model = getModel(
        config.provider as Parameters<typeof getModel>[0],
        config.model as Parameters<typeof getModel>[1],
      );
      if (!model) {
        throw new Error(`Workflow task agent model not found: ${config.provider}/${config.model}`);
      }

      const sessionDir = resolveTaskAgentSessionDir(options.artifactDir);
      const shouldResumeFromSession =
        Boolean(args.resumeSession?.trim()) && (!args.messages || args.messages.length === 0);
      const sessionManager = await resolveTaskAgentSessionManager({
        cwd: taskRoot,
        sessionDir,
        resumeSession: shouldResumeFromSession ? args.resumeSession : undefined,
      });
      const agentDir = options.agentDir;
      mkdirSync(sessionDir, { recursive: true });

      const authStorage = AuthStorage.inMemory();
      syncAuthStorage(authStorage);
      const modelRegistryFactory = ModelRegistry as unknown as {
        create?: (authStorage: AuthStorage, modelPath: string) => ModelRegistry;
        new (authStorage: AuthStorage, modelPath: string): ModelRegistry;
      };
      const modelRegistryPath = join(agentDir, "models.json");
      const modelRegistry =
        typeof modelRegistryFactory.create === "function"
          ? modelRegistryFactory.create(authStorage, modelRegistryPath)
          : new modelRegistryFactory(authStorage, modelRegistryPath);
      const settingsManager = SettingsManager.create(taskRoot, agentDir);
      if (!shouldResumeFromSession) {
        sessionManager.appendSessionInfo("Workflow Task Agent");
      }
      const resourceLoader = new DefaultResourceLoader({
        cwd: taskRoot,
        agentDir,
        settingsManager,
        noExtensions: true,
        systemPromptOverride: () => resolvedSystemPrompt,
        appendSystemPromptOverride: () => [],
      });
      await resourceLoader.reload();
      const promptBinding = buildWorkflowTaskPromptBinding({
        promptLibraryState: options.promptLibraryState,
        resolvedSystemPrompt,
        resourceLoader,
      });

      const sessionIdentity = {
        surfacePiSessionId: sessionManager.getSessionId(),
      };
      const resumeHandleRef: { current: string | null } = {
        current: args.resumeSession ?? null,
      };
      const projectionContextRef: {
        current: WorkflowTaskAttemptProjectionContext | null;
      } = {
        current: null,
      };
      const executeTypescriptTool = createWorkflowTaskExecuteTypescriptTool({
        cwd: taskRoot,
        workspaceRoot: options.workspaceRoot,
        store: options.store,
        getSurfacePiSessionId: () => sessionIdentity.surfacePiSessionId,
        getResumeHandle: () => resumeHandleRef.current,
        getSmithersIdentity: () => smithersIdentity,
        getPromptBinding: () => promptBinding,
        setProjectionContext: (projection) => {
          projectionContextRef.current = projection;
        },
        webProvider,
      });
      const directTools = createSvvyDirectTools({
        cwd: taskRoot,
        runtime: { current: null },
        store: options.store,
        workflowLibrary: createWorkflowLibrary(taskRoot),
        webProvider,
      });
      const cxTools = createCxTools({ cwd: taskRoot });
      let sessionForListTools: {
        getActiveToolNames(): string[];
        getAllTools(): { name: string; description?: string; parameters?: unknown }[];
      } | null = null;

      const { session } = await createAgentSession({
        cwd: taskRoot,
        agentDir,
        authStorage,
        modelRegistry,
        sessionManager,
        settingsManager,
        model,
        thinkingLevel: config.reasoningEffort,
        noTools: "builtin",
        customTools: createCustomToolDefinitions(
          selectWorkflowTaskAgentTools({
            configuredToolSurface: config.toolSurface,
            tools: [
              createListToolsTool({
                getSession: () => sessionForListTools,
              }),
              ...cxTools,
              ...directTools.codingTools,
              ...directTools.artifactTools,
              ...directTools.webTools,
              executeTypescriptTool,
            ],
          }),
        ),
        resourceLoader,
      });
      sessionForListTools = session;
      assertTaskAgentToolSurface(
        session.getActiveToolNames(),
        selectWorkflowTaskAgentToolNames(config.toolSurface),
      );

      const durableSessionManager =
        (session as { sessionManager?: SessionManager }).sessionManager ?? sessionManager;
      sessionIdentity.surfacePiSessionId = durableSessionManager.getSessionId();
      const resumeHandle =
        durableSessionManager.getSessionFile() ?? sessionIdentity.surfacePiSessionId;
      resumeHandleRef.current = resumeHandle;
      bindWorkflowTaskAttemptResumeHandle({
        resumeHandle,
        identity: smithersIdentity,
      });

      args.onEvent?.({
        type: "started",
        engine: "pi",
        title: "pi workflow task agent",
        resume: resumeHandle,
        detail: {
          surfacePiSessionId: sessionIdentity.surfacePiSessionId,
          smithers: smithersIdentity ?? undefined,
        },
      });

      const promptInput = buildWorkflowTaskPromptInput(args);
      const inputMessageCount = promptInput.messages.length;
      const prePromptMessageCount = session.agent.state.messages.length;
      let responseCursor = prePromptMessageCount + inputMessageCount;
      let streamedAssistantText = "";
      let timeoutId: ReturnType<typeof setTimeout> | undefined;
      const timeoutMs = resolveTimeoutMs(args.timeout);
      let timeoutTriggered = false;

      const unsubscribe = session.subscribe((event) => {
        if (event.type === "message_update") {
          const assistantMessageEvent =
            event.assistantMessageEvent &&
            typeof event.assistantMessageEvent === "object" &&
            !Array.isArray(event.assistantMessageEvent)
              ? (event.assistantMessageEvent as { type?: string; delta?: string })
              : undefined;
          if (
            assistantMessageEvent?.type === "text_delta" &&
            typeof assistantMessageEvent.delta === "string"
          ) {
            streamedAssistantText += assistantMessageEvent.delta;
            args.onStdout?.(assistantMessageEvent.delta);
            args.onEvent?.({
              type: "action",
              engine: "pi",
              phase: "updated",
              action: {
                id: "workflow-task-assistant",
                kind: "note",
                title: "assistant",
              },
              message: assistantMessageEvent.delta,
            });
          }
          return;
        }

        if (event.type === "turn_end") {
          const deltaMessages = cloneAgentMessages(
            session.agent.state.messages.slice(responseCursor),
          );
          if (deltaMessages.length > 0) {
            responseCursor = session.agent.state.messages.length;
            args.onStepFinish?.({
              response: {
                messages: deltaMessages,
              },
            });
          }
          return;
        }

        if (event.type === "tool_execution_start") {
          args.onEvent?.({
            type: "action",
            engine: "pi",
            phase: "started",
            action: {
              id: event.toolCallId,
              kind: "tool",
              title: event.toolName,
            },
          });
          return;
        }

        if (event.type === "tool_execution_update") {
          args.onEvent?.({
            type: "action",
            engine: "pi",
            phase: "updated",
            action: {
              id: event.toolCallId,
              kind: "tool",
              title: event.toolName,
            },
            message: summarizeValue(event.partialResult),
          });
          return;
        }

        if (event.type === "tool_execution_end") {
          args.onEvent?.({
            type: "action",
            engine: "pi",
            phase: "completed",
            ok: !event.isError,
            action: {
              id: event.toolCallId,
              kind: "tool",
              title: event.toolName,
            },
            message: summarizeValue(event.result),
          });
        }
      });

      const abortPrompt = () => {
        void session.abort();
      };
      args.abortSignal?.addEventListener("abort", abortPrompt, { once: true });
      if (timeoutMs) {
        timeoutId = setTimeout(() => {
          timeoutTriggered = true;
          void session.abort();
        }, timeoutMs);
      }

      try {
        await session.agent.prompt(promptInput.messages);
        if (timeoutTriggered) {
          const timeoutError = new Error(`Workflow task agent timed out after ${timeoutMs}ms.`);
          timeoutError.name = "AbortError";
          throw timeoutError;
        }

        const trailingResponseMessages = cloneAgentMessages(
          session.agent.state.messages.slice(responseCursor),
        );
        if (trailingResponseMessages.length > 0) {
          responseCursor = session.agent.state.messages.length;
          args.onStepFinish?.({
            response: {
              messages: trailingResponseMessages,
            },
          });
        }

        const responseMessages = cloneAgentMessages(
          session.agent.state.messages.slice(prePromptMessageCount + inputMessageCount),
        );
        const text =
          getLatestAssistantText(responseMessages).trim() ||
          streamedAssistantText.trim() ||
          getLatestAssistantText(session.agent.state.messages).trim();
        if (text && streamedAssistantText.trim().length === 0) {
          args.onStdout?.(text);
        }
        const usage = normalizeLatestAssistantUsage(responseMessages, session.agent.state.messages);
        recordWorkflowTaskAgentContextBudget({
          store: options.store,
          projectionContext: projectionContextRef.current,
          smithersIdentity,
          usage,
          maxTokens: model.contextWindow,
        });

        args.onEvent?.({
          type: "completed",
          engine: "pi",
          ok: true,
          answer: text,
          usage,
          resume: resumeHandle,
        });

        return {
          text,
          output: tryParseJson(text),
          usage,
          totalUsage: usage,
          response: {
            messages: responseMessages,
          },
        };
      } catch (error) {
        const message = timeoutTriggered
          ? `Workflow task agent timed out after ${timeoutMs}ms.`
          : error instanceof Error
            ? error.message
            : "Workflow task agent prompt failed.";
        args.onStderr?.(message);
        args.onEvent?.({
          type: "completed",
          engine: "pi",
          ok: false,
          error: message,
          resume: resumeHandle,
        });
        if (timeoutTriggered) {
          const timeoutError = new Error(message);
          timeoutError.name = "AbortError";
          throw timeoutError;
        }
        throw error;
      } finally {
        if (timeoutId) {
          clearTimeout(timeoutId);
        }
        unsubscribe();
        args.abortSignal?.removeEventListener("abort", abortPrompt);
        session.dispose();
      }
    },
  };
}

function recordWorkflowTaskAgentContextBudget(input: {
  store: StructuredSessionStateStore;
  projectionContext: WorkflowTaskAttemptProjectionContext | null;
  smithersIdentity: SmithersTaskAttemptIdentity | null;
  usage: Record<string, unknown> | undefined;
  maxTokens: number | undefined;
}): void {
  const usedTokens =
    input.usage && typeof input.usage.inputTokens === "number" ? input.usage.inputTokens : null;
  if (!usedTokens || !input.maxTokens) {
    return;
  }

  const existing = input.projectionContext
    ? input.store.findWorkflowTaskAttemptBySmithersIdentity({
        smithersRunId: input.projectionContext.smithersRunId,
        nodeId: input.projectionContext.nodeId,
        iteration: input.projectionContext.iteration,
        attempt: input.projectionContext.attempt,
      })
    : input.smithersIdentity
      ? input.store.findWorkflowTaskAttemptBySmithersIdentity({
          smithersRunId: input.smithersIdentity.runId,
          nodeId: input.smithersIdentity.nodeId,
          iteration: input.smithersIdentity.iteration,
          attempt: input.smithersIdentity.attempt,
        })
      : null;
  if (!existing) {
    return;
  }

  input.store.upsertWorkflowTaskAttempt({
    workflowRunId: existing.workflowRunId,
    smithersRunId: existing.smithersRunId,
    nodeId: existing.nodeId,
    iteration: existing.iteration,
    attempt: existing.attempt,
    surfacePiSessionId: existing.surfacePiSessionId,
    title: existing.title,
    summary: existing.summary,
    kind: existing.kind,
    status: existing.status,
    smithersState: existing.smithersState,
    prompt: existing.prompt,
    responseText: existing.responseText,
    error: existing.error,
    cached: existing.cached,
    jjPointer: existing.jjPointer,
    jjCwd: existing.jjCwd,
    heartbeatAt: existing.heartbeatAt,
    agentId: existing.agentId,
    agentModel: existing.agentModel,
    agentEngine: existing.agentEngine,
    agentResume: existing.agentResume,
    meta: {
      ...existing.meta,
      contextBudget: {
        usedTokens,
        maxTokens: input.maxTokens,
      },
    },
    startedAt: existing.startedAt,
    finishedAt: existing.finishedAt,
  });
}

function createWorkflowTaskExecuteTypescriptTool(input: {
  cwd: string;
  workspaceRoot: string;
  store: StructuredSessionStateStore;
  getSurfacePiSessionId: () => string;
  getResumeHandle: () => string | null;
  getSmithersIdentity: () => SmithersTaskAttemptIdentity | null;
  getPromptBinding: () => WorkflowTaskPromptBinding;
  setProjectionContext: (projection: WorkflowTaskAttemptProjectionContext) => void;
  webProvider: ReturnType<typeof createWebProvider>;
}): AgentTool<typeof executeTypescriptParamsSchema, ExecuteTypescriptResult> {
  return {
    label: "Code Mode",
    name: EXECUTE_TYPESCRIPT_TOOL_NAME,
    description:
      "Run bounded TypeScript against selected duplicated direct tools inside the workflow task agent.",
    parameters: executeTypescriptParamsSchema,
    execute: async (_toolCallId, params, signal) => {
      const projection = await waitForWorkflowTaskAttemptProjection({
        store: input.store,
        workspaceRoot: input.workspaceRoot,
        surfacePiSessionId: input.getSurfacePiSessionId(),
        agentResume: input.getResumeHandle(),
        smithersIdentity: input.getSmithersIdentity(),
        promptBinding: input.getPromptBinding(),
      });
      input.setProjectionContext(projection);
      const result = await runExecuteTypescript({
        cwd: input.cwd,
        store: createStructuredWorkflowTaskExecuteTypescriptStore({
          store: input.store,
          projectionContext: projection,
        }),
        signal,
        typescriptCode: params.typescriptCode,
        context: {
          actor: "workflow-task",
          surfacePiSessionId: projection.surfacePiSessionId,
          turnId: null,
          workflowTaskAttemptId: projection.workflowTaskAttemptId,
          threadId: projection.threadId,
          workflowRunId: projection.workflowRunId,
          executor: "workflow-task-agent",
        },
        webProvider: input.webProvider,
      });

      return {
        content: [
          {
            type: "text",
            text: JSON.stringify(result),
          },
        ],
        details: result,
      };
    },
  };
}

function createStructuredWorkflowTaskExecuteTypescriptStore(input: {
  store: StructuredSessionStateStore;
  projectionContext: WorkflowTaskAttemptProjectionContext;
}): ExecuteTypescriptCommandStore {
  return {
    createCommand(config) {
      const command = input.store.createCommand({
        turnId: config.turnId ?? null,
        workflowTaskAttemptId:
          config.workflowTaskAttemptId ?? input.projectionContext.workflowTaskAttemptId,
        surfacePiSessionId: config.surfacePiSessionId ?? input.projectionContext.surfacePiSessionId,
        threadId: config.threadId ?? input.projectionContext.threadId,
        workflowRunId: config.workflowRunId ?? input.projectionContext.workflowRunId,
        parentCommandId: config.parentCommandId,
        toolName: config.toolName,
        executor: config.executor,
        visibility: config.visibility,
        title: config.title,
        summary: config.summary,
        facts: config.facts,
        attempts: config.attempts,
      });
      return { id: command.id };
    },
    startCommand(commandId) {
      input.store.startCommand(commandId);
    },
    finishCommand(config) {
      input.store.finishCommand(config);
    },
    createArtifact(config) {
      const artifact = input.store.createArtifact({
        threadId: config.threadId ?? input.projectionContext.threadId,
        workflowRunId: config.workflowRunId ?? input.projectionContext.workflowRunId,
        workflowTaskAttemptId:
          config.workflowTaskAttemptId ?? input.projectionContext.workflowTaskAttemptId,
        sourceCommandId: config.sourceCommandId,
        kind: config.kind,
        name: config.name,
        path: config.path,
        content: config.content,
      });
      return { id: artifact.id, path: artifact.path };
    },
  };
}

async function waitForWorkflowTaskAttemptProjection(input: {
  store: StructuredSessionStateStore;
  workspaceRoot: string;
  surfacePiSessionId: string;
  agentResume: string | null;
  smithersIdentity: SmithersTaskAttemptIdentity | null;
  promptBinding: WorkflowTaskPromptBinding;
  timeoutMs?: number;
}): Promise<WorkflowTaskAttemptProjectionContext> {
  if (!input.smithersIdentity) {
    throw new Error(
      "Workflow task agent projection requires exact Smithers task attempt identity.",
    );
  }
  const agentResume = input.agentResume?.trim() || null;

  const deadline = Date.now() + (input.timeoutMs ?? 5_000);
  while (Date.now() <= deadline) {
    const smithersAttempt = findSmithersAttemptByIdentity({
      workspaceRoot: input.workspaceRoot,
      identity: input.smithersIdentity,
    });
    const workflowRun = smithersAttempt
      ? input.store.findWorkflowRunBySmithersRunId(smithersAttempt.runId)
      : null;
    if (workflowRun && smithersAttempt) {
      const smithersMeta =
        parseTaskAttemptMeta(smithersAttempt.metaJson) ??
        parseTaskAttemptMeta(smithersAttempt.heartbeatDataJson) ??
        {};
      const attempt = input.store.upsertWorkflowTaskAttempt({
        workflowRunId: workflowRun.id,
        smithersRunId: smithersAttempt.runId,
        nodeId: smithersAttempt.nodeId,
        iteration: smithersAttempt.iteration,
        attempt: smithersAttempt.attempt,
        surfacePiSessionId: input.surfacePiSessionId,
        title:
          readTaskAttemptMetaString(smithersAttempt.metaJson, "label") ?? smithersAttempt.nodeId,
        summary: summarizeWorkflowTaskAttempt(smithersAttempt),
        kind: "agent",
        status: mapSmithersAttemptStateToStructuredStatus(smithersAttempt.state),
        smithersState: smithersAttempt.state,
        prompt: readTaskAttemptMetaString(smithersAttempt.metaJson, "prompt"),
        responseText: smithersAttempt.responseText ?? null,
        error: readTaskAttemptErrorMessage(smithersAttempt.errorJson),
        cached: Boolean(smithersAttempt.cached),
        jjPointer: smithersAttempt.jjPointer ?? null,
        jjCwd: smithersAttempt.jjCwd ?? null,
        heartbeatAt:
          typeof smithersAttempt.heartbeatAtMs === "number"
            ? new Date(smithersAttempt.heartbeatAtMs).toISOString()
            : null,
        agentId: readTaskAttemptMetaString(smithersAttempt.metaJson, "agentId"),
        agentModel: readTaskAttemptMetaString(smithersAttempt.metaJson, "agentModel"),
        agentEngine:
          readTaskAttemptMetaString(smithersAttempt.metaJson, "agentEngine") ??
          readTaskAttemptMetaString(smithersAttempt.heartbeatDataJson, "agentEngine"),
        agentResume,
        meta: {
          ...smithersMeta,
          promptBinding: input.promptBinding,
        },
        startedAt: new Date(smithersAttempt.startedAtMs).toISOString(),
        finishedAt:
          typeof smithersAttempt.finishedAtMs === "number"
            ? new Date(smithersAttempt.finishedAtMs).toISOString()
            : null,
      });
      return {
        threadId: attempt.threadId,
        workflowRunId: attempt.workflowRunId,
        workflowTaskAttemptId: attempt.id,
        surfacePiSessionId: attempt.surfacePiSessionId ?? input.surfacePiSessionId,
        smithersRunId: attempt.smithersRunId,
        nodeId: attempt.nodeId,
        iteration: attempt.iteration,
        attempt: attempt.attempt,
      };
    }
    await Bun.sleep(25);
  }

  throw new Error(
    `Timed out waiting for workflow task attempt projection ${input.smithersIdentity.runId}:${input.smithersIdentity.nodeId}:${input.smithersIdentity.iteration}:${input.smithersIdentity.attempt}.`,
  );
}

type SmithersAttemptProjectionRow = {
  runId: string;
  nodeId: string;
  iteration: number;
  attempt: number;
  state: string;
  startedAtMs: number;
  finishedAtMs?: number | null;
  heartbeatAtMs?: number | null;
  heartbeatDataJson?: string | null;
  errorJson?: string | null;
  metaJson?: string | null;
  responseText?: string | null;
  cached?: number | boolean | null;
  jjPointer?: string | null;
  jjCwd?: string | null;
};

function findSmithersAttemptByIdentity(input: {
  workspaceRoot: string;
  identity: SmithersTaskAttemptIdentity;
}): SmithersAttemptProjectionRow | null {
  const dbPath = join(input.workspaceRoot, ".svvy", "smithers-runtime", "smithers.db");
  if (!existsSync(dbPath)) {
    return null;
  }

  const db = new Database(dbPath, { readonly: true });
  try {
    return (
      (db
        .query(
          `SELECT
             run_id AS runId,
             node_id AS nodeId,
             iteration AS iteration,
             attempt AS attempt,
             state AS state,
             started_at_ms AS startedAtMs,
             finished_at_ms AS finishedAtMs,
             heartbeat_at_ms AS heartbeatAtMs,
             heartbeat_data_json AS heartbeatDataJson,
             error_json AS errorJson,
             meta_json AS metaJson,
             response_text AS responseText,
             cached AS cached,
             jj_pointer AS jjPointer,
             jj_cwd AS jjCwd
           FROM _smithers_attempts
           WHERE run_id = ?
             AND node_id = ?
             AND iteration = ?
             AND attempt = ?
           LIMIT 1`,
        )
        .get(
          input.identity.runId,
          input.identity.nodeId,
          input.identity.iteration,
          input.identity.attempt,
        ) as SmithersAttemptProjectionRow | undefined) ?? null
    );
  } finally {
    db.close();
  }
}

function parseTaskAttemptMeta(metaJson: string | null | undefined): Record<string, unknown> | null {
  if (!metaJson) {
    return null;
  }
  try {
    const parsed = JSON.parse(metaJson);
    return parsed && typeof parsed === "object" && !Array.isArray(parsed)
      ? (parsed as Record<string, unknown>)
      : null;
  } catch {
    return null;
  }
}

function buildWorkflowTaskPromptBinding(input: {
  promptLibraryState: PromptLibraryState;
  resolvedSystemPrompt: string;
  resourceLoader: {
    getAgentsFiles(): { agentsFiles: Array<{ path: string; content: string }> };
  };
}): WorkflowTaskPromptBinding {
  return {
    actor: "workflow-task",
    promptRevisionId: String(input.promptLibraryState.revision),
    resolvedPromptHash: hashContent(input.resolvedSystemPrompt),
    resolvedPromptTextArtifactId: null,
    boundExternalSourceHashes: input.resourceLoader
      .getAgentsFiles()
      .agentsFiles.map(
        (source) => `${source.path.replaceAll("\\", "/")}:${hashContent(source.content)}`,
      ),
    boundAt: new Date().toISOString(),
  };
}

function hashContent(content: string): string {
  return createHash("sha256").update(content).digest("hex");
}

function readTaskAttemptMetaString(
  metaJson: string | null | undefined,
  key: string,
): string | null {
  const meta = parseTaskAttemptMeta(metaJson);
  return meta && typeof meta[key] === "string" ? (meta[key] as string) : null;
}

function readTaskAttemptErrorMessage(errorJson: string | null | undefined): string | null {
  if (!errorJson) {
    return null;
  }
  try {
    const parsed = JSON.parse(errorJson);
    return parsed && typeof parsed === "object" && typeof parsed.message === "string"
      ? (parsed.message as string)
      : null;
  } catch {
    return null;
  }
}

function mapSmithersAttemptStateToStructuredStatus(
  state: string,
): "running" | "waiting" | "completed" | "failed" | "cancelled" {
  switch (state) {
    case "waiting-timer":
      return "waiting";
    case "finished":
      return "completed";
    case "failed":
      return "failed";
    case "cancelled":
      return "cancelled";
    default:
      return "running";
  }
}

function summarizeWorkflowTaskAttempt(input: SmithersAttemptProjectionRow): string {
  switch (input.state) {
    case "finished":
      return `Workflow task attempt ${input.nodeId} completed.`;
    case "failed":
      return `Workflow task attempt ${input.nodeId} failed.`;
    case "cancelled":
      return `Workflow task attempt ${input.nodeId} was cancelled.`;
    case "waiting-timer":
      return `Workflow task attempt ${input.nodeId} is waiting on a timer.`;
    default:
      return `Workflow task attempt ${input.nodeId} is running.`;
  }
}

function resolveTaskAgentSessionDir(artifactDir: string): string {
  return join(artifactDir, "..", "..", "task-agent-sessions");
}

async function resolveTaskAgentSessionManager(input: {
  cwd: string;
  sessionDir: string;
  resumeSession?: string;
}): Promise<SessionManager> {
  const resumeHandle = input.resumeSession?.trim();
  if (!resumeHandle) {
    return SessionManager.create(input.cwd, input.sessionDir);
  }

  if (looksLikeSessionPath(resumeHandle)) {
    return SessionManager.open(resumeHandle, input.sessionDir);
  }

  const sessions = await SessionManager.list(input.cwd, input.sessionDir);
  const match = sessions.find((session) => session.id === resumeHandle);
  if (!match) {
    throw new Error(`Workflow task agent resume session id was not found exactly: ${resumeHandle}`);
  }
  return SessionManager.open(match.path, input.sessionDir);
}

function createCustomToolDefinitions(tools: readonly AgentTool<any>[]): ToolDefinition[] {
  return tools.map((tool) => ({
    name: tool.name,
    label: tool.label,
    description: tool.description,
    parameters: tool.parameters,
    prepareArguments: tool.prepareArguments,
    execute: async (toolCallId, params, signal, onUpdate) =>
      await tool.execute(toolCallId, params, signal, onUpdate),
  }));
}

function syncAuthStorage(authStorage: AuthStorage): void {
  for (const provider of getProviders()) {
    const apiKey = resolveApiKey(provider);
    if (apiKey) {
      authStorage.setRuntimeApiKey(provider, apiKey);
    } else {
      authStorage.removeRuntimeApiKey(provider);
    }
  }
}

function selectWorkflowTaskAgentTools(input: {
  configuredToolSurface: readonly WorkflowTaskToolName[];
  tools: readonly AgentTool<any>[];
}): AgentTool<any>[] {
  const toolsByName = new Map(input.tools.map((tool) => [tool.name, tool]));
  return selectWorkflowTaskAgentToolNames(input.configuredToolSurface).map((toolName) => {
    const tool = toolsByName.get(toolName);
    if (!tool) {
      throw new Error(`Workflow task agent tool is unavailable: ${toolName}.`);
    }
    return tool;
  });
}

function selectWorkflowTaskAgentToolNames(
  configuredToolSurface: readonly WorkflowTaskToolName[],
): string[] {
  const requested = new Set(configuredToolSurface);
  const unknown = configuredToolSurface.filter(
    (toolName) => !WORKFLOW_TASK_TOOL_REGISTRY.includes(toolName),
  );
  if (unknown.length > 0) {
    throw new Error(`Workflow task agent config contains unknown tools: ${unknown.join(", ")}.`);
  }
  return [
    "list_tools",
    ...WORKFLOW_TASK_TOOL_REGISTRY.filter((toolName) => requested.has(toolName)),
  ];
}

function assertTaskAgentToolSurface(
  activeToolNames: string[],
  expectedToolNames: readonly string[],
): void {
  const unexpected = activeToolNames.filter((name) => !expectedToolNames.includes(name));
  const missing = expectedToolNames.filter((name) => !activeToolNames.includes(name));
  if (unexpected.length > 0 || missing.length > 0) {
    throw new Error(
      [
        "Workflow task agent received the wrong tool surface.",
        unexpected.length > 0 ? `Unexpected: ${unexpected.join(", ")}.` : "",
        missing.length > 0 ? `Missing: ${missing.join(", ")}.` : "",
      ]
        .filter(Boolean)
        .join(" "),
    );
  }
}

function buildWorkflowTaskPromptInput(input: WorkflowTaskAgentGenerateArgs): {
  messages: AgentMessage[];
} {
  if (input.messages && input.messages.length > 0) {
    return {
      messages: normalizeAgentMessages(input.messages),
    };
  }

  return {
    messages: [createUserAgentMessage(buildWorkflowTaskPromptText(input))],
  };
}

function buildWorkflowTaskPromptText(input: WorkflowTaskAgentGenerateArgs): string {
  const parts: string[] = [];
  const prompt = typeof input.prompt === "string" ? input.prompt.trim() : "";
  if (prompt) {
    parts.push(prompt);
  }
  if (input.outputSchema) {
    parts.push("Return only the requested final JSON object. Do not wrap it in markdown fences.");
  }
  return parts.join("\n\n").trim();
}

function createUserAgentMessage(text: string): AgentMessage {
  return {
    role: "user",
    content: text,
    timestamp: Date.now(),
  } as AgentMessage;
}

function normalizeAgentMessages(messages: AgentMessage[]): AgentMessage[] {
  const timestampBase = Date.now();
  return messages.map((message, index) => normalizeAgentMessage(message, timestampBase + index));
}

function normalizeAgentMessage(message: AgentMessage, timestamp: number): AgentMessage {
  if (message.role === "user") {
    return {
      ...message,
      content: normalizeUserContent(message.content),
      timestamp: typeof message.timestamp === "number" ? message.timestamp : timestamp,
    } as AgentMessage;
  }

  if (message.role === "assistant") {
    if (
      Array.isArray(message.content) &&
      typeof message.timestamp === "number" &&
      typeof (message as { model?: unknown }).model === "string" &&
      isRecord((message as { usage?: unknown }).usage)
    ) {
      return message;
    }

    return {
      role: "assistant",
      content: normalizeAssistantContent(message.content),
      api: (message as { api?: unknown }).api as never,
      provider: ((message as { provider?: unknown }).provider ?? "") as never,
      model:
        typeof (message as { model?: unknown }).model === "string"
          ? (message as { model: string }).model
          : "pi",
      usage: normalizePiUsage((message as { usage?: unknown }).usage) as never,
      stopReason:
        typeof (message as { stopReason?: unknown }).stopReason === "string"
          ? ((message as { stopReason: string }).stopReason as never)
          : ("stop" as never),
      timestamp:
        typeof (message as { timestamp?: unknown }).timestamp === "number"
          ? (message as { timestamp: number }).timestamp
          : timestamp,
      errorMessage:
        typeof (message as { errorMessage?: unknown }).errorMessage === "string"
          ? (message as { errorMessage: string }).errorMessage
          : undefined,
      responseId:
        typeof (message as { responseId?: unknown }).responseId === "string"
          ? (message as { responseId: string }).responseId
          : undefined,
    } as AgentMessage;
  }

  if (message.role === "toolResult") {
    return {
      role: "toolResult",
      toolCallId:
        typeof (message as { toolCallId?: unknown }).toolCallId === "string"
          ? (message as { toolCallId: string }).toolCallId
          : "tool-result",
      toolName:
        typeof (message as { toolName?: unknown }).toolName === "string"
          ? (message as { toolName: string }).toolName
          : EXECUTE_TYPESCRIPT_TOOL_NAME,
      content: normalizeToolResultContent((message as { content?: unknown }).content),
      details: (message as { details?: unknown }).details,
      isError: Boolean((message as { isError?: unknown }).isError),
      timestamp:
        typeof (message as { timestamp?: unknown }).timestamp === "number"
          ? (message as { timestamp: number }).timestamp
          : timestamp,
    } as AgentMessage;
  }

  return message;
}

function normalizeUserContent(content: unknown): string | Array<{ type: "text"; text: string }> {
  if (typeof content === "string") {
    return content;
  }

  const parts = normalizeTextParts(content)
    .map((part) =>
      part.type === "text" && typeof part.text === "string"
        ? { type: "text" as const, text: part.text }
        : null,
    )
    .filter((part): part is { type: "text"; text: string } => part !== null);
  return parts.length > 0 ? parts : "";
}

function normalizeAssistantContent(content: unknown): Array<{ type: string; text?: string }> {
  const parts = normalizeTextParts(content);
  return parts.length > 0 ? parts : [{ type: "text", text: "" }];
}

function normalizeToolResultContent(content: unknown): Array<{ type: "text"; text: string }> {
  const parts = normalizeTextParts(content)
    .map((part) =>
      part.type === "text" && typeof part.text === "string"
        ? { type: "text" as const, text: part.text }
        : null,
    )
    .filter((part): part is { type: "text"; text: string } => part !== null);
  return parts.length > 0 ? parts : [{ type: "text", text: "" }];
}

function normalizeTextParts(content: unknown): Array<{ type: string; text?: string }> {
  if (typeof content === "string") {
    return [{ type: "text", text: content }];
  }
  if (!Array.isArray(content)) {
    return [];
  }

  return content
    .map((part) => {
      if (typeof part === "string") {
        return { type: "text", text: part };
      }
      if (!part || typeof part !== "object") {
        return null;
      }
      if (typeof (part as { type?: unknown }).type === "string") {
        return part as { type: string; text?: string };
      }
      if (typeof (part as { text?: unknown }).text === "string") {
        return {
          type: "text",
          text: (part as { text: string }).text,
        };
      }
      return null;
    })
    .filter((part): part is { type: string; text?: string } => part !== null);
}

function messageToText(message: { content?: unknown }): string {
  const content = message.content;
  if (typeof content === "string") {
    return content;
  }
  if (!Array.isArray(content)) {
    return "";
  }

  return content
    .map((part) => {
      if (typeof part === "string") {
        return part;
      }
      if (
        part &&
        typeof part === "object" &&
        typeof (part as { text?: unknown }).text === "string"
      ) {
        return (part as { text: string }).text;
      }
      return "";
    })
    .filter(Boolean)
    .join("\n");
}

function getLatestAssistantText(messages: AgentMessage[]): string {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index];
    if (!message || message.role !== "assistant") {
      continue;
    }
    return messageToText(message);
  }

  return "";
}

function normalizeLatestAssistantUsage(
  responseMessages: AgentMessage[],
  allMessages: AgentMessage[],
): Record<string, unknown> | undefined {
  const latestAssistant = [...responseMessages, ...allMessages]
    .toReversed()
    .find((message) => message.role === "assistant");
  if (!latestAssistant || latestAssistant.role !== "assistant") {
    return undefined;
  }

  const usage = normalizePiUsage(latestAssistant.usage);
  return usage.totalTokens === undefined ? undefined : usage;
}

function normalizePiUsage(value: unknown): {
  inputTokens?: number;
  inputTokenDetails: {
    noCacheTokens?: number;
    cacheReadTokens?: number;
    cacheWriteTokens?: number;
  };
  outputTokens?: number;
  outputTokenDetails: {
    textTokens?: number;
    reasoningTokens?: number;
  };
  totalTokens?: number;
} {
  const usage = isRecord(value) ? value : {};
  const input = typeof usage.input === "number" ? usage.input : undefined;
  const output = typeof usage.output === "number" ? usage.output : undefined;
  const cacheRead = typeof usage.cacheRead === "number" ? usage.cacheRead : undefined;
  const cacheWrite = typeof usage.cacheWrite === "number" ? usage.cacheWrite : undefined;
  const total =
    typeof usage.totalTokens === "number"
      ? usage.totalTokens
      : input !== undefined || output !== undefined
        ? (input ?? 0) + (output ?? 0)
        : undefined;

  return {
    inputTokens: input,
    inputTokenDetails: {
      noCacheTokens:
        input !== undefined ? Math.max(0, input - (cacheRead ?? 0) - (cacheWrite ?? 0)) : undefined,
      cacheReadTokens: cacheRead,
      cacheWriteTokens: cacheWrite,
    },
    outputTokens: output,
    outputTokenDetails: {
      textTokens: undefined,
      reasoningTokens: undefined,
    },
    totalTokens: total,
  };
}

function cloneAgentMessages(messages: AgentMessage[]): AgentMessage[] {
  return structuredClone(messages);
}

function summarizeValue(value: unknown, maxLength = 400): string | undefined {
  if (value == null) {
    return undefined;
  }

  const text =
    typeof value === "string"
      ? value
      : (() => {
          try {
            return JSON.stringify(value);
          } catch {
            return String(value);
          }
        })();
  return text.length <= maxLength ? text : `${text.slice(0, maxLength - 1)}…`;
}

function resolveTimeoutMs(timeout: WorkflowTaskAgentGenerateArgs["timeout"]): number | null {
  if (typeof timeout === "number" && Number.isFinite(timeout) && timeout > 0) {
    return timeout;
  }
  if (
    timeout &&
    typeof timeout === "object" &&
    typeof timeout.totalMs === "number" &&
    Number.isFinite(timeout.totalMs) &&
    timeout.totalMs > 0
  ) {
    return timeout.totalMs;
  }
  return null;
}

function resolveWorkflowTaskRoot(input: WorkflowTaskAgentGenerateArgs): string {
  if (typeof input.rootDir === "string" && input.rootDir.trim().length > 0) {
    return input.rootDir;
  }
  const toolContext = getToolContext();
  if (toolContext?.rootDir) {
    return toolContext.rootDir;
  }
  throw new Error("Workflow task agent requires an explicit Smithers task root.");
}

function readSmithersTaskAttemptIdentity(input: {
  workspaceRoot: string;
  rootDir: string;
  resumeSession?: string;
}): SmithersTaskAttemptIdentity | null {
  const contextIdentity = readSmithersTaskAttemptIdentityFromContext();
  if (contextIdentity) {
    return contextIdentity;
  }
  const resumeHandle = input.resumeSession?.trim();
  if (resumeHandle) {
    const boundIdentity = taskAttemptIdentityByResumeHandle.get(resumeHandle);
    if (boundIdentity) {
      return boundIdentity;
    }
  }
  return findUniqueInProgressSmithersTaskAttempt({
    workspaceRoot: input.workspaceRoot,
    rootDir: input.rootDir,
  });
}

function readSmithersTaskAttemptIdentityFromContext(): SmithersTaskAttemptIdentity | null {
  const toolContext = getToolContext();
  if (!toolContext) {
    return null;
  }
  return {
    runId: toolContext.runId,
    nodeId: toolContext.nodeId,
    iteration: toolContext.iteration,
    attempt: toolContext.attempt,
  };
}

function bindWorkflowTaskAttemptResumeHandle(input: {
  resumeHandle: string | null;
  identity: SmithersTaskAttemptIdentity | null;
}): void {
  const resumeHandle = input.resumeHandle?.trim();
  if (!resumeHandle || !input.identity) {
    return;
  }
  const existing = taskAttemptIdentityByResumeHandle.get(resumeHandle);
  if (existing && !smithersTaskAttemptIdentityEquals(existing, input.identity)) {
    throw new Error(
      `Workflow task agent resume handle already belongs to ${existing.runId}:${existing.nodeId}:${existing.iteration}:${existing.attempt}; refusing to rebind it to ${input.identity.runId}:${input.identity.nodeId}:${input.identity.iteration}:${input.identity.attempt}.`,
    );
  }
  taskAttemptIdentityByResumeHandle.set(resumeHandle, input.identity);
}

function findUniqueInProgressSmithersTaskAttempt(input: {
  workspaceRoot: string;
  rootDir: string;
}): SmithersTaskAttemptIdentity | null {
  const dbPath = join(input.workspaceRoot, ".svvy", "smithers-runtime", "smithers.db");
  if (!existsSync(dbPath)) {
    return null;
  }

  const db = new Database(dbPath, { readonly: true });
  try {
    const rows = db
      .query(
        `SELECT
           run_id AS runId,
           node_id AS nodeId,
           iteration AS iteration,
           attempt AS attempt
         FROM _smithers_attempts
         WHERE state = 'in-progress'
           AND jj_cwd = ?
           AND json_extract(meta_json, '$.agentId') = ?
           AND json_extract(meta_json, '$.kind') = 'agent'`,
      )
      .all(input.rootDir, "svvy-workflow-task-agent") as SmithersTaskAttemptIdentity[];
    if (rows.length > 1) {
      const identities = rows
        .map((row) => `${row.runId}:${row.nodeId}:${row.iteration}:${row.attempt}`)
        .join(", ");
      throw new Error(
        `Workflow task agent Smithers context is ambiguous for root ${input.rootDir}: ${identities}.`,
      );
    }
    return rows[0] ?? null;
  } finally {
    db.close();
  }
}

function smithersTaskAttemptIdentityEquals(
  left: SmithersTaskAttemptIdentity,
  right: SmithersTaskAttemptIdentity,
): boolean {
  return (
    left.runId === right.runId &&
    left.nodeId === right.nodeId &&
    left.iteration === right.iteration &&
    left.attempt === right.attempt
  );
}

function looksLikeSessionPath(value: string): boolean {
  return value.includes("/") || value.includes("\\") || value.endsWith(".jsonl");
}

function tryParseJson(text: string): unknown | undefined {
  const trimmed = text.trim();
  if (!trimmed.startsWith("{") && !trimmed.startsWith("[")) {
    return undefined;
  }

  try {
    return JSON.parse(trimmed);
  } catch {
    return undefined;
  }
}

function normalizeWorkflowTaskAgentGenerateArgs(value: unknown): WorkflowTaskAgentGenerateArgs {
  if (!isRecord(value)) {
    return {};
  }

  return {
    prompt: typeof value.prompt === "string" ? value.prompt : undefined,
    messages: Array.isArray(value.messages) ? (value.messages as AgentMessage[]) : undefined,
    resumeSession: typeof value.resumeSession === "string" ? value.resumeSession : undefined,
    rootDir: typeof value.rootDir === "string" ? value.rootDir : undefined,
    abortSignal: isAbortSignal(value.abortSignal) ? value.abortSignal : undefined,
    timeout:
      typeof value.timeout === "number" || (value.timeout && typeof value.timeout === "object")
        ? (value.timeout as WorkflowTaskAgentGenerateArgs["timeout"])
        : undefined,
    options: value.options,
    onEvent:
      typeof value.onEvent === "function"
        ? (value.onEvent as WorkflowTaskAgentGenerateArgs["onEvent"])
        : undefined,
    onStepFinish:
      typeof value.onStepFinish === "function"
        ? (value.onStepFinish as WorkflowTaskAgentGenerateArgs["onStepFinish"])
        : undefined,
    onStdout:
      typeof value.onStdout === "function"
        ? (value.onStdout as WorkflowTaskAgentGenerateArgs["onStdout"])
        : undefined,
    onStderr:
      typeof value.onStderr === "function"
        ? (value.onStderr as WorkflowTaskAgentGenerateArgs["onStderr"])
        : undefined,
    outputSchema: value.outputSchema,
  };
}

function isAbortSignal(value: unknown): value is AbortSignal {
  return typeof AbortSignal !== "undefined" && value instanceof AbortSignal;
}

function isRecord(value: unknown): value is Record<string, any> {
  return value !== null && typeof value === "object";
}
