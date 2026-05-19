# `execute_typescript` and Direct Tool Surface

## Product Contract

svvy exposes native direct tools as the normal coding-agent interface. The direct tools are the canonical surface for semantic code navigation, reading files, searching, editing, writing, running shell commands, creating artifacts, actor-owned workflow discovery, and using provider-backed web search and fetch.

`execute_typescript` is an actor-local composition tool. It receives a bounded TypeScript snippet, injects a small `api` object generated for the current actor, typechecks the snippet against that actor-specific declaration, runs it, and records each nested `api` call as a child command. Agents use it when TypeScript control flow is useful: batching, looping, filtering, aggregation, handler-owned workflow discovery, bash-backed inspection, or artifact evidence.

Ordinary one-shot repository actions use direct tools. Code mode does not own general repository I/O.
Code mode also does not widen actor authority: if an actor does not receive a direct tool or Smithers tool in its normal callable surface, it does not receive that capability through `execute_typescript`.

## Direct Tools

### Coding Tools

svvy ships PI's coding tools directly, wrapped by svvy command recording:

| Tool | Purpose |
| --- | --- |
| `read` | Read one file with optional offset and line limit. |
| `grep` | Search file contents. |
| `find` | Find files by pattern. |
| `ls` | List directory entries. |
| `edit` | Apply targeted edits to an existing file. |
| `write` | Create or replace a file. |
| `bash` | Run a bounded shell command in the active workspace or task root. |

The wrapper keeps PI's input schemas and behavior as the source of truth. svvy adds durable command records, turn-decision projection, transcript visibility, and artifact links where applicable.

### cx Semantic Navigation Tools

svvy ships the full cx API as native direct tools:

| Tool | Purpose |
| --- | --- |
| `cx_overview` | Inspect a semantic table of contents for a file or directory. |
| `cx_symbols` | Search semantic symbols across the project. |
| `cx_definition` | Read a symbol definition body without reading the full source file. |
| `cx_references` | Find semantic references and callers for a symbol. |
| `cx_lang_list` | List supported cx language grammars and installation state. |
| `cx_lang_add` | Install one or more cx language grammars. |
| `cx_lang_remove` | Remove one or more cx language grammars. |
| `cx_cache_path` | Show the cx cache path for the workspace. |
| `cx_cache_clean` | Clean the cx cache for the workspace. |

Agents use this code-navigation ladder when the target language is supported:

```text
cx_overview -> cx_symbols -> cx_definition / cx_references -> read / grep / find / ls
```

### Artifact Tools

Artifacts are first-class product state, so svvy provides artifact tools directly:

| Tool | Purpose |
| --- | --- |
| `artifact_write_text` | Persist a text artifact. |
| `artifact_write_json` | Persist a JSON artifact. |
| `artifact_attach_file` | Attach an existing file as artifact evidence. |

Artifacts are for durable byproducts and evidence: logs, reports, large outputs, benchmark data, screenshots, workflow exports, or retained generated files that should be inspectable without becoming normal requested repository state.

### Workflow Discovery Tools

Handler threads discover reusable workflow assets and workflow-authoring models through direct workflow tools:

| Tool | Purpose |
| --- | --- |
| `workflow_list_assets` | List saved or artifact workflow definitions, prompts, and components. |
| `workflow_list_models` | List provider/model options available for workflow task-agent authoring. |

Smithers runtime control remains on Smithers-native tools such as `smithers_list_workflows`, `smithers_run_workflow`, `smithers_get_run`, and workflow wait/control tools. Workflow discovery tools only expose source-library metadata and model inventory.

The orchestrator does not receive direct workflow discovery tools, Smithers runtime tools, or `api.workflow_*` through `execute_typescript`. When the orchestrator wants workflow action, it delegates by calling `thread_start`.

Workflow task agents do not receive direct workflow discovery tools, Smithers runtime tools, or `api.workflow_*` through `execute_typescript`. Their workflow context comes from the Smithers task attempt and their task-local instructions, not from workflow-supervision APIs.

