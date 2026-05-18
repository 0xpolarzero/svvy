import {
  ApplicationMenu,
  BrowserWindow,
  Updater,
  Utils,
  defineElectrobunRPC,
} from "electrobun/bun";
import { getModel, getModels, getProviders } from "@mariozechner/pi-ai";
import { randomUUID } from "node:crypto";
import { spawn, spawnSync } from "node:child_process";
import {
  copyFileSync,
  existsSync,
  mkdirSync,
  readFileSync,
  statSync,
  writeFileSync,
} from "node:fs";
import { basename, extname, join, relative, resolve, sep } from "node:path";
import type {
  AuthStateResponse,
  ChatRPCSchema,
  ComposerAttachment,
  ComposerMentionKind,
  ImportComposerAttachmentInput,
  ProviderAuthInfo,
  SendPromptRequest,
  SendPromptResponse,
  SwitchWorkspaceBranchResponse,
} from "../shared/workspace-contract";
import {
  getShortcut,
  getShortcutAccelerator,
  isAppMenuAction,
  type AppMenuAction,
} from "../shared/shortcut-registry";
import {
  DEFAULT_AGENT_SETTINGS,
  type AgentDefaults,
  type SessionMode,
} from "../shared/agent-settings";
import {
  getProviderEnvVar,
  removeCredential,
  resolveApiKey,
  resolveAuthState,
  setApiKey as storeApiKey,
} from "./auth-store";
import { refreshIfNeeded, startOAuthLogin, supportsOAuth } from "./oauth-login";
import { DEFAULT_SYSTEM_PROMPT } from "./default-system-prompt";
import { getSvvyAgentDir, type SessionDefaults } from "./session-catalog";
import {
  deleteSavedWorkflowLibraryPath,
  readSavedWorkflowLibraryReadModel,
} from "./smithers-runtime/workflow-library";
import { resolveWorkspaceCwd } from "./workspace-context";
import { positionNativeTrafficLights } from "./native-window-controls";
import { WorkspaceRuntimeRegistry, type WorkspaceRuntime } from "./workspace-runtime-registry";
import { createAppWorkspaceTabsStore } from "./app-workspace-tabs-store";
import { createAppWorkspaceUiRestoreStore } from "./app-workspace-ui-restore-store";
import { getWorkspaceRuntimeForRequest, stripWorkspaceId } from "./workspace-rpc-routing";

const DEV_SERVER_PORT = 5173;
const DEV_SERVER_URL = `http://localhost:${DEV_SERVER_PORT}`;
const DEV_SERVER_WAIT_TIMEOUT_MS = 15_000;
const DEV_SERVER_POLL_INTERVAL_MS = 250;
const DEFAULT_RPC_TIMEOUT_MS = 120000;
const ENV_FILES = [".env.local", ".env"];
const PREFERRED_PROVIDERS = ["zai", "openai", "anthropic", "google"];
const PREFERRED_MODEL_FRAGMENTS = [
  "glm-5-turbo",
  "glm-4.7-flashx",
  "glm-4.7-flash",
  "gpt-5.4-mini",
  "gpt-5.4",
  "gpt-5",
  "gpt-4o",
  "claude-sonnet",
  "gemini-2.5",
  "glm-4.7",
  "glm-4.5",
];
let resolvedDefaults: AgentDefaults | null = null;
let mainWindow: BrowserWindow | null = null;
const startupWorkspaceCwd = resolveWorkspaceCwd();
const appWorkspaceTabsStore = createAppWorkspaceTabsStore({
  agentDir: getSvvyAgentDir(),
});
const appWorkspaceUiRestoreStore = createAppWorkspaceUiRestoreStore({
  agentDir: getSvvyAgentDir(),
});

const NATIVE_TRAFFIC_LIGHT_POSITION = {
  leading: 18,
  top: 13,
} as const;

function appMenuItem(action: AppMenuAction): {
  label: string;
  action: string;
  accelerator: string;
} {
  return {
    label: getShortcut(action).label,
    action,
    accelerator: getShortcutAccelerator(action) ?? "",
  };
}

function loadEnvFile(filePath: string): void {
  if (!existsSync(filePath)) return;

  try {
    const content = readFileSync(filePath, "utf8");
    for (const rawLine of content.split(/\r?\n/)) {
      const line = rawLine.trim();
      if (!line || line.startsWith("#")) continue;

      const equalsIndex = line.indexOf("=");
      if (equalsIndex < 0) continue;

      const key = line.slice(0, equalsIndex).trim();
      if (!key || process.env[key] !== undefined) continue;

      let value = line.slice(equalsIndex + 1).trim();
      if (
        (value.startsWith('"') && value.endsWith('"')) ||
        (value.startsWith("'") && value.endsWith("'"))
      ) {
        value = value.slice(1, -1);
      }

      if (value) process.env[key] = value;
    }
  } catch {
    // Ignore malformed or unreadable env files.
  }
}

function loadRuntimeEnv(cwd: string): void {
  for (const file of ENV_FILES) {
    loadEnvFile(join(cwd, file));
  }
}

function getRpcRequestTimeoutMs(): number {
  const source =
    process.env.ELECTROBUN_RPC_TIMEOUT_MS ??
    process.env.ELECTROBUN_RPC_REQUEST_TIMEOUT_MS ??
    process.env.VITE_ELECTROBUN_RPC_TIMEOUT_MS;

  const parsed = Number(source);
  if (!Number.isFinite(parsed) || parsed <= 0) return DEFAULT_RPC_TIMEOUT_MS;

  return Math.trunc(parsed);
}

type DevBrowserToolsRecorder = {
  recordError: (
    kind: "app" | "rpc",
    message: string,
    source: string,
    details?: Record<string, unknown>,
    error?: unknown,
  ) => void;
  recordEvent: (eventName: string, payload?: Record<string, unknown>) => void;
  recordLog: (
    level: "debug" | "info" | "warn" | "error",
    message: string,
    source: string,
    context?: Record<string, unknown>,
  ) => void;
};

type DevServerMode = "auto" | "wait";

function getDevServerMode(): DevServerMode {
  return process.env.SVVY_VITE_DEV_SERVER === "wait" ? "wait" : "auto";
}

async function isDevServerReady(): Promise<boolean> {
  try {
    const response = await fetch(DEV_SERVER_URL, {
      method: "HEAD",
    });
    return response.ok;
  } catch {
    return false;
  }
}

async function waitForDevServer(timeoutMs: number): Promise<boolean> {
  const deadline = Date.now() + timeoutMs;

  while (Date.now() < deadline) {
    if (await isDevServerReady()) {
      return true;
    }
    await Bun.sleep(DEV_SERVER_POLL_INTERVAL_MS);
  }

  return false;
}

function getApiKeyMissingError(provider: string): string {
  const envVar = getProviderEnvVar(provider);
  if (!envVar) {
    return `No API key configured for provider "${provider}".`;
  }
  return `Missing ${envVar} for provider "${provider}". Add one in Provider settings.`;
}

async function getMainViewUrl(channel: string): Promise<string> {
  if (channel === "dev") {
    const mode = getDevServerMode();
    const ready =
      mode === "wait"
        ? await waitForDevServer(DEV_SERVER_WAIT_TIMEOUT_MS)
        : await isDevServerReady();

    if (ready) {
      console.log(`HMR enabled: using Vite dev server at ${DEV_SERVER_URL}`);
      return DEV_SERVER_URL;
    }

    console.log("Vite dev server not running. Run `bun run dev`.");
  }
  return "views://mainview/index.html";
}

function resolveSendDefaults(runtime: WorkspaceRuntime, request: SendPromptRequest): AgentDefaults {
  const defaults = getDefaultAgentSettings(runtime);
  return {
    provider: request.provider || defaults.provider,
    model: request.model || defaults.model,
    reasoningEffort: request.reasoningEffort || defaults.reasoningEffort,
  };
}

