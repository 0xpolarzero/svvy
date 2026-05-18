import type { SerializedDockview } from "dockview-core";
import type { WorkspacePaneSurfaceTarget } from "../shared/workspace-contract";
import type { WorkspaceInspectorSelection } from "./chat-storage";

export const PRIMARY_CHAT_PANE_ID = "primary";
export const MIN_PANE_HEIGHT_PX = 260;
export const WORKSPACE_LAYOUT_SLOT_IDS = ["A", "B", "C"] as const;

export type DockviewSplitDirection = "left" | "right" | "above" | "below";
export type WorkspaceLayoutSlotId = (typeof WORKSPACE_LAYOUT_SLOT_IDS)[number];

export interface PaneLocalState {
  scroll: null | {
    transcriptAnchorId: string | null;
    offsetPx: number;
  };
  inspectorSelection: WorkspaceInspectorSelection | null;
  timelineDensity: "compact" | "comfortable";
}

export type DockviewPanelChromeKind =
  | "orchestrator"
  | "handler-thread"
  | "workflow-inspector"
  | "artifact"
  | "project-ci"
  | "saved-workflow-library"
  | "prompt-library"
  | "app-logs"
  | "open-workspace"
  | "command"
  | "workflow-task-attempt"
  | "empty"
  | "unavailable";

export interface DockviewPanelChromeState {
  title: string;
  subtitle: string | null;
  icon: string | null;
  kind: DockviewPanelChromeKind;
  closable: boolean;
  floatable: boolean;
  popoutable: boolean;
}

export interface DockviewPanelRestoreState {
  unavailableReason: string | null;
  lastKnownLocationLabel: string | null;
}

export interface DockviewPanelPlacementState {
  referencePanelId: string;
  direction: DockviewSplitDirection;
  size?: number;
}

export interface WorkspaceDockviewPanelState {
  panelId: string;
  binding: WorkspacePaneSurfaceTarget | null;
  localState: PaneLocalState;
  chrome?: DockviewPanelChromeState;
  placement?: DockviewPanelPlacementState | null;
  restore?: DockviewPanelRestoreState;
}

export interface CompactThreadSurfaceState {
  kind: "compact-thread";
  workspaceSessionId: string;
  threadId: string;
  panelId: string | null;
  density: PaneLocalState["timelineDensity"];
}

export interface CompactWorkflowRunSurfaceState {
  kind: "compact-workflow-run";
  workspaceSessionId: string;
  threadId: string;
  workflowRunId: string;
  panelId: string | null;
  density: PaneLocalState["timelineDensity"];
}

export type CompactWorkspaceSurfaceState =
  | CompactThreadSurfaceState
  | CompactWorkflowRunSurfaceState;

export interface WorkspaceDockviewLayoutState {
  dockview: SerializedDockview | null;
  panels: WorkspaceDockviewPanelState[];
  compactSurfaces: CompactWorkspaceSurfaceState[];
  focusedPanelId: string | null;
  updatedAt: string;
}

export type WorkspacePaneLayoutState = WorkspaceDockviewLayoutState;

export interface WorkspaceLayoutSlotSummary {
  id: WorkspaceLayoutSlotId;
  initialized: boolean;
  active: boolean;
  updatedAt: string | null;
}

export type DockviewOpenTarget =
  | { kind: "focused-panel" }
  | { kind: "panel"; panelId: string }
  | { kind: "split"; panelId: string; direction: DockviewSplitDirection; size?: number }
  | { kind: "tab"; groupId: string; index?: number }
  | { kind: "new-panel"; direction: "right" | "below"; size?: number }
  | { kind: "edge"; direction: DockviewSplitDirection; size?: number }
  | { kind: "floating"; box?: { x: number; y: number; width: number; height: number } }
  | { kind: "popout"; box?: { left: number; top: number; width: number; height: number } };

export type PaneOpenTarget = DockviewOpenTarget;

