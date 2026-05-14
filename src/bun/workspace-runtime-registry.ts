import { basename, join } from "node:path";
import { randomUUID } from "node:crypto";
import type {
  AppLogUpdateMessage,
  SurfaceSyncMessage,
  WorkspaceInfoResponse,
  WorkspaceSyncMessage,
  WorkspaceTabInfo,
} from "../shared/workspace-contract";
import { createAppLogger, type BridgeLogLevel } from "./app-logger";
import { createAppLogStore, type AppLogStore } from "./app-log-store";
import { createSessionAgentSettingsStore } from "./session-agent-settings";
import {
  getSvvyAgentDir,
  WorkspaceSessionCatalog,
  type TitleGenerationLogEvent,
} from "./session-catalog";
import { canonicalizeWorkspaceCwd } from "./workspace-context";
import { WorkspacePathIndex } from "./workspace-path-index";

type WorkspaceRuntimeRegistryOptions = {
  initialCwd: string;
  openInitialWorkspace?: boolean;
  agentDir?: string;
  forwardBridgeLog?: (
    level: BridgeLogLevel,
    message: string,
    source: string,
    details?: Record<string, unknown>,
    error?: unknown,
  ) => void;
  onAppLogUpdate?: (workspaceId: string, payload: AppLogUpdateMessage) => void;
  onSurfaceSync?: (workspaceId: string, payload: SurfaceSyncMessage) => void;
  onWorkspaceSync?: (workspaceId: string, payload: WorkspaceSyncMessage) => void;
};

export type WorkspaceRuntime = {
  workspaceId: string;
  cwd: string;
  label: string;
  openedAt: string;
  catalog: WorkspaceSessionCatalog;
  pathIndex: WorkspacePathIndex;
  agentSettingsStore: ReturnType<typeof createSessionAgentSettingsStore>;
  appLogStore: AppLogStore;
  appLog: ReturnType<typeof createAppLogger>;
  getInfo: () => WorkspaceInfoResponse;
  dispose: () => Promise<void>;
};

type RuntimeRecord = WorkspaceRuntime & {
  unsubscribeAppLog: () => void;
};

export class WorkspaceRuntimeRegistry {
  private readonly runtimes = new Map<string, RuntimeRecord>();
  private readonly agentDir: string;
  private activeWorkspaceId: string | null = null;

  constructor(private readonly options: WorkspaceRuntimeRegistryOptions) {
    this.agentDir = options.agentDir ?? getSvvyAgentDir();
    if (options.openInitialWorkspace) {
      this.activeWorkspaceId = this.openWorkspace(options.initialCwd).workspaceId;
    }
  }

  openWorkspace(cwd: string): WorkspaceRuntime {
    const workspaceCwd = canonicalizeWorkspaceCwd(cwd);
    const workspaceId = `${workspaceCwd}#${randomUUID()}`;
    const runtime = this.createRuntime(workspaceId, workspaceCwd);
    this.runtimes.set(workspaceId, runtime);
    this.activeWorkspaceId = workspaceId;
    return runtime;
  }

  getRuntime(workspaceId: string): WorkspaceRuntime {
    const runtime = this.runtimes.get(workspaceId);
    if (!runtime) {
      throw new Error(`Workspace is not open: ${workspaceId}`);
    }
    return runtime;
  }

  getActiveRuntime(): WorkspaceRuntime {
    if (!this.activeWorkspaceId) {
      throw new Error("No workspace is active.");
    }
    return this.getRuntime(this.activeWorkspaceId);
  }

  getActiveRuntimeOrNull(): WorkspaceRuntime | null {
    return this.activeWorkspaceId ? this.getRuntime(this.activeWorkspaceId) : null;
  }

  getActiveWorkspaceId(): string | null {
    return this.activeWorkspaceId;
  }

  getInitialCwd(): string {
    return this.options.initialCwd;
  }

  setActiveWorkspace(workspaceId: string): WorkspaceRuntime {
    const runtime = this.getRuntime(workspaceId);
    this.activeWorkspaceId = runtime.workspaceId;
    return runtime;
  }