function getDefaultAgentSettings(runtime?: WorkspaceRuntime): AgentDefaults {
  if (runtime) {
    const savedDefault = runtime.agentSettingsStore.getState().sessionAgents.defaultSession;
    if (savedDefault.provider && savedDefault.model) {
      return {
        provider: savedDefault.provider,
        model: savedDefault.model,
        reasoningEffort: savedDefault.reasoningEffort,
      };
    }
  }
  if (resolvedDefaults) {
    return resolvedDefaults;
  }

  const providers = getProviders();
  const preferredProviders = PREFERRED_PROVIDERS.filter(
    (provider): provider is (typeof providers)[number] =>
      providers.includes(provider as (typeof providers)[number]),
  );
  const orderedProviders = [
    ...preferredProviders,
    ...providers.filter((provider) => !PREFERRED_PROVIDERS.includes(provider)),
  ];

  for (const provider of orderedProviders) {
    const models = getModels(provider);
    if (models.length === 0) continue;

    const preferredModel =
      PREFERRED_MODEL_FRAGMENTS.flatMap((fragment) =>
        models.filter((model) => model.id.includes(fragment)),
      )[0] ?? models[0];
    if (!preferredModel) continue;

    resolvedDefaults = {
      provider,
      model: preferredModel.id,
      reasoningEffort: DEFAULT_AGENT_SETTINGS.reasoningEffort,
    };
    return resolvedDefaults;
  }

  resolvedDefaults = DEFAULT_AGENT_SETTINGS;
  return resolvedDefaults;
}

function getSessionDefaults(
  runtime: WorkspaceRuntime,
  mode: SessionMode = "orchestrator",
): SessionDefaults {
  const agentSettings =
    runtime.agentSettingsStore.getState().sessionAgents[
      mode === "dumb" ? "dumbOrchestrator" : "defaultSession"
    ];
  const defaults =
    agentSettings.provider && agentSettings.model
      ? agentSettings
      : getDefaultAgentSettings(runtime);
  return {
    model: defaults.model,
    provider: defaults.provider,
    systemPrompt: runtime.catalog.buildOrchestratorSystemPrompt(agentSettings),
    thinkingLevel: defaults.reasoningEffort,
    sessionMode: mode,
    sessionAgentKey: mode === "dumb" ? "dumbOrchestrator" : "defaultSession",
  };
}

function createAuthState(provider: string): AuthStateResponse {
  const state = resolveAuthState(provider);
  if (!state.connected) {
    return {
      connected: false,
      message: getApiKeyMissingError(provider),
    };
  }

  return {
    connected: true,
    accountId: `${provider}-${state.keyType}`,
  };
}

function getWorkspaceBranch(cwd: string): string | undefined {
  const result = spawnSync("git", ["rev-parse", "--abbrev-ref", "HEAD"], {
    cwd,
    encoding: "utf8",
    stdio: ["ignore", "pipe", "ignore"],
  });
  if (result.status !== 0) {
    return undefined;
  }

  const branch = result.stdout.trim();
  return branch && branch !== "HEAD" ? branch : undefined;
}

function getWorkspaceBranches(cwd: string): string[] {
  const result = spawnSync("git", ["for-each-ref", "--format=%(refname:short)", "refs/heads"], {
    cwd,
    encoding: "utf8",
    stdio: ["ignore", "pipe", "ignore"],
  });
  if (result.status !== 0) {
    return [];
  }

  return result.stdout
    .split(/\r?\n/)
    .map((branch) => branch.trim())
    .filter(Boolean);
}

function switchWorkspaceBranch(
  runtime: WorkspaceRuntime,
  branch: string,
): SwitchWorkspaceBranchResponse {
  const nextBranch = branch.trim();
  const branches = getWorkspaceBranches(runtime.cwd);
  if (!nextBranch || !branches.includes(nextBranch)) {
    return {
      ok: false,
      workspace: addWorkspaceBranch(runtime.getInfo()),
      error: "Branch is not available in this workspace.",
    };
  }

  if (getWorkspaceBranch(runtime.cwd) === nextBranch) {
    return { ok: true, workspace: addWorkspaceBranch(runtime.getInfo()) };
  }

  const result = spawnSync("git", ["switch", nextBranch], {
    cwd: runtime.cwd,
    encoding: "utf8",
    stdio: ["ignore", "pipe", "pipe"],
  });
  if (result.status !== 0) {
    const message = (result.stderr || result.stdout).trim() || "Unable to switch branch.";
    return {
      ok: false,
      workspace: addWorkspaceBranch(runtime.getInfo()),
      error: message,
    };
  }

  runtime.pathIndex.refresh();
  runtime.appLog.info("workspace", "Workspace branch switched.", {
    workspaceId: runtime.workspaceId,
    branch: nextBranch,
  });
  recordDevBrowserToolsEvent("workspace.branch-switched", {
    workspaceId: runtime.workspaceId,
    branch: nextBranch,
  });
  return { ok: true, workspace: addWorkspaceBranch(runtime.getInfo()) };
}

function resolveSafeWorkspacePath(
  runtime: WorkspaceRuntime,
  workspaceRelativePath: string,
): string | null {
  const cwd = runtime.cwd;
  const normalizedRelativePath = workspaceRelativePath.trim().replace(/\\/g, "/").replace(/^@/, "");
  if (
    !normalizedRelativePath ||
    normalizedRelativePath.startsWith("/") ||
    normalizedRelativePath.includes("\0") ||
    normalizedRelativePath.split("/").includes("..")
  ) {
    return null;
  }

  const absolutePath = resolve(cwd, normalizedRelativePath);
  const root = resolve(cwd);
  if (absolutePath !== root && !absolutePath.startsWith(`${root}${sep}`)) return null;
  return absolutePath;
}

function getWorkspacePathKind(absolutePath: string): ComposerMentionKind | "missing" {
  try {
    const stats = statSync(absolutePath);
    return stats.isDirectory() ? "folder" : "file";
  } catch {
    return "missing";
  }
}

function sanitizeAttachmentName(name: string): string {
  const sanitized = name.replace(/[^A-Za-z0-9._-]+/g, "-").replace(/^-+|-+$/g, "");
  return sanitized || "attachment";
}

function imageMimeTypeFromPath(path: string): string | null {
  const extension = extname(path).toLowerCase();
  if (extension === ".png") return "image/png";
  if (extension === ".jpg" || extension === ".jpeg") return "image/jpeg";
  if (extension === ".gif") return "image/gif";
  if (extension === ".webp") return "image/webp";
  return null;
}

function importedAttachmentPath(
  cwd: string,
  name: string,
): { absolutePath: string; workspaceRelativePath: string } {
  const attachmentId = randomUUID();
  const relativePath = join(
    ".svvy",
    "attachments",
    "user-input",
    `${attachmentId}-${sanitizeAttachmentName(name)}`,
  );
  const absolutePath = resolve(cwd, relativePath);
  mkdirSync(join(cwd, ".svvy", "attachments", "user-input"), { recursive: true });
  return { absolutePath, workspaceRelativePath: relativePath.split(sep).join("/") };
}