export function getSidebarSessionOpenTarget(event?: Pick<MouseEvent, "metaKey">): PaneOpenTarget {
  return event?.metaKey ? { kind: "focused-panel" } : { kind: "new-panel", direction: "right" };
}

export function createDefaultPaneLocalState(): PaneLocalState {
  return {
    scroll: null,
    inspectorSelection: null,
    timelineDensity: "comfortable",
  };
}

export function createPanelChrome(
  binding: WorkspacePaneSurfaceTarget | null,
): DockviewPanelChromeState {
  if (!binding) {
    return {
      title: "Empty",
      subtitle: null,
      icon: null,
      kind: "empty",
      closable: true,
      floatable: true,
      popoutable: false,
    };
  }

  switch (binding.surface) {
    case "orchestrator":
      return chrome("Orchestrator", binding.workspaceSessionId, "orchestrator", true);
    case "thread":
      return chrome(
        "Handler Thread",
        binding.threadId ?? binding.surfacePiSessionId,
        "handler-thread",
        true,
      );
    case "workflow-inspector":
      return chrome("Workflow Inspector", binding.workflowRunId, "workflow-inspector", true);
    case "saved-workflow-library":
      return chrome("Workflows", ".svvy/workflows", "saved-workflow-library", true);
    case "prompt-library":
      return chrome("Context", "instructions", "prompt-library", true);
    case "app-logs":
      return chrome("Logs", "workspace", "app-logs", true);
    case "open-workspace":
      return chrome("Open Workspace", "choose a folder", "open-workspace", true);
    case "command":
      return chrome("Command Inspector", binding.commandId, "command", true);
    case "workflow-task-attempt":
      return chrome(
        "Workflow Task-Agent",
        binding.workflowTaskAttemptId,
        "workflow-task-attempt",
        true,
      );
    case "artifact":
      return chrome("Artifact", binding.artifactId, "artifact", true);
    case "project-ci-check":
      return chrome("Project CI Check", binding.checkResultId, "project-ci", true);
  }
}

export function createDockviewPanelState(
  panelId: string,
  binding: WorkspacePaneSurfaceTarget,
  placement: DockviewPanelPlacementState | null = null,
): WorkspaceDockviewPanelState {
  return {
    panelId,
    binding: { ...binding },
    localState: createDefaultPaneLocalState(),
    chrome: createPanelChrome(binding),
    placement: placement ? { ...placement } : null,
    restore: {
      unavailableReason: null,
      lastKnownLocationLabel: null,
    },
  };
}

export function createEmptyPaneLayout(
  now = new Date().toISOString(),
): WorkspaceDockviewLayoutState {
  return {
    dockview: null,
    panels: [],
    compactSurfaces: [],
    focusedPanelId: null,
    updatedAt: now,
  };
}

export function isInitializedPaneLayout(layout: WorkspaceDockviewLayoutState): boolean {
  return layout.panels.some((panel) => panel.binding !== null);
}

export function normalizePaneLayout(
  layout: Partial<WorkspaceDockviewLayoutState>,
  now = new Date().toISOString(),
): WorkspaceDockviewLayoutState {
  const rawPanels = Array.isArray(layout.panels) ? layout.panels : [];

  if (rawPanels.length === 0) {
    return createEmptyPaneLayout(now);
  }

  const panels = rawPanels.flatMap((panel) => {
    const next = panel as Partial<WorkspaceDockviewPanelState>;
    if (!next.binding) {
      return [];
    }
    const binding = { ...next.binding };
    return [
      {
        panelId: String(next.panelId ?? createPanelId()),
        binding,
        localState: {
          ...createDefaultPaneLocalState(),
          ...next.localState,
          inspectorSelection: next.localState?.inspectorSelection ?? null,
          scroll: next.localState?.scroll ?? null,
        },
        chrome: {
          ...createPanelChrome(binding),
          ...next.chrome,
        },
        placement: normalizePlacement(next.placement),
        restore: {
          unavailableReason: null,
          lastKnownLocationLabel: null,
          ...next.restore,
        },
      },
    ];
  });

  if (panels.length === 0) {
    return createEmptyPaneLayout(now);
  }

  const focusedPanelId =
    layout.focusedPanelId && panels.some((panel) => panel.panelId === layout.focusedPanelId)
      ? layout.focusedPanelId
      : panels[0]!.panelId;
  const droppedPanels = panels.length !== rawPanels.length;
  const panelIds = new Set(panels.map((panel) => panel.panelId));
  const dockview = sanitizeSerializedDockview(layout.dockview, panelIds, droppedPanels);

  return {
    dockview,
    panels,
    compactSurfaces: Array.isArray(layout.compactSurfaces)
      ? layout.compactSurfaces.map((surface) => ({ ...surface }))
      : [],
    focusedPanelId,
    updatedAt: now,
  };
}

