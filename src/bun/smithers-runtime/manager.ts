import { SmithersDb } from "@smithers-orchestrator/db";
import { Database } from "bun:sqlite";
import {
  chatAttemptKey,
  parseNodeOutputEvent,
  type ParsedNodeOutputEvent,
} from "@smithers-orchestrator/cli/chat";
import { diagnoseRunEffect } from "@smithers-orchestrator/cli/why-diagnosis";
import {
  ensureSmithersTables,
  runWorkflow,
  signalRun,
  type RunStatus,
  type SmithersEvent,
  type SmithersWorkflow,
} from "smithers-orchestrator";
import type { ThinkingLevel } from "@mariozechner/pi-agent-core";
import { Effect } from "effect";
import { mkdirSync } from "node:fs";
import { join } from "node:path";
import { randomUUID } from "node:crypto";
import { z } from "zod";
import type {
  StructuredSessionStateStore,
  StructuredWorkflowWaitKind,
  StructuredWorkflowRunRecord,
  StructuredWorkflowStatus,
  StructuredProjectCiCheckStatus,
  StructuredProjectCiStatus,
} from "../structured-session-state";
import type { PromptLibraryState } from "../../shared/prompt-library";
import {
  compileRunnableWorkflowLaunchContract,
  type RunnableWorkflowLaunchContract,
} from "./workflow-launch-contract";
import {
  SmithersNativeAdapter,
  type NativeChatTranscript,
  type NativeNodeDetail,
  type NativeResolveApprovalInput,
  type NativeRunEvent,
} from "./native-adapter";
import { loadRunnableWorkflowRegistry } from "./workflow-registry";
import type { RunnableWorkflowRegistryEntry } from "./workflow-authoring-contract";

type WorkflowTaskAgentDefaults = {
  provider: string;
  model: string;
  reasoningEffort: ThinkingLevel;
  promptLibraryState?: PromptLibraryState;
};

type WorkflowMonitor = {
  id: string;
  sessionId: string;
  threadId: string;
  workflowId: string;
  abortController: AbortController;
  trackedRunIds: Set<string>;
};

type WorkflowOwnership = {
  sessionId: string;
  threadId: string;
  workflowId: string;
  structuredWorkflowId: string;
  commandId: string;
};

type WorkflowExecutionProjection = {
  status: StructuredWorkflowStatus;
  smithersStatus: RunStatus;
  waitKind: StructuredWorkflowWaitKind | null;
  heartbeatAt: string | null;
  finishedAt: string | null;
  summary: string;
  projectCiProjection: ProjectCiProjection | null;
};

type ProjectionSource = "launch" | "progress" | "control" | "bootstrap" | "failure" | "query";

type SmithersRuntimeManagerOptions = {
  cwd: string;
  agentDir: string;
  store: StructuredSessionStateStore;
  getTaskAgentDefaults: () => WorkflowTaskAgentDefaults;
  onStructuredStateChanged?: (sessionId: string) => void | Promise<void>;
  onHandlerAttention?: (input: {
    sessionId: string;
    threadId: string;
    workflowRunId: string;
    smithersRunId: string;
    workflowId: string;
    summary: string;
    reason: string;
  }) => boolean | Promise<boolean>;
};

type LaunchWorkflowInput = {
  sessionId: string;
  threadId: string;
  workflowId: string;
  launchInput: unknown;
  commandId: string;
  runId?: string;
};

type LaunchWorkflowResult = {
  workflowId: string;
  label: string;
  sourceScope: RunnableWorkflowRegistryEntry["sourceScope"];
  entryPath: string;
  definitionPaths: string[];
  promptPaths: string[];
  componentPaths: string[];
  assetPaths: string[];
  launchInput: Record<string, unknown>;
  runId: string;
  resumedRunId: string | null;
  structuredWorkflowRunId: string;
  status: StructuredWorkflowStatus;
  smithersStatus: RunStatus;
  summary: string;
};

type WorkflowListInput = {
  workflowId?: string;
  sourceScope?: RunnableWorkflowRegistryEntry["sourceScope"];
  productKind?: RunnableWorkflowRegistryEntry["productKind"];
  pathPrefix?: string;
};

type WorkflowListEntry = {
  workflowId: string;
  label: string;
  summary: string;
  sourceScope: RunnableWorkflowRegistryEntry["sourceScope"];
  entryPath: string;
  productKind?: RunnableWorkflowRegistryEntry["productKind"];
  definitionPaths: string[];
  promptPaths: string[];
  componentPaths: string[];
  assetPaths: string[];
  launchInputSchema: Record<string, unknown>;
  resultSchema?: Record<string, unknown>;
};

export type WorkflowLaunchDiagnostic = {
  path?: string;
  message: string;
  code?: string;
};

export type WorkflowLaunchPreflightResult =
  | {
      success: true;
      workflow: WorkflowListEntry;
      launchInput: Record<string, unknown>;
    }
  | {
      success: false;
      workflowId: string;
      diagnostics: WorkflowLaunchDiagnostic[];
      workflow?: WorkflowListEntry;
    };

type SmithersAttemptRow = {
  runId: string;
  nodeId: string;
  iteration: number;
  attempt: number;
  state: string;
  startedAtMs: number;
  finishedAtMs?: number | null;
  heartbeatAtMs?: number | null;
  errorJson?: string | null;
  metaJson?: string | null;
  responseText?: string | null;
  cached?: boolean | null;
  jjPointer?: string | null;
  jjCwd?: string | null;
};

// Fixture-only workflow definition used by unit/e2e harnesses. Normal product startup discovers
// runnable entries from workspace `.svvy/` assets instead of registering these definitions.
export type TestWorkflowDefinition = {
  id: string;
  label: string;
  summary: string;
  workflowName?: string;
  launchSchema: RunnableWorkflowRegistryEntry["launchSchema"];
  productKind?: RunnableWorkflowRegistryEntry["productKind"];
  resultSchema?: RunnableWorkflowRegistryEntry["resultSchema"];
  workflow: SmithersWorkflow<any>;
  sourceScope?: RunnableWorkflowRegistryEntry["sourceScope"];
  entryPath?: string;
  definitionPaths?: string[];
  promptPaths?: string[];
  componentPaths?: string[];
};

const projectCiCheckStatusSchema = z.enum(["passed", "failed", "cancelled", "skipped", "blocked"]);