  listOpenWorkspaces(): WorkspaceTabInfo[] {
    return Array.from(this.runtimes.values()).map((runtime) => ({
      ...runtime.getInfo(),
      openedAt: runtime.openedAt,
    }));
  }

  async closeWorkspace(workspaceId: string): Promise<boolean> {
    const runtime = this.runtimes.get(workspaceId);
    if (!runtime) {
      return false;
    }

    this.runtimes.delete(workspaceId);
    await runtime.dispose();

    if (this.activeWorkspaceId === workspaceId) {
      const next = this.runtimes.keys().next().value as string | undefined;
      this.activeWorkspaceId = next ?? null;
    }
    return true;
  }

  private createRuntime(workspaceId: string, cwd: string): RuntimeRecord {
    const label = basename(cwd) || "workspace";
    const runtimeDir = join(
      this.agentDir,
      "workspace-runtimes",
      sanitizeWorkspaceRuntimeId(workspaceId),
    );
    const catalog = new WorkspaceSessionCatalog(
      cwd,
      this.agentDir,
      join(runtimeDir, "sessions"),
      join(runtimeDir, "namer"),
      workspaceId,
    );
    const pathIndex = new WorkspacePathIndex(cwd);
    const agentSettingsStore = createSessionAgentSettingsStore({
      cwd,
      agentDir: this.agentDir,
    });
    const appLogStore = createAppLogStore({
      databasePath: join(runtimeDir, "app-logs-v1.sqlite"),
    });
    const appLog = createAppLogger({
      store: appLogStore,
      forwardBridgeLog: (level, message, source, details, error) => {
        this.options.forwardBridgeLog?.(
          level,
          message,
          source,
          { ...(details ?? {}), workspaceId },
          error,
        );
      },
    });
    const unsubscribeAppLog = appLog.subscribe((entries, summary) => {
      this.options.onAppLogUpdate?.(workspaceId, {
        entries,
        summary,
      });
    });

    catalog.setWorkspaceSyncListener((payload) => {
      this.options.onWorkspaceSync?.(workspaceId, {
        ...payload,
        workspaceId,
      });
    });
    catalog.setSurfaceSyncListener((payload) => {
      this.options.onSurfaceSync?.(workspaceId, {
        ...payload,
        workspaceId,
      });
    });
    catalog.setTitleGenerationLogListener((event) => {
      recordTitleGenerationLog(appLog, event);
    });

    const runtime: RuntimeRecord = {
      workspaceId,
      cwd,
      label,
      openedAt: new Date().toISOString(),
      catalog,
      pathIndex,
      agentSettingsStore,
      appLogStore,
      appLog,
      unsubscribeAppLog,
      getInfo: () => ({
        workspaceId,
        cwd,
        workspaceLabel: label,
      }),
      dispose: async () => {
        unsubscribeAppLog();
        catalog.setWorkspaceSyncListener(null);
        catalog.setSurfaceSyncListener(null);
        catalog.setTitleGenerationLogListener(null);
        await catalog.dispose();
        appLogStore.close();
      },
    };
    return runtime;
  }
}

function sanitizeWorkspaceRuntimeId(workspaceId: string): string {
  return workspaceId.replace(/^[/\\]/, "").replace(/[/\\:#]/g, "-");
}

function recordTitleGenerationLog(
  appLog: ReturnType<typeof createAppLogger>,
  event: TitleGenerationLogEvent,
): void {
  const message = formatTitleGenerationLogMessage(event);
  const details = {
    status: event.status,
    ...(event.status === "completed" ? { title: event.title } : {}),
    workspaceSessionId: event.sessionId,
  };
  if (event.level === "warning") {
    appLog.warning("session.title", message, {
      ...details,
      failureReason: event.error,
    });
    return;
  }
  appLog.info("session.title", message, details);
}

function formatTitleGenerationLogMessage(event: TitleGenerationLogEvent): string {
  switch (event.status) {
    case "queued":
      return "Session title generation queued.";
    case "started":
      return "Session title generation started.";
    case "completed":
      return "Session title generation completed.";
    case "failed":
      return "Session title generation failed.";
  }
}