export function bindPane(
  layout: WorkspaceDockviewLayoutState,
  panelId: string,
  binding: WorkspacePaneSurfaceTarget | null,
): WorkspaceDockviewLayoutState {
  return touch({
    ...layout,
    panels: layout.panels.map((panel) =>
      panel.panelId === panelId
        ? { ...panel, binding: binding ? { ...binding } : null, chrome: createPanelChrome(binding) }
        : panel,
    ),
    focusedPanelId: panelId,
  });
}

export function focusPane(
  layout: WorkspaceDockviewLayoutState,
  panelId: string,
): WorkspaceDockviewLayoutState {
  if (!layout.panels.some((panel) => panel.panelId === panelId)) {
    return layout;
  }
  return touch({ ...layout, focusedPanelId: panelId });
}

export function setPaneInspectorSelection(
  layout: WorkspaceDockviewLayoutState,
  panelId: string,
  selection: WorkspaceInspectorSelection | null,
): WorkspaceDockviewLayoutState {
  return touch({
    ...layout,
    panels: layout.panels.map((panel) =>
      panel.panelId === panelId
        ? {
            ...panel,
            localState: {
              ...panel.localState,
              inspectorSelection: selection ? structuredClone(selection) : null,
            },
          }
        : panel,
    ),
  });
}

export function setPaneScroll(
  layout: WorkspaceDockviewLayoutState,
  panelId: string,
  scroll: PaneLocalState["scroll"],
): WorkspaceDockviewLayoutState {
  return touch({
    ...layout,
    panels: layout.panels.map((panel) =>
      panel.panelId === panelId
        ? { ...panel, localState: { ...panel.localState, scroll: scroll ? { ...scroll } : null } }
        : panel,
    ),
  });
}

export function addDockviewPanel(
  layout: WorkspaceDockviewLayoutState,
  binding: WorkspacePaneSurfaceTarget,
  panelId = createPanelId(),
  placement: DockviewPanelPlacementState | null = null,
): WorkspaceDockviewLayoutState {
  return touch({
    ...layout,
    panels: [...layout.panels, createDockviewPanelState(panelId, binding, placement)],
    focusedPanelId: panelId,
  });
}

export function removeDockviewPanel(
  layout: WorkspaceDockviewLayoutState,
  panelId: string,
): WorkspaceDockviewLayoutState {
  const panels = layout.panels.filter((panel) => panel.panelId !== panelId);
  const panelIds = new Set(panels.map((panel) => panel.panelId));
  return touch({
    ...layout,
    dockview: sanitizeSerializedDockview(
      layout.dockview,
      panelIds,
      panels.length !== layout.panels.length,
    ),
    panels,
    focusedPanelId:
      layout.focusedPanelId === panelId ? (panels[0]?.panelId ?? null) : layout.focusedPanelId,
  });
}

export function splitPane(
  layout: WorkspaceDockviewLayoutState,
  panelId: string,
  direction: DockviewSplitDirection,
  options: { duplicateBinding?: boolean; size?: number; nextPaneId?: string } = {},
): WorkspaceDockviewLayoutState {
  const source = layout.panels.find((panel) => panel.panelId === panelId);
  if (!options.duplicateBinding || !source?.binding) {
    return layout;
  }
  const binding = { ...source.binding };
  return addDockviewPanel(layout, binding, options.nextPaneId ?? createPanelId(), {
    referencePanelId: panelId,
    direction,
    size: options.size,
  });
}