function createComposerAttachmentFromPath(
  cwd: string,
  selectedPath: string,
): ComposerAttachment | null {
  const absolutePath = resolve(selectedPath);
  const kind = getWorkspacePathKind(absolutePath);
  if (kind === "missing") return null;

  const workspaceRelativePath = relative(cwd, absolutePath);
  const isWorkspacePath =
    workspaceRelativePath !== "" &&
    !workspaceRelativePath.startsWith("..") &&
    !workspaceRelativePath.includes(`..${sep}`) &&
    resolve(cwd, workspaceRelativePath) === absolutePath;
  const normalizedPath = (isWorkspacePath ? workspaceRelativePath : absolutePath)
    .split(sep)
    .join("/");
  const stats = statSync(absolutePath);
  const mimeType = kind === "file" ? imageMimeTypeFromPath(absolutePath) : null;

  if (kind === "file" && !isWorkspacePath) {
    const imported = importedAttachmentPath(cwd, basename(absolutePath));
    copyFileSync(absolutePath, imported.absolutePath);
    const importedMimeType = mimeType ?? imageMimeTypeFromPath(imported.absolutePath);
    return {
      id: `attachment:${imported.workspaceRelativePath}`,
      kind: importedMimeType?.startsWith("image/") ? "image" : "file",
      name: basename(absolutePath),
      path: imported.workspaceRelativePath,
      workspaceRelativePath: imported.workspaceRelativePath,
      mimeType: importedMimeType ?? undefined,
      sizeBytes: stats.size,
      dataBase64: importedMimeType?.startsWith("image/")
        ? readFileSync(imported.absolutePath).toString("base64")
        : undefined,
    };
  }

  return {
    id: `${kind}:${normalizedPath}`,
    kind: mimeType?.startsWith("image/") ? "image" : kind,
    name: basename(absolutePath),
    path: normalizedPath,
    workspaceRelativePath: isWorkspacePath ? workspaceRelativePath.split(sep).join("/") : undefined,
    mimeType: mimeType ?? undefined,
    sizeBytes: kind === "file" ? stats.size : undefined,
    dataBase64: mimeType?.startsWith("image/")
      ? readFileSync(absolutePath).toString("base64")
      : undefined,
  };
}

function createImportedComposerAttachment(
  cwd: string,
  input: ImportComposerAttachmentInput,
): ComposerAttachment {
  const name = sanitizeAttachmentName(input.name || "attachment");
  const imported = importedAttachmentPath(cwd, name);
  const bytes = Buffer.from(input.dataBase64, "base64");
  writeFileSync(imported.absolutePath, bytes);
  const mimeType = input.mimeType || imageMimeTypeFromPath(name) || "application/octet-stream";
  return {
    id: `attachment:${imported.workspaceRelativePath}`,
    kind: mimeType.startsWith("image/") ? "image" : "file",
    name,
    path: imported.workspaceRelativePath,
    workspaceRelativePath: imported.workspaceRelativePath,
    mimeType,
    sizeBytes: bytes.byteLength,
    dataBase64: mimeType.startsWith("image/") ? input.dataBase64 : undefined,
  };
}

function openPathInPreferredEditor(
  runtime: WorkspaceRuntime,
  path: string,
): { opened: boolean; editor: string } {
  const preferences = runtime.agentSettingsStore.getState().appPreferences;
  const editor = preferences.preferredExternalEditor;
  if (editor === "system") {
    return { opened: Utils.openPath(path), editor };
  }

  const appNameByEditor = {
    code: "Visual Studio Code",
    cursor: "Cursor",
    zed: "Zed",
    sublime: "Sublime Text",
  } satisfies Record<Exclude<typeof editor, "system" | "custom">, string>;
  if (editor !== "custom") {
    try {
      const child = spawn("/usr/bin/open", ["-a", appNameByEditor[editor], path], {
        cwd: runtime.cwd,
        detached: true,
        stdio: "ignore",
      });
      child.unref();
      return { opened: true, editor };
    } catch (error) {
      runtime.appLog.warning("external-editor", "External editor app launch failed.", {
        editor,
        path,
        message: error instanceof Error ? error.message : String(error),
      });
      return { opened: false, editor };
    }
  }

  const configuredCommand = preferences.customExternalEditorCommand;
  const [command, ...baseArgs] = configuredCommand.split(/\s+/).filter(Boolean);
  if (!command) {
    runtime.appLog.warning("external-editor", "Custom external editor command is empty.", { path });
    return { opened: false, editor };
  }

  try {
    const child = spawn(command, [...baseArgs, path], {
      cwd: runtime.cwd,
      detached: true,
      stdio: "ignore",
    });
    child.unref();
    return { opened: true, editor };
  } catch (error) {
    runtime.appLog.warning("external-editor", "Custom external editor command failed.", {
      command,
      path,
      message: error instanceof Error ? error.message : String(error),
    });
    return { opened: false, editor };
  }
}

function listProviderAuthSummaries(): ProviderAuthInfo[] {
  const providerIds = [...getProviders(), "tinyfish", "firecrawl"];
  return providerIds.map((provider) => {
    const state = resolveAuthState(provider);
    return {
      provider,
      hasKey: state.connected,
      keyType: state.keyType,
      supportsOAuth: supportsOAuth(provider),
    };
  });
}

let devBrowserToolsRecorder: DevBrowserToolsRecorder = {
  recordError: () => {},
  recordEvent: () => {},
  recordLog: () => {},
};

const recordDevBrowserToolsEvent: DevBrowserToolsRecorder["recordEvent"] = (...args) =>
  devBrowserToolsRecorder.recordEvent(...args);
const recordDevBrowserToolsLog: DevBrowserToolsRecorder["recordLog"] = (...args) =>
  devBrowserToolsRecorder.recordLog(...args);
const recordDevBrowserToolsError: DevBrowserToolsRecorder["recordError"] = (...args) =>
  devBrowserToolsRecorder.recordError(...args);

const workspaceRuntimeRegistry = new WorkspaceRuntimeRegistry({
  initialCwd: startupWorkspaceCwd,
  openInitialWorkspace: !!process.env.SVVY_WORKSPACE_CWD,
  forwardBridgeLog: (level, message, source, details, error) => {
    if (level === "error") {
      recordDevBrowserToolsError("app", message, source, details, error);
      return;
    }
    recordDevBrowserToolsLog(level === "warning" ? "warn" : level, message, source, details);
  },
  onAppLogUpdate: (workspaceId, payload) => {
    rpc.send.sendAppLogUpdate({
      ...payload,
      workspaceId,
    });
  },
  onWorkspaceSync: (_workspaceId, payload) => {
    rpc.send.sendWorkspaceSync(payload);
  },
  onSurfaceSync: (_workspaceId, payload) => {
    rpc.send.sendSurfaceSync(payload);
  },
});

function recordAppRuntimeLog(
  level: "info" | "warning",
  message: string,
  source: string,
  details?: Record<string, unknown>,
): void {
  const runtime = workspaceRuntimeRegistry.getActiveRuntimeOrNull();
  if (!runtime) {
    recordDevBrowserToolsLog(level === "warning" ? "warn" : level, message, source, details);
    return;
  }
  runtime.appLog[level](mapRuntimeLogSource(source), message, details);
}

function recordAppRuntimeError(
  kind: string,
  message: string,
  source: string,
  details?: Record<string, unknown>,
  error?: unknown,
): void {
  const runtime = workspaceRuntimeRegistry.getActiveRuntimeOrNull();
  if (!runtime) {
    recordDevBrowserToolsError(kind === "rpc" ? "rpc" : "app", message, source, details, error);
    return;
  }
  runtime.appLog.error(mapRuntimeLogSource(source, kind), message, error, details);
}

function mapRuntimeLogSource(source: string, kind?: string) {
  if (source.includes("auth") || source.includes("oauth")) return "auth.provider" as const;
  if (source.includes("sendPrompt")) return "prompt" as const;
  if (source.includes("session")) return "session" as const;
  if (source.includes("surface")) return "surface" as const;
  if (source.includes("workflow")) return "workflow.library" as const;
  if (source.includes("editor")) return "external-editor" as const;
  if (source.includes("dev-browser-tools")) return "app.bridge" as const;
  if (source.includes("settings")) return "settings" as const;
  if (kind === "rpc" || source.includes("rpc")) return "app.rpc" as const;
  return "app.lifecycle" as const;
}

