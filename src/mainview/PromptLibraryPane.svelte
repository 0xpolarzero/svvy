<script lang="ts">
  import ExternalLinkIcon from "@lucide/svelte/icons/external-link";
  import PlusIcon from "@lucide/svelte/icons/plus";
  import RotateCcwIcon from "@lucide/svelte/icons/rotate-ccw";
  import Trash2Icon from "@lucide/svelte/icons/trash-2";
  import { onMount } from "svelte";
  import type { ChatRuntime } from "./chat-runtime";
  import Badge from "./ui/Badge.svelte";
  import Button from "./ui/Button.svelte";
  import Checkbox from "./ui/Checkbox.svelte";
  import CompactCombobox, { type CompactComboboxOption } from "./ui/CompactCombobox.svelte";
  import Dialog from "./ui/Dialog.svelte";
  import Input from "./ui/Input.svelte";
  import TextArea from "./ui/TextArea.svelte";
  import Tooltip from "./ui/Tooltip.svelte";
  import type {
    PromptLibraryActor,
    PromptLibraryContextPack,
    PromptLibraryGeneratedEntry,
    PromptLibraryInstructionBlock,
    PromptLibraryState,
  } from "../shared/prompt-library";
  import type { WorkspaceTabInfo } from "../shared/workspace-contract";

  type PromptLibrarySection = "instructions" | "contextPacks";
  type PromptLibraryActorKey = PromptLibraryActor;
  type PromptLibraryBlockState = "builtin" | "edited" | "custom" | "generated";

  type PromptLibraryBlock = {
    id: string;
    section: PromptLibrarySection;
    title: string;
    summary: string;
    content: string;
    actorKeys: PromptLibraryActorKey[];
    enabled: boolean;
    appGlobal: boolean;
    workspaceKeys: string[];
    deletable: boolean;
    state: PromptLibraryBlockState;
    generatedEntries?: PromptLibraryGeneratedEntry[];
  };

  type PromptLibraryActorAggregate = {
    id: PromptLibraryActorKey;
    label: string;
    summary: string;
    blockIds: string[];
    generatedEntries: PromptLibraryGeneratedEntry[];
  };

  type PromptLibraryReadModel = {
    updatedAt: string;
    sections: Record<PromptLibrarySection, PromptLibraryBlock[]>;
    actors: PromptLibraryActorAggregate[];
  };
  type AutosaveStatus = "idle" | "dirty" | "saving" | "saved" | "error";
  type DraftSnapshot = {
    blockId: string;
    section: PromptLibrarySection;
    title: string;
    content: string;
    enabled: boolean;
    appGlobal: boolean;
    workspaceKeys: string[];
    actorKeys: PromptLibraryActorKey[];
  };

  type Props = {
    runtime: ChatRuntime;
    panelId: string;
  };

  let { runtime, panelId }: Props = $props();

  const SECTION_LABELS: Record<PromptLibrarySection, string> = {
    instructions: "Instructions",
    contextPacks: "Context Packs",
  };
  const BACKGROUND_REFRESH_INTERVAL_MS = 5000;

  let readModel = $state<PromptLibraryReadModel | null>(null);
  let promptState = $state<PromptLibraryState | null>(null);
  let defaultPromptState = $state<PromptLibraryState | null>(null);
  let generatedEntriesByActor = $state<Record<
    PromptLibraryActorKey,
    PromptLibraryGeneratedEntry[]
  > | null>(null);
  let selectedKind = $state<"block" | "actor">("block");
  let selectedId = $state<string | null>(null);
  let draftTitle = $state("");
  let draftContent = $state("");
  let draftEnabled = $state(true);
  let draftAppGlobal = $state(true);
  let draftWorkspaceKeys = $state<string[]>([]);
  let draftActorKeys = $state<PromptLibraryActorKey[]>([]);
  let workspaceScopeOptions = $state<CompactComboboxOption[]>([]);
  let loading = $state(true);
  let saving = $state(false);
  let autosaveStatus = $state<AutosaveStatus>("idle");
  let lastSavedAt = $state<string | null>(null);
  let autosaveTimer: ReturnType<typeof setTimeout> | null = null;
  let instantSaveQueue: Promise<void> = Promise.resolve();
  let error = $state<string | null>(null);
  let actionMessage = $state<string | null>(null);
  let resetCandidate = $state<PromptLibraryBlock | null>(null);

  const allBlocks = $derived([
    ...(readModel?.sections.instructions ?? []),
    ...(readModel?.sections.contextPacks ?? []),
  ]);
  const selectedBlock = $derived(
    selectedKind === "block" ? (allBlocks.find((block) => block.id === selectedId) ?? null) : null,
  );
  const selectedActor = $derived(
    selectedKind === "actor"
      ? (readModel?.actors.find((actor) => actor.id === selectedId) ?? null)
      : null,
  );
  const actorBlocks = $derived(
    selectedActor ? allBlocks.filter((block) => selectedActor.blockIds.includes(block.id)) : [],
  );
  const isDirty = $derived(
    !!selectedBlock &&
      (draftTitle !== selectedBlock.title ||
        draftContent !== selectedBlock.content ||
        draftEnabled !== selectedBlock.enabled ||
        draftAppGlobal !== selectedBlock.appGlobal ||
        draftWorkspaceKeys.join("|") !== selectedBlock.workspaceKeys.join("|") ||
        draftActorKeys.join("|") !== selectedBlock.actorKeys.join("|")),
  );
  const isTextDirty = $derived(
    !!selectedBlock && (draftTitle !== selectedBlock.title || draftContent !== selectedBlock.content),
  );
  const saveStatusLabel = $derived(getSaveStatusLabel());
  function clonePromptLibraryState(state: PromptLibraryState): PromptLibraryState {
    return JSON.parse(JSON.stringify(state)) as PromptLibraryState;
  }

  function getSaveStatusLabel(): string {
    if (autosaveStatus === "saving") return "Saving...";
    if (autosaveStatus === "dirty") return "Unsaved changes";
    if (autosaveStatus === "error") return "Save failed";
    if (lastSavedAt) return `Saved ${new Intl.DateTimeFormat(undefined, {
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
    }).format(new Date(lastSavedAt))}`;
    return "Saved";
  }

  function blockKindLabel(block: PromptLibraryBlock): string {
    return block.section === "instructions" ? "instruction" : "context pack";
  }

  function resetBlockLabel(block: PromptLibraryBlock): string {
    return `Reset to the builtin ${blockKindLabel(block)}`;
  }

  function deleteBlockLabel(block: PromptLibraryBlock): string {
    if (!canDeleteBlock(block)) {
      return `Builtin ${blockKindLabel(block)}s cannot be deleted`;
    }
    return `Delete ${blockKindLabel(block)}`;
  }

  function enabledCheckboxTooltipFor(enabled: boolean): string {
    return enabled
      ? "Disable this block so it is not injected into the system prompt"
      : "Enable this block so it is injected into the system prompt";
  }

  function actorControlCaption(block: PromptLibraryBlock): string {
    return block.section === "instructions"
      ? "Injected into the system prompt for selected actors."
      : "Always loaded for selected actors. Other actors can still request it when needed.";
  }

  const workspaceScopeOptionsWithSelected = $derived.by<CompactComboboxOption[]>(() => {
    const optionsByValue = new Map(workspaceScopeOptions.map((option) => [option.value, option]));
    for (const key of draftWorkspaceKeys) {
      if (!optionsByValue.has(key)) {
        optionsByValue.set(key, {
          value: key,
          label: workspaceLabelFromKey(key),
          triggerLabel: workspaceLabelFromKey(key),
          searchText: key,
        });
      }
    }
    return [...optionsByValue.values()];
  });

  function scopeControlCaption(appGlobal: boolean, workspaceKeys: readonly string[]): string {
    if (appGlobal) return "Applies in every workspace.";
    const count = workspaceKeys.length;
    return `Applies in ${count} ${count === 1 ? "workspace" : "workspaces"}.`;
  }

  function workspaceLabelFromKey(key: string): string {
    const parts = key.split(/[\\/]/).filter(Boolean);
    return parts.at(-1) ?? key;
  }

  function workspaceOptionFor(workspace: WorkspaceTabInfo): CompactComboboxOption {
    const label = workspace.workspaceLabel || workspaceLabelFromKey(workspace.cwd);
    return {
      value: workspace.cwd,
      label,
      triggerLabel: label,
      searchText: `${label} ${workspace.cwd}`,
    };
  }

  function mergeWorkspaceOptions(workspaces: readonly WorkspaceTabInfo[]): CompactComboboxOption[] {
    const byCwd = new Map<string, CompactComboboxOption>();
    for (const workspace of workspaces) {
      const cwd = workspace.cwd.trim();
      if (!cwd) continue;
      byCwd.set(cwd, workspaceOptionFor(workspace));
    }
    return [...byCwd.values()].toSorted((left, right) => left.label.localeCompare(right.label));
  }

  async function loadWorkspaceScopeOptions() {
    const [tabState, openWorkspaces] = await Promise.all([
      runtime.storage.appWorkspaceTabs.get().catch(() => null),
      runtime.listOpenWorkspaces().catch(() => []),
    ]);
    const now = new Date().toISOString();
    workspaceScopeOptions = mergeWorkspaceOptions([
      ...(tabState?.knownWorkspaces ?? []),
      ...(tabState?.tabs ?? []),
      ...openWorkspaces,
      {
        workspaceId: runtime.workspaceId,
        workspaceLabel: runtime.workspaceLabel,
        cwd: runtime.cwd,
        branch: runtime.branch,
        openedAt: now,
      },
    ]);
  }

  function currentDraftSnapshot(block: PromptLibraryBlock): DraftSnapshot {
    return {
      blockId: block.id,
      section: block.section,
      title: draftTitle,
      content: draftContent,
      enabled: draftEnabled,
      appGlobal: draftAppGlobal,
      workspaceKeys: [...draftWorkspaceKeys],
      actorKeys: [...draftActorKeys],
    };
  }

  function blockSnapshot(block: PromptLibraryBlock, overrides: Partial<DraftSnapshot> = {}): DraftSnapshot {
    return {
      blockId: block.id,
      section: block.section,
      title: block.title,
      content: block.content,
      enabled: block.enabled,
      appGlobal: block.appGlobal,
      workspaceKeys: [...block.workspaceKeys],
      actorKeys: [...block.actorKeys],
      ...overrides,
    };
  }

  function applyDraftSnapshotToState(state: PromptLibraryState, snapshot: DraftSnapshot) {
    if (snapshot.section === "instructions") {
      const block = state.instructionBlocks[snapshot.blockId];
      if (block) {
        state.instructionBlocks[snapshot.blockId] = {
          ...block,
          title: snapshot.title.trim(),
          body: snapshot.content,
          enabled: snapshot.enabled,
          scope: {
            ...block.scope,
            appGlobal: snapshot.appGlobal,
            workspaceKeys: [...snapshot.workspaceKeys],
          },
        } satisfies PromptLibraryInstructionBlock;
      }
    } else {
      const pack = state.contextPacks[snapshot.blockId];
      if (pack) {
        state.contextPacks[snapshot.blockId] = {
          ...pack,
          title: snapshot.title.trim(),
          body: snapshot.content,
          enabled: snapshot.enabled,
          scope: {
            ...pack.scope,
            appGlobal: snapshot.appGlobal,
            workspaceKeys: [...snapshot.workspaceKeys],
          },
          allowedActors: [...snapshot.actorKeys],
        } satisfies PromptLibraryContextPack;
      }
    }

    const actors: PromptLibraryActorKey[] = ["orchestrator", "handler", "workflow-task"];
    for (const actor of actors) {
      const recipe = state.actorRecipes[actor];
      const ids =
        snapshot.section === "instructions" ? recipe.instructionBlockIds : recipe.contextPackIds;
      const shouldInclude = snapshot.actorKeys.includes(actor);
      const nextIds = ids.filter((id) => id !== snapshot.blockId);
      if (shouldInclude) {
        nextIds.push(snapshot.blockId);
      }
      if (snapshot.section === "instructions") {
        recipe.instructionBlockIds = nextIds;
      } else {
        recipe.contextPackIds = nextIds;
      }
    }
  }

  function sameStringList(left: readonly string[], right: readonly string[]): boolean {
    return left.length === right.length && left.every((value, index) => value === right[index]);
  }

  function getActorKeysForBlock(
    state: PromptLibraryState,
    section: PromptLibrarySection,
    id: string,
  ): PromptLibraryActorKey[] {
    const actors: PromptLibraryActorKey[] = ["orchestrator", "handler", "workflow-task"];
    return actors.filter((actor) => {
      const recipe = state.actorRecipes[actor];
      const ids = section === "instructions" ? recipe.instructionBlockIds : recipe.contextPackIds;
      return ids.includes(id);
    });
  }

  function isDefaultInstructionEdited(
    block: PromptLibraryInstructionBlock,
    actorKeys: PromptLibraryActorKey[],
    defaults: PromptLibraryState | null,
  ): boolean {
    const defaultBlock = defaults?.instructionBlocks[block.id];
    if (!defaultBlock) return true;
    return (
      block.title !== defaultBlock.title ||
      block.summary !== defaultBlock.summary ||
      block.body !== defaultBlock.body ||
      block.enabled !== defaultBlock.enabled ||
      block.scope.appGlobal !== defaultBlock.scope.appGlobal ||
      !sameStringList(block.scope.workspaceKeys, defaultBlock.scope.workspaceKeys) ||
      block.actor !== defaultBlock.actor ||
      !sameStringList(actorKeys, getActorKeysForBlock(defaults, "instructions", block.id))
    );
  }

  function isDefaultContextPackEdited(
    pack: PromptLibraryContextPack,
    actorKeys: PromptLibraryActorKey[],
    defaults: PromptLibraryState | null,
  ): boolean {
    const defaultPack = defaults?.contextPacks[pack.id];
    if (!defaultPack) return true;
    return (
      pack.title !== defaultPack.title ||
      pack.summary !== defaultPack.summary ||
      pack.body !== defaultPack.body ||
      pack.enabled !== defaultPack.enabled ||
      pack.scope.appGlobal !== defaultPack.scope.appGlobal ||
      !sameStringList(pack.scope.workspaceKeys, defaultPack.scope.workspaceKeys) ||
      !sameStringList(pack.allowedActors, defaultPack.allowedActors) ||
      pack.optionalContextKey !== defaultPack.optionalContextKey ||
      !sameStringList(actorKeys, getActorKeysForBlock(defaults, "contextPacks", pack.id))
    );
  }

  function updateRecipeMembershipForBlock(
    state: PromptLibraryState,
    defaults: PromptLibraryState,
    section: PromptLibrarySection,
    id: string,
  ) {
    for (const actor of Object.keys(state.actorRecipes) as PromptLibraryActorKey[]) {
      const recipe = state.actorRecipes[actor];
      const defaultRecipe = defaults.actorRecipes[actor];
      const defaultIds =
        section === "instructions"
          ? defaultRecipe.instructionBlockIds
          : defaultRecipe.contextPackIds;
      const shouldInclude = defaultIds.includes(id);
      const ids =
        section === "instructions" ? recipe.instructionBlockIds : recipe.contextPackIds;
      const nextIds = ids.filter((candidate) => candidate !== id);
      if (shouldInclude) {
        nextIds.push(id);
      }
      if (section === "instructions") {
        recipe.instructionBlockIds = nextIds;
      } else {
        recipe.contextPackIds = nextIds;
      }
    }
  }

  function canResetBlock(block: PromptLibraryBlock): boolean {
    return block.section === "instructions"
      ? Boolean(defaultPromptState?.instructionBlocks[block.id])
      : Boolean(defaultPromptState?.contextPacks[block.id]);
  }

  function canDeleteBlock(block: PromptLibraryBlock): boolean {
    return block.deletable;
  }

  function actorLabel(actorKey: PromptLibraryActorKey): string {
    switch (actorKey) {
      case "orchestrator":
        return "Orchestrator";
      case "handler":
        return "Handler";
      case "workflow-task":
        return "Workflow Task";
    }
  }

  function actorChipLabel(actorKey: PromptLibraryActorKey): string {
    switch (actorKey) {
      case "orchestrator":
        return "orchestrator";
      case "handler":
        return "handler";
      case "workflow-task":
        return "workflow task";
    }
  }

  function generatedEntriesForActor(actor: PromptLibraryActorKey): PromptLibraryGeneratedEntry[] {
    return generatedEntriesByActor?.[actor] ?? [];
  }

  function actorSummary(actor: PromptLibraryActorKey): string {
    if (actor === "orchestrator") return "Strategy, routing, delegation, and final answers.";
    if (actor === "handler") return "Delegated objective ownership and Smithers supervision.";
    return "Task-local Smithers workflow execution.";
  }

  function convertStateToReadModel(state: PromptLibraryState): PromptLibraryReadModel {
    const actors: PromptLibraryActorKey[] = ["orchestrator", "handler", "workflow-task"];
    const instructionEntries = Object.values(state.instructionBlocks).map((block) => {
      const actorKeys = getActorKeysForBlock(state, "instructions", block.id);
      return {
        id: block.id,
        section: "instructions" as const,
        title: block.title,
        summary: block.summary,
        content: block.body,
        actorKeys,
        enabled: block.enabled,
        appGlobal: block.scope.appGlobal,
        workspaceKeys: [...block.scope.workspaceKeys],
        deletable: !block.default,
        state:
          block.default && isDefaultInstructionEdited(block, actorKeys, defaultPromptState)
            ? "edited"
            : block.default
              ? "builtin"
              : "custom",
      };
    });
    const contextEntries = Object.values(state.contextPacks).map((pack) => {
      const actorKeys = getActorKeysForBlock(state, "contextPacks", pack.id);
      return {
        id: pack.id,
        section: "contextPacks" as const,
        title: pack.title,
        summary: pack.summary,
        content: pack.body,
        actorKeys,
        enabled: pack.enabled,
        appGlobal: pack.scope.appGlobal,
        workspaceKeys: [...pack.scope.workspaceKeys],
        deletable: !pack.default,
        state:
          pack.default && isDefaultContextPackEdited(pack, actorKeys, defaultPromptState)
            ? "edited"
            : pack.default
              ? "builtin"
              : "custom",
      };
    });
    return {
      updatedAt: state.updatedAt,
      sections: {
        instructions: instructionEntries,
        contextPacks: contextEntries,
      },
      actors: actors.map((actor) => ({
        id: actor,
        label: actorLabel(actor),
        summary: actorSummary(actor),
        blockIds: [
          ...(state.actorRecipes[actor]?.instructionBlockIds ?? []),
          ...(state.actorRecipes[actor]?.contextPackIds ?? []),
        ],
        generatedEntries: generatedEntriesForActor(actor),
      })),
    };
  }

  function selectBlock(block: PromptLibraryBlock, options: { flushCurrentDraft?: boolean } = {}) {
    if (options.flushCurrentDraft ?? true) {
      void flushPendingAutosave();
    }
    selectedKind = "block";
    selectedId = block.id;
    draftTitle = block.title;
    draftContent = block.content;
    draftEnabled = block.enabled;
    draftAppGlobal = block.appGlobal;
    draftWorkspaceKeys = [...block.workspaceKeys];
    draftActorKeys = [...block.actorKeys];
    actionMessage = null;
  }

  function selectActor(actor: PromptLibraryActorAggregate, options: { flushCurrentDraft?: boolean } = {}) {
    if (options.flushCurrentDraft ?? true) {
      void flushPendingAutosave();
    }
    selectedKind = "actor";
    selectedId = actor.id;
    draftTitle = "";
    draftContent = "";
    draftEnabled = true;
    draftAppGlobal = true;
    draftWorkspaceKeys = [];
    draftActorKeys = [];
    actionMessage = null;
  }

  function syncSelection(model: PromptLibraryReadModel) {
    if (selectedKind === "actor") {
      const actor = model.actors.find((candidate) => candidate.id === selectedId) ?? model.actors[0] ?? null;
      if (actor) {
        selectActor(actor, { flushCurrentDraft: false });
        return;
      }
    }
    const block = [
      ...model.sections.instructions,
      ...model.sections.contextPacks,
    ].find((candidate) => candidate.id === selectedId) ?? model.sections.instructions[0] ?? model.sections.contextPacks[0] ?? null;
    if (block) {
      selectBlock(block, { flushCurrentDraft: false });
    } else {
      selectedId = null;
    }
  }

  function canRunBackgroundRefresh(): boolean {
    return (
      !loading &&
      !saving &&
      !isDirty &&
      autosaveStatus !== "dirty" &&
      autosaveStatus !== "saving" &&
      !autosaveTimer &&
      !resetCandidate
    );
  }

  async function loadLibrary(options: { background?: boolean } = {}) {
    const background = options.background ?? false;
    if (background && !canRunBackgroundRefresh()) return;
    if (autosaveTimer) {
      clearTimeout(autosaveTimer);
      autosaveTimer = null;
    }
    if (!background) {
      loading = true;
      error = null;
    }
    try {
      const [state, defaults, generatedEntries] = await Promise.all([
        runtime.getPromptLibrary(),
        runtime.getPromptLibraryDefaults(),
        runtime.getPromptLibraryGeneratedEntries(),
      ]);
      promptState = state;
      defaultPromptState = defaults;
      lastSavedAt = state.updatedAt;
      autosaveStatus = "saved";
      generatedEntriesByActor = generatedEntries;
      const next = convertStateToReadModel(state);
      readModel = next;
      syncSelection(next);
    } catch (err) {
      if (!background) {
        error = err instanceof Error ? err.message : "Unable to load context library.";
      }
    } finally {
      if (!background) {
        loading = false;
      }
    }
  }

  async function openGeneratedSource(entry: PromptLibraryGeneratedEntry) {
    actionMessage = null;
    try {
      const opened = await runtime.openWorkspaceSourceInEditor(entry.sourcePath);
      actionMessage = opened
        ? `Opened ${entry.sourcePath}`
        : `Could not open ${entry.sourcePath}. Check the configured external editor.`;
    } catch (err) {
      actionMessage = err instanceof Error ? err.message : "Unable to view in external editor.";
    }
  }

  function notifyPromptLibraryCurrentChanged(options: { unsnapshotted?: boolean } = {}) {
    window.dispatchEvent(
      new CustomEvent("svvy:prompt-library-current-changed", {
        detail: { panelId, ...options },
      }),
    );
  }

  async function persistDraftSnapshot(snapshot: DraftSnapshot) {
    if (!promptState || !snapshot.title.trim() || !snapshot.content.trim()) return;
    saving = true;
    actionMessage = null;
    autosaveStatus = "saving";
    try {
      const state = clonePromptLibraryState(promptState);
      applyDraftSnapshotToState(state, snapshot);
      const nextState = await runtime.updatePromptLibrary({ state });
      promptState = nextState;
      lastSavedAt = nextState.updatedAt;
      const next = convertStateToReadModel(nextState);
      readModel = next;
      autosaveStatus = "saved";
      notifyPromptLibraryCurrentChanged();
    } catch (err) {
      actionMessage = err instanceof Error ? err.message : "Unable to save prompt block.";
      autosaveStatus = "error";
    } finally {
      saving = false;
    }
  }

  function persistControlChange(snapshot: DraftSnapshot) {
    if (autosaveTimer) {
      clearTimeout(autosaveTimer);
      autosaveTimer = null;
    }
    autosaveStatus = "saving";
    notifyPromptLibraryCurrentChanged({ unsnapshotted: true });
    instantSaveQueue = instantSaveQueue
      .catch(() => undefined)
      .then(() => persistDraftSnapshot(snapshot));
    void instantSaveQueue;
  }

  function persistBlockEnabledChange(block: PromptLibraryBlock, enabled: boolean) {
    if (selectedKind === "block" && selectedId === block.id) {
      draftEnabled = enabled;
    }
    persistControlChange(blockSnapshot(block, { enabled }));
  }

  async function flushPendingAutosave() {
    if (autosaveTimer) {
      clearTimeout(autosaveTimer);
      autosaveTimer = null;
    }
    if (!selectedBlock || !isDirty || !draftTitle.trim() || !draftContent.trim()) return;
    await persistDraftSnapshot(currentDraftSnapshot(selectedBlock));
  }

  function scheduleAutosave(block: PromptLibraryBlock) {
    if (autosaveTimer) {
      clearTimeout(autosaveTimer);
    }
    autosaveStatus = "dirty";
    autosaveTimer = setTimeout(() => {
      autosaveTimer = null;
      void persistDraftSnapshot(currentDraftSnapshot(block));
    }, 700);
  }

  $effect(() => {
    if (!selectedBlock || loading) return;

    if (!isTextDirty) {
      if (autosaveTimer) {
        clearTimeout(autosaveTimer);
        autosaveTimer = null;
      }
      if (saving) {
        autosaveStatus = "saving";
      } else if (autosaveStatus !== "error") {
        autosaveStatus = "saved";
      }
      return;
    }

    if (!draftTitle.trim() || !draftContent.trim()) {
      autosaveStatus = "dirty";
      notifyPromptLibraryCurrentChanged({ unsnapshotted: true });
      return;
    }

    if (saving) {
      autosaveStatus = "saving";
      return;
    }

    scheduleAutosave(selectedBlock);
    notifyPromptLibraryCurrentChanged({ unsnapshotted: true });

    return () => {
      if (autosaveTimer) {
        clearTimeout(autosaveTimer);
        autosaveTimer = null;
      }
    };
  });

  function createBlockId(section: PromptLibrarySection, state: PromptLibraryState): string {
    const prefix = section === "instructions" ? "instruction" : "context-pack";
    const existingIds = new Set([
      ...Object.keys(state.instructionBlocks),
      ...Object.keys(state.contextPacks),
    ]);
    for (let index = 1; ; index += 1) {
      const id = `${prefix}-${index}`;
      if (!existingIds.has(id)) {
        return id;
      }
    }
  }

  async function addBlock(section: PromptLibrarySection) {
    if (!promptState) return;
    await flushPendingAutosave();
    saving = true;
    actionMessage = null;
    try {
      const state = clonePromptLibraryState(promptState);
      const id = createBlockId(section, state);
      const actors: PromptLibraryActorKey[] = ["orchestrator", "handler", "workflow-task"];
      if (section === "instructions") {
        state.instructionBlocks[id] = {
          id,
          title: "New Instruction",
          summary: "User-created instruction.",
          body: "Write the instruction here.",
          enabled: true,
          scope: {
            appGlobal: true,
            workspaceKeys: [],
          },
          actor: "common",
          default: false,
        };
        for (const actor of actors) {
          state.actorRecipes[actor].instructionBlockIds = [
            ...state.actorRecipes[actor].instructionBlockIds.filter((candidate) => candidate !== id),
            id,
          ];
        }
      } else {
        state.contextPacks[id] = {
          id,
          title: "New Context Pack",
          summary: "User-created context pack.",
          body: "Write the context pack here.",
          enabled: true,
          scope: {
            appGlobal: true,
            workspaceKeys: [],
          },
          allowedActors: actors,
          default: false,
        };
        for (const actor of actors) {
          state.actorRecipes[actor].contextPackIds = [
            ...state.actorRecipes[actor].contextPackIds.filter((candidate) => candidate !== id),
            id,
          ];
        }
      }
      const nextState = await runtime.updatePromptLibrary({ state });
      promptState = nextState;
      lastSavedAt = nextState.updatedAt;
      autosaveStatus = "saved";
      const next = convertStateToReadModel(nextState);
      readModel = next;
      const block = next.sections[section].find((candidate) => candidate.id === id);
      notifyPromptLibraryCurrentChanged();
      if (block) {
        selectBlock(block, { flushCurrentDraft: false });
      } else {
        syncSelection(next);
      }
    } catch (err) {
      actionMessage =
        err instanceof Error
          ? err.message
          : `Unable to create ${section === "instructions" ? "instruction" : "context pack"}.`;
    } finally {
      saving = false;
    }
  }

  async function deleteBlock(block: PromptLibraryBlock) {
    if (!promptState || !canDeleteBlock(block)) return;
    if (autosaveTimer) {
      clearTimeout(autosaveTimer);
      autosaveTimer = null;
    }
    saving = true;
    actionMessage = null;
    try {
      const state = clonePromptLibraryState(promptState);
      if (block.section === "instructions") {
        delete state.instructionBlocks[block.id];
        for (const actor of Object.keys(state.actorRecipes) as PromptLibraryActorKey[]) {
          state.actorRecipes[actor].instructionBlockIds = state.actorRecipes[
            actor
          ].instructionBlockIds.filter((id) => id !== block.id);
        }
      } else {
        delete state.contextPacks[block.id];
        for (const actor of Object.keys(state.actorRecipes) as PromptLibraryActorKey[]) {
          state.actorRecipes[actor].contextPackIds = state.actorRecipes[
            actor
          ].contextPackIds.filter((id) => id !== block.id);
        }
      }
      const nextState = await runtime.updatePromptLibrary({ state });
      promptState = nextState;
      lastSavedAt = nextState.updatedAt;
      autosaveStatus = "saved";
      const next = convertStateToReadModel(nextState);
      readModel = next;
      syncSelection(next);
      notifyPromptLibraryCurrentChanged();
    } catch (err) {
      actionMessage = err instanceof Error ? err.message : "Unable to delete prompt block.";
    } finally {
      saving = false;
    }
  }

  async function resetBlock(block: PromptLibraryBlock) {
    if (!promptState || !defaultPromptState) return;
    if (autosaveTimer) {
      clearTimeout(autosaveTimer);
      autosaveTimer = null;
    }
    saving = true;
    actionMessage = null;
    try {
      const state = clonePromptLibraryState(promptState);
      if (block.section === "instructions") {
        const defaultBlock = defaultPromptState.instructionBlocks[block.id];
        if (!defaultBlock) return;
        state.instructionBlocks[block.id] = JSON.parse(
          JSON.stringify(defaultBlock),
        ) as PromptLibraryInstructionBlock;
      } else {
        const defaultPack = defaultPromptState.contextPacks[block.id];
        if (!defaultPack) return;
        state.contextPacks[block.id] = JSON.parse(
          JSON.stringify(defaultPack),
        ) as PromptLibraryContextPack;
      }
      updateRecipeMembershipForBlock(state, defaultPromptState, block.section, block.id);
      const nextState = await runtime.updatePromptLibrary({ state });
      promptState = nextState;
      lastSavedAt = nextState.updatedAt;
      autosaveStatus = "saved";
      const next = convertStateToReadModel(nextState);
      readModel = next;
      syncSelection(next);
      notifyPromptLibraryCurrentChanged();
    } catch (err) {
      actionMessage = err instanceof Error ? err.message : "Unable to reset prompt block.";
    } finally {
      saving = false;
      resetCandidate = null;
    }
  }

  function stateTone(state: PromptLibraryBlockState): "neutral" | "info" | "success" | "warning" {
    if (state === "edited") return "warning";
    if (state === "custom") return "success";
    if (state === "generated") return "info";
    return "neutral";
  }

  function stateLabel(state: PromptLibraryBlockState): string {
    return state;
  }

  onMount(() => {
    void loadLibrary();
    void loadWorkspaceScopeOptions();

    const handleHeaderFlush = (event: Event) => {
      const detail = (event as CustomEvent<{
        panelId?: string;
        register?: (promise: Promise<void>) => void;
      }>).detail;
      if (detail?.panelId !== panelId) return;
      detail.register?.(flushPendingAutosave());
    };
    const handleHeaderReload = (event: Event) => {
      const detail = (event as CustomEvent<{ panelId?: string }>).detail;
      if (detail?.panelId !== panelId) return;
      void loadLibrary();
    };
    const refreshInBackground = () => {
      if (document.visibilityState === "visible") {
        void loadLibrary({ background: true });
        void loadWorkspaceScopeOptions();
      }
    };
    const interval = window.setInterval(refreshInBackground, BACKGROUND_REFRESH_INTERVAL_MS);
    window.addEventListener("svvy:prompt-library-flush", handleHeaderFlush);
    window.addEventListener("svvy:prompt-library-reload", handleHeaderReload);
    window.addEventListener("focus", refreshInBackground);
    document.addEventListener("visibilitychange", refreshInBackground);

    return () => {
      window.clearInterval(interval);
      window.removeEventListener("svvy:prompt-library-flush", handleHeaderFlush);
      window.removeEventListener("svvy:prompt-library-reload", handleHeaderReload);
      window.removeEventListener("focus", refreshInBackground);
      document.removeEventListener("visibilitychange", refreshInBackground);
      if (autosaveTimer) {
        clearTimeout(autosaveTimer);
        autosaveTimer = null;
      }
    };
  });
