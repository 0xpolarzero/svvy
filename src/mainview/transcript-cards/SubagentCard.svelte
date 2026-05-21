<script lang="ts" module>
  import type { TranscriptStatus } from "./StatusBadge.svelte";

  export type TranscriptAgentType =
    | "orchestrator"
    | "handler-thread"
    | "workflow-task-agent"
    | "explorer"
    | "implementer"
    | "reviewer"
    | "workflow-writer";

  export type TranscriptSubagent = {
    id: string;
    type: TranscriptAgentType;
    headline: string;
    status: TranscriptStatus;
    model: string;
    tokens?: number;
  };
</script>

<script lang="ts">
  import BotIcon from "@lucide/svelte/icons/bot";
  import Code2Icon from "@lucide/svelte/icons/code-2";
  import EyeIcon from "@lucide/svelte/icons/eye";
  import SearchIcon from "@lucide/svelte/icons/search";
  import WorkflowIcon from "@lucide/svelte/icons/workflow";
  import ModelBadge from "./ModelBadge.svelte";
  import StatusBadge from "./StatusBadge.svelte";

  type Props = {
    agent: TranscriptSubagent;
    class?: string;
    expandable?: boolean;
    onclick?: (agent: TranscriptSubagent) => void;
  };

  let { agent, class: className = "", expandable = true, onclick }: Props = $props();

  const agentConfig = {
    orchestrator: { icon: BotIcon, label: "orchestrator", tone: "orange" },
    "handler-thread": { icon: BotIcon, label: "handler", tone: "blue" },
    "workflow-task-agent": { icon: WorkflowIcon, label: "task-agent", tone: "cyan" },
    explorer: { icon: SearchIcon, label: "explorer", tone: "blue" },
    implementer: { icon: Code2Icon, label: "implementer", tone: "purple" },
    reviewer: { icon: EyeIcon, label: "reviewer", tone: "cyan" },
    "workflow-writer": { icon: WorkflowIcon, label: "workflow-writer", tone: "muted" },
  } as const;

  const config = $derived(agentConfig[agent.type] ?? agentConfig["handler-thread"]);
  const Icon = $derived(config.icon);
  const tokenLabel = $derived(agent.tokens ? `${(agent.tokens / 1000).toFixed(1)}k` : null);

  function open() {
    if (!expandable) return;
    onclick?.(agent);
  }

  function keydown(event: KeyboardEvent) {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      open();
    }
  }
</script>

<button
  type="button"
  class={`transcript-subagent-card flex items-center gap-2 px-2 py-1.5 bg-muted/40 border border-border/50 text-left min-w-0 ${expandable ? "hover:bg-muted/70 transition-colors cursor-pointer group focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring" : ""} ${className}`.trim()}
  aria-disabled={!expandable}
  onclick={open}
  onkeydown={keydown}
  data-testid={`subagent-card-${agent.id}`}
>
  <Icon class="w-3.5 h-3.5 text-muted-foreground group-hover:text-foreground transition-colors flex-shrink-0" strokeWidth={2.1} />

  <div class="flex-1 min-w-0 flex items-center gap-1.5">
    <span class="text-xs font-medium text-foreground truncate">{agent.headline}</span>
    <span class="text-xs font-mono text-muted-foreground/60 uppercase tracking-wide flex-shrink-0">{config.label}</span>
  </div>

  <div class="flex items-center gap-1.5 flex-shrink-0">
    <StatusBadge status={agent.status} dotOnly size="xs" />
    {#if tokenLabel}
      <span class="text-xs font-mono text-muted-foreground/60">{tokenLabel}</span>
    {/if}
    <ModelBadge model={agent.model} size="xs" />
    {#if expandable}
      <span class="text-xs font-mono text-muted-foreground/40" aria-hidden="true">-&gt;</span>
    {/if}
  </div>
</button>

<style>
  .transcript-subagent-card {
    border-radius: var(--ui-radius-md);
  }
</style>
