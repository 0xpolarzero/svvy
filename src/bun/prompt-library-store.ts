import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import type {
  PromptLibraryActor,
  PromptLibraryActorRecipe,
  PromptLibraryContextPack,
  PromptLibraryGeneratedSectionId,
  PromptLibraryInstructionBlock,
  PromptLibraryState,
} from "../shared/prompt-library";
import { createDefaultPromptLibraryState } from "./default-system-prompt";

export type PromptLibraryStore = {
  getState(): PromptLibraryState;
  updateState(state: PromptLibraryState): PromptLibraryState;
  resetState(): PromptLibraryState;
  getPath(): string;
};

const PROMPT_LIBRARY_FILENAME = "prompt-library.json";
const ACTORS: PromptLibraryActor[] = ["orchestrator", "handler", "workflow-task"];
const GENERATED_SECTION_IDS: PromptLibraryGeneratedSectionId[] = [
  "web-context",
  "workflow-authoring-contract",
  "handler-workflow-authoring-appendix",
  "loaded-optional-context",
  "execute-typescript",
];

export function createPromptLibraryStore(input: { agentDir: string }): PromptLibraryStore {
  const libraryPath = join(input.agentDir, PROMPT_LIBRARY_FILENAME);

  const writeState = (state: PromptLibraryState): PromptLibraryState => {
    mkdirSync(dirname(libraryPath), { recursive: true });
    writeFileSync(libraryPath, `${JSON.stringify(state, null, 2)}\n`);
    return state;
  };

  const readState = (): PromptLibraryState => {
    if (!existsSync(libraryPath)) {
      return writeState(createDefaultPromptLibraryState());
    }
    try {
      return normalizePromptLibraryState(
        JSON.parse(readFileSync(libraryPath, "utf8")) as Partial<PromptLibraryState>,
      );
    } catch {
      return writeState(createDefaultPromptLibraryState());
    }
  };

  return {
    getState: readState,
    updateState: (state) => {
      const current = readState();
      const normalized = normalizePromptLibraryState(state, {
        revision: current.revision + 1,
        updatedAt: new Date().toISOString(),
      });
      return writeState(normalized);
    },
    resetState: () => {
      const current = readState();
      return writeState(
        createDefaultPromptLibraryState(new Date().toISOString(), current.revision + 1),
      );
    },
    getPath: () => libraryPath,
  };
}

export function normalizePromptLibraryState(
  input: Partial<PromptLibraryState>,
  overrides: Partial<Pick<PromptLibraryState, "revision" | "updatedAt">> = {},
): PromptLibraryState {
  const defaults = createDefaultPromptLibraryState();
  const instructionBlocks = normalizeInstructionBlocks(input.instructionBlocks, defaults);
  const contextPacks = normalizeContextPacks(input.contextPacks, defaults);
  const actorRecipes = normalizeActorRecipes(input.actorRecipes, defaults);
  return {
    version: 1,
    revision: normalizePositiveInteger(overrides.revision ?? input.revision, defaults.revision),
    updatedAt: normalizeTimestamp(overrides.updatedAt ?? input.updatedAt, defaults.updatedAt),
    instructionBlocks,
    contextPacks,
    actorRecipes,
  };
}

function normalizeInstructionBlocks(
  input: PromptLibraryState["instructionBlocks"] | undefined,
  defaults: PromptLibraryState,
): PromptLibraryState["instructionBlocks"] {
  const source = input ?? defaults.instructionBlocks;
  const output: PromptLibraryState["instructionBlocks"] = {};
  for (const [id, block] of Object.entries(source)) {
    const normalized = normalizeInstructionBlock(id, block);
    if (normalized) {
      output[normalized.id] = normalized;
    }
  }
  return output;
}

function normalizeInstructionBlock(
  fallbackId: string,
  input: PromptLibraryInstructionBlock,
): PromptLibraryInstructionBlock | null {
  const id = normalizeIdentifier(input.id || fallbackId);
  const actor = input.actor === "common" || ACTORS.includes(input.actor) ? input.actor : "common";
  const title = requireText(input.title, id);
  const body = requireText(input.body, "");
  if (!id || !title || !body) {
    return null;
  }
  return {
    id,
    title,
    summary: input.summary?.trim() ?? "",
    body,
    enabled: input.enabled !== false,
    scope: normalizeScope(input.scope),
    actor,
    default: input.default === true,
  };
}