</script>

<section class="prompt-library" aria-label="Context library">
  {#if error}
    <p class="library-message error">{error}</p>
  {:else if loading}
    <p class="library-message">Loading context library...</p>
  {:else if readModel}
    <div class="library-body">
      <div class="library-list" role="list" aria-label="Context library entries">
        {#each Object.entries(readModel.sections) as [section, blocks] (section)}
          <section class="library-group">
            <header class="library-group-header">
              <span>{SECTION_LABELS[section as PromptLibrarySection]}</span>
              <span class="library-group-actions">
                <strong>{blocks.length}</strong>
                <Button
                  class="library-tool-button library-add-button"
                  variant="ghost"
                  size="xs"
                  iconOnly
                  disabled={saving}
                  aria-label={`Add ${SECTION_LABELS[section as PromptLibrarySection].toLowerCase()}`}
                  title={`Add ${SECTION_LABELS[section as PromptLibrarySection].toLowerCase()}`}
                  onclick={() => addBlock(section as PromptLibrarySection)}
                >
                  <PlusIcon aria-hidden="true" size={12} strokeWidth={2.1} />
                </Button>
              </span>
            </header>
            {#each blocks as block (block.id)}
              <div
                class={`library-row ${selectedKind === "block" && selectedId === block.id ? "active" : ""} ${!block.enabled ? "disabled" : ""}`.trim()}
              >
                <button class="library-row-main" type="button" onclick={() => selectBlock(block)}>
                  <span class="row-top">
                    <strong>{block.title}</strong>
                  </span>
                  <span class="row-meta">
                    {#each block.actorKeys as actorKey (actorKey)}
                      <span class={`actor-usage-chip actor-${actorKey}`}>{actorChipLabel(actorKey)}</span>
                    {/each}
                  </span>
                </button>
                <span class="library-row-controls">
                  <Badge class="context-state-badge" tone={stateTone(block.state)}>{stateLabel(block.state)}</Badge>
                  <Tooltip label={enabledCheckboxTooltipFor(block.enabled)} side="left">
                    <Checkbox
                      size="sm"
                      checked={block.enabled}
                      aria-label={`${block.enabled ? "Disable" : "Enable"} ${block.title}`}
                      onchange={(event) => {
                        const enabled = (event.currentTarget as HTMLInputElement).checked;
                        persistBlockEnabledChange(block, enabled);
                      }}
                    />
                  </Tooltip>
                </span>
              </div>
            {/each}
          </section>
        {/each}

        <section class="library-group actors-group">
          <header class="library-group-header">
            <span>Actors</span>
            <strong>{readModel.actors.length}</strong>
          </header>
          {#each readModel.actors as actor (actor.id)}
            <button
              type="button"
              class={`library-row actor-row ${selectedKind === "actor" && selectedId === actor.id ? "active" : ""}`.trim()}
              onclick={() => selectActor(actor)}
            >
              <span class="row-top">
                <strong>{actor.label}</strong>
                <Badge tone="info">{actor.blockIds.length}</Badge>
              </span>
              <span class="row-summary">{actor.summary}</span>
              <span class="row-meta">
                <code>{actor.generatedEntries.length} generated</code>
              </span>
            </button>
          {/each}
        </section>

      </div>

      <article class="library-detail">
        {#if selectedBlock}
          <header class="detail-header">
            <div>
              <p>{SECTION_LABELS[selectedBlock.section]}</p>
              <h3>{draftTitle || selectedBlock.title}</h3>
            </div>
            <div class="detail-actions">
              <span class={`save-status save-status-${autosaveStatus}`}>{saveStatusLabel}</span>
              <Tooltip label={resetBlockLabel(selectedBlock)} disabled={saving || !canResetBlock(selectedBlock)}>
                <Button
                  class="detail-action-button"
                  variant="ghost"
                  size="xs"
                  iconOnly
                  disabled={saving || !canResetBlock(selectedBlock)}
                  aria-label={resetBlockLabel(selectedBlock)}
                  onclick={() => {
                    resetCandidate = selectedBlock;
                  }}
                >
                  <RotateCcwIcon aria-hidden="true" size={13} strokeWidth={1.9} />
                </Button>
              </Tooltip>
              <Tooltip label={deleteBlockLabel(selectedBlock)} disabled={saving}>
                <Button
                  class="detail-action-button detail-danger-button"
                  variant="ghost"
                  size="xs"
                  iconOnly
                  disabled={saving || !canDeleteBlock(selectedBlock)}
                  aria-label={deleteBlockLabel(selectedBlock)}
                  onclick={() => deleteBlock(selectedBlock)}
                >
                  <Trash2Icon aria-hidden="true" size={13} strokeWidth={1.9} />
                </Button>
              </Tooltip>
            </div>
          </header>

          {#if actionMessage}
            <p class="library-message inline">{actionMessage}</p>
          {/if}

          {#if !draftEnabled}
            <p class="disabled-detail-warning">
              This {blockKindLabel(selectedBlock)} is disabled and will not be injected into the system prompt.
            </p>
          {/if}

          <label class="field">
            <span>Title</span>
            <Input bind:value={draftTitle} />
          </label>
          <label class="field">
            <span>Content</span>
            <TextArea bind:value={draftContent} resize="vertical" />
          </label>

          <div class="control-grid" aria-label="Context block controls">
            <section class="control-section">
              <span class="control-label">Actors</span>
              <div class="actor-inclusion-chips">
                {#each ["orchestrator", "handler", "workflow-task"] as actorKey (actorKey)}
                  {@const typedActorKey = actorKey as PromptLibraryActorKey}
                  {@const actorChecked = draftActorKeys.includes(typedActorKey)}
                  <label class={`actor-inclusion-chip actor-${actorKey} ${actorChecked ? "checked" : ""}`.trim()}>
                    <Checkbox
                      size="sm"
                      checked={actorChecked}
                      aria-label={`${actorChecked ? "Remove from" : "Include in"} ${actorLabel(typedActorKey)}`}
                      onchange={(event) => {
                        const checked = (event.currentTarget as HTMLInputElement).checked;
                        if (checked) {
                          draftActorKeys = [...draftActorKeys, typedActorKey];
                        } else {
                          draftActorKeys = draftActorKeys.filter((candidate) => candidate !== typedActorKey);
                        }
                        persistControlChange({
                          ...currentDraftSnapshot(selectedBlock),
                          actorKeys: draftActorKeys,
                        });
                      }}
                    />
                    <span>{actorChipLabel(typedActorKey)}</span>
                  </label>
                {/each}
              </div>
              <span class="control-caption">{actorControlCaption(selectedBlock)}</span>
            </section>
            <section class="control-section">
              <span class="control-label">Scope</span>
              <div class="scope-control-row">
                <label class="scope-check-row">
                  <Checkbox
                    size="sm"
                    checked={draftAppGlobal}
                    aria-label="App global"
                    onchange={(event) => {
                      const appGlobal = (event.currentTarget as HTMLInputElement).checked;
                      draftAppGlobal = appGlobal;
                      persistControlChange({
                        ...currentDraftSnapshot(selectedBlock),
                        appGlobal,
                      });
                    }}
                  />
                  <span>App global</span>
                </label>
                <Tooltip
                  label="Uncheck App global first to select workspaces"
                  disabled={!draftAppGlobal}
                  side="top"
                >
                  <CompactCombobox
                    values={draftWorkspaceKeys}
                    multiple
                    options={workspaceScopeOptionsWithSelected}
                    ariaLabel="Workspace scope"
                    placeholder="Select workspaces"
                    emptyLabel="No workspaces found."
                    disabled={draftAppGlobal}
                    triggerClass="scope-select"
                    menuClass="scope-menu"
                    optionClass="scope-option"
                    leadingIcon="workspace"
                    onBeforeOpen={loadWorkspaceScopeOptions}
                    onMultiSelect={(workspaceKeys) => {
                      draftWorkspaceKeys = workspaceKeys;
                      persistControlChange({
                        ...currentDraftSnapshot(selectedBlock),
                        workspaceKeys,
                      });
                    }}
                  />
                </Tooltip>
              </div>
              <span class="control-caption">{scopeControlCaption(draftAppGlobal, draftWorkspaceKeys)}</span>
            </section>
          </div>

          {#if selectedBlock.generatedEntries?.length}
            <section class="detail-section">
              <h4>Generated Context</h4>
              {#each selectedBlock.generatedEntries as entry (entry.id)}
                <div class="generated-entry">
                  <span>
                    <span class="generated-entry-title">
                      <strong>{entry.title}</strong>
                      <code>{entry.source}</code>
                    </span>
                    <Tooltip label="View in external editor">
                      <Button
                        class="library-tool-button"
                        variant="ghost"
                        size="xs"
                        iconOnly
                        aria-label={`View ${entry.sourcePath} in external editor`}
                        onclick={() => openGeneratedSource(entry)}
                      >
                        <ExternalLinkIcon aria-hidden="true" size={13} strokeWidth={1.9} />
                      </Button>
                    </Tooltip>
                  </span>
                  <pre><code>{entry.content}</code></pre>
                </div>
              {/each}
            </section>
          {/if}
        {:else if selectedActor}
          <header class="detail-header">
            <div>
              <p>Actor Aggregate</p>
              <h3>{selectedActor.label}</h3>
            </div>
          </header>
          <p class="actor-summary">{selectedActor.summary}</p>

          {#if actionMessage}
            <p class="library-message inline">{actionMessage}</p>
          {/if}

          <section class="detail-section">
            <h4>Included Context</h4>
            {#each actorBlocks as block (block.id)}
              <button type="button" class="included-block" onclick={() => selectBlock(block)}>
                <span>{block.title}</span>
                <code>{SECTION_LABELS[block.section]}</code>
              </button>
            {/each}
          </section>

          <section class="detail-section">
            <h4>Generated Context</h4>
            {#each selectedActor.generatedEntries as entry (entry.id)}
              <div class="generated-entry">
                <span>
                  <span class="generated-entry-title">
                    <strong>{entry.title}</strong>
                    <code>{entry.source}</code>
                  </span>
                  <Tooltip label="View in external editor">
                    <Button
                      class="library-tool-button"
                      variant="ghost"
                      size="xs"
                      iconOnly
                      aria-label={`View ${entry.sourcePath} in external editor`}
                      onclick={() => openGeneratedSource(entry)}
                    >
                      <ExternalLinkIcon aria-hidden="true" size={13} strokeWidth={1.9} />
                    </Button>
                  </Tooltip>
                </span>
                <pre><code>{entry.content}</code></pre>
              </div>
            {/each}
          </section>
        {/if}
      </article>
    </div>
  {/if}
</section>

{#if resetCandidate}
  {@const blockToReset = resetCandidate}
  <Dialog
    title={resetBlockLabel(blockToReset)}
    eyebrow="Context Library"
    description={`This restores "${blockToReset.title}" to its builtin ${blockKindLabel(blockToReset)} text, enabled state, scope, and actor settings. Current edits for this ${blockKindLabel(blockToReset)} will be discarded.`}
    width="md"
    onClose={() => {
      resetCandidate = null;
    }}
  >
    <div class="reset-dialog-actions">
      <Button size="sm" variant="ghost" onclick={() => {
        resetCandidate = null;
      }}>Cancel</Button>
      <Button size="sm" variant="danger" disabled={saving} onclick={() => void resetBlock(blockToReset)}>
        Reset
      </Button>
    </div>
  </Dialog>
{/if}

<style>
  .prompt-library {
    display: grid;
    grid-template-rows: minmax(0, 1fr);
    height: 100%;
    min-height: 0;
    color: var(--ui-text-primary);
    background: var(--ui-surface);
  }

  .detail-header,
  .row-top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.65rem;
    min-width: 0;
  }

  .detail-header p,
  .library-group-header,
  .detail-section h4,
  .field span {
    margin: 0;
    color: var(--ui-text-secondary);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    text-transform: uppercase;
  }

  .detail-header h3 {
    margin: 0.12rem 0 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--text-base);
    font-weight: 600;
    line-height: 1.2;
  }

  .library-body {
    display: grid;
    grid-template-columns: minmax(17rem, 0.86fr) minmax(0, 1.14fr);
    min-height: 0;
  }

  .library-list,
  .library-detail {
    min-height: 0;
    overflow: auto;
  }

  .library-list {
    display: grid;
    align-content: start;
    gap: 0.42rem;
    padding: 0.35rem;
    border-right: 1px solid color-mix(in oklab, var(--ui-border-soft) 90%, transparent);
    background: color-mix(in oklab, var(--ui-surface-subtle) 84%, transparent);
  }

  .library-group {
    display: grid;
    gap: 0.25rem;
  }

  .library-group-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    padding: 0.38rem 0.4rem 0.2rem;
    color: var(--ui-text-tertiary);
  }

  .library-group-actions {
    display: inline-flex;
    align-items: center;
    gap: 0.24rem;
  }

  :global(.prompt-library .library-tool-button),
  :global(.prompt-library .detail-action-button) {
    display: inline-grid;
    place-items: center;
    width: 1.45rem;
    height: 1.45rem;
    min-height: 1.45rem;
    padding: 0;
    border: 0;
    border-radius: var(--ui-radius-sm);
    background: transparent;
    color: var(--ui-text-tertiary);
    box-shadow: none;
    transition:
      background-color 150ms cubic-bezier(0.19, 1, 0.22, 1),
      color 150ms cubic-bezier(0.19, 1, 0.22, 1),
      opacity 150ms cubic-bezier(0.19, 1, 0.22, 1);
  }

  :global(.prompt-library .library-tool-button.icon-only),
  :global(.prompt-library .detail-action-button.icon-only) {
    width: 1.45rem;
  }

  :global(.prompt-library .library-tool-button:hover:not(:disabled)),
  :global(.prompt-library .detail-action-button:hover:not(:disabled)) {
    outline: none;
    background: var(--ui-surface-subtle);
    color: var(--ui-text-primary);
  }

  :global(.prompt-library .library-tool-button:focus-visible),
  :global(.prompt-library .detail-action-button:focus-visible) {
    outline: none;
    background: var(--ui-surface-subtle);
    color: var(--ui-text-primary);
    box-shadow: var(--ui-focus-ring);
  }

  :global(.prompt-library .library-tool-button:disabled),
  :global(.prompt-library .detail-action-button:disabled) {
    cursor: not-allowed;
    opacity: 0.48;
  }

  :global(.prompt-library .library-add-button) {
    opacity: 0.66;
  }

  .library-group:hover :global(.library-add-button),
  .library-group:focus-within :global(.library-add-button) {
    opacity: 1;
  }

  :global(.prompt-library .library-add-button:hover:not(:disabled)) {
    background: color-mix(in oklab, var(--ui-surface-muted) 76%, var(--ui-surface-subtle));
    color: var(--ui-text-primary);
  }

  :global(.prompt-library .library-add-button:focus-visible) {
    background: color-mix(in oklab, var(--ui-surface-muted) 76%, var(--ui-surface-subtle));
  }

  .library-row,
  .included-block {
    display: grid;
    gap: 0.28rem;
    width: 100%;
    padding: 0.5rem 0.56rem;
    border: 1px solid transparent;
    border-radius: var(--ui-radius-sm);
    background: transparent;
    color: inherit;
    text-align: left;
  }

  .library-row:hover,
  .library-row:focus-within,
  .library-row.active,
  .included-block:hover,
  .included-block:focus-visible {
    outline: none;
    border-color: color-mix(in oklab, var(--ui-border-soft) 88%, transparent);
    background: color-mix(in oklab, var(--ui-surface-raised) 88%, transparent);
  }

  .library-row.active {
    box-shadow: inset 2px 0 0 var(--ui-accent);
  }

  .library-row:not(.actor-row) {
    grid-template-columns: minmax(0, 1fr) max-content;
    align-items: start;
    column-gap: 0.44rem;
  }

  .library-row-main {
    display: grid;
    min-width: 0;
    gap: 0.28rem;
    padding: 0;
    border: 0;
    background: transparent;
    color: inherit;
    text-align: left;
    cursor: pointer;
  }

  .library-row-main:focus-visible {
    outline: none;
  }

  .library-row-controls {
    display: inline-grid;
    justify-items: end;
    gap: 0.38rem;
    padding-top: 0.02rem;
  }

  .library-row.disabled {
    color: var(--ui-text-tertiary);
  }

  .library-row.disabled .library-row-main {
    opacity: 0.68;
  }

  .row-top strong,
  .included-block span {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--text-sm);
  }

  .row-summary,
  .row-meta,
  .library-message,
  .actor-summary,
  .generated-entry {
    color: var(--ui-text-secondary);
    font-size: var(--text-xs);
    line-height: 1.45;
  }

  .row-meta {
    display: flex;
    flex-wrap: wrap;
    gap: 0.25rem;
  }

  .actor-usage-chip {
    display: inline-flex;
    align-items: center;
    min-height: 0.98rem;
    padding: 0.04rem 0.28rem;
    border-radius: var(--ui-radius-sm);
    background: color-mix(in oklab, var(--ui-surface-muted) 34%, transparent);
    color: var(--ui-text-tertiary);
    font-family: var(--font-mono);
    font-size: 10px;
    font-weight: 500;
    line-height: 1;
    white-space: nowrap;
  }

  .actor-usage-chip.actor-orchestrator {
    background: color-mix(in oklab, var(--ui-accent-soft) 16%, var(--ui-surface-muted));
    color: color-mix(in oklab, var(--ui-accent) 34%, var(--ui-text-tertiary));
  }

  .actor-usage-chip.actor-handler {
    background: color-mix(in oklab, var(--ui-info-soft) 17%, var(--ui-surface-muted));
    color: color-mix(in oklab, var(--ui-info) 33%, var(--ui-text-tertiary));
  }

  .actor-usage-chip.actor-workflow-task {
    background: color-mix(in oklab, var(--ui-success-soft) 16%, var(--ui-surface-muted));
    color: color-mix(in oklab, var(--ui-success) 30%, var(--ui-text-tertiary));
  }

  :global(.prompt-library .context-state-badge) {
    min-height: 0.9rem;
    padding: 0.04rem 0.24rem;
    border-color: color-mix(in oklab, var(--ui-border-soft) 72%, transparent);
    background: color-mix(in oklab, var(--ui-surface-muted) 42%, transparent);
    color: var(--ui-text-tertiary);
    font-size: 10px;
    font-weight: 500;
  }

  code {
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    color: var(--ui-text-tertiary);
  }

  .library-detail {
    display: grid;
    align-content: start;
    gap: 0.72rem;
    padding: 0.72rem;
  }

  .detail-actions {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: flex-end;
    gap: 0.24rem;
  }

  :global(.prompt-library .detail-danger-button) {
    color: var(--ui-text-tertiary);
  }

  :global(.prompt-library .detail-danger-button:hover:not(:disabled)) {
    background: var(--ui-surface-subtle);
    color: var(--ui-danger);
  }

  :global(.prompt-library .detail-danger-button:focus-visible) {
    color: var(--ui-danger);
  }

  .save-status {
    display: inline-flex;
    align-items: center;
    min-height: 1.32rem;
    padding: 0 0.16rem;
    color: var(--ui-text-tertiary);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    line-height: 1;
    white-space: nowrap;
  }

  .save-status-dirty,
  .save-status-saving {
    color: var(--ui-accent-strong);
  }

  .save-status-error {
    color: var(--ui-danger);
  }

  .disabled-detail-warning {
    margin: 0;
    padding: 0.45rem 0.55rem;
    border: 1px solid color-mix(in oklab, var(--ui-warning) 36%, var(--ui-border-soft));
    border-radius: var(--ui-radius-sm);
    background: color-mix(in oklab, var(--ui-warning-soft) 54%, transparent);
    color: color-mix(in oklab, var(--ui-warning) 42%, var(--ui-text-primary));
    font-size: var(--text-xs);
    line-height: 1.4;
  }

  .reset-dialog-actions {
    display: flex;
    justify-content: flex-end;
    gap: 0.42rem;
  }

  .field {
    display: grid;
    gap: 0.32rem;
  }

  .field :global(.ui-textarea) {
    min-height: 14rem;
    font-family: var(--font-mono);
    font-size: var(--text-sm);
  }

  .control-grid {
    display: grid;
    gap: 0.58rem;
    padding: 0.1rem 0 0;
  }

  .control-section {
    display: grid;
    gap: 0.34rem;
  }

  .control-label {
    color: var(--ui-text-secondary);
    font-size: var(--text-xs);
    font-weight: 600;
  }

  .control-caption {
    max-width: 34rem;
    color: var(--ui-text-tertiary);
    font-size: var(--text-xs);
    line-height: 1.38;
  }

  .actor-inclusion-chips {
    display: flex;
    flex-wrap: wrap;
    gap: 0.34rem;
  }

  .actor-inclusion-chip,
  .scope-check-row {
    display: flex;
    align-items: center;
    gap: 0.38rem;
    color: var(--ui-text-secondary);
    font-size: var(--text-xs);
  }

  .scope-control-row {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 1rem;
  }

  .actor-inclusion-chip {
    min-height: 1.32rem;
    padding: 0.16rem 0.38rem;
    border-radius: var(--ui-radius-sm);
    background: color-mix(in oklab, var(--ui-surface-muted) 54%, transparent);
    color: var(--ui-text-secondary);
    font-family: var(--font-mono);
    font-weight: 500;
    line-height: 1;
    white-space: nowrap;
    cursor: pointer;
    transition:
      background-color 150ms cubic-bezier(0.19, 1, 0.22, 1),
      color 150ms cubic-bezier(0.19, 1, 0.22, 1),
      opacity 150ms cubic-bezier(0.19, 1, 0.22, 1);
  }

  .actor-inclusion-chip:not(.checked) {
    opacity: 0.78;
  }

  .actor-inclusion-chip.checked {
    background: color-mix(in oklab, var(--ui-surface-muted) 70%, transparent);
    color: var(--ui-text-primary);
  }

  .actor-inclusion-chip:hover {
    opacity: 1;
  }

  .scope-check-row {
    min-height: 1.42rem;
    width: fit-content;
    cursor: pointer;
  }

  .detail-section {
    display: grid;
    gap: 0.5rem;
  }

  .included-block {
    grid-template-columns: minmax(0, 1fr) max-content;
    align-items: center;
  }

  .generated-entry {
    display: grid;
    gap: 0.4rem;
    padding: 0.55rem;
    border: 1px solid color-mix(in oklab, var(--ui-border-soft) 76%, transparent);
    border-radius: var(--ui-radius-sm);
    background: color-mix(in oklab, var(--ui-surface-subtle) 72%, transparent);
  }

  .generated-entry > span {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 0.6rem;
  }

  .generated-entry-title {
    display: grid;
    min-width: 0;
    gap: 0.12rem;
  }

  .generated-entry-title strong,
  .generated-entry-title code {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  pre {
    max-height: 12rem;
    margin: 0;
    overflow: auto;
    padding: 0.56rem;
    border-radius: var(--ui-radius-sm);
    background: color-mix(in oklab, var(--ui-code) 92%, transparent);
    color: var(--ui-text-primary);
    font-family: var(--font-mono);
    font-size: var(--text-xs);
    line-height: 1.5;
    white-space: pre-wrap;
  }

  pre code {
    color: inherit;
    font-size: inherit;
  }

  .library-message {
    margin: 0;
    padding: 0.6rem 0.78rem;
  }

  .library-message.inline {
    padding: 0;
    color: var(--ui-accent-strong);
  }

  .library-message.error {
    color: var(--ui-danger);
  }
</style>