function getWorkspaceRuntime(input: Parameters<typeof getWorkspaceRuntimeForRequest>[1]) {
  return getWorkspaceRuntimeForRequest(workspaceRuntimeRegistry, input);
}

function addWorkspaceBranch<T extends { cwd: string }>(info: T): T & { branch?: string } {
  return {
    ...info,
    branch: getWorkspaceBranch(info.cwd),
  };
}

const rpc = defineElectrobunRPC<ChatRPCSchema, "bun">("bun", {
  maxRequestTime: getRpcRequestTimeoutMs(),
  handlers: {
    requests: {
      getDefaults: async () => {
        return getDefaultAgentSettings();
      },
      getAgentSettings: async (input) => {
        return getWorkspaceRuntime(input).agentSettingsStore.getState();
      },
      getAppPreferences: async (input) => {
        return getWorkspaceRuntime(input).agentSettingsStore.getState().appPreferences;
      },
      getPromptLibrary: async (input) => {
        return getWorkspaceRuntime(input).catalog.getPromptLibraryState();
      },
      getPromptLibraryDefaults: async (input) => {
        return getWorkspaceRuntime(input).catalog.getDefaultPromptLibraryState();
      },
      updatePromptLibrary: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const { state } = input;
        const next = runtime.catalog.updatePromptLibraryState(state);
        runtime.appLog.info("settings", "Prompt library updated.", {
          revision: next.revision,
        });
        return next;
      },
      resetPromptLibrary: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const next = runtime.catalog.resetPromptLibraryState();
        runtime.appLog.info("settings", "Prompt library reset.", {
          revision: next.revision,
        });
        return next;
      },
      listPromptLibrarySnapshots: async (input) => {
        return getWorkspaceRuntime(input).catalog.listPromptLibrarySnapshots();
      },
      createPromptLibrarySnapshot: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const { name } = input;
        const snapshot = runtime.catalog.createPromptLibrarySnapshot(name);
        runtime.appLog.info("settings", "Prompt library snapshot created.", {
          snapshotId: snapshot.id,
          name: snapshot.name,
        });
        return snapshot;
      },
      renamePromptLibrarySnapshot: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const { snapshotId, name } = input;
        const snapshot = runtime.catalog.renamePromptLibrarySnapshot(snapshotId, name);
        runtime.appLog.info("settings", "Prompt library snapshot renamed.", {
          snapshotId: snapshot.id,
          name: snapshot.name,
        });
        return snapshot;
      },
      restorePromptLibrarySnapshot: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const { snapshotId } = input;
        const next = runtime.catalog.restorePromptLibrarySnapshot(snapshotId);
        runtime.appLog.info("settings", "Prompt library snapshot loaded.", {
          snapshotId,
          revision: next.revision,
        });
        return next;
      },
      getPromptLibraryGeneratedEntries: async (input) => {
        return getWorkspaceRuntime(input).catalog.getPromptLibraryGeneratedEntries();
      },
      getPromptLibraryExternalSources: async (input) => {
        return getWorkspaceRuntime(input).catalog.getPromptLibraryExternalSources();
      },
      updateSessionAgentDefault: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const { key, settings } = input;
        resolvedDefaults = null;
        runtime.appLog.info("settings", "Session agent defaults updated.", { key });
        return runtime.agentSettingsStore.setSessionAgentDefault(key, settings);
      },
      updateWorkflowAgent: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const { key, settings } = input;
        runtime.appLog.info("settings", "Workflow agent settings updated.", { key });
        return runtime.agentSettingsStore.setWorkflowAgent(key, settings);
      },
      updateAppPreferences: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const preferences = stripWorkspaceId(input);
        runtime.appLog.info("settings", "App preferences updated.", {
          appAppearance: preferences.appAppearance,
          preferredExternalEditor: preferences.preferredExternalEditor,
        });
        return runtime.agentSettingsStore.setAppPreferences(preferences);
      },
      ensureWorkflowAgentsComponent: async (input) => {
        return {
          path: getWorkspaceRuntime(input).agentSettingsStore.ensureWorkflowAgentsComponent(),
        };
      },
      getProviderAuthState: async ({
        providerId,
      }: {
        providerId?: string;
      }): Promise<AuthStateResponse> => {
        const defaults = getDefaultAgentSettings();
        return createAuthState(providerId || defaults.provider);
      },
      openWorkspace: async ({ cwd }) => {
        const selectedCwd =
          cwd ??
          (
            await Utils.openFileDialog({
              startingFolder:
                workspaceRuntimeRegistry.getActiveRuntimeOrNull()?.cwd ??
                workspaceRuntimeRegistry.getInitialCwd(),
              allowedFileTypes: "*",
              canChooseFiles: false,
              canChooseDirectory: true,
              allowsMultipleSelection: false,
            })
          )[0];
        if (!selectedCwd) return { workspace: null };
        const runtime = workspaceRuntimeRegistry.acquireWorkspace(selectedCwd);
        runtime.appLog.info("workspace", "Workspace opened.", { workspaceId: runtime.workspaceId });
        recordDevBrowserToolsEvent("workspace.opened", { workspaceId: runtime.workspaceId });
        return { workspace: addWorkspaceBranch(runtime.getInfo()) };
      },
      getOpenWorkspaces: async () => {
        return workspaceRuntimeRegistry.listOpenWorkspaces().map(addWorkspaceBranch);
      },
      getDefaultWorkspace: async () => {
        return addWorkspaceBranch(workspaceRuntimeRegistry.getDefaultWorkspace().getInfo());
      },
      getAppWorkspaceTabs: async () => {
        return appWorkspaceTabsStore.getState();
      },
      setAppWorkspaceTabs: async (state) => {
        appWorkspaceTabsStore.setState(state);
        return { ok: true };
      },
      getWorkspaceUiRestore: async ({ workspaceId }) => {
        const runtime = getWorkspaceRuntime({ workspaceId });
        if (runtime.kind === "default") {
          return null;
        }
        return appWorkspaceUiRestoreStore.getState(workspaceId);
      },
      setWorkspaceUiRestore: async ({ workspaceId, state }) => {
        const runtime = getWorkspaceRuntime({ workspaceId });
        if (runtime.kind !== "default") {
          appWorkspaceUiRestoreStore.setState(workspaceId, state);
        }
        return { ok: true };
      },
      setActiveWorkspace: async ({ workspaceId }) => {
        const runtime = workspaceRuntimeRegistry.setActiveWorkspace(workspaceId);
        runtime.appLog.info("workspace", "Active workspace changed.", {
          workspaceId: runtime.workspaceId,
        });
        recordDevBrowserToolsEvent("workspace.activated", { workspaceId: runtime.workspaceId });
        return { ok: true };
      },
      closeWorkspace: async ({ workspaceId }) => {
        const closed = await workspaceRuntimeRegistry.closeWorkspace(workspaceId);
        recordDevBrowserToolsEvent("workspace.closed", { workspaceId, closed });
        return { ok: closed };
      },
      getWorkspaceInfo: (input) => {
        return addWorkspaceBranch(getWorkspaceRuntime(input).getInfo());
      },
      listWorkspaceBranches: (input) => {
        const runtime = getWorkspaceRuntime(input);
        const currentBranch = getWorkspaceBranch(runtime.cwd);
        return {
          currentBranch,
          branches: getWorkspaceBranches(runtime.cwd).map((branch) => ({
            name: branch,
            current: branch === currentBranch,
          })),
        };
      },
      switchWorkspaceBranch: (input) => {
        return switchWorkspaceBranch(getWorkspaceRuntime(input), input.branch);
      },
      getAppLogs: (query) => {
        const runtime = query
          ? getWorkspaceRuntime(query)
          : workspaceRuntimeRegistry.getActiveRuntime();
        return runtime.appLogStore.query(query ? stripWorkspaceId(query) : undefined);
      },
      getAppLogSummary: (input) => getWorkspaceRuntime(input).appLogStore.summary(),
      markAppLogsSeen: ({ workspaceId, throughSeq }) =>
        workspaceRuntimeRegistry.getRuntime(workspaceId).appLogStore.markSeen(throughSeq),
      writeClipboardText: ({ text }) => {
        Utils.clipboardWriteText(text);
        return { ok: true };
      },
      listWorkspacePaths: (input) => {
        const runtime = getWorkspaceRuntime(input);
        return input.refresh ? runtime.pathIndex.refresh() : runtime.pathIndex.list();
      },
      pickWorkspaceAttachments: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const cwd = runtime.cwd;
        const selectedPaths = await Utils.openFileDialog({
          startingFolder: cwd,
          allowedFileTypes: "*",
          canChooseFiles: true,
          canChooseDirectory: true,
          allowsMultipleSelection: true,
        });
        const attachments = [];
        const skippedPaths = [];

        for (const selectedPath of selectedPaths) {
          if (!selectedPath) continue;
          const attachment = createComposerAttachmentFromPath(cwd, selectedPath);
          if (!attachment) {
            skippedPaths.push(selectedPath);
            continue;
          }
          attachments.push(attachment);
        }

        return { attachments, skippedPaths };
      },
      importComposerAttachments: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const attachments = [];
        const skippedPaths = [];
        for (const attachment of input.attachments) {
          try {
            attachments.push(createImportedComposerAttachment(runtime.cwd, attachment));
          } catch {
            skippedPaths.push(attachment.name);
          }
        }
        return { attachments, skippedPaths };
      },
      openWorkspacePath: (input) => {
        const runtime = getWorkspaceRuntime(input);
        const absolutePath = resolveSafeWorkspacePath(runtime, input.workspaceRelativePath);
        if (!absolutePath) return { opened: false, kind: "missing" };

        const kind = getWorkspacePathKind(absolutePath);
        if (kind === "missing") return { opened: false, kind };

        const opened = kind === "folder" ? Utils.openPath(absolutePath) : true;
        if (kind === "file") {
          Utils.showItemInFolder(absolutePath);
        }
        return { opened, kind };
      },
      getSavedWorkflowLibrary: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const state = runtime.agentSettingsStore.getState();
        runtime.appLog.info("workflow.library", "Saved workflow library read.");
        return await readSavedWorkflowLibraryReadModel(runtime.cwd, state.appPreferences);
      },
      deleteSavedWorkflowLibraryItem: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const { path } = input;
        const state = runtime.agentSettingsStore.getState();
        try {
          const result = await deleteSavedWorkflowLibraryPath(
            runtime.cwd,
            path,
            state.appPreferences,
          );
          runtime.appLog.info("workflow.library", "Saved workflow library item deleted.", { path });
          return result;
        } catch (error) {
          runtime.appLog.error("workflow.library", "Saved workflow deletion failed.", error, {
            path,
          });
          throw error;
        }
      },
      openWorkspaceSourceInEditor: (input) => {
        const runtime = getWorkspaceRuntime(input);
        const { path } = input;
        const absolutePath = resolveSafeWorkspacePath(runtime, path);
        if (!absolutePath || getWorkspacePathKind(absolutePath) === "missing") {
          runtime.appLog.warning("external-editor", "Workspace source file does not exist.", {
            path,
          });
          throw new Error(`Workspace source file does not exist: ${path}`);
        }
        const result = openPathInPreferredEditor(runtime, absolutePath);
        runtime.appLog.info("external-editor", "Workspace source opened in external editor.", {
          path,
          editor: result.editor,
          opened: result.opened,
        });
        return { ...result, path };
      },
      openPromptLibraryExternalSourceInEditor: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const sources = await runtime.catalog.getPromptLibraryExternalSources();
        const source = sources.find((candidate) => candidate.path === input.path);
        if (!source || getWorkspacePathKind(source.path) === "missing") {
          runtime.appLog.warning("external-editor", "Prompt standards source does not exist.", {
            path: input.path,
          });
          throw new Error(`Prompt standards source does not exist: ${input.path}`);
        }
        const result = openPathInPreferredEditor(runtime, source.path);
        runtime.appLog.info(
          "external-editor",
          "Prompt standards source opened in external editor.",
          {
            path: source.path,
            editor: result.editor,
            opened: result.opened,
          },
        );
        return { ...result, path: source.path };
      },
      listSessions: async (input) => {
        return await getWorkspaceRuntime(input).catalog.listSessions();
      },
      getCommandInspector: async (input) => {
        const { sessionId, commandId } = input;
        return await getWorkspaceRuntime(input).catalog.getCommandInspector({
          sessionId,
          commandId,
        });
      },
      listHandlerThreads: async (input) => {
        return await getWorkspaceRuntime(input).catalog.listHandlerThreads({
          sessionId: input.sessionId,
        });
      },
      getHandlerThreadInspector: async (input) => {
        const { sessionId, threadId } = input;
        return await getWorkspaceRuntime(input).catalog.getHandlerThreadInspector({
          sessionId,
          threadId,
        });
      },
      getWorkflowTaskAttemptInspector: async (input) => {
        const { sessionId, workflowTaskAttemptId } = input;
        return await getWorkspaceRuntime(input).catalog.getWorkflowTaskAttemptInspector({
          sessionId,
          workflowTaskAttemptId,
        });
      },
      getWorkflowInspector: async (input) => {
        return await getWorkspaceRuntime(input).catalog.getWorkflowInspector(
          stripWorkspaceId(input),
        );
      },
      streamWorkflowInspector: async (input) => {
        return await getWorkspaceRuntime(input).catalog.streamWorkflowInspector(
          stripWorkspaceId(input),
        );
      },
      getProjectCiStatus: async (input) => {
        return await getWorkspaceRuntime(input).catalog.getProjectCiStatus({
          sessionId: input.sessionId,
        });
      },
      getArtifactPreview: async (input) => {
        const { sessionId, artifactId } = input;
        return await getWorkspaceRuntime(input).catalog.getArtifactPreview({
          sessionId,
          artifactId,
        });
      },
      createSession: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const { title, parentSessionId, mode } = input;
        const session = await runtime.catalog.createSession(
          { title, parentSessionId, mode },
          getSessionDefaults(runtime, mode ?? "orchestrator"),
        );
        recordDevBrowserToolsEvent("session.created", {
          parentSessionId: parentSessionId ?? null,
          sessionId: session.target.workspaceSessionId,
          title: title?.trim() || null,
        });
        runtime.appLog.info("session", "Workspace session created.", {
          parentSessionId: parentSessionId ?? null,
          workspaceSessionId: session.target.workspaceSessionId,
        });
        return session;
      },
      openSession: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const { sessionId } = input;
        const session = await runtime.catalog.openSession(sessionId, DEFAULT_SYSTEM_PROMPT);
        recordDevBrowserToolsEvent("session.opened", {
          sessionId,
        });
        runtime.appLog.info("session", "Workspace session opened.", {
          workspaceSessionId: sessionId,
        });
        return session;
      },
      recordSessionOpened: async (input) => {
        getWorkspaceRuntime(input);
        const { sessionId } = input;
        recordDevBrowserToolsEvent("session.opened", {
          sessionId,
        });
        return { ok: true };
      },
      openSurface: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const { target } = input;
        const session = await runtime.catalog.openSurface(target, DEFAULT_SYSTEM_PROMPT);
        recordDevBrowserToolsEvent("surface.opened", {
          surface: target.surface,
          surfacePiSessionId: target.surfacePiSessionId,
          threadId: target.threadId ?? null,
          workspaceSessionId: target.workspaceSessionId,
        });
        runtime.appLog.info("surface", "Surface opened.", {
          surface: target.surface,
          workspaceSessionId: target.workspaceSessionId,
          surfacePiSessionId: target.surfacePiSessionId,
          threadId: target.threadId,
        });
        return session;
      },
      closeSurface: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const { target } = input;
        const result = await runtime.catalog.closeSurface(target);
        recordDevBrowserToolsEvent("surface.closed", {
          surface: target.surface,
          surfacePiSessionId: target.surfacePiSessionId,
          threadId: target.threadId ?? null,
          workspaceSessionId: target.workspaceSessionId,
        });
        runtime.appLog.info("surface", "Surface closed.", {
          surface: target.surface,
          workspaceSessionId: target.workspaceSessionId,
          surfacePiSessionId: target.surfacePiSessionId,
          threadId: target.threadId,
        });
        return result;
      },
      renameSession: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const { sessionId, title } = input;
        const result = await runtime.catalog.renameSession(sessionId, title);
        recordDevBrowserToolsEvent("session.renamed", {
          sessionId,
          title,
        });
        runtime.appLog.info("session", "Workspace session renamed.", {
          workspaceSessionId: sessionId,
          title,
        });
        return result;
      },
      setSessionMode: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const { target, mode } = input;
        const result = await runtime.catalog.setSessionMode(
          target,
          mode,
          getSessionDefaults(runtime, mode),
        );
        if (result.ok && result.snapshot) {
          recordDevBrowserToolsEvent("session.mode.changed", {
            mode,
            sessionId: target.workspaceSessionId,
            surfacePiSessionId: target.surfacePiSessionId,
          });
        } else {
          runtime.appLog.error("session", result.error ?? "Session mode update failed.", {
            mode,
            sessionId: target.workspaceSessionId,
            surfacePiSessionId: target.surfacePiSessionId,
          });
        }
        return result;
      },
      forkSession: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const { sessionId, title, messageTimestamp } = input;
        const session = await runtime.catalog.forkSession(
          { sessionId, title, messageTimestamp },
          getSessionDefaults(runtime),
        );
        recordDevBrowserToolsEvent("session.forked", {
          sessionId,
          targetSessionId: session.target.workspaceSessionId,
          messageTimestamp: messageTimestamp ?? null,
          title: title?.trim() || null,
        });
        return session;
      },
      deleteSession: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const { sessionId } = input;
        const result = await runtime.catalog.deleteSession(sessionId);
        recordDevBrowserToolsEvent("session.deleted", { sessionId });
        runtime.appLog.info("session", "Workspace session deleted.", {
          workspaceSessionId: sessionId,
        });
        return result;
      },
      pinSession: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const { sessionId } = input;
        const result = await runtime.catalog.pinSession(sessionId);
        recordDevBrowserToolsEvent("session.pinned", { sessionId });
        runtime.appLog.info("session", "Workspace session pinned.", {
          workspaceSessionId: sessionId,
        });
        return result;
      },
      unpinSession: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const { sessionId } = input;
        const result = await runtime.catalog.unpinSession(sessionId);
        recordDevBrowserToolsEvent("session.unpinned", { sessionId });
        runtime.appLog.info("session", "Workspace session unpinned.", {
          workspaceSessionId: sessionId,
        });
        return result;
      },
      archiveSession: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const { sessionId } = input;
        const result = await runtime.catalog.archiveSession(sessionId);
        recordDevBrowserToolsEvent("session.archived", { sessionId });
        runtime.appLog.info("session", "Workspace session archived.", {
          workspaceSessionId: sessionId,
        });
        return result;
      },
      unarchiveSession: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const { sessionId } = input;
        const result = await runtime.catalog.unarchiveSession(sessionId);
        recordDevBrowserToolsEvent("session.unarchived", { sessionId });
        runtime.appLog.info("session", "Workspace session unarchived.", {
          workspaceSessionId: sessionId,
        });
        return result;
      },
      markSessionUnread: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const { sessionId } = input;
        const result = await runtime.catalog.markSessionUnread(sessionId);
        recordDevBrowserToolsEvent("session.marked-unread", { sessionId });
        runtime.appLog.info("session", "Workspace session marked unread.", {
          workspaceSessionId: sessionId,
        });
        return result;
      },
      markSessionRead: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const { sessionId } = input;
        const result = await runtime.catalog.markSessionRead(sessionId);
        recordDevBrowserToolsEvent("session.marked-read", { sessionId });
        runtime.appLog.info("session", "Workspace session marked read.", {
          workspaceSessionId: sessionId,
        });
        return result;
      },
      recordFocusedSession: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const { sessionId, surfacePiSessionId } = input;
        const result = await runtime.catalog.recordFocusedSession({
          sessionId,
          surfacePiSessionId,
        });
        if (sessionId) {
          recordDevBrowserToolsEvent("session.focused", { sessionId });
        }
        return result;
      },
      setArchivedGroupCollapsed: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const { collapsed } = input;
        const result = await runtime.catalog.setArchivedGroupCollapsed({ collapsed });
        recordDevBrowserToolsEvent("session.archived-group.toggled", { collapsed });
        return result;
      },
      setSessionNavigationSectionState: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const { section, collapsed, sizePx } = input;
        const result = await runtime.catalog.setSessionNavigationSectionState({
          section,
          collapsed,
          sizePx,
        });
        recordDevBrowserToolsEvent("session.navigation-section.updated", {
          section,
          collapsed,
          sizePx,
        });
        return result;
      },
      sendPrompt: async (payload): Promise<SendPromptResponse> => {
        const runtime = getWorkspaceRuntime(payload);
        const resolved = resolveSendDefaults(runtime, payload);

        if (supportsOAuth(resolved.provider)) {
          await refreshIfNeeded(resolved.provider);
        }

        const apiKey = resolveApiKey(resolved.provider);
        if (!apiKey) {
          const message = getApiKeyMissingError(resolved.provider);
          runtime.appLog.warning(
            "auth.provider",
            "Configured provider is not connected for prompt.",
            {
              provider: resolved.provider,
              workspaceSessionId: payload.target.workspaceSessionId,
              surfacePiSessionId: payload.target.surfacePiSessionId,
              threadId: payload.target.threadId,
            },
          );
          throw new Error(message);
        }

        const model = getModel(
          resolved.provider as Parameters<typeof getModel>[0],
          resolved.model as Parameters<typeof getModel>[1],
        );
        let surfacePiSessionId = payload.target.surfacePiSessionId;

        recordDevBrowserToolsEvent("prompt.requested", {
          messageCount: payload.messages.length,
          model: model.id,
          provider: resolved.provider,
          requestedSurfacePiSessionId: payload.target.surfacePiSessionId,
          requestedWorkspaceSessionId: payload.target.workspaceSessionId,
          requestedThreadId: payload.target.threadId ?? null,
        });
        runtime.appLog.info("prompt", "Prompt requested.", {
          messageCount: payload.messages.length,
          model: model.id,
          provider: resolved.provider,
          workspaceSessionId: payload.target.workspaceSessionId,
          surfacePiSessionId: payload.target.surfacePiSessionId,
          threadId: payload.target.threadId,
        });

        const session = await runtime.catalog.sendPrompt({
          target: payload.target,
          provider: resolved.provider,
          model: model.id,
          thinkingLevel: resolved.reasoningEffort,
          messages: payload.messages,
          systemPrompt: payload.systemPrompt ?? DEFAULT_SYSTEM_PROMPT,
          queueOnly: payload.queueOnly ?? false,
          onEvent: (event) => {
            if (event.type === "start") {
              recordDevBrowserToolsEvent("prompt.started", {
                model: model.id,
                provider: resolved.provider,
                surfacePiSessionId,
                workspaceSessionId: payload.target.workspaceSessionId,
                threadId: payload.target.threadId ?? null,
              });
              runtime.appLog.info("prompt", "Prompt started.", {
                model: model.id,
                provider: resolved.provider,
                workspaceSessionId: payload.target.workspaceSessionId,
                surfacePiSessionId,
                threadId: payload.target.threadId,
              });
            } else if (event.type === "done") {
              recordDevBrowserToolsEvent("prompt.finished", {
                model: model.id,
                provider: resolved.provider,
                reason: event.reason,
                surfacePiSessionId,
                workspaceSessionId: payload.target.workspaceSessionId,
                threadId: payload.target.threadId ?? null,
              });
              runtime.appLog.info("prompt", "Prompt finished.", {
                model: model.id,
                provider: resolved.provider,
                reason: event.reason,
                workspaceSessionId: payload.target.workspaceSessionId,
                surfacePiSessionId,
                threadId: payload.target.threadId,
              });
            } else if (event.type === "error") {
              const message =
                event.error.content.find((block) => block.type === "text")?.text ||
                "Prompt failed.";
              recordDevBrowserToolsEvent("prompt.failed", {
                model: model.id,
                provider: resolved.provider,
                reason: event.reason,
                surfacePiSessionId,
                workspaceSessionId: payload.target.workspaceSessionId,
                threadId: payload.target.threadId ?? null,
              });
              runtime.appLog.error("prompt", message, {
                model: model.id,
                provider: resolved.provider,
                reason: event.reason,
                surfacePiSessionId,
                workspaceSessionId: payload.target.workspaceSessionId,
                threadId: payload.target.threadId ?? null,
              });
            }
          },
        });

        surfacePiSessionId = session.target.surfacePiSessionId;
        runtime.appLog.info(
          "prompt",
          session.queued ? "Prompt queued for active surface." : "Prompt dispatched to pi runtime.",
          {
            model: model.id,
            provider: resolved.provider,
            queued: session.queued ?? false,
            surfacePiSessionId,
            workspaceSessionId: session.target.workspaceSessionId,
            threadId: session.target.threadId,
          },
        );
        return session;
      },
      steerPrompt: async (payload): Promise<SendPromptResponse> => {
        const runtime = getWorkspaceRuntime(payload);
        const resolved = resolveSendDefaults(runtime, payload);

        if (supportsOAuth(resolved.provider)) {
          await refreshIfNeeded(resolved.provider);
        }

        const apiKey = resolveApiKey(resolved.provider);
        if (!apiKey) {
          const message = getApiKeyMissingError(resolved.provider);
          runtime.appLog.warning(
            "auth.provider",
            "Configured provider is not connected for prompt steering.",
            {
              provider: resolved.provider,
              workspaceSessionId: payload.target.workspaceSessionId,
              surfacePiSessionId: payload.target.surfacePiSessionId,
              threadId: payload.target.threadId,
            },
          );
          throw new Error(message);
        }

        const model = getModel(
          resolved.provider as Parameters<typeof getModel>[0],
          resolved.model as Parameters<typeof getModel>[1],
        );
        recordDevBrowserToolsEvent("prompt.steer.requested", {
          messageCount: payload.messages.length,
          model: model.id,
          provider: resolved.provider,
          requestedSurfacePiSessionId: payload.target.surfacePiSessionId,
          requestedWorkspaceSessionId: payload.target.workspaceSessionId,
          requestedThreadId: payload.target.threadId ?? null,
        });
        runtime.appLog.info("prompt", "Prompt steer requested.", {
          messageCount: payload.messages.length,
          model: model.id,
          provider: resolved.provider,
          workspaceSessionId: payload.target.workspaceSessionId,
          surfacePiSessionId: payload.target.surfacePiSessionId,
          threadId: payload.target.threadId,
        });

        const session = await runtime.catalog.steerPrompt({
          target: payload.target,
          provider: resolved.provider,
          model: model.id,
          thinkingLevel: resolved.reasoningEffort,
          messages: payload.messages,
          systemPrompt: payload.systemPrompt ?? DEFAULT_SYSTEM_PROMPT,
        });

        runtime.appLog.info("prompt", "Prompt steer dispatched to pi runtime.", {
          model: model.id,
          provider: resolved.provider,
          surfacePiSessionId: session.target.surfacePiSessionId,
          workspaceSessionId: session.target.workspaceSessionId,
          threadId: session.target.threadId,
        });
        return session;
      },
      deleteQueuedSurfaceMessage: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const result = await runtime.catalog.deleteQueuedSurfaceMessage(input);
        runtime.appLog.info("prompt", "Queued surface message deleted.", {
          workspaceSessionId: input.target.workspaceSessionId,
          surfacePiSessionId: input.target.surfacePiSessionId,
          threadId: input.target.threadId,
          queuedMessageId: input.queuedMessageId,
        });
        return result;
      },
      editQueuedSurfaceMessage: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const result = await runtime.catalog.editQueuedSurfaceMessage(input);
        runtime.appLog.info("prompt", "Queued surface message restored to composer.", {
          workspaceSessionId: input.target.workspaceSessionId,
          surfacePiSessionId: input.target.surfacePiSessionId,
          threadId: input.target.threadId,
          queuedMessageId: input.queuedMessageId,
        });
        return result;
      },
      reorderQueuedSurfaceMessage: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const result = await runtime.catalog.reorderQueuedSurfaceMessage(input);
        runtime.appLog.info("prompt", "Queued surface messages reordered.", {
          workspaceSessionId: input.target.workspaceSessionId,
          surfacePiSessionId: input.target.surfacePiSessionId,
          threadId: input.target.threadId,
          queuedMessageId: input.queuedMessageId,
          beforeQueuedMessageId: input.beforeQueuedMessageId ?? null,
        });
        return result;
      },
      steerQueuedSurfaceMessage: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const result = await runtime.catalog.steerQueuedSurfaceMessage(input);
        runtime.appLog.info("prompt", "Queued surface message steered.", {
          workspaceSessionId: input.target.workspaceSessionId,
          surfacePiSessionId: input.target.surfacePiSessionId,
          threadId: input.target.threadId,
          queuedMessageId: input.queuedMessageId,
        });
        return result;
      },
      cancelPrompt: async (input): Promise<{ ok: boolean }> => {
        const runtime = getWorkspaceRuntime(input);
        const { target } = input;
        await runtime.catalog.cancelPrompt(target);
        recordDevBrowserToolsEvent("prompt.cancel.requested", {
          surfacePiSessionId: target.surfacePiSessionId,
          threadId: target.threadId ?? null,
          workspaceSessionId: target.workspaceSessionId,
        });
        runtime.appLog.info("prompt", "Prompt cancellation requested.", {
          workspaceSessionId: target.workspaceSessionId,
          surfacePiSessionId: target.surfacePiSessionId,
          threadId: target.threadId,
        });
        return { ok: true };
      },
      setSurfaceModel: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const { target, provider, model } = input;
        const result = await runtime.catalog.setSurfaceModel(target, provider, model);
        if (result.ok) {
          recordDevBrowserToolsEvent("surface.model.changed", {
            model,
            surfacePiSessionId: target.surfacePiSessionId,
            threadId: target.threadId ?? null,
            workspaceSessionId: target.workspaceSessionId,
          });
          runtime.appLog.info("surface", "Surface model changed.", {
            model,
            provider,
            workspaceSessionId: target.workspaceSessionId,
            surfacePiSessionId: target.surfacePiSessionId,
            threadId: target.threadId,
          });
        } else {
          runtime.appLog.error(
            "surface",
            `Surface pi session ${target.surfacePiSessionId} was not found for model update.`,
            {
              model,
              surfacePiSessionId: target.surfacePiSessionId,
            },
          );
        }
        return result;
      },
      setSurfaceThoughtLevel: async (input) => {
        const runtime = getWorkspaceRuntime(input);
        const { target, level } = input;
        const result = await runtime.catalog.setSurfaceThoughtLevel(target, level);
        if (result.ok) {
          recordDevBrowserToolsEvent("surface.reasoning.changed", {
            level,
            surfacePiSessionId: target.surfacePiSessionId,
            threadId: target.threadId ?? null,
            workspaceSessionId: target.workspaceSessionId,
          });
          runtime.appLog.info("surface", "Surface reasoning changed.", {
            level,
            workspaceSessionId: target.workspaceSessionId,
            surfacePiSessionId: target.surfacePiSessionId,
            threadId: target.threadId,
          });
        } else {
          runtime.appLog.error(
            "surface",
            `Surface pi session ${target.surfacePiSessionId} was not found for reasoning update.`,
            {
              level,
              surfacePiSessionId: target.surfacePiSessionId,
            },
          );
        }
        return result;
      },
      listProviderAuths: async (): Promise<ProviderAuthInfo[]> => listProviderAuthSummaries(),
      setProviderApiKey: async ({
        providerId,
        apiKey,
      }: {
        providerId: string;
        apiKey: string;
      }): Promise<{ ok: boolean }> => {
        storeApiKey(providerId, apiKey);
        recordDevBrowserToolsEvent("provider.auth.updated", {
          keyType: "apikey",
          providerId,
        });
        workspaceRuntimeRegistry
          .getActiveRuntimeOrNull()
          ?.appLog.info("auth.provider", "Provider auth updated.", {
            providerId,
            keyType: "apikey",
          });
        return { ok: true };
      },
      startOAuth: async ({
        providerId,
      }: {
        providerId: string;
      }): Promise<{ ok: boolean; error?: string }> => {
        try {
          await startOAuthLogin(providerId);
          recordDevBrowserToolsEvent("provider.oauth.started", { providerId });
          workspaceRuntimeRegistry
            .getActiveRuntimeOrNull()
            ?.appLog.info("auth.provider", "Provider OAuth started.", { providerId });
          return { ok: true };
        } catch (error) {
          const message = error instanceof Error ? error.message : String(error);
          recordAppRuntimeError("rpc", message, "bun.oauth", { providerId }, error);
          return {
            ok: false,
            error: message,
          };
        }
      },
      removeProviderAuth: async ({
        providerId,
      }: {
        providerId: string;
      }): Promise<{ ok: boolean }> => {
        removeCredential(providerId);
        recordDevBrowserToolsEvent("provider.auth.removed", { providerId });
        workspaceRuntimeRegistry
          .getActiveRuntimeOrNull()
          ?.appLog.info("auth.provider", "Provider auth removed.", { providerId });
        return { ok: true };
      },
    },
  },
});