### Web Tools

Web access is provided through provider-backed direct tools:

| Tool | Purpose |
| --- | --- |
| `web_search` | Search the public web through the active Web Provider. |
| `web_fetch` | Fetch and extract a known public web page through the active Web Provider, writing fetched content to artifacts. |

The active provider is selected in settings. TinyFish and Firecrawl require their own API keys. By default no provider is selected, so `web_search`, `web_fetch`, and `api.web_*` are absent. The agent-facing schemas are generated from checked-in provider contracts so TinyFish and Firecrawl can each expose the shape that fits that provider best without fetching remote provider docs at runtime.

Detailed behavior is specified in `docs/specs/web-tools.spec.md`.

## Agent-Facing Tool Sets

### Orchestrator

The orchestrator receives cx tools, direct coding tools, direct artifact tools, `execute_typescript`, and orchestration tools such as `thread_start` and `wait`. It uses direct tools for local bounded work and starts handler threads for larger owned work. It does not receive workflow discovery tools, Smithers tools, or `api.workflow_*`.

### Handler

Handler threads receive cx tools, direct coding tools, direct artifact tools, direct workflow discovery tools, `execute_typescript`, `request_context`, `thread_handoff`, wait tools, and Smithers supervision tools. Their `execute_typescript` SDK includes `api.workflow_*` for typed composition over workflow asset discovery and workflow-authoring model lookup. They inspect, edit, author, run, and reconcile workflows through their handler-owned direct and Smithers-native tool surface without relying on code mode as the primary I/O mechanism.

### Workflow Task Agent

Workflow task agents receive task-local cx tools, direct coding tools, direct artifact tools, and `execute_typescript`. Their working directory is the Smithers task root or assigned worktree. They do not receive handler/orchestrator control tools, workflow discovery tools, Smithers supervision tools, or `api.workflow_*`.

## `execute_typescript` Input

```ts
type ExecuteTypescriptInput = {
  typescriptCode: string;
};
```

The snippet body is wrapped as an async function:

```ts
export default async function __svvy(api: SvvyApi, console: SvvyConsole) {
  // user snippet
}
```

The snippet can `return` any JSON-serializable value or a small diagnostic object. The full submitted snippet is stored as an artifact for the attempt.

## Runtime Rules

- The snippet is typechecked before execution against the generated actor-specific `SvvyApi` declaration.
- Node built-ins are not part of the snippet contract.
- The injected `api` object is the only host capability in the snippet contract, and its namespaces must be authorized by the current actor capability profile.
- The injected `console` captures small log lines into the result.
- Each nested `api` call creates a child command under the parent `execute_typescript` command.
- Direct tools remain the preferred path for one-shot reads, edits, writes, and commands.
- Code mode does not expose `edit` or `write`; file modification belongs to direct tools.
- Code mode exposes only read-only cx operations.
- Code mode does not expose Smithers runtime control. Handlers use direct `smithers_*` tools for Smithers supervision, not `api.smithers`.

## Injected API

The source contract is generated from `src/bun/execute-typescript-api-contract.ts` into `generated/execute-typescript-api.generated.ts` and embedded in the system prompt. The generated declaration is actor-specific. `SvvyApi` means the current actor's API shape, not one universal interface shared by all actors.

