import { afterEach, describe, expect, it } from "bun:test";
import {
  chmodSync,
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { basename, dirname, join } from "node:path";
import { tmpdir } from "node:os";
import type { PromptExecutionRuntimeHandle } from "./prompt-execution-context";
import { createWorkflowLibrary } from "./smithers-runtime/workflow-library";
import {
  createStructuredSessionStateStore,
  type StructuredSessionStateStore,
} from "./structured-session-state";
import { createExecuteTypescriptTool } from "./execute-typescript-tool";
import { createWebProvider } from "./web-runtime/provider-registry";

const stores: StructuredSessionStateStore[] = [];
const tempDirs: string[] = [];
const originalCxBin = process.env.SVVY_CX_BIN;
const originalFetch = globalThis.fetch;

afterEach(() => {
  while (stores.length > 0) {
    stores.pop()?.close();
  }
  while (tempDirs.length > 0) {
    const dir = tempDirs.pop();
    if (dir) {
      rmSync(dir, { force: true, recursive: true });
    }
  }
  if (originalCxBin === undefined) {
    delete process.env.SVVY_CX_BIN;
  } else {
    process.env.SVVY_CX_BIN = originalCxBin;
  }
  globalThis.fetch = originalFetch;
});

function createWorkspaceRoot(): string {
  const root = mkdtempSync(join(tmpdir(), "svvy-execute-typescript-"));
  tempDirs.push(root);
  return root;
}

function writeWorkspaceFile(workspaceRoot: string, path: string, text: string): void {
  const filePath = join(workspaceRoot, path);
  mkdirSync(dirname(filePath), { recursive: true });
  writeFileSync(filePath, text, "utf8");
}

function writeFakeCxBinary(workspaceRoot: string): string {
  const path = join(workspaceRoot, "fake-cx");
  writeFileSync(
    path,
    [
      "#!/usr/bin/env bun",
      "const args = Bun.argv.slice(2);",
      "if (args[0] === 'overview') console.log(JSON.stringify([{ file: args[1] ?? '.', symbols: ['main'] }]));",
      "else if (args[0] === 'symbols') console.log(JSON.stringify([{ file: 'src/index.ts', name: 'main', kind: 'function' }]));",
      "else if (args[0] === 'definition') console.log(JSON.stringify([{ file: 'src/index.ts', line: 1, body: 'function main() {}' }]));",
      "else if (args[0] === 'references') console.log(JSON.stringify([{ file: 'src/index.ts', line: 2, caller: 'boot' }]));",
      "else if (args[0] === 'lang' && args[1] === 'list') console.log('typescript installed');",
      "else if (args[0] === 'cache' && args[1] === 'path') console.log('/tmp/cx-cache');",
      "else { console.error('unsupported fake cx command'); process.exit(2); }",
    ].join("\n"),
    "utf8",
  );
  chmodSync(path, 0o755);
  process.env.SVVY_CX_BIN = path;
  return path;
}

function createStore(sessionId: string, workspaceCwd: string): StructuredSessionStateStore {
  const store = createStructuredSessionStateStore({
    workspace: {
      id: workspaceCwd,
      label: "svvy",
      cwd: workspaceCwd,
    },
  });
  store.upsertPiSession({
    sessionId,
    title: "Execute Typescript",
    provider: "openai",
    model: "gpt-5.4",
    reasoningEffort: "medium",
    messageCount: 1,
    status: "running",
    createdAt: "2026-04-16T10:00:00.000Z",
    updatedAt: "2026-04-16T10:00:00.000Z",
  });
  stores.push(store);
  return store;
}

function createRuntime(
  store: StructuredSessionStateStore,
  sessionId: string,
  promptText = "Inspect the repository with execute_typescript",
): PromptExecutionRuntimeHandle {
  const turn = store.startTurn({
    sessionId,
    surfacePiSessionId: sessionId,
    requestSummary: promptText,
  });

  return {
    current: {
      sessionId,
      turnId: turn.id,
      surfacePiSessionId: sessionId,
      surfaceThreadId: null,
      surfaceKind: "orchestrator",
      defaultEpisodeKind: "analysis",
      rootThreadId: null,
      promptText,
      rootEpisodeKind: "analysis",
      sessionWaitApplied: false,
      threadWasTerminalAtStart: false,
    },
  };
}

function createHandlerRuntime(
  store: StructuredSessionStateStore,
  sessionId: string,
  promptText = "Inspect the repository with handler execute_typescript",
): PromptExecutionRuntimeHandle {
  const orchestratorTurn = store.startTurn({
    sessionId,
    surfacePiSessionId: sessionId,
    requestSummary: "Delegate handler work",
  });
  const thread = store.createThread({
    turnId: orchestratorTurn.id,
    surfacePiSessionId: `${sessionId}-handler`,
    title: "Handler work",
    objective: promptText,
  });
  store.finishTurn({ turnId: orchestratorTurn.id, status: "completed" });
  const handlerTurn = store.startTurn({
    sessionId,
    surfacePiSessionId: thread.surfacePiSessionId,
    threadId: thread.id,
    requestSummary: promptText,
  });

  return {
    current: {
      sessionId,
      turnId: handlerTurn.id,
      surfacePiSessionId: thread.surfacePiSessionId,
      surfaceThreadId: thread.id,
      surfaceKind: "handler",
      defaultEpisodeKind: "change",
      rootThreadId: thread.id,
      promptText,
      rootEpisodeKind: "change",
      sessionWaitApplied: false,
      threadWasTerminalAtStart: false,
    },
  };
}

describe("execute_typescript tool", () => {
  it("requires an active prompt runtime", async () => {
    const workspaceCwd = createWorkspaceRoot();
    const tool = createExecuteTypescriptTool({
      cwd: workspaceCwd,
      runtime: { current: null },
      store: createStore("session-no-runtime", workspaceCwd),
    });

    await expect(
      tool.execute("tool-call-1", {
        typescriptCode: "return { ok: true };",
      }),
    ).rejects.toThrow("execute_typescript can only run during an active prompt.");
  });

  it("returns structured diagnostics and persists the submitted snippet before runtime execution", async () => {
    const workspaceCwd = createWorkspaceRoot();
    const store = createStore("session-static-failure", workspaceCwd);
    const runtime = createRuntime(store, "session-static-failure");
    const tool = createExecuteTypescriptTool({
      cwd: workspaceCwd,
      runtime,
      store,
    });

    const result = await tool.execute("tool-call-2", {
      typescriptCode: "const title: string = 42;",
    });

    expect(result.details).toMatchObject({
      success: false,
      error: {
        stage: "typecheck",
      },
    });

    const snapshot = store.getSessionState("session-static-failure");
    expect(snapshot.turns[0]?.turnDecision).toBe("execute_typescript");
    expect(snapshot.commands).toEqual([
      expect.objectContaining({
        toolName: "execute_typescript",
        executor: "orchestrator",
        visibility: "summary",
        status: "failed",
      }),
    ]);
    expect(snapshot.artifacts.map((artifact) => artifact.name)).toEqual([
      "execute-typescript.ts",
      "execute-typescript.diagnostics.json",
    ]);
    const [snippetArtifact, diagnosticsArtifact] = snapshot.artifacts;
    expect(basename(snippetArtifact!.path!)).toBe(`${snippetArtifact!.id}-execute-typescript.ts`);
    expect(existsSync(snippetArtifact!.path!)).toBe(true);
    expect(readFileSync(snippetArtifact!.path!, "utf8")).toBe("const title: string = 42;");
    expect(existsSync(diagnosticsArtifact!.path!)).toBe(true);
    expect(snapshot.episodes).toEqual([]);
  });

  it("omits workflow APIs for orchestrator code mode at typecheck and runtime", async () => {
    const workspaceCwd = createWorkspaceRoot();
    writeWorkspaceFile(
      workspaceCwd,
      ".svvy/workflows/prompts/proof.mdx",
      "---\ntitle: Proof prompt\nsummary: Prompt proof.\n---\n",
    );
    const store = createStore("session-orchestrator-workflow-denied", workspaceCwd);
    const runtime = createRuntime(store, "session-orchestrator-workflow-denied");
    const tool = createExecuteTypescriptTool({
      cwd: workspaceCwd,
      runtime,
      store,
      workflowLibrary: createWorkflowLibrary(workspaceCwd),
    });

    const typechecked = await tool.execute("tool-call-workflow-typecheck-denied", {
      typescriptCode: 'return await api.workflow_list_assets({ scope: "saved" });',
    });
    expect(typechecked.details.success).toBe(false);
    expect(typechecked.details.error?.stage).toBe("typecheck");
    expect(typechecked.details.error?.message).toContain("workflow");

    const dynamic = await tool.execute("tool-call-workflow-runtime-denied", {
      typescriptCode: 'return await (api as any).workflow_list_assets({ scope: "saved" });',
    });
    expect(dynamic.details.success).toBe(false);
    expect(dynamic.details.error?.stage).toBe("runtime");
    expect(dynamic.details.error?.message).toBe(
      "execute_typescript api.workflow_* helpers are not available for orchestrator actors.",
    );
    const snapshot = store.getSessionState("session-orchestrator-workflow-denied");
    expect(snapshot.commands.map((command) => command.toolName)).not.toContain(
      "workflow_list_assets",
    );
  });

  it("runs a typed composition through duplicated direct tools and records child commands", async () => {
    const workspaceCwd = createWorkspaceRoot();
    writeWorkspaceFile(workspaceCwd, "notes.txt", "alpha\nbeta\n");
    writeFakeCxBinary(workspaceCwd);
    writeWorkspaceFile(
      workspaceCwd,
      ".svvy/workflows/prompts/proof.mdx",
      "---\ntitle: Proof prompt\nsummary: Prompt proof.\n---\n",
    );

    const store = createStore("session-success", workspaceCwd);
    const runtime = createHandlerRuntime(
      store,
      "session-success",
      "Inspect a file and persist a summary",
    );
    const tool = createExecuteTypescriptTool({
      cwd: workspaceCwd,
      runtime,
      store,
      workflowLibrary: createWorkflowLibrary(workspaceCwd),
    });

    const result = await tool.execute("tool-call-3", {
      typescriptCode: [
        'const file = await api.read({ path: "notes.txt" });',
        'const overview = await api.cx_overview({ path: "." });',
        'const status = await api.bash({ command: "printf clean" });',
        'const assets = await api.workflow_list_assets({ kind: "prompt", scope: "saved" });',
        "const artifact = await api.artifact_write_text({",
        '  name: "summary.md",',
        '  text: `${file.content[0]?.type === "text" ? file.content[0].text.split("\\n")[0] : ""}:${status.content[0]?.type === "text" ? status.content[0].text.trim() : ""}`',
        "});",
        'console.log("artifact", artifact.details.path);',
        "return { assetCount: assets.details.assets.length, artifactId: artifact.details.artifactId, cxCommand: overview.details.command.join(' ') };",
      ].join("\n"),
    });

    expect(result.details).toMatchObject({
      success: true,
      result: {
        assetCount: 1,
        artifactId: expect.any(String),
        cxCommand: "cx overview . --json",
      },
    });

    const snapshot = store.getSessionState("session-success");
    expect(
      snapshot.turns.find((turn) => turn.surfacePiSessionId === "session-success-handler")
        ?.turnDecision,
    ).toBe("execute_typescript");
    const [parentCommand, ...childCommands] = snapshot.commands;
    expect(parentCommand).toMatchObject({
      toolName: "execute_typescript",
      status: "succeeded",
    });
    expect(parentCommand?.summary).toContain("Read 1 tool result");
    expect(parentCommand?.summary).toContain("Ran 1 bash command");
    expect(parentCommand?.summary).toContain("Ran 1 cx navigation call");
    expect(parentCommand?.summary).toContain("Created 1 artifact");
    expect(parentCommand?.summary).toContain("Discovered 1 workflow asset");
    expect(childCommands.map((command) => command.toolName)).toEqual([
      "read",
      "cx_overview",
      "bash",
      "workflow_list_assets",
      "artifact_write_text",
    ]);
    expect(childCommands).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          parentCommandId: parentCommand!.id,
          toolName: "cx_overview",
          executor: "execute_typescript",
          visibility: "trace",
          status: "succeeded",
          facts: expect.objectContaining({
            command: ["cx", "overview", ".", "--json"],
            exitCode: 0,
            resultCount: 1,
          }),
        }),
        expect.objectContaining({
          parentCommandId: parentCommand!.id,
          toolName: "read",
          executor: "execute_typescript",
          visibility: "trace",
          status: "succeeded",
          facts: expect.objectContaining({
            path: "notes.txt",
          }),
        }),
        expect.objectContaining({
          parentCommandId: parentCommand!.id,
          toolName: "bash",
          executor: "execute_typescript",
          visibility: "summary",
          status: "succeeded",
          facts: expect.objectContaining({
            command: "printf clean",
          }),
        }),
        expect.objectContaining({
          parentCommandId: parentCommand!.id,
          toolName: "artifact_write_text",
          executor: "execute_typescript",
          visibility: "summary",
          status: "succeeded",
          facts: expect.objectContaining({
            artifactId: expect.any(String),
          }),
        }),
      ]),
    );
    expect(snapshot.artifacts.map((artifact) => artifact.name)).toEqual([
      "execute-typescript.ts",
      "summary.md",
      "execute-typescript.logs.log",
    ]);
  });

  it("exposes api.web_* and records artifact-backed fetch child facts", async () => {
    const workspaceCwd = createWorkspaceRoot();
    const store = createStore("session-web", workspaceCwd);
    const runtime = createRuntime(store, "session-web", "Fetch web evidence");
    globalThis.fetch = (async () =>
      new Response(
        JSON.stringify({
          data: {
            markdown: "Fetched evidence",
            url: "https://example.com/reference",
            metadata: { title: "Docs" },
          },
        }),
        {
          status: 200,
          headers: { "content-type": "application/json" },
        },
      )) as unknown as typeof fetch;
    const tool = createExecuteTypescriptTool({
      cwd: workspaceCwd,
      runtime,
      store,
      webProvider: createWebProvider({ provider: "firecrawl" }, { firecrawlApiKey: "fc-key" }),
    });

    const result = await tool.execute("tool-call-web", {
      typescriptCode: [
        'const fetched = await api.web_fetch({ url: "https://example.com/reference" });',
        "const artifactPath = fetched.details.artifacts?.[0]?.path;",
        "if (!artifactPath) throw new Error('missing artifact path');",
        "const body = await api.read({ path: artifactPath });",
        "return { provider: fetched.details.providerId, artifactPath, body: body.content[0] };",
      ].join("\n"),
    });

    expect(result.details.success).toBe(true);
    const snapshot = store.getSessionState("session-web");
    const webFetch = snapshot.commands.find((command) => command.toolName === "web_fetch");
    expect(webFetch).toMatchObject({
      executor: "execute_typescript",
      status: "succeeded",
      visibility: "summary",
      facts: expect.objectContaining({
        providerId: "firecrawl",
        toolName: "web_fetch",
        artifactPaths: expect.arrayContaining([expect.stringContaining("web-fetch")]),
        metadataArtifactId: expect.any(String),
      }),
    });
    expect(snapshot.commands.map((command) => command.toolName)).toEqual([
      "execute_typescript",
      "web_fetch",
      "read",
    ]);
    expect(snapshot.artifacts.map((artifact) => artifact.name)).toEqual([
      "execute-typescript.ts",
      "web-fetch.md",
      "web-fetch.metadata.json",
    ]);
  });

  it("typechecks api.web_* against the selected provider contract", async () => {
    const workspaceCwd = createWorkspaceRoot();
    const store = createStore("session-web-types", workspaceCwd);
    const runtime = createRuntime(store, "session-web-types", "Check web provider typing");
    globalThis.fetch = (async () =>
      new Response(
        JSON.stringify({
          data: [{ url: "https://example.com/docs", title: "Docs" }],
        }),
        {
          status: 200,
          headers: { "content-type": "application/json" },
        },
      )) as unknown as typeof fetch;
    const tool = createExecuteTypescriptTool({
      cwd: workspaceCwd,
      runtime,
      store,
      webProvider: createWebProvider({ provider: "firecrawl" }, { firecrawlApiKey: "fc-key" }),
    });

    const accepted = await tool.execute("tool-call-web-types-ok", {
      typescriptCode:
        'return await api.web_search({ query: "docs", scrapeOptions: { formats: ["markdown"], onlyMainContent: true } });',
    });
    expect(accepted.details.success).toBe(true);

    const rejected = await tool.execute("tool-call-web-types-bad", {
      typescriptCode: 'return await api.web_search({ query: "docs", site: "example.com" });',
    });
    expect(rejected.details.success).toBe(false);
    expect(rejected.details.error?.stage).toBe("typecheck");
    expect(rejected.details.error?.message).toContain("site");
  });

  it("typechecks api.web_* against the TinyFish SDK fetch contract", async () => {
    const workspaceCwd = createWorkspaceRoot();
    const store = createStore("session-web-tinyfish-types", workspaceCwd);
    const runtime = createRuntime(store, "session-web-tinyfish-types", "Check TinyFish web typing");
    const previousFetch = globalThis.fetch;
    globalThis.fetch = (async () =>
      new Response(
        JSON.stringify({
          results: [
            {
              url: "https://example.com/docs",
              final_url: "https://example.com/docs",
              title: "Docs",
              description: null,
              language: "en",
              author: null,
              published_date: null,
              latency_ms: 10,
              format: "markdown",
              text: "TinyFish docs",
            },
          ],
          errors: [],
        }),
        { status: 200, headers: { "content-type": "application/json" } },
      )) as unknown as typeof fetch;
    const tool = createExecuteTypescriptTool({
      cwd: workspaceCwd,
      runtime,
      store,
      webProvider: createWebProvider({ provider: "tinyfish" }, { tinyfishApiKey: "tf-key" }),
    });
    try {
      const accepted = await tool.execute("tool-call-tinyfish-web-types-ok", {
        typescriptCode:
          'const result = await api.web_fetch({ urls: ["https://example.com/docs"], links: true }); return result.details.providerId;',
      });
      expect(accepted.details.success).toBe(true);

      const rejected = await tool.execute("tool-call-tinyfish-web-types-bad", {
        typescriptCode: 'return await api.web_fetch({ url: "https://example.com/docs" });',
      });
      expect(rejected.details.success).toBe(false);
      expect(rejected.details.error?.stage).toBe("typecheck");
      expect(rejected.details.error?.message).toContain("url");
    } finally {
      globalThis.fetch = previousFetch;
    }
  });

  it("fetches one real page through TinyFish and writes artifact-backed output", async () => {
    const tinyfishApiKey = process.env.TINYFISH_API_KEY?.trim();
    if (!tinyfishApiKey) {
      return;
    }
    const workspaceCwd = createWorkspaceRoot();
    const store = createStore("session-web-tinyfish-live", workspaceCwd);
    const runtime = createRuntime(
      store,
      "session-web-tinyfish-live",
      "Fetch one live TinyFish page",
    );
    const tool = createExecuteTypescriptTool({
      cwd: workspaceCwd,
      runtime,
      store,
      webProvider: createWebProvider({ provider: "tinyfish" }, { tinyfishApiKey }),
    });

    const result = await tool.execute("tool-call-tinyfish-live-fetch", {
      typescriptCode: [
        'const fetched = await api.web_fetch({ urls: ["https://example.com"], format: "markdown" });',
        "const artifactPath = fetched.details.artifacts?.[0]?.path;",
        "if (!artifactPath) throw new Error('missing TinyFish fetch artifact path');",
        "const body = await api.read({ path: artifactPath });",
        "return {",
        "  providerId: fetched.details.providerId,",
        "  artifactCount: fetched.details.artifacts?.length ?? 0,",
        "  metadataPath: fetched.details.metadataArtifact?.path,",
        "  bodyPreview: body.content[0],",
        "};",
      ].join("\n"),
    });

    expect(result.details.success).toBe(true);
    const snapshot = store.getSessionState("session-web-tinyfish-live");
    const webFetch = snapshot.commands.find((command) => command.toolName === "web_fetch");
    expect(webFetch).toMatchObject({
      executor: "execute_typescript",
      status: "succeeded",
      facts: expect.objectContaining({
        providerId: "tinyfish",
        toolName: "web_fetch",
        url: "https://example.com/",
        artifactPaths: expect.arrayContaining([expect.stringContaining("web-fetch")]),
        metadataArtifactId: expect.any(String),
      }),
    });
    const fetchArtifact = snapshot.artifacts.find((artifact) => artifact.name === "web-fetch.md");
    const metadataArtifact = snapshot.artifacts.find(
      (artifact) => artifact.name === "web-fetch.metadata.json",
    );
    expect(fetchArtifact?.path).toBeTruthy();
    expect(metadataArtifact?.path).toBeTruthy();
    const fetchContent = readFileSync(fetchArtifact!.path!, "utf8");
    const metadataContent = readFileSync(metadataArtifact!.path!, "utf8");
    expect(fetchContent).toContain("documentation examples");
    expect(metadataContent).toContain("https://example.com");
    expect(JSON.stringify(snapshot)).not.toContain(tinyfishApiKey);
  }, 30000);

  it("omits api.web_* when no keyed provider is configured", async () => {
    const workspaceCwd = createWorkspaceRoot();
    const store = createStore("session-web-absent", workspaceCwd);
    const runtime = createRuntime(store, "session-web-absent", "Check missing web provider");
    const tool = createExecuteTypescriptTool({
      cwd: workspaceCwd,
      runtime,
      store,
    });

    const result = await tool.execute("tool-call-web-absent", {
      typescriptCode: 'return await api.web_search({ query: "docs" });',
    });

    expect(result.details.success).toBe(false);
    expect(result.details.error?.stage).toBe("typecheck");
    expect(result.details.error?.message).toContain("web");
  });
});