const appMenu: Parameters<typeof ApplicationMenu.setApplicationMenu>[0] = [
  {
    label: "svvy",
    submenu: [
      { role: "about" },
      { type: "separator" },
      { role: "hide", accelerator: "CommandOrControl+H" },
      { role: "hideOthers", accelerator: "CommandOrControl+Option+H" },
      { role: "showAll" },
      { type: "separator" },
      { role: "quit", accelerator: "CommandOrControl+Q" },
    ],
  },
  {
    label: "File",
    submenu: [
      appMenuItem("workspace.open"),
      appMenuItem("workspace.newTab"),
      appMenuItem("workspace.openInNewTab"),
      { type: "separator" },
      appMenuItem("session.new"),
      appMenuItem("session.dumb"),
    ],
  },
  {
    label: "Edit",
    submenu: [
      { role: "undo", accelerator: "CommandOrControl+Z" },
      { role: "redo", accelerator: "CommandOrControl+Shift+Z" },
      { type: "separator" },
      { role: "cut", accelerator: "CommandOrControl+X" },
      { role: "copy", accelerator: "CommandOrControl+C" },
      { role: "paste", accelerator: "CommandOrControl+V" },
      { role: "pasteAndMatchStyle" },
      { role: "delete" },
      { type: "separator" },
      { role: "selectAll", accelerator: "CommandOrControl+A" },
    ],
  },
  {
    label: "View",
    submenu: [
      appMenuItem("commandPalette.open"),
      appMenuItem("quickOpen.open"),
      { type: "separator" },
      appMenuItem("sidebar.toggle"),
      { type: "separator" },
      appMenuItem("surface.logs.open"),
      appMenuItem("surface.workflows.open"),
      appMenuItem("surface.context.open"),
    ],
  },
  {
    label: "Window",
    submenu: [
      { role: "close" },
      { role: "minimize" },
      { role: "zoom" },
      { type: "separator" },
      { role: "bringAllToFront" },
    ],
  },
];

