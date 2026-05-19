import type { SmithersDb } from "@smithers-orchestrator/db";
import { getDevToolsSnapshotRoute, streamDevToolsRoute } from "@smithers-orchestrator/server";
import {
  createSemanticToolDefinitions,
  type SemanticToolCallResult,
  type SemanticToolDefinition,
} from "@smithers-orchestrator/cli/mcp/semantic-tools";

export type NativeResolveApprovalInput = {
  action: "approve" | "deny";
  runId?: string;
  workflowName?: string;
  nodeId?: string;
  iteration?: number;
  note?: string;
  decidedBy?: string;
  decision?: unknown;
};

export type NativeRunEvent = {
  runId: string;
  seq: number;
  timestampMs: number;
  type: string;
  payload: unknown | null;
};

export type NativeRunDiagnosis = {
  runId: string;
  status: string;
  summary: string;
  blockers: Array<Record<string, unknown>>;
  [key: string]: unknown;
};

export type NativeNodeDetail = {
  node: {
    runId: string;
    nodeId: string;
    iteration: number;
    state: string;
    lastAttempt: number | null;
    updatedAtMs: number | null;
    outputTable: string | null;
    label: string | null;
  };
  status: string;
  attemptsSummary: Record<string, number>;
  attempts: Array<Record<string, unknown>>;
  toolCalls: Array<Record<string, unknown>>;
  output: {
    validated: unknown | null;
    raw: unknown | null;
    source: string;
    cacheKey: string | null;
  };
  [key: string]: unknown;
};

export type NativeChatTranscript = {
  runId: string;
  attempts: Array<Record<string, unknown>>;
  messages: Array<{
    id: string;
    attemptKey: string;
    nodeId: string;
    iteration: number;
    attempt: number;
    role: "user" | "assistant" | "stderr";
    stream: "stdout" | "stderr" | null;
    timestampMs: number;
    text: string;
    source: "prompt" | "event" | "responseText";
  }>;
};

export class SmithersNativeAdapter {
  private readonly tools: Map<string, SemanticToolDefinition>;

  constructor(
    private readonly db: SmithersDb,
    private readonly cwd: string,
  ) {
    this.tools = new Map(
      createSemanticToolDefinitions({
        cwd: () => this.cwd,
        openDb: async () => ({
          adapter: this.db,
          dbPath: "embedded-runtime",
          cleanup: () => {},
        }),
      }).map((tool) => [tool.name, tool]),
    );
  }

  async listRuns(input: { limit?: number; status?: string }) {
    return await this.call<{ runs: Array<Record<string, unknown>> }>("list_runs", input);
  }

  async getRun(input: { runId: string }) {
    return await this.call<{ run: Record<string, unknown> }>("get_run", input);
  }

  async watchRun(input: { runId: string; intervalMs?: number; timeoutMs?: number }) {
    return await this.call<{
      runId: string;
      intervalMs: number;
      pollCount: number;
      reachedTerminal: boolean;
      timedOut: boolean;
      finalRun: Record<string, unknown>;
      snapshots: Array<{ observedAtMs: number; run: Record<string, unknown> }>;
    }>("watch_run", {
      runId: input.runId,
      intervalMs: input.intervalMs ?? 1_000,
      timeoutMs: input.timeoutMs ?? 30_000,
    });
  }

  async explainRun(input: { runId: string }) {
    return await this.call<{ diagnosis: NativeRunDiagnosis }>("explain_run", input);
  }

  async listPendingApprovals(input?: { runId?: string; workflowName?: string; nodeId?: string }) {
    return await this.call<{ approvals: Array<Record<string, unknown>> }>(
      "list_pending_approvals",
      input ?? {},
    );
  }

  async resolveApproval(input: NativeResolveApprovalInput) {
    return await this.call<{
      action: "approve" | "deny";
      approval: Record<string, unknown>;
      run: Record<string, unknown> | null;
    }>("resolve_approval", input);
  }

