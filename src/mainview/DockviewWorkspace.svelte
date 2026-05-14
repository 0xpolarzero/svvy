<script lang="ts">
  import { mount, unmount } from "svelte";
  import { onDestroy, onMount } from "svelte";
  import {
    DockviewComponent,
    type GroupPanelPartInitParameters,
    type IContentRenderer,
    type IHeaderActionsRenderer,
    type IDisposable,
    type ITabRenderer,
    type SerializedDockview,
    type TabPartInitParameters,
  } from "dockview-core";
  import "dockview-core/dist/styles/dockview.css";
  import DockviewPanelHost from "./DockviewPanelHost.svelte";
  import type { ChatRuntime } from "./chat-runtime";
  import type { WorkspaceDockviewPanelState } from "./pane-layout";

  type DockviewPanelRenderer = "onlyWhenVisible" | "always";

  type Props = {
    runtime: ChatRuntime;
    panels: WorkspaceDockviewPanelState[];
    dockviewLayout: SerializedDockview | null;
    focusedPanelId: string | null;
    layoutEpoch?: number;
    onFocusPanel: (panelId: string) => void;
    onPersistDockview: (dockview: SerializedDockview | null, focusedPanelId: string | null) => void;
  };

  let {
    runtime,
    panels,
    dockviewLayout,
    focusedPanelId,
    layoutEpoch = 0,
    onFocusPanel,
    onPersistDockview,
  }: Props = $props();
  let hostElement = $state<HTMLDivElement | null>(null);
  let dockview: DockviewComponent | null = null;
  let applying = false;
  let layoutFrame: number | null = null;
  let unsubscribeRuntime: (() => void) | null = null;
  const tabRenderers = new Set<SurfaceTabRenderer>();

  type PaneTabState = "waiting" | "streaming" | "error" | null;

  type PaneTabPresentation = {
    title: string;
    typeLabel: string;
    state: PaneTabState;
  };

  class SurfaceContentRenderer implements IContentRenderer {
    readonly element = document.createElement("div");
    private component: Record<string, unknown> | null = null;
    private panelId = "";

    init(params: GroupPanelPartInitParameters): void {
      this.element.className = "dockview-surface-content";
      this.panelId = params.api.id;
      this.component = mount(DockviewPanelHost, {
        target: this.element,
        props: {
          runtime,
          panelId: this.panelId,
        },
      }) as Record<string, unknown>;
    }

    update(): void {
      if (!this.component) return;
      unmount(this.component);
      this.component = mount(DockviewPanelHost, {
        target: this.element,
        props: {
          runtime,
          panelId: this.panelId,
        },
      }) as Record<string, unknown>;
    }

    dispose(): void {
      if (this.component) {
        unmount(this.component);
        this.component = null;
      }
      this.element.replaceChildren();
    }
  }

  class SurfaceTabRenderer implements ITabRenderer {
    readonly element = document.createElement("button");
    private params: TabPartInitParameters | null = null;

    init(params: TabPartInitParameters): void {
      this.params = params;
      this.element.type = "button";
      this.element.className = "dockview-surface-tab";
      this.element.addEventListener("click", this.focus);
      tabRenderers.add(this);
      this.render();
    }

    update(): void {
      this.render();
    }

    dispose(): void {
      this.element.removeEventListener("click", this.focus);
      tabRenderers.delete(this);
    }

    private focus = (): void => {
      if (!this.params) return;
      this.params.containerApi.setActivePanel(this.params.api);
      onFocusPanel(this.params.api.id);
    };

    render(): void {
      const panelId = this.params?.api.id ?? "";
      const panel = panels.find((candidate) => candidate.panelId === panelId);
      const presentation = getPaneTabPresentation(panelId);

      this.element.replaceChildren();
      this.element.title = `${presentation.title} · ${presentation.typeLabel}${
        presentation.state ? ` · ${presentation.state}` : ""
      }`;
      this.element.ariaLabel = this.element.title;
      this.element.dataset.focused = panel?.panelId === focusedPanelId ? "true" : "false";
      this.element.dataset.state = presentation.state ?? "idle";

      const title = document.createElement("span");
      title.className = "dockview-surface-tab-title";
      title.textContent = presentation.title;

      const type = document.createElement("span");
      type.className = "dockview-surface-tab-type";
      type.textContent = presentation.typeLabel;

      this.element.append(title, type);

      if (presentation.state) {
        const statePin = document.createElement("span");
        statePin.className = "dockview-surface-tab-state";
        statePin.dataset.state = presentation.state;
        statePin.ariaHidden = "true";
        this.element.append(statePin);
      }
    }
  }

  function refreshSurfaceTabs(): void {
    for (const renderer of tabRenderers) {
      renderer.render();
    }
  }

  function getPaneTabPresentation(panelId: string): PaneTabPresentation {
    const panel = panels.find((candidate) => candidate.panelId === panelId) ?? null;
    const binding = panel?.binding ?? null;
    const session = binding?.workspaceSessionId
      ? runtime.sessions.find((candidate) => candidate.id === binding.workspaceSessionId) ?? null
      : null;
    const controller = runtime.getPaneController(panelId);
    const typeLabel = getPaneTypeLabel(panel);
    const title = session?.title ?? panel?.chrome?.title ?? "Surface";

    return {
      title,
      typeLabel,
      state: getPaneTabState(panel, session, controller),
    };
  }

  function getPaneTypeLabel(panel: WorkspaceDockviewPanelState | null): string {
    switch (panel?.binding?.surface) {
      case "orchestrator":
        return "orchestrator";
      case "thread":
        return "handler";
      case "workflow-inspector":
        return "workflow";
      case "saved-workflow-library":
        return "library";
      case "app-logs":
        return "logs";
      case "command":
        return "command";
      case "workflow-task-attempt":
        return "task";
      case "artifact":
        return "artifact";
      case "project-ci-check":
        return "ci";
      default:
        return "empty";
    }
  }

  function getPaneTabState(
    panel: WorkspaceDockviewPanelState | null,
    session: ChatRuntime["sessions"][number] | null,
    controller: ReturnType<ChatRuntime["getPaneController"]>,
  ): PaneTabState {
    if (controller?.agent.state.error || session?.status === "error") {
      return "error";
    }
    if (controller?.agent.state.isStreaming || controller?.promptStatus === "streaming") {
      return "streaming";
    }
    if (panel?.binding?.surface === "thread") {
      const threadId = panel.binding.threadId;
      if (threadId && session?.threadIdsByStatus?.waiting.includes(threadId)) {
        return "waiting";
      }
      return null;
    }
    if (panel?.binding?.surface === "orchestrator" && session?.status === "waiting") {
      return "waiting";
    }
    return null;
  }

  class SurfaceHeaderActionsRenderer implements IHeaderActionsRenderer {
    readonly element = document.createElement("div");
    private group: Parameters<IHeaderActionsRenderer["init"]>[0]["group"] | null = null;
    private activePanelDisposable: IDisposable | null = null;

    init(params: Parameters<IHeaderActionsRenderer["init"]>[0]): void {
      this.group = params.group;
      this.element.className = "dockview-surface-actions";
      this.element.append(
        this.createActionButton("Duplicate pane right", "split-right", () => this.duplicate("right")),
        this.createActionButton("Duplicate pane below", "split-below", () => this.duplicate("below")),
        this.createActionButton("Close pane", "close", () => this.close()),
      );
      this.syncDisabledState();
      this.activePanelDisposable = params.api.onDidActivePanelChange(() => this.syncDisabledState());
    }

    dispose(): void {
      this.activePanelDisposable?.dispose();
      this.activePanelDisposable = null;
      this.element.replaceChildren();
      this.group = null;
    }

    private get activePanelId(): string | null {
      return this.group?.activePanel?.id ?? null;
    }

    private createActionButton(
      label: string,
      icon: "split-right" | "split-below" | "close",
      action: () => void,
    ): HTMLButtonElement {
      const button = document.createElement("button");
      button.type = "button";
      button.className = `dockview-surface-action action-${icon}`;
      button.ariaLabel = label;
      button.innerHTML = getActionIcon(icon);
      attachDelayedTooltip(button, label);
      button.addEventListener("click", (event) => {
        event.preventDefault();
        event.stopPropagation();
        action();
      });
      return button;
    }

    private duplicate(direction: "right" | "below"): void {
      const panelId = this.activePanelId;
      if (!panelId) return;
      void runtime.splitPane(panelId, direction, { duplicateBinding: true });
    }

    private close(): void {
      const panelId = this.activePanelId;
      if (!panelId) return;
      void runtime.closePane(panelId);
    }

    private syncDisabledState(): void {
      const disabled = !this.activePanelId;
      for (const child of this.element.querySelectorAll("button")) {
        child.toggleAttribute("disabled", disabled);
      }
    }
  }

  function getActionIcon(icon: "split-right" | "split-below" | "close"): string {
    if (icon === "close") {
      return '<svg viewBox="0 0 16 16" aria-hidden="true"><path d="M4.25 4.25 11.75 11.75M11.75 4.25 4.25 11.75" /></svg>';
    }
    if (icon === "split-below") {
      return '<svg viewBox="0 0 16 16" aria-hidden="true"><rect x="2.75" y="2.75" width="10.5" height="10.5" rx="1.6" /><path d="M2.75 8h10.5" /><path d="M8 10.75v-2.5M6.75 9.5 8 10.75 9.25 9.5" /></svg>';
    }
    return '<svg viewBox="0 0 16 16" aria-hidden="true"><rect x="2.75" y="2.75" width="10.5" height="10.5" rx="1.6" /><path d="M8 2.75v10.5" /><path d="M10.75 8.1h-2.5M9.5 6.85l1.25 1.25L9.5 9.35" /></svg>';
  }

  function attachDelayedTooltip(button: HTMLButtonElement, label: string): void {
    let timer: ReturnType<typeof setTimeout> | null = null;
    let tooltip: HTMLDivElement | null = null;

    const remove = () => {
      if (timer) {
        clearTimeout(timer);
        timer = null;
      }
      tooltip?.remove();
      tooltip = null;
    };

    const show = () => {
      if (button.disabled || tooltip) return;
      const rect = button.getBoundingClientRect();
      tooltip = document.createElement("div");
      tooltip.className = "imperative-action-tooltip";
      tooltip.textContent = label;
      tooltip.style.left = `${Math.max(10, Math.min(window.innerWidth - 10, rect.left + rect.width / 2))}px`;
      tooltip.style.top = `${Math.max(10, Math.min(window.innerHeight - 10, rect.bottom + 8))}px`;
      document.body.append(tooltip);
    };

    const schedule = () => {
      remove();
      timer = setTimeout(show, 1000);
    };

    button.addEventListener("pointerenter", schedule);
    button.addEventListener("focus", schedule);
    button.addEventListener("pointerleave", remove);
    button.addEventListener("blur", remove);
    button.addEventListener("click", remove);
  }

  function createDockview(): void {
    if (!hostElement || dockview) return;
    dockview = new DockviewComponent(hostElement, {
      proportionalLayout: true,
      createComponent: () => new SurfaceContentRenderer(),
      createTabComponent: () => new SurfaceTabRenderer(),
      createRightHeaderActionComponent: () => new SurfaceHeaderActionsRenderer(),
      defaultRenderer: "onlyWhenVisible",
      defaultTabComponent: "surfaceTab",
      noPanelsOverlay: "emptyGroup",
      dndEdges: {
        activationSize: { value: 72, type: "pixels" },
        size: { value: 28, type: "percentage" },
        smallWidthBoundary: 420,
        smallHeightBoundary: 280,
      },
      getTabContextMenuItems: ({ panel }) => [
        {
          label: "Duplicate Panel",
          action: () => void runtime.splitPane(panel.id, "right", { duplicateBinding: true }),
        },
        {
          label: "Close Panel",
          action: () => void runtime.closePane(panel.id),
        },
        "separator",
        "close",
      ],
    });

    dockview.onDidActivePanelChange((panel) => {
      if (panel) onFocusPanel(panel.id);
      persistDockview();
    });
    dockview.onDidAddPanel(() => persistDockview());
    dockview.onDidRemovePanel((panel) => {
      if (!applying) {
        void runtime.closePane(panel.id);
      }
      persistDockview();
    });
    dockview.onDidLayoutChange(() => persistDockview());
    dockview.onDidDrop(() => persistDockview());
    dockview.onDidCreateTabGroup(() => persistDockview());
    dockview.onDidDestroyTabGroup(() => persistDockview());
    syncDockviewPanels();
    if (dockviewLayout) {
      try {
        applying = true;
        dockview.fromJSON(dockviewLayout, { reuseExistingPanels: true });
      } catch {
        syncDockviewPanels();
      } finally {
        applying = false;
      }
      syncDockviewPanels();
    }
    scheduleDockviewLayout();
  }

  function persistDockview(): void {
    if (!dockview || applying) return;
    onPersistDockview(dockview.toJSON(), dockview.activePanel?.id ?? null);
  }

  function layoutDockview(): void {
    if (!dockview || !hostElement) return;
    const width = hostElement.clientWidth;
    const height = hostElement.clientHeight;
    if (width <= 0 || height <= 0) return;
    const wasApplying = applying;
    applying = true;
    try {
      dockview.layout(width, height, true);
    } finally {
      applying = wasApplying;
    }
  }

  function scheduleDockviewLayout(): void {
    if (layoutFrame !== null) {
      window.cancelAnimationFrame(layoutFrame);
    }
    layoutFrame = window.requestAnimationFrame(() => {
      layoutFrame = null;
      layoutDockview();
    });
  }

  function getDockviewPanel(panelId: string) {
    return dockview?.panels.find((panel) => panel.id === panelId);
  }

  function getPanelRenderer(panel: WorkspaceDockviewPanelState): DockviewPanelRenderer {
    const surface = panel.binding?.surface;
    return surface === "orchestrator" || surface === "thread" ? "always" : "onlyWhenVisible";
  }

  function syncDockviewPanels(): void {
    if (!dockview) return;
    applying = true;
    try {
      for (const panel of panels) {
        if (!getDockviewPanel(panel.panelId)) {
          const referencePanel = panel.placement?.referencePanelId
            ? getDockviewPanel(panel.placement.referencePanelId)
            : undefined;
          dockview.addPanel({
            id: panel.panelId,
            component: "surface",
            tabComponent: "surfaceTab",
            title: panel.chrome?.title ?? "Surface",
            renderer: getPanelRenderer(panel),
            minimumWidth: 0,
            minimumHeight: 0,
            inactive: panel.panelId !== focusedPanelId,
            position:
              dockview.totalPanels > 0
                ? {
                    referencePanel: referencePanel ?? dockview.activePanel ?? dockview.panels[0]!,
                    direction: panel.placement?.direction ?? "right",
                  }
                : undefined,
          });
        }
      }
      for (const panel of dockview.panels) {
        if (!panels.some((candidate) => candidate.panelId === panel.id)) {
          dockview.removePanel(panel);
        }
      }
      const focused = focusedPanelId ? getDockviewPanel(focusedPanelId) : undefined;
      if (focused && dockview.activePanel?.id !== focused.id) {
        dockview.setActivePanel(focused);
      }
    } finally {
      applying = false;
    }
  }

  $effect(() => {
    panels;
    focusedPanelId;
    syncDockviewPanels();
    refreshSurfaceTabs();
  });

  $effect(() => {
    layoutEpoch;
    scheduleDockviewLayout();
  });

  onMount(() => {
    createDockview();
    unsubscribeRuntime = runtime.subscribe(refreshSurfaceTabs);
  });

  onDestroy(() => {
    unsubscribeRuntime?.();
    unsubscribeRuntime = null;
    if (layoutFrame !== null) {
      window.cancelAnimationFrame(layoutFrame);
      layoutFrame = null;
    }
    dockview?.dispose();
    dockview = null;
  });
