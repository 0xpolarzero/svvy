<script lang="ts">
  import { mount, unmount } from "svelte";
  import { onDestroy, onMount } from "svelte";
  import { DockviewComponent, type IContentRenderer, type ITabRenderer, type GroupPanelPartInitParameters, type TabPartInitParameters, type SerializedDockview } from "dockview-core";
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

  function createDockview(): void {
    if (!hostElement || dockview) return;
    dockview = new DockviewComponent(hostElement, {
      proportionalLayout: true,
      createComponent: () => new SurfaceContentRenderer(),
      createTabComponent: () => new SurfaceTabRenderer(),
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
</style>
