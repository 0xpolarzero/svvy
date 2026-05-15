export type PromptLibraryActor = "orchestrator" | "handler" | "workflow-task";

export type PromptLibraryGeneratedSectionId =
  | "web-context"
  | "workflow-authoring-contract"
  | "handler-workflow-authoring-appendix"
  | "loaded-optional-context"
  | "execute-typescript";

export interface PromptLibraryGeneratedEntry {
  id: PromptLibraryGeneratedSectionId;
  title: string;
  source: string;
  sourcePath: string;
  content: string;
}

export interface PromptLibraryExternalSource {
  id: string;
  kind: "AGENTS.md" | "CLAUDE.md";
  title: string;
  path: string;
  content: string;
  contentHash: string;
  order: number;
}

export interface PromptLibraryScope {
  appGlobal: boolean;
  workspaceKeys: string[];
}

export interface PromptLibraryInstructionBlock {
  id: string;
  title: string;
  summary: string;
  body: string;
  enabled: boolean;
  scope: PromptLibraryScope;
  actor: PromptLibraryActor | "common";
  default: boolean;
}

export interface PromptLibraryContextPack {
  id: string;
  title: string;
  summary: string;
  body: string;
  enabled: boolean;
  scope: PromptLibraryScope;
  allowedActors: PromptLibraryActor[];
  default: boolean;
  optionalContextKey?: string;
}

export interface PromptLibraryActorRecipe {
  actor: PromptLibraryActor;
  instructionBlockIds: string[];
  contextPackIds: string[];
  generatedSectionIds: PromptLibraryGeneratedSectionId[];
}

export interface PromptLibraryState {
  version: 1;
  revision: number;
  updatedAt: string;
  instructionBlocks: Record<string, PromptLibraryInstructionBlock>;
  contextPacks: Record<string, PromptLibraryContextPack>;
  actorRecipes: Record<PromptLibraryActor, PromptLibraryActorRecipe>;
}

export interface UpdatePromptLibraryRequest {
  state: PromptLibraryState;
}

export interface PromptLibrarySnapshotSummary {
  id: string;
  name: string;
  createdAt: string;
  revision: number;
  contentKey: string;
}

export interface CreatePromptLibrarySnapshotRequest {
  name: string;
}

export interface RenamePromptLibrarySnapshotRequest {
  snapshotId: string;
  name: string;
}

export interface RestorePromptLibrarySnapshotRequest {
  snapshotId: string;
}

export function getPromptLibraryContentKey(state: PromptLibraryState): string {
  return JSON.stringify(
    sortPromptLibraryValue({
      version: state.version,
      instructionBlocks: state.instructionBlocks,
      contextPacks: state.contextPacks,
      actorRecipes: state.actorRecipes,
    }),
  );
}

function sortPromptLibraryValue(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map(sortPromptLibraryValue);
  }
  if (!value || typeof value !== "object") {
    return value;
  }
  return Object.fromEntries(
    Object.entries(value)
      .toSorted(([left], [right]) => left.localeCompare(right))
      .map(([key, entry]) => [key, sortPromptLibraryValue(entry)]),
  );
}