const projectCiResultProjectionSchema = z.object({
  status: z.enum(["passed", "failed", "cancelled", "blocked"]),
  summary: z.string().min(1),
  startedAt: z.string().optional(),
  finishedAt: z.string().optional(),
  checks: z.array(
    z.object({
      checkId: z.string().min(1),
      label: z.string().min(1),
      kind: z.string().min(1),
      status: projectCiCheckStatusSchema,
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

type ProjectCiResultProjection = z.infer<typeof projectCiResultProjectionSchema>;

type ProjectCiProjection =
  | {
      workflowStatus: "completed";
      summary: string;
      result: ProjectCiResultProjection | null;
    }
  | {
      workflowStatus: "failed";
      summary: string;
      result: null;
    };

export class SmithersRuntimeManager {
  private readonly runtimeRoot: string;
  private readonly runtimeDbPath: string;
  private readonly db: SmithersDb;
  private readonly nativeAdapter: SmithersNativeAdapter;
  private readonly testWorkflowEntriesById = new Map<string, RunnableWorkflowRegistryEntry>();
  private readonly workflowEntriesById = new Map<string, RunnableWorkflowRegistryEntry>();
  private readonly launchContractsByWorkflowId = new Map<string, RunnableWorkflowLaunchContract>();
  private workflowLaunchContracts: RunnableWorkflowLaunchContract[] = [];
  private readonly ownershipByRunId = new Map<string, WorkflowOwnership>();
  private readonly monitorByRunId = new Map<string, WorkflowMonitor>();
  private readonly flushPromiseByRunId = new Map<string, Promise<void>>();
  private readonly deliveringAttentionRunIds = new Set<string>();
  private readonly activeWorkflowPromises = new Set<Promise<void>>();
  private readonly activeWorkflowPromiseByRunId = new Map<string, Promise<void>>();

  constructor(private readonly options: SmithersRuntimeManagerOptions) {
    this.runtimeRoot = join(options.cwd, ".svvy", "smithers-runtime");
    mkdirSync(this.runtimeRoot, { recursive: true });
    this.runtimeDbPath = join(this.runtimeRoot, "smithers.db");
    const db = new Database(this.runtimeDbPath);
    ensureSmithersTables(db as any);
    this.db = new SmithersDb(db as any);
    this.nativeAdapter = new SmithersNativeAdapter(this.db, options.cwd);
  }

  listWorkflows(input: WorkflowListInput = {}): WorkflowListEntry[] {
    return this.workflowLaunchContracts
      .filter((contract) => (input.workflowId ? contract.workflowId === input.workflowId : true))
      .filter((contract) => (input.sourceScope ? contract.sourceScope === input.sourceScope : true))
      .filter((contract) => (input.productKind ? contract.productKind === input.productKind : true))
      .filter((contract) =>
        input.pathPrefix
          ? [
              contract.entryPath,
              ...contract.definitionPaths,
              ...contract.promptPaths,
              ...contract.componentPaths,
              ...contract.assetPaths,
            ].some((path) => path.startsWith(input.pathPrefix!))
          : true,
      )
      .map((contract) => ({
        workflowId: contract.workflowId,
        label: contract.label,
        summary: contract.summary,
        sourceScope: contract.sourceScope,
        entryPath: contract.entryPath,
        productKind: contract.productKind,
        definitionPaths: contract.definitionPaths.slice(),
        promptPaths: contract.promptPaths.slice(),
        componentPaths: contract.componentPaths.slice(),
        assetPaths: contract.assetPaths.slice(),
        launchInputSchema: structuredClone(contract.launchInputJsonSchema),
        resultSchema: contract.resultSchemaJsonSchema
          ? structuredClone(contract.resultSchemaJsonSchema)
          : undefined,
      }));
  }

  async refreshWorkflowRegistry(): Promise<void> {
    const registry = await loadRunnableWorkflowRegistry(this.options.cwd);
    this.rebuildWorkflowRegistry([
      ...registry,
      ...Array.from(this.testWorkflowEntriesById.values()),
    ]);
  }

  // Fixture hook for tests that need deterministic in-process workflows without writing `.svvy`
  // entry files. Product code should use saved/artifact entry discovery.
  registerTestWorkflow(definition: TestWorkflowDefinition): void {
    const sourceScope = definition.sourceScope ?? "saved";
    const entryPath =
      definition.entryPath ?? `.svvy/workflows/entries/${definition.id.replace(/_/g, "-")}.tsx`;
    const entry: RunnableWorkflowRegistryEntry = {
      workflowId: definition.id,
      label: definition.label,
      summary: definition.summary,
      sourceScope,
      entryPath,
      productKind: definition.productKind,
      launchSchema: definition.launchSchema,
      resultSchema: definition.resultSchema,
      definitionPaths: definition.definitionPaths ?? [],
      promptPaths: definition.promptPaths ?? [],
      componentPaths: definition.componentPaths ?? [],
      assetPaths: Array.from(
        new Set([
          ...(definition.definitionPaths ?? []),
          ...(definition.promptPaths ?? []),
          ...(definition.componentPaths ?? []),
        ]),
      ).toSorted(),
      createRunnableEntry: () => ({
        workflowId: definition.id,
        workflowSource: sourceScope,
        productKind: definition.productKind,
        launchSchema: definition.launchSchema,
        resultSchema: definition.resultSchema,
        workflow: definition.workflow,
      }),
    };
    this.testWorkflowEntriesById.set(definition.id, entry);
    this.rebuildWorkflowRegistry([
      ...Array.from(this.workflowEntriesById.values()),
      ...Array.from(this.testWorkflowEntriesById.values()),
    ]);
  }

  private rebuildWorkflowRegistry(registry: RunnableWorkflowRegistryEntry[]): void {
    this.workflowEntriesById.clear();
    this.launchContractsByWorkflowId.clear();
    for (const entry of registry) {
      this.workflowEntriesById.set(entry.workflowId, entry);
      this.launchContractsByWorkflowId.set(
        entry.workflowId,
        compileRunnableWorkflowLaunchContract(entry),
      );
    }
    this.workflowLaunchContracts = registry.map((entry) => {
      const contract = this.launchContractsByWorkflowId.get(entry.workflowId);
      if (!contract) {
        throw new Error(`Runnable Smithers workflow contract not found: ${entry.workflowId}`);
      }
      return contract;
    });
  }

  async validateWorkflowLaunchInput(input: {
    workflowId: string;
    launchInput: unknown;
  }): Promise<WorkflowLaunchPreflightResult> {
    await this.refreshWorkflowRegistry();

    const workflowId = input.workflowId.trim();
    const workflow = this.listWorkflows({ workflowId })[0];
    if (!workflow) {
      return {
        success: false,
        workflowId,
        diagnostics: [
          {
            message: `Runnable workflow not found: ${workflowId}`,
            code: "workflow_not_found",
          },
        ],
      };
    }

    const entry = this.requireWorkflowEntry(workflowId);
    const parsedInput = entry.launchSchema.safeParse(input.launchInput);
    if (!parsedInput.success) {
      return {
        success: false,
        workflowId,
        workflow,
        diagnostics: parsedInput.error.issues.map((issue) => ({
          path: issue.path.length > 0 ? issue.path.map(String).join(".") : undefined,
          message: issue.message,
          code: issue.code,
        })),
      };
    }

    return {
      success: true,
      workflow,
      launchInput: parsedInput.data as Record<string, unknown>,
    };
  }

  async launchWorkflow(input: LaunchWorkflowInput): Promise<LaunchWorkflowResult> {
    await this.refreshWorkflowRegistry();
    const entry = this.requireWorkflowEntry(input.workflowId);
    const parsedInput = entry.launchSchema.safeParse(input.launchInput);
    if (!parsedInput.success) {
      throw new Error(parsedInput.error.issues.map((issue) => issue.message).join("; "));
    }

    const requestedRunId = input.runId?.trim() || null;
    if (requestedRunId) {
      await this.requireExplicitResumeOwnership({
        sessionId: input.sessionId,
        threadId: input.threadId,
        workflowId: input.workflowId,
        runId: requestedRunId,
      });
    } else {
      const conflictingRun = await this.findNonterminalThreadRun(
        input.sessionId,
        input.threadId,
        input.workflowId,
      );
      if (conflictingRun) {
        throw new Error(
          `Handler thread ${input.threadId} already owns nonterminal Smithers run ${conflictingRun.smithersRunId} for workflow ${input.workflowId}. Pass runId to resume that exact run, cancel it before launching a fresh run, or wait for it to finish before replacing this workflow.`,
        );
      }
    }

    const runId = requestedRunId ?? `smithers-${randomUUID()}`;
    const existingStructuredRun = this.findStructuredWorkflowRun({
      sessionId: input.sessionId,
      threadId: input.threadId,
      runId,
    });

    const structuredWorkflowRun = existingStructuredRun
      ? this.options.store.updateWorkflow({
          workflowId: existingStructuredRun.id,
          commandId: input.commandId,
          pendingAttentionSeq: null,
        })
      : this.options.store.recordWorkflow({
          threadId: input.threadId,
          commandId: input.commandId,
          smithersRunId: runId,
          workflowName: entry.workflowId,
          workflowSource: entry.sourceScope,
          entryPath: entry.entryPath,
          savedEntryId: entry.sourceScope === "saved" ? entry.workflowId : null,
          status: "running",
          continuedFromRunIds: [],
          activeDescendantRunId: null,
          lastEventSeq: null,
          pendingAttentionSeq: null,
          lastAttentionSeq: null,
          heartbeatAt: null,
          summary: `${requestedRunId ? "Resuming" : "Launching"} ${entry.label}.`,
        });

    this.options.store.updateThread({
      threadId: input.threadId,
      status: "running-workflow",
      wait: null,
    });
    this.clearThreadOwnedSessionWait(input.sessionId, input.threadId);
    this.recordBridgeLifecycleEvent({
      sessionId: input.sessionId,
      workflowRunId: structuredWorkflowRun.id,
      kind: "workflowRun.bridge.projected",
      data: {
        source: "launch",
        smithersRunId: runId,
      },
    });
    this.ownershipByRunId.set(runId, {
      sessionId: input.sessionId,
      threadId: input.threadId,
      workflowId: entry.workflowId,
      structuredWorkflowId: structuredWorkflowRun.id,
      commandId: input.commandId,
    });

    const existingActiveRun = this.activeWorkflowPromiseByRunId.has(runId)
      ? await this.db.getRun(runId)
      : null;
    if (existingActiveRun?.status === "running") {
      await this.emitStructuredStateChanged(input.sessionId);
      return {
        workflowId: entry.workflowId,
        label: entry.label,
        sourceScope: entry.sourceScope,
        entryPath: entry.entryPath,
        definitionPaths: entry.definitionPaths.slice(),
        promptPaths: entry.promptPaths.slice(),
        componentPaths: entry.componentPaths.slice(),
        assetPaths: Array.from(
          new Set([...entry.definitionPaths, ...entry.promptPaths, ...entry.componentPaths]),
        ).toSorted(),
        launchInput: parsedInput.data as Record<string, unknown>,
        runId,
        resumedRunId: requestedRunId,
        structuredWorkflowRunId: structuredWorkflowRun.id,
        status: mapRunStatusToWorkflowStatus(existingActiveRun.status),
        smithersStatus: existingActiveRun.status,
        summary: await this.buildRunSummary(existingActiveRun),
      };
    }

    const monitor = this.createMonitor({
      sessionId: input.sessionId,
      threadId: input.threadId,
      workflowId: entry.workflowId,
      runId,
    });
    this.trackRunIdWithMonitor(monitor, runId);

    const runnableEntry = entry.createRunnableEntry({
      dbPath: this.runtimeDbPath,
    });
    const workflowPromise = this.runWorkflowInBackground({
      runnableEntry,
      monitor,
      runId,
      input: parsedInput.data as Record<string, unknown>,
      resume: Boolean(requestedRunId),
    });
    this.trackActiveWorkflowPromise(runId, workflowPromise);

    await this.waitForRunRegistration(runId);

    await this.emitStructuredStateChanged(input.sessionId);

    return {
      workflowId: entry.workflowId,
      label: entry.label,
      sourceScope: entry.sourceScope,
      entryPath: entry.entryPath,
      definitionPaths: entry.definitionPaths.slice(),
      promptPaths: entry.promptPaths.slice(),
      componentPaths: entry.componentPaths.slice(),
      assetPaths: Array.from(
        new Set([...entry.definitionPaths, ...entry.promptPaths, ...entry.componentPaths]),
      ).toSorted(),
      launchInput: parsedInput.data as Record<string, unknown>,
      runId,
      resumedRunId: requestedRunId,
      structuredWorkflowRunId: structuredWorkflowRun.id,
      status: "running",
      smithersStatus: "running",
      summary: structuredWorkflowRun.summary,
    };
  }

  private async waitForRunRegistration(runId: string, timeoutMs = 5_000): Promise<void> {
    const deadline = Date.now() + timeoutMs;
    while (Date.now() < deadline) {
      const run = await this.db.getRun(runId);
      if (run) {
        return;
      }
      await Bun.sleep(25);
    }
    throw new Error(`Smithers run ${runId} did not become observable after launch.`);
  }

  async listRuns(input?: { limit?: number; status?: string; workflowId?: string }) {
    const { runs } = await this.nativeAdapter.listRuns({
      limit: input?.limit ?? 25,
      status: input?.status,
    });
    const structuredRunBySmithersRunId = this.listStructuredRunBySmithersRunId();
    const filteredRuns = runs.filter((run: any) => {
      if (!input?.workflowId) {
        return true;
      }
      const structuredRun = structuredRunBySmithersRunId.get(run.runId);
      return (
        structuredRun?.savedEntryId === input.workflowId ||
        structuredRun?.workflowName === input.workflowId
      );
    });

    return await Promise.all(
      filteredRuns.map(async (run: any) => {
        const structuredRun = structuredRunBySmithersRunId.get(run.runId) ?? null;
        return {
          runId: run.runId,
          workflowName: structuredRun?.workflowName ?? run.workflowName,
          workflowId: structuredRun?.savedEntryId ?? structuredRun?.workflowName ?? null,
          workflowSource: structuredRun?.workflowSource ?? null,
          entryPath: structuredRun?.entryPath ?? null,
          savedEntryId: structuredRun?.savedEntryId ?? null,
          status: run.status,
          sessionId: structuredRun?.sessionId ?? null,
          threadId: structuredRun?.threadId ?? null,
          createdAt: toIso(run.createdAtMs),
          startedAt: toIso(run.startedAtMs),
          finishedAt: toIso(run.finishedAtMs),
          heartbeatAt: toIso(run.heartbeatAtMs),
          summary: await this.buildRunSummaryFromNative(run),
          native: run,
        };
      }),
    );
  }

  async getSmithersRunDetail(runId: string) {
    await this.flushRunEvents(runId, {
      emitAttention: false,
      source: "query",
    });
    return await this.nativeAdapter.getRun({ runId });
  }

  async getRun(runId: string) {
    await this.flushRunEvents(runId, {
      emitAttention: false,
      source: "query",
    });
    const run = await this.db.getRun(runId);
    if (!run) {
      throw new Error(`Smithers run not found: ${runId}`);
    }
    const workflowRun = this.findStructuredWorkflowRunBySmithersRunId(runId);
    const ownership = workflowRun ? this.ownershipFromWorkflowRun(workflowRun) : null;
    const executionProjection =
      workflowRun && ownership
        ? await this.buildWorkflowExecutionProjection({ run, workflowRun, ownership })
        : null;
    return {
      runId: run.runId,
      workflowName: workflowRun?.workflowName ?? run.workflowName,
      workflowId: workflowRun?.savedEntryId ?? workflowRun?.workflowName ?? null,
      workflowSource: workflowRun?.workflowSource ?? null,
      entryPath: workflowRun?.entryPath ?? null,
      savedEntryId: workflowRun?.savedEntryId ?? null,
      status: run.status,
      createdAt: toIso(run.createdAtMs),
      startedAt: toIso(run.startedAtMs),
      finishedAt: toIso(run.finishedAtMs),
      heartbeatAt: toIso(run.heartbeatAtMs),
      summary: executionProjection?.summary ?? (await this.buildRunSummary(run)),
      structuredWorkflowRunId: workflowRun?.id ?? null,
      threadId: workflowRun?.threadId ?? null,
      continuedFromRunIds: workflowRun?.continuedFromRunIds ?? [],
      activeDescendantRunId: workflowRun?.activeDescendantRunId ?? null,
      waitKind: executionProjection?.waitKind ?? mapRunStatusToWaitKind(run.status),
      lastEventSeq: workflowRun?.lastEventSeq ?? null,
      pendingAttentionSeq: workflowRun?.pendingAttentionSeq ?? null,
      lastAttentionSeq: workflowRun?.lastAttentionSeq ?? null,
    };
  }

  async explainRun(runId: string) {
    const { diagnosis } = await this.nativeAdapter.explainRun({ runId });
    return {
      runId,
      status: typeof diagnosis.status === "string" ? diagnosis.status : "unknown",
      summary:
        typeof diagnosis.summary === "string"
          ? diagnosis.summary
          : `Loaded Smithers diagnosis for run ${runId}.`,
      explanation: typeof diagnosis.summary === "string" ? diagnosis.summary : null,
      diagnosis,
    };
  }

  async watchRun(input: { runId: string; intervalMs?: number; timeoutMs?: number }) {
    return await this.nativeAdapter.watchRun(input);
  }

  async listPendingApprovals(input?: { runId?: string; workflowName?: string; nodeId?: string }) {
    const { approvals } = await this.nativeAdapter.listPendingApprovals(input);
    return approvals;
  }

  async resolveApproval(input: NativeResolveApprovalInput) {
    const result = await this.nativeAdapter.resolveApproval({
      ...input,
      decidedBy: input.decidedBy ?? "svvy-handler",
    });
    const runId =
      typeof result.approval.runId === "string"
        ? result.approval.runId
        : typeof input.runId === "string"
          ? input.runId
          : null;
    if (runId) {
      await this.flushRunEvents(runId, { source: "control" });
    }
    return result;
  }

  async getNodeDetail(input: {
    runId: string;
    nodeId: string;
    iteration?: number;
  }): Promise<NativeNodeDetail> {
    const { detail } = await this.nativeAdapter.getNodeDetail(input);
    return detail;
  }

  async listArtifacts(input: { runId: string; nodeId?: string; includeRaw?: boolean }) {
    return await this.nativeAdapter.listArtifacts(input);
  }

  async getChatTranscript(input: {
    runId: string;
    all?: boolean;
    includeStderr?: boolean;
    tail?: number;
  }): Promise<NativeChatTranscript> {
    return await this.nativeAdapter.getChatTranscript(input);
  }

  async getRunEvents(input: {
    runId: string;
    afterSeq?: number;
    limit?: number;
    nodeId?: string;
    types?: string[];
    sinceTimestampMs?: number;
  }): Promise<NativeRunEvent[]> {
    const { events } = await this.nativeAdapter.getRunEvents(input);
    return events;
  }

  async sendSignal(input: {
    runId: string;
    signalName: string;
    data?: unknown;
    correlationId?: string;
  }) {
    const delivered = await Effect.runPromise(
      signalRun(this.db, input.runId, input.signalName, input.data ?? {}, {
        correlationId: input.correlationId ?? null,
        receivedBy: "svvy-handler",
      }),
    );
    await this.flushRunEvents(input.runId, { source: "control" });
    const run = await this.getRun(input.runId).catch(() => null);
    return {
      ok: true,
      runId: input.runId,
      signalName: delivered.signalName,
      seq: delivered.seq,
      correlationId: delivered.correlationId,
      receivedAtMs: delivered.receivedAtMs,
      receivedAt: toIso(delivered.receivedAtMs),
      run,
    };
  }

  async listFrames(input: { runId: string; limit?: number; afterFrameNo?: number }) {
    const run = await this.db.getRun(input.runId);
    if (!run) {
      throw new Error(`Smithers run not found: ${input.runId}`);
    }

    const frames = await this.db.listFrames(input.runId, input.limit ?? 50, input.afterFrameNo);
    return frames.map((frame: any) => mapFrameRow(frame));
  }

  async getDevToolsSnapshot(input: { runId: string; frameNo?: number }) {
    return await this.nativeAdapter.getDevToolsSnapshot(input);
  }

  async streamDevTools(input: {
    runId: string;
    afterSeq?: number;
    timeoutMs?: number;
    maxEvents?: number;
    pollIntervalMs?: number;
  }) {
    return await this.nativeAdapter.streamDevTools(input);
  }

  async cancelRun(runId: string) {
    const run = await this.db.getRun(runId);
    if (!run) {
      throw new Error(`Smithers run not found: ${runId}`);
    }
    if (run.status === "waiting-approval" || run.status === "waiting-timer") {
      await this.cancelPausedRun(runId, run.status);
      await this.flushRunEvents(runId, { source: "control" });
      return {
        ok: true,
        runId,
        status: "cancelled",
      };
    }
    if (run.status !== "running" || !isSmithersRunHeartbeatFresh(run)) {
      throw new Error("Run is not currently active");
    }

    await this.db.requestRunCancel(runId, Date.now());
    const monitor = this.monitorByRunId.get(runId);
    monitor?.abortController.abort();
    await this.flushRunEvents(runId, { source: "control" });
    return {
      ok: true,
      runId,
      status: "cancel-requested",
    };
  }

  async listUnresolvedWorkflowRunsForThread(input: {
    sessionId: string;
    threadId: string;
  }): Promise<
    Array<{
      workflowRunId: string;
      smithersRunId: string;
      workflowId: string;
      status: string;
    }>
  > {
    const snapshot = this.options.store.getSessionState(input.sessionId);
    const unresolved: Array<{
      workflowRunId: string;
      smithersRunId: string;
      workflowId: string;
      status: string;
    }> = [];
    for (const workflowRun of snapshot.workflowRuns.filter(
      (entry) => entry.threadId === input.threadId,
    )) {
      const run = await this.db.getRun(workflowRun.smithersRunId);
      if (run && !isTerminalRunStatus(run.status)) {
        unresolved.push({
          workflowRunId: workflowRun.id,
          smithersRunId: workflowRun.smithersRunId,
          workflowId: workflowRun.savedEntryId ?? workflowRun.workflowName,
          status: run.status,
        });
      }
    }
    return unresolved;
  }

  async getDerivedSessionSnapshot(sessionId: string) {
    const snapshot = this.options.store.getSessionState(sessionId);
    const workflowRuns = await Promise.all(
      snapshot.workflowRuns.map(async (workflowRun) => {
        const run = await this.db.getRun(workflowRun.smithersRunId);
        if (!run) {
          return workflowRun;
        }
        const projection = await this.buildWorkflowExecutionProjection({
          run,
          workflowRun,
          ownership: this.ownershipFromWorkflowRun(workflowRun),
        });
        return withWorkflowExecutionProjection(workflowRun, projection);
      }),
    );
    return {
      ...snapshot,
      workflowRuns,
    };
  }

  private async cancelPausedRun(
    runId: string,
    status: Extract<RunStatus, "waiting-approval" | "waiting-timer">,
  ): Promise<void> {
    const cancelledAtMs = Date.now();
    if (status === "waiting-timer") {
      const nodes = await this.db.listNodes(runId);
      for (const node of (nodes as any[]).filter((entry) => entry.state === "waiting-timer")) {
        const iteration = node.iteration ?? 0;
        const attempts = await this.db.listAttempts(runId, node.nodeId, iteration);
        const waitingAttempt = (attempts as any[]).find(
          (attempt) => attempt.state === "waiting-timer",
        );
        if (!waitingAttempt) {
          continue;
        }
        await this.db.updateAttempt(runId, node.nodeId, iteration, waitingAttempt.attempt, {
          state: "cancelled",
          finishedAtMs: cancelledAtMs,
        });
        await this.db.insertNode({
          runId,
          nodeId: node.nodeId,
          iteration,
          state: "cancelled",
          lastAttempt: waitingAttempt.attempt,
          updatedAtMs: cancelledAtMs,
          outputTable: node.outputTable ?? "",
          label: node.label ?? null,
        });
        await this.db.insertEventWithNextSeq({
          runId,
          timestampMs: cancelledAtMs,
          type: "TimerCancelled",
          payloadJson: JSON.stringify({
            type: "TimerCancelled",
            runId,
            timerId: node.nodeId,
            timestampMs: cancelledAtMs,
          }),
        });
      }
    }

    await this.db.updateRun(runId, {
      status: "cancelled",
      finishedAtMs: cancelledAtMs,
      heartbeatAtMs: null,
      runtimeOwnerId: null,
      cancelRequestedAtMs: null,
    });
    await this.db.insertEventWithNextSeq({
      runId,
      timestampMs: cancelledAtMs,
      type: "RunCancelled",
      payloadJson: JSON.stringify({
        type: "RunCancelled",
        runId,
        timestampMs: cancelledAtMs,
      }),
    });
  }

  async restoreSessionSupervision(
    sessionId: string,
    options: {
      emitAttention?: boolean;
    } = {},
  ): Promise<void> {
    await this.refreshWorkflowRegistry();
    const snapshot = this.options.store.getSessionState(sessionId);
    this.hydrateRunOwnershipForSession(sessionId);

    for (const workflowRun of snapshot.workflowRuns) {
      await this.flushRunEvents(workflowRun.smithersRunId, {
        emitAttention: false,
        source: "bootstrap",
      });
      await this.reattachWorkflowRunOnRestore(workflowRun.smithersRunId);
    }

    if (options.emitAttention ?? true) {
      await this.deliverPendingHandlerAttention(sessionId);
    }
  }

  async deliverPendingHandlerAttention(sessionId: string, threadId?: string): Promise<void> {
    const snapshot = this.options.store.getSessionState(sessionId);
    const pendingRuns = snapshot.workflowRuns
      .filter(
        (workflowRun) =>
          (!threadId || workflowRun.threadId === threadId) &&
          workflowRun.pendingAttentionSeq !== null &&
          workflowRun.pendingAttentionSeq !== workflowRun.lastAttentionSeq,
      )
      .toSorted((left, right) => left.updatedAt.localeCompare(right.updatedAt));

    for (const workflowRun of pendingRuns) {
      await this.tryDeliverPendingHandlerAttention(workflowRun.smithersRunId, {
        source: "bootstrap",
      });
    }
  }

  async close(): Promise<void> {
    const seen = new Set<WorkflowMonitor>();
    for (const monitor of this.monitorByRunId.values()) {
      if (seen.has(monitor)) {
        continue;
      }
      seen.add(monitor);
      monitor.abortController.abort();
    }

    if (this.activeWorkflowPromises.size > 0) {
      await Promise.allSettled(Array.from(this.activeWorkflowPromises));
    }

    this.monitorByRunId.clear();
    this.ownershipByRunId.clear();
    this.activeWorkflowPromiseByRunId.clear();
  }

  private createMonitor(input: {
    sessionId: string;
    threadId: string;
    workflowId: string;
    runId: string;
  }): WorkflowMonitor {
    return {
      id: `smithers-monitor-${randomUUID()}`,
      sessionId: input.sessionId,
      threadId: input.threadId,
      workflowId: input.workflowId,
      abortController: new AbortController(),
      trackedRunIds: new Set([input.runId]),
    };
  }

  private trackRunIdWithMonitor(monitor: WorkflowMonitor, runId: string): void {
    monitor.trackedRunIds.add(runId);
    this.monitorByRunId.set(runId, monitor);
  }

  private async runWorkflowInBackground(input: {
    runnableEntry: ReturnType<RunnableWorkflowRegistryEntry["createRunnableEntry"]>;
    monitor: WorkflowMonitor;
    runId: string;
    input: Record<string, unknown>;
    resume: boolean;
  }) {
    try {
      await Effect.runPromise(
        runWorkflow(input.runnableEntry.workflow, {
          runId: input.runId,
          input: input.input,
          resume: input.resume,
          force: input.resume,
          rootDir: this.options.cwd,
          signal: input.monitor.abortController.signal,
          onProgress: (event: SmithersEvent) => {
            void this.handleProgressEvent(input.monitor, event);
          },
        }),
      );
    } catch (error) {
      if (!input.monitor.abortController.signal.aborted) {
        await this.captureUnexpectedWorkflowFailure(input.runId, error);
      }
    } finally {
      await Promise.all(
        Array.from(input.monitor.trackedRunIds).map(async (runId) => {
          await this.flushRunEvents(runId, { source: "progress" });
        }),
      );
    }
  }

  private trackActiveWorkflowPromise(runId: string, workflowPromise: Promise<void>): void {
    this.activeWorkflowPromises.add(workflowPromise);
    this.activeWorkflowPromiseByRunId.set(runId, workflowPromise);
    void workflowPromise.finally(() => {
      this.activeWorkflowPromises.delete(workflowPromise);
      if (this.activeWorkflowPromiseByRunId.get(runId) === workflowPromise) {
        this.activeWorkflowPromiseByRunId.delete(runId);
      }
    });
  }

  private async reattachWorkflowRunOnRestore(runId: string): Promise<void> {
    if (this.activeWorkflowPromiseByRunId.has(runId)) {
      return;
    }

    const ownership = this.ownershipByRunId.get(runId) ?? this.rehydrateRunOwnership(runId);
    if (!ownership) {
      return;
    }
    const workflowRun = this.findStructuredWorkflowRunById(
      ownership.sessionId,
      ownership.structuredWorkflowId,
    );
    if (!workflowRun) {
      return;
    }
    const entry = this.workflowEntriesById.get(ownership.workflowId);
    if (!entry) {
      return;
    }
    const run = await this.db.getRun(runId);
    if (!run || run.status !== "running" || isTerminalRunStatus(run.status)) {
      return;
    }
    const monitor = this.createMonitor({
      sessionId: ownership.sessionId,
      threadId: ownership.threadId,
      workflowId: ownership.workflowId,
      runId,
    });
    this.trackRunIdWithMonitor(monitor, runId);
    const runnableEntry = entry.createRunnableEntry({
      dbPath: this.runtimeDbPath,
    });
    const workflowPromise = this.runWorkflowInBackground({
      runnableEntry,
      monitor,
      runId,
      input: {},
      resume: true,
    });
    this.trackActiveWorkflowPromise(runId, workflowPromise);
  }

  private async handleProgressEvent(monitor: WorkflowMonitor, event: SmithersEvent) {
    if (event.type === "RunContinuedAsNew") {
      await this.ensureContinuedRunOwnership({
        monitor,
        parentRunId: event.runId,
        childRunId: event.newRunId,
      });
    }
    await this.flushRunEvents(event.runId, { source: "progress" });
  }

  private async ensureContinuedRunOwnership(input: {
    monitor: WorkflowMonitor;
    parentRunId: string;
    childRunId: string;
  }) {
    if (this.ownershipByRunId.has(input.childRunId)) {
      this.trackRunIdWithMonitor(input.monitor, input.childRunId);
      return;
    }

    const parentOwnership = this.ownershipByRunId.get(input.parentRunId);
    if (!parentOwnership) {
      return;
    }
    const parentWorkflowRun = this.options.store
      .getSessionState(parentOwnership.sessionId)
      .workflowRuns.find((workflowRun) => workflowRun.id === parentOwnership.structuredWorkflowId);
    const childStructuredRun = this.options.store.recordWorkflow({
      threadId: parentOwnership.threadId,
      commandId: parentOwnership.commandId,
      smithersRunId: input.childRunId,
      workflowName: parentWorkflowRun?.workflowName ?? parentOwnership.workflowId,
      workflowSource: parentWorkflowRun?.workflowSource ?? "artifact",
      entryPath: parentWorkflowRun?.entryPath ?? null,
      savedEntryId: parentWorkflowRun?.savedEntryId ?? null,
      status: "running",
      continuedFromRunIds: [
        ...(parentWorkflowRun?.continuedFromRunIds ?? []),
        parentOwnership.structuredWorkflowId,
      ],
      activeDescendantRunId: null,
      lastEventSeq: null,
      pendingAttentionSeq: null,
      lastAttentionSeq: null,
      heartbeatAt: null,
      summary: `Continuing ${parentWorkflowRun?.workflowName ?? parentOwnership.workflowId} as a new Smithers run.`,
    });
    this.options.store.updateWorkflow({
      workflowId: parentOwnership.structuredWorkflowId,
      commandId: parentOwnership.commandId,
      activeDescendantRunId: childStructuredRun.id,
      pendingAttentionSeq: null,
    });
    this.ownershipByRunId.set(input.childRunId, {
      ...parentOwnership,
      structuredWorkflowId: childStructuredRun.id,
    });
    this.trackRunIdWithMonitor(input.monitor, input.childRunId);
  }

  private async flushRunEvents(
    runId: string,
    options: {
      emitAttention?: boolean;
      source?: ProjectionSource;
    } = {},
  ): Promise<void> {
    while (true) {
      const existing = this.flushPromiseByRunId.get(runId);
      if (!existing) {
        break;
      }
      if (this.deliveringAttentionRunIds.has(runId)) {
        // The handler attention turn may inspect the same run that is currently
        // delivering that attention. Reusing the durable projection avoids
        // waiting on the flush that is waiting on this handler turn.
        return;
      }
      await existing;
    }

    const flushPromise = (async () => {
      const ownership = this.ownershipByRunId.get(runId) ?? this.rehydrateRunOwnership(runId);
      if (!ownership) {
        return;
      }

      const structuredRun = this.findStructuredWorkflowRunById(
        ownership.sessionId,
        ownership.structuredWorkflowId,
      );
      let lastEventSeq = structuredRun?.lastEventSeq ?? null;
      let attentionSeq: number | null = null;
      let attentionReason: string | null = null;

      while (true) {
        const events = await this.db.listEvents(runId, lastEventSeq ?? -1, 200);
        if (events.length === 0) {
          break;
        }

        for (const eventRow of events) {
          const nextSeq = Number(eventRow.seq ?? lastEventSeq ?? -1);
          lastEventSeq = Math.max(lastEventSeq ?? -1, nextSeq);
          const event = parseJson(eventRow.payloadJson) as SmithersEvent | null;
          if (!event) {
            continue;
          }
          if (requiresHandlerAttention(event)) {
            attentionSeq = Number(eventRow.seq);
            attentionReason = describeAttentionEvent(event);
          }
        }

        if (events.length < 200) {
          break;
        }
      }

      await this.refreshStructuredProjection(
        runId,
        {
          lastEventSeq,
          observedAttentionSeq: attentionSeq,
          observedAttentionReason: attentionReason,
        },
        {
          emitAttention: options.emitAttention ?? true,
          source: options.source ?? "progress",
        },
      );
    })().finally(() => {
      this.flushPromiseByRunId.delete(runId);
    });

    this.flushPromiseByRunId.set(runId, flushPromise);
    await flushPromise;
  }

  private async refreshStructuredProjection(
    runId: string,
    input: {
      lastEventSeq: number | null;
      observedAttentionSeq: number | null;
      observedAttentionReason: string | null;
    },
    options: {
      emitAttention: boolean;
      source: ProjectionSource;
    },
  ) {
    const ownership = this.ownershipByRunId.get(runId);
    if (!ownership) {
      return;
    }
    const run = await this.db.getRun(runId);
    if (!run) {
      return;
    }

    const currentWorkflowRun = this.findStructuredWorkflowRunById(
      ownership.sessionId,
      ownership.structuredWorkflowId,
    );
    if (!currentWorkflowRun) {
      return;
    }
    const executionProjection = await this.buildWorkflowExecutionProjection({
      run,
      workflowRun: currentWorkflowRun,
      ownership,
    });
    const currentPendingAttentionSeq = currentWorkflowRun?.pendingAttentionSeq ?? null;
    const currentDeliveredAttentionSeq = currentWorkflowRun?.lastAttentionSeq ?? null;
    const nextPendingAttentionSeq =
      input.observedAttentionSeq !== null &&
      input.observedAttentionSeq >
        Math.max(currentPendingAttentionSeq ?? -1, currentDeliveredAttentionSeq ?? -1)
        ? input.observedAttentionSeq
        : currentPendingAttentionSeq;
    const nextWorkflowRun = this.options.store.updateWorkflow({
      workflowId: ownership.structuredWorkflowId,
      commandId: ownership.commandId,
      lastEventSeq: input.lastEventSeq,
      pendingAttentionSeq: nextPendingAttentionSeq,
      lastAttentionSeq: currentDeliveredAttentionSeq,
    });
    this.recordBridgeLifecycleEvent({
      sessionId: ownership.sessionId,
      workflowRunId: nextWorkflowRun.id,
      kind: "workflowRun.bridge.projected",
      data: {
        source: options.source,
        smithersRunId: runId,
        lastEventSeq: input.lastEventSeq,
        pendingAttentionSeq: nextPendingAttentionSeq,
        lastAttentionSeq: currentDeliveredAttentionSeq,
      },
    });
    if (
      nextPendingAttentionSeq !== null &&
      nextPendingAttentionSeq !== currentPendingAttentionSeq
    ) {
      this.recordBridgeLifecycleEvent({
        sessionId: ownership.sessionId,
        workflowRunId: nextWorkflowRun.id,
        kind: "workflowRun.bridge.attention.pending",
        data: {
          source: options.source,
          smithersRunId: runId,
          attentionSeq: nextPendingAttentionSeq,
        },
      });
    }

    await this.projectWorkflowTaskAttempts(runId, ownership);
    const projectedWorkflowRun = withWorkflowExecutionProjection(
      nextWorkflowRun,
      executionProjection,
    );
    const projectCiProjection = executionProjection.projectCiProjection;
    if (projectCiProjection?.workflowStatus === "completed" && projectCiProjection.result) {
      const entry = this.workflowEntriesById.get(ownership.workflowId);
      this.options.store.recordProjectCiResult({
        workflowRunId: projectedWorkflowRun.id,
        workflowId: ownership.workflowId,
        entryPath: entry?.entryPath ?? projectedWorkflowRun.entryPath ?? "",
        status: projectCiProjection.result.status as StructuredProjectCiStatus,
        summary: projectCiProjection.result.summary,
        startedAt: projectCiProjection.result.startedAt ?? projectedWorkflowRun.startedAt,
        finishedAt: projectCiProjection.result.finishedAt ?? projectedWorkflowRun.finishedAt,
        checks: projectCiProjection.result.checks.map((check) => ({
          checkId: check.checkId,
          label: check.label,
          kind: check.kind,
          status: check.status as StructuredProjectCiCheckStatus,
          required: check.required,
          command: check.command ?? null,
          exitCode: check.exitCode ?? null,
          summary: check.summary,
          artifactIds: check.artifactIds,
          startedAt: check.startedAt ?? null,
          finishedAt: check.finishedAt ?? null,
        })),
      });
    }
    this.applyThreadProjection({
      sessionId: ownership.sessionId,
      threadId: ownership.threadId,
      workflowRun: projectedWorkflowRun,
    });
    await this.emitStructuredStateChanged(ownership.sessionId);

    const currentThread =
      this.options.store
        .getSessionState(ownership.sessionId)
        .threads.find((thread) => thread.id === ownership.threadId) ?? null;
    const isReplayedTerminalStateAfterHandoff = isTerminalWorkflowReplayAfterThreadCompletion(
      currentThread?.status ?? null,
      projectedWorkflowRun.status,
    );

    if (
      options.emitAttention &&
      !isReplayedTerminalStateAfterHandoff &&
      projectedWorkflowRun.pendingAttentionSeq !== null &&
      projectedWorkflowRun.pendingAttentionSeq !== projectedWorkflowRun.lastAttentionSeq
    ) {
      await this.tryDeliverPendingHandlerAttention(runId, {
        source: options.source,
        summary: executionProjection.summary,
        reason:
          input.observedAttentionSeq === projectedWorkflowRun.pendingAttentionSeq
            ? input.observedAttentionReason
            : null,
      });
    }
  }

  private async projectWorkflowTaskAttempts(runId: string, ownership: WorkflowOwnership) {
    const attempts = (await this.db.listAttemptsForRun(runId)) as SmithersAttemptRow[];
    if (attempts.length === 0) {
      return;
    }

    const outputEvents = (await listAllRunEventsByType(this.db, runId, ["NodeOutput"]))
      .map((event) => parseNodeOutputEvent(event as any))
      .filter((event): event is NonNullable<typeof event> => event !== null);
    const outputEventsByAttemptKey = new Map<string, typeof outputEvents>();
    for (const outputEvent of outputEvents) {
      if (!outputEvent.nodeId) {
        continue;
      }
      const key = chatAttemptKey({
        nodeId: outputEvent.nodeId,
        iteration: outputEvent.iteration ?? 0,
        attempt: outputEvent.attempt ?? 1,
      });
      const list = outputEventsByAttemptKey.get(key) ?? [];
      list.push(outputEvent);
      outputEventsByAttemptKey.set(key, list);
    }

    for (const attempt of attempts) {
      const meta = parseJson(attempt.metaJson);
      const structuredAttempt = this.options.store.upsertWorkflowTaskAttempt({
        workflowRunId: ownership.structuredWorkflowId,
        smithersRunId: runId,
        nodeId: attempt.nodeId,
        iteration: attempt.iteration ?? 0,
        attempt: attempt.attempt,
        surfacePiSessionId: readMetaString(meta, "agentSurfacePiSessionId"),
        title: readMetaString(meta, "label") ?? attempt.nodeId,
        summary: summarizeWorkflowTaskAttemptProjection(attempt, meta),
        kind: readWorkflowTaskAttemptKind(meta),
        status: mapAttemptStateToWorkflowTaskAttemptStatus(attempt.state),
        smithersState: attempt.state,
        prompt: readMetaString(meta, "prompt"),
        responseText: attempt.responseText ?? null,
        error: parseAttemptErrorMessage(attempt.errorJson),
        cached: Boolean(attempt.cached),
        jjPointer: attempt.jjPointer ?? null,
        jjCwd: attempt.jjCwd ?? null,
        heartbeatAt: toIso(attempt.heartbeatAtMs),
        agentId: readMetaString(meta, "agentId"),
        agentModel: readMetaString(meta, "agentModel"),
        agentEngine: readMetaString(meta, "agentEngine"),
        agentResume:
          readMetaString(meta, "agentResume") ??
          readNestedMetaString(meta, "hijackHandoff", "resume"),
        meta,
        startedAt: toIso(attempt.startedAtMs) ?? new Date(0).toISOString(),
        finishedAt: toIso(attempt.finishedAtMs),
      });

      this.options.store.replaceWorkflowTaskMessages({
        workflowTaskAttemptId: structuredAttempt.id,
        messages: buildWorkflowTaskAttemptMessages({
          runId,
          attempt,
          prompt: readMetaString(meta, "prompt"),
          responseText: attempt.responseText ?? null,
          outputEvents:
            outputEventsByAttemptKey.get(
              chatAttemptKey({
                nodeId: attempt.nodeId,
                iteration: attempt.iteration ?? 0,
                attempt: attempt.attempt,
              }),
            ) ?? [],
        }),
      });
    }
  }

  private async resolveProjectCiProjection(input: {
    runId: string;
    ownership: WorkflowOwnership;
    runStatus: RunStatus;
  }): Promise<ProjectCiProjection | null> {
    if (input.runStatus !== "finished") {
      return null;
    }

    const entry = this.workflowEntriesById.get(input.ownership.workflowId);
    if (entry?.productKind !== "project-ci") {
      return null;
    }

    const existingCiRun =
      this.options.store
        .getSessionState(input.ownership.sessionId)
        .ciRuns.find((ciRun) => ciRun.workflowRunId === input.ownership.structuredWorkflowId) ??
      null;
    if (existingCiRun) {
      return {
        workflowStatus: "completed",
        summary: existingCiRun.summary,
        result: null,
      };
    }

    if (!entry.resultSchema) {
      return {
        workflowStatus: "failed",
        summary:
          "Project CI entry declared productKind but did not provide a result schema, so no CI records were created.",
        result: null,
      };
    }

    const output = await this.readDurableTerminalOutput(input.runId);
    if (!output) {
      return {
        workflowStatus: "failed",
        summary:
          "Project CI Smithers run finished without durable terminal output, so no CI records were created.",
        result: null,
      };
    }

    const entryValidation = entry.resultSchema.safeParse(output);
    if (!entryValidation.success) {
      return {
        workflowStatus: "failed",
        summary:
          "Project CI terminal result output did not validate against the entry result schema.",
        result: null,
      };
    }

    const projectionValidation = projectCiResultProjectionSchema.safeParse(entryValidation.data);
    if (!projectionValidation.success) {
      return {
        workflowStatus: "failed",
        summary:
          "Project CI terminal result output validated against its entry schema but did not match the Project CI projection contract.",
        result: null,
      };
    }

    return {
      workflowStatus: "completed",
      summary: projectionValidation.data.summary,
      result: projectionValidation.data,
    };
  }

  private async readDurableTerminalOutput(runId: string): Promise<unknown[] | null> {
    const nodes = (
      (await this.db.listNodes(runId)) as Array<{
        nodeId?: string;
        node_id?: string;
        iteration?: number | null;
        updatedAtMs?: number | null;
        updated_at_ms?: number | null;
        outputTable?: string | null;
        output_table?: string | null;
      }>
    )
      .map((node) => ({
        nodeId: node.nodeId ?? node.node_id,
        iteration: node.iteration ?? 0,
        updatedAtMs: node.updatedAtMs ?? node.updated_at_ms ?? 0,
        outputTable: node.outputTable ?? node.output_table,
      }))
      .filter((node) => node.outputTable === "output" && typeof node.nodeId === "string");
    if (nodes.length === 0) {
      return null;
    }

    const rows: unknown[] = [];
    for (const node of nodes.toSorted(compareDurableOutputNodes)) {
      const rawOutput =
        (await this.db.getRawNodeOutputForIteration(
          "output",
          runId,
          node.nodeId!,
          node.iteration,
        )) ?? readRawNodeOutputForIteration(this.db, "output", runId, node.nodeId!, node.iteration);
      if (!rawOutput) {
        continue;
      }
      rows.push(normalizeDurableSmithersOutputRow(rawOutput));
    }

    return rows.length > 0 ? rows : null;
  }

  private applyThreadProjection(input: {
    sessionId: string;
    threadId: string;
    workflowRun: StructuredWorkflowRunRecord;
  }) {
    const currentThread =
      this.options.store
        .getSessionState(input.sessionId)
        .threads.find((thread) => thread.id === input.threadId) ?? null;
    const { workflowRun } = input;
    if (
      isTerminalWorkflowReplayAfterThreadCompletion(
        currentThread?.status ?? null,
        workflowRun.status,
      )
    ) {
      // The same terminal Smithers state can be observed more than once through the live
      // progress callback, the monitor's final flush, or later recovery reads. Once the
      // handler has reconciled that terminal result and closed the span with thread.handoff,
      // replaying it must not reopen the thread.
      this.clearThreadOwnedSessionWait(input.sessionId, input.threadId);
      return;
    }

    switch (workflowRun.status) {
      case "running":
        this.options.store.updateThread({
          threadId: input.threadId,
          status: "running-workflow",
          wait: null,
        });
        this.clearThreadOwnedSessionWait(input.sessionId, input.threadId);
        break;
      case "waiting": {
        const wait = {
          owner: "workflow" as const,
          kind: deriveThreadWaitKind(workflowRun),
          reason: workflowRun.summary,
          resumeWhen: describeWaitResumeWhen(workflowRun.waitKind),
          since: workflowRun.updatedAt,
        };
        this.options.store.updateThread({
          threadId: input.threadId,
          status: "waiting",
          wait,
        });
        try {
          this.options.store.setSessionWait({
            sessionId: input.sessionId,
            owner: {
              kind: "thread",
              threadId: input.threadId,
            },
            kind: wait.kind,
            reason: wait.reason,
            resumeWhen: wait.resumeWhen,
          });
        } catch {
          // Another runnable thread still exists; keep the thread-local wait only.
        }
        break;
      }
      case "continued":
        this.options.store.updateThread({
          threadId: input.threadId,
          status: "troubleshooting",
          wait: null,
        });
        this.clearThreadOwnedSessionWait(input.sessionId, input.threadId);
        break;
      case "completed":
        // Workflow completion returns control to the handler. The delegated objective stays
        // active until the handler explicitly closes the current span with thread.handoff.
        this.options.store.updateThread({
          threadId: input.threadId,
          status: "running-handler",
          wait: null,
        });
        this.clearThreadOwnedSessionWait(input.sessionId, input.threadId);
        break;
      case "failed":
      case "cancelled":
        this.options.store.updateThread({
          threadId: input.threadId,
          status: "troubleshooting",
          wait: null,
        });
        this.clearThreadOwnedSessionWait(input.sessionId, input.threadId);
        break;
    }
  }

  private async captureUnexpectedWorkflowFailure(runId: string, error: unknown) {
    const ownership = this.ownershipByRunId.get(runId);
    if (!ownership) {
      return;
    }
    const currentWorkflowRun = this.findStructuredWorkflowRunById(
      ownership.sessionId,
      ownership.structuredWorkflowId,
    );
    const message =
      error instanceof Error
        ? error.message
        : "The supervised Smithers workflow failed unexpectedly.";
    const nextPendingAttentionSeq =
      Math.max(
        currentWorkflowRun?.pendingAttentionSeq ?? -1,
        currentWorkflowRun?.lastAttentionSeq ?? -1,
        currentWorkflowRun?.lastEventSeq ?? -1,
      ) + 1;
    const nextWorkflowRun = this.options.store.updateWorkflow({
      workflowId: ownership.structuredWorkflowId,
      commandId: ownership.commandId,
      continuedFromRunIds: currentWorkflowRun?.continuedFromRunIds,
      activeDescendantRunId: currentWorkflowRun?.activeDescendantRunId ?? null,
      lastEventSeq: currentWorkflowRun?.lastEventSeq ?? null,
      pendingAttentionSeq: nextPendingAttentionSeq,
      lastAttentionSeq: currentWorkflowRun?.lastAttentionSeq ?? null,
    });
    this.recordBridgeLifecycleEvent({
      sessionId: ownership.sessionId,
      workflowRunId: nextWorkflowRun.id,
      kind: "workflowRun.bridge.projected",
      data: {
        source: "failure",
        smithersRunId: runId,
        lastEventSeq: nextWorkflowRun.lastEventSeq,
        pendingAttentionSeq: nextWorkflowRun.pendingAttentionSeq,
        lastAttentionSeq: nextWorkflowRun.lastAttentionSeq,
      },
    });
    this.recordBridgeLifecycleEvent({
      sessionId: ownership.sessionId,
      workflowRunId: nextWorkflowRun.id,
      kind: "workflowRun.bridge.attention.pending",
      data: {
        source: "failure",
        smithersRunId: runId,
        attentionSeq: nextPendingAttentionSeq,
      },
    });
    this.applyThreadProjection({
      sessionId: ownership.sessionId,
      threadId: ownership.threadId,
      workflowRun: withWorkflowExecutionProjection(nextWorkflowRun, {
        status: "failed",
        smithersStatus: "failed",
        waitKind: null,
        heartbeatAt: null,
        finishedAt: new Date().toISOString(),
        summary: message,
        projectCiProjection: null,
      }),
    });
    await this.emitStructuredStateChanged(ownership.sessionId);
    await this.tryDeliverPendingHandlerAttention(runId, {
      source: "failure",
      summary: message,
      reason: "The supervised Smithers workflow failed unexpectedly.",
    });
  }

  private clearThreadOwnedSessionWait(sessionId: string, threadId: string) {
    const wait = this.options.store.getSessionState(sessionId).session.wait;
    if (wait?.owner.kind === "thread" && wait.owner.threadId === threadId) {
      this.options.store.clearSessionWait({ sessionId });
    }
  }

  private hydrateRunOwnershipForSession(sessionId: string): void {
    const snapshot = this.options.store.getSessionState(sessionId);
    for (const workflowRun of snapshot.workflowRuns) {
      this.ownershipByRunId.set(workflowRun.smithersRunId, {
        sessionId: workflowRun.sessionId,
        threadId: workflowRun.threadId,
        workflowId: workflowRun.savedEntryId ?? workflowRun.workflowName,
        structuredWorkflowId: workflowRun.id,
        commandId: workflowRun.commandId,
      });
    }
  }

  private rehydrateRunOwnership(runId: string): WorkflowOwnership | null {
    const workflowRun = this.findStructuredWorkflowRunBySmithersRunId(runId);
    if (!workflowRun) {
      return null;
    }
    const ownership = {
      sessionId: workflowRun.sessionId,
      threadId: workflowRun.threadId,
      workflowId: workflowRun.savedEntryId ?? workflowRun.workflowName,
      structuredWorkflowId: workflowRun.id,
      commandId: workflowRun.commandId,
    } satisfies WorkflowOwnership;
    this.ownershipByRunId.set(runId, ownership);
    return ownership;
  }

  private async tryDeliverPendingHandlerAttention(
    runId: string,
    input: {
      source: ProjectionSource;
      summary?: string;
      reason?: string | null;
    },
  ): Promise<boolean> {
    const ownership = this.ownershipByRunId.get(runId) ?? this.rehydrateRunOwnership(runId);
    if (!ownership) {
      return false;
    }

    const workflowRun = this.findStructuredWorkflowRunById(
      ownership.sessionId,
      ownership.structuredWorkflowId,
    );
    const run = await this.db.getRun(runId);
    const executionProjection =
      workflowRun && run
        ? await this.buildWorkflowExecutionProjection({ run, workflowRun, ownership })
        : null;
    if (
      !workflowRun ||
      workflowRun.pendingAttentionSeq === null ||
      workflowRun.pendingAttentionSeq === workflowRun.lastAttentionSeq
    ) {
      return false;
    }

    const thread =
      this.options.store
        .getSessionState(ownership.sessionId)
        .threads.find((entry) => entry.id === ownership.threadId) ?? null;
    if (
      isTerminalWorkflowReplayAfterThreadCompletion(
        thread?.status ?? null,
        executionProjection?.status ?? workflowRun.status,
      )
    ) {
      return false;
    }

    let delivered = false;
    this.deliveringAttentionRunIds.add(runId);
    try {
      delivered =
        (await this.options.onHandlerAttention?.({
          sessionId: ownership.sessionId,
          threadId: ownership.threadId,
          workflowRunId: workflowRun.id,
          smithersRunId: runId,
          workflowId: ownership.workflowId,
          summary: input.summary ?? executionProjection?.summary ?? workflowRun.summary,
          reason:
            input.reason ??
            (await this.readPendingAttentionReason(runId, workflowRun.pendingAttentionSeq)) ??
            "The supervised workflow needs handler attention.",
        })) ?? false;
    } finally {
      this.deliveringAttentionRunIds.delete(runId);
    }
    if (!delivered) {
      return false;
    }

    this.options.store.updateWorkflow({
      workflowId: workflowRun.id,
      commandId: workflowRun.commandId,
      continuedFromRunIds: workflowRun.continuedFromRunIds,
      activeDescendantRunId: workflowRun.activeDescendantRunId,
      lastEventSeq: workflowRun.lastEventSeq,
      pendingAttentionSeq: null,
      lastAttentionSeq: workflowRun.pendingAttentionSeq,
    });
    this.recordBridgeLifecycleEvent({
      sessionId: ownership.sessionId,
      workflowRunId: workflowRun.id,
      kind: "workflowRun.bridge.attention.delivered",
      data: {
        source: input.source,
        smithersRunId: runId,
        attentionSeq: workflowRun.pendingAttentionSeq,
      },
    });
    await this.emitStructuredStateChanged(ownership.sessionId);
    return true;
  }

  private async readPendingAttentionReason(
    runId: string,
    attentionSeq: number,
  ): Promise<string | null> {
    const events = await this.db.listEvents(runId, attentionSeq - 1, 10);
    const eventRow = events.find((entry: any) => Number(entry.seq) === attentionSeq);
    if (!eventRow) {
      return null;
    }
    const event = parseJson(eventRow.payloadJson) as SmithersEvent | null;
    return event ? describeAttentionEvent(event) : null;
  }

  private recordBridgeLifecycleEvent(input: {
    sessionId: string;
    workflowRunId: string;
    kind: string;
    data?: Record<string, unknown>;
  }): void {
    this.options.store.recordLifecycleEvent({
      sessionId: input.sessionId,
      kind: input.kind,
      subjectKind: "workflowRun",
      subjectId: input.workflowRunId,
      data: input.data,
    });
  }

  private async emitStructuredStateChanged(sessionId: string) {
    await this.options.onStructuredStateChanged?.(sessionId);
  }

  private async getRunDiagnosis(runId: string): Promise<any> {
    return await Effect.runPromise(diagnoseRunEffect(this.db as any, runId));
  }

  private requireWorkflowEntry(workflowId: string): RunnableWorkflowRegistryEntry {
    const workflow = this.workflowEntriesById.get(workflowId);
    if (!workflow) {
      throw new Error(`Runnable Smithers workflow not found: ${workflowId}`);
    }
    return workflow;
  }

  private findStructuredWorkflowRun(input: {
    sessionId: string;
    threadId: string;
    runId: string;
  }): StructuredWorkflowRunRecord | null {
    return (
      this.options.store
        .getSessionState(input.sessionId)
        .workflowRuns.find(
          (workflowRun) =>
            workflowRun.threadId === input.threadId && workflowRun.smithersRunId === input.runId,
        ) ?? null
    );
  }

  private async requireExplicitResumeOwnership(input: {
    sessionId: string;
    threadId: string;
    workflowId: string;
    runId: string;
  }): Promise<void> {
    const workflowRun = this.findStructuredWorkflowRunBySmithersRunId(input.runId);
    if (!workflowRun) {
      throw new Error(
        `Smithers run ${input.runId} is not owned by a svvy handler thread; cannot resume it from smithers.run_workflow.`,
      );
    }
    if (workflowRun.sessionId !== input.sessionId) {
      throw new Error(
        `Smithers run ${input.runId} belongs to session ${workflowRun.sessionId}, not ${input.sessionId}.`,
      );
    }
    if (workflowRun.threadId !== input.threadId) {
      throw new Error(
        `Smithers run ${input.runId} belongs to handler thread ${workflowRun.threadId}, not ${input.threadId}.`,
      );
    }
    if (
      workflowRun.savedEntryId !== input.workflowId &&
      workflowRun.workflowName !== input.workflowId
    ) {
      const actualWorkflowId = workflowRun.savedEntryId ?? workflowRun.workflowName;
      throw new Error(
        `Smithers run ${input.runId} belongs to workflow ${actualWorkflowId}, not ${input.workflowId}.`,
      );
    }
    const smithersRun = await this.db.getRun(input.runId);
    if (!smithersRun) {
      throw new Error(`Smithers run ${input.runId} was not found in the Smithers runtime.`);
    }
    if (isTerminalRunStatus(smithersRun.status)) {
      throw new Error(
        `Smithers run ${input.runId} is already ${smithersRun.status}; cannot resume a terminal workflow run.`,
      );
    }
  }

  private async findNonterminalThreadRun(
    sessionId: string,
    threadId: string,
    workflowId: string,
  ): Promise<StructuredWorkflowRunRecord | null> {
    const candidates = this.options.store
      .getSessionState(sessionId)
      .workflowRuns.filter(
        (workflowRun) =>
          workflowRun.threadId === threadId &&
          (workflowRun.savedEntryId === workflowId || workflowRun.workflowName === workflowId),
      )
      .toSorted((left, right) => right.updatedAt.localeCompare(left.updatedAt));

    for (const workflowRun of candidates) {
      const smithersRun = await this.db.getRun(workflowRun.smithersRunId);
      if (smithersRun && !isTerminalRunStatus(smithersRun.status)) {
        return workflowRun;
      }
    }
    return null;
  }

  private findStructuredWorkflowRunBySmithersRunId(
    runId: string,
  ): StructuredWorkflowRunRecord | null {
    const inMemoryOwnership = this.ownershipByRunId.get(runId);
    if (inMemoryOwnership) {
      return this.findStructuredWorkflowRunById(
        inMemoryOwnership.sessionId,
        inMemoryOwnership.structuredWorkflowId,
      );
    }

    for (const session of this.options.store.listSessionStates()) {
      const match = session.workflowRuns.find((workflowRun) => workflowRun.smithersRunId === runId);
      if (match) {
        return match;
      }
    }

    return null;
  }

  private findStructuredWorkflowRunById(
    sessionId: string,
    workflowRunId: string,
  ): StructuredWorkflowRunRecord | null {
    return (
      this.options.store
        .getSessionState(sessionId)
        .workflowRuns.find((workflowRun) => workflowRun.id === workflowRunId) ?? null
    );
  }

  private listStructuredRunBySmithersRunId(): Map<string, StructuredWorkflowRunRecord> {
    const workflowRunsBySmithersRunId = new Map<string, StructuredWorkflowRunRecord>();
    for (const session of this.options.store.listSessionStates()) {
      for (const workflowRun of session.workflowRuns) {
        if (!workflowRunsBySmithersRunId.has(workflowRun.smithersRunId)) {
          workflowRunsBySmithersRunId.set(workflowRun.smithersRunId, workflowRun);
        }
      }
    }
    return workflowRunsBySmithersRunId;
  }

  private ownershipFromWorkflowRun(workflowRun: StructuredWorkflowRunRecord): WorkflowOwnership {
    return {
      sessionId: workflowRun.sessionId,
      threadId: workflowRun.threadId,
      workflowId: workflowRun.savedEntryId ?? workflowRun.workflowName,
      structuredWorkflowId: workflowRun.id,
      commandId: workflowRun.commandId,
    };
  }

  private async buildWorkflowExecutionProjection(input: {
    run: any;
    workflowRun: StructuredWorkflowRunRecord;
    ownership: WorkflowOwnership;
  }): Promise<WorkflowExecutionProjection> {
    const projectCiProjection = await this.resolveProjectCiProjection({
      runId: input.run.runId,
      ownership: input.ownership,
      runStatus: input.run.status,
    });
    const diagnosis =
      input.run.status === "waiting-event" ? await this.getRunDiagnosis(input.run.runId) : null;
    return {
      status: projectCiProjection?.workflowStatus ?? mapRunStatusToWorkflowStatus(input.run.status),
      smithersStatus: input.run.status,
      waitKind: mapRunStatusToWaitKind(input.run.status),
      heartbeatAt: toIso(input.run.heartbeatAtMs),
      finishedAt: toIso(input.run.finishedAtMs),
      summary:
        projectCiProjection?.summary ??
        diagnosis?.summary ??
        (await this.buildRunSummary(input.run)),
      projectCiProjection,
    };
  }

  private async buildRunSummary(run: any): Promise<string> {
    const nodeCounts = await this.db.countNodesByState(run.runId);
    const countsText = nodeCounts.map((entry: any) => `${entry.count} ${entry.state}`).join(", ");
    const parts = [`${run.workflowName} is ${describeRunStatus(run.status)}`];
    if (countsText) {
      parts.push(countsText);
    }
    return `${parts.join("; ")}.`;
  }

  private async buildRunSummaryFromNative(run: Record<string, unknown>): Promise<string> {
    const workflowName =
      typeof run.workflowName === "string" && run.workflowName.trim().length > 0
        ? run.workflowName
        : "Smithers workflow";
    const status = typeof run.status === "string" ? run.status : "unknown";
    const countsByState =
      run.countsByState && typeof run.countsByState === "object"
        ? (run.countsByState as Record<string, unknown>)
        : {};
    const countsText = Object.entries(countsByState)
      .map(([state, count]) => `${String(count)} ${state}`)
      .join(", ");
    const parts = [`${workflowName} is ${describeRunStatus(status)}`];
    if (countsText) {
      parts.push(countsText);
    }
    return `${parts.join("; ")}.`;
  }
}

function mapRunStatusToWorkflowStatus(status: RunStatus): StructuredWorkflowStatus {
  switch (status) {
    case "running":
      return "running";
    case "waiting-approval":
    case "waiting-event":
    case "waiting-timer":
      return "waiting";
    case "continued":
      return "continued";
    case "finished":
      return "completed";
    case "failed":
      return "failed";
    case "cancelled":
      return "cancelled";
  }
}

function withWorkflowExecutionProjection(
  workflowRun: StructuredWorkflowRunRecord,
  projection: WorkflowExecutionProjection,
): StructuredWorkflowRunRecord {
  return {
    ...workflowRun,
    status: projection.status,
    smithersStatus: projection.smithersStatus,
    waitKind: projection.waitKind,
    heartbeatAt: projection.heartbeatAt,
    summary: projection.summary,
    finishedAt: projection.finishedAt,
  };
}

function isSmithersRunHeartbeatFresh(run: {
  status?: string | null;
  heartbeatAtMs?: number | null;
}): boolean {
  return (
    run.status === "running" &&
    typeof run.heartbeatAtMs === "number" &&
    Date.now() - run.heartbeatAtMs <= 5_000
  );
}

function mapAttemptStateToWorkflowTaskAttemptStatus(
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

function readWorkflowTaskAttemptKind(
  meta: Record<string, unknown> | null,
): "agent" | "compute" | "static" | "unknown" {
  const kind = readMetaString(meta, "kind");
  return kind === "agent" || kind === "compute" || kind === "static" ? kind : "unknown";
}

function readMetaString(meta: Record<string, unknown> | null, key: string): string | null {
  return meta && typeof meta[key] === "string" ? (meta[key] as string) : null;
}

function readNestedMetaString(
  meta: Record<string, unknown> | null,
  key: string,
  nestedKey: string,
): string | null {
  const record = meta?.[key];
  if (!record || typeof record !== "object" || Array.isArray(record)) {
    return null;
  }
  return typeof (record as Record<string, unknown>)[nestedKey] === "string"
    ? ((record as Record<string, unknown>)[nestedKey] as string)
    : null;
}

function parseAttemptErrorMessage(errorJson: string | null | undefined): string | null {
  const error = parseJson(errorJson);
  if (!error || typeof error !== "object") {
    return null;
  }
  return typeof (error as Record<string, unknown>).message === "string"
    ? ((error as Record<string, unknown>).message as string)
    : null;
}

function summarizeWorkflowTaskAttemptProjection(
  attempt: SmithersAttemptRow,
  meta: Record<string, unknown> | null,
): string {
  const label = readMetaString(meta, "label") ?? attempt.nodeId;
  const error = parseAttemptErrorMessage(attempt.errorJson);
  switch (attempt.state) {
    case "waiting-timer":
      return `${label} is waiting on a timer.`;
    case "finished":
      return attempt.responseText?.trim()
        ? `${label} finished with a task-agent response.`
        : `${label} finished.`;
    case "failed":
      return error ? `${label} failed: ${error}` : `${label} failed.`;
    case "cancelled":
      return `${label} was cancelled.`;
    default:
      return `${label} is running.`;
  }
}

function buildWorkflowTaskAttemptMessages(input: {
  runId: string;
  attempt: SmithersAttemptRow;
  prompt: string | null;
  responseText: string | null;
  outputEvents: ParsedNodeOutputEvent[];
}): Array<{
  id: string;
  role: "user" | "assistant" | "stderr";
  source: "prompt" | "event" | "responseText";
  smithersEventSeq?: number | null;
  text: string;
  createdAt: string;
}> {
  const messages: Array<{
    id: string;
    role: "user" | "assistant" | "stderr";
    source: "prompt" | "event" | "responseText";
    smithersEventSeq?: number | null;
    text: string;
    createdAt: string;
  }> = [];
  const attemptRef = `${input.runId}:${input.attempt.nodeId}:${input.attempt.iteration}:${input.attempt.attempt}`;
  const prompt = input.prompt?.trim() ?? "";
  if (prompt) {
    messages.push({
      id: `workflow-task-message:${attemptRef}:prompt`,
      role: "user",
      source: "prompt",
      text: prompt,
      createdAt: toIso(input.attempt.startedAtMs) ?? new Date(0).toISOString(),
    });
  }

  const sortedOutputEvents = input.outputEvents.toSorted((left, right) =>
    (left.timestampMs ?? 0) === (right.timestampMs ?? 0)
      ? (left.seq ?? 0) - (right.seq ?? 0)
      : (left.timestampMs ?? 0) - (right.timestampMs ?? 0),
  );
  let sawAssistantOutput = false;
  for (const outputEvent of sortedOutputEvents) {
    if (outputEvent.stream === "stdout") {
      sawAssistantOutput = true;
    }
    messages.push({
      id: `workflow-task-message:${input.runId}:event:${outputEvent.seq}`,
      role: outputEvent.stream === "stderr" ? "stderr" : "assistant",
      source: "event",
      smithersEventSeq: outputEvent.seq,
      text: outputEvent.text ?? "",
      createdAt: toIso(outputEvent.timestampMs) ?? new Date(0).toISOString(),
    });
  }

  const responseText = input.responseText?.trim() ?? "";
  if (responseText && !sawAssistantOutput) {
    messages.push({
      id: `workflow-task-message:${attemptRef}:response`,
      role: "assistant",
      source: "responseText",
      text: responseText,
      createdAt:
        toIso(input.attempt.finishedAtMs ?? input.attempt.startedAtMs) ?? new Date(0).toISOString(),
    });
  }

  return messages;
}

function mapRunStatusToWaitKind(status: RunStatus): StructuredWorkflowWaitKind | null {
  switch (status) {
    case "waiting-approval":
      return "approval";
    case "waiting-event":
      return "event";
    case "waiting-timer":
      return "timer";
    default:
      return null;
  }
}

function requiresHandlerAttention(event: SmithersEvent): boolean {
  switch (event.type) {
    case "RunFinished":
    case "RunFailed":
    case "RunCancelled":
    case "RunContinuedAsNew":
    case "ApprovalRequested":
      return true;
    case "RunStatusChanged":
      return event.status === "waiting-approval" || event.status === "waiting-event";
    default:
      return false;
  }
}

function describeAttentionEvent(event: SmithersEvent): string {
  switch (event.type) {
    case "RunFinished":
      return "The supervised workflow finished and the handler must reconcile the result.";
    case "RunFailed":
      return "The supervised workflow failed and the handler must troubleshoot it.";
    case "RunCancelled":
      return "The supervised workflow was cancelled and the handler must decide what to do next.";
    case "RunContinuedAsNew":
      return "Smithers continued the workflow as a new run and the handler must keep supervising it.";
    case "ApprovalRequested":
      return "The supervised workflow is waiting on approval.";
    case "RunStatusChanged":
      return event.status === "waiting-approval"
        ? "The supervised workflow is waiting on approval."
        : "The supervised workflow is waiting on an external event or signal.";
    default:
      return "The supervised workflow needs handler attention.";
  }
}

function describeRunStatus(status: string): string {
  switch (status) {
    case "waiting-approval":
      return "waiting for approval";
    case "waiting-event":
      return "waiting for an external event";
    case "waiting-timer":
      return "waiting on a timer";
    case "finished":
      return "completed";
    default:
      return status;
  }
}

function deriveThreadWaitKind(
  workflowRun: StructuredWorkflowRunRecord,
): "approval" | "signal" | "timer" | "external" {
  switch (workflowRun.waitKind) {
    case "approval":
      return "approval";
    case "timer":
      return "timer";
    case "event":
      return "signal";
    default:
      return "external";
  }
}

function describeWaitResumeWhen(waitKind: StructuredWorkflowWaitKind | null): string {
  switch (waitKind) {
    case "approval":
      return "Resume when the approval is resolved.";
    case "timer":
      return "Resume when the timer fires.";
    case "event":
      return "Resume when the required event or signal arrives.";
    default:
      return "Resume when the workflow can make forward progress again.";
  }
}

function isTerminalWorkflowStatus(status: StructuredWorkflowStatus): boolean {
  return status === "completed" || status === "failed" || status === "cancelled";
}

function isTerminalRunStatus(status: string): boolean {
  return status === "finished" || status === "failed" || status === "cancelled";
}

function isTerminalWorkflowReplayAfterThreadCompletion(
  threadStatus: string | null,
  workflowStatus: StructuredWorkflowStatus,
): boolean {
  return threadStatus === "completed" && isTerminalWorkflowStatus(workflowStatus);
}

function compareDurableOutputNodes(
  left: { nodeId?: string; iteration: number; updatedAtMs: number },
  right: { nodeId?: string; iteration: number; updatedAtMs: number },
): number {
  const updatedDelta = left.updatedAtMs - right.updatedAtMs;
  if (updatedDelta !== 0) {
    return updatedDelta;
  }
  const nodeDelta = (left.nodeId ?? "").localeCompare(right.nodeId ?? "");
  if (nodeDelta !== 0) {
    return nodeDelta;
  }
  return left.iteration - right.iteration;
}

function readRawNodeOutputForIteration(
  db: SmithersDb,
  tableName: string,
  runId: string,
  nodeId: string,
  iteration: number,
): Record<string, unknown> | null {
  try {
    const sqlite = (db as unknown as { db?: Database }).db;
    if (!sqlite) {
      return null;
    }
    const escaped = tableName.replaceAll(`"`, `""`);
    return (
      (sqlite
        .query(
          `SELECT * FROM "${escaped}" WHERE run_id = ? AND node_id = ? AND iteration = ? LIMIT 1`,
        )
        .get(runId, nodeId, iteration) as Record<string, unknown> | undefined) ?? null
    );
  } catch {
    return null;
  }
}

function normalizeDurableSmithersOutputRow(row: Record<string, unknown>): Record<string, unknown> {
  const normalized: Record<string, unknown> = {};
  for (const [rawKey, rawValue] of Object.entries(row)) {
    if (
      rawKey === "runId" ||
      rawKey === "run_id" ||
      rawKey === "nodeId" ||
      rawKey === "node_id" ||
      rawKey === "iteration"
    ) {
      continue;
    }
    normalized[snakeToCamel(rawKey)] = parseDurableSqliteJsonValue(rawValue);
  }
  return normalized;
}

function parseDurableSqliteJsonValue(value: unknown): unknown {
  if (typeof value !== "string") {
    return value;
  }
  const trimmed = value.trim();
  if (!trimmed || (!trimmed.startsWith("{") && !trimmed.startsWith("["))) {
    return value;
  }
  try {
    return JSON.parse(trimmed);
  } catch {
    return value;
  }
}

function snakeToCamel(value: string): string {
  return value.replace(/_([a-z])/g, (_match, char: string) => char.toUpperCase());
}

function parseJson(value: string | null | undefined): any {
  if (!value) {
    return null;
  }
  try {
    return JSON.parse(value);
  } catch {
    return null;
  }
}

function toIso(timestampMs: number | null | undefined): string | null {
  return typeof timestampMs === "number" ? new Date(timestampMs).toISOString() : null;
}

function mapFrameRow(frame: {
  runId: string;
  frameNo: number;
  createdAtMs: number;
  xmlJson: string;
  xmlHash: string;
  encoding: string;
  mountedTaskIdsJson: string | null;
  taskIndexJson: string | null;
  note: string | null;
}) {
  return {
    runId: frame.runId,
    frameNo: frame.frameNo,
    createdAtMs: frame.createdAtMs,
    createdAt: toIso(frame.createdAtMs),
    xml: parseJson(frame.xmlJson),
    xmlHash: frame.xmlHash,
    encoding: frame.encoding,
    mountedTaskIds: parseJson(frame.mountedTaskIdsJson),
    taskIndex: parseJson(frame.taskIndexJson),
    note: frame.note ?? null,
  };
}

async function listAllRunEventsByType(
  db: SmithersDb,
  runId: string,
  types: string[],
): Promise<Array<Record<string, unknown>>> {
  const events: Array<Record<string, unknown>> = [];
  let afterSeq = -1;

  while (true) {
    const batch = await db.listEventHistory(runId, {
      afterSeq,
      limit: 1_000,
      types,
    });
    if (batch.length === 0) {
      break;
    }
    events.push(...(batch as Array<Record<string, unknown>>));
    const lastEvent = batch[batch.length - 1];
    afterSeq = Number(lastEvent?.seq ?? afterSeq);
    if (batch.length < 1_000) {
      break;
    }
  }

  return events;
}
