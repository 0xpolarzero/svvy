import type { AgentTool, AgentToolResult } from "@mariozechner/pi-agent-core";
import { Type } from "@mariozechner/pi-ai";
import type { Static, TSchema } from "typebox";
import { mkdirSync } from "node:fs";
import { join } from "node:path";

const overviewSchema = Type.Object(
  {
    path: Type.Optional(Type.String({ minLength: 1 })),
    full: Type.Optional(Type.Boolean()),
    root: Type.Optional(Type.String({ minLength: 1 })),
  },
  { additionalProperties: false },
);

const symbolsSchema = Type.Object(
  {
    kind: Type.Optional(Type.String({ minLength: 1 })),
    name: Type.Optional(Type.String({ minLength: 1 })),
    file: Type.Optional(Type.String({ minLength: 1 })),
    limit: Type.Optional(Type.Integer({ minimum: 1 })),
    offset: Type.Optional(Type.Integer({ minimum: 0 })),
    all: Type.Optional(Type.Boolean()),
    root: Type.Optional(Type.String({ minLength: 1 })),
  },
  { additionalProperties: false },
);

const definitionSchema = Type.Object(
  {
    name: Type.String({ minLength: 1 }),
    kind: Type.Optional(Type.String({ minLength: 1 })),
    from: Type.Optional(Type.String({ minLength: 1 })),
    maxLines: Type.Optional(Type.Integer({ minimum: 1 })),
    limit: Type.Optional(Type.Integer({ minimum: 1 })),
    offset: Type.Optional(Type.Integer({ minimum: 0 })),
    all: Type.Optional(Type.Boolean()),
    root: Type.Optional(Type.String({ minLength: 1 })),
  },
  { additionalProperties: false },
);

const referencesSchema = Type.Object(
  {
    name: Type.String({ minLength: 1 }),
    file: Type.Optional(Type.String({ minLength: 1 })),
    unique: Type.Optional(Type.Boolean()),
    limit: Type.Optional(Type.Integer({ minimum: 1 })),
    offset: Type.Optional(Type.Integer({ minimum: 0 })),
    all: Type.Optional(Type.Boolean()),
    root: Type.Optional(Type.String({ minLength: 1 })),
  },
  { additionalProperties: false },
);

const emptySchema = Type.Object({}, { additionalProperties: false });

const languageMutationSchema = Type.Object(
  {
    languages: Type.Array(Type.String({ minLength: 1 }), { minItems: 1 }),
  },
  { additionalProperties: false },
);

export type CxOverviewInput = Static<typeof overviewSchema>;
export type CxSymbolsInput = Static<typeof symbolsSchema>;
export type CxDefinitionInput = Static<typeof definitionSchema>;
export type CxReferencesInput = Static<typeof referencesSchema>;

type CxToolOptions = {
  cwd: string;
  runner?: CxRunner;
};

export type CxCommandResult = {
  command: string[];
  exitCode: number;
  stdout: string;
  stderr: string;
  json: unknown | null;
};

export type CxRunner = (input: {
  cwd: string;
  args: string[];
  signal?: AbortSignal;
}) => Promise<CxCommandResult>;

