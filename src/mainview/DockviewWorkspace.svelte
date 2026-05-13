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
    focusedPanelId: string;
    onFocusPanel: (panelId: string) => void;
    onPersistDockview: (dockview: SerializedDockview | null, focusedPanelId: string | null) => void;
  };

  let { runtime, panels, dockviewLayout, focusedPanelId, onFocusPanel, onPersistDockview }: Props = $props();
  let hostElement = $state<HTMLDivElement | null>(null);
  let dockview: DockviewComponent | null = null;
  let applying = false;

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
      this.render();
    }

    update(): void {
      this.render();
    }

    dispose(): void {
      this.element.removeEventListener("click", this.focus);
    }

    private focus = (): void => {
      if (!this.params) return;
      this.params.containerApi.setActivePanel(this.params.api);
      onFocusPanel(this.params.api.id);
    };

    private render(): void {
      const panel = panels.find((candidate) => candidate.panelId === this.params?.api.id);
      this.element.textContent = panel?.chrome?.title ?? this.params?.title ?? "Surface";
      this.element.dataset.focused = panel?.panelId === focusedPanelId ? "true" : "false";
    }
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
      button.title = label;
      button.innerHTML = getActionIcon(icon);
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
  }

  function persistDockview(): void {
    if (!dockview || applying) return;
    onPersistDockview(dockview.toJSON(), dockview.activePanel?.id ?? null);
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
            minimumWidth: 320,
            minimumHeight: 260,
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
  });

  onMount(() => {
    createDockview();
  });

  onDestroy(() => {
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
</style>