function normalizeContextPacks(
  input: PromptLibraryState["contextPacks"] | undefined,
  defaults: PromptLibraryState,
): PromptLibraryState["contextPacks"] {
  const source = input ?? defaults.contextPacks;
  const output: PromptLibraryState["contextPacks"] = {};
  for (const [id, pack] of Object.entries(source)) {
    const normalized = normalizeContextPack(id, pack);
    if (normalized) {
      output[normalized.id] = normalized;
    }
  }
  return output;
}

function normalizeContextPack(
  fallbackId: string,
  input: PromptLibraryContextPack,
): PromptLibraryContextPack | null {
  const id = normalizeIdentifier(input.id || fallbackId);
  const title = requireText(input.title, id);
  const body = requireText(input.body, "");
  if (!id || !title || !body) {
    return null;
  }
  const allowedActors = (input.allowedActors ?? []).filter((actor) => ACTORS.includes(actor));
  return {
    id,
    title,
    summary: input.summary?.trim() ?? "",
    body,
    enabled: input.enabled !== false,
    scope: normalizeScope(input.scope),
    allowedActors: allowedActors.length > 0 ? allowedActors : ACTORS,
    default: input.default === true,
    optionalContextKey: input.optionalContextKey?.trim() || undefined,
  };
}

function normalizeActorRecipes(
  input: PromptLibraryState["actorRecipes"] | undefined,
  defaults: PromptLibraryState,
): PromptLibraryState["actorRecipes"] {
  const source = input ?? defaults.actorRecipes;
  const output = {} as PromptLibraryState["actorRecipes"];
  for (const actor of ACTORS) {
    output[actor] = normalizeActorRecipe(actor, source[actor], defaults.actorRecipes[actor]);
  }
  return output;
}

function normalizeActorRecipe(
  actor: PromptLibraryActor,
  input: PromptLibraryActorRecipe | undefined,
  fallback: PromptLibraryActorRecipe,
): PromptLibraryActorRecipe {
  return {
    actor,
    instructionBlockIds: normalizeIdList(input?.instructionBlockIds, fallback.instructionBlockIds),
    contextPackIds: normalizeIdList(input?.contextPackIds, fallback.contextPackIds),
    generatedSectionIds: normalizeGeneratedSectionIds(
      input?.generatedSectionIds,
      fallback.generatedSectionIds,
    ),
  };
}

function normalizeGeneratedSectionIds(
  input: readonly string[] | undefined,
  fallback: readonly PromptLibraryGeneratedSectionId[],
): PromptLibraryGeneratedSectionId[] {
  const sections = (input ?? fallback).filter(
    (section): section is PromptLibraryGeneratedSectionId =>
      GENERATED_SECTION_IDS.includes(section as PromptLibraryGeneratedSectionId),
  );
  return sections.length > 0 ? [...new Set(sections)] : [...fallback];
}

function normalizeIdList(
  input: readonly string[] | undefined,
  fallback: readonly string[],
): string[] {
  const ids = (input ?? fallback).map(normalizeIdentifier).filter(Boolean);
  return ids.length > 0 ? [...new Set(ids)] : [...fallback];
}

function normalizeIdentifier(value: string): string {
  return value.trim();
}

function requireText(value: string, fallback: string): string {
  return value?.trim() || fallback;
}

function normalizeScope(input: PromptLibraryInstructionBlock["scope"] | undefined) {
  return {
    appGlobal: input?.appGlobal !== false,
    workspaceKeys: Array.isArray(input?.workspaceKeys)
      ? [...new Set(input.workspaceKeys.map((key) => key.trim()).filter(Boolean))]
      : [],
  };
}

function normalizePositiveInteger(value: unknown, fallback: number): number {
  const parsed = Number(value);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : fallback;
}

function normalizeTimestamp(value: unknown, fallback: string): string {
  if (typeof value !== "string" || !value.trim()) {
    return fallback;
  }
  return Number.isNaN(new Date(value).getTime()) ? fallback : value;
}
