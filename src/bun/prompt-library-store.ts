import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { randomUUID } from "node:crypto";
import { getPromptLibraryContentKey } from "../shared/prompt-library";
import type {
  PromptLibraryActor,
  PromptLibraryActorRecipe,
  PromptLibraryContextPack,
  PromptLibraryGeneratedSectionId,
  PromptLibraryInstructionBlock,
  PromptLibrarySnapshotSummary,
  PromptLibraryState,
} from "../shared/prompt-library";
import { createDefaultPromptLibraryState } from "./default-system-prompt";

export type PromptLibraryStore = {
  getState(): PromptLibraryState;
  updateState(state: PromptLibraryState): PromptLibraryState;
  resetState(): PromptLibraryState;
  listSnapshots(): PromptLibrarySnapshotSummary[];
  createSnapshot(name: string): PromptLibrarySnapshotSummary;
  renameSnapshot(snapshotId: string, name: string): PromptLibrarySnapshotSummary;
  restoreSnapshot(snapshotId: string): PromptLibraryState;
  getPath(): string;
};

const PROMPT_LIBRARY_FILENAME = "prompt-library.json";
const PROMPT_LIBRARY_SNAPSHOTS_FILENAME = "prompt-library-snapshots.json";
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
  const snapshotsPath = join(input.agentDir, PROMPT_LIBRARY_SNAPSHOTS_FILENAME);

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

  const writeSnapshots = (snapshots: PromptLibrarySnapshot[]): PromptLibrarySnapshot[] => {
    mkdirSync(dirname(snapshotsPath), { recursive: true });
    writeFileSync(snapshotsPath, `${JSON.stringify({ version: 1, snapshots }, null, 2)}\n`);
    return snapshots;
  };

  const readSnapshots = (): PromptLibrarySnapshot[] => {
    if (!existsSync(snapshotsPath)) {
      return [];
    }
    try {
      const parsed = JSON.parse(readFileSync(snapshotsPath, "utf8")) as Partial<{
        snapshots: unknown[];
      }>;
      return normalizeSnapshots(parsed.snapshots);
    } catch {
      return writeSnapshots([]);
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
    listSnapshots: () => summarizeSnapshots(readSnapshots()),
    createSnapshot: (name) => {
      const snapshots = readSnapshots();
      const state = readState();
      const snapshot = normalizeSnapshot({
        id: randomUUID(),
        name: normalizeSnapshotName(name, state.updatedAt),
        createdAt: new Date().toISOString(),
        state,
      });
      if (!snapshot) {
        throw new Error("Unable to create prompt library snapshot.");
      }
      writeSnapshots([snapshot, ...snapshots]);
      return summarizeSnapshot(snapshot);
    },
    renameSnapshot: (snapshotId, name) => {
      const snapshots = readSnapshots();
      const index = snapshots.findIndex((snapshot) => snapshot.id === snapshotId);
      if (index === -1) {
        throw new Error("Prompt library snapshot not found.");
      }
      const snapshot = {
        ...snapshots[index]!,
        name: normalizeSnapshotName(name, snapshots[index]!.createdAt),
      };
      snapshots[index] = snapshot;
      writeSnapshots(snapshots);
      return summarizeSnapshot(snapshot);
    },
    restoreSnapshot: (snapshotId) => {
      const snapshots = readSnapshots();
      const snapshot = snapshots.find((candidate) => candidate.id === snapshotId);
      if (!snapshot) {
        throw new Error("Prompt library snapshot not found.");
      }
      const current = readState();
      const restored = normalizePromptLibraryState(snapshot.state, {
        revision: current.revision + 1,
        updatedAt: new Date().toISOString(),
      });
      return writeState(restored);
    },
    getPath: () => libraryPath,
  };
}

type PromptLibrarySnapshot = {
  id: string;
  name: string;
  createdAt: string;
  state: PromptLibraryState;
};

function normalizeSnapshots(input: unknown[] | undefined): PromptLibrarySnapshot[] {
  return (input ?? [])
    .map((entry) => normalizeSnapshot(entry))
    .filter((entry): entry is PromptLibrarySnapshot => Boolean(entry))
    .toSorted((left, right) => right.createdAt.localeCompare(left.createdAt));
}

function normalizeSnapshot(input: unknown): PromptLibrarySnapshot | null {
  if (!input || typeof input !== "object") {
    return null;
  }
  const record = input as Partial<PromptLibrarySnapshot>;
  const id = typeof record.id === "string" ? record.id.trim() : "";
  if (!id || !record.state) {
    return null;
  }
  const createdAt = normalizeTimestamp(record.createdAt, new Date().toISOString());
  const state = normalizePromptLibraryState(record.state);
  return {
    id,
    name: normalizeSnapshotName(record.name, createdAt),
    createdAt,
    state,
  };
}

function summarizeSnapshots(
  snapshots: readonly PromptLibrarySnapshot[],
): PromptLibrarySnapshotSummary[] {
  return snapshots.map(summarizeSnapshot);
}

function summarizeSnapshot(snapshot: PromptLibrarySnapshot): PromptLibrarySnapshotSummary {
  return {
    id: snapshot.id,
    name: snapshot.name,
    createdAt: snapshot.createdAt,
    revision: snapshot.state.revision,
    contentKey: getPromptLibraryContentKey(snapshot.state),
  };
}

function normalizeSnapshotName(value: unknown, createdAt: string): string {
  if (typeof value === "string" && value.trim()) {
    return value.trim();
  }
  return `Snapshot ${new Intl.DateTimeFormat("en", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(createdAt))}`;
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