</script>

<div class="dockview-workbench" data-testid="dockview-workbench" bind:this={hostElement}></div>

<style>
  .dockview-workbench {
    min-height: 0;
    height: 100%;
    width: 100%;
    overflow: hidden;
    --dv-background-color: var(--ui-shell);
    --dv-paneview-active-outline-color: var(--ui-accent);
    --dv-tabs-and-actions-container-background-color: color-mix(in oklab, var(--ui-panel) 92%, var(--ui-shell));
    --dv-activegroup-visiblepanel-tab-background-color: var(--ui-panel);
    --dv-inactivegroup-visiblepanel-tab-background-color: color-mix(in oklab, var(--ui-panel) 86%, var(--ui-shell));
    --dv-activegroup-hiddenpanel-tab-background-color: color-mix(in oklab, var(--ui-panel) 70%, var(--ui-shell));
    --dv-inactivegroup-hiddenpanel-tab-background-color: color-mix(in oklab, var(--ui-panel) 66%, var(--ui-shell));
    --dv-separator-border: var(--ui-border);
    --dv-tabs-container-scrollbar-color: var(--ui-border-strong);
  }

  :global(.dockview-surface-content) {
    height: 100%;
    min-height: 0;
    overflow: hidden;
    background: var(--ui-panel);
  }

  :global(.dockview-surface-tab) {
    display: inline-flex;
    align-items: center;
    gap: 0.38rem;
    height: 100%;
    min-width: 0;
    border: 0;
    background: transparent;
    color: var(--ui-text);
    font: inherit;
    font-size: 0.76rem;
    padding: 0 0.55rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  :global(.dockview-surface-tab[data-focused="true"]) {
    color: var(--ui-accent);
  }

  :global(.dockview-surface-tab-title) {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-weight: 580;
    color: var(--ui-text-primary);
  }

  :global(.dockview-surface-tab-type) {
    flex: 0 0 auto;
    padding: 0.08rem 0.26rem;
    border: 1px solid color-mix(in oklab, var(--ui-border-strong) 58%, transparent);
    border-radius: var(--ui-radius-sm);
    background: color-mix(in oklab, var(--ui-surface-subtle) 72%, transparent);
    color: var(--ui-text-tertiary);
    font-family: var(--font-mono);
    font-size: 0.58rem;
    font-weight: 650;
    line-height: 1.15;
  }

  :global(.dockview-surface-tab-state) {
    flex: 0 0 auto;
    width: 0.43rem;
    height: 0.43rem;
    border-radius: 999px;
    box-shadow: 0 0 0 1px color-mix(in oklab, var(--ui-panel) 86%, transparent);
  }

  :global(.dockview-surface-tab-state[data-state="streaming"]) {
    background: var(--ui-accent);
    box-shadow:
      0 0 0 1px color-mix(in oklab, var(--ui-accent) 42%, transparent),
      0 0 0.45rem color-mix(in oklab, var(--ui-accent) 42%, transparent);
    animation: dockview-state-pulse 1.4s ease-in-out infinite;
  }

  :global(.dockview-surface-tab-state[data-state="waiting"]) {
    background: var(--ui-status-waiting);
  }

  :global(.dockview-surface-tab-state[data-state="error"]) {
    background: var(--ui-danger);
  }

  @keyframes dockview-state-pulse {
    0%,
    100% {
      opacity: 0.62;
      transform: scale(0.92);
    }
    50% {
      opacity: 1;
      transform: scale(1);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    :global(.dockview-surface-tab-state[data-state="streaming"]) {
      animation: none;
    }
  }

  :global(.dockview-surface-actions) {
    display: flex;
    align-items: center;
    gap: 0.16rem;
    height: 100%;
    padding: 0 0.32rem 0 0.18rem;
  }

  :global(.dockview-surface-action) {
    display: grid;
    place-items: center;
    width: 1.45rem;
    height: 1.45rem;
    border: 0;
    border-radius: var(--ui-radius-sm);
    background: transparent;
    color: var(--ui-text-tertiary);
    cursor: pointer;
  }

  :global(.dockview-surface-action:hover:not(:disabled)),
  :global(.dockview-surface-action:focus-visible) {
    background: color-mix(in oklab, var(--ui-surface-subtle) 84%, var(--ui-accent) 16%);
    color: var(--ui-text-primary);
    outline: none;
  }

  :global(.dockview-surface-action.action-close:hover:not(:disabled)),
  :global(.dockview-surface-action.action-close:focus-visible) {
    background: color-mix(in oklab, var(--ui-danger-soft) 72%, var(--ui-surface) 28%);
    color: var(--ui-danger);
  }

  :global(.dockview-surface-action:disabled) {
    cursor: default;
    opacity: 0.36;
  }

  :global(.dockview-surface-action svg) {
    width: 0.9rem;
    height: 0.9rem;
    fill: none;
    stroke: currentColor;
    stroke-linecap: round;
    stroke-linejoin: round;
    stroke-width: 1.55;
  }

  :global(.imperative-action-tooltip) {
    position: fixed;
    z-index: var(--ui-z-dialog);
    max-width: min(18rem, calc(100vw - 1.25rem));
    padding: 0.34rem 0.46rem;
    border: 1px solid color-mix(in oklab, var(--ui-border-strong) 72%, transparent);
    border-radius: var(--ui-radius-sm);
    background: color-mix(in oklab, var(--ui-surface-raised) 96%, black 4%);
    color: var(--ui-text-primary);
    box-shadow:
      0 18px 36px color-mix(in oklab, black 28%, transparent),
      0 2px 8px color-mix(in oklab, black 18%, transparent);
    font-size: 0.68rem;
    font-weight: 560;
    line-height: 1.25;
    pointer-events: none;
    transform: translate(-50%, 0);
    animation: dockview-tooltip-in 110ms cubic-bezier(0.19, 1, 0.22, 1);
  }

  @keyframes dockview-tooltip-in {
    from {
      opacity: 0;
      filter: blur(2px);
    }
    to {
      opacity: 1;
      filter: blur(0);
    }
  }
</style>