```ts
interface SvvyConsole {
  log(...args: unknown[]): void;
  info(...args: unknown[]): void;
  warn(...args: unknown[]): void;
  error(...args: unknown[]): void;
}

interface TextContent {
  type: "text";
  text: string;
}

interface ImageContent {
  type: "image";
  data: string;
  mimeType: string;
}

interface ToolResult<TDetails = unknown> {
  content: Array<TextContent | ImageContent>;
  details: TDetails;
}

interface BaseSvvyApi {
  read(input: { path: string; offset?: number; limit?: number }): Promise<ToolResult>;
  grep(input: {
    pattern: string;
    path?: string;
    glob?: string;
    ignoreCase?: boolean;
    literal?: boolean;
    context?: number;
    limit?: number;
  }): Promise<ToolResult>;
  find(input: { pattern: string; path?: string; limit?: number }): Promise<ToolResult>;
  ls(input: { path?: string; limit?: number }): Promise<ToolResult>;
  bash(input: { command: string; timeout?: number }): Promise<ToolResult>;
  cx: {
    overview(input?: CxOverviewInput): Promise<ToolResult<CxCommandDetails>>;
    symbols(input?: CxSymbolsInput): Promise<ToolResult<CxCommandDetails>>;
    definition(input: CxDefinitionInput): Promise<ToolResult<CxCommandDetails>>;
    references(input: CxReferencesInput): Promise<ToolResult<CxCommandDetails>>;
    lang: {
      list(): Promise<ToolResult<CxCommandDetails>>;
    };
    cache: {
      path(): Promise<ToolResult<CxCommandDetails>>;
    };
  };
  artifact: {
    write_text(input: { name: string; text: string }): Promise<ToolResult<ArtifactWriteResult>>;
    write_json(input: {
      name: string;
      value: unknown;
      pretty?: boolean;
    }): Promise<ToolResult<ArtifactWriteResult>>;
    attach_file(input: { path: string; name?: string }): Promise<ToolResult<ArtifactWriteResult>>;
  };
}

type OrchestratorSvvyApi = BaseSvvyApi;

type WorkflowTaskAgentSvvyApi = BaseSvvyApi;

interface HandlerSvvyApi extends BaseSvvyApi {
  workflow: {
    list_assets(input?: WorkflowListAssetsInput): Promise<ToolResult<WorkflowListAssetsDetails>>;
    list_models(): Promise<ToolResult<WorkflowListModelsDetails>>;
  };
}

// The generated prompt declares one concrete alias for the current actor:
// type SvvyApi = OrchestratorSvvyApi;
// type SvvyApi = HandlerSvvyApi;
// type SvvyApi = WorkflowTaskAgentSvvyApi;
```

The code-mode API duplicates these direct tools only:

| Code-mode function | Direct tool duplicated |
| --- | --- |
| `api.read` | `read` |
| `api.grep` | `grep` |
| `api.find` | `find` |
| `api.ls` | `ls` |
| `api.bash` | `bash` |
| `api.cx_overview` | `cx_overview` |
| `api.cx_symbols` | `cx_symbols` |
| `api.cx_definition` | `cx_definition` |
| `api.cx_references` | `cx_references` |
| `api.cx_lang_list` | `cx_lang_list` |
| `api.cx_cache_path` | `cx_cache_path` |
| `api.artifact_write_text` | `artifact_write_text` |
| `api.artifact_write_json` | `artifact_write_json` |
| `api.artifact_attach_file` | `artifact_attach_file` |
| `api.workflow_list_assets` | `workflow_list_assets`, handler only |
| `api.workflow_list_models` | `workflow_list_models`, handler only |
| `api.web_search` | `web_search`, only when a keyed web provider is ready |
| `api.web_fetch` | `web_fetch`, only when a keyed web provider is ready |

`edit`, `write`, `cx_lang_add`, `cx_lang_remove`, and `cx_cache_clean` are not duplicated inside code mode. Agents call those tools directly so modifications to repository or cx runtime state stay explicit in the transcript and command stream.

`api.workflow_*` is generated only for handler-thread actors. It is a typed composition surface for workflow asset discovery and workflow-authoring model lookup. It is not a Smithers runtime-control surface, and it is absent for orchestrators and workflow task agents.