  async getNodeDetail(input: { runId: string; nodeId: string; iteration?: number }) {
    return await this.call<{ detail: NativeNodeDetail }>("get_node_detail", input);
  }

  async listArtifacts(input: { runId: string; nodeId?: string; includeRaw?: boolean }) {
    return await this.call<{ artifacts: Array<Record<string, unknown>> }>("list_artifacts", input);
  }

  async getChatTranscript(input: {
    runId: string;
    all?: boolean;
    includeStderr?: boolean;
    tail?: number;
  }) {
    return await this.call<NativeChatTranscript>("get_chat_transcript", input);
  }

  async getRunEvents(input: {
    runId: string;
    afterSeq?: number;
    limit?: number;
    nodeId?: string;
    types?: string[];
    sinceTimestampMs?: number;
  }) {
    return await this.call<{
      runId: string;
      events: NativeRunEvent[];
    }>("get_run_events", input);
  }

  async getDevToolsSnapshot(input: { runId: string; frameNo?: number }) {
    return (await getDevToolsSnapshotRoute({
      adapter: this.db as any,
      runId: input.runId,
      frameNo: input.frameNo,
    })) as Record<string, unknown>;
  }

  async streamDevTools(input: {
    runId: string;
    afterSeq?: number;
    timeoutMs?: number;
    maxEvents?: number;
    pollIntervalMs?: number;
  }) {
    const timeoutMs = Math.max(1, input.timeoutMs ?? 500);
    const maxEvents = Math.max(1, input.maxEvents ?? 25);
    const abortController = new AbortController();
    let endReason: "timeout" | "max-events" | "stream-closed" = "stream-closed";
    const timeoutId = setTimeout(() => {
      endReason = "timeout";
      abortController.abort();
    }, timeoutMs);
    const events: Array<Record<string, unknown>> = [];

    try {
      for await (const event of streamDevToolsRoute({
        adapter: this.db as any,
        runId: input.runId,
        fromSeq: input.afterSeq,
        pollIntervalMs: input.pollIntervalMs,
        signal: abortController.signal,
      })) {
        events.push(event as Record<string, unknown>);
        if (events.length >= maxEvents) {
          endReason = "max-events";
          break;
        }
      }
    } finally {
      clearTimeout(timeoutId);
      abortController.abort();
    }

    const lastEvent = events[events.length - 1] as
      | {
          kind?: string;
          delta?: { seq?: number };
          snapshot?: { seq?: number };
        }
      | undefined;
    const lastSeq =
      lastEvent?.kind === "delta"
        ? Number(lastEvent.delta?.seq ?? 0)
        : Number(lastEvent?.snapshot?.seq ?? 0);

    return {
      runId: input.runId,
      afterSeq: input.afterSeq ?? null,
      lastSeq: Number.isFinite(lastSeq) ? lastSeq : null,
      events,
      endReason,
      timeoutMs,
      maxEvents,
    };
  }

  private async call<T>(name: string, input: Record<string, unknown>): Promise<T> {
    const tool = this.tools.get(name);
    if (!tool) {
      throw new Error(`Smithers semantic tool not available: ${name}`);
    }

    const result = (await tool.handler(stripUndefined(input))) as SemanticToolCallResult;
    if (result.structuredContent.ok) {
      return result.structuredContent.data as T;
    }

    throw new Error(formatSemanticToolError(name, result));
  }
}

function formatSemanticToolError(name: string, result: SemanticToolCallResult): string {
  const error = result.structuredContent.error;
  if (!error) {
    return `Smithers semantic tool ${name} failed.`;
  }
  const details =
    error.details && Object.keys(error.details).length > 0
      ? ` ${JSON.stringify(error.details)}`
      : "";
  return `${error.code}: ${error.message}${details}`;
}

function stripUndefined(input: Record<string, unknown>): Record<string, unknown> {
  return Object.fromEntries(Object.entries(input).filter(([, value]) => value !== undefined));
}