const localInfoChannelPromise = Updater.localInfo.channel();

ApplicationMenu.setApplicationMenu(appMenu);
ApplicationMenu.on("application-menu-clicked", (event) => {
  const action = (event as { data?: { action?: unknown } }).data?.action;
  if (!isAppMenuAction(action)) {
    return;
  }
  rpc.send.sendAppMenuAction({ action });
});

loadRuntimeEnv(startupWorkspaceCwd);

const appChannel = await localInfoChannelPromise;
const url = await getMainViewUrl(appChannel);

mainWindow = new BrowserWindow({
  title: "svvy",
  frame: {
    x: 0,
    y: 0,
    width: 1180,
    height: 820,
  },
  titleBarStyle: process.platform === "darwin" ? "hiddenInset" : "default",
  hidden: process.platform === "darwin",
  rpc,
});

if (appChannel === "dev") {
  const { mountDevBrowserToolsBridge } = await import("./dev-browser-tools-bridge");
  const mountedDevBrowserToolsBridge = await mountDevBrowserToolsBridge({
    defaultSystemPrompt: DEFAULT_SYSTEM_PROMPT,
    getDefaultAgentSettings,
    getMainWindow: () => mainWindow,
    getActiveWorkspace: () => workspaceRuntimeRegistry.getActiveRuntimeOrNull()?.getInfo() ?? null,
    getOpenWorkspaces: () => workspaceRuntimeRegistry.listOpenWorkspaces(),
    getWorkspaceBranch,
    listProviderAuthSummaries,
    listOpenSurfaceSnapshots: async () =>
      (await workspaceRuntimeRegistry
        .getActiveRuntimeOrNull()
        ?.catalog.listOpenSurfaceSnapshots()) ?? [],
    listWorkspaceSessions: async () =>
      (await workspaceRuntimeRegistry.getActiveRuntimeOrNull()?.catalog.listSessions()) ?? {
        sessions: [],
      },
    mainWindow,
  });
  devBrowserToolsRecorder = mountedDevBrowserToolsBridge;

  recordDevBrowserToolsEvent("app.ready", {
    bridgeUrl: mountedDevBrowserToolsBridge.url ?? null,
    url,
    workspaceId: workspaceRuntimeRegistry.getActiveWorkspaceId(),
  });
  recordAppRuntimeLog("info", "svvy dev browser tools bridge mounted.", "dev-browser-tools", {
    appId: mountedDevBrowserToolsBridge.appId,
    bridgeUrl: mountedDevBrowserToolsBridge.url ?? null,
  });
  console.log(
    `svvy bridge: ${JSON.stringify({
      appId: mountedDevBrowserToolsBridge.appId,
      bridgeUrl: mountedDevBrowserToolsBridge.url ?? null,
    })}`,
  );
}

mainWindow.webview.loadURL(url);
positionNativeTrafficLights(mainWindow.ptr, NATIVE_TRAFFIC_LIGHT_POSITION);
mainWindow.show();

void mainWindow;

console.log("svvy desktop app started");