`api.web_*` helpers are an optional code-mode API block. It is generated only when the active Web Provider is selected and ready with its API key. Its concrete input and output types are generated from the active provider's checked-in direct-tool contracts. Changing providers or key state regenerates the `api.web_*` declaration before the next turn. It is meant for batching, filtering, aggregation, and artifact evidence over multiple independent searches or fetches. One-shot web lookups should use the direct `web_search` or `web_fetch` tools. `api.web_fetch` follows the same deterministic artifact-backed behavior as direct `web_fetch`: fetched page bodies are written to artifacts and the result returns artifact references. If no provider is ready, `api.web_*` helpers are absent and snippets that reference it fail typecheck.

## Examples

### Batch Reads and Aggregate a Result

```ts
const matches = await api.grep({ pattern: "StructuredTurnDecision", glob: "src/**/*.ts" });
const files = matches.content
  .filter((entry) => entry.type === "text")
  .flatMap((entry) => entry.text.match(/[^\s:]+\.ts/g) ?? []);
const uniqueFiles = [...new Set(files)].slice(0, 5);
const reads = await Promise.all(uniqueFiles.map((path) => api.read({ path, limit: 80 })));

return {
  files: uniqueFiles,
  previews: reads.map((result) =>
    result.content.filter((entry) => entry.type === "text").map((entry) => entry.text).join("\n"),
  ),
};
```

### Handler Workflow Discovery

This example is valid only in a handler-thread `execute_typescript` turn.

```ts
const prompts = await api.workflow_list_assets({ kind: "prompt", scope: "saved" });
const components = await api.workflow_list_assets({ kind: "component", scope: "saved" });
const models = await api.workflow_list_models();

return {
  promptCount: prompts.details.assets.length,
  componentCount: components.details.assets.length,
  modelCount: models.details.models.length,
};
```

### Semantic Navigation Batch

```ts
const overview = await api.cx_overview({ path: "src/bun" });
const symbols = await api.cx_symbols({ kind: "function", name: "create*" });
const definitions = await Promise.all(
  symbols.details.json && Array.isArray(symbols.details.json)
    ? symbols.details.json
        .slice(0, 3)
        .map((entry: any) => api.cx_definition({ name: String(entry.name), from: entry.file }))
    : [],
);

return {
  overviewCommand: overview.details.command,
  definitionCount: definitions.length,
};
```

### Bash-Backed Inspection

```ts
const result = await api.bash({ command: "bun test src/bun/execute-typescript-tool.test.ts" });
const output = result.content
  .filter((entry) => entry.type === "text")
  .map((entry) => entry.text)
  .join("\n");

await api.artifact_write_text({
  name: "execute-typescript-test-output.txt",
  text: output,
});

return { outputBytes: output.length };
```

## Recording

Every direct tool call and every nested code-mode call is recorded as a structured command.

Direct tool command records include:

- tool name
- owning turn or workflow task attempt
- status
- visibility
- title and summary
- tool input and selected result facts when available

`execute_typescript` command records include:

- parent command for the snippet attempt
- submitted snippet artifact
- diagnostics artifact when typecheck fails
- logs artifact when console output exists
- child command records for nested `api` calls
- parent rollup facts for reads, searches, bash calls, artifacts, and workflow discovery

## Visibility

Read/search/discovery calls are trace-visible by default. File modifications, artifact creation, bash commands, failures, waits, handoffs, Smithers control, and `execute_typescript` parents are summary-visible by default.

## Failure Semantics

Typecheck failures stop execution before any nested call runs. Runtime failures finish the parent command as failed, preserve successful child command records that already occurred, and attach the thrown error to the result.

Nested failures finish their child command as failed. The snippet may catch and handle the error; otherwise the parent fails.

## Prompt Exposure

The agent receives:

- ordinary tool declarations for direct tools
- ordinary tool declarations for control and Smithers tools on surfaces that own them
- one `execute_typescript` tool declaration
- a generated TypeScript declaration block for the injected code-mode API
- prompt guidance that direct tools are the default path and code mode is for typed composition

The generated declaration is the prompt contract. Handwritten prose may explain when to use code mode, but it does not redefine the interface.