export function closePane(
  layout: WorkspaceDockviewLayoutState,
  panelId: string,
): WorkspaceDockviewLayoutState {
  return removeDockviewPanel(layout, panelId);
}

export function setDockviewSerializedLayout(
  layout: WorkspaceDockviewLayoutState,
  dockview: SerializedDockview | null,
  focusedPanelId = layout.focusedPanelId,
): WorkspaceDockviewLayoutState {
  const panelIds = new Set(layout.panels.map((panel) => panel.panelId));
  return touch({
    ...layout,
    dockview: sanitizeSerializedDockview(dockview, panelIds),
    focusedPanelId,
  });
}

export function getOpenPaneLocations(
  layout: WorkspaceDockviewLayoutState,
  predicate: (binding: WorkspacePaneSurfaceTarget) => boolean,
): { paneId: string; panelId: string; label: string; focused: boolean }[] {
  return layout.panels
    .filter((panel) => panel.binding && predicate(panel.binding))
    .map((panel, index) => ({
      paneId: panel.panelId,
      panelId: panel.panelId,
      label:
        panel.restore?.lastKnownLocationLabel ?? (index === 0 ? "Docked" : `Docked ${index + 1}`),
      focused: panel.panelId === layout.focusedPanelId,
    }));
}

export function createPanelId(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return `panel-${crypto.randomUUID()}`;
  }
  return `panel-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

function chrome(
  title: string,
  subtitle: string | null,
  kind: DockviewPanelChromeKind,
  floatable: boolean,
): DockviewPanelChromeState {
  return {
    title,
    subtitle,
    icon: null,
    kind,
    closable: true,
    floatable,
    popoutable: false,
  };
}

function touch(layout: WorkspaceDockviewLayoutState): WorkspaceDockviewLayoutState {
  return {
    ...layout,
    updatedAt: new Date().toISOString(),
  };
}

function normalizePlacement(value: unknown): DockviewPanelPlacementState | null {
  if (!value || typeof value !== "object") {
    return null;
  }
  const candidate = value as Partial<DockviewPanelPlacementState>;
  if (
    typeof candidate.referencePanelId !== "string" ||
    !["left", "right", "above", "below"].includes(String(candidate.direction))
  ) {
    return null;
  }
  return {
    referencePanelId: candidate.referencePanelId,
    direction: candidate.direction as DockviewSplitDirection,
    size: typeof candidate.size === "number" ? candidate.size : undefined,
  };
}

function isSerializedDockview(value: unknown): value is SerializedDockview {
  return !!value && typeof value === "object" && "grid" in value && "panels" in value;
}

function sanitizeSerializedDockview(
  value: unknown,
  panelIds: Set<string>,
  forceDiscard = false,
): SerializedDockview | null {
  if (forceDiscard || !isSerializedDockview(value)) {
    return null;
  }

  const serializedPanelIds = collectSerializedDockviewPanelIds(value);
  if (serializedPanelIds.size === 0) {
    return panelIds.size === 0 ? null : value;
  }

  if (serializedPanelIds.size !== panelIds.size) {
    return null;
  }
  for (const panelId of serializedPanelIds) {
    if (!panelIds.has(panelId)) {
      return null;
    }
  }

  return value;
}

function collectSerializedDockviewPanelIds(value: SerializedDockview): Set<string> {
  const ids = new Set<string>();
  const panels = (value as { panels?: unknown }).panels;
  if (Array.isArray(panels)) {
    for (const panel of panels) {
      if (panel && typeof panel === "object") {
        const id = (panel as { id?: unknown }).id;
        if (typeof id === "string") {
          ids.add(id);
        }
      }
    }
    return ids;
  }
  if (panels && typeof panels === "object") {
    for (const id of Object.keys(panels)) {
      ids.add(id);
    }
  }
  return ids;
}