export function createCxTools(options: CxToolOptions): AgentTool<any>[] {
  return [
    createCxTool({
      name: "cx_overview",
      label: "cx_overview",
      description: "Inspect a semantic table of contents for a file or directory.",
      parameters: overviewSchema,
      options,
      buildArgs: (input: CxOverviewInput) => {
        const args = ["overview", input.path?.trim() || "."];
        pushFlag(args, "--full", input.full);
        pushOption(args, "--root", input.root);
        args.push("--json");
        return args;
      },
    }),
    createCxTool({
      name: "cx_symbols",
      label: "cx_symbols",
      description: "Search semantic symbols across the project.",
      parameters: symbolsSchema,
      options,
      buildArgs: (input: CxSymbolsInput) => {
        const args = ["symbols"];
        pushOption(args, "--kind", input.kind);
        pushOption(args, "--name", input.name);
        pushOption(args, "--file", input.file);
        pushPagination(args, input);
        pushOption(args, "--root", input.root);
        args.push("--json");
        return args;
      },
    }),
    createCxTool({
      name: "cx_definition",
      label: "cx_definition",
      description: "Read a symbol definition body without reading the full source file.",
      parameters: definitionSchema,
      options,
      buildArgs: (input: CxDefinitionInput) => {
        const args = ["definition", "--name", input.name];
        pushOption(args, "--kind", input.kind);
        pushOption(args, "--from", input.from);
        pushOption(args, "--max-lines", input.maxLines);
        pushPagination(args, input);
        pushOption(args, "--root", input.root);
        args.push("--json");
        return args;
      },
    }),
    createCxTool({
      name: "cx_references",
      label: "cx_references",
      description: "Find semantic references and callers for a symbol.",
      parameters: referencesSchema,
      options,
      buildArgs: (input: CxReferencesInput) => {
        const args = ["references", "--name", input.name];
        pushOption(args, "--file", input.file);
        pushFlag(args, "--unique", input.unique);
        pushPagination(args, input);
        pushOption(args, "--root", input.root);
        args.push("--json");
        return args;
      },
    }),
    createCxTool({
      name: "cx_lang_list",
      label: "cx_lang_list",
      description: "List supported cx language grammars and installation state.",
      parameters: emptySchema,
      options,
      buildArgs: () => ["lang", "list"],
    }),
    createCxTool({
      name: "cx_lang_add",
      label: "cx_lang_add",
      description: "Install one or more cx language grammars.",
      parameters: languageMutationSchema,
      options,
      buildArgs: (input: Static<typeof languageMutationSchema>) => [
        "lang",
        "add",
        ...input.languages,
      ],
    }),
    createCxTool({
      name: "cx_lang_remove",
      label: "cx_lang_remove",
      description: "Remove one or more cx language grammars.",
      parameters: languageMutationSchema,
      options,
      buildArgs: (input: Static<typeof languageMutationSchema>) => [
        "lang",
        "remove",
        ...input.languages,
      ],
    }),
    createCxTool({
      name: "cx_cache_path",
      label: "cx_cache_path",
      description: "Show the cx cache path used for this workspace.",
      parameters: emptySchema,
      options,
      buildArgs: () => ["cache", "path"],
    }),
    createCxTool({
      name: "cx_cache_clean",
      label: "cx_cache_clean",
      description: "Clean the cx cache for this workspace.",
      parameters: emptySchema,
      options,
      buildArgs: () => ["cache", "clean"],
    }),
  ];
}

function createCxTool<TToolSchema extends TSchema>(input: {
  name: string;
  label: string;
  description: string;
  parameters: TToolSchema;
  options: CxToolOptions;
  buildArgs: (params: Static<TToolSchema>) => string[];
}): AgentTool<TToolSchema> {
  return {
    name: input.name,
    label: input.label,
    description: input.description,
    parameters: input.parameters,
    async execute(_toolCallId, params, signal) {
      const runner = input.options.runner ?? runCxCommand;
      const result = await runner({
        cwd: input.options.cwd,
        args: input.buildArgs(params),
        signal,
      });
      if (result.exitCode !== 0) {
        throw new Error(result.stderr || result.stdout || `${input.name} failed.`);
      }
      return cxToolResult(result);
    },
  };
}

function pushOption(args: string[], name: string, value: string | number | undefined): void {
  if (value === undefined) {
    return;
  }
  const normalized = typeof value === "string" ? value.trim() : String(value);
  if (!normalized) {
    return;
  }
  args.push(name, normalized);
}

function pushFlag(args: string[], name: string, value: boolean | undefined): void {
  if (value) {
    args.push(name);
  }
}

function pushPagination(
  args: string[],
  input: { limit?: number; offset?: number; all?: boolean },
): void {
  pushOption(args, "--limit", input.limit);
  pushOption(args, "--offset", input.offset);
  pushFlag(args, "--all", input.all);
}

async function runCxCommand(input: {
  cwd: string;
  args: string[];
  signal?: AbortSignal;
}): Promise<CxCommandResult> {
  const cxBin = process.env.SVVY_CX_BIN || Bun.which("cx");
  if (!cxBin) {
    throw new Error("cx runtime binary not found for cx_* tools.");
  }
  const cacheDir = process.env.CX_CACHE_DIR || join(input.cwd, ".svvy", "cx-cache");
  mkdirSync(cacheDir, { recursive: true });
  const proc = Bun.spawn([cxBin, ...input.args], {
    cwd: input.cwd,
    stdout: "pipe",
    stderr: "pipe",
    signal: input.signal,
    env: {
      ...process.env,
      CX_CACHE_DIR: cacheDir,
    },
  });
  const [stdout, stderr, exitCode] = await Promise.all([
    new Response(proc.stdout).text(),
    new Response(proc.stderr).text(),
    proc.exited,
  ]);
  return {
    command: ["cx", ...input.args],
    exitCode,
    stdout,
    stderr,
    json: parseJson(stdout),
  };
}

function parseJson(text: string): unknown | null {
  const trimmed = text.trim();
  if (!trimmed) {
    return null;
  }
  try {
    return JSON.parse(trimmed);
  } catch {
    return null;
  }
}

function cxToolResult(result: CxCommandResult): AgentToolResult<CxCommandResult> {
  return {
    content: [
      {
        type: "text",
        text: result.stdout.trim() || result.stderr.trim() || "cx completed successfully.",
      },
    ],
    details: result,
  };
}
