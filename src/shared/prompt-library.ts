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
